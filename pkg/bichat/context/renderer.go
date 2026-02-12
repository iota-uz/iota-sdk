package context

import "github.com/iota-uz/iota-sdk/pkg/bichat/types"

// Renderer converts blocks to provider-specific formats and estimates tokens.
// Each provider (Anthropic, OpenAI, Gemini) has different message formats and tokenization.
type Renderer interface {
	// Render converts a block to provider-specific format.
	Render(block ContextBlock) (RenderedBlock, error)

	// EstimateTokens returns the token count for a block using provider-specific tokenization.
	EstimateTokens(block ContextBlock) (int, error)

	// Provider returns the provider identifier.
	Provider() string
}

// RenderedBlock is the type-safe output of rendering.
// Renderers now produce canonical types.Message instances instead of provider-specific formats.
type RenderedBlock struct {
	// Messages contains 0..n canonical messages from this block.
	// System blocks produce system-role messages.
	// History blocks produce one message per history entry.
	// Turn blocks produce user-role messages.
	Messages []types.Message

	// Metadata contains provider-specific metadata.
	Metadata map[string]any
}

// Tokenizer estimates tokens for text content.
// Providers can implement custom tokenizers for accurate token counting.
type Tokenizer interface {
	// CountTokens returns the number of tokens in the given text.
	CountTokens(text string) (int, error)
}

// SimpleTokenizer provides a basic word-based tokenization estimate.
// For production use, providers should implement accurate tokenizers.
type SimpleTokenizer struct {
	// TokensPerWord is the average tokens per word (default: 1.3 for English).
	TokensPerWord float64
}

// NewSimpleTokenizer creates a new SimpleTokenizer with default settings.
func NewSimpleTokenizer() *SimpleTokenizer {
	return &SimpleTokenizer{
		TokensPerWord: 1.3, // Rough estimate for English
	}
}

// CountTokens estimates tokens using word count.
func (t *SimpleTokenizer) CountTokens(text string) (int, error) {
	// Simple word-based estimation
	wordCount := 0
	inWord := false

	for _, r := range text {
		if r == ' ' || r == '\n' || r == '\t' || r == '\r' {
			inWord = false
		} else if !inWord {
			wordCount++
			inWord = true
		}
	}

	return int(float64(wordCount) * t.TokensPerWord), nil
}
