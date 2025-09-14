package builders

import (
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/spf13/cobra"
)

// CommandOptions holds options for building a command
type CommandOptions struct {
	Use     string
	Short   string
	Long    string
	Example string
	RunE    func(cmd *cobra.Command, args []string) error
	Run     func() error
	Context string // Context for error messages
}

// NewCommand creates a new cobra command with consistent patterns
func NewCommand(opts CommandOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   opts.Use,
		Short: opts.Short,
		Long:  opts.Long,
	}

	if opts.Example != "" {
		cmd.Example = opts.Example
	}

	// Handle both RunE and Run patterns
	if opts.RunE != nil {
		cmd.RunE = WrapRunE(opts.RunE, opts.Context)
	} else if opts.Run != nil {
		cmd.Run = WrapRun(opts.Run, opts.Context)
	}

	return cmd
}

// SubCommandOptions holds options for building a subcommand group
type SubCommandOptions struct {
	Use     string
	Short   string
	Long    string
	Example string
}

// NewSubCommand creates a new subcommand group
func NewSubCommand(opts SubCommandOptions) *cobra.Command {
	return &cobra.Command{
		Use:     opts.Use,
		Short:   opts.Short,
		Long:    opts.Long,
		Example: opts.Example,
	}
}

// SimpleCommand creates a simple command that just runs a function
func SimpleCommand(use, short, long string, fn func() error, context string) *cobra.Command {
	return NewCommand(CommandOptions{
		Use:     use,
		Short:   short,
		Long:    long,
		Run:     fn,
		Context: context,
	})
}

// MessageCommand creates a command that displays a message before running
func MessageCommand(use, short, long, message string, fn func() error, context string) *cobra.Command {
	return NewCommand(CommandOptions{
		Use:   use,
		Short: short,
		Long:  long,
		Run: func() error {
			if message != "" {
				conf := configuration.Use()
				conf.Logger().Info(message)
			}
			return fn()
		},
		Context: context,
	})
}
