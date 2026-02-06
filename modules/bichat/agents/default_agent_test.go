package agents

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/bichat/storage"
	"github.com/iota-uz/iota-sdk/pkg/bichat/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockQueryExecutor is a mock implementation of bichatsql.QueryExecutor for testing.
type mockQueryExecutor struct {
	executeQueryFn func(ctx context.Context, sql string, params []any, timeout time.Duration) (*bichatsql.QueryResult, error)
}

func (m *mockQueryExecutor) ExecuteQuery(ctx context.Context, sql string, params []any, timeout time.Duration) (*bichatsql.QueryResult, error) {
	if m.executeQueryFn != nil {
		return m.executeQueryFn(ctx, sql, params, timeout)
	}
	return &bichatsql.QueryResult{
		Columns:  []string{},
		Rows:     [][]any{},
		RowCount: 0,
	}, nil
}

// mockKBSearcher is a mock implementation of KBSearcher for testing.
type mockKBSearcher struct {
	searchFn      func(ctx context.Context, query string, opts kb.SearchOptions) ([]kb.SearchResult, error)
	isAvailableFn func() bool
}

func (m *mockKBSearcher) Search(ctx context.Context, query string, opts kb.SearchOptions) ([]kb.SearchResult, error) {
	if m.searchFn != nil {
		return m.searchFn(ctx, query, opts)
	}
	return []kb.SearchResult{}, nil
}

func (m *mockKBSearcher) GetDocument(ctx context.Context, id string) (*kb.Document, error) {
	return nil, nil
}

func (m *mockKBSearcher) IsAvailable() bool {
	if m.isAvailableFn != nil {
		return m.isAvailableFn()
	}
	return true
}

// mockFileStorage is a mock implementation of FileStorage for testing.
type mockFileStorage struct {
	saveFn   func(ctx context.Context, filename string, content io.Reader, metadata storage.FileMetadata) (string, error)
	getFn    func(ctx context.Context, url string) (io.ReadCloser, error)
	deleteFn func(ctx context.Context, url string) error
	existsFn func(ctx context.Context, url string) (bool, error)
}

func (m *mockFileStorage) Save(ctx context.Context, filename string, content io.Reader, metadata storage.FileMetadata) (string, error) {
	if m.saveFn != nil {
		return m.saveFn(ctx, filename, content, metadata)
	}
	return "http://localhost/files/" + filename, nil
}

func (m *mockFileStorage) Get(ctx context.Context, url string) (io.ReadCloser, error) {
	if m.getFn != nil {
		return m.getFn(ctx, url)
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func (m *mockFileStorage) Delete(ctx context.Context, url string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, url)
	}
	return nil
}

func (m *mockFileStorage) Exists(ctx context.Context, url string) (bool, error) {
	if m.existsFn != nil {
		return m.existsFn(ctx, url)
	}
	return true, nil
}

func TestNewDefaultBIAgent(t *testing.T) {
	t.Parallel()

	executor := &mockQueryExecutor{}

	agent, err := NewDefaultBIAgent(executor)
	require.NoError(t, err)
	require.NotNil(t, agent)

	// Verify agent metadata
	metadata := agent.Metadata()
	assert.Equal(t, "bi_agent", metadata.Name)
	assert.Equal(t, "Business Intelligence assistant with SQL and KB access", metadata.Description)
	assert.Equal(t, "Use for data analysis, reporting, and BI queries", metadata.WhenToUse)
	assert.Equal(t, "gpt-4", metadata.Model)
	assert.Equal(t, []string{agents.ToolFinalAnswer}, metadata.TerminationTools)
}

func TestNewDefaultBIAgent_NilExecutor(t *testing.T) {
	t.Parallel()

	agent, err := NewDefaultBIAgent(nil)
	require.Error(t, err)
	require.Nil(t, agent)
	assert.Contains(t, err.Error(), "executor is required")
}

func TestDefaultBIAgent_CoreTools(t *testing.T) {
	t.Parallel()

	executor := &mockQueryExecutor{}
	agent, err := NewDefaultBIAgent(executor)
	require.NoError(t, err)

	// Verify core tools are registered
	agentTools := agent.Tools()
	toolNames := make(map[string]bool)
	for _, tool := range agentTools {
		toolNames[tool.Name()] = true
	}

	// Core tools that should always be present
	coreTools := []string{
		"get_current_time",
		"schema_list",
		"schema_describe",
		"sql_execute",
		"export_query_to_excel",
		"draw_chart",
		"ask_user_question",
	}

	for _, toolName := range coreTools {
		assert.True(t, toolNames[toolName], "Core tool %s should be registered", toolName)
	}

	// Verify optional tools are NOT present by default
	assert.False(t, toolNames["kb_search"], "kb_search should not be present without KBSearcher")
	assert.False(t, toolNames["export_data_to_excel"], "export_data_to_excel should not be present without ExportTools")
	assert.False(t, toolNames["export_to_pdf"], "export_to_pdf should not be present without ExportTools")
}

func TestDefaultBIAgent_WithKBSearcher(t *testing.T) {
	t.Parallel()

	executor := &mockQueryExecutor{}
	kbSearcher := &mockKBSearcher{}

	agent, err := NewDefaultBIAgent(
		executor,
		WithKBSearcher(kbSearcher),
	)
	require.NoError(t, err)

	// Verify KB search tool is registered
	agentTools := agent.Tools()
	toolNames := make(map[string]bool)
	for _, tool := range agentTools {
		toolNames[tool.Name()] = true
	}

	assert.True(t, toolNames["kb_search"], "kb_search tool should be present when KBSearcher is provided")
}

func TestDefaultBIAgent_WithExportTools(t *testing.T) {
	t.Parallel()

	executor := &mockQueryExecutor{}
	fileStorage := &mockFileStorage{}

	// Create export tools
	excelTool := tools.NewExportToExcelTool(
		tools.WithOutputDir("/tmp/exports"),
		tools.WithBaseURL("http://localhost/exports"),
	)
	pdfTool := tools.NewExportToPDFTool("http://gotenberg:3000", fileStorage)

	// Create agent with export tools
	agent, err := NewDefaultBIAgent(
		executor,
		WithExportTools(excelTool, pdfTool),
	)
	require.NoError(t, err)

	// Verify export tools are registered
	agentTools := agent.Tools()
	toolNames := make(map[string]bool)
	for _, tool := range agentTools {
		toolNames[tool.Name()] = true
	}

	assert.True(t, toolNames["export_data_to_excel"], "export_data_to_excel tool should be present when configured")
	assert.True(t, toolNames["export_to_pdf"], "export_to_pdf tool should be present when configured")
}

func TestDefaultBIAgent_WithoutExportTools(t *testing.T) {
	t.Parallel()

	executor := &mockQueryExecutor{}

	agent, err := NewDefaultBIAgent(executor)
	require.NoError(t, err)

	// Verify Excel export tool is NOT registered by default
	agentTools := agent.Tools()
	toolNames := make(map[string]bool)
	for _, tool := range agentTools {
		toolNames[tool.Name()] = true
	}

	assert.True(t, toolNames["export_query_to_excel"], "export_query_to_excel should be present by default")
	assert.False(t, toolNames["export_data_to_excel"], "export_data_to_excel tool should not be present without configuration")
	assert.False(t, toolNames["export_to_pdf"], "export_to_pdf tool should not be present without configuration")
}

func TestDefaultBIAgent_WithModel(t *testing.T) {
	t.Parallel()

	executor := &mockQueryExecutor{}

	agent, err := NewDefaultBIAgent(
		executor,
		WithModel("gpt-3.5-turbo"),
	)
	require.NoError(t, err)

	metadata := agent.Metadata()
	assert.Equal(t, "gpt-3.5-turbo", metadata.Model)
}

func TestDefaultBIAgent_ToolRouting(t *testing.T) {
	t.Parallel()

	executor := &mockQueryExecutor{
		executeQueryFn: func(ctx context.Context, sql string, params []any, timeout time.Duration) (*bichatsql.QueryResult, error) {
			// Schema tools use pg_catalog/information_schema via adapters.
			if strings.Contains(sql, "pg_catalog.pg_views") || strings.Contains(sql, "FROM pg_class c") {
				return &bichatsql.QueryResult{
					Columns:  []string{"schema", "name", "approximate_row_count"},
					Rows:     [][]any{{"analytics", "test_view", int64(10)}},
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

	agent, err := NewDefaultBIAgent(executor)
	require.NoError(t, err)

	ctx := context.Background()

	tests := []struct {
		name        string
		toolName    string
		input       string
		expectError bool
	}{
		{
			name:        "get_current_time tool",
			toolName:    "get_current_time",
			input:       `{"timezone":"UTC"}`,
			expectError: false,
		},
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
			name:        "draw_chart tool",
			toolName:    "draw_chart",
			input:       `{"chartType":"line","title":"Test Chart","series":[{"name":"Series 1","data":[1,2,3]}]}`,
			expectError: false,
		},
		{
			name:        "unknown tool",
			toolName:    "unknown_tool",
			input:       `{}`,
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

func TestDefaultBIAgent_AllOptions(t *testing.T) {
	t.Parallel()

	executor := &mockQueryExecutor{}
	kbSearcher := &mockKBSearcher{}
	fileStorage := &mockFileStorage{}

	agent, err := NewDefaultBIAgent(
		executor,
		WithKBSearcher(kbSearcher),
		WithModel("claude-3-opus"),
		WithExportTools(
			tools.NewExportToExcelTool(),
			tools.NewExportToPDFTool("http://gotenberg:3000", fileStorage),
		),
	)
	require.NoError(t, err)

	// Verify metadata
	metadata := agent.Metadata()
	assert.Equal(t, "bi_agent", metadata.Name)
	assert.Equal(t, "claude-3-opus", metadata.Model)

	// Verify optional tools are registered
	agentTools := agent.Tools()
	toolNames := make(map[string]bool)
	for _, tool := range agentTools {
		toolNames[tool.Name()] = true
	}

	assert.True(t, toolNames["kb_search"], "kb_search should be present")
	assert.True(t, toolNames["export_data_to_excel"], "export_data_to_excel should be present when configured")
	assert.True(t, toolNames["export_to_pdf"], "export_to_pdf should be present when configured")
}

func TestDefaultBIAgent_InterfaceCompliance(t *testing.T) {
	t.Parallel()

	executor := &mockQueryExecutor{}
	agent, err := NewDefaultBIAgent(executor)
	require.NoError(t, err)

	// Verify agent implements ExtendedAgent interface
	var _ agents.ExtendedAgent = agent

	// Verify agent implements Agent interface
	var _ agents.Agent = agent
}
