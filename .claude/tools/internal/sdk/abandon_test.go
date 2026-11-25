package sdk

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const zipFailure = `{"Path":"github.com/iota-uz/iota-sdk","Version":"v0.55.0","Error":"github.com/iota-uz/iota-sdk@v0.55.0: reading https://proxy.golang.org/github.com/iota-uz/iota-sdk/@v/v0.55.0.info: 404 Not Found\n\tserver response: not found: create zip: pkg/automation/catalog/nul.go: malformed file path"}`

// abandonRunner serves the release state and answers the module download with download; it records
// the state written and the download command.
func abandonRunner(t *testing.T, state *State, download string, written *bool, command *[]string) fakeRunner {
	t.Helper()
	return fakeRunner{call: func(_ string, input []byte, name string, args []string) ([]byte, error) {
		if name == "env" {
			*command = args
			return []byte(download), nil
		}
		method, endpoint := args[2], strings.TrimPrefix(args[3], "repos/"+Repository+"/")
		switch {
		case endpoint == "contents/state.json?ref="+StateBranch:
			return fileResponse(*state, "state-sha"), nil
		case endpoint == "contents/state.json" && method == "PUT":
			var body map[string]string
			require.NoError(t, json.Unmarshal(input, &body))
			data, err := base64.StdEncoding.DecodeString(body["content"])
			require.NoError(t, err)
			*state = State{}
			require.NoError(t, json.Unmarshal(data, state))
			*written = true
			return []byte(`{}`), nil
		}
		return nil, fmt.Errorf("unexpected API: %s %s", method, endpoint)
	}}
}

// False green: checking only the returned candidate would miss a state that still holds the candidate,
// loses ready, or forgets the reason.
func TestAbandon_RetiresAnUndownloadablePartialPublication(t *testing.T) {
	sha := strings.Repeat("a", 40)
	ready := &Candidate{Version: "0.54.0", SHA: strings.Repeat("b", 40), Phase: "ready"}
	state := State{Requests: []int{1421}, Ready: ready, Releases: map[string]*Candidate{"0.54.0": ready},
		Candidate: &Candidate{Version: "0.55.0", SHA: sha, Phase: "publishing", RunID: "1"}}
	var written bool
	var command []string
	abandoned, proof, err := (GitHub{Runner: abandonRunner(t, &state, zipFailure, &written, &command), Repo: Repository}).Abandon(context.Background(), sha)
	require.NoError(t, err)
	require.True(t, written)
	require.Nil(t, state.Candidate)
	require.Equal(t, "0.54.0", state.Ready.Version)
	require.Equal(t, []int{1421}, state.Requests)
	require.Equal(t, AbandonedPhase, state.Releases["0.55.0"].Phase)
	require.Contains(t, proof, "create zip")
	require.Equal(t, abandoned, *state.Releases["0.55.0"])
	require.Contains(t, command, "GOPROXY="+goModuleProxy)
	require.Contains(t, command, "GOFLAGS=")
	require.Equal(t, GoModulePath+"@v0.55.0", command[len(command)-1])
}

// False green: a fake that always fails the download would accept abandoning a release that is fine,
// or one the proxy could not answer for.
func TestAbandon_RefusesWithoutProofOrForAnotherCandidate(t *testing.T) {
	sha := strings.Repeat("a", 40)
	for name, tc := range map[string]struct {
		candidate Candidate
		sha       string
		download  string
		err       string
	}{
		"module downloads": {Candidate{Version: "0.55.0", SHA: sha, Phase: "publishing"}, sha, `{"Version":"v0.55.0","Zip":"/x.zip"}`, "resume its publication"},
		"network failure":  {Candidate{Version: "0.55.0", SHA: sha, Phase: "publishing"}, sha, `{"Version":"v0.55.0","Error":"dial tcp: lookup proxy.golang.org: no such host"}`, "could not prove"},
		"no JSON answer":   {Candidate{Version: "0.55.0", SHA: sha, Phase: "publishing"}, sha, ``, "no JSON answer"},
		"another SHA":      {Candidate{Version: "0.55.0", SHA: sha, Phase: "publishing"}, strings.Repeat("c", 40), zipFailure, "another SHA"},
		"failed candidate": {Candidate{Version: "0.55.0", SHA: sha, Phase: "failed"}, sha, zipFailure, "superseded"},
	} {
		t.Run(name, func(t *testing.T) {
			candidate := tc.candidate
			state := State{Candidate: &candidate}
			var written bool
			var command []string
			_, _, err := (GitHub{Runner: abandonRunner(t, &state, tc.download, &written, &command), Repo: Repository}).Abandon(context.Background(), tc.sha)
			require.ErrorContains(t, err, tc.err)
			require.False(t, written)
		})
	}
}

// False green: testing only a version equal to the abandoned one would miss two abandoned patches or a
// bump that already clears them.
func TestPastAbandoned_NeverReissuesAnAbandonedVersion(t *testing.T) {
	abandoned := func(versions ...string) map[string]*Candidate {
		releases := map[string]*Candidate{"0.54.0": {Version: "0.54.0", Phase: "ready"}}
		for _, v := range versions {
			releases[v] = &Candidate{Version: v, Phase: AbandonedPhase}
		}
		return releases
	}
	require.Equal(t, "0.55.0", pastAbandoned("0.55.0", abandoned()))
	require.Equal(t, "0.55.1", pastAbandoned("0.55.0", abandoned("0.55.0")))
	require.Equal(t, "0.55.1", pastAbandoned("0.54.1", abandoned("0.55.0")))
	require.Equal(t, "0.55.2", pastAbandoned("0.55.0", abandoned("0.55.0", "0.55.1")))
	require.Equal(t, "0.56.0", pastAbandoned("0.56.0", abandoned("0.55.0")))
}

// False green: pastAbandoned alone would not prove that Prepare applies it to the version it commits.
func TestPrepare_ReleasesAboveAnAbandonedVersion(t *testing.T) {
	source, readySource, readySHA, merge, sha := strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("d", 40), strings.Repeat("e", 40), strings.Repeat("c", 40)
	ready := &Candidate{Version: "0.54.0", Source: readySource, SHA: readySHA, Phase: "ready"}
	state := State{Requests: []int{1421}, Ready: ready, Releases: map[string]*Candidate{
		"0.54.0": ready,
		"0.55.0": {Version: "0.55.0", SHA: strings.Repeat("f", 40), Phase: AbandonedPhase},
	}}
	var createdRefs []string
	committed := map[string]string{}
	runner := fakeRunner{call: func(_ string, input []byte, _ string, args []string) ([]byte, error) {
		method, endpoint := args[2], strings.TrimPrefix(args[3], "repos/"+Repository+"/")
		file := func(text string) []byte {
			return encoded(content{SHA: "f", Content: base64.StdEncoding.EncodeToString([]byte(text))})
		}
		switch {
		case endpoint == "contents/state.json?ref="+StateBranch:
			return fileResponse(state, "state-sha"), nil
		case endpoint == "contents/state.json" && method == "PUT":
			var body map[string]string
			require.NoError(t, json.Unmarshal(input, &body))
			data, err := base64.StdEncoding.DecodeString(body["content"])
			require.NoError(t, err)
			state = State{}
			require.NoError(t, json.Unmarshal(data, &state))
			return []byte(`{}`), nil
		case endpoint == "commits/main":
			return encoded(map[string]string{"sha": source}), nil
		case strings.HasPrefix(endpoint, "pulls/"):
			return encoded(Pull{Merged: true, Merge: merge}), nil
		case endpoint == "compare/"+merge+"..."+source:
			return []byte(`{"status":"ahead"}`), nil
		case endpoint == "compare/"+merge+"..."+readySHA:
			return []byte(`{"status":"diverged"}`), nil
		case strings.HasPrefix(endpoint, "contents/web/sdk/package.json"):
			return file("{\n  \"version\": \"0.54.0\"\n}"), nil
		case strings.HasPrefix(endpoint, "contents/pkg/sdkidentity/identity.go"):
			return file("const ReleaseVersion = \"0.54.0\"\n"), nil
		case strings.HasPrefix(endpoint, "contents/web/client-host/src/identity.ts"):
			return file("export const SDK_RELEASE_VERSION = '0.54.0'\n"), nil
		case endpoint == "git/trees/"+readySource+"?recursive=1":
			return []byte(`{"tree":[]}`), nil
		case endpoint == "git/trees/"+source+"?recursive=1":
			return []byte(`{"tree":[{"path":".changes/feature.json","sha":"feature"}]}`), nil
		case strings.HasPrefix(endpoint, "contents/.changes/feature.json"):
			return fileResponse(Change{Bump: "minor", Summary: "feature"}, "feature"), nil
		case endpoint == "git/commits/"+source:
			return []byte(`{"tree":{"sha":"base-tree"}}`), nil
		case endpoint == "git/trees" && method == "POST":
			var body struct {
				Tree []struct {
					Path    string `json:"path"`
					Content string `json:"content"`
				} `json:"tree"`
			}
			require.NoError(t, json.Unmarshal(input, &body))
			for _, entry := range body.Tree {
				committed[entry.Path] = entry.Content
			}
			return []byte(`{"sha":"tree"}`), nil
		case endpoint == "git/commits" && method == "POST":
			return encoded(map[string]string{"sha": sha}), nil
		case endpoint == "git/refs" && method == "POST":
			var body map[string]string
			require.NoError(t, json.Unmarshal(input, &body))
			createdRefs = append(createdRefs, body["ref"])
			return []byte(`{}`), nil
		}
		return nil, fmt.Errorf("unexpected API: %s %s", method, endpoint)
	}}
	candidate, err := (GitHub{Runner: runner, Repo: Repository}).Prepare(context.Background(), "200", false)
	require.NoError(t, err)
	require.Equal(t, "0.55.1", candidate.Version)
	require.Equal(t, candidate, state.Candidate)
	require.Equal(t, AbandonedPhase, state.Releases["0.55.0"].Phase)
	require.Contains(t, committed["pkg/sdkidentity/identity.go"], `"0.55.1"`)
	require.Contains(t, committed["web/client-host/src/identity.ts"], `'0.55.1'`)
	require.Equal(t, []string{"refs/heads/sdk-candidates/v0.55.1-" + source[:12]}, createdRefs)
}
