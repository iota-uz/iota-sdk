package formatters

import (
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

// TestEscapeMarkdownCell tests the markdown cell escaping utility.
func TestEscapeMarkdownCell(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		maxWidth int
		want     string
	}{
		{
			name:     "empty string",
			input:    "",
			maxWidth: 0,
			want:     "",
		},
		{
			name:     "pipe character",
			input:    "col|value",
			maxWidth: 0,
			want:     "col\\|value",
		},
		{
			name:     "newline",
			input:    "line1\nline2",
			maxWidth: 0,
			want:     "line1\\nline2",
		},
		{
			name:     "carriage return",
			input:    "line1\rline2",
			maxWidth: 0,
			want:     "line1\\nline2",
		},
		{
			name:     "CRLF",
			input:    "line1\r\nline2",
			maxWidth: 0,
			want:     "line1\\nline2",
		},
		{
			name:     "no truncation when maxWidth is 0",
			input:    "this is a very long string that should not be truncated",
			maxWidth: 0,
			want:     "this is a very long string that should not be truncated",
		},
		{
			name:     "truncation when string exceeds maxWidth",
			input:    "this is a very long string that exceeds the maximum width",
			maxWidth: 20,
			want:     "this is a very lo...",
		},
		{
			name:     "no truncation when string is shorter than maxWidth",
			input:    "short",
			maxWidth: 20,
			want:     "short",
		},
		{
			name:     "whitespace trimming",
			input:    "  value  ",
			maxWidth: 0,
			want:     "value",
		},
		{
			name:     "all special characters",
			input:    "col|val\r\nue",
			maxWidth: 0,
			want:     "col\\|val\\nue",
		},
		{
			name:     "truncation when maxWidth is 1 (avoid negative slice)",
			input:    "long",
			maxWidth: 1,
			want:     ".",
		},
		{
			name:     "truncation when maxWidth is 2 (avoid negative slice)",
			input:    "long",
			maxWidth: 2,
			want:     "..",
		},
		{
			name:     "truncation when maxWidth is 3 (avoid negative slice)",
			input:    "long",
			maxWidth: 3,
			want:     "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := EscapeMarkdownCell(tt.input, tt.maxWidth)
			if got != tt.want {
				t.Errorf("EscapeMarkdownCell() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestQueryResultFormatter tests the query result formatter.
func TestQueryResultFormatter(t *testing.T) {
	t.Parallel()

	formatter := NewQueryResultFormatter()
	opts := types.DefaultFormatOptions()

	tests := []struct {
		name      string
		payload   any
		wantErr   bool
		wantTexts []string
	}{
		{
			name:    "wrong payload type",
			payload: "invalid",
			wantErr: true,
		},
		{
			name: "empty columns",
			payload: types.QueryResultFormatPayload{
				Query:       "SELECT * FROM empty",
				ExecutedSQL: "SELECT * FROM empty",
				DurationMs:  10,
				Columns:     []string{},
				Rows:        [][]any{},
				RowCount:    0,
				Limit:       100,
			},
			wantTexts: []string{
				"Query executed successfully.",
				"No columns returned.",
				"```sql\nSELECT * FROM empty\n```",
			},
		},
		{
			name: "empty rows",
			payload: types.QueryResultFormatPayload{
				Query:       "SELECT * FROM users WHERE false",
				ExecutedSQL: "SELECT * FROM users WHERE false",
				DurationMs:  5,
				Columns:     []string{"id", "name"},
				Rows:        [][]any{},
				RowCount:    0,
				Limit:       100,
			},
			wantTexts: []string{
				"Query executed successfully.",
				"No rows returned.",
				"```sql\nSELECT * FROM users WHERE false\n```",
			},
		},
		{
			name: "normal result with columns and rows",
			payload: types.QueryResultFormatPayload{
				Query:       "SELECT id, name FROM users",
				ExecutedSQL: "SELECT id, name FROM users LIMIT 100",
				DurationMs:  25,
				Columns:     []string{"id", "name"},
				Rows: [][]any{
					{1, "Alice"},
					{2, "Bob"},
				},
				RowCount: 2,
				Limit:    100,
			},
			wantTexts: []string{
				"Query executed successfully.",
				"Duration: 25ms",
				"Returned: 2 row(s)",
				"Limit: 100",
				"Truncated: no",
				"| id | name |",
				"| 1 | Alice |",
				"| 2 | Bob |",
				"```sql\nSELECT id, name FROM users LIMIT 100\n```",
			},
		},
		{
			name: "truncated result at MaxRows",
			payload: types.QueryResultFormatPayload{
				Query:       "SELECT id FROM numbers",
				ExecutedSQL: "SELECT id FROM numbers LIMIT 30",
				DurationMs:  50,
				Columns:     []string{"id"},
				Rows:        generateRows(30),
				RowCount:    30,
				Limit:       1000,
				Truncated:   true,
			},
			wantTexts: []string{
				"Query executed successfully.",
				"Truncated: yes",
				"Use a follow-up query",
			},
		},
		{
			name: "nil values show NULL",
			payload: types.QueryResultFormatPayload{
				Query:       "SELECT id, nullable FROM data",
				ExecutedSQL: "SELECT id, nullable FROM data",
				DurationMs:  10,
				Columns:     []string{"id", "nullable"},
				Rows: [][]any{
					{1, nil},
					{2, "value"},
				},
				RowCount: 2,
				Limit:    100,
			},
			wantTexts: []string{
				"| 1 | NULL |",
				"| 2 | value |",
			},
		},
		{
			name: "pipe characters in data are escaped",
			payload: types.QueryResultFormatPayload{
				Query:       "SELECT val FROM test",
				ExecutedSQL: "SELECT val FROM test",
				DurationMs:  10,
				Columns:     []string{"val"},
				Rows: [][]any{
					{"data|with|pipes"},
				},
				RowCount: 1,
				Limit:    100,
			},
			wantTexts: []string{
				"data\\|with\\|pipes",
			},
		},
		{
			name: "newlines in cells are escaped",
			payload: types.QueryResultFormatPayload{
				Query:       "SELECT text FROM test",
				ExecutedSQL: "SELECT text FROM test",
				DurationMs:  10,
				Columns:     []string{"text"},
				Rows: [][]any{
					{"line1\nline2"},
				},
				RowCount: 1,
				Limit:    100,
			},
			wantTexts: []string{
				"line1\\nline2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := formatter.Format(tt.payload, opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("Format() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			for _, text := range tt.wantTexts {
				if !strings.Contains(got, text) {
					t.Errorf("Format() output missing %q\nGot:\n%s", text, got)
				}
			}
		})
	}
}

// TestExplainPlanFormatter tests the explain plan formatter.
func TestExplainPlanFormatter(t *testing.T) {
	t.Parallel()

	formatter := NewExplainPlanFormatter()
	opts := types.DefaultFormatOptions()

	tests := []struct {
		name      string
		payload   any
		wantErr   bool
		wantTexts []string
	}{
		{
			name:    "wrong payload type",
			payload: 123,
			wantErr: true,
		},
		{
			name: "empty plan",
			payload: types.ExplainPlanPayload{
				Query:       "SELECT * FROM users",
				ExecutedSQL: "EXPLAIN SELECT * FROM users",
				DurationMs:  5,
				PlanLines:   []string{},
			},
			wantTexts: []string{
				"Explain plan generated successfully.",
				"No plan output.",
			},
		},
		{
			name: "normal plan",
			payload: types.ExplainPlanPayload{
				Query:       "SELECT * FROM users WHERE id = 1",
				ExecutedSQL: "EXPLAIN SELECT * FROM users WHERE id = 1",
				DurationMs:  8,
				PlanLines: []string{
					"Seq Scan on users",
					"  Filter: (id = 1)",
				},
			},
			wantTexts: []string{
				"Explain plan generated successfully.",
				"Duration: 8ms",
				"```text\nSeq Scan on users",
				"Filter: (id = 1)",
				"```",
			},
		},
		{
			name: "truncated plan",
			payload: types.ExplainPlanPayload{
				Query:       "SELECT * FROM complex_join",
				ExecutedSQL: "EXPLAIN SELECT * FROM complex_join",
				DurationMs:  50,
				PlanLines: []string{
					"Nested Loop",
					"  -> Seq Scan on table1",
					"  -> Index Scan on table2",
				},
				Truncated: true,
			},
			wantTexts: []string{
				"Explain plan generated successfully.",
				"...",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := formatter.Format(tt.payload, opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("Format() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			for _, text := range tt.wantTexts {
				if !strings.Contains(got, text) {
					t.Errorf("Format() output missing %q\nGot:\n%s", text, got)
				}
			}
		})
	}
}

// TestSchemaListFormatter tests the schema list formatter.
func TestSchemaListFormatter(t *testing.T) {
	t.Parallel()

	formatter := NewSchemaListFormatter()
	opts := types.DefaultFormatOptions()

	tests := []struct {
		name      string
		payload   any
		wantErr   bool
		wantTexts []string
		noTexts   []string
	}{
		{
			name:    "wrong payload type",
			payload: []string{"invalid"},
			wantErr: true,
		},
		{
			name: "empty tables",
			payload: types.SchemaListPayload{
				Tables:    []types.SchemaListTable{},
				HasAccess: false,
			},
			wantTexts: []string{
				"## Available Tables",
				"0 table(s) found.",
			},
		},
		{
			name: "without access control",
			payload: types.SchemaListPayload{
				Tables: []types.SchemaListTable{
					{Name: "users", RowCount: 100, Description: "User accounts"},
					{Name: "orders", RowCount: 500, Description: ""},
				},
				HasAccess: false,
			},
			wantTexts: []string{
				"## Available Tables",
				"| # | Table | ~Rows | Description |",
				"| 1 | users | ~100 | User accounts |",
				"| 2 | orders | ~500 | - |",
				"2 table(s) found.",
			},
			noTexts: []string{
				"Access",
			},
		},
		{
			name: "with access control",
			payload: types.SchemaListPayload{
				Tables: []types.SchemaListTable{
					{Name: "users", RowCount: 100, Description: "User accounts"},
					{Name: "sensitive", RowCount: 50, Description: "Sensitive data"},
				},
				ViewInfos: []types.ViewAccessInfo{
					{Access: "ok"},
					{Access: "denied"},
				},
				HasAccess: true,
			},
			wantTexts: []string{
				"## Available Tables",
				"| # | Table | ~Rows | Access | Description |",
				"| 1 | users | ~100 | ok | User accounts |",
				"| 2 | sensitive | ~50 | denied | Sensitive data |",
				"2 table(s) found.",
			},
		},
		{
			name: "tables with and without descriptions",
			payload: types.SchemaListPayload{
				Tables: []types.SchemaListTable{
					{Name: "products", RowCount: 1000, Description: "Product catalog"},
					{Name: "temp", RowCount: 0, Description: ""},
				},
				HasAccess: false,
			},
			wantTexts: []string{
				"| 1 | products | ~1000 | Product catalog |",
				"| 2 | temp | - | - |",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := formatter.Format(tt.payload, opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("Format() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			for _, text := range tt.wantTexts {
				if !strings.Contains(got, text) {
					t.Errorf("Format() output missing %q\nGot:\n%s", text, got)
				}
			}
			for _, text := range tt.noTexts {
				if strings.Contains(got, text) {
					t.Errorf("Format() output should not contain %q\nGot:\n%s", text, got)
				}
			}
		})
	}
}

// TestSchemaDescribeFormatter tests the schema describe formatter.
func TestSchemaDescribeFormatter(t *testing.T) {
	t.Parallel()

	formatter := NewSchemaDescribeFormatter()
	opts := types.DefaultFormatOptions()
	defaultVal := "0"

	tests := []struct {
		name      string
		payload   any
		wantErr   bool
		wantTexts []string
		noTexts   []string
	}{
		{
			name:    "wrong payload type",
			payload: map[string]any{"invalid": true},
			wantErr: true,
		},
		{
			name: "without descriptions",
			payload: types.SchemaDescribePayload{
				Name:   "users",
				Schema: "public",
				Columns: []types.SchemaDescribeColumn{
					{Name: "id", Type: "integer", Nullable: false, DefaultValue: nil},
					{Name: "email", Type: "text", Nullable: false, DefaultValue: nil},
				},
			},
			wantTexts: []string{
				"## Table: users (public)",
				"| # | Column | Type | Nullable | Default |",
				"| 1 | id | integer | NO | - |",
				"| 2 | email | text | NO | - |",
				"2 column(s)",
			},
			noTexts: []string{
				"Description",
			},
		},
		{
			name: "with descriptions",
			payload: types.SchemaDescribePayload{
				Name:   "products",
				Schema: "public",
				Columns: []types.SchemaDescribeColumn{
					{Name: "id", Type: "uuid", Nullable: false, DefaultValue: nil, Description: "Primary key"},
					{Name: "name", Type: "text", Nullable: false, DefaultValue: nil, Description: "Product name"},
					{Name: "stock", Type: "integer", Nullable: true, DefaultValue: &defaultVal, Description: ""},
				},
			},
			wantTexts: []string{
				"## Table: products (public)",
				"| # | Column | Type | Nullable | Default | Description |",
				"| 1 | id | uuid | NO | - | Primary key|",
				"| 2 | name | text | NO | - | Product name|",
				"| 3 | stock | integer | YES | 0 | -|",
				"3 column(s)",
			},
		},
		{
			name: "nullable and non-nullable columns",
			payload: types.SchemaDescribePayload{
				Name:   "orders",
				Schema: "public",
				Columns: []types.SchemaDescribeColumn{
					{Name: "id", Type: "bigint", Nullable: false, DefaultValue: nil},
					{Name: "notes", Type: "text", Nullable: true, DefaultValue: nil},
				},
			},
			wantTexts: []string{
				"| 1 | id | bigint | NO | - |",
				"| 2 | notes | text | YES | - |",
			},
		},
		{
			name: "columns with and without default values",
			payload: types.SchemaDescribePayload{
				Name:   "settings",
				Schema: "public",
				Columns: []types.SchemaDescribeColumn{
					{Name: "id", Type: "integer", Nullable: false, DefaultValue: nil},
					{Name: "enabled", Type: "boolean", Nullable: false, DefaultValue: &defaultVal},
				},
			},
			wantTexts: []string{
				"| 1 | id | integer | NO | - |",
				"| 2 | enabled | boolean | NO | 0 |",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := formatter.Format(tt.payload, opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("Format() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			for _, text := range tt.wantTexts {
				if !strings.Contains(got, text) {
					t.Errorf("Format() output missing %q\nGot:\n%s", text, got)
				}
			}
			for _, text := range tt.noTexts {
				if strings.Contains(got, text) {
					t.Errorf("Format() output should not contain %q\nGot:\n%s", text, got)
				}
			}
		})
	}
}

// TestToolErrorFormatter tests the tool error formatter.
func TestToolErrorFormatter(t *testing.T) {
	t.Parallel()

	formatter := NewToolErrorFormatter()
	opts := types.DefaultFormatOptions()

	tests := []struct {
		name      string
		payload   any
		wantErr   bool
		wantTexts []string
	}{
		{
			name:    "wrong payload type",
			payload: []int{1, 2, 3},
			wantErr: true,
		},
		{
			name: "ToolErrorPayload with hints",
			payload: types.ToolErrorPayload{
				Code:    "INVALID_QUERY",
				Message: "Syntax error in SQL",
				Hints:   []string{"Check your SELECT clause", "Verify table names"},
			},
			wantTexts: []string{
				`"error"`,
				`"code": "INVALID_QUERY"`,
				`"message": "Syntax error in SQL"`,
				`"hints"`,
				"Check your SELECT clause",
				"Verify table names",
			},
		},
		{
			name: "ToolErrorPayload without hints",
			payload: types.ToolErrorPayload{
				Code:    "NOT_FOUND",
				Message: "Table not found",
				Hints:   nil,
			},
			wantTexts: []string{
				`"error"`,
				`"code": "NOT_FOUND"`,
				`"message": "Table not found"`,
				`"hints": null`,
			},
		},
		{
			name: "SQLDiagnosisPayload",
			payload: types.SQLDiagnosisPayload{
				Code:       "42P01",
				Message:    "relation \"users\" does not exist",
				Table:      "users",
				Column:     "",
				Suggestion: "Did you mean \"user\"?",
				Hints:      []string{"Check spelling", "Use schema.table notation"},
			},
			wantTexts: []string{
				`"error"`,
				`"code": "42P01"`,
				`"message": "relation \"users\" does not exist"`,
				`"table": "users"`,
				`"suggestion": "Did you mean \"user\"?"`,
				`"hints"`,
				"Check spelling",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := formatter.Format(tt.payload, opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("Format() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			for _, text := range tt.wantTexts {
				if !strings.Contains(got, text) {
					t.Errorf("Format() output missing %q\nGot:\n%s", text, got)
				}
			}
		})
	}
}

// TestJSONFormatter tests the JSON formatter.
func TestJSONFormatter(t *testing.T) {
	t.Parallel()

	formatter := NewJSONFormatter()
	opts := types.DefaultFormatOptions()

	tests := []struct {
		name      string
		payload   any
		wantErr   bool
		wantTexts []string
	}{
		{
			name: "JSONPayload unwrapping",
			payload: types.JSONPayload{
				Output: map[string]any{
					"status": "success",
					"count":  42,
				},
			},
			wantTexts: []string{
				`"status":"success"`,
				`"count":42`,
			},
		},
		{
			name: "pointer JSONPayload unwrapping",
			payload: &types.JSONPayload{
				Output: map[string]any{
					"status": "ok",
					"value":  100,
				},
			},
			wantTexts: []string{
				`"status":"ok"`,
				`"value":100`,
			},
		},
		{
			name: "raw struct payload",
			payload: map[string]any{
				"name":  "Alice",
				"age":   30,
				"admin": true,
			},
			wantTexts: []string{
				`"name":"Alice"`,
				`"age":30`,
				`"admin":true`,
			},
		},
		{
			name: "nested objects",
			payload: types.JSONPayload{
				Output: map[string]any{
					"user": map[string]any{
						"id":   123,
						"name": "Bob",
					},
					"settings": map[string]any{
						"theme": "dark",
					},
				},
			},
			wantTexts: []string{
				`"user"`,
				`"id":123`,
				`"name":"Bob"`,
				`"settings"`,
				`"theme":"dark"`,
			},
		},
		{
			name: "empty output",
			payload: types.JSONPayload{
				Output: map[string]any{},
			},
			wantTexts: []string{
				"{}",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := formatter.Format(tt.payload, opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("Format() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			for _, text := range tt.wantTexts {
				if !strings.Contains(got, text) {
					t.Errorf("Format() output missing %q\nGot:\n%s", text, got)
				}
			}
		})
	}
}

// TestArtifactListFormatter tests the artifact list formatter.
func TestArtifactListFormatter(t *testing.T) {
	t.Parallel()

	formatter := NewArtifactListFormatter()
	opts := types.DefaultFormatOptions()

	tests := []struct {
		name      string
		payload   any
		wantErr   bool
		wantTexts []string
	}{
		{
			name:    "wrong payload type",
			payload: "not an artifact list",
			wantErr: true,
		},
		{
			name: "empty artifacts list",
			payload: types.ArtifactListPayload{
				Page:       1,
				TotalPages: 1,
				Artifacts:  []types.ArtifactEntry{},
				HasNext:    false,
				HitCap:     false,
			},
			wantTexts: []string{
				"## Artifacts (page 1/1)",
				"| id | type | name | mime | size_bytes | created_at |",
				"has_next_page: false",
			},
		},
		{
			name: "pagination info",
			payload: types.ArtifactListPayload{
				Page:       2,
				TotalPages: 5,
				Artifacts: []types.ArtifactEntry{
					{
						ID:        "abc-123",
						Type:      "export",
						Name:      "report.xlsx",
						MimeType:  "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
						SizeBytes: 1024,
						CreatedAt: "2024-01-15T10:30:00Z",
					},
				},
				HasNext: true,
				HitCap:  false,
			},
			wantTexts: []string{
				"## Artifacts (page 2/5)",
				"| abc-123 | export | report.xlsx |",
				"| 1024 |",
				"has_next_page: true (use page=3)",
			},
		},
		{
			name: "HasNext flag with page hint",
			payload: types.ArtifactListPayload{
				Page:       1,
				TotalPages: 3,
				Artifacts: []types.ArtifactEntry{
					{
						ID:        "xyz-456",
						Type:      "chart",
						Name:      "sales.json",
						MimeType:  "application/json",
						SizeBytes: 2048,
						CreatedAt: "2024-01-16T14:20:00Z",
					},
				},
				HasNext: true,
				HitCap:  false,
			},
			wantTexts: []string{
				"has_next_page: true (use page=2)",
			},
		},
		{
			name: "HitCap flag with note",
			payload: types.ArtifactListPayload{
				Page:       1,
				TotalPages: 1,
				Artifacts: []types.ArtifactEntry{
					{
						ID:        "def-789",
						Type:      "export",
						Name:      "data.csv",
						MimeType:  "text/csv",
						SizeBytes: 512,
						CreatedAt: "2024-01-17T09:00:00Z",
					},
				},
				HasNext: false,
				HitCap:  true,
			},
			wantTexts: []string{
				"Note: artifact listing reached tool cap and may be truncated.",
			},
		},
		{
			name: "names with pipes are escaped",
			payload: types.ArtifactListPayload{
				Page:       1,
				TotalPages: 1,
				Artifacts: []types.ArtifactEntry{
					{
						ID:        "pipe-test",
						Type:      "export",
						Name:      "file|with|pipes.txt",
						MimeType:  "text/plain",
						SizeBytes: 128,
						CreatedAt: "2024-01-18T12:00:00Z",
					},
				},
				HasNext: false,
				HitCap:  false,
			},
			wantTexts: []string{
				"file\\|with\\|pipes.txt",
			},
		},
		{
			name: "mimetypes with pipes are escaped",
			payload: types.ArtifactListPayload{
				Page:       1,
				TotalPages: 1,
				Artifacts: []types.ArtifactEntry{
					{
						ID:        "mime-test",
						Type:      "export",
						Name:      "file.dat",
						MimeType:  "application/x-custom|type",
						SizeBytes: 256,
						CreatedAt: "2024-01-19T15:00:00Z",
					},
				},
				HasNext: false,
				HitCap:  false,
			},
			wantTexts: []string{
				"application/x-custom\\|type",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := formatter.Format(tt.payload, opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("Format() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			for _, text := range tt.wantTexts {
				if !strings.Contains(got, text) {
					t.Errorf("Format() output missing %q\nGot:\n%s", text, got)
				}
			}
		})
	}
}

// TestArtifactContentFormatter tests the artifact content formatter.
func TestArtifactContentFormatter(t *testing.T) {
	t.Parallel()

	formatter := NewArtifactContentFormatter()
	opts := types.DefaultFormatOptions()

	tests := []struct {
		name      string
		payload   any
		wantErr   bool
		wantTexts []string
	}{
		{
			name:    "wrong payload type",
			payload: 12345,
			wantErr: true,
		},
		{
			name: "out of range page",
			payload: types.ArtifactContentPayload{
				ID:         "art-123",
				Type:       "export",
				Name:       "report.txt",
				MimeType:   "text/plain",
				Page:       10,
				TotalPages: 5,
				PageSize:   1000,
				Content:    "",
				HasNext:    false,
				OutOfRange: true,
			},
			wantTexts: []string{
				"## Artifact Read",
				"- id: art-123",
				"- page: 10/5",
				"Requested page is out of range for this artifact content.",
			},
		},
		{
			name: "empty content",
			payload: types.ArtifactContentPayload{
				ID:         "art-456",
				Type:       "chart",
				Name:       "empty.json",
				MimeType:   "application/json",
				Page:       1,
				TotalPages: 1,
				PageSize:   1000,
				Content:    "",
				HasNext:    false,
				OutOfRange: false,
			},
			wantTexts: []string{
				"## Artifact Read",
				"- id: art-456",
				"- type: chart",
				"- name: empty.json",
				"- mime: application/json",
				"(no content on this page)",
				"has_next_page: false",
			},
		},
		{
			name: "normal content with pagination",
			payload: types.ArtifactContentPayload{
				ID:         "art-789",
				Type:       "export",
				Name:       "data.csv",
				MimeType:   "text/csv",
				Page:       2,
				TotalPages: 4,
				PageSize:   1000,
				Content:    "id,name,email\n1,Alice,alice@example.com\n2,Bob,bob@example.com",
				HasNext:    true,
				OutOfRange: false,
			},
			wantTexts: []string{
				"## Artifact Read",
				"- id: art-789",
				"- page: 2/4",
				"- page_size: 1000",
				"id,name,email",
				"1,Alice,alice@example.com",
				"has_next_page: true (use page=3)",
			},
		},
		{
			name: "HasNext flag with page hint",
			payload: types.ArtifactContentPayload{
				ID:         "art-next",
				Type:       "export",
				Name:       "large.txt",
				MimeType:   "text/plain",
				Page:       1,
				TotalPages: 10,
				PageSize:   500,
				Content:    "Some content here...",
				HasNext:    true,
				OutOfRange: false,
			},
			wantTexts: []string{
				"has_next_page: true (use page=2)",
			},
		},
		{
			name: "last page no next",
			payload: types.ArtifactContentPayload{
				ID:         "art-last",
				Type:       "export",
				Name:       "final.txt",
				MimeType:   "text/plain",
				Page:       5,
				TotalPages: 5,
				PageSize:   1000,
				Content:    "This is the last page.",
				HasNext:    false,
				OutOfRange: false,
			},
			wantTexts: []string{
				"This is the last page.",
				"has_next_page: false",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := formatter.Format(tt.payload, opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("Format() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			for _, text := range tt.wantTexts {
				if !strings.Contains(got, text) {
					t.Errorf("Format() output missing %q\nGot:\n%s", text, got)
				}
			}
		})
	}
}

// Helper function to generate rows for truncation tests.
func generateRows(n int) [][]any {
	rows := make([][]any, n)
	for i := 0; i < n; i++ {
		rows[i] = []any{i + 1}
	}
	return rows
}
