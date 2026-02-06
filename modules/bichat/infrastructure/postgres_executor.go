package infrastructure

import (
	"context"
	"fmt"
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
// System catalog queries (pg_catalog, information_schema) are allowed for schema introspection.
// SQL security is enforced by PostgreSQL role permissions (bichat_agent_role has SELECT only on analytics schema).
func (e *PostgresQueryExecutor) ExecuteQuery(ctx context.Context, sql string, params []any, timeout time.Duration) (*bichatsql.QueryResult, error) {
	const op serrors.Op = "PostgresQueryExecutor.ExecuteQuery"

	// Get tenant ID from context for multi-tenant isolation
	// Exception: System catalog queries don't need tenant context
	var tenantID string
	if !e.isSystemCatalogQuery(sql) {
		tid, err := composables.UseTenantID(ctx)
		if err != nil {
			return nil, serrors.E(op, err, "tenant ID required for query execution")
		}
		tenantID = tid.String()
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

	// SECURITY: Set tenant_id session variable for automatic view filtering
	// The analytics schema views use current_setting('app.tenant_id', true)::UUID for tenant isolation
	// Note: SET commands don't support parameterized queries, but tenant_id is validated as a UUID
	if tenantID != "" {
		setSQL := fmt.Sprintf("SET LOCAL app.tenant_id = '%s'", tenantID)
		_, err = tx.Exec(queryCtx, setSQL)
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

	// Collect rows (canonical format: [][]any)
	var results [][]any
	maxRows := 1000
	hitLimit := false

	for rows.Next() {
		if len(results) >= maxRows {
			hitLimit = true
			break
		}

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
		Truncated: hitLimit,
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
