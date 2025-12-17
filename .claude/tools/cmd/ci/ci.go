package ci

import "github.com/spf13/cobra"

var CICmd = &cobra.Command{
	Use:   "ci",
	Short: "CI/CD utilities",
	Long:  `CI/CD utilities for workflow analysis and management.`,
}

func init() {
	CICmd.AddCommand(failuresCmd)
}
