package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

func (g GitHub) Register(ctx context.Context, number int) error {
	if number <= 0 {
		return fmt.Errorf("a positive SDK PR number is required")
	}
	pr, err := g.Pull(ctx, number)
	if err != nil {
		return err
	}
	if pr.State == "closed" && !pr.Merged {
		return fmt.Errorf("SDK PR #%d was closed without merging", number)
	}
	return g.UpdateState(ctx, func(state *State) error {
		if !slices.Contains(state.Requests, number) {
			state.Requests = append(state.Requests, number)
		}
		return nil
	})
}

func (g GitHub) changes(ctx context.Context, ref string) (map[string]string, error) {
	var tree struct {
		Truncated bool `json:"truncated"`
		Tree      []struct {
			Path string `json:"path"`
			SHA  string `json:"sha"`
		} `json:"tree"`
	}
	if err := g.API(ctx, "GET", "git/trees/"+ref+"?recursive=1", nil, &tree); err != nil {
		return nil, err
	}
	if tree.Truncated {
		return nil, fmt.Errorf("GitHub truncated the tree; cannot safely calculate the release")
	}
	result := map[string]string{}
	for _, entry := range tree.Tree {
		if strings.HasPrefix(entry.Path, ".changes/") && strings.HasSuffix(entry.Path, ".json") {
			result[entry.Path] = entry.SHA
		}
	}
	return result, nil
}

func (g GitHub) Prepare(ctx context.Context, runID string, retry bool) (*Candidate, error) {
	state, _, err := g.ReadState(ctx)
	if err != nil {
		return nil, err
	}
	expectedCandidate := state.Candidate
	if len(state.Requests) == 0 && state.Candidate == nil {
		return nil, nil
	}
	source, err := g.Ref(ctx, "main")
	if err != nil {
		return nil, err
	}
	if state.Candidate != nil {
		candidate := *state.Candidate
		if candidate.Phase == "publishing" {
			return &candidate, nil
		}
		if candidate.Phase == "testing" && candidate.RunID != runID {
			var previous struct {
				Conclusion string `json:"conclusion"`
			}
			if err := g.API(ctx, "GET", "actions/runs/"+candidate.RunID, nil, &previous); err != nil {
				return nil, err
			}
			if previous.Conclusion == "" {
				return nil, nil
			}
			candidate.Phase = "failed"
			if err := g.saveCandidate(ctx, candidate, expectedCandidate); err != nil {
				return nil, err
			}
			savedCandidate := candidate
			expectedCandidate = &savedCandidate
		}
		// Publication is retried at the same SHA even when main advances.
		if candidate.Phase != "failed" || candidate.Source == source {
			if candidate.Phase == "failed" && !retry {
				return nil, nil
			}
			candidate.RunID = runID
			candidate.Phase = "testing"
			return &candidate, g.saveCandidate(ctx, candidate, expectedCandidate)
		}
	}
	needed := false
	var fulfilled []int
	for _, number := range state.Requests {
		pr, err := g.Pull(ctx, number)
		if err != nil {
			return nil, err
		}
		if !pr.Merged {
			if pr.State == "closed" {
				fulfilled = append(fulfilled, number)
			}
			continue
		}
		included, err := g.Contains(ctx, source, pr.Merge)
		if err != nil {
			return nil, err
		}
		if !included {
			return nil, fmt.Errorf("SDK PR #%d is not included in main", number)
		}
		if state.Ready != nil {
			included, err = g.Contains(ctx, state.Ready.SHA, pr.Merge)
			if err != nil {
				return nil, err
			}
			if included {
				fulfilled = append(fulfilled, number)
				continue
			}
		}
		needed = true
	}
	if len(fulfilled) > 0 {
		if err := g.UpdateState(ctx, func(current *State) error {
			current.Requests = slices.DeleteFunc(current.Requests, func(number int) bool { return slices.Contains(fulfilled, number) })
			return nil
		}); err != nil {
			return nil, err
		}
	}
	if !needed {
		return nil, nil
	}
	packageData, _, err := g.File(ctx, "web/sdk/package.json", source)
	if err != nil {
		return nil, err
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if err = json.Unmarshal(packageData, &pkg); err != nil {
		return nil, err
	}
	current := pkg.Version
	if current == "" {
		return nil, fmt.Errorf("canonical package has no version")
	}
	baseline := ""
	if state.Ready != nil {
		current, baseline = state.Ready.Version, state.Ready.Source
	} else {
		baseline, err = g.Ref(ctx, "v"+current)
		if err != nil {
			return nil, fmt.Errorf("bootstrap requires the current canonical version tag: %w", err)
		}
	}
	before, err := g.changes(ctx, baseline)
	if err != nil {
		return nil, err
	}
	after, err := g.changes(ctx, source)
	if err != nil {
		return nil, err
	}
	for path, sha := range before {
		if after[path] != sha {
			return nil, fmt.Errorf("published change declaration was removed or modified: %s", path)
		}
	}
	var changes []Change
	paths := make([]string, 0, len(after))
	for path := range after {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	for _, path := range paths {
		sha := after[path]
		if before[path] == sha {
			continue
		}
		if before[path] != "" {
			return nil, fmt.Errorf("published change declaration was modified: %s", path)
		}
		data, _, err := g.File(ctx, path, source)
		if err != nil {
			return nil, err
		}
		change, err := Decode[Change](data)
		if err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}
	version, err := NextVersion(current, changes)
	if err != nil {
		return nil, err
	}
	versionField := regexp.MustCompile(`("version"\s*:\s*)"[^"]+"`)
	if len(versionField.FindAllIndex(packageData, -1)) != 1 {
		return nil, fmt.Errorf("canonical package must contain exactly one version field")
	}
	packageData = versionField.ReplaceAll(packageData, []byte(`${1}"`+version+`"`))
	identity, _, err := g.File(ctx, "pkg/sdkidentity/identity.go", source)
	if err != nil {
		return nil, err
	}
	pattern := regexp.MustCompile(`ReleaseVersion\s*=\s*"[^"]+"`)
	if len(pattern.FindAll(identity, -1)) != 1 {
		return nil, fmt.Errorf("expected exactly one Go release identity")
	}
	identity = pattern.ReplaceAll(identity, []byte(`ReleaseVersion  = "`+version+`"`))
	clientHostIdentity, _, err := g.File(ctx, "web/client-host/src/identity.ts", source)
	if err != nil {
		return nil, err
	}
	clientHostPattern := regexp.MustCompile(`SDK_RELEASE_VERSION\s*=\s*'[^']+'`)
	if len(clientHostPattern.FindAll(clientHostIdentity, -1)) != 1 {
		return nil, fmt.Errorf("expected exactly one client-host release identity")
	}
	clientHostIdentity = clientHostPattern.ReplaceAll(clientHostIdentity, []byte(`SDK_RELEASE_VERSION = '`+version+`'`))
	metadata, _ := json.MarshalIndent(map[string]any{"version": version, "source": source, "changes": changes}, "", "  ")
	sha, err := g.CommitFiles(ctx, source, "chore: prepare SDK v"+version, map[string][]byte{
		"web/sdk/package.json":            append(packageData, '\n'),
		"pkg/sdkidentity/identity.go":     identity,
		"web/client-host/src/identity.ts": clientHostIdentity,
		".sdk/release.json":               append(metadata, '\n'),
	})
	if err != nil {
		return nil, err
	}
	// Source is part of the branch name: a failed, untagged candidate can be
	// superseded without moving a ref that another run might still be reading.
	branch := "sdk-candidates/v" + version + "-" + source[:12]
	if err = g.API(ctx, "POST", "git/refs", map[string]string{"ref": "refs/heads/" + branch, "sha": sha}, nil); err != nil {
		if !strings.Contains(err.Error(), "HTTP 422") {
			return nil, err
		}
		// Recover a candidate created before a crash saving state.
		sha, err = g.Ref(ctx, branch)
		if err != nil {
			return nil, err
		}
	}
	candidate := Candidate{Version: version, Source: source, SHA: sha, Phase: "testing", RunID: runID}
	return &candidate, g.saveCandidate(ctx, candidate, expectedCandidate)
}

func (g GitHub) saveCandidate(ctx context.Context, candidate Candidate, expected *Candidate) error {
	return g.UpdateState(ctx, func(state *State) error {
		if (state.Candidate == nil) != (expected == nil) || state.Candidate != nil && *state.Candidate != *expected {
			if state.Candidate != nil && state.Candidate.Phase == "publishing" {
				return fmt.Errorf("a partially published release must be resumed, never superseded")
			}
			return fmt.Errorf("release candidate changed while preparing; retry")
		}
		state.Candidate = &candidate
		return nil
	})
}

func (g GitHub) SetPhase(ctx context.Context, sha, phase string) error {
	if phase != "failed" && phase != "publishing" && phase != "ready" {
		return fmt.Errorf("invalid candidate phase")
	}
	return g.UpdateState(ctx, func(state *State) error {
		if state.Candidate == nil || state.Candidate.SHA != sha {
			return fmt.Errorf("candidate changed; refusing to update another SHA")
		}
		if state.Candidate.Phase == "publishing" && phase == "failed" {
			return fmt.Errorf("a partially published release must be resumed, never superseded")
		}
		state.Candidate.Phase = phase
		if phase == "ready" {
			if state.Releases == nil {
				state.Releases = map[string]*Candidate{}
			}
			state.Releases[state.Candidate.Version] = state.Candidate
			state.Ready, state.Candidate = state.Candidate, nil
		}
		return nil
	})
}
