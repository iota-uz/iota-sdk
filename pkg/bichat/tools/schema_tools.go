package tools

import (
	"context"
	"fmt"
	"regexp"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// validIdentifierPattern validates SQL identifiers (table/column names).
var validIdentifierPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// isValidIdentifier validates that a name is a valid SQL identifier.
func isValidIdentifier(name string) bool {
	return validIdentifierPattern.MatchString(name)
}

// SchemaListTool lists all available tables and views in a schema.
type SchemaListTool struct {
	lister bichatsql.SchemaLister
}

// NewSchemaListTool creates a new schema list tool.
func NewSchemaListTool(lister bichatsql.SchemaLister) agents.Tool {
	return &SchemaListTool{
		lister: lister,
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

	tables, err := t.lister.SchemaList(ctx)
	if err != nil {
		return FormatToolError(
			ErrCodeQueryError,
			fmt.Sprintf("failed to list schema: %v", err),
			HintCheckConnection,
		), serrors.E(op, err, "failed to list schema")
	}

	if len(tables) == 0 {
		return FormatToolError(
			ErrCodeNoData,
			"no tables or views found in analytics schema",
			"Analytics schema may not be initialized",
			"Contact administrator to set up analytics views",
		), serrors.E(op, "no tables found")
	}

	// Convert to map format for tool output
	result := make([]map[string]any, len(tables))
	for i, table := range tables {
		result[i] = map[string]any{
			"schema": table.Schema,
			"name":   table.Name,
			"type":   "view",
		}
		if table.RowCount > 0 {
			result[i]["row_count"] = table.RowCount
		}
		if table.Description != "" {
			result[i]["description"] = table.Description
		}
	}

	return agents.FormatToolOutput(result)
}

// SchemaDescribeTool provides detailed schema information for a specific table.
type SchemaDescribeTool struct {
	describer bichatsql.SchemaDescriber
}

// NewSchemaDescribeTool creates a new schema describe tool.
func NewSchemaDescribeTool(describer bichatsql.SchemaDescriber) agents.Tool {
	return &SchemaDescribeTool{
		describer: describer,
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

	schema, err := t.describer.SchemaDescribe(ctx, params.TableName)
	if err != nil {
		return FormatToolError(
			ErrCodeQueryError,
			fmt.Sprintf("failed to describe schema: %v", err),
			HintCheckConnection,
		), serrors.E(op, err, "failed to describe schema")
	}

	if schema == nil || len(schema.Columns) == 0 {
		return FormatToolError(
			ErrCodeNoData,
			fmt.Sprintf("table not found: %s", params.TableName),
			HintUseSchemaList,
			"Check spelling and case sensitivity",
			"Table must exist in analytics schema",
		), serrors.E(op, fmt.Sprintf("table not found: %s", params.TableName))
	}

	// Convert to map format for tool output
	columns := make([]map[string]interface{}, len(schema.Columns))
	for i, col := range schema.Columns {
		colMap := map[string]interface{}{
			"column_name": col.Name,
			"data_type":   col.Type,
			"is_nullable": col.Nullable,
		}
		if col.DefaultValue != nil {
			colMap["column_default"] = *col.DefaultValue
		}
		if col.Description != "" {
			colMap["description"] = col.Description
		}
		columns[i] = colMap
	}

	result := map[string]interface{}{
		"table_name":  schema.Name,
		"schema":      schema.Schema,
		"columns":     columns,
		"indexes":     []map[string]interface{}{},
		"constraints": []map[string]interface{}{},
		"samples":     map[string][]interface{}{},
	}

	return agents.FormatToolOutput(result)
}
