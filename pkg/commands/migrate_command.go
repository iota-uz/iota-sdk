package commands

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/logging"
	"github.com/iota-uz/iota-sdk/pkg/schema/collector"
	"github.com/iota-uz/utils/env"
	"github.com/jackc/pgx/v5/pgxpool"
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	logFile, logger, err := logging.FileLogger(conf.LogrusLogLevel(), conf.Loki.LogPath)
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer func() {
		if err := logFile.Close(); err != nil {
			log.Printf("Failed to close log file: %v", err)
		}
	}()

	if err := ensureDirectories(); err != nil {
		return err
	}

	command := os.Args[1]

	pool, err := pgxpool.New(ctx, conf.Database.Opts)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pool.Close()

	app := application.New(pool, eventbus.NewEventPublisher(conf.Logger()))
	if err := modules.Load(app, mods...); err != nil {
		return err
	}

	switch command {
	case "collect":
		return handleSchemaCommands(ctx, command, app, logger.Level)
	default:
		return handleMigrationCommands(ctx, command, app)
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
	app application.Application,
) error {
	// Get the migration manager from the application
	migrations := app.Migrations()

	switch command {
	case "up":
		if err := migrations.Run(); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}
	case "down":
		if err := migrations.Rollback(); err != nil {
			return fmt.Errorf("failed to rollback migrations: %w", err)
		}
	case "redo":
		if err := migrations.Rollback(); err != nil {
			return errors.Join(err, errors.New("failed to rollback migrations"))
		}
		if err := migrations.Run(); err != nil {
			return errors.Join(err, errors.New("failed to run migrations"))
		}

	default:
		return fmt.Errorf("unsupported command: %s\nSupported commands: 'up', 'down', 'redo', 'collect'", command)
	}

	return nil
}
