package tools

import (
	"context"
	"strings"
	"testing"
	"time"

	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestValidateQueryParameters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		query     string
		params    []any
		wantError bool
		errMsg    string
	}{
		{
			name:      "no placeholders, no params - valid",
			query:     "SELECT * FROM users",
			params:    nil,
			wantError: false,
		},
		{
			name:      "placeholders with params - valid",
			query:     "SELECT * FROM users WHERE id = $1 AND name = $2",
			params:    []any{1, "Alice"},
			wantError: false,
		},
		{
			name:      "placeholders without params - invalid",
			query:     "SELECT * FROM users WHERE id = $1",
			params:    nil,
			wantError: true,
			errMsg:    "query contains placeholders",
		},
		{
			name:      "multiple placeholders without params - invalid",
			query:     "SELECT * FROM users WHERE id = $1 AND name = $2 AND age > $3",
			params:    nil,
			wantError: true,
			errMsg:    "placeholders",
		},
		{
			name:      "params without placeholders - invalid",
			query:     "SELECT * FROM users",
			params:    []any{1},
			wantError: true,
			errMsg:    "params provided but query contains no placeholders",
		},
		{
			name:      "empty params slice with placeholders - invalid",
			query:     "SELECT * FROM users WHERE id = $1",
			params:    []any{},
			wantError: true,
			errMsg:    "query contains placeholders",
		},
		{
			name:      "duplicate placeholders - still reports error",
			query:     "SELECT * FROM users WHERE id = $1 OR parent_id = $1",
			params:    nil,
			wantError: true,
			errMsg:    "$1",
		},
		{
			name:      "max placeholder index exceeds params length - invalid",
			query:     "SELECT * FROM users WHERE id = $2",
			params:    []any{123},
			wantError: true,
			errMsg:    "$2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateQueryParameters(tt.query, tt.params)

			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error message = %v, want substring %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateReadOnlyQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		query     string
		wantError bool
		errMsg    string
	}{
		{
			name:      "simple SELECT - valid",
			query:     "SELECT * FROM users",
			wantError: false,
		},
		{
			name:      "SELECT with JOIN - valid",
			query:     "SELECT u.name, o.total FROM users u JOIN orders o ON u.id = o.user_id",
			wantError: false,
		},
		{
			name:      "CTE with WITH - valid",
			query:     "WITH recent AS (SELECT * FROM orders WHERE activity_date > NOW() - INTERVAL '1 day') SELECT * FROM recent",
			wantError: false,
		},
		{
			name:      "INSERT - invalid",
			query:     "INSERT INTO users (name) VALUES ('Alice')",
			wantError: true,
			errMsg:    "only SELECT queries are allowed",
		},
		{
			name:      "UPDATE - invalid",
			query:     "UPDATE users SET name = 'Bob' WHERE id = 1",
			wantError: true,
			errMsg:    "only SELECT queries are allowed",
		},
		{
			name:      "DELETE - invalid",
			query:     "DELETE FROM users WHERE id = 1",
			wantError: true,
			errMsg:    "only SELECT queries are allowed",
		},
		{
			name:      "DROP TABLE - invalid",
			query:     "DROP TABLE users",
			wantError: true,
			errMsg:    "only SELECT queries are allowed",
		},
		{
			name:      "CREATE TABLE - invalid",
			query:     "CREATE TABLE new_users (id INT)",
			wantError: true,
			errMsg:    "only SELECT queries are allowed",
		},
		{
			name:      "ALTER TABLE - invalid",
			query:     "ALTER TABLE users ADD COLUMN email TEXT",
			wantError: true,
			errMsg:    "only SELECT queries are allowed",
		},
		{
			name:      "TRUNCATE - invalid",
			query:     "TRUNCATE TABLE users",
			wantError: true,
			errMsg:    "only SELECT queries are allowed",
		},
		{
			name:      "SELECT followed by comment - valid",
			query:     "SELECT * FROM users -- this is a comment",
			wantError: false,
		},
		{
			name:      "SELECT with updated_at column - valid",
			query:     "SELECT id, updated_at FROM users ORDER BY id",
			wantError: false,
		},
		{
			name:      "SELECT with update in string literal - valid",
			query:     "SELECT 'UPDATE' AS word, id FROM users",
			wantError: false,
		},
		{
			name:      "SELECT with update in quoted identifier - valid",
			query:     `SELECT "update" FROM users`,
			wantError: false,
		},
		{
			name:      "WITH containing UPDATE - invalid",
			query:     "WITH changed AS (UPDATE users SET first_name = 'x' RETURNING id) SELECT * FROM changed",
			wantError: true,
			errMsg:    "UPDATE",
		},
		{
			name:      "SQL injection attempt - invalid",
			query:     "SELECT * FROM users; DROP TABLE users;",
			wantError: true,
			errMsg:    "DROP",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateReadOnlyQuery(tt.query)

			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error message = %v, want substring %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// mockSQLExecutorForValidation implements bichatsql.QueryExecutor for testing.
type mockSQLExecutorForValidation struct {
	result *bichatsql.QueryResult
	err    error
}

func (m *mockSQLExecutorForValidation) ExecuteQuery(ctx context.Context, sql string, params []any, timeout time.Duration) (*bichatsql.QueryResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.result != nil {
		return m.result, nil
	}
	return &bichatsql.QueryResult{
		Columns:  []string{"id", "name"},
		Rows:     [][]any{{int64(1), "Alice"}},
		RowCount: 1,
	}, nil
}

func TestSQLExecuteToolParameterValidation(t *testing.T) {
	t.Parallel()

	executor := &mockSQLExecutorForValidation{}

	tool := NewSQLExecuteTool(executor)

	tests := []struct {
		name      string
		input     string
		wantError bool
		errCode   string
	}{
		{
			name:      "query with placeholders but no params",
			input:     `{"query": "SELECT * FROM users WHERE id = $1"}`,
			wantError: true,
			errCode:   "INVALID_REQUEST",
		},
		{
			name:      "valid query without placeholders",
			input:     `{"query": "SELECT * FROM users LIMIT 10"}`,
			wantError: false,
		},
		{
			name:      "valid query with params array",
			input:     `{"query": "SELECT * FROM users WHERE id = $1 AND name = $2", "params": [1, "Alice"]}`,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := tool.Call(context.Background(), tt.input)

			if tt.wantError {
				// Validation errors return nil error but formatted error string
				if err != nil {
					t.Fatalf("expected validation error to return nil error, got: %v", err)
				}
				if !strings.Contains(result, tt.errCode) {
					t.Errorf("expected error code %s, got: %s", tt.errCode, result)
				}
				if !strings.Contains(result, "placeholders") {
					t.Errorf("expected 'placeholders' in error, got: %s", result)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestSQLExecuteTool_EnforcesLimitAndWrapsQuery(t *testing.T) {
	t.Parallel()

	var gotSQL string
	executor := &mockSQLExecutorForValidation{
		result: &bichatsql.QueryResult{
			Columns:  []string{"id"},
			Rows:     [][]any{{1}, {2}, {3}, {4}, {5}, {6}},
			RowCount: 6,
		},
	}

	tool := NewSQLExecuteTool(&struct {
		*mockSQLExecutorForValidation
	}{
		mockSQLExecutorForValidation: executor,
	})

	// Override ExecuteQuery to capture SQL.
	toolImpl := tool.(*SQLExecuteTool)
	toolImpl.executor = &mockSQLExecutorCapture{
		fn: func(ctx context.Context, sql string, params []any, timeout time.Duration) (*bichatsql.QueryResult, error) {
			gotSQL = sql
			return executor.ExecuteQuery(ctx, sql, params, timeout)
		},
	}

	outStr, err := tool.Call(context.Background(), `{"query":"SELECT id FROM users","limit":5}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(gotSQL, "SELECT * FROM (SELECT id FROM users) AS _bichat_q LIMIT 6") {
		t.Fatalf("expected wrapped SQL with LIMIT 6, got: %s", gotSQL)
	}

	if !strings.Contains(outStr, "Returned: 5 row(s)") {
		t.Fatalf("expected returned rows in output, got: %s", outStr)
	}
	if !strings.Contains(outStr, "Truncated: yes") {
		t.Fatalf("expected truncated=yes in output, got: %s", outStr)
	}
	if !strings.Contains(outStr, "| id |") {
		t.Fatalf("expected markdown table header, got: %s", outStr)
	}
	if !strings.Contains(outStr, "```sql") || !strings.Contains(outStr, "AS _bichat_q LIMIT 6") {
		t.Fatalf("expected executed SQL block, got: %s", outStr)
	}
}

func TestSQLExecuteTool_PassesParamsToExecutor(t *testing.T) {
	t.Parallel()

	var gotParams []any
	executor := &mockSQLExecutorCapture{
		fn: func(ctx context.Context, sql string, params []any, timeout time.Duration) (*bichatsql.QueryResult, error) {
			gotParams = append([]any(nil), params...)
			return &bichatsql.QueryResult{
				Columns:  []string{"ok"},
				Rows:     [][]any{{true}},
				RowCount: 1,
			}, nil
		},
	}

	tool := NewSQLExecuteTool(executor)

	_, err := tool.Call(context.Background(), `{"query":"SELECT * FROM users WHERE id = $1 AND name = $2","params":[123,"Alice"]}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotParams) != 2 || gotParams[0] != float64(123) || gotParams[1] != "Alice" {
		t.Fatalf("unexpected params passed to executor: %#v", gotParams)
	}
}

func TestSQLExecuteTool_ExplainPlan(t *testing.T) {
	t.Parallel()

	var gotSQL string
	executor := &mockSQLExecutorCapture{
		fn: func(ctx context.Context, sql string, params []any, timeout time.Duration) (*bichatsql.QueryResult, error) {
			gotSQL = sql
			return &bichatsql.QueryResult{
				Columns:  []string{"QUERY PLAN"},
				Rows:     [][]any{{"Seq Scan on users"}},
				RowCount: 1,
			}, nil
		},
	}

	tool := NewSQLExecuteTool(executor)

	outStr, err := tool.Call(context.Background(), `{"query":"SELECT * FROM users","explain_plan":true}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(gotSQL, "EXPLAIN") {
		t.Fatalf("expected EXPLAIN query, got: %s", gotSQL)
	}

	if !strings.Contains(outStr, "Explain plan generated successfully.") {
		t.Fatalf("expected explain header, got: %s", outStr)
	}
	if !strings.Contains(outStr, "```text") || !strings.Contains(outStr, "Seq Scan on users") {
		t.Fatalf("expected plan markdown code block, got: %s", outStr)
	}
	if !strings.Contains(outStr, "```sql") || !strings.Contains(outStr, gotSQL) {
		t.Fatalf("expected executed SQL block, got: %s", outStr)
	}
}

func TestFormatValue_PGNumeric(t *testing.T) {
	t.Parallel()

	var n pgtype.Numeric
	if err := n.Scan("160000"); err != nil {
		t.Fatalf("scan numeric: %v", err)
	}

	got := formatValue(n)
	if got != "160000" {
		t.Fatalf("expected 160000, got %#v", got)
	}

	n.Valid = false
	if got = formatValue(n); got != nil {
		t.Fatalf("expected nil for invalid numeric, got %#v", got)
	}
}

type mockSQLExecutorCapture struct {
	fn func(ctx context.Context, sql string, params []any, timeout time.Duration) (*bichatsql.QueryResult, error)
}

func (m *mockSQLExecutorCapture) ExecuteQuery(ctx context.Context, sql string, params []any, timeout time.Duration) (*bichatsql.QueryResult, error) {
	return m.fn(ctx, sql, params, timeout)
}
