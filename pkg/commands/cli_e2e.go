package commands

import (
	"github.com/iota-uz/iota-sdk/pkg/commands/e2e"
	"github.com/spf13/cobra"
)

// NewE2ECommand creates the e2e command group with all subcommands
func NewE2ECommand() *cobra.Command {
	e2eCmd := &cobra.Command{
		Use:   "e2e",
		Short: "E2E database management",
		Long:  `Manage end-to-end testing database including creation, seeding, migrations, and cleanup operations.`,
		Example: `  # Run e2e tests with database setup
  command e2e test

  # Reset database with fresh data
  command e2e reset

  # Create empty database
  command e2e create`,
	}

	// Add all e2e subcommands
	e2eCmd.AddCommand(newE2EResetCmd())
	e2eCmd.AddCommand(newE2ECreateCmd())
	e2eCmd.AddCommand(newE2EDropCmd())
	e2eCmd.AddCommand(newE2EMigrateCmd())
	e2eCmd.AddCommand(newE2ESeedCmd())
	e2eCmd.AddCommand(newE2ETestCmd())

	return e2eCmd
}

func newE2EResetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reset",
		Short: "Drop and recreate database with fresh data",
		Long:  `Completely resets the e2e database by dropping it, recreating it, running migrations, and seeding fresh test data.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return e2e.Reset()
		},
	}
}

func newE2ECreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "Create empty e2e database",
		Long:  `Creates an empty e2e database without running migrations or seeding data.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return e2e.Create()
		},
	}
}

func newE2EDropCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "drop",
		Short: "Drop e2e database",
		Long:  `Completely removes the e2e database and all its data.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return e2e.Drop()
		},
	}
}

func newE2EMigrateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Run migrations on existing e2e database",
		Long:  `Applies all pending migrations to the existing e2e database.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return e2e.Migrate()
		},
	}
}

func newE2ESeedCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "seed",
		Short: "Seed existing e2e database with test data",
		Long:  `Populates the existing e2e database with test data required for end-to-end testing.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return e2e.Seed()
		},
	}
}

func newE2ETestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test",
		Short: "Set up database and run e2e tests",
		Long:  `Sets up the e2e database with migrations and seed data, then runs Cypress tests against a running e2e development server.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return e2e.Test()
		},
	}
}
