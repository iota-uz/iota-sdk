package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/iota-uz/iota-sdk/pkg/cli/commands"
)

// NewRootCommand creates the root command with all subcommands
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "command",
		Short: "Command Line Tool for IOTA SDK",
		Long:  `A unified command-line interface for IOTA SDK operations including database management, testing, and utility functions.`,
	}

	// Add all command groups
	rootCmd.AddCommand(commands.NewUtilityCommands()...)
	rootCmd.AddCommand(commands.NewDocCommand())
	rootCmd.AddCommand(commands.NewE2ECommand())
	rootCmd.AddCommand(commands.NewMigrateCommand())

	return rootCmd
}

// Execute runs the root command
func Execute() {
	if err := NewRootCommand().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
