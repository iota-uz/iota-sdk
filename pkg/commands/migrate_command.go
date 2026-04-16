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

var (
	ErrNoCommand = errors.New("expected 'up', 'down', 'redo', or 'status' subcommands")
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

// RunMigration executes the given migration subcommand using the provided database config.
// cfg is resolved by the caller (typically a cobra RunE or MigrateWithSubcommand).
func RunMigration(cfg *dbconfig.Config, subcommand string) error {
	if err := ensureDirectories(); err != nil {
		return err
	}

	ctx := context.Background()

	switch subcommand {
	case "up", "down", "redo", "status":
		pool, err := common.GetDefaultDatabasePool(cfg)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer pool.Close()
		return handleMigrationCommands(ctx, subcommand, application.NewMigrationManager(pool, *cfg, logrus.StandardLogger()))

	default:
		return fmt.Errorf("unsupported command: %s\nSupported commands: 'up', 'down', 'redo', 'status'", subcommand)
	}
}

// Migrate reads the subcommand from os.Args and delegates to RunMigration.
// cfg must be resolved by the caller before invoking this.
// Deprecated: use RunMigration directly from cobra RunE.
func Migrate(cfg *dbconfig.Config) error {
	if len(os.Args) < 2 {
		return ErrNoCommand
	}
	return RunMigration(cfg, os.Args[1])
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
) error {
	switch command {
	case "up":
		if err := migrationManager.Run(); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}
	case "down":
		if err := migrationManager.Rollback(); err != nil {
			return fmt.Errorf("failed to rollback migrations: %w", err)
		}
	case "redo":
		if err := migrationManager.Rollback(); err != nil {
			return errors.Join(err, errors.New("failed to rollback migrations"))
		}
		if err := migrationManager.Run(); err != nil {
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
