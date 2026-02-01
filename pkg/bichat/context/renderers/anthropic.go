package renderers

import (
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/bichat/context"
)

// AnthropicRenderer renders blocks for Anthropic Claude models.
type AnthropicRenderer struct {
	tokenizer context.Tokenizer
}

// AnthropicOption configures the Anthropic renderer.
type AnthropicOption func(*AnthropicRenderer)

// WithAnthropicTokenizer sets a custom tokenizer for the renderer.
func WithAnthropicTokenizer(tokenizer context.Tokenizer) AnthropicOption {
	return func(r *AnthropicRenderer) {
		r.tokenizer = tokenizer
	}
}

// NewAnthropicRenderer creates a new renderer for Anthropic Claude models.
func NewAnthropicRenderer(opts ...AnthropicOption) *AnthropicRenderer {
	r := &AnthropicRenderer{
		tokenizer: context.NewSimpleTokenizer(),
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// Provider returns the provider identifier.
func (r *AnthropicRenderer) Provider() string {
	return "anthropic"
}

// Render converts a block to Anthropic format.
func (r *AnthropicRenderer) Render(block context.ContextBlock) (context.RenderedBlock, error) {
	switch block.Meta.Kind {
	case context.KindPinned:
		return r.renderPinned(block)
	case context.KindReference:
		return r.renderReference(block)
	case context.KindMemory:
		return r.renderMemory(block)
	case context.KindState:
		return r.renderState(block)
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
func (r *AnthropicRenderer) EstimateTokens(block context.ContextBlock) (int, error) {
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

// renderPinned renders a pinned system block.
func (r *AnthropicRenderer) renderPinned(block context.ContextBlock) (context.RenderedBlock, error) {
	// Extract text from payload
	text, err := extractText(block.Payload)
	if err != nil {
		return context.RenderedBlock{}, err
	}

	return context.RenderedBlock{
		SystemContent: text,
	}, nil
}

// renderReference renders a reference block.
func (r *AnthropicRenderer) renderReference(block context.ContextBlock) (context.RenderedBlock, error) {
	// References typically go in system prompt for Anthropic
	text, err := extractText(block.Payload)
	if err != nil {
		return context.RenderedBlock{}, err
	}

	return context.RenderedBlock{
		SystemContent: text,
	}, nil
}

// renderMemory renders a memory block.
func (r *AnthropicRenderer) renderMemory(block context.ContextBlock) (context.RenderedBlock, error) {
	// Memory blocks go in system prompt
	text, err := extractText(block.Payload)
	if err != nil {
		return context.RenderedBlock{}, err
	}

	return context.RenderedBlock{
		SystemContent: text,
	}, nil
}

// renderState renders a state block.
func (r *AnthropicRenderer) renderState(block context.ContextBlock) (context.RenderedBlock, error) {
	// State blocks go in system prompt
	text, err := extractText(block.Payload)
	if err != nil {
		return context.RenderedBlock{}, err
	}

	return context.RenderedBlock{
		SystemContent: text,
	}, nil
}

// renderToolOutput renders a tool output block.
func (r *AnthropicRenderer) renderToolOutput(block context.ContextBlock) (context.RenderedBlock, error) {
	// Tool outputs are messages with role "user" and tool_result
	// For simplicity, we'll render as a user message with the tool output
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

// renderHistory renders a history block.
func (r *AnthropicRenderer) renderHistory(block context.ContextBlock) (context.RenderedBlock, error) {
	// History blocks are already in message format
	// For simplicity, we'll assume the payload is a slice of messages
	// Real implementation would use the conversation history codec
	return context.RenderedBlock{
		Message: block.Payload,
	}, nil
}

// renderTurn renders a turn block.
func (r *AnthropicRenderer) renderTurn(block context.ContextBlock) (context.RenderedBlock, error) {
	// Turn blocks are user messages
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

// extractText is a helper to extract text from various payload formats.
func extractText(payload any) (string, error) {
	switch v := payload.(type) {
	case string:
		return v, nil
	case map[string]any:
		if text, ok := v["text"].(string); ok {
			return text, nil
		}
		if content, ok := v["content"].(string); ok {
			return content, nil
		}
		// Try to extract all string values
		var parts []string
		for _, val := range v {
			if s, ok := val.(string); ok {
				parts = append(parts, s)
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n"), nil
		}
		return "", fmt.Errorf("no text field found in payload map")
	default:
		return fmt.Sprintf("%v", v), nil
	}
}
