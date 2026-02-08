package tools

import (
	"context"
	"fmt"
	"regexp"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	bichatctx "github.com/iota-uz/iota-sdk/pkg/bichat/context"
	"github.com/iota-uz/iota-sdk/pkg/bichat/context/formatters"
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

// CallStructured executes the schema list operation and returns a structured result.
func (t *SchemaListTool) CallStructured(ctx context.Context, input string) (*agents.ToolResult, error) {
	const op serrors.Op = "SchemaListTool.CallStructured"

	tables, err := t.lister.SchemaList(ctx)
	if err != nil {
		return &agents.ToolResult{
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
				Code:    string(ErrCodeQueryError),
				Message: fmt.Sprintf("failed to list schema: %v", err),
				Hints:   []string{HintCheckConnection},
			},
		}, serrors.E(op, err, "failed to list schema")
	}

	if len(tables) == 0 {
		return &agents.ToolResult{
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
				Code:    string(ErrCodeNoData),
				Message: "no tables or views found in analytics schema",
				Hints:   []string{"Analytics schema may not be initialized", "Contact administrator to set up analytics views"},
			},
		}, nil // Data condition, not infrastructure failure
	}

	// Check permissions if view access control is configured
	var viewInfos []formatters.ViewAccessInfo
	hasAccess := t.viewAccess != nil
	if t.viewAccess != nil {
		viewNames := make([]string, len(tables))
		for i, table := range tables {
			viewNames[i] = table.Name
		}
		rawInfos, _ := t.viewAccess.GetAccessibleViews(ctx, viewNames)
		for _, info := range rawInfos {
			viewInfos = append(viewInfos, formatters.ViewAccessInfo{Access: string(info.Access)})
		}
	}

	// Build payload
	schemaListTables := make([]formatters.SchemaListTable, len(tables))
	for i, table := range tables {
		schemaListTables[i] = formatters.SchemaListTable{
			Name:        table.Name,
			RowCount:    table.RowCount,
			Description: table.Description,
		}
	}

	return &agents.ToolResult{
		CodecID: formatters.CodecSchemaList,
		Payload: formatters.SchemaListPayload{
			Tables:    schemaListTables,
			ViewInfos: viewInfos,
			HasAccess: hasAccess,
		},
	}, nil
}

// Call executes the schema list operation.
func (t *SchemaListTool) Call(ctx context.Context, input string) (string, error) {
	result, err := t.CallStructured(ctx, input)
	if err != nil {
		if result != nil {
			// Format the result even when there's an error (for error display)
			registry := formatters.DefaultFormatterRegistry()
			if f := registry.Get(result.CodecID); f != nil {
				formatted, fmtErr := f.Format(result.Payload, bichatctx.DefaultFormatOptions())
				if fmtErr == nil {
					return formatted, err
				}
			}
		}
		return "", err
	}
	registry := formatters.DefaultFormatterRegistry()
	f := registry.Get(result.CodecID)
	if f == nil {
		return agents.FormatToolOutput(result.Payload)
	}
	return f.Format(result.Payload, bichatctx.DefaultFormatOptions())
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

// CallStructured executes the schema describe operation and returns a structured result.
func (t *SchemaDescribeTool) CallStructured(ctx context.Context, input string) (*agents.ToolResult, error) {
	const op serrors.Op = "SchemaDescribeTool.CallStructured"

	params, err := agents.ParseToolInput[schemaDescribeInput](input)
	if err != nil {
		return &agents.ToolResult{
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
				Code:    string(ErrCodeInvalidRequest),
				Message: fmt.Sprintf("failed to parse input: %v", err),
				Hints:   []string{HintCheckRequiredFields},
			},
		}, nil // Input validation error, not infrastructure failure
	}

	if params.TableName == "" {
		return &agents.ToolResult{
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
				Code:    string(ErrCodeInvalidRequest),
				Message: "table_name parameter is required",
				Hints:   []string{HintCheckRequiredFields, "Use schema_list to see available tables"},
			},
		}, nil // Input validation error, not infrastructure failure
	}

	if !isValidIdentifier(params.TableName) {
		return &agents.ToolResult{
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
				Code:    string(ErrCodeInvalidRequest),
				Message: fmt.Sprintf("invalid table name '%s': must match pattern ^[a-zA-Z_][a-zA-Z0-9_]*$", params.TableName),
				Hints:   []string{HintCheckFieldFormat, "Table names must start with letter or underscore", "Use schema_list to see valid table names"},
			},
		}, nil // Input validation error, not infrastructure failure
	}

	// Check view permission if configured
	if t.viewAccess != nil {
		canAccess, err := t.viewAccess.CanAccess(ctx, params.TableName)
		if err != nil {
			return &agents.ToolResult{
				CodecID: formatters.CodecToolError,
				Payload: formatters.ToolErrorPayload{
					Code:    string(ErrCodeQueryError),
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

			requiredPerms := t.viewAccess.GetRequiredPermissions(params.TableName)
			deniedViews := []permissions.DeniedView{{
				Name:                params.TableName,
				RequiredPermissions: requiredPerms,
			}}

			errMsg := permissions.FormatPermissionError(userName, deniedViews)

			return &agents.ToolResult{
				CodecID: formatters.CodecToolError,
				Payload: formatters.ToolErrorPayload{
					Code:    string(ErrCodePermissionDenied),
					Message: errMsg,
					Hints:   []string{HintRequestAccess, HintCheckAccessibleViews},
				},
			}, nil
		}
	}

	schema, err := t.describer.SchemaDescribe(ctx, params.TableName)
	if err != nil {
		return &agents.ToolResult{
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
				Code:    string(ErrCodeQueryError),
				Message: fmt.Sprintf("failed to describe schema: %v", err),
				Hints:   []string{HintCheckConnection},
			},
		}, serrors.E(op, err, "failed to describe schema")
	}

	if schema == nil || len(schema.Columns) == 0 {
		return &agents.ToolResult{
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
				Code:    string(ErrCodeNoData),
				Message: fmt.Sprintf("table not found: %s", params.TableName),
				Hints:   []string{HintUseSchemaList, "Check spelling and case sensitivity", "Table must exist in analytics schema"},
			},
		}, nil // Data condition, not infrastructure failure
	}

	// Build payload
	columns := make([]formatters.SchemaDescribeColumn, len(schema.Columns))
	for i, col := range schema.Columns {
		columns[i] = formatters.SchemaDescribeColumn{
			Name:         col.Name,
			Type:         col.Type,
			Nullable:     col.Nullable,
			DefaultValue: col.DefaultValue,
			Description:  col.Description,
		}
	}

	return &agents.ToolResult{
		CodecID: formatters.CodecSchemaDescribe,
		Payload: formatters.SchemaDescribePayload{
			Name:    schema.Name,
			Schema:  schema.Schema,
			Columns: columns,
		},
	}, nil
}

// Call executes the schema describe operation.
func (t *SchemaDescribeTool) Call(ctx context.Context, input string) (string, error) {
	result, err := t.CallStructured(ctx, input)
	if err != nil {
		if result != nil {
			// Format the result even when there's an error (for error display)
			registry := formatters.DefaultFormatterRegistry()
			if f := registry.Get(result.CodecID); f != nil {
				formatted, fmtErr := f.Format(result.Payload, bichatctx.DefaultFormatOptions())
				if fmtErr == nil {
					return formatted, err
				}
			}
		}
		return "", err
	}
	registry := formatters.DefaultFormatterRegistry()
	f := registry.Get(result.CodecID)
	if f == nil {
		return agents.FormatToolOutput(result.Payload)
	}
	return f.Format(result.Payload, bichatctx.DefaultFormatOptions())
}
