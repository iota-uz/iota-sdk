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
	"github.com/iota-uz/iota-sdk/pkg/schema/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNoCommand = errors.New("expected 'up', 'down', 'redo', 'collect', 'detect', 'generate', or 'validate' subcommands")
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
	case "collect", "detect", "generate", "validate":
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

	store, err := migrations.NewStore(migrationsPath)
	if err != nil {
		return fmt.Errorf("failed to initialize migration store: %w", err)
	}

	switch command {
	case "collect":
		changes, err := collector.CollectMigrations(ctx)
		if err != nil {
			return fmt.Errorf("failed to collect migrations: %w", err)
		}
		return collector.StoreMigrations(changes)

	case "detect":
		changes, err := collector.CollectMigrations(ctx)
		if err != nil {
			return fmt.Errorf("failed to detect schema changes: %w", err)
		}
		if len(changes.Changes) == 0 {
			fmt.Println("No schema changes detected")
			return nil
		}
		fmt.Printf("Detected %d schema changes\n", len(changes.Changes))
		for _, change := range changes.Changes {
			fmt.Printf("- %s: %s\n", change.Type, change.ObjectName)
		}
		return collector.StoreMigrations(changes)

	case "generate":
		changes, err := collector.CollectMigrations(ctx)
		if err != nil {
			return fmt.Errorf("failed to generate migration: %w", err)
		}
		return collector.StoreMigrations(changes)

	case "validate":
		errors := store.ValidateMigrations()
		if len(errors) == 0 {
			fmt.Println("All migrations are valid")
			return nil
		}
		fmt.Printf("Found %d validation errors:\n", len(errors))
		for _, err := range errors {
			fmt.Printf("- %s\n", err)
		}
		return fmt.Errorf("migration validation failed")

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
		return fmt.Errorf("unsupported command: %s\nSupported commands: 'up', 'down', 'redo', 'collect', 'detect', 'generate', 'validate'", command)
	}

	return nil
}
