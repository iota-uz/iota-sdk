package commands

import (
	"github.com/spf13/cobra"

	"github.com/iota-uz/iota-sdk/pkg/cli/builders"
	"github.com/iota-uz/iota-sdk/pkg/commands"
)

// NewE2ECommand creates the e2e command group with all subcommands
func NewE2ECommand() *cobra.Command {
	e2eCmd := builders.NewSubCommand(builders.SubCommandOptions{
		Use:   "e2e",
		Short: "E2E database management",
		Long:  `Manage end-to-end testing database including creation, seeding, migrations, and cleanup operations.`,
		Example: `  # Set up complete e2e environment
  command e2e setup

  # Reset database with fresh data
  command e2e reset

  # Create empty database
  command e2e create`,
	})

	// Add all e2e subcommands
	e2eCmd.AddCommand(newE2ESetupCmd())
	e2eCmd.AddCommand(newE2EResetCmd())
	e2eCmd.AddCommand(newE2ECreateCmd())
	e2eCmd.AddCommand(newE2EDropCmd())
	e2eCmd.AddCommand(newE2EMigrateCmd())
	e2eCmd.AddCommand(newE2ESeedCmd())

	return e2eCmd
}

func newE2ESetupCmd() *cobra.Command {
	return builders.SimpleCommand(
		"setup",
		"Create database, run migrations, and seed test data",
		`Performs a complete e2e database setup by creating the database, applying all migrations, and seeding with test data.`,
		commands.E2ESetup,
		"set up e2e database",
	)
}

func newE2EResetCmd() *cobra.Command {
	return builders.SimpleCommand(
		"reset",
		"Drop and recreate database with fresh data",
		`Completely resets the e2e database by dropping it, recreating it, running migrations, and seeding fresh test data.`,
		commands.E2EReset,
		"reset e2e database",
	)
}

func newE2ECreateCmd() *cobra.Command {
	return builders.MessageCommand(
		"create",
		"Create empty e2e database",
		`Creates an empty e2e database without running migrations or seeding data.`,
		"📦 Creating e2e database...",
		commands.E2ECreate,
		"create e2e database",
	)
}

func newE2EDropCmd() *cobra.Command {
	return builders.MessageCommand(
		"drop",
		"Drop e2e database",
		`Completely removes the e2e database and all its data.`,
		"🗑️  Dropping e2e database...",
		commands.E2EDrop,
		"drop e2e database",
	)
}

func newE2EMigrateCmd() *cobra.Command {
	return builders.MessageCommand(
		"migrate",
		"Run migrations on existing e2e database",
		`Applies all pending migrations to the existing e2e database.`,
		"🔧 Running migrations on e2e database...",
		commands.E2EMigrate,
		"migrate e2e database",
	)
}

func newE2ESeedCmd() *cobra.Command {
	return builders.MessageCommand(
		"seed",
		"Seed existing e2e database with test data",
		`Populates the existing e2e database with test data required for end-to-end testing.`,
		"🌱 Seeding e2e database...",
		commands.E2ESeed,
		"seed e2e database",
	)
}
