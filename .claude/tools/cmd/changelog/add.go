package changelog

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	addTitle       string
	addDescription string
	addDryRun      bool
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new CHANGELOG entry",
	Long:  `Add a new entry to CHANGELOG.md with FIFO enforcement.`,
	RunE:  runAdd,
}

func init() {
	addCmd.Flags().StringVar(&addTitle, "title", "", "Entry title")
	addCmd.Flags().StringVar(&addDescription, "description", "", "Entry description")
	addCmd.Flags().BoolVar(&addDryRun, "dry-run", false, "Show what would be added")
}

func runAdd(cmd *cobra.Command, args []string) error {
	// TODO: Implement add entry functionality
	fmt.Println("Not yet implemented")
	return nil
}
