// Package e2e provides this package.
package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
	"github.com/jackc/pgx/v5"
	"github.com/sirupsen/logrus"
)

// CreateRaw drops and recreates the e2e database.
func CreateRaw(cfg *dbconfig.Config, logger *logrus.Logger) error {
	ctx := context.Background()
	ensureE2EDatabaseEnv()

	connString := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password)

	conn, err := pgx.Connect(ctx, connString)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres database: %w", err)
	}
	defer func() {
		_ = conn.Close(ctx)
	}()

	_, err = conn.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", E2EDBName))
	if err != nil {
		return fmt.Errorf("failed to drop existing e2e database: %w", err)
	}

	_, err = conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", E2EDBName))
	if err != nil {
		return fmt.Errorf("failed to create e2e database: %w", err)
	}

	logger.Info("Created e2e database", "database", E2EDBName)
	return nil
}

// DropRaw removes the e2e database.
func DropRaw(cfg *dbconfig.Config, logger *logrus.Logger) error {
	ctx := context.Background()
	ensureE2EDatabaseEnv()

	connString := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password)

	conn, err := pgx.Connect(ctx, connString)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres database: %w", err)
	}
	defer func() {
		_ = conn.Close(ctx)
	}()

	_, err = conn.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", E2EDBName))
	if err != nil {
		return fmt.Errorf("failed to drop e2e database: %w", err)
	}

	logger.Info("Dropped e2e database", "database", E2EDBName)
	return nil
}

// Migrate applies all migrations to the e2e database.
func Migrate(cfg *dbconfig.Config, logger *logrus.Logger) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

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

	ensureE2EDatabaseEnv()

	pool, err := GetE2EPool(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to e2e database for migrations: %w", err)
	}
	defer pool.Close()

	migrations := application.NewMigrationManagerLegacy(pool)
	if err := migrations.Run(); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	logger.Info("Applied migrations to e2e database")
	return nil
}

// Setup performs a complete e2e database setup.
func Setup(cfg *dbconfig.Config, logger *logrus.Logger) error {
	ensureE2EDatabaseEnv()
	logger.Info("Setting up e2e database...")

	exists, err := DatabaseExists(cfg)
	if err != nil {
		return fmt.Errorf("failed to check if database exists: %w", err)
	}

	if exists {
		logger.Info("E2E database exists, clearing data instead of recreating...")
		if err := TruncateAllTables(cfg, logger); err != nil {
			return fmt.Errorf("failed to truncate tables: %w", err)
		}
	} else {
		logger.Info("E2E database does not exist, creating fresh database...")
		if err := CreateRaw(cfg, logger); err != nil {
			return err
		}
	}
	if err := Migrate(cfg, logger); err != nil {
		return err
	}

	if err := SeedRaw(cfg, logger); err != nil {
		return err
	}

	logger.Info("E2E database setup complete!")
	return nil
}

// ResetRaw drops, recreates, migrates, and seeds the e2e database.
func ResetRaw(cfg *dbconfig.Config, logger *logrus.Logger) error {
	ensureE2EDatabaseEnv()
	logger.Info("Resetting e2e database...")

	if err := CreateRaw(cfg, logger); err != nil {
		return err
	}
	if err := Migrate(cfg, logger); err != nil {
		return err
	}
	if err := SeedRaw(cfg, logger); err != nil {
		return err
	}

	logger.Info("E2E database reset complete!")
	return nil
}

// Reset drops and recreates the e2e database with fresh data.
func Reset(cfg *dbconfig.Config, logger *logrus.Logger) error {
	return ResetRaw(cfg, logger)
}

// DatabaseExists checks if the e2e database exists.
func DatabaseExists(cfg *dbconfig.Config) (bool, error) {
	ctx := context.Background()
	ensureE2EDatabaseEnv()

	connString := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password)

	conn, err := pgx.Connect(ctx, connString)
	if err != nil {
		return false, fmt.Errorf("failed to connect to postgres database: %w", err)
	}
	defer func() {
		_ = conn.Close(ctx)
	}()

	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)"
	err = conn.QueryRow(ctx, query, E2EDBName).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check if database exists: %w", err)
	}

	return exists, nil
}

// TruncateAllTables clears all data from the e2e database while preserving connections.
func TruncateAllTables(cfg *dbconfig.Config, logger *logrus.Logger) error {
	ctx := context.Background()
	ensureE2EDatabaseEnv()

	pool, err := GetE2EPool(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to e2e database: %w", err)
	}
	defer pool.Close()

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

	if len(tables) > 0 {
		for _, table := range tables {
			truncateQuery := fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", table)
			if _, err := pool.Exec(ctx, truncateQuery); err != nil {
				return fmt.Errorf("failed to truncate table %s: %w", table, err)
			}
		}
		logger.Info("Truncated all tables in e2e database", "count", len(tables))
	} else {
		logger.Info("No tables found to truncate in e2e database")
	}

	return nil
}
