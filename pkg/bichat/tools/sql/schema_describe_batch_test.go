package sql

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/bichat/permissions"
	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingDescriber records every requested name and returns table-specific
// schemas / errors based on the supplied maps.
type recordingDescriber struct {
	mu       sync.Mutex
	calls    []string
	schemas  map[string]*bichatsql.TableSchema
	errs     map[string]error
	fallback *bichatsql.TableSchema
}

func (m *recordingDescriber) SchemaDescribe(ctx context.Context, tableName string) (*bichatsql.TableSchema, error) {
	m.mu.Lock()
	m.calls = append(m.calls, tableName)
	m.mu.Unlock()
	if err, ok := m.errs[tableName]; ok && err != nil {
		return nil, err
	}
	if s, ok := m.schemas[tableName]; ok {
		return s, nil
	}
	return m.fallback, nil
}

func (m *recordingDescriber) callsCopy() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.calls))
	copy(out, m.calls)
	return out
}

// fakeViewAccess is a minimal ViewAccessControl mock for the batch tool.
type fakeViewAccess struct {
	denied   map[string]bool
	required map[string][]string
	canErr   error
}

func (f *fakeViewAccess) CanAccess(ctx context.Context, viewName string) (bool, error) {
	if f.canErr != nil {
		return false, f.canErr
	}
	if f.denied[viewName] {
		return false, nil
	}
	return true, nil
}

func (f *fakeViewAccess) GetAccessibleViews(ctx context.Context, views []string) ([]permissions.ViewInfo, error) {
	out := make([]permissions.ViewInfo, len(views))
	for i, v := range views {
		access := "ok"
		if f.denied[v] {
			access = "denied"
		}
		out[i] = permissions.ViewInfo{Name: v, Access: access}
	}
	return out, nil
}

func (f *fakeViewAccess) CheckQueryPermissions(ctx context.Context, sql string) ([]permissions.DeniedView, error) {
	return nil, nil
}

func (f *fakeViewAccess) GetRequiredPermissions(viewName string) []string {
	return f.required[viewName]
}

func TestSchemaDescribeBatchTool_HappyPath(t *testing.T) {
	t.Parallel()

	describer := &recordingDescriber{
		schemas: map[string]*bichatsql.TableSchema{
			"public.users": {
				Name:    "users",
				Schema:  "public",
				Columns: []bichatsql.ColumnInfo{{Name: "id", Type: "integer"}},
			},
			"crm.clients": {
				Name:    "clients",
				Schema:  "crm",
				Columns: []bichatsql.ColumnInfo{{Name: "client_id", Type: "uuid"}},
			},
			"insurance.contracts": {
				Name:    "contracts",
				Schema:  "insurance",
				Columns: []bichatsql.ColumnInfo{{Name: "contract_no", Type: "text"}},
			},
		},
	}
	tool := NewSchemaDescribeBatchTool(describer)

	out, err := tool.Call(context.Background(), `{"table_names": ["public.users", "crm.clients", "insurance.contracts"]}`)
	require.NoError(t, err)

	assert.Contains(t, out, "## public.users")
	assert.Contains(t, out, "| id | integer |")
	assert.Contains(t, out, "## crm.clients")
	assert.Contains(t, out, "| client_id | uuid |")
	assert.Contains(t, out, "## insurance.contracts")
	assert.Contains(t, out, "| contract_no | text |")
	assert.Contains(t, out, "---", "section separator")
}

func TestSchemaDescribeBatchTool_EmptyArrayRejected(t *testing.T) {
	t.Parallel()

	describer := &recordingDescriber{}
	tool := NewSchemaDescribeBatchTool(describer)

	out, err := tool.Call(context.Background(), `{"table_names": []}`)
	require.NoError(t, err)
	assert.Contains(t, out, "INVALID_REQUEST")
}

func TestSchemaDescribeBatchTool_AllWhitespaceTrimsToEmpty(t *testing.T) {
	t.Parallel()

	describer := &recordingDescriber{}
	tool := NewSchemaDescribeBatchTool(describer)

	out, err := tool.Call(context.Background(), `{"table_names": [" ", ""]}`)
	require.NoError(t, err)
	assert.Contains(t, out, "INVALID_REQUEST")
	assert.Empty(t, describer.callsCopy(), "no describer calls when all names are blank")
}

func TestSchemaDescribeBatchTool_InvalidIdentifier(t *testing.T) {
	t.Parallel()

	describer := &recordingDescriber{}
	tool := NewSchemaDescribeBatchTool(describer)

	out, err := tool.Call(context.Background(), `{"table_names": ["users; DROP TABLE users;--", "ok_table"]}`)
	require.NoError(t, err)
	assert.Contains(t, out, "INVALID_REQUEST")
	assert.Contains(t, out, "invalid table name")
	assert.Empty(t, describer.callsCopy(), "no describes run when validation fails")
}

func TestSchemaDescribeBatchTool_DeduplicatesNames(t *testing.T) {
	t.Parallel()

	describer := &recordingDescriber{
		fallback: &bichatsql.TableSchema{
			Name:    "t",
			Schema:  "public",
			Columns: []bichatsql.ColumnInfo{{Name: "id", Type: "integer"}},
		},
	}
	tool := NewSchemaDescribeBatchTool(describer)

	_, err := tool.Call(context.Background(), `{"table_names": ["a", "a", " a ", "b"]}`)
	require.NoError(t, err)

	calls := describer.callsCopy()
	require.Len(t, calls, 2, "duplicates should collapse")

	got := map[string]bool{}
	for _, c := range calls {
		got[c] = true
	}
	assert.True(t, got["a"])
	assert.True(t, got["b"])
}

func TestSchemaDescribeBatchTool_PartialFailure(t *testing.T) {
	t.Parallel()

	describer := &recordingDescriber{
		schemas: map[string]*bichatsql.TableSchema{
			"good": {
				Name:    "good",
				Schema:  "public",
				Columns: []bichatsql.ColumnInfo{{Name: "id", Type: "integer"}},
			},
		},
		errs: map[string]error{
			"bad": errors.New("connection reset"),
		},
	}
	tool := NewSchemaDescribeBatchTool(describer)

	out, err := tool.Call(context.Background(), `{"table_names": ["good", "bad"]}`)
	require.NoError(t, err)

	assert.Contains(t, out, "## good")
	assert.Contains(t, out, "| id | integer |")
	assert.Contains(t, out, "## bad")
	assert.Contains(t, out, "error: connection reset")
}

func TestSchemaDescribeBatchTool_AllFailedReportsToolError(t *testing.T) {
	t.Parallel()

	describer := &recordingDescriber{
		errs: map[string]error{
			"a": errors.New("boom-a"),
			"b": errors.New("boom-b"),
		},
	}
	tool := NewSchemaDescribeBatchTool(describer)

	out, err := tool.Call(context.Background(), `{"table_names": ["a", "b"]}`)
	require.NoError(t, err)
	assert.Contains(t, out, "QUERY_ERROR")
	assert.Contains(t, out, "failed to describe any of the requested tables")
	assert.Contains(t, out, "boom-a")
	assert.Contains(t, out, "boom-b")
}

func TestSchemaDescribeBatchTool_TableNotFoundIsPerEntryError(t *testing.T) {
	t.Parallel()

	describer := &recordingDescriber{
		schemas: map[string]*bichatsql.TableSchema{
			"present": {
				Name:    "present",
				Schema:  "public",
				Columns: []bichatsql.ColumnInfo{{Name: "id", Type: "integer"}},
			},
			"missing": nil,
		},
	}
	tool := NewSchemaDescribeBatchTool(describer)

	out, err := tool.Call(context.Background(), `{"table_names": ["present", "missing"]}`)
	require.NoError(t, err)

	assert.Contains(t, out, "## present")
	assert.Contains(t, out, "## missing")
	assert.Contains(t, out, "error: table not found: missing")
}

func TestSchemaDescribeBatchTool_ViewAccessDeniesEntireBatch(t *testing.T) {
	t.Parallel()

	describer := &recordingDescriber{
		fallback: &bichatsql.TableSchema{
			Name:    "ok",
			Schema:  "public",
			Columns: []bichatsql.ColumnInfo{{Name: "id", Type: "integer"}},
		},
	}
	vac := &fakeViewAccess{
		denied:   map[string]bool{"forbidden_view": true},
		required: map[string][]string{"forbidden_view": {"Reports.Read"}},
	}
	tool := NewSchemaDescribeBatchTool(describer, WithSchemaDescribeBatchViewAccess(vac))

	out, err := tool.Call(context.Background(), `{"table_names": ["ok_table", "forbidden_view"]}`)
	require.NoError(t, err)

	assert.Contains(t, out, "PERMISSION_DENIED")
	assert.Contains(t, out, "forbidden_view")
	assert.Empty(t, describer.callsCopy(), "view access denial should short-circuit before describing")
}

func TestSchemaDescribeBatchTool_ViewAccessAllowed(t *testing.T) {
	t.Parallel()

	describer := &recordingDescriber{
		fallback: &bichatsql.TableSchema{
			Name:    "ok",
			Schema:  "public",
			Columns: []bichatsql.ColumnInfo{{Name: "id", Type: "integer"}},
		},
	}
	vac := &fakeViewAccess{}
	tool := NewSchemaDescribeBatchTool(describer, WithSchemaDescribeBatchViewAccess(vac))

	out, err := tool.Call(context.Background(), `{"table_names": ["ok_table", "schema.qualified"]}`)
	require.NoError(t, err)
	assert.Contains(t, out, "## ok_table")
	assert.Contains(t, out, "## schema.qualified")
}

func TestSchemaDescribeBatchTool_ParametersSchema(t *testing.T) {
	t.Parallel()

	tool := NewSchemaDescribeBatchTool(&recordingDescriber{})
	params := tool.Parameters()

	require.Equal(t, "object", params["type"])
	props, ok := params["properties"].(map[string]any)
	require.True(t, ok)
	tableNames, ok := props["table_names"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "array", tableNames["type"])
	assert.Equal(t, 1, tableNames["minItems"])

	required, ok := params["required"].([]string)
	require.True(t, ok)
	assert.Equal(t, []string{"table_names"}, required)
}

func TestSchemaDescribeBatchTool_Name(t *testing.T) {
	t.Parallel()
	tool := NewSchemaDescribeBatchTool(&recordingDescriber{})
	assert.Equal(t, "schema_describe_batch", tool.Name())
	assert.NotEmpty(t, tool.Description())
	assert.Contains(t, tool.Description(), "Describe multiple")
}
