package sdk

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// False green: only testing one change would miss precedence across a batch.
func TestNextVersion_BatchedChanges(t *testing.T) {
	for _, tc := range []struct {
		current string
		bumps   []string
		want    string
		bad     bool
	}{
		{"0.5.6", []string{"patch", "minor", "patch"}, "0.6.0", false},
		{"0.5.6", []string{"none", "patch"}, "0.5.7", false},
		{"0.5.6", []string{"major"}, "0.6.0", false},
		{"1.4.2", []string{"minor"}, "1.5.0", false},
		{"1.4.2", []string{"major"}, "", true},
		{"0.5.6", []string{"none"}, "", true},
		{"0.5.6-rc.1", []string{"patch"}, "", true},
		{"0.5.6", []string{"typo"}, "", true},
	} {
		t.Run(tc.current+strings.Join(tc.bumps, "+"), func(t *testing.T) {
			var changes []Change
			for _, bump := range tc.bumps {
				changes = append(changes, Change{Bump: bump, Summary: "changed behavior"})
			}
			got, err := NextVersion(tc.current, changes)
			if tc.bad {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

type fakeRunner struct {
	call func(string, []byte, string, []string) ([]byte, error)
}

func (r fakeRunner) Run(_ context.Context, dir string, input []byte, name string, args ...string) ([]byte, error) {
	return r.call(dir, input, name, args)
}
func encoded(value any) []byte { data, _ := json.Marshal(value); return data }
func fileResponse(value any, sha string) []byte {
	return encoded(content{SHA: sha, Content: base64.StdEncoding.EncodeToString(encoded(value))})
}

// False green: a fake that ignores the second read would hide lost concurrent requests.
func TestUpdateState_RetriesConflictWithoutLosingAnotherRequest(t *testing.T) {
	state := State{Requests: []int{1}}
	writes := 0
	runner := fakeRunner{call: func(_ string, input []byte, _ string, args []string) ([]byte, error) {
		if args[2] == "GET" {
			return fileResponse(state, fmt.Sprint(writes)), nil
		}
		writes++
		if writes == 1 {
			state.Requests = append(state.Requests, 2)
			return nil, fmt.Errorf("HTTP 409")
		}
		var request map[string]string
		require.NoError(t, json.Unmarshal(input, &request))
		data, err := base64.StdEncoding.DecodeString(request["content"])
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(data, &state))
		return []byte(`{}`), nil
	}}
	err := (GitHub{Runner: runner, Repo: Repository}).UpdateState(context.Background(), func(s *State) error {
		s.Requests = append(s.Requests, 3)
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, []int{1, 2, 3}, state.Requests)
}

// False green: main must differ from the candidate to exercise publication recovery.
func TestPrepare_ResumesPublishedCandidateWithoutRebuilding(t *testing.T) {
	candidate := Candidate{Version: "0.6.0", SHA: strings.Repeat("a", 40), Source: strings.Repeat("b", 40), Phase: "publishing", RunID: "123"}
	runner := fakeRunner{call: func(_ string, _ []byte, _ string, args []string) ([]byte, error) {
		path := args[3]
		if strings.Contains(path, "contents/state.json") {
			return fileResponse(State{Candidate: &candidate}, "state"), nil
		}
		if strings.HasSuffix(path, "commits/main") {
			return encoded(map[string]string{"sha": strings.Repeat("c", 40)}), nil
		}
		t.Fatalf("unexpected API: %v", args)
		return nil, nil
	}}
	got, err := (GitHub{Runner: runner, Repo: Repository}).Prepare(context.Background(), "456", false)
	require.NoError(t, err)
	require.Equal(t, &candidate, got)
}

// False green: checking only the successful case would permit retry-until-green.
func TestPrepare_DoesNotAutomaticallyRetryFailedTestsAtSameSource(t *testing.T) {
	source := strings.Repeat("a", 40)
	runner := fakeRunner{call: func(_ string, _ []byte, _ string, args []string) ([]byte, error) {
		if strings.Contains(args[3], "contents/state.json") {
			return fileResponse(State{Candidate: &Candidate{Source: source, Phase: "failed"}}, "state"), nil
		}
		if strings.HasSuffix(args[3], "commits/main") {
			return encoded(map[string]string{"sha": source}), nil
		}
		t.Fatalf("unexpected API: %v", args)
		return nil, nil
	}}
	got, err := (GitHub{Runner: runner, Repo: Repository}).Prepare(context.Background(), "456", false)
	require.NoError(t, err)
	require.Nil(t, got)
}

// False green: a mock returning ahead for unrelated histories would certify the wrong release.
func TestResolve_RequiresAncestryAndAnUnchangedTag(t *testing.T) {
	for _, scenario := range []string{"unrelated", "moved-tag", "ready"} {
		t.Run(scenario, func(t *testing.T) {
			sha := strings.Repeat("a", 40)
			runner := fakeRunner{call: func(_ string, _ []byte, _ string, args []string) ([]byte, error) {
				switch {
				case strings.Contains(args[3], "pulls/"):
					return encoded(Pull{Merged: true, Merge: strings.Repeat("b", 40)}), nil
				case strings.Contains(args[3], "contents/state.json"):
					return fileResponse(State{Ready: &Candidate{SHA: sha, Version: "0.6.0", Phase: "ready"}}, "state"), nil
				case strings.Contains(args[3], "compare/"):
					status := "ahead"
					if scenario == "unrelated" {
						status = "diverged"
					}
					return encoded(map[string]string{"status": status}), nil
				case strings.Contains(args[3], "commits/v"):
					if scenario == "moved-tag" {
						return encoded(map[string]string{"sha": strings.Repeat("c", 40)}), nil
					}
					return encoded(map[string]string{"sha": sha}), nil
				default:
					t.Fatalf("unexpected API %v", args)
					return nil, nil
				}
			}}
			got, err := (GitHub{Runner: runner, Repo: Repository}).Resolve(context.Background(), 10)
			if scenario == "moved-tag" {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if scenario == "unrelated" {
				require.Nil(t, got)
			} else {
				require.Equal(t, sha, got.SHA)
			}
		})
	}
}

// False green: a clean-only fixture would not detect destructive declaration edits.
func TestCheckChanges_UsesRealGitDiff(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	git("init", "-q")
	git("config", "user.name", "Test")
	git("config", "user.email", "test@example.invalid")
	require.NoError(t, os.Mkdir(filepath.Join(root, ".changes"), 0755))
	path := filepath.Join(root, ".changes", "one.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"bump":"patch","summary":"fix"}`), 0644))
	git("add", ".")
	git("commit", "-qm", "baseline")
	git("branch", "baseline")
	require.NoError(t, os.WriteFile(filepath.Join(root, ".changes", "two.json"), []byte(`{"bump":"minor","summary":"feature"}`), 0644))
	git("add", ".")
	git("commit", "-qm", "feature")
	require.NoError(t, CheckChanges(context.Background(), ExecRunner{}, root, "baseline"))
	require.NoError(t, os.WriteFile(path, []byte(`{"bump":"minor","summary":"rewritten"}`), 0644))
	git("add", ".")
	git("commit", "-qm", "rewrite")
	require.Error(t, CheckChanges(context.Background(), ExecRunner{}, root, "baseline"))
}

// False green: lexical checks alone accept a symlink escaping the consumer root.
func TestSafeDir_RejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Symlink(t.TempDir(), filepath.Join(root, "web")))
	_, err := safeDir(root, "web")
	require.Error(t, err)
}

// False green: unknown fields silently ignored can turn a release request into a preview.
func TestDecode_RejectsUnknownFieldsAndTrailingValues(t *testing.T) {
	_, err := Decode[Change]([]byte(`{"bump":"patch","summary":"fix","typo":true}`))
	require.Error(t, err)
	_, err = Decode[Change]([]byte(`{"bump":"patch","summary":"fix"} {}`))
	require.Error(t, err)
}
