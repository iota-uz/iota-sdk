package sdk

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPromote_WaitsForExternalReleaseAndAppliesDependenciesOnce(t *testing.T) {
	moduleName, err := (ExecRunner{}).Run(context.Background(), "../../../..", nil, "env", "GOWORK=off", "go", "list", "-m")
	require.NoError(t, err)
	modulePath := strings.TrimSpace(string(moduleName))
	root := t.TempDir()
	d := Dependency{PR: 12, Channel: "preview", GoDir: "."}
	require.NoError(t, WriteDependency(root, d))
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("old"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.work"), []byte("// Managed by sdk-tools sdk preview\n"), 0600))
	sha := strings.Repeat("a", 40)
	ready := &Candidate{Version: "0.6.0", SHA: sha, Source: sha, Phase: "ready"}
	runChecks, updates := 0, 0
	runner := fakeRunner{call: func(_ string, _ []byte, name string, args []string) ([]byte, error) {
		if name == "git" {
			require.Equal(t, "status", args[0])
			return nil, nil
		}
		if name == "env" {
			if args[2] == "get" {
				require.Equal(t, modulePath+"@v"+ready.Version, args[len(args)-1])
				updates++
				return nil, os.WriteFile(filepath.Join(root, "go.mod"), []byte("new"), 0600)
			}
			if args[2] == "list" {
				require.Equal(t, modulePath, args[len(args)-1])
				return []byte(`{"Version":"v0.6.0"}`), nil
			}
			if args[2] == "vet" {
				return nil, nil
			}
		}
		require.Equal(t, "gh", name)
		require.Equal(t, "api", args[0])
		require.Equal(t, "GET", args[2], "promotion must only read from GitHub")

		endpoint := strings.TrimPrefix(args[3], "repos/"+Repository+"/")
		switch {
		case endpoint == "pulls/12":
			return encoded(Pull{Merged: true, Merge: sha}), nil
		case strings.HasPrefix(endpoint, "contents/state.json"):
			state := State{}
			if runChecks >= 2 {
				state.Ready = ready
				state.Releases = map[string]*Candidate{ready.Version: ready}
			}
			return fileResponse(state, "state"), nil
		case strings.HasPrefix(endpoint, "actions/workflows/"):
			runChecks++
			return []byte(`{"workflow_runs":[]}`), nil
		case strings.HasPrefix(endpoint, "compare/"):
			return []byte(`{"status":"identical"}`), nil
		case endpoint == "commits/v0.6.0":
			return encoded(map[string]string{"sha": sha}), nil
		}
		return nil, fmt.Errorf("unexpected API %s", endpoint)
	}}
	g := GitHub{Runner: runner, Repo: Repository}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var output bytes.Buffer
	require.NoError(t, g.promote(ctx, root, &output, time.Millisecond))
	require.Equal(t, 2, runChecks)
	require.Contains(t, output.String(), "Waiting for a verified SDK release containing PR #12")
	got, err := ReadDependency(root)
	require.NoError(t, err)
	require.Equal(t, "release", got.Channel)
	require.Equal(t, ready.Version, got.Version)
	require.NoFileExists(t, filepath.Join(root, "go.work"))
	require.NoError(t, g.Promote(context.Background(), root, io.Discard))
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
	require.Error(t, (GitHub{Runner: runner, Repo: Repository}).Promote(context.Background(), root, io.Discard))
}

func TestPromote_StopsWaitingWhenCanceled(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, WriteDependency(root, Dependency{PR: 12, Channel: "preview", GoDir: "."}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runner := fakeRunner{call: func(_ string, _ []byte, name string, args []string) ([]byte, error) {
		require.Equal(t, "gh", name)
		require.Equal(t, "api", args[0])
		require.Equal(t, "GET", args[2])
		switch endpoint := strings.TrimPrefix(args[3], "repos/"+Repository+"/"); {
		case endpoint == "pulls/12":
			return encoded(Pull{Merged: true, Merge: strings.Repeat("a", 40)}), nil
		case strings.HasPrefix(endpoint, "contents/state.json"):
			return fileResponse(State{}, "state"), nil
		case strings.HasPrefix(endpoint, "actions/workflows/"):
			cancel()
			return []byte(`{"workflow_runs":[]}`), nil
		default:
			return nil, fmt.Errorf("unexpected API %s", endpoint)
		}
	}}
	require.ErrorIs(t, (GitHub{Runner: runner, Repo: Repository}).Promote(ctx, root, io.Discard), context.Canceled)
}

func TestPromote_ReportsFailedVerificationAtCurrentSource(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, WriteDependency(root, Dependency{PR: 12, Channel: "preview", GoDir: "."}))
	sha := strings.Repeat("a", 40)
	runner := fakeRunner{call: func(_ string, _ []byte, name string, args []string) ([]byte, error) {
		require.Equal(t, "gh", name)
		require.Equal(t, "api", args[0])
		require.Equal(t, "GET", args[2])
		switch endpoint := strings.TrimPrefix(args[3], "repos/"+Repository+"/"); {
		case endpoint == "pulls/12":
			return encoded(Pull{Merged: true, Merge: sha}), nil
		case strings.HasPrefix(endpoint, "contents/state.json"):
			return fileResponse(State{Candidate: &Candidate{Source: sha, Phase: "failed"}}, "state"), nil
		case strings.HasPrefix(endpoint, "actions/workflows/"):
			return []byte(`{"workflow_runs":[]}`), nil
		case endpoint == "commits/main":
			return encoded(map[string]string{"sha": sha}), nil
		default:
			return nil, fmt.Errorf("unexpected API %s", endpoint)
		}
	}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := (GitHub{Runner: runner, Repo: Repository}).Promote(ctx, root, io.Discard)
	require.ErrorContains(t, err, "SDK verification failed at "+sha)
	require.ErrorContains(t, err, "inspect release.yml")
}
