package git

import (
	"context"
	"fmt"
	"os"

	"github.com/iota-uz/iota-sdk/sdk-tools/internal/git"
	"github.com/iota-uz/iota-sdk/sdk-tools/internal/output"
	"github.com/spf13/cobra"
)

var (
	checkBranchJSONFlag   bool
	checkBranchStrictFlag bool
)

var checkBranchCmd = &cobra.Command{
	Use:   "check-branch",
	Short: "Check if current branch is protected",
	Long:  `Check if current branch is a protected branch (main only).`,
	RunE:  runCheckBranch,
}

func init() {
	checkBranchCmd.Flags().BoolVar(&checkBranchJSONFlag, "json", false, "Output as JSON")
	checkBranchCmd.Flags().BoolVar(&checkBranchStrictFlag, "strict", false, "Exit with code 1 if on protected branch")
}

func runCheckBranch(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	formatter := output.New(os.Stdout, checkBranchJSONFlag)

	branch, err := git.GetCurrentBranch(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	isProtected := git.IsProtectedBranch(branch)

	if formatter.IsJSON() {
		result := map[string]interface{}{
			"branch":      branch,
			"protected":   isProtected,
			"branch_type": "feature",
		}

		if isProtected {
			result["branch_type"] = "protected"
		}

		if err := formatter.PrintJSON(result); err != nil {
			return err
		}

		if isProtected && checkBranchStrictFlag {
			os.Exit(1)
		}

		return nil
	}

	var message string
	if isProtected {
		message = output.Yellow(fmt.Sprintf("WARNING: On protected branch: %s", branch))
	} else {
		message = output.Green(fmt.Sprintf("OK: On feature branch: %s", branch))
	}

	if err := formatter.PrintTextLn(message); err != nil {
		return err
	}

	if isProtected && checkBranchStrictFlag {
		os.Exit(1)
	}

	return nil
}
