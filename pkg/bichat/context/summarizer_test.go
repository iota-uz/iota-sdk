package context

import (
	"context"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockModel implements agents.Model for testing
type MockModel struct {
	generateFunc func(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) (*agents.Response, error)
	infoFunc     func() agents.ModelInfo
	streamFunc   func(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) (types.Generator[agents.Chunk], error)
	pricingFunc  func() agents.ModelPricing
}

func (m *MockModel) Generate(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) (*agents.Response, error) {
	if m.generateFunc != nil {
		return m.generateFunc(ctx, req, opts...)
	}
	return &agents.Response{
		Message: types.Message{
			Role:    types.RoleAssistant,
			Content: "Mock summary of conversation",
		},
		Usage: types.TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
	}, nil
}

func (m *MockModel) Stream(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) (types.Generator[agents.Chunk], error) {
	if m.streamFunc != nil {
		return m.streamFunc(ctx, req, opts...)
	}
	return nil, nil
}

func (m *MockModel) Info() agents.ModelInfo {
	if m.infoFunc != nil {
		return m.infoFunc()
	}
	return agents.ModelInfo{
		Name:     "mock-model",
		Provider: "test",
	}
}

func (m *MockModel) HasCapability(capability agents.Capability) bool {
	return false
}

func (m *MockModel) Pricing() agents.ModelPricing {
	if m.pricingFunc != nil {
		return m.pricingFunc()
	}
	return agents.ModelPricing{}
}

// MockTokenEstimator implements agents.TokenEstimator for testing
type MockTokenEstimator struct {
	estimateTokensFunc   func(ctx context.Context, text string) (int, error)
	estimateMessagesFunc func(ctx context.Context, messages []types.Message) (int, error)
}

func (e *MockTokenEstimator) EstimateTokens(ctx context.Context, text string) (int, error) {
	if e.estimateTokensFunc != nil {
		return e.estimateTokensFunc(ctx, text)
	}
	return len(text) / 4, nil // Simple character-based estimate
}

func (e *MockTokenEstimator) EstimateMessages(ctx context.Context, messages []types.Message) (int, error) {
	if e.estimateMessagesFunc != nil {
		return e.estimateMessagesFunc(ctx, messages)
	}
	total := 0
	for _, msg := range messages {
		total += len(msg.Content) / 4
	}
	return total, nil
}

func TestLLMHistorySummarizer_SummarizeMessages_EmptyMessages(t *testing.T) {
	t.Parallel()

	model := &MockModel{}
	estimator := &MockTokenEstimator{}
	summarizer := NewLLMHistorySummarizer(model, estimator)

	summary, tokens, err := summarizer.SummarizeMessages(context.Background(), []types.Message{}, 500)

	require.NoError(t, err)
	assert.Empty(t, summary)
	assert.Equal(t, 0, tokens)
}

func TestLLMHistorySummarizer_SummarizeMessages_Success(t *testing.T) {
	t.Parallel()

	expectedSummary := "User asked about sales data. Agent provided Q1 sales analysis showing 20% growth."

	model := &MockModel{
		generateFunc: func(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) (*agents.Response, error) {
			// Verify system prompt is set
			assert.Greater(t, len(req.Messages), 0)
			assert.Equal(t, types.RoleSystem, req.Messages[0].Role)
			assert.Contains(t, req.Messages[0].Content, "conversation summarizer")

			// Verify user prompt contains messages
			assert.Equal(t, 2, len(req.Messages))
			assert.Equal(t, types.RoleUser, req.Messages[1].Role)
			assert.Contains(t, req.Messages[1].Content, "Conversation to summarize")

			return &agents.Response{
				Message: types.Message{
					Role:    types.RoleAssistant,
					Content: expectedSummary,
				},
				Usage: types.TokenUsage{
					PromptTokens:     200,
					CompletionTokens: 25,
					TotalTokens:      225,
				},
			}, nil
		},
	}

	estimator := &MockTokenEstimator{
		estimateTokensFunc: func(ctx context.Context, text string) (int, error) {
			if text == expectedSummary {
				return 25, nil
			}
			return len(text) / 4, nil
		},
	}

	summarizer := NewLLMHistorySummarizer(model, estimator)

	messages := []types.Message{
		*types.UserMessage("Show me Q1 sales data"),
		*types.AssistantMessage("Let me fetch that for you."),
		{
			Role:    types.RoleAssistant,
			Content: "Here's the Q1 sales analysis: revenue grew 20% YoY.",
			ToolCalls: []types.ToolCall{
				{
					ID:        "call_1",
					Name:      "sql_execute",
					Arguments: `{"query": "SELECT * FROM sales WHERE quarter = 'Q1'"}`,
				},
			},
		},
	}

	summary, tokens, err := summarizer.SummarizeMessages(context.Background(), messages, 500)

	require.NoError(t, err)
	assert.Equal(t, expectedSummary, summary)
	assert.Equal(t, 25, tokens) // Uses actual token usage from LLM response
}

func TestLLMHistorySummarizer_SummarizeMessages_TokenReduction(t *testing.T) {
	t.Parallel()

	model := &MockModel{
		generateFunc: func(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) (*agents.Response, error) {
			return &agents.Response{
				Message: types.Message{
					Role:    types.RoleAssistant,
					Content: "Concise summary",
				},
				Usage: types.TokenUsage{
					CompletionTokens: 10,
				},
			}, nil
		},
	}

	estimator := &MockTokenEstimator{
		estimateMessagesFunc: func(ctx context.Context, messages []types.Message) (int, error) {
			// Original messages: large token count
			return 5000, nil
		},
		estimateTokensFunc: func(ctx context.Context, text string) (int, error) {
			// Summary: reduced token count
			return 10, nil
		},
	}

	summarizer := NewLLMHistorySummarizer(model, estimator)

	// Create long conversation
	messages := []types.Message{
		*types.UserMessage("Long message " + string(make([]byte, 1000))),
		*types.AssistantMessage("Long response " + string(make([]byte, 1000))),
		*types.UserMessage("Another long message " + string(make([]byte, 1000))),
		*types.AssistantMessage("Another long response " + string(make([]byte, 1000))),
	}

	originalTokens, _ := estimator.EstimateMessages(context.Background(), messages)
	summary, summaryTokens, err := summarizer.SummarizeMessages(context.Background(), messages, 500)

	require.NoError(t, err)
	assert.NotEmpty(t, summary)
	assert.Less(t, summaryTokens, originalTokens, "Summary should use fewer tokens than original")
	assert.Equal(t, 10, summaryTokens)
}

func TestLLMHistorySummarizer_WithCustomSystemPrompt(t *testing.T) {
	t.Parallel()

	customPrompt := "Custom summarization instructions"

	model := &MockModel{
		generateFunc: func(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) (*agents.Response, error) {
			// Verify custom system prompt is used
			assert.Greater(t, len(req.Messages), 0)
			assert.Equal(t, types.RoleSystem, req.Messages[0].Role)
			assert.Equal(t, customPrompt, req.Messages[0].Content)

			return &agents.Response{
				Message: types.Message{
					Role:    types.RoleAssistant,
					Content: "Custom summary",
				},
			}, nil
		},
	}

	estimator := &MockTokenEstimator{}
	summarizer := NewLLMHistorySummarizer(model, estimator, WithSystemPrompt(customPrompt))

	messages := []types.Message{
		*types.UserMessage("Test message"),
	}

	summary, _, err := summarizer.SummarizeMessages(context.Background(), messages, 500)

	require.NoError(t, err)
	assert.Equal(t, "Custom summary", summary)
}

func TestLLMHistorySummarizer_TargetTokens_MaxTokensCalculation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		targetTokens    int
		expectedMaxToks int
	}{
		{
			name:            "normal target",
			targetTokens:    500,
			expectedMaxToks: 600, // 500 * 1.2
		},
		{
			name:            "small target",
			targetTokens:    50,
			expectedMaxToks: 100, // Minimum
		},
		{
			name:            "zero target",
			targetTokens:    0,
			expectedMaxToks: 100, // Minimum
		},
		{
			name:            "large target",
			targetTokens:    2000,
			expectedMaxToks: 2400, // 2000 * 1.2
		},
	}

	for _, tt := range tests {
		tt := tt // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var receivedMaxTokens int
			model := &MockModel{
				generateFunc: func(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) (*agents.Response, error) {
					// Extract maxTokens from options
					cfg := &agents.GenerateConfig{}
					for _, opt := range opts {
						opt(cfg)
					}
					if cfg.MaxTokens != nil {
						receivedMaxTokens = *cfg.MaxTokens
					}

					return &agents.Response{
						Message: types.Message{
							Role:    types.RoleAssistant,
							Content: "Summary",
						},
					}, nil
				},
			}

			estimator := &MockTokenEstimator{}
			summarizer := NewLLMHistorySummarizer(model, estimator)

			_, _, err := summarizer.SummarizeMessages(context.Background(),
				[]types.Message{*types.UserMessage("Test")},
				tt.targetTokens)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedMaxToks, receivedMaxTokens)
		})
	}
}

func TestLLMHistorySummarizer_ToolCallsIncluded(t *testing.T) {
	t.Parallel()

	model := &MockModel{
		generateFunc: func(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) (*agents.Response, error) {
			// Verify tool calls are mentioned in prompt
			assert.Equal(t, 2, len(req.Messages))
			userPrompt := req.Messages[1].Content
			assert.Contains(t, userPrompt, "tool calls")
			assert.Contains(t, userPrompt, "sql_execute")

			return &agents.Response{
				Message: types.Message{
					Role:    types.RoleAssistant,
					Content: "Summary with tool call info",
				},
			}, nil
		},
	}

	estimator := &MockTokenEstimator{}
	summarizer := NewLLMHistorySummarizer(model, estimator)

	messages := []types.Message{
		{
			Role:    types.RoleAssistant,
			Content: "Executing query...",
			ToolCalls: []types.ToolCall{
				{
					ID:        "call_1",
					Name:      "sql_execute",
					Arguments: `{"query": "SELECT * FROM sales"}`,
				},
			},
		},
	}

	summary, _, err := summarizer.SummarizeMessages(context.Background(), messages, 500)

	require.NoError(t, err)
	assert.Contains(t, summary, "Summary")
}

func TestNoOpSummarizer_AlwaysReturnsEmpty(t *testing.T) {
	t.Parallel()

	summarizer := NewNoOpSummarizer()

	messages := []types.Message{
		*types.UserMessage("Test message"),
		*types.AssistantMessage("Test response"),
	}

	summary, tokens, err := summarizer.SummarizeMessages(context.Background(), messages, 500)

	require.NoError(t, err)
	assert.Empty(t, summary)
	assert.Equal(t, 0, tokens)
}

func TestBuildSummarizationPrompt(t *testing.T) {
	t.Parallel()

	messages := []types.Message{
		*types.UserMessage("Hello"),
		*types.AssistantMessage("Hi there!"),
		{
			Role:    types.RoleAssistant,
			Content: "Running query",
			ToolCalls: []types.ToolCall{
				{Name: "sql_execute"},
				{Name: "export_excel"},
			},
		},
	}

	prompt := buildSummarizationPrompt(messages, 500)

	// Verify structure
	assert.Contains(t, prompt, "Conversation to summarize")
	assert.Contains(t, prompt, "~500 tokens")
	assert.Contains(t, prompt, "[Message 1 - user]")
	assert.Contains(t, prompt, "Hello")
	assert.Contains(t, prompt, "[Message 2 - assistant]")
	assert.Contains(t, prompt, "Hi there!")
	assert.Contains(t, prompt, "[2 tool calls]")
	assert.Contains(t, prompt, "- sql_execute")
	assert.Contains(t, prompt, "- export_excel")
}

func TestBuildSummarizationPrompt_TruncatesLongMessages(t *testing.T) {
	t.Parallel()

	longContent := string(make([]byte, 3000))
	messages := []types.Message{
		*types.UserMessage(longContent),
	}

	prompt := buildSummarizationPrompt(messages, 500)

	// Verify truncation
	assert.Contains(t, prompt, "...")
	assert.Less(t, len(prompt), len(longContent)+500, "Prompt should be truncated")
}
