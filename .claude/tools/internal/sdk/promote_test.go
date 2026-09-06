package sdk

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// False green: returning a ready release before watch would never exercise dispatch and waiting.
func TestPromote_DispatchesWaitsAndIsIdempotent(t *testing.T) {
	root := t.TempDir()
	d := Dependency{PR: 12, Channel: "preview", GoDir: "."}
	require.NoError(t, WriteDependency(root, d))
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("old"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.work"), []byte("// Managed by sdk-tools sdk preview\n"), 0600))
	sha := strings.Repeat("a", 40)
	ready := &Candidate{Version: "0.6.0", SHA: sha, Source: sha, Phase: "ready"}
	watched, updates := false, 0
	dispatched := false
	runner := fakeRunner{call: func(_ string, _ []byte, name string, args []string) ([]byte, error) {
		if name == "git" {
			require.Equal(t, "status", args[0])
			return nil, nil
		}
		if name == "env" {
			if args[2] == "get" {
				updates++
				return nil, os.WriteFile(filepath.Join(root, "go.mod"), []byte("new"), 0600)
			}
			if args[2] == "list" {
				return []byte(`{"Version":"v0.6.0"}`), nil
			}
			if args[2] == "vet" {
				return nil, nil
			}
		}
		if name == "gh" && args[0] == "run" {
			require.Equal(t, []string{"run", "watch", "123", "--repo", Repository, "--exit-status"}, args)
			watched = true
			return nil, nil
		}
		if name != "gh" || args[0] != "api" {
			return nil, fmt.Errorf("unexpected command %s %v", name, args)
		}
		if args[2] == "POST" {
			require.False(t, dispatched)
			require.Equal(t, "repos/"+Repository+"/actions/workflows/release.yml/dispatches", args[3])
			dispatched = true
			return nil, nil
		}
		require.Equal(t, "GET", args[2], "promotion must not modify a consumer repository")

		endpoint := strings.TrimPrefix(args[3], "repos/"+Repository+"/")
		switch {
		case endpoint == "pulls/12":
			return encoded(Pull{Merged: true, Merge: sha}), nil
		case strings.HasPrefix(endpoint, "contents/state.json"):
			state := State{}
			if watched {
				state.Ready = ready
				state.Releases = map[string]*Candidate{ready.Version: ready}
			}
			return fileResponse(state, "state"), nil
		case strings.HasPrefix(endpoint, "actions/workflows/"):
			if !dispatched {
				return []byte(`{"workflow_runs":[]}`), nil
			}
			return []byte(`{"workflow_runs":[{"id":123,"status":"in_progress","html_url":"https://github.com/example/run/123"}]}`), nil
		case strings.HasPrefix(endpoint, "compare/"):
			return []byte(`{"status":"identical"}`), nil
		case endpoint == "commits/v0.6.0":
			return encoded(map[string]string{"sha": sha}), nil
		}
		return nil, fmt.Errorf("unexpected API %s", endpoint)
	}}
	g := GitHub{Runner: runner, Repo: Repository}
	require.NoError(t, g.Promote(context.Background(), root, false, io.Discard))
	require.True(t, dispatched)
	got, err := ReadDependency(root)
	require.NoError(t, err)
	require.Equal(t, "release", got.Channel)
	require.Equal(t, ready.Version, got.Version)
	require.NoFileExists(t, filepath.Join(root, "go.work"))
	require.NoError(t, g.Promote(context.Background(), root, false, io.Discard))
	require.Equal(t, 1, updates)
}

// False green: a runner that doesn't write files cannot prove rollback after go get.
func TestFinalize_RestoresLocalFilesWhenVerificationFails(t *testing.T) {
	root := t.TempDir()
	d := Dependency{PR: 12, Channel: "preview", GoDir: "."}
	require.NoError(t, WriteDependency(root, d))
	path := filepath.Join(root, "go.mod")
	require.NoError(t, os.WriteFile(path, []byte("old"), 0600))
	sha := strings.Repeat("a", 40)
	runner := fakeRunner{call: func(_ string, _ []byte, name string, args []string) ([]byte, error) {
		if name == "git" {
			return nil, nil
		}
		if name == "env" {
			if args[2] == "get" {
				return nil, os.WriteFile(path, []byte("modified"), 0600)
			}
			if args[2] == "list" {
				return []byte(`{"Version":"v0.6.0"}`), nil
			}
			if args[2] == "vet" {
				return nil, fmt.Errorf("consumer fails verification")
			}
		}
		endpoint := args[3]
		switch {
		case strings.Contains(endpoint, "pulls/"):
			return encoded(Pull{Merged: true, Merge: sha}), nil
		case strings.Contains(endpoint, "contents/state.json"):
			return fileResponse(State{Ready: &Candidate{Version: "0.6.0", SHA: sha, Phase: "ready"}}, "state"), nil
		case strings.Contains(endpoint, "compare/"):
			return []byte(`{"status":"identical"}`), nil
		case strings.Contains(endpoint, "commits/"):
			return encoded(map[string]string{"sha": sha}), nil
		}
		return nil, fmt.Errorf("unexpected call")
	}}
	changed, err := (GitHub{Runner: runner, Repo: Repository}).Finalize(context.Background(), root)
	require.Error(t, err)
	require.False(t, changed)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, []byte("old"), data)
	got, err := ReadDependency(root)
	require.NoError(t, err)
	require.Equal(t, d, got)
}

// False green: allowing any command after an unmerged PR could start a costly release.
func TestPromote_RejectsUnmergedPR(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, WriteDependency(root, Dependency{PR: 12, Channel: "preview", GoDir: "."}))
	runner := fakeRunner{call: func(_ string, _ []byte, _ string, args []string) ([]byte, error) {
		require.Equal(t, "repos/"+Repository+"/pulls/12", args[3])
		return encoded(Pull{State: "open"}), nil
	}}
	require.Error(t, (GitHub{Runner: runner, Repo: Repository}).Promote(context.Background(), root, false, io.Discard))
}

// False green: observing only the first dispatch would miss an unbounded second release run.
func TestPromote_DoesNotRedispatchWhenItsRunFinishesWithoutReadyRelease(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, WriteDependency(root, Dependency{PR: 12, Channel: "preview", GoDir: "."}))
	sha := strings.Repeat("a", 40)
	dispatches := 0
	watched := false
	runner := fakeRunner{call: func(_ string, _ []byte, name string, args []string) ([]byte, error) {
		if name == "gh" && args[0] == "run" {
			watched = true
			return nil, nil
		}
		if name != "gh" || args[0] != "api" {
			return nil, fmt.Errorf("unexpected command %s %v", name, args)
		}
		method := args[2]
		endpoint := strings.TrimPrefix(args[3], "repos/"+Repository+"/")
		if method == "POST" {
			dispatches++
			return nil, nil
		}
		switch {
		case endpoint == "pulls/12":
			return encoded(Pull{Merged: true, Merge: sha}), nil
		case strings.HasPrefix(endpoint, "contents/state.json"):
			return fileResponse(State{}, "state"), nil
		case strings.HasPrefix(endpoint, "actions/workflows/"):
			if dispatches == 0 || watched {
				return []byte(`{"workflow_runs":[]}`), nil
			}
			return []byte(`{"workflow_runs":[{"id":123,"status":"in_progress"}]}`), nil
		}
		return nil, fmt.Errorf("unexpected API %s %s", method, endpoint)
	}}
	err := (GitHub{Runner: runner, Repo: Repository}).Promote(context.Background(), root, false, io.Discard)
	require.Error(t, err)
	require.Equal(t, 1, dispatches)
}
