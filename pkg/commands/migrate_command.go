package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/schema/ast"
	"github.com/iota-uz/iota-sdk/pkg/schema/collector"
	"github.com/iota-uz/iota-sdk/pkg/schema/diff"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

var (
	ErrNoCommand = errors.New("expected 'up', 'down', 'redo', or 'collect' subcommands")
)

// ensureDirectories creates necessary directories if they don't exist
func ensureDirectories() error {
	// Make sure migrations directory exists
	migrationsPath := os.Getenv("MIGRATIONS_DIR")
	if migrationsPath == "" {
		migrationsPath = "migrations"
	}

	if err := os.MkdirAll(migrationsPath, 0755); err != nil {
		return fmt.Errorf("failed to create migrations directory: %w", err)
	}

	return nil
}

func Migrate(mods ...application.Module) error {
	defer func() {
		if r := recover(); r != nil {
			configuration.Use().Unload()
			debug.PrintStack()
			os.Exit(1)
		}
	}()

	if len(os.Args) < 2 {
		return ErrNoCommand
	}

	conf := configuration.Use()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err := ensureDirectories(); err != nil {
		return err
	}

	command := os.Args[1]

	switch command {
	case "collect":
		return handleSchemaCommands(ctx, command, conf.LogrusLogLevel())
	default:
		return handleMigrationCommands(ctx, command, conf, mods...)
	}
}

func handleSchemaCommands(ctx context.Context, command string, logLevel logrus.Level) error {
	migrationsPath := os.Getenv("MIGRATIONS_DIR")
	if migrationsPath == "" {
		migrationsPath = "migrations"
	}

	modulesPath := os.Getenv("MODULES_DIR")
	if modulesPath == "" {
		modulesPath = "modules"
	}

	// Set log level for all components
	ast.SetLogLevel(logLevel)
	diff.SetLogLevel(logLevel)

	collector := collector.New(collector.Config{
		ModulesPath:    modulesPath,
		MigrationsPath: migrationsPath,
		SQLDialect:     "postgres",
		LogLevel:       logLevel,
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
