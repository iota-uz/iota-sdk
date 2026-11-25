package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// AbandonedPhase marks a partially published version that can never become ready: its tag (and npm
// version) exist, but the Go module proxy cannot serve the module. The record stays in releases, the
// tag and the npm version are never rewritten, and Prepare never issues the version again.
const AbandonedPhase = "abandoned"

// goModuleProxy is where the proof of unavailability is taken: the public proxy and checksum database
// every consumer resolves the module through.
const goModuleProxy = "https://proxy.golang.org"

// permanentModuleFailures are the go command's answers for a module version that will never download:
// the proxy cannot build its zip, or its content disagrees with the checksum database. A network or
// proxy outage answers otherwise and proves nothing.
var permanentModuleFailures = []string{"create zip", "malformed file path", "checksum mismatch", "SECURITY ERROR", "invalid version"}

// Abandon retires the partially published candidate at sha when its Go module provably cannot be
// downloaded (go mod download through the public proxy fails permanently) and returns it with the
// proof. Only a publishing candidate is abandoned: a testing or failed one is superseded by the next
// release anyway. The candidate moves to releases with the abandoned phase, the candidate slot is
// cleared, and ready keeps pointing at the last ready release. The next release takes a version above
// every abandoned one. The state gains no new field: sdkctl builds that predate this decode it strictly.
func (g GitHub) Abandon(ctx context.Context, sha string) (Candidate, string, error) {
	state, _, err := g.ReadState(ctx)
	if err != nil {
		return Candidate{}, "", err
	}
	if err := abandonable(state, sha); err != nil {
		return Candidate{}, "", err
	}
	proof, err := ProveGoModuleUnavailable(ctx, g.Runner, state.Candidate.Version)
	if err != nil {
		return Candidate{}, "", err
	}
	var abandoned Candidate
	err = g.UpdateState(ctx, func(state *State) error {
		if err := abandonable(*state, sha); err != nil {
			return err
		}
		abandoned = *state.Candidate
		abandoned.Phase = AbandonedPhase
		if state.Releases == nil {
			state.Releases = map[string]*Candidate{}
		}
		state.Releases[abandoned.Version] = &abandoned
		state.Candidate = nil
		return nil
	})
	return abandoned, proof, err
}

func abandonable(state State, sha string) error {
	if state.Candidate == nil || state.Candidate.SHA != sha {
		return fmt.Errorf("candidate changed; refusing to abandon another SHA")
	}
	if state.Candidate.Phase != "publishing" {
		return fmt.Errorf("only a partially published candidate is abandoned; a %s candidate is superseded by the next release", state.Candidate.Phase)
	}
	return nil
}

// ProveGoModuleUnavailable downloads the module version through the public proxy into an empty module
// cache and returns the go command's error when it fails permanently. A download that succeeds, or one
// that fails for another reason (network, proxy outage), is an error: nothing is proven.
func ProveGoModuleUnavailable(ctx context.Context, runner Runner, version string) (string, error) {
	if !versionPattern.MatchString(version) {
		return "", fmt.Errorf("invalid stable version %q", version)
	}
	dir, err := os.MkdirTemp("", "sdk-abandon-")
	if err != nil {
		return "", err
	}
	defer removeModuleCache(dir)
	module := GoModulePath + "@v" + version
	// go mod download -json reports a module's failure in its JSON (and exits 1); the shell keeps the
	// exit status from discarding that answer, so only a go command that could not run is an error.
	out, err := runner.Run(ctx, dir, nil, "env", "GOWORK=off", "GOFLAGS=", "GOPROXY="+goModuleProxy, "GONOPROXY=", "GOPRIVATE=", "GONOSUMDB=", "GOINSECURE=", "GOSUMDB=sum.golang.org", "GOMODCACHE="+filepath.Join(dir, "cache"),
		"sh", "-c", `go mod download -json "$1" || true`, "sh", module)
	if err != nil {
		return "", err
	}
	var answer struct {
		Error string `json:"Error"`
	}
	if err := json.Unmarshal(out, &answer); err != nil {
		return "", fmt.Errorf("go mod download %s gave no JSON answer: %w", module, err)
	}
	if answer.Error == "" {
		return "", fmt.Errorf("%s downloads through %s; resume its publication instead of abandoning it", module, goModuleProxy)
	}
	for _, failure := range permanentModuleFailures {
		if strings.Contains(answer.Error, failure) {
			return "go mod download " + module + ": " + answer.Error, nil
		}
	}
	return "", fmt.Errorf("could not prove that %s is permanently unavailable: %s", module, answer.Error)
}

// removeModuleCache deletes a temporary module cache, whose files the go command makes read-only.
func removeModuleCache(dir string) {
	_ = filepath.WalkDir(dir, func(path string, _ fs.DirEntry, err error) error {
		if err == nil {
			_ = os.Chmod(path, 0o755)
		}
		return nil
	})
	_ = os.RemoveAll(dir)
}

// pastAbandoned is version, or the patch after the highest abandoned version when version would not be
// above it: an abandoned version's tag exists, so it is never issued again, and the release that
// replaces it carries at least its changes.
func pastAbandoned(version string, releases map[string]*Candidate) string {
	highest := ""
	for v, release := range releases {
		if release != nil && release.Phase == AbandonedPhase && (highest == "" || versionLess(highest, v)) {
			highest = v
		}
	}
	if highest == "" || versionLess(highest, version) {
		return version
	}
	parts := versionParts(highest)
	return fmt.Sprintf("%d.%d.%d", parts[0], parts[1], parts[2]+1)
}

func versionLess(a, b string) bool {
	x, y := versionParts(a), versionParts(b)
	for i := range x {
		if x[i] != y[i] {
			return x[i] < y[i]
		}
	}
	return false
}

func versionParts(version string) [3]int {
	var parts [3]int
	for i, part := range strings.SplitN(version, ".", 3) {
		parts[i], _ = strconv.Atoi(part)
	}
	return parts
}
