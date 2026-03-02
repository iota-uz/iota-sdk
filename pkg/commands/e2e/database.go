// Package e2e provides this package.
package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/pkg/commands/common"
	"github.com/iota-uz/iota-sdk/pkg/commands/safety"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/jackc/pgx/v5"
)

// Create drops and creates an empty e2e database
func Create(opts safety.RunOptions) error {
	ctx := context.Background()
	conf := configuration.Use()
	originalDBName := conf.Database.Name
	conf.Database.Name = E2EDBName
	defer func() { conf.Database.Name = originalDBName }()

	pool, poolErr := GetE2EPool()
	if poolErr == nil {
		defer pool.Close()
	}
	preflight, err := safety.RunPreflight(ctx, pool, safety.OperationE2ECreate)
	if err != nil {
		return fmt.Errorf("failed to run e2e create preflight: %w", err)
	}
	safety.PrintPreflight(os.Stdout, preflight, opts)
	if err := safety.EnforceSafety(opts, preflight, os.Stdin, os.Stdout); err != nil {
		return err
	}
	if opts.DryRun {
		safety.PrintOutcomeSummary(os.Stdout, "e2e create", false, true)
		return nil
	}

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
	_, err = conn.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", E2EDBName))
	if err != nil {
		return fmt.Errorf("failed to drop existing e2e database: %w", err)
	}

	// Create new e2e database
	_, err = conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", E2EDBName))
	if err != nil {
		return fmt.Errorf("failed to create e2e database: %w", err)
	}

	conf.Logger().Info("Created e2e database", "database", E2EDBName)
	safety.PrintOutcomeSummary(os.Stdout, "e2e create", true, false)
	return nil
}

// Drop removes the e2e database
func Drop(opts safety.RunOptions) error {
	ctx := context.Background()
	conf := configuration.Use()
	originalDBName := conf.Database.Name
	conf.Database.Name = E2EDBName
	defer func() { conf.Database.Name = originalDBName }()

	pool, poolErr := GetE2EPool()
	if poolErr == nil {
		defer pool.Close()
	}
	preflight, err := safety.RunPreflight(ctx, pool, safety.OperationE2EDrop)
	if err != nil {
		return fmt.Errorf("failed to run e2e drop preflight: %w", err)
	}
	safety.PrintPreflight(os.Stdout, preflight, opts)
	if err := safety.EnforceSafety(opts, preflight, os.Stdin, os.Stdout); err != nil {
		return err
	}
	if opts.DryRun {
		safety.PrintOutcomeSummary(os.Stdout, "e2e drop", false, true)
		return nil
	}

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
	_, err = conn.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", E2EDBName))
	if err != nil {
		return fmt.Errorf("failed to drop e2e database: %w", err)
	}

	conf.Logger().Info("Dropped e2e database", "database", E2EDBName)
	safety.PrintOutcomeSummary(os.Stdout, "e2e drop", true, false)
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
	_ = os.Setenv("DB_NAME", E2EDBName)

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

	// Check if database exists first
	exists, err := DatabaseExists()
	if err != nil {
		return fmt.Errorf("failed to check if database exists: %w", err)
	}

	if exists {
		conf.Logger().Info("E2E database exists, clearing data instead of recreating...")
		// Database exists, just clear the data to avoid connection conflicts
		if err := TruncateAllTables(); err != nil {
			return fmt.Errorf("failed to truncate tables: %w", err)
		}
	} else {
		conf.Logger().Info("E2E database does not exist, creating fresh database...")
		// Database doesn't exist, create it
		if err := Create(safety.RunOptions{
			Force: true,
			Yes:   true,
		}); err != nil {
			return err
		}
		// Apply migrations for new database
		if err := Migrate(); err != nil {
			return err
		}
	}

	// Always seed with fresh test data
	if err := Seed(safety.RunOptions{
		Yes: true,
	}); err != nil {
		return err
	}

	conf.Logger().Info("E2E database setup complete!")
	return nil
}

// Reset drops and recreates the e2e database with fresh data
func Reset(opts safety.RunOptions) error {
	conf := configuration.Use()
	conf.Logger().Info("Resetting e2e database...")

	ctx := context.Background()
	originalDBName := conf.Database.Name
	conf.Database.Name = E2EDBName
	defer func() { conf.Database.Name = originalDBName }()

	pool, poolErr := GetE2EPool()
	if poolErr == nil {
		defer pool.Close()
	}
	preflight, err := safety.RunPreflight(ctx, pool, safety.OperationE2EReset)
	if err != nil {
		return fmt.Errorf("failed to run e2e reset preflight: %w", err)
	}
	safety.PrintPreflight(os.Stdout, preflight, opts)
	if err := safety.EnforceSafety(opts, preflight, os.Stdin, os.Stdout); err != nil {
		return err
	}
	if opts.DryRun {
		safety.PrintOutcomeSummary(os.Stdout, "e2e reset", false, true)
		return nil
	}

	createOpts := opts
	createOpts.DryRun = false
	createOpts.Yes = true
	createOpts.Force = true
	if err := Create(createOpts); err != nil { // This drops and recreates
		return err
	}
	if err := Migrate(); err != nil {
		return err
	}
	seedOpts := opts
	seedOpts.DryRun = false
	seedOpts.Yes = true
	if err := Seed(seedOpts); err != nil {
		return err
	}

	conf.Logger().Info("E2E database reset complete!")
	safety.PrintOutcomeSummary(os.Stdout, "e2e reset", true, false)
	return nil
}

// DatabaseExists checks if the e2e database exists
func DatabaseExists() (bool, error) {
	ctx := context.Background()
	conf := configuration.Use()

	// Connect directly to postgres database
	connString := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=disable",
		conf.Database.Host, conf.Database.Port, conf.Database.User, conf.Database.Password)

	conn, err := pgx.Connect(ctx, connString)
	if err != nil {
		return false, fmt.Errorf("failed to connect to postgres database: %w", err)
	}
	defer func() {
		_ = conn.Close(ctx)
	}()

	// Check if database exists
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)"
	err = conn.QueryRow(ctx, query, E2EDBName).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check if database exists: %w", err)
	}

	return exists, nil
}

// TruncateAllTables clears all data from the e2e database while preserving connections
func TruncateAllTables() error {
	ctx := context.Background()
	conf := configuration.Use()

	// Set environment variable for e2e database
	_ = os.Setenv("DB_NAME", E2EDBName)

	pool, err := GetE2EPool()
	if err != nil {
		return fmt.Errorf("failed to connect to e2e database: %w", err)
	}
	defer pool.Close()

	// Get all table names
	query := `
		SELECT tablename
		FROM pg_tables
		WHERE schemaname = 'public'
		AND tablename NOT LIKE 'schema_migrations%'
		ORDER BY tablename
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to get table names: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return fmt.Errorf("failed to scan table name: %w", err)
		}
		tables = append(tables, tableName)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating table names: %w", err)
	}

	// Truncate all tables with CASCADE to handle foreign keys
	if len(tables) > 0 {
		for _, table := range tables {
			truncateQuery := fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", table)
			if _, err := pool.Exec(ctx, truncateQuery); err != nil {
				return fmt.Errorf("failed to truncate table %s: %w", table, err)
			}
		}
		conf.Logger().Info("Truncated all tables in e2e database", "count", len(tables))
	} else {
		conf.Logger().Info("No tables found to truncate in e2e database")
	}

	return nil
}
