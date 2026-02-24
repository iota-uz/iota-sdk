package sql

import (
	"context"
	"testing"

	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSchemaDescriber implements bichatsql.SchemaDescriber for testing.
type mockSchemaDescriber struct {
	schema *bichatsql.TableSchema
	err    error
}

func (m *mockSchemaDescriber) SchemaDescribe(ctx context.Context, tableName string) (*bichatsql.TableSchema, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.schema != nil {
		return m.schema, nil
	}
	return &bichatsql.TableSchema{
		Name:   tableName,
		Schema: "public",
		Columns: []bichatsql.ColumnInfo{
			{Name: "id", Type: "integer", IsPrimaryKey: true},
			{Name: "name", Type: "text"},
		},
	}, nil
}

// mockSchemaLister implements bichatsql.SchemaLister for testing.
type mockSchemaLister struct {
	tables []bichatsql.TableInfo
	err    error
}

func (m *mockSchemaLister) SchemaList(ctx context.Context) ([]bichatsql.TableInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.tables != nil {
		return m.tables, nil
	}
	return []bichatsql.TableInfo{
		{Name: "users", Schema: "public", RowCount: 100, Description: "User accounts"},
		{Name: "orders", Schema: "public", RowCount: 1000, Description: "Customer orders"},
	}, nil
}

// TestFormatSampleDataTable removed - formatSampleDataTable function was removed in Phase 6.
// Schema describe now uses sql.SchemaDescriber interface directly.

func TestSchemaDescribeToolWithSampleData(t *testing.T) {
	t.Parallel()

	describer := &mockSchemaDescriber{
		schema: &bichatsql.TableSchema{
			Name:   "users",
			Schema: "public",
			Columns: []bichatsql.ColumnInfo{
				{Name: "id", Type: "integer", IsPrimaryKey: true},
				{Name: "name", Type: "text"},
			},
		},
	}

	tool := NewSchemaDescribeTool(describer)

	input := `{"table_name": "users"}`
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)

	assert.Contains(t, result, "| Column | Type |", "markdown table header")
	assert.Contains(t, result, "| id | integer |", "first column row")
	assert.Contains(t, result, "| name | text |", "second column row")
}

func TestSchemaDescribeToolLargeDataset(t *testing.T) {
	t.Parallel()

	describer := &mockSchemaDescriber{
		schema: &bichatsql.TableSchema{
			Name:   "large_table",
			Schema: "public",
			Columns: []bichatsql.ColumnInfo{
				{Name: "id", Type: "integer", IsPrimaryKey: true},
			},
		},
	}

	tool := NewSchemaDescribeTool(describer)

	input := `{"table_name": "large_table"}`
	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)

	assert.Contains(t, result, "| Column | Type |", "markdown table header")
	assert.Contains(t, result, "| id | integer |", "id column row")
}

func TestSchemaDescribeToolInvalidTableName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		tableName string
	}{
		{
			name:      "contains space",
			tableName: "invalid table",
		},
		{
			name:      "contains special chars",
			tableName: "invalid@table",
		},
		{
			name:      "starts with number",
			tableName: "123table",
		},
		{
			name:      "sql injection attempt",
			tableName: "users; DROP TABLE users;--",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			describer := &mockSchemaDescriber{}
			tool := NewSchemaDescribeTool(describer)

			input := `{"table_name": "` + tt.tableName + `"}`
			result, err := tool.Call(context.Background(), input)

			require.NoError(t, err, "validation failure returns nil error with details in result")
			assert.Contains(t, result, "INVALID_REQUEST", "error format")
			assert.Contains(t, result, "invalid table name", "error message")
		})
	}
}

func TestSchemaListTool(t *testing.T) {
	t.Parallel()

	lister := &mockSchemaLister{
		tables: []bichatsql.TableInfo{
			{Name: "policies", Schema: "analytics", RowCount: 100, Description: "Policy view"},
			{Name: "payments", Schema: "analytics", RowCount: 200, Description: "Payment view"},
		},
	}

	tool := NewSchemaListTool(lister)

	result, err := tool.Call(context.Background(), `{}`)
	require.NoError(t, err)

	assert.Contains(t, result, "policies")
	assert.Contains(t, result, "payments")
	assert.Contains(t, result, "~100")
	assert.Contains(t, result, "~200")
}
