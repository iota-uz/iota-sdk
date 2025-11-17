package changelog

import (
	"fmt"

	"github.com/spf13/cobra"
)

var validateJSON bool

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate CHANGELOG.md format",
	Long:  `Validate CHANGELOG.md for format compliance.`,
	RunE:  runValidate,
}

func init() {
	validateCmd.Flags().BoolVar(&validateJSON, "json", false, "Output as JSON")
}

func runValidate(cmd *cobra.Command, args []string) error {
	// TODO: Implement validation
	fmt.Println("Not yet implemented")
	return nil
}
