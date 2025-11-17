package git

import "github.com/spf13/cobra"

// GitCmd represents the git command group
var GitCmd = &cobra.Command{
	Use:   "git",
	Short: "Git workflow utilities",
	Long:  `Git workflow utilities for the IOTA SDK project.`,
}

func init() {
	GitCmd.AddCommand(changesCmd)
	GitCmd.AddCommand(checkBranchCmd)
	GitCmd.AddCommand(precommitCmd)
}
