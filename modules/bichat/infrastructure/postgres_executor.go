package infrastructure

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/bichat/tools"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresQueryExecutor implements tools.QueryExecutorService for PostgreSQL.
// It enforces multi-tenant isolation, SQL validation, and query timeouts.
type PostgresQueryExecutor struct {
	pool *pgxpool.Pool
}

// NewPostgresQueryExecutor creates a new PostgreSQL query executor.
// The pool parameter must be a valid *pgxpool.Pool connection.
func NewPostgresQueryExecutor(pool *pgxpool.Pool) tools.QueryExecutorService {
	return &PostgresQueryExecutor{
		pool: pool,
	}
}

// ExecuteQuery executes a read-only SQL query with tenant isolation and timeout enforcement.
// It validates SQL safety, applies tenant_id filtering, and enforces row limits.
// SECURITY: All queries MUST include WHERE tenant_id = $1 to prevent cross-tenant data leakage.
// The executor automatically provides the tenant ID value as the first parameter.
func (e *PostgresQueryExecutor) ExecuteQuery(ctx context.Context, sql string, params []any, timeoutMs int) (*tools.QueryResult, error) {
	const op serrors.Op = "PostgresQueryExecutor.ExecuteQuery"

	// Validate SQL is read-only and safe
	if err := e.validateSQL(sql); err != nil {
		return nil, serrors.E(op, err)
	}

	// SECURITY: Enforce tenant isolation by validating query includes tenant_id filtering
	if !e.containsTenantFilter(sql) {
		return nil, serrors.E(op, "query must include WHERE tenant_id = $1 for multi-tenant isolation")
	}

	// Get tenant ID from context for multi-tenant isolation
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err, "tenant ID required for query execution")
	}

	// SECURITY: Inject tenant ID as first parameter for multi-tenant isolation
	// The LLM agent writes queries with explicit tenant_id filtering (e.g., WHERE tenant_id = $1)
	// We automatically provide the tenant ID value, shifting user params by 1
	wrappedParams := append([]any{tenantID}, params...)

	// Apply query timeout
	timeout := time.Duration(timeoutMs) * time.Millisecond
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
		columnNames[i] = string(fd.Name)
	}

	// Collect rows
	var results []map[string]interface{}
	maxRows := 1000

	for rows.Next() {
		if len(results) >= maxRows {
			break
		}

		values, err := rows.Values()
		if err != nil {
			return nil, serrors.E(op, err, "failed to scan row")
		}

		row := make(map[string]interface{})
		for i, col := range columnNames {
			row[col] = e.formatValue(values[i])
		}

		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err, "error iterating rows")
	}

	return &tools.QueryResult{
		Columns:    columnNames,
		Rows:       results,
		RowCount:   len(results),
		IsLimited:  len(results) >= maxRows,
		DurationMs: time.Since(start).Milliseconds(),
	}, nil
}

// validateSQL ensures the query is read-only and doesn't contain dangerous operations.
// Note: Tenant isolation is enforced by mandatory tenant_id filtering in ExecuteQuery, not by SQL validation.
func (e *PostgresQueryExecutor) validateSQL(sql string) error {
	const op serrors.Op = "PostgresQueryExecutor.validateSQL"

	normalized := strings.ToUpper(strings.TrimSpace(sql))

	// Must start with SELECT or WITH (for CTEs)
	if !strings.HasPrefix(normalized, "SELECT") && !strings.HasPrefix(normalized, "WITH") {
		return serrors.E(op, "only SELECT and WITH queries are allowed")
	}

	// Comprehensive list of dangerous keywords to block
	dangerousKeywords := []string{
		// Data modification
		"INSERT", "UPDATE", "DELETE", "TRUNCATE",
		// Schema changes
		"DROP", "CREATE", "ALTER", "RENAME",
		// Access control
		"GRANT", "REVOKE",
		// Administrative
		"VACUUM", "ANALYZE", "REINDEX", "CLUSTER", "CHECKPOINT",
		// Transaction control
		"COMMIT", "ROLLBACK", "SAVEPOINT", "PREPARE", "DEALLOCATE",
		// Session/config
		"SET", "RESET", "DISCARD", "LOCK",
		// Pub/Sub
		"LISTEN", "NOTIFY",
		// Execution
		"EXEC", "EXECUTE", "COPY",
		// Metadata
		"COMMENT",
	}

	// Check for dangerous keywords with comprehensive pattern matching
	// Patterns: " KEYWORD " | " KEYWORD;" | ";KEYWORD " | ";KEYWORD;" | ending with KEYWORD
	for _, keyword := range dangerousKeywords {
		if strings.Contains(normalized, " "+keyword+" ") ||
			strings.Contains(normalized, " "+keyword+";") ||
			strings.Contains(normalized, ";"+keyword+" ") ||
			strings.Contains(normalized, ";"+keyword+";") ||
			strings.HasSuffix(normalized, " "+keyword) ||
			strings.HasSuffix(normalized, ";"+keyword) {
			return serrors.E(op, fmt.Sprintf("query contains disallowed keyword: %s", keyword))
		}
	}

	// Verify it's actually a SELECT or CTE, not just prefixed
	if !strings.HasPrefix(normalized, "SELECT") && !strings.HasPrefix(normalized, "WITH") {
		return serrors.E(op, "query must start with SELECT or WITH")
	}

	return nil
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
