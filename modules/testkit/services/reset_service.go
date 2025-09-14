package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

type ResetService struct {
	app application.Application
}

func NewResetService(app application.Application) *ResetService {
	return &ResetService{
		app: app,
	}
}

func (s *ResetService) TruncateAllTables(ctx context.Context) error {
	logger := composables.UseLogger(ctx)
	db := s.app.DB()

	// Get all table names except migration-related tables
	query := `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public'
		AND table_type = 'BASE TABLE'
		AND table_name NOT LIKE '%migration%'
		AND table_name NOT LIKE 'schema_%'
	`

	rows, err := db.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to query tables: %w", err)
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
		return fmt.Errorf("error iterating table rows: %w", err)
	}

	if len(tables) == 0 {
		logger.Info("No tables found to truncate")
		return nil
	}

	// Begin transaction for atomic truncation
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Disable foreign key checks temporarily
	if _, err := tx.Exec(ctx, "SET session_replication_role = replica;"); err != nil {
		return fmt.Errorf("failed to disable foreign key checks: %w", err)
	}

	// Truncate all tables
	for _, table := range tables {
		query := fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE;", table)
		if _, err := tx.Exec(ctx, query); err != nil {
			logger.WithError(err).WithField("table", table).Error("Failed to truncate table")
			return fmt.Errorf("failed to truncate table %s: %w", table, err)
		}
		logger.WithField("table", table).Debug("Truncated table")
	}

	// Re-enable foreign key checks
	if _, err := tx.Exec(ctx, "SET session_replication_role = DEFAULT;"); err != nil {
		return fmt.Errorf("failed to re-enable foreign key checks: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit truncate transaction: %w", err)
	}

	logger.WithField("tableCount", len(tables)).Info("Successfully truncated all tables")
	logger.WithField("tables", strings.Join(tables, ", ")).Debug("Truncated tables")

	return nil
}

func (s *ResetService) CleanUploads(ctx context.Context) error {
	logger := composables.UseLogger(ctx)
	// TODO: Implement file cleanup for test uploads
	// This should clean up files in the uploads directory that were created during tests
	logger.Debug("Upload cleanup not yet implemented")
	return nil
}
