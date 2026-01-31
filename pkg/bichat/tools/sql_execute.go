package tools

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// QueryExecutorService defines the interface for executing SQL queries.
// Consumers can implement this interface to provide database access.
type QueryExecutorService interface {
	// ExecuteQuery executes a read-only SQL query and returns results.
	// The timeoutMs parameter specifies the query timeout in milliseconds.
	ExecuteQuery(ctx context.Context, sql string, params []any, timeoutMs int) (*QueryResult, error)
}

// QueryResult represents the result of a SQL query execution.
type QueryResult struct {
	Columns    []string                 `json:"columns"`
	Rows       []map[string]interface{} `json:"rows"`
	RowCount   int                      `json:"row_count"`
	IsLimited  bool                     `json:"is_limited"`
	DurationMs int64                    `json:"duration_ms,omitempty"`
}

// SQLExecuteTool executes SQL queries against a database via QueryExecutorService.
// It validates queries to ensure they are read-only and enforces row limits.
type SQLExecuteTool struct {
	executor QueryExecutorService
}

// NewSQLExecuteTool creates a new SQL execute tool.
// The executor parameter provides database access and should be provided by the consumer.
func NewSQLExecuteTool(executor QueryExecutorService) agents.Tool {
	return &SQLExecuteTool{
		executor: executor,
	}
}

// Name returns the tool name.
func (t *SQLExecuteTool) Name() string {
	return "sql_execute"
}

// Description returns the tool description for the LLM.
func (t *SQLExecuteTool) Description() string {
	return "Execute a read-only SQL query against the analytics database. " +
		"Use this for simple queries. For complex multi-step queries, delegate to the SQL agent. " +
		"Only SELECT queries are allowed. Results are limited to 1000 rows."
}

// Parameters returns the JSON Schema for tool parameters.
func (t *SQLExecuteTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The SQL SELECT query to execute. Must be read-only.",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of rows to return (default: 100, max: 1000)",
				"default":     100,
			},
			"explain_plan": map[string]any{
				"type":        "boolean",
				"description": "If true, return the query execution plan instead of results",
				"default":     false,
			},
		},
		"required": []string{"query"},
	}
}

// sqlExecuteInput represents the parsed input parameters.
type sqlExecuteInput struct {
	Query       string `json:"query"`
	Limit       int    `json:"limit,omitempty"`
	ExplainPlan bool   `json:"explain_plan,omitempty"`
}

// Call executes the SQL query and returns results as JSON.
func (t *SQLExecuteTool) Call(ctx context.Context, input string) (string, error) {
	const op serrors.Op = "SQLExecuteTool.Call"

	// Parse input
	params, err := agents.ParseToolInput[sqlExecuteInput](input)
	if err != nil {
		return "", serrors.E(op, err, "failed to parse input")
	}

	if params.Query == "" {
		return "", serrors.E(op, "query parameter is required")
	}

	// Set defaults
	if params.Limit == 0 {
		params.Limit = 100
	}
	if params.Limit > 1000 {
		params.Limit = 1000
	}

	// Validate query is read-only
	if err := validateReadOnlyQuery(params.Query); err != nil {
		return "", serrors.E(op, err)
	}

	// Execute via service
	result, err := t.executor.ExecuteQuery(ctx, params.Query, nil, 30000) // 30 second timeout
	if err != nil {
		return "", serrors.E(op, err, "query execution failed")
	}

	// Format response
	return agents.FormatToolOutput(result)
}

// validateReadOnlyQuery ensures the query is a SELECT statement.
func validateReadOnlyQuery(query string) error {
	const op serrors.Op = "tools.validateReadOnlyQuery"

	normalized := strings.ToUpper(strings.TrimSpace(query))

	// Must start with SELECT or WITH (for CTEs)
	if !strings.HasPrefix(normalized, "SELECT") && !strings.HasPrefix(normalized, "WITH") {
		return serrors.E(op, "only SELECT queries are allowed")
	}

	// Blacklist dangerous keywords
	dangerousKeywords := []string{
		"INSERT", "UPDATE", "DELETE", "DROP", "CREATE", "ALTER",
		"TRUNCATE", "GRANT", "REVOKE", "EXEC", "EXECUTE",
	}

	for _, keyword := range dangerousKeywords {
		if strings.Contains(normalized, keyword) {
			return serrors.E(op, fmt.Sprintf("query contains disallowed keyword: %s", keyword))
		}
	}

	return nil
}

// DefaultQueryExecutor is a default implementation of QueryExecutorService using pgxpool.
// Consumers can use this or provide their own implementation.
type DefaultQueryExecutor struct {
	pool *pgxpool.Pool
}

// NewDefaultQueryExecutor creates a new default query executor.
func NewDefaultQueryExecutor(pool *pgxpool.Pool) QueryExecutorService {
	return &DefaultQueryExecutor{
		pool: pool,
	}
}

// ExecuteQuery executes a SQL query with the given timeout.
func (e *DefaultQueryExecutor) ExecuteQuery(ctx context.Context, query string, params []any, timeoutMs int) (*QueryResult, error) {
	const op serrors.Op = "DefaultQueryExecutor.ExecuteQuery"

	// Add timeout to context
	timeout := time.Duration(timeoutMs) * time.Millisecond
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()

	// Execute query
	rows, err := e.pool.Query(ctx, query, params...)
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

	// Collect rows
	var results []map[string]interface{}
	rowCount := 0
	maxRows := 1000

	for rows.Next() {
		if rowCount >= maxRows {
			break
		}

		values, err := rows.Values()
		if err != nil {
			return nil, serrors.E(op, err, "failed to scan row")
		}

		row := make(map[string]interface{})
		for i, col := range columnNames {
			row[col] = formatValue(values[i])
		}

		results = append(results, row)
		rowCount++
	}

	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err, "error iterating rows")
	}

	duration := time.Since(start)

	return &QueryResult{
		Columns:    columnNames,
		Rows:       results,
		RowCount:   len(results),
		IsLimited:  len(results) >= maxRows,
		DurationMs: duration.Milliseconds(),
	}, nil
}

// formatValue formats a database value for JSON serialization.
func formatValue(value interface{}) interface{} {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case time.Time:
		return v.Format(time.RFC3339)
	case []byte:
		return string(v)
	case sql.NullString:
		if v.Valid {
			return v.String
		}
		return nil
	case sql.NullInt64:
		if v.Valid {
			return v.Int64
		}
		return nil
	case sql.NullFloat64:
		if v.Valid {
			return v.Float64
		}
		return nil
	case sql.NullBool:
		if v.Valid {
			return v.Bool
		}
		return nil
	case sql.NullTime:
		if v.Valid {
			return v.Time.Format(time.RFC3339)
		}
		return nil
	case pgx.Rows:
		// Handle nested rows if any
		return nil
	default:
		return v
	}
}
