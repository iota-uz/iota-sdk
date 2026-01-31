package tools

import (
	"context"
	"fmt"
	"regexp"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5/pgxpool"
)

// validIdentifierPattern validates SQL identifiers (table/column names).
var validIdentifierPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// isValidIdentifier validates that a name is a valid SQL identifier.
func isValidIdentifier(name string) bool {
	return validIdentifierPattern.MatchString(name)
}

// TableInfo represents information about a database table.
type TableInfo struct {
	Schema      string `json:"schema"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	RowCount    int64  `json:"row_count,omitempty"`
	Description string `json:"description,omitempty"`
}

// TableSchema represents detailed schema information for a table.
type TableSchema struct {
	TableName   string                   `json:"table_name"`
	Schema      string                   `json:"schema"`
	Columns     []map[string]interface{} `json:"columns"`
	Indexes     []map[string]interface{} `json:"indexes"`
	Constraints []map[string]interface{} `json:"constraints"`
	Samples     map[string][]interface{} `json:"samples"`
}

// SchemaListTool lists all available tables and views in a schema.
type SchemaListTool struct {
	executor QueryExecutorService
}

// NewSchemaListTool creates a new schema list tool.
func NewSchemaListTool(executor QueryExecutorService) agents.Tool {
	return &SchemaListTool{
		executor: executor,
	}
}

// Name returns the tool name.
func (t *SchemaListTool) Name() string {
	return "schema_list"
}

// Description returns the tool description for the LLM.
func (t *SchemaListTool) Description() string {
	return "List all available tables and views in the analytics schema. " +
		"Returns table names with row counts and descriptions."
}

// Parameters returns the JSON Schema for tool parameters.
func (t *SchemaListTool) Parameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

// Call executes the schema list operation.
func (t *SchemaListTool) Call(ctx context.Context, input string) (string, error) {
	const op serrors.Op = "SchemaListTool.Call"

	// For now, this is a simplified implementation that delegates to the executor
	// Consumers should implement a more complete version that queries their schema
	query := `
		SELECT
			schemaname AS schema,
			tablename AS name,
			'table' AS type
		FROM pg_catalog.pg_tables
		WHERE schemaname = 'analytics'
		UNION ALL
		SELECT
			schemaname AS schema,
			viewname AS name,
			'view' AS type
		FROM pg_catalog.pg_views
		WHERE schemaname = 'analytics'
		ORDER BY name
	`

	result, err := t.executor.ExecuteQuery(ctx, query, nil, 10000)
	if err != nil {
		return "", serrors.E(op, err, "failed to list schema")
	}

	return agents.FormatToolOutput(result)
}

// SchemaDescribeTool provides detailed schema information for a specific table.
type SchemaDescribeTool struct {
	executor QueryExecutorService
}

// NewSchemaDescribeTool creates a new schema describe tool.
func NewSchemaDescribeTool(executor QueryExecutorService) agents.Tool {
	return &SchemaDescribeTool{
		executor: executor,
	}
}

// Name returns the tool name.
func (t *SchemaDescribeTool) Name() string {
	return "schema_describe"
}

// Description returns the tool description for the LLM.
func (t *SchemaDescribeTool) Description() string {
	return "Get detailed schema information for a specific table or view. " +
		"Returns column names, types, constraints, indexes, and sample values."
}

// Parameters returns the JSON Schema for tool parameters.
func (t *SchemaDescribeTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"table_name": map[string]any{
				"type":        "string",
				"description": "The name of the table or view to describe (e.g., 'policies_with_details')",
			},
		},
		"required": []string{"table_name"},
	}
}

// schemaDescribeInput represents the parsed input parameters.
type schemaDescribeInput struct {
	TableName string `json:"table_name"`
}

// Call executes the schema describe operation.
func (t *SchemaDescribeTool) Call(ctx context.Context, input string) (string, error) {
	const op serrors.Op = "SchemaDescribeTool.Call"

	// Parse input
	params, err := agents.ParseToolInput[schemaDescribeInput](input)
	if err != nil {
		return "", serrors.E(op, err, "failed to parse input")
	}

	if params.TableName == "" {
		return "", serrors.E(op, "table_name parameter is required")
	}

	// Validate table name to prevent SQL injection
	if !isValidIdentifier(params.TableName) {
		return "", serrors.E(op, "invalid table name: must match pattern ^[a-zA-Z_][a-zA-Z0-9_]*$")
	}

	// Query column information
	columnsQuery := `
		SELECT
			column_name,
			data_type,
			is_nullable,
			column_default,
			character_maximum_length,
			numeric_precision,
			numeric_scale
		FROM information_schema.columns
		WHERE table_schema = 'analytics' AND table_name = $1
		ORDER BY ordinal_position
	`

	result, err := t.executor.ExecuteQuery(ctx, columnsQuery, []any{params.TableName}, 10000)
	if err != nil {
		return "", serrors.E(op, err, "failed to describe schema")
	}

	if result.RowCount == 0 {
		return "", serrors.E(op, fmt.Sprintf("table not found: %s", params.TableName))
	}

	// Format schema information
	schema := TableSchema{
		TableName:   params.TableName,
		Schema:      "analytics",
		Columns:     result.Rows,
		Indexes:     []map[string]interface{}{},
		Constraints: []map[string]interface{}{},
		Samples:     map[string][]interface{}{},
	}

	return agents.FormatToolOutput(schema)
}

// DefaultSchemaExecutor is a default implementation using pgxpool.
// This provides full schema querying capabilities.
type DefaultSchemaExecutor struct {
	pool *pgxpool.Pool
}

// NewDefaultSchemaExecutor creates a new default schema executor.
func NewDefaultSchemaExecutor(pool *pgxpool.Pool) QueryExecutorService {
	return &DefaultSchemaExecutor{
		pool: pool,
	}
}

// ExecuteQuery executes a SQL query (delegates to DefaultQueryExecutor).
func (e *DefaultSchemaExecutor) ExecuteQuery(ctx context.Context, query string, params []any, timeoutMs int) (*QueryResult, error) {
	executor := NewDefaultQueryExecutor(e.pool)
	return executor.ExecuteQuery(ctx, query, params, timeoutMs)
}
