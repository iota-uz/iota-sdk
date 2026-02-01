package agents

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/storage"
	"github.com/iota-uz/iota-sdk/pkg/bichat/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockQueryExecutor is a mock implementation of QueryExecutorService for testing.
type mockQueryExecutor struct {
	executeQueryFn func(ctx context.Context, sql string, params []any, timeoutMs int) (*tools.QueryResult, error)
}

func (m *mockQueryExecutor) ExecuteQuery(ctx context.Context, sql string, params []any, timeoutMs int) (*tools.QueryResult, error) {
	if m.executeQueryFn != nil {
		return m.executeQueryFn(ctx, sql, params, timeoutMs)
	}
	return &tools.QueryResult{
		Columns:  []string{},
		Rows:     []map[string]interface{}{},
		RowCount: 0,
	}, nil
}

// mockKBSearcher is a mock implementation of KBSearcher for testing.
type mockKBSearcher struct {
	searchFn      func(ctx context.Context, query string, limit int) ([]tools.SearchResult, error)
	isAvailableFn func() bool
}

func (m *mockKBSearcher) Search(ctx context.Context, query string, limit int) ([]tools.SearchResult, error) {
	if m.searchFn != nil {
		return m.searchFn(ctx, query, limit)
	}
	return []tools.SearchResult{}, nil
}

func (m *mockKBSearcher) IsAvailable() bool {
	if m.isAvailableFn != nil {
		return m.isAvailableFn()
	}
	return true
}

// mockExcelExporter is a mock implementation of ExcelExporter for testing.
type mockExcelExporter struct {
	exportFn func(ctx context.Context, data *tools.QueryResult, filename string) (string, error)
}

func (m *mockExcelExporter) ExportToExcel(ctx context.Context, data *tools.QueryResult, filename string) (string, error) {
	if m.exportFn != nil {
		return m.exportFn(ctx, data, filename)
	}
	return "/exports/test.xlsx", nil
}

// mockPDFExporter is a mock implementation of PDFExporter for testing.
type mockPDFExporter struct {
	exportFn func(ctx context.Context, html string, filename string, landscape bool) (string, error)
}

func (m *mockPDFExporter) ExportToPDF(ctx context.Context, html string, filename string, landscape bool) (string, error) {
	if m.exportFn != nil {
		return m.exportFn(ctx, html, filename, landscape)
	}
	return "/exports/test.pdf", nil
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

func TestDefaultBIAgent_SystemPrompt(t *testing.T) {
	t.Parallel()

	executor := &mockQueryExecutor{}
	agent, err := NewDefaultBIAgent(executor)
	require.NoError(t, err)

	ctx := context.Background()
	prompt := agent.SystemPrompt(ctx)

	// Verify prompt contains key sections
	assert.NotEmpty(t, prompt)
	assert.Contains(t, prompt, "Business Intelligence assistant")
	assert.Contains(t, prompt, "AVAILABLE TOOLS")
	assert.Contains(t, prompt, "WORKFLOW GUIDELINES")
	assert.Contains(t, prompt, "get_current_time")
	assert.Contains(t, prompt, "schema_list")
	assert.Contains(t, prompt, "schema_describe")
	assert.Contains(t, prompt, "sql_execute")
	assert.Contains(t, prompt, "draw_chart")
	assert.Contains(t, prompt, "ask_user_question")
	assert.Contains(t, prompt, "final_answer")
	assert.Contains(t, prompt, "read-only")
	assert.Contains(t, prompt, "1000 rows")
}

func TestDefaultBIAgent_ToolRouting(t *testing.T) {
	t.Parallel()

	executor := &mockQueryExecutor{
		executeQueryFn: func(ctx context.Context, sql string, params []any, timeoutMs int) (*tools.QueryResult, error) {
			return &tools.QueryResult{
				Columns: []string{"id", "name"},
				Rows: []map[string]interface{}{
					{"id": 1, "name": "test"},
				},
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
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
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

func TestBuildBISystemPrompt(t *testing.T) {
	t.Parallel()

	prompt := buildBISystemPrompt(false, nil)

	// Verify prompt structure
	assert.NotEmpty(t, prompt)
	assert.True(t, len(prompt) > 500, "System prompt should be comprehensive")

	// Verify key sections exist
	sections := []string{
		"AVAILABLE TOOLS",
		"WORKFLOW GUIDELINES",
		"UNDERSTAND THE REQUEST",
		"EXPLORE THE SCHEMA",
		"WRITE SAFE SQL",
		"VISUALIZE DATA",
		"PROVIDE CLEAR ANSWERS",
		"IMPORTANT CONSTRAINTS",
		"EXAMPLE WORKFLOW",
	}

	for _, section := range sections {
		assert.True(t, strings.Contains(prompt, section), "Prompt should contain section: %s", section)
	}

	// Verify all tool names are mentioned
	toolNames := []string{
		"get_current_time",
		"schema_list",
		"schema_describe",
		"sql_execute",
		"draw_chart",
		"kb_search",
		"ask_user_question",
		"final_answer",
	}

	for _, toolName := range toolNames {
		assert.True(t, strings.Contains(prompt, toolName), "Prompt should mention tool: %s", toolName)
	}

	// Verify safety constraints are mentioned
	safetyKeywords := []string{
		"read-only",
		"SELECT",
		"1000 rows",
		"30 seconds",
		"validate",
		"sensitive data",
	}

	for _, keyword := range safetyKeywords {
		assert.True(t, strings.Contains(prompt, keyword), "Prompt should mention safety keyword: %s", keyword)
	}
}

func TestBuildBISystemPrompt_WithCodeInterpreter(t *testing.T) {
	t.Parallel()

	promptWithCodeInterpreter := buildBISystemPrompt(true, nil)
	promptWithoutCodeInterpreter := buildBISystemPrompt(false, nil)

	// Code interpreter should be mentioned when enabled
	assert.True(t, strings.Contains(promptWithCodeInterpreter, "code_interpreter"))
	assert.True(t, strings.Contains(promptWithCodeInterpreter, "Python"))

	// Code interpreter should not be mentioned when disabled
	assert.False(t, strings.Contains(promptWithoutCodeInterpreter, "code_interpreter"))
}

func TestBuildBISystemPrompt_WithRegistry(t *testing.T) {
	t.Parallel()

	// Create executor and registry with SQLAgent
	executor := &mockServicesExecutor{}
	registry := agents.NewAgentRegistry()
	sqlAgent, err := NewSQLAgent(executor)
	require.NoError(t, err)
	err = registry.Register(sqlAgent)
	require.NoError(t, err)

	// Build prompts with and without registry
	promptWithRegistry := buildBISystemPrompt(false, registry)
	promptWithoutRegistry := buildBISystemPrompt(false, nil)

	// Verify delegation tool is mentioned when registry is provided
	assert.Contains(t, promptWithRegistry, "task")
	assert.Contains(t, promptWithRegistry, "# Available Agents")
	assert.Contains(t, promptWithRegistry, "sql-analyst")
	assert.Contains(t, promptWithRegistry, "DELEGATION GUIDELINES")

	// Verify delegation tool is not mentioned without registry
	assert.NotContains(t, promptWithoutRegistry, "# Available Agents")
	assert.NotContains(t, promptWithoutRegistry, "DELEGATION GUIDELINES")
}

func TestDefaultBIAgent_ToolCount(t *testing.T) {
	t.Parallel()

	executor := &mockQueryExecutor{}
	fileStorage := &mockFileStorage{}

	tests := []struct {
		name          string
		opts          []BIAgentOption
		expectedCount int
	}{
		{
			name:          "core tools only",
			opts:          []BIAgentOption{},
			expectedCount: 6, // time, schema_list, schema_describe, sql_execute, chart, ask_user_question
		},
		{
			name: "with KB searcher",
			opts: []BIAgentOption{
				WithKBSearcher(&mockKBSearcher{}),
			},
			expectedCount: 7,
		},
		{
			name: "with export tools",
			opts: []BIAgentOption{
				WithExportTools(
					tools.NewExportToExcelTool(),
					tools.NewExportToPDFTool("http://gotenberg:3000", fileStorage),
				),
			},
			expectedCount: 8, // core 6 + excel + pdf
		},
		{
			name: "with all options",
			opts: []BIAgentOption{
				WithKBSearcher(&mockKBSearcher{}),
				WithExportTools(
					tools.NewExportToExcelTool(),
					tools.NewExportToPDFTool("http://gotenberg:3000", fileStorage),
				),
			},
			expectedCount: 9, // core 6 + kb + excel + pdf
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent, err := NewDefaultBIAgent(executor, tt.opts...)
			require.NoError(t, err)

			agentTools := agent.Tools()
			assert.Len(t, agentTools, tt.expectedCount, "Tool count mismatch")
		})
	}
}
