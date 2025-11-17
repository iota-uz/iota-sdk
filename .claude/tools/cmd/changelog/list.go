package changelog

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	listCount int
	listJSON  bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List CHANGELOG entries",
	Long:  `List the most recent CHANGELOG.md entries.`,
	RunE:  runList,
}

func init() {
	listCmd.Flags().IntVar(&listCount, "count", 10, "Number of entries to show")
	listCmd.Flags().BoolVar(&listJSON, "json", false, "Output as JSON")
}

func runList(cmd *cobra.Command, args []string) error {
	// TODO: Implement list functionality
	fmt.Println("Not yet implemented")
	return nil
}
