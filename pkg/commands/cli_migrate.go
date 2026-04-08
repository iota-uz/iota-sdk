// Package commands provides this package.
package commands

import (
	"github.com/spf13/cobra"
)

// NewMigrateCommand creates the migrate command group with all subcommands
func NewMigrateCommand() *cobra.Command {
	migrateCmd := &cobra.Command{
		Use:   "migrate",
		Short: "Database migration management",
		Long:  `Manage database schema migrations: apply, roll back, or redo.`,
		Example: `  # Apply all pending migrations
  command migrate up

  # Rollback last migration
  command migrate down

  # Rollback and reapply last migration
  command migrate redo

  # Show migration status
  command migrate status`,
	}

	migrateCmd.AddCommand(newMigrateUpCmd())
	migrateCmd.AddCommand(newMigrateDownCmd())
	migrateCmd.AddCommand(newMigrateRedoCmd())
	migrateCmd.AddCommand(newMigrateStatusCmd())

	return migrateCmd
}

func newMigrateUpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Apply all pending migrations",
		Long:  `Applies all pending database migrations to bring the schema up to the latest version.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return MigrateWithSubcommand("up")
		},
	}
}

func newMigrateDownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down",
		Short: "Rollback the last migration",
		Long:  `Rolls back the most recently applied database migration.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return MigrateWithSubcommand("down")
		},
	}
}

func newMigrateRedoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "redo",
		Short: "Rollback and reapply the last migration",
		Long:  `Rolls back the most recent migration and then reapplies it, useful for testing migration changes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return MigrateWithSubcommand("redo")
		},
	}
}

func newMigrateStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show migration status",
		Long:  `Displays the status of all migrations, showing which are applied and which are pending.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return MigrateWithSubcommand("status")
		},
	}
}
