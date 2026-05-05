// Package sql provides this package.
package sql

import (
	"context"
	"fmt"
	"regexp"

	tools "github.com/iota-uz/iota-sdk/pkg/bichat/tools"

	"github.com/iota-uz/iota-sdk/pkg/bichat/permissions"
	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// validIdentifierPattern validates SQL identifiers, optionally schema-qualified (e.g., "schema.table").
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
	return "List all available tables and views with approximate row counts. " +
		"Call this before writing SQL to see what's accessible. Pay attention to foreign keys and indexes for optimal query performance."
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
				Message: "no tables or views accessible to the current role",
				Hints:   []string{"Verify schema allowlist is configured", "Check that the current role has SELECT grants"},
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

