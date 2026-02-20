package infrastructure

import (
	"context"
	"strings"
	"time"

	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresQueryExecutor implements bichatsql.QueryExecutor for PostgreSQL.
// It enforces multi-tenant isolation, SQL validation, and query timeouts.
type PostgresQueryExecutor struct {
	pool *pgxpool.Pool
}

// NewPostgresQueryExecutor creates a new PostgreSQL query executor.
// The pool parameter must be a valid *pgxpool.Pool connection.
func NewPostgresQueryExecutor(pool *pgxpool.Pool) bichatsql.QueryExecutor {
	return &PostgresQueryExecutor{
		pool: pool,
	}
}

// ExecuteQuery executes a read-only SQL query with tenant isolation and timeout enforcement.
// SECURITY: Multi-tenant isolation is enforced at the database layer using PostgreSQL session variables.
// The analytics schema views automatically filter by current_setting('app.tenant_id', true)::UUID.
// For non-system-catalog queries, only the analytics schema is allowed to prevent cross-tenant leakage
// (e.g. if the pool had broader privileges, public.* would not be filtered by app.tenant_id).
// System catalog queries (pg_catalog, information_schema) are allowed for schema introspection.
func (e *PostgresQueryExecutor) ExecuteQuery(ctx context.Context, sql string, params []any, timeout time.Duration) (*bichatsql.QueryResult, error) {
	const op serrors.Op = "PostgresQueryExecutor.ExecuteQuery"

	// Get tenant ID from context for multi-tenant isolation
	// Exception: System catalog queries don't need tenant context
	var tenantID string
	systemCatalog := e.isSystemCatalogQuery(sql)
	if !systemCatalog {
		tid, err := composables.UseTenantID(ctx)
		if err != nil {
			return nil, serrors.E(op, err, "tenant ID required for query execution")
		}
		tenantID = tid.String()
		if !e.referencesAnalyticsSchemaOnly(sql) {
			return nil, serrors.E(op, nil, "query must reference only analytics schema for tenant isolation")
		}
	}

	// Apply query timeout
	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Record start time
	start := time.Now()

	// Begin transaction for session variable isolation
	tx, err := e.pool.Begin(queryCtx)
	if err != nil {
		return nil, serrors.E(op, err, "failed to begin transaction")
	}
	defer tx.Rollback(queryCtx) //nolint:errcheck

	// SECURITY: Set tenant_id via parameterized set_config to avoid string interpolation
	if tenantID != "" {
		_, err = tx.Exec(queryCtx, "SELECT set_config('app.tenant_id', $1, true)", tenantID)
		if err != nil {
			return nil, serrors.E(op, err, "failed to set tenant context")
		}
	}

	// Execute query - views will automatically filter by tenant
	rows, err := tx.Query(queryCtx, sql, params...)
	if err != nil {
		return nil, serrors.E(op, err, "query execution failed")
	}
	defer rows.Close()

	// Get column descriptions
	fieldDescriptions := rows.FieldDescriptions()
	columnNames := make([]string, len(fieldDescriptions))
	for i, fd := range fieldDescriptions {
		columnNames[i] = fd.Name
	}

	// Collect rows (canonical format: [][]any).
	// Do not enforce a global executor cap here: tool-level limits (e.g. sql_execute,
	// export_query_to_excel) must control row count semantics.
	var results [][]any

	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, serrors.E(op, err, "failed to scan row")
		}

		// Format values and create row slice
		row := make([]any, len(values))
		for i, val := range values {
			row[i] = e.formatValue(val)
		}

		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err, "error iterating rows")
	}

	// Close rows before committing transaction
	rows.Close()

	// Commit transaction (session variable automatically cleared on commit)
	if err := tx.Commit(queryCtx); err != nil {
		return nil, serrors.E(op, err, "failed to commit transaction")
	}

	return &bichatsql.QueryResult{
		Columns:   columnNames,
		Rows:      results,
		RowCount:  len(results),
		Truncated: false,
		Duration:  time.Since(start),
		SQL:       sql,
	}, nil
}

// formatValue formats a database value for JSON serialization.
func (e *PostgresQueryExecutor) formatValue(value interface{}) interface{} {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case time.Time:
		return v.Format(time.RFC3339)
	case []byte:
		return string(v)
	default:
		return v
	}
}

// isSystemCatalogQuery checks if query accesses PostgreSQL system catalogs.
// System catalog queries are allowed without tenant context for schema introspection.
// Detects queries accessing system tables (pg_*, information_schema) for metadata operations.
func (e *PostgresQueryExecutor) isSystemCatalogQuery(sql string) bool {
	normalized := strings.ToLower(sql)

	// Explicit schema qualifiers
	if strings.Contains(normalized, "pg_catalog.") || strings.Contains(normalized, "information_schema.") {
		return true
	}

	// System table names (without schema prefix)
	// Common patterns: FROM pg_class, JOIN pg_namespace, etc.
	systemTables := []string{
		"pg_class", "pg_namespace", "pg_attribute", "pg_type",
		"pg_constraint", "pg_index", "pg_proc", "pg_description",
		"pg_tables", "pg_views", "pg_indexes", "pg_stats",
	}

	for _, table := range systemTables {
		// Match table name as a whole word (from/join pg_class, not my_pg_class_copy)
		// Simple check: preceded by whitespace/comma/paren and followed by whitespace/comma/paren
		if strings.Contains(normalized, " "+table+" ") ||
			strings.Contains(normalized, " "+table+"\n") ||
			strings.Contains(normalized, "\n"+table+" ") ||
			strings.Contains(normalized, ","+table+" ") ||
			strings.Contains(normalized, "("+table+" ") {
			return true
		}
	}

	// information_schema tables
	if strings.Contains(normalized, "information_schema.columns") ||
		strings.Contains(normalized, "information_schema.tables") ||
		strings.Contains(normalized, "information_schema.views") {
		return true
	}

	return false
}

// referencesAnalyticsSchemaOnly returns true if the query only references the analytics schema.
// Rejects public.* and requires explicit analytics. to prevent cross-tenant data leakage.
// Whitespace (newlines, tabs) is normalized to spaces so multi-line queries are accepted.
func (e *PostgresQueryExecutor) referencesAnalyticsSchemaOnly(sql string) bool {
	normalized := normalizeSQLForSchemaCheck(sql)
	if strings.Contains(normalized, " from public.") || strings.Contains(normalized, " join public.") {
		return false
	}
	return strings.Contains(normalized, " from analytics.") || strings.Contains(normalized, " join analytics.")
}

// normalizeSQLForSchemaCheck lowercases and collapses runs of whitespace to a single space.
// This allows schema checks to match "FROM analytics.x" regardless of newlines/tabs before FROM.
func normalizeSQLForSchemaCheck(sql string) string {
	sql = strings.TrimSpace(sql)
	var b strings.Builder
	b.Grow(len(sql))
	prevSpace := false
	for _, r := range strings.ToLower(sql) {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	return b.String()
}
