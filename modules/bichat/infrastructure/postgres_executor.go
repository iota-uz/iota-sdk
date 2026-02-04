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
// It applies tenant_id filtering and enforces row limits.
// SECURITY: All queries MUST include WHERE tenant_id = $1 to prevent cross-tenant data leakage.
// Exception: Queries to system catalogs (pg_catalog, information_schema) are allowed without
// tenant_id filtering for schema introspection. SQL security is enforced by PostgreSQL role
// permissions (bichat_agent_role has SELECT only on analytics schema).
func (e *PostgresQueryExecutor) ExecuteQuery(ctx context.Context, sql string, params []any, timeout time.Duration) (*bichatsql.QueryResult, error) {
	const op serrors.Op = "PostgresQueryExecutor.ExecuteQuery"

	// SECURITY: Enforce tenant isolation by validating query includes tenant_id filtering
	// Exception: System catalog queries are allowed for schema introspection
	if !e.containsTenantFilter(sql) && !e.isSystemCatalogQuery(sql) {
		return nil, serrors.E(op, "query must include WHERE tenant_id = $1 for multi-tenant isolation")
	}

	// Prepare parameters based on query type
	var wrappedParams []any
	if e.isSystemCatalogQuery(sql) {
		// System catalog queries don't use tenant_id parameter
		wrappedParams = params
	} else {
		// Get tenant ID from context for multi-tenant isolation
		tenantID, err := composables.UseTenantID(ctx)
		if err != nil {
			return nil, serrors.E(op, err, "tenant ID required for query execution")
		}

		// SECURITY: Inject tenant ID as first parameter for multi-tenant isolation
		// The LLM agent writes queries with explicit tenant_id filtering (e.g., WHERE tenant_id = $1)
		// We automatically provide the tenant ID value, shifting user params by 1
		wrappedParams = append([]any{tenantID}, params...)
	}

	// Apply query timeout
	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Record start time
	start := time.Now()

	// Execute query with tenant ID as first parameter
	rows, err := e.pool.Query(queryCtx, sql, wrappedParams...)
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

// containsTenantFilter checks if SQL query includes tenant_id filtering.
// This is REQUIRED for multi-tenant isolation - all queries MUST filter by tenant_id.
func (e *PostgresQueryExecutor) containsTenantFilter(sql string) bool {
	normalized := strings.ToLower(sql)
	// Check for "tenant_id" keyword in query
	// LLM agents MUST write queries like: WHERE tenant_id = $1
	return strings.Contains(normalized, "tenant_id")
}

// isSystemCatalogQuery checks if query accesses PostgreSQL system catalogs.
// System catalog queries are allowed without tenant_id for schema introspection.
func (e *PostgresQueryExecutor) isSystemCatalogQuery(sql string) bool {
	normalized := strings.ToLower(sql)
	// Check if query accesses system catalogs
	return strings.Contains(normalized, "pg_catalog.") ||
		strings.Contains(normalized, "information_schema.")
}
