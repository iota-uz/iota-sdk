package tools

import (
	"context"
	stdlibsql "database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SQLExecuteTool executes SQL queries against a database via bichatsql.QueryExecutor.
// It validates queries to ensure they are read-only and enforces row limits.
type SQLExecuteTool struct {
	executor bichatsql.QueryExecutor
}

// NewSQLExecuteTool creates a new SQL execute tool.
// The executor parameter provides database access and should be provided by the consumer.
func NewSQLExecuteTool(executor bichatsql.QueryExecutor) agents.Tool {
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
	Query       string         `json:"query"`
	Params      map[string]any `json:"params,omitempty"`
	Limit       int            `json:"limit,omitempty"`
	ExplainPlan bool           `json:"explain_plan,omitempty"`
}

// placeholderPattern matches PostgreSQL placeholder syntax ($1, $2, etc.)
var placeholderPattern = regexp.MustCompile(`\$\d+`)

// Call executes the SQL query and returns results as JSON.
func (t *SQLExecuteTool) Call(ctx context.Context, input string) (string, error) {
	const op serrors.Op = "SQLExecuteTool.Call"

	// Parse input
	params, err := agents.ParseToolInput[sqlExecuteInput](input)
	if err != nil {
		return FormatToolError(
			ErrCodeInvalidRequest,
			fmt.Sprintf("failed to parse input: %v", err),
			HintCheckRequiredFields,
			HintCheckFieldTypes,
		), serrors.E(op, err, "failed to parse input")
	}

	if params.Query == "" {
		return FormatToolError(
			ErrCodeInvalidRequest,
			"query parameter is required",
			HintCheckRequiredFields,
		), serrors.E(op, "query parameter is required")
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
		return FormatToolError(
			ErrCodePolicyViolation,
			err.Error(),
			HintOnlySelectAllowed,
			HintNoWriteOperations,
			HintUseSchemaList,
		), serrors.E(op, err)
	}

	// Check for placeholder/parameter mismatch
	if err := validateQueryParameters(params.Query, params.Params); err != nil {
		return FormatToolError(
			ErrCodeInvalidRequest,
			err.Error(),
			"Use parameter binding for SQL injection protection",
			"Parameter binding is not yet implemented - use literal values in query for now",
			HintCheckSQLSyntax,
		), serrors.E(op, err)
	}

	// Execute via executor
	result, err := t.executor.ExecuteQuery(ctx, params.Query, nil, 30*time.Second)
	if err != nil {
		return FormatToolError(
			ErrCodeQueryError,
			fmt.Sprintf("query execution failed: %v", err),
			HintCheckSQLSyntax,
			HintVerifyTableNames,
			HintCheckJoinConditions,
		), serrors.E(op, err, "query execution failed")
	}

	// Convert to map format for tool output (tools expect map format)
	resultMap := map[string]any{
		"columns":     result.Columns,
		"rows":        result.AllMaps(),
		"row_count":   result.RowCount,
		"is_limited":  result.Truncated,
		"duration_ms": result.Duration.Milliseconds(),
	}

	// Format response
	return agents.FormatToolOutput(resultMap)
}

// validateReadOnlyQuery ensures the query is a SELECT statement.
func validateReadOnlyQuery(query string) error {
	normalized := strings.ToUpper(strings.TrimSpace(query))

	// Must start with SELECT or WITH (for CTEs)
	if !strings.HasPrefix(normalized, "SELECT") && !strings.HasPrefix(normalized, "WITH") {
		return fmt.Errorf("only SELECT queries are allowed")
	}

	// Blacklist dangerous keywords
	dangerousKeywords := []string{
		"INSERT", "UPDATE", "DELETE", "DROP", "CREATE", "ALTER",
		"TRUNCATE", "GRANT", "REVOKE", "EXEC", "EXECUTE",
	}

	for _, keyword := range dangerousKeywords {
		if strings.Contains(normalized, keyword) {
			return fmt.Errorf("query contains disallowed keyword: %s", keyword)
		}
	}

	return nil
}

// validateQueryParameters checks for placeholder/parameter mismatches.
// Returns an error if the query contains placeholders but no params are provided.
func validateQueryParameters(query string, params map[string]any) error {
	placeholders := placeholderPattern.FindAllString(query, -1)

	// If placeholders found but no params provided
	if len(placeholders) > 0 && len(params) == 0 {
		// Extract unique placeholders for clearer error message
		uniquePlaceholders := make(map[string]bool)
		for _, ph := range placeholders {
			uniquePlaceholders[ph] = true
		}

		placeholderList := make([]string, 0, len(uniquePlaceholders))
		for ph := range uniquePlaceholders {
			placeholderList = append(placeholderList, ph)
		}

		return fmt.Errorf("query contains placeholders (%s) but no params provided. Use parameter binding for SQL injection protection", strings.Join(placeholderList, ", "))
	}

	// If params provided but no placeholders found
	if len(params) > 0 && len(placeholders) == 0 {
		return fmt.Errorf("params provided but query contains no placeholders")
	}

	return nil
}

// DefaultQueryExecutor is a default implementation of bichatsql.QueryExecutor using pgxpool.
// Consumers can use this or provide their own implementation.
type DefaultQueryExecutor struct {
	pool *pgxpool.Pool
}

// NewDefaultQueryExecutor creates a new default query executor.
func NewDefaultQueryExecutor(pool *pgxpool.Pool) bichatsql.QueryExecutor {
	return &DefaultQueryExecutor{
		pool: pool,
	}
}

// ExecuteQuery executes a SQL query with the given timeout.
func (e *DefaultQueryExecutor) ExecuteQuery(ctx context.Context, query string, params []any, timeout time.Duration) (*bichatsql.QueryResult, error) {
	const op serrors.Op = "DefaultQueryExecutor.ExecuteQuery"

	// Add timeout to context
	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()

	// Execute query
	rows, err := e.pool.Query(queryCtx, query, params...)
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

		// Format values
		row := make([]any, len(values))
		for i, val := range values {
			row[i] = formatValue(val)
		}

		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err, "error iterating rows")
	}

	duration := time.Since(start)

	return &bichatsql.QueryResult{
		Columns:   columnNames,
		Rows:      results,
		RowCount:  len(results),
		Truncated: hitLimit,
		Duration:  duration,
		SQL:       query,
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
	case stdlibsql.NullString:
		if v.Valid {
			return v.String
		}
		return nil
	case stdlibsql.NullInt64:
		if v.Valid {
			return v.Int64
		}
		return nil
	case stdlibsql.NullFloat64:
		if v.Valid {
			return v.Float64
		}
		return nil
	case stdlibsql.NullBool:
		if v.Valid {
			return v.Bool
		}
		return nil
	case stdlibsql.NullTime:
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
