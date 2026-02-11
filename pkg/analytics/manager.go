package analytics

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ManagerOption configures a ViewManager.
type ManagerOption func(*ViewManager)

// WithSchema sets the default schema for views that don't specify one.
func WithSchema(schema string) ManagerOption {
	return func(m *ViewManager) {
		m.schema = schema
	}
}

// ViewManager is the single source of truth for analytics view definitions.
// It holds all registered views and can sync them to the database.
type ViewManager struct {
	views  []View
	schema string
}

// NewViewManager creates a new ViewManager with the given options.
func NewViewManager(opts ...ManagerOption) *ViewManager {
	m := &ViewManager{
		schema: "analytics",
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Register adds one or more views to the manager.
// Views with an empty Schema field inherit the manager's default schema.
func (m *ViewManager) Register(views ...View) {
	for i := range views {
		if views[i].Schema == "" {
			views[i].Schema = m.schema
		}
	}
	m.views = append(m.views, views...)
}

// Schema returns the manager's default schema name.
func (m *ViewManager) Schema() string {
	return m.schema
}

// Views returns a copy of all registered views.
func (m *ViewManager) Views() []View {
	out := make([]View, len(m.views))
	copy(out, m.views)
	return out
}

// Sync creates or replaces all registered views in the database within a single transaction.
// It does NOT create the schema (assumes it exists from migration).
// It does NOT drop views (old migration views remain, harmless).
func (m *ViewManager) Sync(ctx context.Context, pool *pgxpool.Pool) error {
	if len(m.views) == 0 {
		return nil
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("analytics.ViewManager.Sync: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	for _, v := range m.views {
		schema := v.Schema
		if schema == "" {
			schema = m.schema
		}

		qualifiedName := fmt.Sprintf("%s.%s", quoteIdent(schema), quoteIdent(v.Name))

		// DROP first to allow column type changes (CREATE OR REPLACE cannot change types).
		drop := fmt.Sprintf("DROP VIEW IF EXISTS %s CASCADE", qualifiedName)
		if _, err := tx.Exec(ctx, drop); err != nil {
			return fmt.Errorf("analytics.ViewManager.Sync: drop %s.%s: %w", schema, v.Name, err)
		}

		ddl := fmt.Sprintf("CREATE VIEW %s AS %s", qualifiedName, v.SQL)
		if _, err := tx.Exec(ctx, ddl); err != nil {
			return fmt.Errorf("analytics.ViewManager.Sync: view %s.%s: %w", schema, v.Name, err)
		}

		if v.Description != "" {
			comment := fmt.Sprintf("COMMENT ON VIEW %s.%s IS %s",
				quoteIdent(schema), quoteIdent(v.Name), quoteLiteral(v.Description))
			if _, err := tx.Exec(ctx, comment); err != nil {
				return fmt.Errorf("analytics.ViewManager.Sync: comment on %s.%s: %w", schema, v.Name, err)
			}
		}

		for col, comment := range v.ColumnComments {
			colComment := fmt.Sprintf("COMMENT ON COLUMN %s.%s.%s IS %s",
				quoteIdent(schema), quoteIdent(v.Name), quoteIdent(col), quoteLiteral(comment))
			if _, err := tx.Exec(ctx, colComment); err != nil {
				return fmt.Errorf("analytics.ViewManager.Sync: column comment %s.%s.%s: %w", schema, v.Name, col, err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("analytics.ViewManager.Sync: commit: %w", err)
	}

	return nil
}

// quoteIdent quotes a SQL identifier to prevent injection.
func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// quoteLiteral quotes a SQL string literal.
func quoteLiteral(s string) string {
	return `'` + strings.ReplaceAll(s, `'`, `''`) + `'`
}
