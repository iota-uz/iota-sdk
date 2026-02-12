package agents

import (
	"context"
	"testing"
	"time"

	"strings"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSQLExecutor is a mock implementation of bichatsql.QueryExecutor for testing.
type mockSQLExecutor struct {
	executeQueryFn func(ctx context.Context, sql string, params []any, timeout time.Duration) (*bichatsql.QueryResult, error)
}

func (m *mockSQLExecutor) ExecuteQuery(ctx context.Context, sql string, params []any, timeout time.Duration) (*bichatsql.QueryResult, error) {
	if m.executeQueryFn != nil {
		return m.executeQueryFn(ctx, sql, params, timeout)
	}
	return &bichatsql.QueryResult{
		Columns:  []string{},
		Rows:     [][]any{},
		RowCount: 0,
	}, nil
}

func TestNewSQLAgent(t *testing.T) {
	t.Parallel()

	executor := &mockSQLExecutor{}

	agent, err := NewSQLAgent(executor)
	require.NoError(t, err)
	require.NotNil(t, agent)

	// Verify agent metadata
	metadata := agent.Metadata()
	assert.Equal(t, "sql-analyst", metadata.Name)
	assert.Equal(t, "Specialized agent for SQL query generation and database analysis", metadata.Description)
	assert.Contains(t, metadata.WhenToUse, "SQL queries")
	assert.Equal(t, "gpt-4", metadata.Model)
	assert.Equal(t, []string{agents.ToolFinalAnswer}, metadata.TerminationTools)
}

func TestNewSQLAgent_NilExecutor(t *testing.T) {
	t.Parallel()

	agent, err := NewSQLAgent(nil)
	require.Error(t, err)
	require.Nil(t, agent)
	assert.Contains(t, err.Error(), "executor is required")
}

func TestSQLAgent_CoreTools(t *testing.T) {
	t.Parallel()

	executor := &mockSQLExecutor{}
	agent, err := NewSQLAgent(executor)
	require.NoError(t, err)

	// Verify core SQL tools are registered
	agentTools := agent.Tools()
	toolNames := make(map[string]bool)
	for _, tool := range agentTools {
		toolNames[tool.Name()] = true
	}

	// SQL-specific tools that should always be present
	expectedTools := []string{
		"schema_list",
		"schema_describe",
		"sql_execute",
	}

	for _, toolName := range expectedTools {
		assert.True(t, toolNames[toolName], "SQL tool %s should be registered", toolName)
	}

	// Verify non-SQL tools are not registered by default.
	// This avoids brittle exact tool-count assertions while protecting the agent's intended scope.
	for _, toolName := range []string{
		"get_current_time",
		"draw_chart",
		"ask_user_question",
		"kb_search",
		"export_data_to_excel",
		"export_to_pdf",
	} {
		assert.False(t, toolNames[toolName], "SQL agent should not register %s", toolName)
	}
}

func TestSQLAgent_WithModel(t *testing.T) {
	t.Parallel()

	executor := &mockSQLExecutor{}

	agent, err := NewSQLAgent(
		executor,
		WithSQLAgentModel("gpt-3.5-turbo"),
	)
	require.NoError(t, err)

	metadata := agent.Metadata()
	assert.Equal(t, "gpt-3.5-turbo", metadata.Model)
}

func TestSQLAgent_ToolRouting(t *testing.T) {
	t.Parallel()

	executor := &mockSQLExecutor{
		executeQueryFn: func(ctx context.Context, sql string, params []any, timeout time.Duration) (*bichatsql.QueryResult, error) {
			// Schema tools use pg_catalog/information_schema via adapters.
			if strings.Contains(sql, "FROM pg_class") || strings.Contains(sql, "pg_namespace") {
				return &bichatsql.QueryResult{
					Columns:  []string{"schema", "name", "approximate_row_count"},
					Rows:     [][]any{{"analytics", "test_table", int64(123)}},
					RowCount: 1,
				}, nil
			}
			if strings.Contains(sql, "pg_catalog.pg_views") {
				return &bichatsql.QueryResult{
					Columns:  []string{"schema", "name", "type"},
					Rows:     [][]any{{"analytics", "test_view", "view"}},
					RowCount: 1,
				}, nil
			}
			if strings.Contains(sql, "information_schema.columns") {
				return &bichatsql.QueryResult{
					Columns:  []string{"column_name", "data_type", "is_nullable", "column_default", "character_maximum_length", "numeric_precision", "numeric_scale"},
					Rows:     [][]any{{"id", "integer", "NO", nil, nil, nil, nil}},
					RowCount: 1,
				}, nil
			}
			return &bichatsql.QueryResult{
				Columns:  []string{"id", "name"},
				Rows:     [][]any{{1, "test"}},
				RowCount: 1,
			}, nil
		},
	}

	agent, err := NewSQLAgent(executor)
	require.NoError(t, err)

	ctx := context.Background()

	tests := []struct {
		name        string
		toolName    string
		input       string
		expectError bool
	}{
		{
			name:        "schema_list tool",
			toolName:    "schema_list",
			input:       `{}`,
			expectError: false,
		},
		{
			name:        "sql_execute tool",
			toolName:    "sql_execute",
			input:       `{"query":"SELECT * FROM users LIMIT 10"}`,
			expectError: false,
		},
		{
			name:        "unknown tool",
			toolName:    "unknown_tool",
			input:       `{}`,
			expectError: true,
		},
		{
			name:        "ask_user_question not available",
			toolName:    "ask_user_question",
			input:       `{"question":"test"}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := agent.OnToolCall(ctx, tt.toolName, tt.input)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, result)
			}
		})
	}
}

func TestSQLAgent_InterfaceCompliance(t *testing.T) {
	t.Parallel()

	executor := &mockSQLExecutor{}
	agent, err := NewSQLAgent(executor)
	require.NoError(t, err)

	// Verify agent implements ExtendedAgent interface
	var _ agents.ExtendedAgent = agent

	// Verify agent implements Agent interface
	var _ agents.Agent = agent
}
