package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// WorkflowRun represents a GitHub workflow run
type WorkflowRun struct {
	DatabaseID   int       `json:"databaseId"`
	DisplayTitle string    `json:"displayTitle"`
	WorkflowName string    `json:"workflowName"`
	CreatedAt    time.Time `json:"createdAt"`
	URL          string    `json:"url"`
}

// Client is a GitHub CLI wrapper
type Client struct{}

// New creates a new GitHub client
func New() *Client {
	return &Client{}
}

// IsInstalled checks if the gh CLI is installed
func (c *Client) IsInstalled(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "gh", "--version")
	return cmd.Run() == nil
}

// GetLatestFailedRunID gets the ID of the latest failed run for a branch
func (c *Client) GetLatestFailedRunID(ctx context.Context, branch string) (int, error) {
	cmd := exec.CommandContext(ctx, "gh", "run", "list",
		"--branch", branch,
		"--status", "failure",
		"-L", "1",
		"--json", "databaseId",
		"--jq", ".[0].databaseId")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("failed to get latest failed run: %w", err)
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" || output == "null" {
		return 0, nil
	}

	var runID int
	if err := json.Unmarshal([]byte(output), &runID); err != nil {
		return 0, fmt.Errorf("failed to parse run ID: %w", err)
	}

	return runID, nil
}

// ListFailedRuns lists failed runs for a branch
func (c *Client) ListFailedRuns(ctx context.Context, branch string, limit int) ([]WorkflowRun, error) {
	cmd := exec.CommandContext(ctx, "gh", "run", "list",
		"--branch", branch,
		"--status", "failure",
		"-L", fmt.Sprintf("%d", limit),
		"--json", "databaseId,displayTitle,workflowName,createdAt,url")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to list failed runs: %w", err)
	}

	var runs []WorkflowRun
	if err := json.Unmarshal(stdout.Bytes(), &runs); err != nil {
		return nil, fmt.Errorf("failed to parse runs: %w", err)
	}

	return runs, nil
}

// GetFailedJobs gets failed jobs from a run
func (c *Client) GetFailedJobs(ctx context.Context, runID int) ([]string, error) {
	cmd := exec.CommandContext(ctx, "gh", "run", "view",
		fmt.Sprintf("%d", runID),
		"--json", "jobs",
		"--jq", `.jobs[] | select(.conclusion == "failure") | .name`)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to get failed jobs: %w", err)
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return []string{}, nil
	}

	return strings.Split(output, "\n"), nil
}

// GetFailedLogs gets error logs from a failed run
func (c *Client) GetFailedLogs(ctx context.Context, runID int, limit int) ([]string, error) {
	cmd := exec.CommandContext(ctx, "bash", "-c",
		fmt.Sprintf("gh run view %d --log-failed 2>&1 | grep -iE '(error|fail|panic|assertion|expected|fatal|--- fail)' | tail -%d", runID, limit))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil && err.Error() != "exit status 1" {
		return nil, fmt.Errorf("failed to get error logs: %w", err)
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return []string{}, nil
	}

	return strings.Split(output, "\n"), nil
}
