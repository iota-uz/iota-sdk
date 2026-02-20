package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/excel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockQueryExecutor is a test double for bichatsql.QueryExecutor
type mockQueryExecutor struct {
	result *bichatsql.QueryResult
	err    error
}

func (m *mockQueryExecutor) ExecuteQuery(ctx context.Context, sql string, params []any, timeout time.Duration) (*bichatsql.QueryResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func TestExportQueryToExcelTool_Parameters(t *testing.T) {
	tmpDir := t.TempDir()
	executor := &mockQueryExecutor{}

	tool := NewExportQueryToExcelTool(
		executor,
		WithQueryOutputDir(tmpDir),
		WithQueryBaseURL("http://test.com/exports"),
	)

	params := tool.Parameters()
	assert.NotNil(t, params)

	// Verify sql is required
	required, ok := params["required"].([]string)
	require.True(t, ok)
	assert.Contains(t, required, "sql")

	// Verify properties
	props, ok := params["properties"].(map[string]any)
	require.True(t, ok)

	sql, ok := props["sql"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "string", sql["type"])

	filename, ok := props["filename"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "string", filename["type"])
	assert.Equal(t, "export.xlsx", filename["default"])
}

func TestExportQueryToExcelTool_Call_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Mock executor with sample data
	executor := &mockQueryExecutor{
		result: &bichatsql.QueryResult{
			Columns: []string{"id", "name", "amount"},
			Rows: [][]any{
				{int64(1), "Alice", 100.50},
				{int64(2), "Bob", 200.75},
			},
			RowCount:  2,
			Truncated: false,
			Duration:  10 * time.Millisecond,
		},
	}

	tool := NewExportQueryToExcelTool(
		executor,
		WithQueryOutputDir(tmpDir),
		WithQueryBaseURL("http://test.com/exports"),
		WithQueryExportOptions(excel.DefaultOptions()),
		WithQueryStyleOptions(excel.DefaultStyleOptions()),
	)

	input := `{
		"sql": "SELECT id, name, amount FROM test_table",
		"filename": "test_export",
		"description": "Test export file"
	}`

	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)

	// Parse result
	var output exportQueryOutput
	err = json.Unmarshal([]byte(result), &output)
	require.NoError(t, err)

	assert.Equal(t, "http://test.com/exports/test_export.xlsx", output.URL)
	assert.Equal(t, "test_export.xlsx", output.Filename)
	assert.Equal(t, 2, output.RowCount)
	assert.Equal(t, "Test export file", output.Description)
	assert.Positive(t, output.FileSizeKB)

	// Verify file exists
	filePath := filepath.Join(tmpDir, "test_export.xlsx")
	_, err = os.Stat(filePath)
	assert.NoError(t, err, "Excel file should exist")
}

func TestExportQueryToExcelTool_CallStructured_EmitsArtifact(t *testing.T) {
	tmpDir := t.TempDir()
	executor := &mockQueryExecutor{
		result: &bichatsql.QueryResult{
			Columns:  []string{"id"},
			Rows:     [][]any{{int64(1)}},
			RowCount: 1,
		},
	}

	tool := NewExportQueryToExcelTool(
		executor,
		WithQueryOutputDir(tmpDir),
		WithQueryBaseURL("http://test.com/exports"),
	)

	result, err := tool.CallStructured(context.Background(), `{"sql":"SELECT id FROM test","filename":"artifact.xlsx"}`)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Artifacts, 1)

	artifact := result.Artifacts[0]
	assert.Equal(t, "export", artifact.Type)
	assert.Equal(t, "artifact.xlsx", artifact.Name)
	assert.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", artifact.MimeType)
	assert.Equal(t, "http://test.com/exports/artifact.xlsx", artifact.URL)
	assert.Equal(t, 1, artifact.Metadata["row_count"])
	assert.Positive(t, artifact.SizeBytes)
}

func TestExportQueryToExcelTool_Call_DefaultFilename(t *testing.T) {
	tmpDir := t.TempDir()

	executor := &mockQueryExecutor{
		result: &bichatsql.QueryResult{
			Columns:   []string{"col1"},
			Rows:      [][]any{{"value1"}},
			RowCount:  1,
			Truncated: false,
		},
	}

	tool := NewExportQueryToExcelTool(
		executor,
		WithQueryOutputDir(tmpDir),
		WithQueryBaseURL("http://test.com"),
	)

	input := `{"sql": "SELECT col1 FROM test"}`

	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)

	var output exportQueryOutput
	err = json.Unmarshal([]byte(result), &output)
	require.NoError(t, err)

	assert.Equal(t, "export.xlsx", output.Filename)
	assert.Equal(t, "http://test.com/export.xlsx", output.URL)
}

func TestExportQueryToExcelTool_Call_AutoAppendExtension(t *testing.T) {
	tmpDir := t.TempDir()

	executor := &mockQueryExecutor{
		result: &bichatsql.QueryResult{
			Columns:  []string{"col1"},
			Rows:     [][]any{{"value1"}},
			RowCount: 1,
		},
	}

	tool := NewExportQueryToExcelTool(
		executor,
		WithQueryOutputDir(tmpDir),
		WithQueryBaseURL("http://test.com"),
	)

	input := `{
		"sql": "SELECT col1 FROM test",
		"filename": "report"
	}`

	result, err := tool.Call(context.Background(), input)
	require.NoError(t, err)

	var output exportQueryOutput
	err = json.Unmarshal([]byte(result), &output)
	require.NoError(t, err)

	assert.Equal(t, "report.xlsx", output.Filename)
}

func TestExportQueryToExcelTool_Call_BuildsAbsoluteURLFromRequestWhenBaseURLEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	executor := &mockQueryExecutor{
		result: &bichatsql.QueryResult{
			Columns:  []string{"col1"},
			Rows:     [][]any{{"value1"}},
			RowCount: 1,
		},
	}

	tool := NewExportQueryToExcelTool(
		executor,
		WithQueryOutputDir(tmpDir),
	)

	req := httptest.NewRequest(http.MethodPost, "http://internal/stream", nil)
	req.Header.Set("X-Forwarded-Host", "bi.example.com")
	req.Header.Set("X-Forwarded-Proto", "https")
	ctx := composables.WithParams(context.Background(), &composables.Params{Request: req})

	result, err := tool.Call(ctx, `{"sql":"SELECT col1 FROM test","filename":"report.xlsx"}`)
	require.NoError(t, err)

	var output exportQueryOutput
	require.NoError(t, json.Unmarshal([]byte(result), &output))
	assert.Equal(t, "https://bi.example.com/report.xlsx", output.URL)
}

func TestExportQueryToExcelTool_Call_ValidationErrors(t *testing.T) {
	tmpDir := t.TempDir()
	executor := &mockQueryExecutor{}

	tool := NewExportQueryToExcelTool(
		executor,
		WithQueryOutputDir(tmpDir),
		WithQueryBaseURL("http://test.com"),
	)

	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "missing sql",
			input: `{"filename": "test.xlsx"}`,
		},
		{
			name:  "empty sql",
			input: `{"sql": ""}`,
		},
		{
			name:  "write operation",
			input: `{"sql": "DELETE FROM test"}`,
		},
		{
			name:  "insert operation",
			input: `{"sql": "INSERT INTO test VALUES (1)"}`,
		},
		{
			name:  "update operation",
			input: `{"sql": "UPDATE test SET col=1"}`,
		},
		{
			name:  "drop operation",
			input: `{"sql": "DROP TABLE test"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Call(context.Background(), tt.input)
			require.NoError(t, err, "Validation errors should return nil error for: %s", tt.name)
			assert.Contains(t, result, "error", "Expected formatted error in result for: %s", tt.name)
		})
	}
}

func TestExportQueryToExcelTool_Call_QueryExecutionError(t *testing.T) {
	tmpDir := t.TempDir()

	executor := &mockQueryExecutor{
		err: assert.AnError,
	}

	tool := NewExportQueryToExcelTool(
		executor,
		WithQueryOutputDir(tmpDir),
		WithQueryBaseURL("http://test.com"),
	)

	input := `{"sql": "SELECT * FROM test"}`

	_, err := tool.Call(context.Background(), input)
	assert.Error(t, err)
}

func TestApplyRowLimit(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		maxRows  int
		expected string
	}{
		{
			name:     "no existing limit",
			query:    "SELECT * FROM test",
			maxRows:  1000,
			expected: "SELECT * FROM (SELECT * FROM test) AS _bichat_export LIMIT 1000",
		},
		{
			name:     "existing limit preserved",
			query:    "SELECT * FROM test LIMIT 500",
			maxRows:  1000,
			expected: "SELECT * FROM test LIMIT 500",
		},
		{
			name:     "existing limit with offset preserved",
			query:    "SELECT * FROM test LIMIT 100 OFFSET 10",
			maxRows:  1000,
			expected: "SELECT * FROM test LIMIT 100 OFFSET 10",
		},
		{
			name:     "CTE with existing limit preserved",
			query:    "WITH cte AS (SELECT * FROM test) SELECT * FROM cte LIMIT 100",
			maxRows:  50000,
			expected: "WITH cte AS (SELECT * FROM test) SELECT * FROM cte LIMIT 100",
		},
		{
			name:     "with where clause",
			query:    "SELECT * FROM test WHERE id > 10",
			maxRows:  50000,
			expected: "SELECT * FROM (SELECT * FROM test WHERE id > 10) AS _bichat_export LIMIT 50000",
		},
		{
			name:     "strips trailing semicolon",
			query:    "SELECT * FROM test;",
			maxRows:  1000,
			expected: "SELECT * FROM (SELECT * FROM test) AS _bichat_export LIMIT 1000",
		},
		{
			name:     "existing limit with semicolon preserved",
			query:    "SELECT * FROM test LIMIT 500;",
			maxRows:  1000,
			expected: "SELECT * FROM test LIMIT 500",
		},
		{
			name:     "CTE query without limit",
			query:    "WITH cte AS (SELECT * FROM test) SELECT * FROM cte",
			maxRows:  1000,
			expected: "WITH cte AS (SELECT * FROM test) SELECT * FROM cte LIMIT 1000",
		},
		{
			name:     "limit in subquery only still wraps",
			query:    "SELECT * FROM (SELECT * FROM test LIMIT 10) sub",
			maxRows:  1000,
			expected: "SELECT * FROM (SELECT * FROM (SELECT * FROM test LIMIT 10) sub) AS _bichat_export LIMIT 1000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyRowLimit(tt.query, tt.maxRows)
			assert.Equal(t, tt.expected, result)
		})
	}
}
