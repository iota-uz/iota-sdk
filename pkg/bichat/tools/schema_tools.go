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

	// Query analytics schema for tenant-isolated views
	// Note: This query does NOT include tenant_id filtering because it queries
	// system catalogs (pg_catalog), not user tables. The views themselves
	// contain tenant isolation logic via current_setting('app.tenant_id').
	query := `
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
		return FormatToolError(
			ErrCodeQueryError,
			fmt.Sprintf("failed to list schema: %v", err),
			HintCheckConnection,
		), serrors.E(op, err, "failed to list schema")
	}

	if result.RowCount == 0 {
		return FormatToolError(
			ErrCodeNoData,
			"no tables or views found in analytics schema",
			"Analytics schema may not be initialized",
			"Contact administrator to set up analytics views",
		), serrors.E(op, "no tables found")
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
		return FormatToolError(
			ErrCodeInvalidRequest,
			fmt.Sprintf("failed to parse input: %v", err),
			HintCheckRequiredFields,
		), serrors.E(op, err, "failed to parse input")
	}

	if params.TableName == "" {
		return FormatToolError(
			ErrCodeInvalidRequest,
			"table_name parameter is required",
			HintCheckRequiredFields,
			"Use schema_list to see available tables",
		), serrors.E(op, "table_name parameter is required")
	}

	// Validate table name to prevent SQL injection
	if !isValidIdentifier(params.TableName) {
		return FormatToolError(
			ErrCodeInvalidRequest,
			fmt.Sprintf("invalid table name '%s': must match pattern ^[a-zA-Z_][a-zA-Z0-9_]*$", params.TableName),
			HintCheckFieldFormat,
			"Table names must start with letter or underscore",
			"Use schema_list to see valid table names",
		), serrors.E(op, "invalid table name: must match pattern ^[a-zA-Z_][a-zA-Z0-9_]*$")
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
		return FormatToolError(
			ErrCodeQueryError,
			fmt.Sprintf("failed to describe schema: %v", err),
			HintCheckConnection,
		), serrors.E(op, err, "failed to describe schema")
	}

	if result.RowCount == 0 {
		return FormatToolError(
			ErrCodeNoData,
			fmt.Sprintf("table not found: %s", params.TableName),
			HintUseSchemaList,
			"Check spelling and case sensitivity",
			"Table must exist in analytics schema",
		), serrors.E(op, fmt.Sprintf("table not found: %s", params.TableName))
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

	// Attempt to fetch sample data (best effort - don't fail if this errors)
	sampleData := t.fetchSampleData(ctx, params.TableName)
	if sampleData != nil {
		schema.Samples = sampleData
	}

	return agents.FormatToolOutput(schema)
}

// fetchSampleData retrieves sample rows and statistics for a table.
// Returns nil if the query fails (graceful degradation).
func (t *SchemaDescribeTool) fetchSampleData(ctx context.Context, tableName string) map[string][]interface{} {
	const op serrors.Op = "SchemaDescribeTool.fetchSampleData"

	// Query for sample rows (limit 4)
	sampleQuery := fmt.Sprintf("SELECT * FROM analytics.%s LIMIT 4", tableName)
	sampleResult, err := t.executor.ExecuteQuery(ctx, sampleQuery, nil, 5000)
	if err != nil {
		// Log but don't fail - sample data is optional
		return nil
	}

	// Query for row count
	countQuery := fmt.Sprintf("SELECT COUNT(*) as row_count FROM analytics.%s", tableName)
	countResult, err := t.executor.ExecuteQuery(ctx, countQuery, nil, 5000)
	if err != nil {
		// Row count is also optional
		return nil
	}

	var rowCount int64
	if len(countResult.Rows) > 0 {
		if count, ok := countResult.Rows[0]["row_count"].(int64); ok {
			rowCount = count
		}
	}

	// Query for indexed columns
	indexQuery := `
		SELECT
			i.relname AS index_name,
			a.attname AS column_name
		FROM
			pg_index ix
			JOIN pg_class t ON t.oid = ix.indrelid
			JOIN pg_class i ON i.oid = ix.indexrelid
			JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(ix.indkey)
			JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE
			n.nspname = 'analytics'
			AND t.relname = $1
		ORDER BY i.relname, a.attnum
	`

	indexResult, err := t.executor.ExecuteQuery(ctx, indexQuery, []any{tableName}, 5000)
	if err != nil {
		// Index information is also optional
		indexResult = &QueryResult{Rows: []map[string]interface{}{}}
	}

	// Format sample data as markdown table
	sampleTable := formatSampleDataTable(sampleResult)

	// Build response with formatted data
	result := map[string][]interface{}{
		"sample_rows": make([]interface{}, len(sampleResult.Rows)),
		"sample_data_table": []interface{}{
			sampleTable,
		},
		"statistics": []interface{}{
			map[string]interface{}{
				"total_rows":            rowCount,
				"has_large_dataset":     rowCount > 1000000,
				"sample_representative": rowCount <= 1000000,
			},
		},
		"indexed_columns": make([]interface{}, len(indexResult.Rows)),
	}

	// Convert sample rows
	for i, row := range sampleResult.Rows {
		result["sample_rows"][i] = row
	}

	// Convert index information
	for i, row := range indexResult.Rows {
		result["indexed_columns"][i] = row
	}

	return result
}

// formatSampleDataTable formats sample rows as a markdown table.
func formatSampleDataTable(result *QueryResult) string {
	if result.RowCount == 0 {
		return "No sample data available."
	}

	var markdown string

	// Header row
	markdown += "|"
	for _, col := range result.Columns {
		markdown += fmt.Sprintf(" %s |", col)
	}
	markdown += "\n"

	// Separator row
	markdown += "|"
	for range result.Columns {
		markdown += " --- |"
	}
	markdown += "\n"

	// Data rows
	for _, row := range result.Rows {
		markdown += "|"
		for _, col := range result.Columns {
			val := row[col]
			if val == nil {
				markdown += " NULL |"
			} else {
				markdown += fmt.Sprintf(" %v |", val)
			}
		}
		markdown += "\n"
	}

	return markdown
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
