package agents

import (
	"context"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/pkoukk/tiktoken-go"
)

// TokenEstimator estimates the number of tokens in text or messages.
// Used for cost tracking and token budget management.
type TokenEstimator interface {
	// EstimateTokens estimates tokens in a single text string.
	EstimateTokens(ctx context.Context, text string) (int, error)

	// EstimateMessages estimates total tokens across multiple messages.
	// Includes overhead for message formatting and role tokens.
	EstimateMessages(ctx context.Context, messages []types.Message) (int, error)
}

// TiktokenEstimator uses the tiktoken library for accurate token counting.
// Supports provider-specific encodings (GPT, Claude via cl100k_base).
//
// Accuracy: High (official OpenAI tokenizer)
// Performance: ~10-50ms per message depending on length
// Dependencies: github.com/pkoukk/tiktoken-go
type TiktokenEstimator struct {
	encoding string // e.g., "cl100k_base" (GPT-4, GPT-3.5-turbo, Claude)
}

// NewTiktokenEstimator creates an accurate token estimator using tiktoken.
//
// Common encodings:
//   - "cl100k_base": GPT-4, GPT-3.5-turbo, Claude (default)
//   - "p50k_base": GPT-3, Codex
//   - "r50k_base": GPT-3 (legacy)
//
// Example:
//
//	estimator := agents.NewTiktokenEstimator("cl100k_base")
//	tokens, _ := estimator.EstimateTokens(ctx, "Hello world")
func NewTiktokenEstimator(encoding string) TokenEstimator {
	if encoding == "" {
		encoding = "cl100k_base" // Default: GPT-4/Claude encoding
	}
	return &TiktokenEstimator{encoding: encoding}
}

// EstimateTokens estimates tokens in text using tiktoken.
func (e *TiktokenEstimator) EstimateTokens(ctx context.Context, text string) (int, error) {
	const op = serrors.Op("agents.TiktokenEstimator.EstimateTokens")

	if text == "" {
		return 0, nil
	}

	// Get tiktoken encoding
	tkm, err := tiktoken.GetEncoding(e.encoding)
	if err != nil {
		return 0, serrors.E(op, err)
	}

	// Encode text to tokens
	tokens := tkm.Encode(text, nil, nil)
	return len(tokens), nil
}

// EstimateMessages estimates total tokens across messages.
// Includes overhead for message structure and role tokens.
func (e *TiktokenEstimator) EstimateMessages(ctx context.Context, messages []types.Message) (int, error) {
	const op = serrors.Op("agents.TiktokenEstimator.EstimateMessages")

	if len(messages) == 0 {
		return 0, nil
	}

	totalTokens := 0

	// Get tiktoken encoding
	tkm, err := tiktoken.GetEncoding(e.encoding)
	if err != nil {
		return 0, serrors.E(op, err)
	}

	for _, msg := range messages {
		// Message overhead (role formatting)
		// Format: <|im_start|>role\ncontent<|im_end|>
		totalTokens += 4 // Overhead per message

		// Role tokens
		roleTokens := tkm.Encode(string(msg.Role), nil, nil)
		totalTokens += len(roleTokens)

		// Content tokens
		if msg.Content != "" {
			contentTokens := tkm.Encode(msg.Content, nil, nil)
			totalTokens += len(contentTokens)
		}

		// Tool calls tokens (if present)
		for _, toolCall := range msg.ToolCalls {
			// Tool name
			nameTokens := tkm.Encode(toolCall.Name, nil, nil)
			totalTokens += len(nameTokens)

			// Tool arguments (JSON)
			if toolCall.Arguments != "" {
				argsTokens := tkm.Encode(toolCall.Arguments, nil, nil)
				totalTokens += len(argsTokens)
			}

			// Overhead for tool call structure
			totalTokens += 3
		}
	}

	// Add 2 tokens for priming the response
	totalTokens += 2

	return totalTokens, nil
}

// CharacterBasedEstimator provides fast token approximation using character counting.
// Rule of thumb: ~4 characters per token for English text.
//
// Accuracy: Medium (~80-90% accurate)
// Performance: <1ms (no external dependencies)
// Use Case: Quick estimates, cost preview, rate limiting
type CharacterBasedEstimator struct {
	charsPerToken float64
}

// NewCharacterBasedEstimator creates a fast character-based token estimator.
//
// charsPerToken: Average characters per token (typically 3.5-4.5 for English)
//   - 4.0: Conservative estimate (default)
//   - 3.5: Aggressive estimate (more tokens)
//   - 4.5: Optimistic estimate (fewer tokens)
//
// Example:
//
//	estimator := agents.NewCharacterBasedEstimator(4.0)
//	tokens, _ := estimator.EstimateTokens(ctx, "Hello world")
func NewCharacterBasedEstimator(charsPerToken float64) TokenEstimator {
	if charsPerToken <= 0 {
		charsPerToken = 4.0 // Default: ~4 chars per token
	}
	return &CharacterBasedEstimator{charsPerToken: charsPerToken}
}

// EstimateTokens estimates tokens using character count heuristic.
func (e *CharacterBasedEstimator) EstimateTokens(ctx context.Context, text string) (int, error) {
	if text == "" {
		return 0, nil
	}

	// Count characters (excluding whitespace for better accuracy)
	chars := len(strings.TrimSpace(text))
	tokens := int(float64(chars) / e.charsPerToken)

	// Minimum 1 token for non-empty text
	if tokens == 0 && chars > 0 {
		tokens = 1
	}

	return tokens, nil
}

// EstimateMessages estimates total tokens across messages using character counts.
func (e *CharacterBasedEstimator) EstimateMessages(ctx context.Context, messages []types.Message) (int, error) {
	if len(messages) == 0 {
		return 0, nil
	}

	totalTokens := 0

	for _, msg := range messages {
		// Message overhead (role + formatting)
		totalTokens += 4

		// Role tokens (~1 token per role)
		totalTokens += 1

		// Content tokens
		if msg.Content != "" {
			contentChars := len(strings.TrimSpace(msg.Content))
			totalTokens += int(float64(contentChars) / e.charsPerToken)
		}

		// Tool calls tokens
		for _, toolCall := range msg.ToolCalls {
			// Tool name (~2-5 tokens)
			nameChars := len(toolCall.Name)
			totalTokens += int(float64(nameChars)/e.charsPerToken) + 1

			// Tool arguments
			if toolCall.Arguments != "" {
				argsChars := len(strings.TrimSpace(toolCall.Arguments))
				totalTokens += int(float64(argsChars) / e.charsPerToken)
			}

			// Tool call overhead
			totalTokens += 3
		}
	}

	// Response priming tokens
	totalTokens += 2

	return totalTokens, nil
}

// NoOpTokenEstimator is a no-op estimator that always returns 0.
// Used as a default when token estimation is disabled.
type NoOpTokenEstimator struct{}

// NewNoOpTokenEstimator creates a no-op token estimator.
func NewNoOpTokenEstimator() TokenEstimator {
	return &NoOpTokenEstimator{}
}

// EstimateTokens always returns 0.
func (e *NoOpTokenEstimator) EstimateTokens(ctx context.Context, text string) (int, error) {
	return 0, nil
}

// EstimateMessages always returns 0.
func (e *NoOpTokenEstimator) EstimateMessages(ctx context.Context, messages []types.Message) (int, error) {
	return 0, nil
}
