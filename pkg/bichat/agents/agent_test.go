package agents

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAgent_Metadata verifies that agent metadata is correctly set and retrieved.
func TestAgent_Metadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		opts     []AgentOption
		expected AgentMetadata
	}{
		{
			name: "default metadata",
			opts: []AgentOption{
				WithName("test_agent"),
			},
			expected: AgentMetadata{
				Name:             "test_agent",
				Model:            "gpt-5.2",
				TerminationTools: []string{},
			},
		},
		{
			name: "full metadata",
			opts: []AgentOption{
				WithName("sql_agent"),
				WithDescription("Executes SQL queries"),
				WithWhenToUse("Use when querying databases"),
				WithModel("gpt-5-mini"),
				WithTerminationTools("final_answer", "submit_result"),
			},
			expected: AgentMetadata{
				Name:             "sql_agent",
				Description:      "Executes SQL queries",
				WhenToUse:        "Use when querying databases",
				Model:            "gpt-5-mini",
				TerminationTools: []string{"final_answer", "submit_result"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			agent := NewBaseAgent(tt.opts...)
			metadata := agent.Metadata()

			assert.Equal(t, tt.expected.Name, metadata.Name)
			assert.Equal(t, tt.expected.Description, metadata.Description)
			assert.Equal(t, tt.expected.WhenToUse, metadata.WhenToUse)
			assert.Equal(t, tt.expected.Model, metadata.Model)
			assert.Equal(t, tt.expected.TerminationTools, metadata.TerminationTools)
		})
	}
}

// TestAgent_Tools verifies that tools are correctly registered and retrieved.
func TestAgent_Tools(t *testing.T) {
	t.Parallel()

	// Create test tools
	tool1 := NewTool(
		"echo",
		"Echoes the input",
		map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		func(ctx context.Context, input string) (string, error) {
			return input, nil
		},
	)

	tool2 := NewTool(
		"reverse",
		"Reverses the input string",
		map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		func(ctx context.Context, input string) (string, error) {
			runes := []rune(input)
			for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
				runes[i], runes[j] = runes[j], runes[i]
			}
			return string(runes), nil
		},
	)

	tests := []struct {
		name          string
		tools         []Tool
		expectedNames []string
	}{
		{
			name:          "no tools",
			tools:         []Tool{},
			expectedNames: []string{},
		},
		{
			name:          "single tool",
			tools:         []Tool{tool1},
			expectedNames: []string{"echo"},
		},
		{
			name:          "multiple tools",
			tools:         []Tool{tool1, tool2},
			expectedNames: []string{"echo", "reverse"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			agent := NewBaseAgent(
				WithName("test_agent"),
				WithTools(tt.tools...),
			)

			tools := agent.Tools()
			assert.Len(t, tools, len(tt.expectedNames))

			for i, tool := range tools {
				assert.Equal(t, tt.expectedNames[i], tool.Name())
			}
		})
	}
}

// TestAgent_SystemPrompt verifies that system prompts are correctly set and retrieved.
func TestAgent_SystemPrompt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		prompt         string
		expectedPrompt string
	}{
		{
			name:           "empty prompt",
			prompt:         "",
			expectedPrompt: "",
		},
		{
			name:           "simple prompt",
			prompt:         "You are a helpful assistant.",
			expectedPrompt: "You are a helpful assistant.",
		},
		{
			name: "multiline prompt",
			prompt: `You are a SQL expert.
Help users query and analyze their databases.
Always use parameterized queries.`,
			expectedPrompt: `You are a SQL expert.
Help users query and analyze their databases.
Always use parameterized queries.`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			agent := NewBaseAgent(
				WithName("test_agent"),
				WithSystemPrompt(tt.prompt),
			)

			ctx := context.Background()
			prompt := agent.SystemPrompt(ctx)
			assert.Equal(t, tt.expectedPrompt, prompt)
		})
	}
}

// TestAgent_OnToolCall verifies that tool calls are correctly routed.
func TestAgent_OnToolCall(t *testing.T) {
	t.Parallel()

	// Create test tools
	echoTool := NewTool(
		"echo",
		"Echoes the input",
		map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		func(ctx context.Context, input string) (string, error) {
			return input, nil
		},
	)

	errorTool := NewTool(
		"error",
		"Always returns an error",
		map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		func(ctx context.Context, input string) (string, error) {
			return "", errors.New("tool error")
		},
	)

	agent := NewBaseAgent(
		WithName("test_agent"),
		WithTools(echoTool, errorTool),
	)

	tests := []struct {
		name        string
		toolName    string
		input       string
		expected    string
		expectError bool
		errorType   error
	}{
		{
			name:        "successful tool call",
			toolName:    "echo",
			input:       "hello",
			expected:    "hello",
			expectError: false,
		},
		{
			name:        "tool returns error",
			toolName:    "error",
			input:       "anything",
			expected:    "",
			expectError: true,
		},
		{
			name:        "tool not found",
			toolName:    "nonexistent",
			input:       "anything",
			expected:    "",
			expectError: true,
			errorType:   ErrToolNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			result, err := agent.OnToolCall(ctx, tt.toolName, tt.input)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorType != nil {
					require.ErrorIs(t, err, tt.errorType)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestBaseAgent_FunctionalOptions verifies that all functional options work correctly.
func TestBaseAgent_FunctionalOptions(t *testing.T) {
	t.Parallel()

	// Create test tools
	tool1 := NewTool(
		"tool1",
		"Test tool 1",
		map[string]any{"type": "object"},
		func(ctx context.Context, input string) (string, error) {
			return "result1", nil
		},
	)
	tool2 := NewTool(
		"tool2",
		"Test tool 2",
		map[string]any{"type": "object"},
		func(ctx context.Context, input string) (string, error) {
			return "result2", nil
		},
	)

	tests := []struct {
		name     string
		options  []AgentOption
		validate func(t *testing.T, agent *BaseAgent)
	}{
		{
			name: "WithName",
			options: []AgentOption{
				WithName("custom_agent"),
			},
			validate: func(t *testing.T, agent *BaseAgent) {
				t.Helper()
				assert.Equal(t, "custom_agent", agent.Metadata().Name)
			},
		},
		{
			name: "WithDescription",
			options: []AgentOption{
				WithName("agent"),
				WithDescription("Custom description"),
			},
			validate: func(t *testing.T, agent *BaseAgent) {
				t.Helper()
				assert.Equal(t, "Custom description", agent.Metadata().Description)
			},
		},
		{
			name: "WithWhenToUse",
			options: []AgentOption{
				WithName("agent"),
				WithWhenToUse("Use when analyzing data"),
			},
			validate: func(t *testing.T, agent *BaseAgent) {
				t.Helper()
				assert.Equal(t, "Use when analyzing data", agent.Metadata().WhenToUse)
			},
		},
		{
			name: "WithTools",
			options: []AgentOption{
				WithName("agent"),
				WithTools(tool1, tool2),
			},
			validate: func(t *testing.T, agent *BaseAgent) {
				t.Helper()
				tools := agent.Tools()
				assert.Len(t, tools, 2)
				assert.Equal(t, "tool1", tools[0].Name())
				assert.Equal(t, "tool2", tools[1].Name())
			},
		},
		{
			name: "WithSystemPrompt",
			options: []AgentOption{
				WithName("agent"),
				WithSystemPrompt("Custom system prompt"),
			},
			validate: func(t *testing.T, agent *BaseAgent) {
				t.Helper()
				ctx := context.Background()
				assert.Equal(t, "Custom system prompt", agent.SystemPrompt(ctx))
			},
		},
		{
			name: "WithModel",
			options: []AgentOption{
				WithName("agent"),
				WithModel("claude-opus-4-6"),
			},
			validate: func(t *testing.T, agent *BaseAgent) {
				t.Helper()
				assert.Equal(t, "claude-opus-4-6", agent.Metadata().Model)
			},
		},
		{
			name: "WithTerminationTools - single",
			options: []AgentOption{
				WithName("agent"),
				WithTerminationTools("final_answer"),
			},
			validate: func(t *testing.T, agent *BaseAgent) {
				t.Helper()
				assert.Equal(t, []string{"final_answer"}, agent.Metadata().TerminationTools)
			},
		},
		{
			name: "WithTerminationTools - multiple",
			options: []AgentOption{
				WithName("agent"),
				WithTerminationTools("final_answer", "submit", "complete"),
			},
			validate: func(t *testing.T, agent *BaseAgent) {
				t.Helper()
				assert.Equal(t, []string{"final_answer", "submit", "complete"}, agent.Metadata().TerminationTools)
			},
		},
		{
			name: "All options combined",
			options: []AgentOption{
				WithName("full_agent"),
				WithDescription("Fully configured agent"),
				WithWhenToUse("Use for everything"),
				WithModel("gpt-5.2"),
				WithTools(tool1, tool2),
				WithSystemPrompt("You are an expert"),
				WithTerminationTools("done", "finish"),
			},
			validate: func(t *testing.T, agent *BaseAgent) {
				t.Helper()
				metadata := agent.Metadata()
				assert.Equal(t, "full_agent", metadata.Name)
				assert.Equal(t, "Fully configured agent", metadata.Description)
				assert.Equal(t, "Use for everything", metadata.WhenToUse)
				assert.Equal(t, []string{"done", "finish"}, metadata.TerminationTools)
				assert.Len(t, agent.Tools(), 2)
				assert.Equal(t, "You are an expert", agent.SystemPrompt(context.Background()))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			agent := NewBaseAgent(tt.options...)
			tt.validate(t, agent)
		})
	}
}

// TestBaseAgent_ToolMapConcurrency verifies thread-safe tool lookup.
func TestBaseAgent_ToolMapConcurrency(t *testing.T) {
	t.Parallel()

	// Create test tools
	tools := make([]Tool, 10)
	for i := 0; i < 10; i++ {
		index := i
		tools[i] = NewTool(
			string(rune('a'+i)),
			"Test tool",
			map[string]any{"type": "object"},
			func(ctx context.Context, input string) (string, error) {
				return string(rune('A' + index)), nil
			},
		)
	}

	agent := NewBaseAgent(
		WithName("concurrent_agent"),
		WithTools(tools...),
	)

	// Simulate concurrent tool calls
	ctx := context.Background()

	// Use channels to collect results from goroutines
	type testResult struct {
		err      error
		result   string
		expected string
	}
	results := make(chan testResult, 1000) // 10 goroutines * 100 iterations

	for i := 0; i < 10; i++ {
		toolName := string(rune('a' + i))
		expectedResult := string(rune('A' + i))

		go func(name, expected string) {
			for j := 0; j < 100; j++ {
				result, err := agent.OnToolCall(ctx, name, "")
				results <- testResult{
					err:      err,
					result:   result,
					expected: expected,
				}
			}
		}(toolName, expectedResult)
	}

	// Collect and assert results in main goroutine
	for i := 0; i < 1000; i++ {
		res := <-results
		require.NoError(t, res.err)
		assert.Equal(t, res.expected, res.result)
	}
}

// TestBaseAgent_ToolNotFound verifies ErrToolNotFound behavior.
func TestBaseAgent_ToolNotFound(t *testing.T) {
	t.Parallel()

	agent := NewBaseAgent(
		WithName("test_agent"),
		WithTools(), // No tools
	)

	ctx := context.Background()
	_, err := agent.OnToolCall(ctx, "nonexistent", "")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrToolNotFound)
}

// TestBaseAgent_EmptyConfiguration verifies default values.
func TestBaseAgent_EmptyConfiguration(t *testing.T) {
	t.Parallel()

	agent := NewBaseAgent()

	metadata := agent.Metadata()
	assert.Empty(t, metadata.Name)
	assert.Equal(t, "gpt-5.2", metadata.Model)
	assert.Empty(t, metadata.TerminationTools)
	assert.Empty(t, agent.Tools())
	assert.Empty(t, agent.SystemPrompt(context.Background()))
}
