// Package analytics provides this package.
package analytics

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
// It holds all registered views and can reconcile them to the database.
type ViewManager struct {
	views  []View
	schema string
}

type viewKey struct {
	schema string
	name   string
}

type txBeginner interface {
	Begin(ctx context.Context) (dbTx, error)
}

type dbTx interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (dbRows, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type dbRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

type pgxPoolAdapter struct {
	pool *pgxpool.Pool
}

func (a pgxPoolAdapter) Begin(ctx context.Context) (dbTx, error) {
	tx, err := a.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}

	return pgxTxAdapter{tx: tx}, nil
}

type pgxTxAdapter struct {
	tx pgx.Tx
}

func (a pgxTxAdapter) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return a.tx.Exec(ctx, sql, args...)
}

func (a pgxTxAdapter) Query(ctx context.Context, sql string, args ...any) (dbRows, error) {
	rows, err := a.tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}

	return pgxRowsAdapter{rows: rows}, nil
}

func (a pgxTxAdapter) Commit(ctx context.Context) error {
	return a.tx.Commit(ctx)
}

func (a pgxTxAdapter) Rollback(ctx context.Context) error {
	return a.tx.Rollback(ctx)
}

type pgxRowsAdapter struct {
	rows pgx.Rows
}

func (a pgxRowsAdapter) Next() bool {
	return a.rows.Next()
}

func (a pgxRowsAdapter) Scan(dest ...any) error {
	return a.rows.Scan(dest...)
}

func (a pgxRowsAdapter) Err() error {
	return a.rows.Err()
}

func (a pgxRowsAdapter) Close() {
	a.rows.Close()
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

// Sync reconciles managed analytics views in the database to match the
// registered Go definitions exactly.
//
// Managed schemas are inferred from the manager's default schema plus the
// schemas of registered views. Any regular view currently present in those
// schemas but missing from the registered definitions is dropped as stale.
//
// Views are recreated with DROP VIEW ... CASCADE + CREATE VIEW to support
// incompatible shape changes. The full reconciliation runs inside one
// transaction, so failures roll back atomically.
func (m *ViewManager) Sync(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return errors.New("analytics.ViewManager.Sync: nil pool")
	}

	return m.sync(ctx, pgxPoolAdapter{pool: pool})
}

func (m *ViewManager) sync(ctx context.Context, beginner txBeginner) error {
	desiredViews, desiredKeys, err := m.normalizedViews()
	if err != nil {
		return fmt.Errorf("analytics.ViewManager.Sync: %w", err)
	}

	managedSchemas := m.managedSchemas(desiredViews)
	if len(managedSchemas) == 0 {
		return nil
	}

	tx, err := beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("analytics.ViewManager.Sync: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	existingViews, err := listExistingViews(ctx, tx, managedSchemas)
	if err != nil {
		return fmt.Errorf("analytics.ViewManager.Sync: list existing views: %w", err)
	}

	for _, staleView := range staleViews(existingViews, desiredKeys) {
		if err := dropView(ctx, tx, staleView); err != nil {
			return fmt.Errorf("analytics.ViewManager.Sync: drop stale %s.%s: %w", staleView.schema, staleView.name, err)
		}
	}

	for _, view := range desiredViews {
		if err := recreateView(ctx, tx, view); err != nil {
			return fmt.Errorf("analytics.ViewManager.Sync: reconcile %s.%s: %w", view.Schema, view.Name, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("analytics.ViewManager.Sync: commit: %w", err)
	}

	return nil
}

func (m *ViewManager) normalizedViews() ([]View, map[viewKey]View, error) {
	normalized := make([]View, len(m.views))
	seen := make(map[viewKey]View, len(m.views))

	for i, view := range m.views {
		if view.Schema == "" {
			view.Schema = m.schema
		}

		if strings.TrimSpace(view.Schema) == "" {
			return nil, nil, fmt.Errorf("view %q has empty schema", view.Name)
		}
		if strings.TrimSpace(view.Name) == "" {
			return nil, nil, errors.New("view definition has empty name")
		}
		if strings.TrimSpace(view.SQL) == "" {
			return nil, nil, fmt.Errorf("view %s.%s has empty SQL", view.Schema, view.Name)
		}

		key := viewKey{schema: view.Schema, name: view.Name}
		if existing, ok := seen[key]; ok {
			return nil, nil, fmt.Errorf(
				"duplicate view definition for %s.%s (existing SQL: %q, duplicate SQL: %q)",
				key.schema,
				key.name,
				existing.SQL,
				view.SQL,
			)
		}

		normalized[i] = view
		seen[key] = view
	}

	return normalized, seen, nil
}

func (m *ViewManager) managedSchemas(views []View) []string {
	schemas := make(map[string]struct{}, len(views)+1)
	if strings.TrimSpace(m.schema) != "" {
		schemas[m.schema] = struct{}{}
	}

	for _, view := range views {
		if strings.TrimSpace(view.Schema) == "" {
			continue
		}
		schemas[view.Schema] = struct{}{}
	}

	if len(schemas) == 0 {
		return nil
	}

	out := make([]string, 0, len(schemas))
	for schema := range schemas {
		out = append(out, schema)
	}
	sort.Strings(out)

	return out
}

func listExistingViews(ctx context.Context, tx dbTx, schemas []string) ([]viewKey, error) {
	const sql = `
SELECT table_schema, table_name
FROM information_schema.views
WHERE table_schema = ANY($1::text[])
ORDER BY table_schema, table_name
`

	rows, err := tx.Query(ctx, sql, schemas)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var views []viewKey
	for rows.Next() {
		var schema string
		var name string
		if err := rows.Scan(&schema, &name); err != nil {
			return nil, err
		}
		views = append(views, viewKey{schema: schema, name: name})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return views, nil
}

func staleViews(existing []viewKey, desired map[viewKey]View) []viewKey {
	stale := make([]viewKey, 0, len(existing))
	for _, view := range existing {
		if _, ok := desired[view]; ok {
			continue
		}
		stale = append(stale, view)
	}

	return stale
}

func recreateView(ctx context.Context, tx dbTx, view View) error {
	key := viewKey{schema: view.Schema, name: view.Name}
	if err := dropView(ctx, tx, key); err != nil {
		return err
	}

	create := fmt.Sprintf("CREATE VIEW %s AS %s", qualifiedViewName(key), view.SQL)
	if _, err := tx.Exec(ctx, create); err != nil {
		return fmt.Errorf("create view: %w", err)
	}

	if view.Description != "" {
		comment := fmt.Sprintf("COMMENT ON VIEW %s IS %s", qualifiedViewName(key), quoteLiteral(view.Description))
		if _, err := tx.Exec(ctx, comment); err != nil {
			return fmt.Errorf("comment on view: %w", err)
		}
	}

	for _, column := range sortedColumnNames(view.ColumnComments) {
		comment := fmt.Sprintf(
			"COMMENT ON COLUMN %s.%s IS %s",
			qualifiedViewName(key),
			quoteIdent(column),
			quoteLiteral(view.ColumnComments[column]),
		)
		if _, err := tx.Exec(ctx, comment); err != nil {
			return fmt.Errorf("comment on column %s: %w", column, err)
		}
	}

	return nil
}

func dropView(ctx context.Context, tx dbTx, view viewKey) error {
	drop := fmt.Sprintf("DROP VIEW IF EXISTS %s CASCADE", qualifiedViewName(view))
	if _, err := tx.Exec(ctx, drop); err != nil {
		return err
	}

	return nil
}

func sortedColumnNames(comments map[string]string) []string {
	if len(comments) == 0 {
		return nil
	}

	columns := make([]string, 0, len(comments))
	for column := range comments {
		columns = append(columns, column)
	}
	sort.Strings(columns)

	return columns
}

func qualifiedViewName(view viewKey) string {
	return fmt.Sprintf("%s.%s", quoteIdent(view.schema), quoteIdent(view.name))
}

// quoteIdent quotes a SQL identifier to prevent injection.
func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// quoteLiteral quotes a SQL string literal.
func quoteLiteral(s string) string {
	return `'` + strings.ReplaceAll(s, `'`, `''`) + `'`
}
