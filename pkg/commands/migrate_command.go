package commands

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/commands/common"
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

func Migrate(mods ...application.Module) error {
	if len(os.Args) < 2 {
		return ErrNoCommand
	}

	if err := ensureDirectories(); err != nil {
		return err
	}

	command := os.Args[1]
	ctx := context.Background()

	switch command {
	case "up", "down", "redo", "status":
		pool, err := common.GetDefaultDatabasePool()
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer pool.Close()
		return handleMigrationCommands(ctx, command, application.NewMigrationManager(pool))

	default:
		return fmt.Errorf("unsupported command: %s\nSupported commands: 'up', 'down', 'redo', 'status'", command)
	}
}

// printMigrationStatus prints the status of migrations in a formatted table
func printMigrationStatus(statuses []application.MigrationStatus) {
	fmt.Println("Migration ID                    Status      Applied At")
	fmt.Println("-----------------------------------------------------------")
	for _, status := range statuses {
		appliedStatus := "pending"
		appliedAt := "-"
		if status.Applied {
			appliedStatus = "applied"
			if status.AppliedAt != nil {
				appliedAt = status.AppliedAt.Format("2006-01-02 15:04:05")
			}
		}
		fmt.Printf("%-30s  %-10s  %s\n", status.ID, appliedStatus, appliedAt)
	}
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
		printMigrationStatus(statuses)

	default:
		return fmt.Errorf("unsupported command: %s\nSupported commands: 'up', 'down', 'redo', 'status'", command)
	}

	return nil
}

// MigrateWithSubcommand runs migration with a specific subcommand
// This is a wrapper for the unified command tool
func MigrateWithSubcommand(subcommand string, mods ...application.Module) error {
	// Temporarily modify os.Args to match the expected format
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	// Set os.Args to simulate direct migrate command call
	os.Args = []string{"migrate", subcommand}

	return Migrate(mods...)
}
