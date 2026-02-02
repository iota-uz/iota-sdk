package agents

import (
	"context"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockServicesQueryExecutor is a mock implementation of bichatservices.QueryExecutorService for testing.
type mockServicesQueryExecutor struct {
	schemaListFn     func(ctx context.Context) ([]bichatservices.TableInfo, error)
	schemaDescribeFn func(ctx context.Context, tableName string) (*bichatservices.TableSchema, error)
	executeQueryFn   func(ctx context.Context, sql string, params []any, timeoutMs int) (*bichatservices.QueryResult, error)
	validateQueryFn  func(sql string) error
}

func (m *mockServicesQueryExecutor) SchemaList(ctx context.Context) ([]bichatservices.TableInfo, error) {
	if m.schemaListFn != nil {
		return m.schemaListFn(ctx)
	}
	return []bichatservices.TableInfo{}, nil
}

func (m *mockServicesQueryExecutor) SchemaDescribe(ctx context.Context, tableName string) (*bichatservices.TableSchema, error) {
	if m.schemaDescribeFn != nil {
		return m.schemaDescribeFn(ctx, tableName)
	}
	return &bichatservices.TableSchema{}, nil
}

func (m *mockServicesQueryExecutor) ExecuteQuery(ctx context.Context, sql string, params []any, timeoutMs int) (*bichatservices.QueryResult, error) {
	if m.executeQueryFn != nil {
		return m.executeQueryFn(ctx, sql, params, timeoutMs)
	}
	return &bichatservices.QueryResult{
		Columns:  []string{},
		Rows:     [][]any{},
		RowCount: 0,
	}, nil
}

func (m *mockServicesQueryExecutor) ValidateQuery(sql string) error {
	if m.validateQueryFn != nil {
		return m.validateQueryFn(sql)
	}
	return nil
}

func TestNewSQLAgent(t *testing.T) {
	t.Parallel()

	executor := &mockServicesQueryExecutor{}

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

	executor := &mockServicesQueryExecutor{}
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

	// Verify count - should only have SQL tools (no KB search, no charts, no HITL)
	assert.Equal(t, 3, len(agentTools), "SQL agent should have exactly 3 tools")
}

func TestSQLAgent_WithModel(t *testing.T) {
	t.Parallel()

	executor := &mockServicesQueryExecutor{}

	agent, err := NewSQLAgent(
		executor,
		WithSQLAgentModel("gpt-3.5-turbo"),
	)
	require.NoError(t, err)

	metadata := agent.Metadata()
	assert.Equal(t, "gpt-3.5-turbo", metadata.Model)
}

func TestSQLAgent_SystemPrompt(t *testing.T) {
	t.Parallel()

	executor := &mockServicesQueryExecutor{}
	agent, err := NewSQLAgent(executor)
	require.NoError(t, err)

	ctx := context.Background()
	prompt := agent.SystemPrompt(ctx)

	// Verify prompt contains key sections
	assert.NotEmpty(t, prompt)
	assert.Contains(t, prompt, "SQL analyst agent")
	assert.Contains(t, prompt, "WORKFLOW")
	assert.Contains(t, prompt, "AVAILABLE TOOLS")
	assert.Contains(t, prompt, "schema_list")
	assert.Contains(t, prompt, "schema_describe")
	assert.Contains(t, prompt, "sql_execute")
	assert.Contains(t, prompt, "final_answer")
	assert.Contains(t, prompt, "read-only")
	assert.Contains(t, prompt, "SELECT")
}

func TestSQLAgent_ToolRouting(t *testing.T) {
	t.Parallel()

	executor := &mockServicesQueryExecutor{
		executeQueryFn: func(ctx context.Context, sql string, params []any, timeoutMs int) (*bichatservices.QueryResult, error) {
			return &bichatservices.QueryResult{
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
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, result)
			}
		})
	}
}

func TestSQLAgent_InterfaceCompliance(t *testing.T) {
	t.Parallel()

	executor := &mockServicesQueryExecutor{}
	agent, err := NewSQLAgent(executor)
	require.NoError(t, err)

	// Verify agent implements ExtendedAgent interface
	var _ agents.ExtendedAgent = agent

	// Verify agent implements Agent interface
	var _ agents.Agent = agent
}

func TestQueryExecutorAdapter(t *testing.T) {
	t.Parallel()

	// Create mock services executor
	servicesExec := &mockServicesQueryExecutor{
		executeQueryFn: func(ctx context.Context, sql string, params []any, timeoutMs int) (*bichatservices.QueryResult, error) {
			return &bichatservices.QueryResult{
				Columns:   []string{"id", "name", "email"},
				Rows:      [][]any{{1, "John", "john@example.com"}, {2, "Jane", "jane@example.com"}},
				RowCount:  2,
				Truncated: false,
				Duration:  10,
			}, nil
		},
	}

	// Create adapter
	adapter := newQueryExecutorAdapter(servicesExec)

	ctx := context.Background()
	result, err := adapter.ExecuteQuery(ctx, "SELECT * FROM users", nil, 5000)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify conversion from [][]any to []map[string]interface{}
	assert.Equal(t, []string{"id", "name", "email"}, result.Columns)
	assert.Equal(t, 2, result.RowCount)
	assert.Equal(t, int64(10), result.DurationMs)
	assert.False(t, result.IsLimited)

	// Verify rows are converted to map format
	assert.Len(t, result.Rows, 2)
	assert.Equal(t, 1, result.Rows[0]["id"])
	assert.Equal(t, "John", result.Rows[0]["name"])
	assert.Equal(t, "john@example.com", result.Rows[0]["email"])
	assert.Equal(t, 2, result.Rows[1]["id"])
	assert.Equal(t, "Jane", result.Rows[1]["name"])
	assert.Equal(t, "jane@example.com", result.Rows[1]["email"])
}
