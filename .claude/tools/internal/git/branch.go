package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// GetCurrentBranch returns the name of the current git branch
func GetCurrentBranch(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "branch", "--show-current")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git branch --show-current failed: %w (stderr: %s)", err, stderr.String())
	}

	branch := strings.TrimSpace(stdout.String())
	return branch, nil
}

// IsProtectedBranch checks if the given branch name is a protected branch (main or staging)
func IsProtectedBranch(branch string) bool {
	return branch == "main" || branch == "staging"
}

// GetMergeBase returns the merge base between the current HEAD and a target branch
func GetMergeBase(ctx context.Context, target string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "merge-base", "HEAD", target)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git merge-base failed: %w (stderr: %s)", err, stderr.String())
	}

	base := strings.TrimSpace(stdout.String())
	return base, nil
}
