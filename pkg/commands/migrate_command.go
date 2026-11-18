// Package commands provides this package.
package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/commands/common"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
	"github.com/sirupsen/logrus"
)

// ensureDirectories creates necessary directories if they don't exist
func ensureDirectories() error {
	dirs := []string{"migrations", "modules"}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

// MigrationOptions controls how a migration subcommand runs.
type MigrationOptions struct {
	// Steps is the number of migrations "down" rolls back. Values below 1
	// mean 1. Ignored when All is set.
	Steps int
	// All makes "down" roll back every applied migration.
	All bool
}

// RunMigration executes the given migration subcommand using the provided database config.
// cfg is resolved by the caller (typically a cobra RunE or MigrateWithSubcommand).
// "down" and "redo" affect only the most recently applied migration; use
// RunMigrationWithOptions to roll back more.
func RunMigration(cfg *dbconfig.Config, subcommand string) error {
	return RunMigrationWithOptions(context.Background(), cfg, subcommand, MigrationOptions{})
}

// RunMigrationWithOptions executes the given migration subcommand with explicit options.
func RunMigrationWithOptions(ctx context.Context, cfg *dbconfig.Config, subcommand string, opts MigrationOptions) error {
	if err := ensureDirectories(); err != nil {
		return err
	}

	switch subcommand {
	case "up", "down", "redo", "status":
		pool, err := common.GetDefaultDatabasePool(cfg)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer pool.Close()
		return handleMigrationCommands(ctx, subcommand, application.NewMigrationManager(pool, *cfg, logrus.StandardLogger()), opts)

	default:
		return fmt.Errorf("unsupported command: %s\nSupported commands: 'up', 'down', 'redo', 'status'", subcommand)
	}
}

// printMigrationStatus prints the status of migrations in a formatted table
func printMigrationStatus(w io.Writer, statuses []application.MigrationStatus) error {
	if _, err := fmt.Fprintln(w, "Migration ID                    Status      Applied At"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "-----------------------------------------------------------"); err != nil {
		return err
	}
	for _, status := range statuses {
		appliedStatus := "pending"
		appliedAt := "-"
		if status.Applied {
			appliedStatus = "applied"
			if status.AppliedAt != nil {
				appliedAt = status.AppliedAt.Format("2006-01-02 15:04:05")
			}
		}
		if _, err := fmt.Fprintf(w, "%-30s  %-10s  %s\n", status.ID, appliedStatus, appliedAt); err != nil {
			return err
		}
	}
	return nil
}

func handleMigrationCommands(
	ctx context.Context,
	command string,
	migrationManager application.MigrationManager,
	opts MigrationOptions,
) error {
	switch command {
	case "up":
		if err := migrationManager.Run(ctx); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}
	case "down":
		var err error
		if opts.All {
			err = migrationManager.RollbackAll(ctx)
		} else {
			err = migrationManager.Rollback(ctx, max(opts.Steps, 1))
		}
		if err != nil {
			return fmt.Errorf("failed to rollback migrations: %w", err)
		}
	case "redo":
		if err := migrationManager.Rollback(ctx, 1); err != nil {
			return errors.Join(err, errors.New("failed to rollback migrations"))
		}
		if err := migrationManager.Run(ctx); err != nil {
			return errors.Join(err, errors.New("failed to run migrations"))
		}
	case "status":
		statuses, err := migrationManager.Status(ctx)
		if err != nil {
			return fmt.Errorf("failed to get migration status: %w", err)
		}
		if err := printMigrationStatus(os.Stdout, statuses); err != nil {
			return fmt.Errorf("failed to print migration status: %w", err)
		}

	default:
		return fmt.Errorf("unsupported command: %s\nSupported commands: 'up', 'down', 'redo', 'status'", command)
	}

	return nil
}
