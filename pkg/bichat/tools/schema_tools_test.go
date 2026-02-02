package tools

import (
	"context"
	"strings"
	"testing"
)

// mockSchemaExecutor implements QueryExecutorService for testing.
type mockSchemaExecutor struct {
	columnsResult *QueryResult
	sampleResult  *QueryResult
	countResult   *QueryResult
	indexResult   *QueryResult
	err           error
}

func (m *mockSchemaExecutor) ExecuteQuery(ctx context.Context, sql string, params []any, timeoutMs int) (*QueryResult, error) {
	if m.err != nil {
		return nil, m.err
	}

	// Route to appropriate result based on query content
	if strings.Contains(sql, "pg_catalog.pg_views") {
		// Schema list query
		return m.columnsResult, nil
	}
	if strings.Contains(sql, "information_schema.columns") {
		return m.columnsResult, nil
	}
	if strings.Contains(sql, "COUNT(*)") {
		return m.countResult, nil
	}
	if strings.Contains(sql, "pg_index") {
		return m.indexResult, nil
	}
	if strings.Contains(sql, "LIMIT 4") {
		return m.sampleResult, nil
	}

	return &QueryResult{Columns: []string{}, Rows: []map[string]interface{}{}}, nil
}

func TestFormatSampleDataTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		result   *QueryResult
		wantRows int
	}{
		{
			name: "empty result",
			result: &QueryResult{
				Columns:  []string{},
				Rows:     []map[string]interface{}{},
				RowCount: 0,
			},
			wantRows: 0,
		},
		{
			name: "with data rows",
			result: &QueryResult{
				Columns: []string{"id", "name", "amount"},
				Rows: []map[string]interface{}{
					{"id": int64(1), "name": "Alice", "amount": 100.5},
					{"id": int64(2), "name": "Bob", "amount": 200.0},
				},
				RowCount: 2,
			},
			wantRows: 2,
		},
		{
			name: "with null values",
			result: &QueryResult{
				Columns: []string{"id", "name", "amount"},
				Rows: []map[string]interface{}{
					{"id": int64(1), "name": "Alice", "amount": nil},
					{"id": int64(2), "name": nil, "amount": 200.0},
				},
				RowCount: 2,
			},
			wantRows: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := formatSampleDataTable(tt.result)

			if tt.wantRows == 0 {
				if result != "No sample data available." {
					t.Errorf("expected 'No sample data available.', got: %s", result)
				}
				return
			}

			// Verify markdown format
			if !strings.Contains(result, "|") {
				t.Errorf("expected markdown table format, got: %s", result)
			}

			// Verify header row
			for _, col := range tt.result.Columns {
				if !strings.Contains(result, col) {
					t.Errorf("expected column %s in output, got: %s", col, result)
				}
			}

			// Verify separator row
			if !strings.Contains(result, "---") {
				t.Errorf("expected separator row with ---, got: %s", result)
			}

			// Count rows (header + separator + data rows)
			lines := strings.Split(strings.TrimSpace(result), "\n")
			expectedLines := 2 + tt.wantRows // header + separator + data
			if len(lines) != expectedLines {
				t.Errorf("expected %d lines, got %d", expectedLines, len(lines))
			}
		})
	}
}

func TestSchemaDescribeToolWithSampleData(t *testing.T) {
	t.Parallel()

	executor := &mockSchemaExecutor{
		columnsResult: &QueryResult{
			Columns: []string{"column_name", "data_type", "is_nullable"},
			Rows: []map[string]interface{}{
				{"column_name": "id", "data_type": "integer", "is_nullable": "NO"},
				{"column_name": "name", "data_type": "text", "is_nullable": "YES"},
			},
			RowCount: 2,
		},
		sampleResult: &QueryResult{
			Columns: []string{"id", "name"},
			Rows: []map[string]interface{}{
				{"id": int64(1), "name": "Alice"},
				{"id": int64(2), "name": "Bob"},
			},
			RowCount: 2,
		},
		countResult: &QueryResult{
			Columns: []string{"row_count"},
			Rows: []map[string]interface{}{
				{"row_count": int64(150000)},
			},
			RowCount: 1,
		},
		indexResult: &QueryResult{
			Columns: []string{"index_name", "column_name"},
			Rows: []map[string]interface{}{
				{"index_name": "idx_users_id", "column_name": "id"},
			},
			RowCount: 1,
		},
	}

	tool := NewSchemaDescribeTool(executor)

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

	executor := &mockSchemaExecutor{
		columnsResult: &QueryResult{
			Columns: []string{"column_name", "data_type"},
			Rows: []map[string]interface{}{
				{"column_name": "id", "data_type": "integer"},
			},
			RowCount: 1,
		},
		sampleResult: &QueryResult{
			Columns:  []string{"id"},
			Rows:     []map[string]interface{}{{"id": int64(1)}},
			RowCount: 1,
		},
		countResult: &QueryResult{
			Columns: []string{"row_count"},
			Rows: []map[string]interface{}{
				{"row_count": int64(2000000)}, // > 1M rows
			},
			RowCount: 1,
		},
		indexResult: &QueryResult{
			Columns:  []string{"index_name", "column_name"},
			Rows:     []map[string]interface{}{},
			RowCount: 0,
		},
	}

	tool := NewSchemaDescribeTool(executor)

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

			executor := &mockSchemaExecutor{}
			tool := NewSchemaDescribeTool(executor)

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

	executor := &mockSchemaExecutor{
		columnsResult: &QueryResult{
			Columns: []string{"schema", "name", "type"},
			Rows: []map[string]interface{}{
				{"schema": "analytics", "name": "policies", "type": "view"},
				{"schema": "analytics", "name": "payments", "type": "view"},
			},
			RowCount: 2,
		},
	}

	tool := NewSchemaListTool(executor)

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
