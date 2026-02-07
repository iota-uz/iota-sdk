package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/iota-uz/iota-sdk/pkg/cli/exitcode"
	"github.com/iota-uz/iota-sdk/pkg/commands"
)

// NewRootCommand creates the root command with all subcommands
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "command",
		Short:         "Command Line Tool for IOTA SDK",
		Long:          `A unified command-line interface for IOTA SDK operations including database management, testing, and utility functions.`,
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	// Add all command groups
	rootCmd.AddCommand(commands.NewUtilityCommands()...)
	rootCmd.AddCommand(commands.NewE2ECommand())
	rootCmd.AddCommand(commands.NewMigrateCommand())
	rootCmd.AddCommand(commands.NewKnowledgeCommand())
	rootCmd.AddCommand(commands.NewBiChatCommand())

	return rootCmd
}

// Execute runs the root command
func Execute() {
	if err := NewRootCommand().Execute(); err != nil {
		var ee *exitcode.Error
		if errors.As(err, &ee) {
			if !ee.Silent {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
			os.Exit(ee.ExitCode())
		}

		var ec interface{ ExitCode() int }
		if errors.As(err, &ec) {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(ec.ExitCode())
		}

		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
