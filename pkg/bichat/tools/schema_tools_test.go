package tools

import (
	"context"
	"strings"
	"testing"

	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
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
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}

	// Verify result contains expected data
	if !strings.Contains(result, "table_name") {
		t.Errorf("expected 'table_name' in result, got: %s", result)
	}
	if !strings.Contains(result, "users") {
		t.Errorf("expected 'users' in result, got: %s", result)
	}
	if !strings.Contains(result, "sample_rows") {
		t.Errorf("expected 'sample_rows' in result, got: %s", result)
	}
	if !strings.Contains(result, "sample_data_table") {
		t.Errorf("expected 'sample_data_table' in result, got: %s", result)
	}
	if !strings.Contains(result, "statistics") {
		t.Errorf("expected 'statistics' in result, got: %s", result)
	}
	if !strings.Contains(result, "total_rows") {
		t.Errorf("expected 'total_rows' in result, got: %s", result)
	}
	if !strings.Contains(result, "has_large_dataset") {
		t.Errorf("expected 'has_large_dataset' in result, got: %s", result)
	}
	if !strings.Contains(result, "sample_representative") {
		t.Errorf("expected 'sample_representative' in result, got: %s", result)
	}
	if !strings.Contains(result, "indexed_columns") {
		t.Errorf("expected 'indexed_columns' in result, got: %s", result)
	}
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
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}

	// Verify statistics indicate large dataset
	if !strings.Contains(result, "has_large_dataset") {
		t.Errorf("expected 'has_large_dataset' in result")
	}
	if !strings.Contains(result, "sample_representative") {
		t.Errorf("expected 'sample_representative' in result")
	}
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

			// Should return error response
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			// Verify error format
			if !strings.Contains(result, "INVALID_REQUEST") {
				t.Errorf("expected INVALID_REQUEST error, got: %s", result)
			}

			if !strings.Contains(result, "invalid table name") {
				t.Errorf("expected 'invalid table name' message, got: %s", result)
			}
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
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}

	// Verify result contains view information
	if !strings.Contains(result, "policies") {
		t.Errorf("expected 'policies' in result, got: %s", result)
	}
	if !strings.Contains(result, "payments") {
		t.Errorf("expected 'payments' in result, got: %s", result)
	}
}
