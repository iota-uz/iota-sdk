package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// GetChangedFiles returns all changed files (staged and unstaged)
func GetChangedFiles(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--name-only", "HEAD")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git diff --name-only HEAD failed: %w (stderr: %s)", err, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return []string{}, nil
	}

	return strings.Split(output, "\n"), nil
}

// GetChangedFilesSince returns files changed since a specific commit/ref
func GetChangedFilesSince(ctx context.Context, since string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--name-only", since, "HEAD")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git diff --name-only %s HEAD failed: %w (stderr: %s)", since, err, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return []string{}, nil
	}

	return strings.Split(output, "\n"), nil
}
