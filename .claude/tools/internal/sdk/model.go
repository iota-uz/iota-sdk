package sdk

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const Repository = "iota-uz/iota-sdk"
const StateBranch = "sdk-release-state"

var versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
var shaPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

type Change struct {
	Bump    string `json:"bump"`
	Summary string `json:"summary"`
}

func (c Change) Validate() error {
	if c.Bump != "none" && c.Bump != "patch" && c.Bump != "minor" && c.Bump != "major" {
		return fmt.Errorf("bump must be none, patch, minor or major")
	}
	if strings.TrimSpace(c.Summary) == "" {
		return fmt.Errorf("a change summary (or reason for none) is required")
	}
	return nil
}

func NextVersion(current string, changes []Change) (string, error) {
	if !versionPattern.MatchString(current) {
		return "", fmt.Errorf("invalid stable version %q", current)
	}
	parts := strings.Split(current, ".")
	numbers := make([]int, 3)
	for i := range parts {
		value, err := strconv.Atoi(parts[i])
		if err != nil || value > 1000000 {
			return "", fmt.Errorf("version component out of range")
		}
		numbers[i] = value
	}
	level := 0
	for _, change := range changes {
		if err := change.Validate(); err != nil {
			return "", err
		}
		level = max(level, map[string]int{"none": 0, "patch": 1, "minor": 2, "major": 3}[change.Bump])
	}
	if level == 0 {
		return "", fmt.Errorf("no releasable changes; add a .changes/*.json declaration")
	}
	// During v0, breaking changes advance the minor version. Leaving v0 is
	// a separate API stability decision, never an incidental automated bump.
	if level == 3 && numbers[0] == 0 {
		level = 2
	}
	if level == 3 {
		return "", fmt.Errorf("a new Go module major requires a reviewed module-path migration")
	}
	index := 3 - level
	numbers[index]++
	for i := index + 1; i < 3; i++ {
		numbers[i] = 0
	}
	return fmt.Sprintf("%d.%d.%d", numbers[0], numbers[1], numbers[2]), nil
}

type Candidate struct {
	Version string `json:"version"`
	Source  string `json:"source"`
	SHA     string `json:"sha"`
	Phase   string `json:"phase"`
	RunID   string `json:"run_id"`
}

type State struct {
	Requests  []int      `json:"requests"`
	Candidate *Candidate `json:"candidate,omitempty"`
	Ready     *Candidate `json:"ready,omitempty"`
}

type Dependency struct {
	PR      int    `json:"pr"`
	Channel string `json:"channel"`
	GoDir   string `json:"go_dir"`
	WebDir  string `json:"web_dir,omitempty"`
	Version string `json:"version,omitempty"`
	SHA     string `json:"sha,omitempty"`
}

func (d Dependency) Validate() error {
	if d.PR <= 0 || (d.Channel != "preview" && d.Channel != "release") {
		return fmt.Errorf("dependency requires a positive SDK PR and preview/release channel")
	}
	for _, dir := range []string{d.GoDir, d.WebDir} {
		if dir == "" {
			continue
		}
		if filepath.IsAbs(dir) || filepath.Clean(dir) != dir || dir == ".." || strings.HasPrefix(dir, "../") || strings.ContainsAny(dir, "\\\n\r") {
			return fmt.Errorf("dependency directory must be a clean relative path: %q", dir)
		}
	}
	if d.GoDir == "" {
		return fmt.Errorf("go_dir is required")
	}
	return nil
}

func Decode[T any](data []byte) (T, error) {
	var value T
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&value)
	if err == nil && decoder.Decode(new(any)) != io.EOF {
		err = fmt.Errorf("unexpected trailing JSON")
	}
	return value, err
}
