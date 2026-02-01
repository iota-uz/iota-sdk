package context

import (
	"context"
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

// HistorySummarizer compresses conversation history using LLM-based summarization.
// Used when OverflowCompact strategy is enabled to intelligently reduce token usage
// while preserving important context.
type HistorySummarizer interface {
	// SummarizeMessages condenses multiple messages into a summary.
	// Returns the summary text, tokens used in the summary, and any error.
	//
	// Parameters:
	//   - ctx: Context for cancellation and deadlines
	//   - messages: Messages to summarize (typically from KindHistory blocks)
	//   - targetTokens: Desired token count for the summary
	//
	// Returns:
	//   - summary: Condensed text representing the conversation
	//   - tokensUsed: Actual tokens in the summary
	//   - error: Any error during summarization
	SummarizeMessages(
		ctx context.Context,
		messages []types.Message,
		targetTokens int,
	) (summary string, tokensUsed int, error error)
}

// LLMHistorySummarizer uses an LLM to generate intelligent conversation summaries.
//
// The summarizer:
//   - Extracts key facts, decisions, and outcomes
//   - Preserves important user preferences and requirements
//   - Maintains chronological coherence
//   - Respects target token count
//
// Example:
//
//	summarizer := context.NewLLMHistorySummarizer(
//	    model,
//	    estimator,
//	    context.WithSystemPrompt("Condense this conversation..."),
//	)
type LLMHistorySummarizer struct {
	model        agents.Model
	estimator    agents.TokenEstimator
	systemPrompt string
}

// SummarizerOption configures an LLMHistorySummarizer.
type SummarizerOption func(*LLMHistorySummarizer)

// WithSystemPrompt sets a custom system prompt for summarization.
// Default prompt focuses on extracting key facts and decisions.
func WithSystemPrompt(prompt string) SummarizerOption {
	return func(s *LLMHistorySummarizer) {
		s.systemPrompt = prompt
	}
}

// NewLLMHistorySummarizer creates an LLM-powered conversation summarizer.
//
// Parameters:
//   - model: LLM model for generating summaries
//   - estimator: Token estimator for tracking token usage
//   - opts: Optional configuration (system prompt override)
//
// Example:
//
//	summarizer := context.NewLLMHistorySummarizer(
//	    anthropic.NewModel(client, config),
//	    agents.NewTiktokenEstimator("cl100k_base"),
//	)
func NewLLMHistorySummarizer(
	model agents.Model,
	estimator agents.TokenEstimator,
	opts ...SummarizerOption,
) HistorySummarizer {
	s := &LLMHistorySummarizer{
		model:     model,
		estimator: estimator,
		systemPrompt: `You are a conversation summarizer. Condense the following conversation into a concise summary.

Focus on:
- Key facts, figures, and data mentioned
- Important decisions or outcomes
- User preferences and requirements
- Critical context for future turns

Keep the summary chronological and factual. Omit greetings and filler.`,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// SummarizeMessages generates an LLM-powered summary of conversation history.
func (s *LLMHistorySummarizer) SummarizeMessages(
	ctx context.Context,
	messages []types.Message,
	targetTokens int,
) (string, int, error) {
	const op = "LLMHistorySummarizer.SummarizeMessages"

	if len(messages) == 0 {
		return "", 0, nil
	}

	// Build prompt for summarization
	userPrompt := buildSummarizationPrompt(messages, targetTokens)

	// Create request
	req := agents.Request{
		Messages: []types.Message{
			*types.SystemMessage(s.systemPrompt),
			*types.UserMessage(userPrompt),
		},
	}

	// Set max tokens for summary (use target with 20% buffer)
	maxTokens := int(float64(targetTokens) * 1.2)
	if maxTokens < 100 {
		maxTokens = 100 // Minimum viable summary
	}

	// Generate summary
	resp, err := s.model.Generate(ctx, req, agents.WithMaxTokens(maxTokens))
	if err != nil {
		return "", 0, fmt.Errorf("%s: failed to generate summary: %w", op, err)
	}

	summary := resp.Message.Content

	// Estimate tokens in summary
	tokensUsed := 0
	if s.estimator != nil {
		tokensUsed, _ = s.estimator.EstimateTokens(ctx, summary)
	}

	// Use actual token usage from LLM if available
	if resp.Usage.CompletionTokens > 0 {
		tokensUsed = resp.Usage.CompletionTokens
	}

	return summary, tokensUsed, nil
}

// buildSummarizationPrompt creates a user prompt from messages.
func buildSummarizationPrompt(messages []types.Message, targetTokens int) string {
	var prompt string
	prompt += fmt.Sprintf("Conversation to summarize (target summary: ~%d tokens):\n\n", targetTokens)

	for i, msg := range messages {
		role := string(msg.Role)
		content := msg.Content

		// Truncate very long messages to avoid excessive prompt size
		if len(content) > 2000 {
			content = content[:2000] + "..."
		}

		prompt += fmt.Sprintf("[Message %d - %s]\n%s\n\n", i+1, role, content)

		// Include tool call summaries if present
		if len(msg.ToolCalls) > 0 {
			prompt += fmt.Sprintf("  [%d tool calls]\n", len(msg.ToolCalls))
			for _, tc := range msg.ToolCalls {
				prompt += fmt.Sprintf("    - %s\n", tc.Name)
			}
			prompt += "\n"
		}
	}

	return prompt
}

// NoOpSummarizer is a no-op summarizer that returns empty summaries.
// Used as a default when summarization is disabled.
type NoOpSummarizer struct{}

// NewNoOpSummarizer creates a no-op summarizer.
func NewNoOpSummarizer() HistorySummarizer {
	return &NoOpSummarizer{}
}

// SummarizeMessages always returns an empty summary.
func (s *NoOpSummarizer) SummarizeMessages(
	ctx context.Context,
	messages []types.Message,
	targetTokens int,
) (string, int, error) {
	return "", 0, nil
}
