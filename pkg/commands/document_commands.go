package commands

import (
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/document"
)

// DocumentGenerateOptions holds the configuration for document generation
type DocumentGenerateOptions struct {
	SourceDir   string
	OutputPath  string
	Recursive   bool
	ExcludeDirs []string
}

// GenerateDocumentation generates documentation based on the provided options
func GenerateDocumentation(opts DocumentGenerateOptions) error {
	config := document.Config{
		SourceDir:   opts.SourceDir,
		OutputPath:  opts.OutputPath,
		Recursive:   opts.Recursive,
		ExcludeDirs: opts.ExcludeDirs,
	}

	if err := document.Generate(config); err != nil {
		return fmt.Errorf("failed to generate documentation: %w", err)
	}

	conf := configuration.Use()
	conf.Logger().Info("Documentation generated successfully", "output", opts.OutputPath)
	return nil
}

// ParseExcludeDirs parses a comma-separated string into a slice of directory names
func ParseExcludeDirs(excludeStr string) []string {
	if excludeStr == "" {
		return nil
	}
	return strings.Split(excludeStr, ",")
}
