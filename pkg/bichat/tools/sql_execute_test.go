package tools

import (
	"context"
	"strings"
	"testing"
)

func TestValidateQueryParameters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		query     string
		params    map[string]any
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
			params:    map[string]any{"id": 1, "name": "Alice"},
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
			params:    map[string]any{"id": 1},
			wantError: true,
			errMsg:    "params provided but query contains no placeholders",
		},
		{
			name:      "empty params map with placeholders - invalid",
			query:     "SELECT * FROM users WHERE id = $1",
			params:    map[string]any{},
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

func TestSQLExecuteToolParameterValidation(t *testing.T) {
	t.Parallel()

	executor := &mockSchemaExecutor{
		columnsResult: &QueryResult{
			Columns:  []string{"id", "name"},
			Rows:     []map[string]interface{}{{"id": int64(1), "name": "Alice"}},
			RowCount: 1,
		},
	}

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
			name:      "query with multiple placeholders",
			input:     `{"query": "SELECT * FROM users WHERE id = $1 AND name = $2"}`,
			wantError: true,
			errCode:   "INVALID_REQUEST",
		},
		{
			name:      "valid query without placeholders",
			input:     `{"query": "SELECT * FROM users LIMIT 10"}`,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := tool.Call(context.Background(), tt.input)

			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error, got nil")
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
