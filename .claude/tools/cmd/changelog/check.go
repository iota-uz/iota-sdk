package changelog

import (
	"context"
	"fmt"
	"os"

	"github.com/iota-uz/iota-sdk/sdk-tools/internal/changelog"
	"github.com/iota-uz/iota-sdk/sdk-tools/internal/output"
	"github.com/spf13/cobra"
)

var (
	checkJSON    bool
	checkExplain bool
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check if CHANGELOG update is required",
	Long:  `Analyze changed files to determine if CHANGELOG.md update is required.`,
	RunE:  runCheck,
}

func init() {
	checkCmd.Flags().BoolVar(&checkJSON, "json", false, "Output as JSON")
	checkCmd.Flags().BoolVar(&checkExplain, "explain", false, "Show detailed reasoning")
}

func runCheck(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	result, err := changelog.AnalyzeChanges(ctx)
	if err != nil {
		return fmt.Errorf("failed to analyze changes: %w", err)
	}

	formatter := output.New(os.Stdout, checkJSON)

	if checkJSON {
		return formatter.PrintJSON(result)
	}

	formatted := changelog.FormatCheckResult(result, checkExplain)
	return formatter.PrintTextLn(formatted)
}
