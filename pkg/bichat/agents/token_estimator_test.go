package agents

import (
	"context"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTiktokenEstimator_EstimateTokens(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	estimator := NewTiktokenEstimator("cl100k_base")

	tests := []struct {
		name         string
		text         string
		wantMinToken int
		wantMaxToken int
	}{
		{
			name:         "empty string",
			text:         "",
			wantMinToken: 0,
			wantMaxToken: 0,
		},
		{
			name:         "simple phrase",
			text:         "Hello world",
			wantMinToken: 2,
			wantMaxToken: 3,
		},
		{
			name:         "longer sentence",
			text:         "The quick brown fox jumps over the lazy dog",
			wantMinToken: 9,
			wantMaxToken: 11,
		},
		{
			name:         "technical text",
			text:         "SELECT * FROM users WHERE tenant_id = $1 AND status = 'active'",
			wantMinToken: 15,
			wantMaxToken: 20,
		},
		{
			name:         "JSON data",
			text:         `{"query": "revenue", "limit": 10, "filters": {"year": 2024}}`,
			wantMinToken: 15,
			wantMaxToken: 25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := estimator.EstimateTokens(ctx, tt.text)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, tokens, tt.wantMinToken, "tokens should be >= min")
			assert.LessOrEqual(t, tokens, tt.wantMaxToken, "tokens should be <= max")
		})
	}
}

func TestTiktokenEstimator_EstimateMessages(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	estimator := NewTiktokenEstimator("cl100k_base")

	tests := []struct {
		name         string
		messages     []types.Message
		wantMinToken int
		wantMaxToken int
	}{
		{
			name:         "empty messages",
			messages:     []types.Message{},
			wantMinToken: 0,
			wantMaxToken: 0,
		},
		{
			name: "single user message",
			messages: []types.Message{
				types.UserMessage("Show me sales data"),
			},
			wantMinToken: 8,
			wantMaxToken: 12,
		},
		{
			name: "conversation with assistant",
			messages: []types.Message{
				types.UserMessage("What is revenue?"),
				types.AssistantMessage("Revenue is the total income."),
			},
			wantMinToken: 15,
			wantMaxToken: 25,
		},
		{
			name: "message with tool calls",
			messages: []types.Message{
				types.AssistantMessage("Let me check the database.",
					types.WithToolCalls(
						types.ToolCall{
							ID:        "call-1",
							Name:      "sql_execute",
							Arguments: `{"query": "SELECT * FROM sales"}`,
						},
					),
				),
			},
			wantMinToken: 20,
			wantMaxToken: 35,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := estimator.EstimateMessages(ctx, tt.messages)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, tokens, tt.wantMinToken, "tokens should be >= min")
			assert.LessOrEqual(t, tokens, tt.wantMaxToken, "tokens should be <= max")
		})
	}
}

func TestCharacterBasedEstimator_EstimateTokens(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	estimator := NewCharacterBasedEstimator(4.0)

	tests := []struct {
		name         string
		text         string
		wantMinToken int
		wantMaxToken int
	}{
		{
			name:         "empty string",
			text:         "",
			wantMinToken: 0,
			wantMaxToken: 0,
		},
		{
			name:         "simple phrase",
			text:         "Hello world",
			wantMinToken: 2,
			wantMaxToken: 3,
		},
		{
			name:         "longer text",
			text:         "The quick brown fox jumps over the lazy dog",
			wantMinToken: 9,
			wantMaxToken: 12,
		},
		{
			name:         "whitespace handling",
			text:         "   trim   spaces   ",
			wantMinToken: 3,
			wantMaxToken: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := estimator.EstimateTokens(ctx, tt.text)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, tokens, tt.wantMinToken, "tokens should be >= min")
			assert.LessOrEqual(t, tokens, tt.wantMaxToken, "tokens should be <= max")
		})
	}
}

func TestCharacterBasedEstimator_EstimateMessages(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	estimator := NewCharacterBasedEstimator(4.0)

	tests := []struct {
		name         string
		messages     []types.Message
		wantMinToken int
		wantMaxToken int
	}{
		{
			name:         "empty messages",
			messages:     []types.Message{},
			wantMinToken: 0,
			wantMaxToken: 0,
		},
		{
			name: "single user message",
			messages: []types.Message{
				types.UserMessage("Show me sales data"),
			},
			wantMinToken: 8,
			wantMaxToken: 12,
		},
		{
			name: "conversation with assistant",
			messages: []types.Message{
				types.UserMessage("What is revenue?"),
				types.AssistantMessage("Revenue is the total income."),
			},
			wantMinToken: 15,
			wantMaxToken: 25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := estimator.EstimateMessages(ctx, tt.messages)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, tokens, tt.wantMinToken, "tokens should be >= min")
			assert.LessOrEqual(t, tokens, tt.wantMaxToken, "tokens should be <= max")
		})
	}
}

func TestCharacterBasedEstimator_ConfigurableCharsPerToken(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	text := "This is a test sentence with exactly fifty chars."

	tests := []struct {
		name          string
		charsPerToken float64
		wantTokens    int
	}{
		{
			name:          "conservative (4.0)",
			charsPerToken: 4.0,
			wantTokens:    12, // 49/4.0 = 12.25 → 12
		},
		{
			name:          "aggressive (3.5)",
			charsPerToken: 3.5,
			wantTokens:    14, // 49/3.5 = 14.0 → 14
		},
		{
			name:          "optimistic (4.5)",
			charsPerToken: 4.5,
			wantTokens:    10, // 49/4.5 = 10.88 → 10
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			estimator := NewCharacterBasedEstimator(tt.charsPerToken)
			tokens, err := estimator.EstimateTokens(ctx, text)
			require.NoError(t, err)
			assert.Equal(t, tt.wantTokens, tokens)
		})
	}
}

func TestNoOpTokenEstimator(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	estimator := NewNoOpTokenEstimator()

	// EstimateTokens always returns 0
	tokens, err := estimator.EstimateTokens(ctx, "any text")
	require.NoError(t, err)
	assert.Equal(t, 0, tokens)

	// EstimateMessages always returns 0
	messages := []types.Message{
		types.UserMessage("test"),
	}
	tokens, err = estimator.EstimateMessages(ctx, messages)
	require.NoError(t, err)
	assert.Equal(t, 0, tokens)
}

// Benchmark tests to compare performance
func BenchmarkTiktokenEstimator_EstimateTokens(b *testing.B) {
	ctx := context.Background()
	estimator := NewTiktokenEstimator("cl100k_base")
	text := "The quick brown fox jumps over the lazy dog"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = estimator.EstimateTokens(ctx, text)
	}
}

func BenchmarkCharacterBasedEstimator_EstimateTokens(b *testing.B) {
	ctx := context.Background()
	estimator := NewCharacterBasedEstimator(4.0)
	text := "The quick brown fox jumps over the lazy dog"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = estimator.EstimateTokens(ctx, text)
	}
}

func BenchmarkTiktokenEstimator_EstimateMessages(b *testing.B) {
	ctx := context.Background()
	estimator := NewTiktokenEstimator("cl100k_base")
	messages := []types.Message{
		types.UserMessage("What is revenue?"),
		types.AssistantMessage("Revenue is the total income."),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = estimator.EstimateMessages(ctx, messages)
	}
}

func BenchmarkCharacterBasedEstimator_EstimateMessages(b *testing.B) {
	ctx := context.Background()
	estimator := NewCharacterBasedEstimator(4.0)
	messages := []types.Message{
		types.UserMessage("What is revenue?"),
		types.AssistantMessage("Revenue is the total income."),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = estimator.EstimateMessages(ctx, messages)
	}
}

// Accuracy comparison test
func TestAccuracyComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping accuracy comparison in short mode")
	}

	ctx := context.Background()
	tiktoken := NewTiktokenEstimator("cl100k_base")
	charBased := NewCharacterBasedEstimator(4.0)

	testCases := []string{
		"Hello world",
		"The quick brown fox jumps over the lazy dog",
		"SELECT * FROM users WHERE tenant_id = $1",
		`{"query": "revenue", "limit": 10}`,
	}

	for _, text := range testCases {
		t.Run(text[:min(20, len(text))], func(t *testing.T) {
			tiktokenCount, err := tiktoken.EstimateTokens(ctx, text)
			require.NoError(t, err)

			charCount, err := charBased.EstimateTokens(ctx, text)
			require.NoError(t, err)

			// Character-based should be within 30% of tiktoken
			diff := float64(abs(tiktokenCount-charCount)) / float64(tiktokenCount)
			assert.LessOrEqual(t, diff, 0.3, "character-based estimate should be within 30%% of tiktoken")

			t.Logf("Text: %q\n  Tiktoken: %d\n  CharBased: %d\n  Diff: %.2f%%",
				text, tiktokenCount, charCount, diff*100)
		})
	}
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
