package sdk

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// False green: merely checking the candidate struct would miss wrong Go/npm
// bytes or creating a public tag before verification. Inspect the API writes.
func TestPrepare_CreatesVersionedCandidateWithoutPublishing(t *testing.T) {
	source, baseline, sha := strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40)
	packageSource := "{\n  \"name\": \"@iota-uz/sdk\",\n  \"exports\": {\n    \".\": {\n      \"types\": \"./dist/index.d.ts\",\n      \"import\": \"./dist/index.js\",\n      \"default\": \"./dist/index.js\"\n    }\n  },\n  \"version\": \"0.5.6\"\n}"
	state := State{Requests: []int{12, 13}}
	var committedFiles map[string]string
	var createdRefs []string
	runner := fakeRunner{call: func(_ string, input []byte, _ string, args []string) ([]byte, error) {
		method, endpoint := args[2], strings.TrimPrefix(args[3], "repos/"+Repository+"/")
		switch {
		case endpoint == "contents/state.json?ref="+StateBranch:
			return fileResponse(state, "state-sha"), nil
		case endpoint == "contents/state.json" && method == "PUT":
			var body map[string]string
			require.NoError(t, json.Unmarshal(input, &body))
			data, err := base64.StdEncoding.DecodeString(body["content"])
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(data, &state))
			return []byte(`{}`), nil
		case endpoint == "commits/main":
			return encoded(map[string]string{"sha": source}), nil
		case endpoint == "commits/v0.5.6":
			return encoded(map[string]string{"sha": baseline}), nil
		case strings.HasPrefix(endpoint, "pulls/"):
			return encoded(Pull{Merged: true, Merge: source}), nil
		case strings.HasPrefix(endpoint, "compare/"):
			return []byte(`{"status":"identical"}`), nil
		case strings.HasPrefix(endpoint, "contents/web/sdk/package.json"):
			return encoded(content{SHA: "package", Content: base64.StdEncoding.EncodeToString([]byte(packageSource))}), nil
		case strings.HasPrefix(endpoint, "contents/pkg/sdkidentity/identity.go"):
			return encoded(content{Content: base64.StdEncoding.EncodeToString([]byte("package sdkidentity\nconst ReleaseVersion = \"0.5.6\"\n"))}), nil
		case endpoint == "git/trees/"+baseline+"?recursive=1":
			return []byte(`{"tree":[]}`), nil
		case endpoint == "git/trees/"+source+"?recursive=1":
			return []byte(`{"tree":[{"path":".changes/fix.json","sha":"fix"},{"path":".changes/feature.json","sha":"feature"}]}`), nil
		case strings.HasPrefix(endpoint, "contents/.changes/fix.json"):
			return fileResponse(Change{Bump: "patch", Summary: "fix"}, "fix"), nil
		case strings.HasPrefix(endpoint, "contents/.changes/feature.json"):
			return fileResponse(Change{Bump: "minor", Summary: "feature"}, "feature"), nil
		case endpoint == "git/commits/"+source:
			return []byte(`{"tree":{"sha":"base-tree"}}`), nil
		case endpoint == "git/trees" && method == "POST":
			var body struct {
				BaseTree string `json:"base_tree"`
				Tree     []struct{ Path, Content string }
			}
			require.NoError(t, json.Unmarshal(input, &body))
			require.Equal(t, "base-tree", body.BaseTree)
			committedFiles = map[string]string{}
			for _, entry := range body.Tree {
				committedFiles[entry.Path] = entry.Content
			}
			return []byte(`{"sha":"candidate-tree"}`), nil
		case endpoint == "git/commits" && method == "POST":
			var body struct {
				Parents []string
				Tree    string
			}
			require.NoError(t, json.Unmarshal(input, &body))
			require.Equal(t, []string{source}, body.Parents)
			require.Equal(t, "candidate-tree", body.Tree)
			return encoded(map[string]string{"sha": sha}), nil
		case endpoint == "git/refs" && method == "POST":
			var body map[string]string
			require.NoError(t, json.Unmarshal(input, &body))
			createdRefs = append(createdRefs, body["ref"])
			return []byte(`{}`), nil
		default:
			return nil, fmt.Errorf("unexpected API: %s %s", method, endpoint)
		}
	}}
	candidate, err := (GitHub{Runner: runner, Repo: Repository}).Prepare(context.Background(), "100", false)
	require.NoError(t, err)
	require.Equal(t, "0.6.0", candidate.Version)
	require.Equal(t, candidate, state.Candidate)
	require.Equal(t, []int{12, 13}, state.Requests)
	require.Equal(t, []string{"refs/heads/sdk-candidates/v0.6.0-" + source[:12]}, createdRefs)
	var pkg map[string]any
	require.NoError(t, json.Unmarshal([]byte(committedFiles["web/sdk/package.json"]), &pkg))
	require.Equal(t, "0.6.0", pkg["version"])
	require.Contains(t, committedFiles["pkg/sdkidentity/identity.go"], `"0.6.0"`)
	require.Equal(t, strings.Replace(packageSource, `"version": "0.5.6"`, `"version": "0.6.0"`, 1)+"\n", committedFiles["web/sdk/package.json"])
	var metadata struct {
		Version, Source string
		Changes         []Change
	}
	require.NoError(t, json.Unmarshal([]byte(committedFiles[".sdk/release.json"]), &metadata))
	require.Equal(t, source, metadata.Source)
	require.Len(t, metadata.Changes, 2)
}

// False green: without an unfinished publication, rejecting a stale SHA is insufficient.
func TestSetPhase_PartialPublicationCannotBeSuperseded(t *testing.T) {
	sha := strings.Repeat("a", 40)
	runner := fakeRunner{call: func(_ string, _ []byte, _ string, args []string) ([]byte, error) {
		require.Equal(t, "GET", args[2])
		return fileResponse(State{Candidate: &Candidate{SHA: sha, Phase: "publishing"}}, "state"), nil
	}}
	require.Error(t, (GitHub{Runner: runner, Repo: Repository}).SetPhase(context.Background(), sha, "failed"))
}

// False green: SetPhase protection alone would still let candidate creation overwrite publishing state.
func TestSaveCandidate_PartialPublicationCannotBeSuperseded(t *testing.T) {
	publishing := strings.Repeat("a", 40)
	replacement := strings.Repeat("b", 40)
	runner := fakeRunner{call: func(_ string, _ []byte, _ string, args []string) ([]byte, error) {
		require.Equal(t, "GET", args[2])
		return fileResponse(State{Candidate: &Candidate{SHA: publishing, Phase: "publishing"}}, "state"), nil
	}}
	err := (GitHub{Runner: runner, Repo: Repository}).saveCandidate(context.Background(), Candidate{SHA: replacement, Phase: "testing"})
	require.ErrorContains(t, err, "never superseded")
}

// False green: a backup-only assertion would not prove rollback of partially written locks.
func TestRestoreFiles_RestoresOldBytesAndRemovesNewFiles(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("original"), 0644))
	backups, err := backupFiles(root, []string{"go.mod", "go.sum"})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("changed"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.sum"), []byte("new"), 0644))
	require.NoError(t, restoreFiles(backups))
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	require.NoError(t, err)
	require.Equal(t, "original", string(data))
	_, err = os.Stat(filepath.Join(root, "go.sum"))
	require.True(t, os.IsNotExist(err))
}

// False green: checking only go.mod version would accept a local replace override.
func TestVerifyLocks_RejectsReplacementAndMismatchedVersion(t *testing.T) {
	for _, result := range []string{
		`{"Version":"v0.5.6"}`,
		`{"Version":"v0.6.0","Replace":{"Dir":"../preview"}}`,
	} {
		runner := fakeRunner{call: func(_ string, _ []byte, _ string, _ []string) ([]byte, error) { return []byte(result), nil }}
		err := VerifyLocks(context.Background(), runner, t.TempDir(), Dependency{PR: 1, GoDir: ".", Channel: "release", Version: "0.6.0", SHA: strings.Repeat("a", 40)})
		require.Error(t, err)
	}
}
