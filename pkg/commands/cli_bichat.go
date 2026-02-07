package commands

import "github.com/spf13/cobra"

// NewBiChatCommand creates the bichat command group.
func NewBiChatCommand() *cobra.Command {
	bichatCmd := &cobra.Command{
		Use:   "bichat",
		Short: "BiChat utilities",
		Long:  "Operational utilities for BiChat development and quality validation.",
	}

	bichatCmd.AddCommand(NewBiChatEvalCommand())

	return bichatCmd
}
