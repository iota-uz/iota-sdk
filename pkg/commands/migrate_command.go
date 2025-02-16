package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/schema/collector"
	"github.com/jackc/pgx/v5/pgxpool"
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

	if err := ensureDirectories(); err != nil {
		return err
	}

	command := os.Args[1]
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	switch command {
	case "collect":
		return handleSchemaCommands(ctx, command)
	default:
		conf := configuration.Use()
		if conf == nil {
			return fmt.Errorf("failed to load configuration")
		}
		return handleMigrationCommands(ctx, command, conf, mods...)
	}
}

func handleSchemaCommands(ctx context.Context, command string) error {
	migrationsPath := os.Getenv("MIGRATIONS_DIR")
	if migrationsPath == "" {
		migrationsPath = "migrations"
	}

	modulesPath := os.Getenv("MODULES_DIR")
	if modulesPath == "" {
		modulesPath = "modules"
	}

	collector := collector.New(collector.Config{
		ModulesPath:    modulesPath,
		MigrationsPath: migrationsPath,
		SQLDialect:     "postgres",
	})

	switch command {
	case "collect":
		changes, err := collector.CollectMigrations(ctx)
		if err != nil {
			return fmt.Errorf("failed to collect migrations: %w", err)
		}
		return collector.StoreMigrations(changes)

	default:
		return fmt.Errorf("unknown schema command: %s", command)
	}
}

func handleMigrationCommands(ctx context.Context, command string, conf *configuration.Configuration, mods ...application.Module) error {
	pool, err := pgxpool.New(ctx, conf.DBOpts)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pool.Close()

	app := application.New(pool, eventbus.NewEventPublisher())
	if err := modules.Load(app, mods...); err != nil {
		return err
	}

	switch command {
	case "up":
		if err := app.RunMigrations(); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}

	case "down":
		if err := app.RollbackMigrations(); err != nil {
			return fmt.Errorf("failed to rollback migrations: %w", err)
		}

	case "redo":
		if err := app.RollbackMigrations(); err != nil {
			return errors.Join(err, errors.New("failed to rollback migrations"))
		}
		if err := app.RunMigrations(); err != nil {
			return errors.Join(err, errors.New("failed to run migrations"))
		}

	default:
		return fmt.Errorf("unsupported command: %s\nSupported commands: 'up', 'down', 'redo', 'collect'", command)
	}

	return nil
}
