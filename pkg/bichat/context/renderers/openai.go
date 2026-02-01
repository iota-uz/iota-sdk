package renderers

import (
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/bichat/context"
)

// OpenAIRenderer renders blocks for OpenAI GPT models.
type OpenAIRenderer struct {
	tokenizer context.Tokenizer
}

// OpenAIOption configures the OpenAI renderer.
type OpenAIOption func(*OpenAIRenderer)

// WithOpenAITokenizer sets a custom tokenizer for the renderer.
func WithOpenAITokenizer(tokenizer context.Tokenizer) OpenAIOption {
	return func(r *OpenAIRenderer) {
		r.tokenizer = tokenizer
	}
}

// NewOpenAIRenderer creates a new renderer for OpenAI GPT models.
func NewOpenAIRenderer(opts ...OpenAIOption) *OpenAIRenderer {
	r := &OpenAIRenderer{
		tokenizer: context.NewSimpleTokenizer(),
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// Provider returns the provider identifier.
func (r *OpenAIRenderer) Provider() string {
	return "openai"
}

// Render converts a block to OpenAI format.
func (r *OpenAIRenderer) Render(block context.ContextBlock) (context.RenderedBlock, error) {
	switch block.Meta.Kind {
	case context.KindPinned:
		return r.renderSystem(block)
	case context.KindReference:
		return r.renderSystem(block)
	case context.KindMemory:
		return r.renderSystem(block)
	case context.KindState:
		return r.renderSystem(block)
	case context.KindToolOutput:
		return r.renderToolOutput(block)
	case context.KindHistory:
		return r.renderHistory(block)
	case context.KindTurn:
		return r.renderTurn(block)
	default:
		return context.RenderedBlock{}, fmt.Errorf("unknown block kind: %s", block.Meta.Kind)
	}
}

// EstimateTokens estimates the token count for a block.
func (r *OpenAIRenderer) EstimateTokens(block context.ContextBlock) (int, error) {
	rendered, err := r.Render(block)
	if err != nil {
		return 0, err
	}

	// Estimate system content
	systemTokens := 0
	if rendered.SystemContent != "" {
		systemTokens, err = r.tokenizer.CountTokens(rendered.SystemContent)
		if err != nil {
			return 0, err
		}
	}

	// Estimate message content
	messageTokens := 0
	if rendered.Message != nil {
		if msg, ok := rendered.Message.(map[string]any); ok {
			if content, ok := msg["content"].(string); ok {
				messageTokens, err = r.tokenizer.CountTokens(content)
				if err != nil {
					return 0, err
				}
			}
		}
	}

	return systemTokens + messageTokens, nil
}

// renderSystem renders system-level blocks.
func (r *OpenAIRenderer) renderSystem(block context.ContextBlock) (context.RenderedBlock, error) {
	text, err := extractText(block.Payload)
	if err != nil {
		return context.RenderedBlock{}, err
	}

	// OpenAI uses system messages instead of system prompt
	return context.RenderedBlock{
		Message: map[string]any{
			"role":    "system",
			"content": text,
		},
	}, nil
}

// renderToolOutput renders a tool output block.
func (r *OpenAIRenderer) renderToolOutput(block context.ContextBlock) (context.RenderedBlock, error) {
	text, err := extractText(block.Payload)
	if err != nil {
		return context.RenderedBlock{}, err
	}

	return context.RenderedBlock{
		Message: map[string]any{
			"role":    "assistant",
			"content": text,
		},
	}, nil
}

// renderHistory renders a history block.
func (r *OpenAIRenderer) renderHistory(block context.ContextBlock) (context.RenderedBlock, error) {
	return context.RenderedBlock{
		Message: block.Payload,
	}, nil
}

// renderTurn renders a turn block.
func (r *OpenAIRenderer) renderTurn(block context.ContextBlock) (context.RenderedBlock, error) {
	text, err := extractText(block.Payload)
	if err != nil {
		return context.RenderedBlock{}, err
	}

	return context.RenderedBlock{
		Message: map[string]any{
			"role":    "user",
			"content": text,
		},
	}, nil
}
