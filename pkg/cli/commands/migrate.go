package commands

import (
	"github.com/spf13/cobra"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/pkg/cli/builders"
	"github.com/iota-uz/iota-sdk/pkg/commands"
)

// NewMigrateCommand creates the migrate command group with all subcommands
func NewMigrateCommand() *cobra.Command {
	migrateCmd := builders.NewSubCommand(builders.SubCommandOptions{
		Use:   "migrate",
		Short: "Database migration management",
		Long:  `Manage database schema migrations including applying, rolling back, and collecting schema changes.`,
		Example: `  # Apply all pending migrations
  command migrate up

  # Rollback last migration
  command migrate down

  # Collect schema changes
  command migrate collect`,
	})

	// Add all migrate subcommands
	migrateCmd.AddCommand(newMigrateUpCmd())
	migrateCmd.AddCommand(newMigrateDownCmd())
	migrateCmd.AddCommand(newMigrateRedoCmd())
	migrateCmd.AddCommand(newMigrateCollectCmd())

	return migrateCmd
}

func newMigrateUpCmd() *cobra.Command {
	return builders.SimpleCommand(
		"up",
		"Apply all pending migrations",
		`Applies all pending database migrations to bring the schema up to the latest version.`,
		func() error {
			return commands.MigrateWithSubcommand("up", modules.BuiltInModules...)
		},
		"apply migrations",
	)
}

func newMigrateDownCmd() *cobra.Command {
	return builders.SimpleCommand(
		"down",
		"Rollback the last migration",
		`Rolls back the most recently applied database migration.`,
		func() error {
			return commands.MigrateWithSubcommand("down", modules.BuiltInModules...)
		},
		"rollback migration",
	)
}

func newMigrateRedoCmd() *cobra.Command {
	return builders.SimpleCommand(
		"redo",
		"Rollback and reapply the last migration",
		`Rolls back the most recent migration and then reapplies it, useful for testing migration changes.`,
		func() error {
			return commands.MigrateWithSubcommand("redo", modules.BuiltInModules...)
		},
		"redo migration",
	)
}

func newMigrateCollectCmd() *cobra.Command {
	return builders.SimpleCommand(
		"collect",
		"Collect schema migrations from modules",
		`Scans all modules for schema changes and collects them into migration files.`,
		func() error {
			return commands.MigrateWithSubcommand("collect", modules.BuiltInModules...)
		},
		"collect schema migrations",
	)
}
