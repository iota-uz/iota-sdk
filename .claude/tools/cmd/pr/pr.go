package pr

import "github.com/spf13/cobra"

var PRCmd = &cobra.Command{
	Use:   "pr",
	Short: "Pull request utilities",
	Long:  `Pull request utilities for analysis and management.`,
}

func init() {
	PRCmd.AddCommand(contextCmd)
}
