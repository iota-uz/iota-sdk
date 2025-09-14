package commands

import (
	"github.com/spf13/cobra"
)

// NewE2ECommand creates the e2e command group with all subcommands
func NewE2ECommand() *cobra.Command {
	e2eCmd := &cobra.Command{
		Use:   "e2e",
		Short: "E2E database management",
		Long:  `Manage end-to-end testing database including creation, seeding, migrations, and cleanup operations.`,
		Example: `  # Set up complete e2e environment
  command e2e setup

  # Reset database with fresh data
  command e2e reset

  # Create empty database
  command e2e create`,
	}

	// Add all e2e subcommands
	e2eCmd.AddCommand(newE2ESetupCmd())
	e2eCmd.AddCommand(newE2EResetCmd())
	e2eCmd.AddCommand(newE2ECreateCmd())
	e2eCmd.AddCommand(newE2EDropCmd())
	e2eCmd.AddCommand(newE2EMigrateCmd())
	e2eCmd.AddCommand(newE2ESeedCmd())
	e2eCmd.AddCommand(newE2ETestCmd())

	return e2eCmd
}

func newE2ESetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Create database, run migrations, and seed test data",
		Long:  `Performs a complete e2e database setup by creating the database, applying all migrations, and seeding with test data.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return E2ESetup()
		},
	}
}

func newE2EResetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reset",
		Short: "Drop and recreate database with fresh data",
		Long:  `Completely resets the e2e database by dropping it, recreating it, running migrations, and seeding fresh test data.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return E2EReset()
		},
	}
}

func newE2ECreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "Create empty e2e database",
		Long:  `Creates an empty e2e database without running migrations or seeding data.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return E2ECreate()
		},
	}
}

func newE2EDropCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "drop",
		Short: "Drop e2e database",
		Long:  `Completely removes the e2e database and all its data.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return E2EDrop()
		},
	}
}

func newE2EMigrateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Run migrations on existing e2e database",
		Long:  `Applies all pending migrations to the existing e2e database.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return E2EMigrate()
		},
	}
}

func newE2ESeedCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "seed",
		Short: "Seed existing e2e database with test data",
		Long:  `Populates the existing e2e database with test data required for end-to-end testing.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return E2ESeed()
		},
	}
}

func newE2ETestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test",
		Short: "Run e2e tests with proper server lifecycle management",
		Long:  `Sets up the e2e environment, starts the server, runs Cypress tests, and ensures proper cleanup.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return E2ETest()
		},
	}
}
