package modulepaths_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/mod/module"
)

// TestTrackedFilesArePublishable checks each tracked file with the rule the module proxy applies
// when it builds the zip of a version (module.CheckFilePath): v0.55.0 shipped pkg/automation/catalog/nul.go,
// and its Go module could not be downloaded.
func TestTrackedFilesArePublishable(t *testing.T) {
	root, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skipf("not in a git checkout: %v", err)
	}
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = strings.TrimSpace(string(root))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	var bad []string
	for _, p := range strings.Split(strings.TrimRight(string(out), "\x00"), "\x00") {
		if p == "" {
			continue
		}
		if err := module.CheckFilePath(filepath.ToSlash(p)); err != nil {
			bad = append(bad, err.Error())
		}
	}
	if len(bad) > 0 {
		t.Fatalf("tracked files the Go module proxy refuses:\n%s", strings.Join(bad, "\n"))
	}
}
