package sql

import (
	"context"
	"fmt"
	"strings"
	"sync"

	tools "github.com/iota-uz/iota-sdk/pkg/bichat/tools"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/permissions"
	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	// schemaDescribeBatchMaxParallel bounds concurrency for parallel describes.
	schemaDescribeBatchMaxParallel = 8
	// schemaDescribeBatchMaxTables caps how many tables a single call may
	// request. Generous for any real workflow; protects against pathological
	// inputs that would otherwise spawn unbounded goroutines.
	schemaDescribeBatchMaxTables = 64
)

// SchemaDescribeBatchToolOption configures a SchemaDescribeBatchTool.
type SchemaDescribeBatchToolOption func(*SchemaDescribeBatchTool)

// SchemaDescribeBatchTool describes multiple tables/views in one call by
// fanning out concurrent SchemaDescribe lookups. Per-table failures are
// reported inline; the batch only fails when no table succeeds (or when
// validation/permission checks reject the request as a whole).
type SchemaDescribeBatchTool struct {
	describer  bichatsql.SchemaDescriber
	viewAccess permissions.ViewAccessControl
}

// NewSchemaDescribeBatchTool creates a new batched schema describe tool.
// The describer parameter provides schema description functionality.
// Optional WithSchemaDescribeBatchViewAccess option enables permission checking.
func NewSchemaDescribeBatchTool(describer bichatsql.SchemaDescriber, opts ...SchemaDescribeBatchToolOption) *SchemaDescribeBatchTool {
	tool := &SchemaDescribeBatchTool{
		describer: describer,
	}

	for _, opt := range opts {
		opt(tool)
	}

	return tool
}

// WithSchemaDescribeBatchViewAccess adds view permission checking to the
// batched schema describe tool. When configured, the tool will deny the
// entire call if any of the requested tables is a view the user cannot access.
func WithSchemaDescribeBatchViewAccess(vac permissions.ViewAccessControl) SchemaDescribeBatchToolOption {
	return func(t *SchemaDescribeBatchTool) {
		t.viewAccess = vac
	}
}

// Name returns the tool name.
func (t *SchemaDescribeBatchTool) Name() string {
	return "schema_describe_batch"
}

// Description returns the tool description for the LLM.
func (t *SchemaDescribeBatchTool) Description() string {
	return "Describe multiple tables or views in one call. Pass table_names as an array of " +
		"names (schema qualification optional, e.g. [\"insurance.contracts\", \"insurance.products\"]). " +
		"Prefer this over multiple individual schema describe calls when you already know which " +
		"tables you need. Returns column information (names, types, nullability, defaults, descriptions) " +
		"for every requested table; per-table errors are reported inline so partial successes are still useful."
}

// Parameters returns the JSON Schema for tool parameters.
func (t *SchemaDescribeBatchTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"table_names": map[string]any{
				"type":        "array",
				"minItems":    1,
				"items":       map[string]any{"type": "string"},
				"description": "Table or view names. Schema qualification is optional.",
			},
		},
		"required": []string{"table_names"},
	}
}

// schemaDescribeBatchInput represents the parsed input parameters.
type schemaDescribeBatchInput struct {
	TableNames []string `json:"table_names"`
}

// CallStructured executes the batched schema describe operation and returns a structured result.
func (t *SchemaDescribeBatchTool) CallStructured(ctx context.Context, input string) (*types.ToolResult, error) {
	const op serrors.Op = "SchemaDescribeBatchTool.CallStructured"

	params, err := agents.ParseToolInput[schemaDescribeBatchInput](input)
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

	// Trim, drop empty, dedupe (preserve first-seen order).
	names := make([]string, 0, len(params.TableNames))
	seen := make(map[string]struct{}, len(params.TableNames))
	for _, n := range params.TableNames {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		names = append(names, n)
	}

	if len(names) == 0 {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: "table_names is required and must contain at least one non-empty entry",
				Hints:   []string{tools.HintCheckRequiredFields, "Use schema_list to see available tables"},
			},
		}, nil
	}

	if len(names) > schemaDescribeBatchMaxTables {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code: string(tools.ErrCodeInvalidRequest),
				Message: fmt.Sprintf(
					"too many table_names: got %d, max %d per call",
					len(names), schemaDescribeBatchMaxTables,
				),
				Hints: []string{"Split the request into multiple calls"},
			},
		}, nil
	}

	// Validate identifiers (collect all bad names so the LLM can fix in one shot).
	var invalid []string
	for _, n := range names {
		if !isValidIdentifier(n) {
			invalid = append(invalid, n)
		}
	}
	if len(invalid) > 0 {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code: string(tools.ErrCodeInvalidRequest),
				Message: fmt.Sprintf(
					"invalid table name(s) %s: use bare identifiers ('table_name') or schema-qualified form ('schema.table_name')",
					strings.Join(quoteAll(invalid), ", "),
				),
				Hints: []string{tools.HintCheckFieldFormat, tools.HintUseSchemaList},
			},
		}, nil
	}

	// Resolve bare names (drop schema prefix) for view access checks and result keys.
	bareNames := make([]string, len(names))
	for i, n := range names {
		bareNames[i] = bareIdentifier(n)
	}

	// View access: if configured, deny the entire batch when any name is a
	// view the caller cannot access. Mirrors single-table behavior.
	if t.viewAccess != nil {
		var denied []permissions.DeniedView
		for _, bare := range bareNames {
			canAccess, err := t.viewAccess.CanAccess(ctx, bare)
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
				denied = append(denied, permissions.DeniedView{
					Name:                bare,
					RequiredPermissions: t.viewAccess.GetRequiredPermissions(bare),
				})
			}
		}
		if len(denied) > 0 {
			user, userErr := composables.UseUser(ctx)
			userName := "User"
			if userErr == nil {
				userName = fmt.Sprintf("%s %s", user.FirstName(), user.LastName())
			}
			errMsg := permissions.FormatPermissionError(userName, denied)
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

	// Fan out describes with bounded parallelism. Acquire the semaphore in
	// the parent goroutine *before* launching, so we never hold more than
	// schemaDescribeBatchMaxParallel goroutines in flight regardless of how
	// many names came in.
	entries := make([]types.SchemaDescribeBatchEntry, len(names))
	sem := make(chan struct{}, schemaDescribeBatchMaxParallel)
	var wg sync.WaitGroup
	for i, n := range names {
		sem <- struct{}{}
		wg.Add(1)
		go func(i int, requested string) {
			defer wg.Done()
			defer func() { <-sem }()

			entry := types.SchemaDescribeBatchEntry{Requested: requested}
			schema, derr := t.describer.SchemaDescribe(ctx, requested)
			switch {
			case derr != nil:
				entry.Error = derr.Error()
			case schema == nil || len(schema.Columns) == 0:
				entry.Error = fmt.Sprintf("table not found: %s", bareIdentifier(requested))
				entry.NotFound = true
			default:
				entry.Name = schema.Name
				entry.Schema = schema.Schema
				cols := make([]types.SchemaDescribeColumn, len(schema.Columns))
				for j, c := range schema.Columns {
					cols[j] = types.SchemaDescribeColumn{
						Name:         c.Name,
						Type:         c.Type,
						Nullable:     c.Nullable,
						DefaultValue: c.DefaultValue,
						Description:  c.Description,
					}
				}
				entry.Columns = cols
			}
			entries[i] = entry
		}(i, n)
	}
	wg.Wait()

	// Decide whether the batch as a whole is unusable. A purely "table not
	// found" outcome is a normal LLM-recoverable result (matches the single
	// tool's TABLE_NOT_FOUND), while any actual describer error means we
	// surface QUERY_ERROR so the model treats it as infrastructure trouble.
	anyOK, anyHardError := false, false
	for _, e := range entries {
		switch {
		case e.Error == "":
			anyOK = true
		case !e.NotFound:
			anyHardError = true
		}
	}
	if !anyOK {
		var msgs []string
		for _, e := range entries {
			msgs = append(msgs, fmt.Sprintf("%s: %s", e.Requested, e.Error))
		}
		code := tools.ErrCodeTableNotFound
		message := "none of the requested tables were found: " + strings.Join(msgs, "; ")
		hints := []string{tools.HintUseSchemaList}
		if anyHardError {
			code = tools.ErrCodeQueryError
			message = "failed to describe any of the requested tables: " + strings.Join(msgs, "; ")
			hints = []string{tools.HintCheckConnection, tools.HintUseSchemaList}
		}
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(code),
				Message: message,
				Hints:   hints,
			},
		}, nil
	}

	return &types.ToolResult{
		CodecID: types.CodecSchemaDescribeBatch,
		Payload: types.SchemaDescribeBatchPayload{
			Tables: entries,
		},
	}, nil
}

// Call executes the batched schema describe operation.
func (t *SchemaDescribeBatchTool) Call(ctx context.Context, input string) (string, error) {
	return tools.FormatStructuredResult(t.CallStructured(ctx, input))
}

// bareIdentifier strips an optional schema prefix from an identifier.
func bareIdentifier(name string) string {
	if idx := strings.Index(name, "."); idx >= 0 {
		return name[idx+1:]
	}
	return name
}

// quoteAll returns a copy of names with each entry wrapped in single quotes
// for stable error message formatting.
func quoteAll(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = "'" + n + "'"
	}
	return out
}
