package agents

import (
	"context"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
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
	assert.Equal(t, agents.Isolated, metadata.Isolation)
	assert.Equal(t, "gpt-4", metadata.Model)
	assert.Equal(t, []string{agents.ToolFinalAnswer}, metadata.TerminationTools)
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
	assert.False(t, toolNames["export_to_excel"], "export_to_excel should not be present without ExcelExporter")
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

func TestDefaultBIAgent_WithExcelExporter(t *testing.T) {
	t.Parallel()

	executor := &mockQueryExecutor{}
	excelExporter := &mockExcelExporter{}

	agent, err := NewDefaultBIAgent(
		executor,
		WithExcelExporter(excelExporter),
	)
	require.NoError(t, err)

	// Verify Excel export tool is registered
	agentTools := agent.Tools()
	toolNames := make(map[string]bool)
	for _, tool := range agentTools {
		toolNames[tool.Name()] = true
	}

	assert.True(t, toolNames["export_to_excel"], "export_to_excel tool should be present when ExcelExporter is provided")
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
	excelExporter := &mockExcelExporter{}
	pdfExporter := &mockPDFExporter{}

	agent, err := NewDefaultBIAgent(
		executor,
		WithKBSearcher(kbSearcher),
		WithExcelExporter(excelExporter),
		WithPDFExporter(pdfExporter),
		WithModel("claude-3-opus"),
	)
	require.NoError(t, err)

	// Verify metadata
	metadata := agent.Metadata()
	assert.Equal(t, "bi_agent", metadata.Name)
	assert.Equal(t, "claude-3-opus", metadata.Model)

	// Verify all optional tools are registered
	agentTools := agent.Tools()
	toolNames := make(map[string]bool)
	for _, tool := range agentTools {
		toolNames[tool.Name()] = true
	}

	assert.True(t, toolNames["kb_search"], "kb_search should be present")
	assert.True(t, toolNames["export_to_excel"], "export_to_excel should be present")
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

	prompt := buildBISystemPrompt()

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
		"EXPORT RESULTS",
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
		"export_to_excel",
		"export_to_pdf",
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

func TestDefaultBIAgent_ToolCount(t *testing.T) {
	t.Parallel()

	executor := &mockQueryExecutor{}

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
			name: "with Excel exporter",
			opts: []BIAgentOption{
				WithExcelExporter(&mockExcelExporter{}),
			},
			expectedCount: 7,
		},
		{
			name: "with all options",
			opts: []BIAgentOption{
				WithKBSearcher(&mockKBSearcher{}),
				WithExcelExporter(&mockExcelExporter{}),
			},
			expectedCount: 8,
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
