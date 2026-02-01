package renderers

import (
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/bichat/context"
)

// GeminiRenderer renders blocks for Google Gemini models.
type GeminiRenderer struct {
	tokenizer context.Tokenizer
}

// GeminiOption configures the Gemini renderer.
type GeminiOption func(*GeminiRenderer)

// WithGeminiTokenizer sets a custom tokenizer for the renderer.
func WithGeminiTokenizer(tokenizer context.Tokenizer) GeminiOption {
	return func(r *GeminiRenderer) {
		r.tokenizer = tokenizer
	}
}

// NewGeminiRenderer creates a new renderer for Google Gemini models.
func NewGeminiRenderer(opts ...GeminiOption) *GeminiRenderer {
	r := &GeminiRenderer{
		tokenizer: context.NewSimpleTokenizer(),
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// Provider returns the provider identifier.
func (r *GeminiRenderer) Provider() string {
	return "gemini"
}

// Render converts a block to Gemini format.
func (r *GeminiRenderer) Render(block context.ContextBlock) (context.RenderedBlock, error) {
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
func (r *GeminiRenderer) EstimateTokens(block context.ContextBlock) (int, error) {
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
			if parts, ok := msg["parts"].([]map[string]string); ok {
				for _, part := range parts {
					if text, ok := part["text"]; ok {
						tokens, err := r.tokenizer.CountTokens(text)
						if err != nil {
							return 0, err
						}
						messageTokens += tokens
					}
				}
			}
		}
	}

	return systemTokens + messageTokens, nil
}

// renderSystem renders system-level blocks.
func (r *GeminiRenderer) renderSystem(block context.ContextBlock) (context.RenderedBlock, error) {
	text, err := extractText(block.Payload)
	if err != nil {
		return context.RenderedBlock{}, err
	}

	// Gemini uses system instructions in a different format
	return context.RenderedBlock{
		SystemContent: text,
	}, nil
}

// renderToolOutput renders a tool output block.
func (r *GeminiRenderer) renderToolOutput(block context.ContextBlock) (context.RenderedBlock, error) {
	text, err := extractText(block.Payload)
	if err != nil {
		return context.RenderedBlock{}, err
	}

	return context.RenderedBlock{
		Message: map[string]any{
			"role": "model",
			"parts": []map[string]string{
				{"text": text},
			},
		},
	}, nil
}

// renderHistory renders a history block.
func (r *GeminiRenderer) renderHistory(block context.ContextBlock) (context.RenderedBlock, error) {
	return context.RenderedBlock{
		Message: block.Payload,
	}, nil
}

// renderTurn renders a turn block.
func (r *GeminiRenderer) renderTurn(block context.ContextBlock) (context.RenderedBlock, error) {
	text, err := extractText(block.Payload)
	if err != nil {
		return context.RenderedBlock{}, err
	}

	return context.RenderedBlock{
		Message: map[string]any{
			"role": "user",
			"parts": []map[string]string{
				{"text": text},
			},
		},
	}, nil
}
