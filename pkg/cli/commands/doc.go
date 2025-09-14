package commands

import (
	"github.com/spf13/cobra"

	"github.com/iota-uz/iota-sdk/pkg/cli/builders"
	"github.com/iota-uz/iota-sdk/pkg/cli/flags"
	"github.com/iota-uz/iota-sdk/pkg/commands"
)

// NewDocCommand creates the documentation generation command
func NewDocCommand() *cobra.Command {
	docFlags := flags.DefaultDocFlags()

	docCmd := builders.NewCommand(builders.CommandOptions{
		Use:   "doc",
		Short: "Generate project documentation",
		Long:  `Generates comprehensive documentation for the project by analyzing source code and creating markdown files.`,
		Example: `  # Generate docs for current directory
  command doc

  # Generate docs with specific options
  command doc --dir ./src --out API.md --recursive

  # Exclude specific directories
  command doc --exclude "vendor,node_modules,tmp"`,
		Run: func() error {
			opts := commands.DocumentGenerateOptions{
				SourceDir:   docFlags.SourceDir,
				OutputPath:  docFlags.OutputPath,
				Recursive:   docFlags.Recursive,
				ExcludeDirs: commands.ParseExcludeDirs(docFlags.ExcludeDirs),
			}
			return commands.GenerateDocumentation(opts)
		},
		Context: "generate documentation",
	})

	// Add flags to command
	docFlags.AddToCommand(docCmd)

	return docCmd
}
