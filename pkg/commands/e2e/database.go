package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/pkg/commands/common"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/jackc/pgx/v5"
)

// Create drops and creates an empty e2e database
func Create() error {
	ctx := context.Background()
	conf := configuration.Use()

	// Connect directly to postgres database
	connString := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=disable",
		conf.Database.Host, conf.Database.Port, conf.Database.User, conf.Database.Password)

	conn, err := pgx.Connect(ctx, connString)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres database: %w", err)
	}
	defer func() {
		_ = conn.Close(ctx)
	}()

	// Drop existing e2e database if exists
	_, err = conn.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", E2E_DB_NAME))
	if err != nil {
		return fmt.Errorf("failed to drop existing e2e database: %w", err)
	}

	// Create new e2e database
	_, err = conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", E2E_DB_NAME))
	if err != nil {
		return fmt.Errorf("failed to create e2e database: %w", err)
	}

	conf.Logger().Info("Created e2e database", "database", E2E_DB_NAME)
	return nil
}

// Drop removes the e2e database
func Drop() error {
	ctx := context.Background()
	conf := configuration.Use()

	// Connect directly to postgres database
	connString := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=disable",
		conf.Database.Host, conf.Database.Port, conf.Database.User, conf.Database.Password)

	conn, err := pgx.Connect(ctx, connString)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres database: %w", err)
	}
	defer func() {
		_ = conn.Close(ctx)
	}()

	// Drop e2e database
	_, err = conn.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", E2E_DB_NAME))
	if err != nil {
		return fmt.Errorf("failed to drop e2e database: %w", err)
	}

	conf.Logger().Info("Dropped e2e database", "database", E2E_DB_NAME)
	return nil
}

// Migrate applies all migrations to the e2e database
func Migrate() error {
	// Get current directory and find project root (where go.mod is)
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Go up directories until we find go.mod (project root)
	projectRoot := wd
	for {
		if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(projectRoot)
		if parent == projectRoot {
			return fmt.Errorf("could not find project root with go.mod")
		}
		projectRoot = parent
	}

	if err := os.Chdir(projectRoot); err != nil {
		return fmt.Errorf("failed to change to project root: %w", err)
	}

	// Set environment variable for e2e database
	_ = os.Setenv("DB_NAME", E2E_DB_NAME)

	conf := configuration.Use()
	pool, err := GetE2EPool()
	if err != nil {
		return fmt.Errorf("failed to connect to e2e database for migrations: %w", err)
	}
	defer pool.Close()

	app, err := common.NewApplication(pool, modules.BuiltInModules...)
	if err != nil {
		return fmt.Errorf("failed to create application: %w", err)
	}

	// Apply migrations
	migrations := app.Migrations()
	if err := migrations.Run(); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	conf.Logger().Info("Applied migrations to e2e database")
	return nil
}

// Setup performs a complete e2e database setup
func Setup() error {
	conf := configuration.Use()
	conf.Logger().Info("Setting up e2e database...")

	if err := Create(); err != nil {
		return err
	}
	if err := Migrate(); err != nil {
		return err
	}
	if err := Seed(); err != nil {
		return err
	}

	conf.Logger().Info("E2E database setup complete!")
	return nil
}

// Reset drops and recreates the e2e database with fresh data
func Reset() error {
	conf := configuration.Use()
	conf.Logger().Info("Resetting e2e database...")

	if err := Create(); err != nil { // This drops and recreates
		return err
	}
	if err := Migrate(); err != nil {
		return err
	}
	if err := Seed(); err != nil {
		return err
	}

	conf.Logger().Info("E2E database reset complete!")
	return nil
}
