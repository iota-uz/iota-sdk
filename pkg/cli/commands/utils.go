package commands

import (
	"github.com/spf13/cobra"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/pkg/cli/builders"
	"github.com/iota-uz/iota-sdk/pkg/commands"
)

// NewUtilityCommands creates all utility commands (check_tr_keys, seed)
func NewUtilityCommands() []*cobra.Command {
	return []*cobra.Command{
		newCheckTrKeysCmd(),
		newSeedCmd(),
	}
}

func newCheckTrKeysCmd() *cobra.Command {
	return builders.SimpleCommand(
		"check_tr_keys",
		"Check translation key consistency across all locales",
		`Validates that all translation keys are present across all configured locales and reports any missing translations.`,
		func() error {
			return commands.CheckTrKeys(modules.BuiltInModules...)
		},
		"check translation keys",
	)
}

func newSeedCmd() *cobra.Command {
	return builders.SimpleCommand(
		"seed",
		"Seed the main database with initial data",
		`Populates the main database with initial seed data including default tenant, users, permissions, and configuration.`,
		func() error {
			return commands.SeedDatabase(modules.BuiltInModules...)
		},
		"seed database",
	)
}
