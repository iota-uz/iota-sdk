package ci

import (
	"context"
	"fmt"
	"os"

	"github.com/iota-uz/iota-sdk/sdk-tools/internal/git"
	"github.com/iota-uz/iota-sdk/sdk-tools/internal/github"
	"github.com/iota-uz/iota-sdk/sdk-tools/internal/output"
	"github.com/spf13/cobra"
)

var (
	runID      int
	jsonOutput bool
)

var failuresCmd = &cobra.Command{
	Use:   "failures",
	Short: "Analyze CI failures",
	Long:  `Analyze CI failures for the current branch.`,
	RunE:  runFailures,
}

type failuresOutput struct {
	Branch      string               `json:"branch"`
	RecentRuns  []github.WorkflowRun `json:"recent_runs,omitempty"`
	LatestRunID int                  `json:"latest_run_id,omitempty"`
	FailedJobs  []string             `json:"failed_jobs,omitempty"`
	ErrorLogs   []string             `json:"error_logs,omitempty"`
	NoFailures  bool                 `json:"no_failures,omitempty"`
}

func init() {
	failuresCmd.Flags().IntVar(&runID, "run-id", 0, "Specific run ID to analyze")
	failuresCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
}

func runFailures(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	formatter := output.New(os.Stdout, jsonOutput)
	ghClient := github.New()

	if !ghClient.IsInstalled(ctx) {
		return fmt.Errorf("GitHub CLI (gh) is not installed")
	}

	branch, err := git.GetCurrentBranch(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	result := failuresOutput{
		Branch: branch,
	}

	targetRunID := runID
	if targetRunID == 0 {
		targetRunID, err = ghClient.GetLatestFailedRunID(ctx, branch)
		if err != nil {
			return fmt.Errorf("failed to get latest failed run: %w", err)
		}

		if targetRunID == 0 {
			result.NoFailures = true
			if formatter.IsJSON() {
				return formatter.PrintJSON(result)
			}
			formatter.PrintTextLn(output.Green(fmt.Sprintf("✓ No failed runs found for branch: %s", branch)))
			return nil
		}

		recentRuns, err := ghClient.ListFailedRuns(ctx, branch, 5)
		if err != nil {
			return fmt.Errorf("failed to list recent runs: %w", err)
		}
		result.RecentRuns = recentRuns
	}

	result.LatestRunID = targetRunID

	failedJobs, err := ghClient.GetFailedJobs(ctx, targetRunID)
	if err != nil {
		return fmt.Errorf("failed to get failed jobs: %w", err)
	}
	result.FailedJobs = failedJobs

	errorLogs, err := ghClient.GetFailedLogs(ctx, targetRunID, 60)
	if err != nil {
		return fmt.Errorf("failed to get error logs: %w", err)
	}
	result.ErrorLogs = errorLogs

	if formatter.IsJSON() {
		return formatter.PrintJSON(result)
	}

	printTextOutput(formatter, result)

	return nil
}

func printTextOutput(formatter *output.Formatter, result failuresOutput) {
	formatter.PrintTextLn(output.Bold(fmt.Sprintf("Branch: %s", result.Branch)))
	formatter.PrintTextLn("")

	if len(result.RecentRuns) > 0 {
		formatter.PrintTextLn(output.Bold("=== Recent Failed Runs ==="))
		for _, run := range result.RecentRuns {
			formatter.PrintTextLn(fmt.Sprintf("  [%d] %s - %s",
				run.DatabaseID,
				run.WorkflowName,
				run.DisplayTitle,
			))
		}
		formatter.PrintTextLn("")
	}

	formatter.PrintTextLn(output.Bold(fmt.Sprintf("=== Latest Failed Run ID: %d ===", result.LatestRunID)))
	formatter.PrintTextLn("")

	if len(result.FailedJobs) > 0 {
		formatter.PrintTextLn(output.Bold("=== Failed Jobs ==="))
		for _, job := range result.FailedJobs {
			formatter.PrintTextLn(fmt.Sprintf("  %s", output.Red(job)))
		}
		formatter.PrintTextLn("")
	}

	if len(result.ErrorLogs) > 0 {
		formatter.PrintTextLn(output.Bold("=== Error Logs ==="))
		for _, log := range result.ErrorLogs {
			formatter.PrintTextLn(log)
		}
	}
}
