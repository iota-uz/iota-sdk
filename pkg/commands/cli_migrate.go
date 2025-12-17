package commands

import (
	"github.com/spf13/cobra"

	"github.com/iota-uz/iota-sdk/modules"
)

// NewMigrateCommand creates the migrate command group with all subcommands
func NewMigrateCommand() *cobra.Command {
	migrateCmd := &cobra.Command{
		Use:   "migrate",
		Short: "Database migration management",
		Long:  `Manage database schema migrations including applying, rolling back, and collecting schema changes.`,
		Example: `  # Apply all pending migrations
  command migrate up

  # Rollback last migration
  command migrate down

  # Collect schema changes
  command migrate collect`,
	}

	// Add all migrate subcommands
	migrateCmd.AddCommand(newMigrateUpCmd())
	migrateCmd.AddCommand(newMigrateDownCmd())
	migrateCmd.AddCommand(newMigrateRedoCmd())
	migrateCmd.AddCommand(newMigrateCollectCmd())

	return migrateCmd
}

func newMigrateUpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Apply all pending migrations",
		Long:  `Applies all pending database migrations to bring the schema up to the latest version.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return MigrateWithSubcommand("up", modules.BuiltInModules...)
		},
	}
}

func newMigrateDownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down",
		Short: "Rollback the last migration",
		Long:  `Rolls back the most recently applied database migration.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return MigrateWithSubcommand("down", modules.BuiltInModules...)
		},
	}
}

func newMigrateRedoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "redo",
		Short: "Rollback and reapply the last migration",
		Long:  `Rolls back the most recent migration and then reapplies it, useful for testing migration changes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return MigrateWithSubcommand("redo", modules.BuiltInModules...)
		},
	}
}

func newMigrateCollectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "collect",
		Short: "Collect schema migrations from modules",
		Long:  `Scans all modules for schema changes and collects them into migration files.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return MigrateWithSubcommand("collect", modules.BuiltInModules...)
		},
	}
}
