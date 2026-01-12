package commands

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/commands/common"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/schema/collector"
	"github.com/iota-uz/utils/env"
	"github.com/sirupsen/logrus"
)

var (
	ErrNoCommand = errors.New("expected 'up', 'down', 'redo', or 'collect' subcommands")
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

	conf := configuration.Use()

	if err := ensureDirectories(); err != nil {
		return err
	}

	command := os.Args[1]
	ctx := context.Background()

	// For DDL migrations (up/down/redo), we only need a database pool and migration manager.
	// Loading modules is not required and can cause issues if modules query the database
	// during registration (e.g., before the schema exists).
	switch command {
	case "collect":
		// Schema collection needs the full application to collect schemas from modules
		app, pool, err := common.NewApplicationWithDefaults(mods...)
		if err != nil {
			return fmt.Errorf("failed to initialize application: %w", err)
		}
		defer pool.Close()
		return handleSchemaCommands(ctx, command, app, conf.Logger().Level)

	case "up", "down", "redo":
		// DDL migrations only need database pool - don't load modules
		pool, err := common.GetDefaultDatabasePool()
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer pool.Close()
		return handleMigrationCommands(ctx, command, application.NewMigrationManager(pool))

	default:
		return fmt.Errorf("unsupported command: %s\nSupported commands: 'up', 'down', 'redo', 'collect'", command)
	}
}

func handleSchemaCommands(
	ctx context.Context,
	command string,
	app application.Application,
	logLevel logrus.Level,
) error {
	migrationsPath := env.GetEnv("MIGRATIONS_DIR", "migrations")

	switch command {
	case "collect":
		collector := collector.New(collector.Config{
			MigrationsPath: migrationsPath,
			LogLevel:       logLevel,
			EmbedFSs:       app.Migrations().SchemaFSs(),
		})
		upChanges, downChanges, err := collector.CollectMigrations(ctx)
		if err != nil {
			return fmt.Errorf("failed to collect migrations: %w", err)
		}
		return collector.StoreMigrations(upChanges, downChanges)

	default:
		return fmt.Errorf("unknown schema command: %s", command)
	}
}

func handleMigrationCommands(
	_ context.Context,
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

	default:
		return fmt.Errorf("unsupported command: %s\nSupported commands: 'up', 'down', 'redo', 'collect'", command)
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
