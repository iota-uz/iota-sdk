package commands

import (
	"github.com/spf13/cobra"

	"github.com/iota-uz/iota-sdk/modules"
)

// NewUtilityCommands creates all utility commands (check_tr_keys, seed, seed_superadmin)
func NewUtilityCommands() []*cobra.Command {
	return []*cobra.Command{
		newCheckTrKeysCmd(),
		newSeedCmd(),
		newSeedSuperadminCmd(),
	}
}

func newCheckTrKeysCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check_tr_keys",
		Short: "Check translation key consistency across all locales",
		Long:  `Validates that all translation keys are present across all configured locales and reports any missing translations.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return CheckTrKeys(modules.BuiltInModules...)
		},
	}
}

func newSeedCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "seed",
		Short: "Seed the main database with initial data",
		Long:  `Populates the main database with initial seed data including default tenant, users, permissions, and configuration.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return SeedDatabase(modules.BuiltInModules...)
		},
	}
}

func newSeedSuperadminCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "seed_superadmin",
		Short: "Seed the database with a superadmin user",
		Long:  `Creates a superadmin user with full permissions for accessing the Super Admin dashboard.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return SeedSuperadmin(modules.BuiltInModules...)
		},
	}
}
