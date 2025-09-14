package flags

import "github.com/spf13/cobra"

// DocFlags holds flags for the documentation command
type DocFlags struct {
	SourceDir   string
	OutputPath  string
	Recursive   bool
	ExcludeDirs string
}

// DefaultDocFlags returns default values for documentation flags
func DefaultDocFlags() DocFlags {
	return DocFlags{
		SourceDir:   "./",
		OutputPath:  "DOCUMENTATION.md",
		Recursive:   false,
		ExcludeDirs: "",
	}
}

// AddToCommand adds doc flags to a command
func (f *DocFlags) AddToCommand(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&f.SourceDir, "dir", "d", f.SourceDir, "Source directory to document")
	cmd.Flags().StringVarP(&f.OutputPath, "out", "o", f.OutputPath, "Output file path")
	cmd.Flags().BoolVarP(&f.Recursive, "recursive", "r", f.Recursive, "Process directories recursively")
	cmd.Flags().StringVarP(&f.ExcludeDirs, "exclude", "e", f.ExcludeDirs, "Comma-separated list of directories to exclude")
}

// MigrateFlags holds common flags for migrate commands (future extensibility)
type MigrateFlags struct {
	// Reserved for future migration-specific flags
}

// E2EFlags holds common flags for e2e commands (future extensibility)
type E2EFlags struct {
	// Reserved for future e2e-specific flags
}
