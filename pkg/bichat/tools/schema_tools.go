package tools

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/permissions"
	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// validIdentifierPattern validates SQL identifiers (table/column names).
var validIdentifierPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// isValidIdentifier validates that a name is a valid SQL identifier.
func isValidIdentifier(name string) bool {
	return validIdentifierPattern.MatchString(name)
}

// SchemaListToolOption configures a SchemaListTool.
type SchemaListToolOption func(*SchemaListTool)

// SchemaListTool lists all available tables and views in a schema.
// Optionally annotates views with access status based on user permissions.
type SchemaListTool struct {
	lister     bichatsql.SchemaLister
	viewAccess permissions.ViewAccessControl
}

// NewSchemaListTool creates a new schema list tool.
// The lister parameter provides schema listing functionality.
// Optional WithSchemaListViewAccess option enables permission-based access annotations.
func NewSchemaListTool(lister bichatsql.SchemaLister, opts ...SchemaListToolOption) agents.Tool {
	tool := &SchemaListTool{
		lister: lister,
	}

	for _, opt := range opts {
		opt(tool)
	}

	return tool
}

// WithSchemaListViewAccess adds view permission checking to the schema list tool.
// When configured, the tool will annotate each view with "access": "ok" or "access": "denied".
func WithSchemaListViewAccess(vac permissions.ViewAccessControl) SchemaListToolOption {
	return func(t *SchemaListTool) {
		t.viewAccess = vac
	}
}

// Name returns the tool name.
func (t *SchemaListTool) Name() string {
	return "schema_list"
}

// Description returns the tool description for the LLM.
func (t *SchemaListTool) Description() string {
	return "List all available tables and views in the analytics schema with approximate row counts."
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

	// Check permissions if view access control is configured
	var viewInfos []permissions.ViewInfo
	if t.viewAccess != nil {
		viewNames := make([]string, len(tables))
		for i, table := range tables {
			viewNames[i] = table.Name
		}
		viewInfos, _ = t.viewAccess.GetAccessibleViews(ctx, viewNames)
	}

	// Build markdown table
	var b strings.Builder
	b.WriteString("## Available Tables\n\n")

	// Header
	if t.viewAccess != nil {
		b.WriteString("| # | Table | ~Rows | Access | Description |\n")
		b.WriteString("| --- | --- | --- | --- | --- |\n")
	} else {
		b.WriteString("| # | Table | ~Rows | Description |\n")
		b.WriteString("| --- | --- | --- | --- |\n")
	}

	// Rows
	for i, table := range tables {
		b.WriteString(fmt.Sprintf("| %d | %s | ", i+1, table.Name))

		// Row count
		if table.RowCount > 0 {
			b.WriteString(fmt.Sprintf("~%d | ", table.RowCount))
		} else {
			b.WriteString("- | ")
		}

		// Access (if configured)
		if t.viewAccess != nil {
			if i < len(viewInfos) {
				b.WriteString(fmt.Sprintf("%s | ", viewInfos[i].Access))
			} else {
				b.WriteString("- | ")
			}
		}

		// Description
		if table.Description != "" {
			b.WriteString(fmt.Sprintf("%s |\n", table.Description))
		} else {
			b.WriteString("- |\n")
		}
	}

	// Footer
	b.WriteString(fmt.Sprintf("\n%d table(s) found.", len(tables)))

	return b.String(), nil
}

// SchemaDescribeToolOption configures a SchemaDescribeTool.
type SchemaDescribeToolOption func(*SchemaDescribeTool)

// SchemaDescribeTool provides detailed schema information for a specific table.
// Optionally checks permissions before returning schema details.
type SchemaDescribeTool struct {
	describer  bichatsql.SchemaDescriber
	viewAccess permissions.ViewAccessControl
}

// NewSchemaDescribeTool creates a new schema describe tool.
// The describer parameter provides schema description functionality.
// Optional WithSchemaDescribeViewAccess option enables permission checking.
func NewSchemaDescribeTool(describer bichatsql.SchemaDescriber, opts ...SchemaDescribeToolOption) agents.Tool {
	tool := &SchemaDescribeTool{
		describer: describer,
	}

	for _, opt := range opts {
		opt(tool)
	}

	return tool
}

// WithSchemaDescribeViewAccess adds view permission checking to the schema describe tool.
// When configured, the tool will deny access to views the user doesn't have permission for.
func WithSchemaDescribeViewAccess(vac permissions.ViewAccessControl) SchemaDescribeToolOption {
	return func(t *SchemaDescribeTool) {
		t.viewAccess = vac
	}
}

// Name returns the tool name.
func (t *SchemaDescribeTool) Name() string {
	return "schema_describe"
}

// Description returns the tool description for the LLM.
func (t *SchemaDescribeTool) Description() string {
	return "Get detailed column information for a specific table or view. " +
		"Returns column names, types, nullability, and defaults."
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

	// Check view permission if configured
	if t.viewAccess != nil {
		canAccess, err := t.viewAccess.CanAccess(ctx, params.TableName)
		if err != nil {
			return FormatToolError(
				ErrCodeQueryError,
				fmt.Sprintf("failed to check view access: %v", err),
				"Contact administrator if this error persists",
			), serrors.E(op, err)
		}

		if !canAccess {
			// Get user for personalized error message
			user, userErr := composables.UseUser(ctx)
			userName := "User"
			if userErr == nil {
				userName = fmt.Sprintf("%s %s", user.FirstName(), user.LastName())
			}

			// Get required permissions
			requiredPerms := t.viewAccess.GetRequiredPermissions(params.TableName)
			deniedViews := []permissions.DeniedView{{
				Name:                params.TableName,
				RequiredPermissions: requiredPerms,
			}}

			errMsg := permissions.FormatPermissionError(userName, deniedViews)

			return FormatToolError(
				ErrCodePermissionDenied,
				errMsg,
				HintRequestAccess,
				HintCheckAccessibleViews,
			), nil
		}
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

	// Check if any column has a description
	hasDescription := false
	for _, col := range schema.Columns {
		if col.Description != "" {
			hasDescription = true
			break
		}
	}

	// Build markdown table
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## Table: %s (%s)\n\n", schema.Name, schema.Schema))

	// Header
	if hasDescription {
		b.WriteString("| # | Column | Type | Nullable | Default | Description |\n")
		b.WriteString("| --- | --- | --- | --- | --- | --- |\n")
	} else {
		b.WriteString("| # | Column | Type | Nullable | Default |\n")
		b.WriteString("| --- | --- | --- | --- | --- |\n")
	}

	// Rows
	for i, col := range schema.Columns {
		b.WriteString(fmt.Sprintf("| %d | %s | %s | ", i+1, col.Name, col.Type))

		// Nullable
		if col.Nullable {
			b.WriteString("YES | ")
		} else {
			b.WriteString("NO | ")
		}

		// Default
		if col.DefaultValue != nil {
			b.WriteString(fmt.Sprintf("%s ", *col.DefaultValue))
		} else {
			b.WriteString("- ")
		}

		// Description (if column exists)
		if hasDescription {
			b.WriteString("| ")
			if col.Description != "" {
				b.WriteString(col.Description)
			} else {
				b.WriteString("-")
			}
		}

		b.WriteString("|\n")
	}

	// Footer
	b.WriteString(fmt.Sprintf("\n%d column(s)", len(schema.Columns)))

	return b.String(), nil
}
