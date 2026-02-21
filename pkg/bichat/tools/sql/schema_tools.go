package sql

import (
	tools "github.com/iota-uz/iota-sdk/pkg/bichat/tools"
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/permissions"
	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// validIdentifierPattern validates SQL identifiers, optionally schema-qualified (e.g., "analytics.policies").
var validIdentifierPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*(\.[a-zA-Z_][a-zA-Z0-9_]*)?$`)

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
func NewSchemaListTool(lister bichatsql.SchemaLister, opts ...SchemaListToolOption) *SchemaListTool {
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

// CallStructured executes the schema list operation and returns a structured result.
func (t *SchemaListTool) CallStructured(ctx context.Context, input string) (*types.ToolResult, error) {
	const op serrors.Op = "SchemaListTool.CallStructured"

	tables, err := t.lister.SchemaList(ctx)
	if err != nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeQueryError),
				Message: fmt.Sprintf("failed to list schema: %v", err),
				Hints:   []string{tools.HintCheckConnection},
			},
		}, serrors.E(op, err, "failed to list schema")
	}

	if len(tables) == 0 {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeNoData),
				Message: "no tables or views found in analytics schema",
				Hints:   []string{"Analytics schema may not be initialized", "Contact administrator to set up analytics views"},
			},
		}, nil // Data condition, not infrastructure failure
	}

	// Check permissions if view access control is configured
	var viewInfos []types.ViewAccessInfo
	hasAccess := t.viewAccess != nil
	if t.viewAccess != nil {
		viewNames := make([]string, len(tables))
		for i, table := range tables {
			viewNames[i] = table.Name
		}
		rawInfos, err := t.viewAccess.GetAccessibleViews(ctx, viewNames)
		if err != nil {
			return &types.ToolResult{
				CodecID: types.CodecToolError,
				Payload: types.ToolErrorPayload{
					Code:    string(tools.ErrCodeQueryError),
					Message: fmt.Sprintf("failed to check view access: %v", err),
					Hints:   []string{"Contact administrator if this error persists"},
				},
			}, serrors.E(op, err)
		}
		for _, info := range rawInfos {
			viewInfos = append(viewInfos, types.ViewAccessInfo{Access: info.Access})
		}
	}

	// Build payload
	schemaListTables := make([]types.SchemaListTable, len(tables))
	for i, table := range tables {
		name := table.Name
		if table.Schema != "" {
			name = table.Schema + "." + table.Name
		}
		schemaListTables[i] = types.SchemaListTable{
			Name:        name,
			RowCount:    table.RowCount,
			Description: table.Description,
		}
	}

	return &types.ToolResult{
		CodecID: types.CodecSchemaList,
		Payload: types.SchemaListPayload{
			Tables:    schemaListTables,
			ViewInfos: viewInfos,
			HasAccess: hasAccess,
		},
	}, nil
}

// Call executes the schema list operation.
func (t *SchemaListTool) Call(ctx context.Context, input string) (string, error) {
	return tools.FormatStructuredResult(t.CallStructured(ctx, input))
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
func NewSchemaDescribeTool(describer bichatsql.SchemaDescriber, opts ...SchemaDescribeToolOption) *SchemaDescribeTool {
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
				"description": "Schema-qualified name of the table or view (e.g., 'analytics.policies_with_details')",
			},
		},
		"required": []string{"table_name"},
	}
}

// schemaDescribeInput represents the parsed input parameters.
type schemaDescribeInput struct {
	TableName string `json:"table_name"`
}

// CallStructured executes the schema describe operation and returns a structured result.
func (t *SchemaDescribeTool) CallStructured(ctx context.Context, input string) (*types.ToolResult, error) {
	const op serrors.Op = "SchemaDescribeTool.CallStructured"

	params, err := agents.ParseToolInput[schemaDescribeInput](input)
	if err != nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: fmt.Sprintf("failed to parse input: %v", err),
				Hints:   []string{tools.HintCheckRequiredFields},
			},
		}, nil // Input validation error, not infrastructure failure
	}

	if params.TableName == "" {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: "table_name parameter is required",
				Hints:   []string{tools.HintCheckRequiredFields, "Use schema_list to see available tables"},
			},
		}, nil // Input validation error, not infrastructure failure
	}

	if !isValidIdentifier(params.TableName) {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: fmt.Sprintf("invalid table name '%s': use schema-qualified format like 'analytics.table_name'", params.TableName),
				Hints:   []string{tools.HintCheckFieldFormat, tools.HintUseSchemaList},
			},
		}, nil // Input validation error, not infrastructure failure
	}

	// Strip schema prefix for underlying calls (describer/permissions expect bare names)
	bareName := params.TableName
	if idx := strings.Index(params.TableName, "."); idx >= 0 {
		bareName = params.TableName[idx+1:]
	}

	// Check view permission if configured
	if t.viewAccess != nil {
		canAccess, err := t.viewAccess.CanAccess(ctx, bareName)
		if err != nil {
			return &types.ToolResult{
				CodecID: types.CodecToolError,
				Payload: types.ToolErrorPayload{
					Code:    string(tools.ErrCodeQueryError),
					Message: fmt.Sprintf("failed to check view access: %v", err),
					Hints:   []string{"Contact administrator if this error persists"},
				},
			}, serrors.E(op, err)
		}

		if !canAccess {
			user, userErr := composables.UseUser(ctx)
			userName := "User"
			if userErr == nil {
				userName = fmt.Sprintf("%s %s", user.FirstName(), user.LastName())
			}

			requiredPerms := t.viewAccess.GetRequiredPermissions(bareName)
			deniedViews := []permissions.DeniedView{{
				Name:                bareName,
				RequiredPermissions: requiredPerms,
			}}

			errMsg := permissions.FormatPermissionError(userName, deniedViews)

			return &types.ToolResult{
				CodecID: types.CodecToolError,
				Payload: types.ToolErrorPayload{
					Code:    string(tools.ErrCodePermissionDenied),
					Message: errMsg,
					Hints:   []string{tools.HintRequestAccess, tools.HintCheckAccessibleViews},
				},
			}, nil
		}
	}

	schema, err := t.describer.SchemaDescribe(ctx, bareName)
	if err != nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeQueryError),
				Message: fmt.Sprintf("failed to describe schema: %v", err),
				Hints:   []string{tools.HintCheckConnection},
			},
		}, serrors.E(op, err, "failed to describe schema")
	}

	if schema == nil || len(schema.Columns) == 0 {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeNoData),
				Message: fmt.Sprintf("table not found: %s", bareName),
				Hints:   []string{tools.HintUseSchemaList, "Check spelling and case sensitivity", "Table must exist in analytics schema"},
			},
		}, nil // Data condition, not infrastructure failure
	}

	// Build payload
	columns := make([]types.SchemaDescribeColumn, len(schema.Columns))
	for i, col := range schema.Columns {
		columns[i] = types.SchemaDescribeColumn{
			Name:         col.Name,
			Type:         col.Type,
			Nullable:     col.Nullable,
			DefaultValue: col.DefaultValue,
			Description:  col.Description,
		}
	}

	return &types.ToolResult{
		CodecID: types.CodecSchemaDescribe,
		Payload: types.SchemaDescribePayload{
			Name:    schema.Name,
			Schema:  schema.Schema,
			Columns: columns,
		},
	}, nil
}

// Call executes the schema describe operation.
func (t *SchemaDescribeTool) Call(ctx context.Context, input string) (string, error) {
	return tools.FormatStructuredResult(t.CallStructured(ctx, input))
}
