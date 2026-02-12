package renderers

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/context"
	"github.com/iota-uz/iota-sdk/pkg/bichat/context/codecs"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
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

	// Estimate tokens for all messages
	totalTokens := 0
	for _, msg := range rendered.Messages {
		tokens, err := r.tokenizer.CountTokens(msg.Content())
		if err != nil {
			return 0, err
		}
		totalTokens += tokens
	}

	return totalTokens, nil
}

// renderPinned renders a pinned system block.
func (r *AnthropicRenderer) renderPinned(block context.ContextBlock) (context.RenderedBlock, error) {
	text, err := extractText(block.Payload)
	if err != nil {
		return context.RenderedBlock{}, err
	}
	return context.RenderedBlock{
		Messages: []types.Message{types.SystemMessage(text)},
	}, nil
}

// renderReference renders a reference block.
func (r *AnthropicRenderer) renderReference(block context.ContextBlock) (context.RenderedBlock, error) {
	text, err := extractText(block.Payload)
	if err != nil {
		return context.RenderedBlock{}, err
	}
	return context.RenderedBlock{
		Messages: []types.Message{types.SystemMessage(text)},
	}, nil
}

// renderMemory renders a memory block.
func (r *AnthropicRenderer) renderMemory(block context.ContextBlock) (context.RenderedBlock, error) {
	text, err := extractText(block.Payload)
	if err != nil {
		return context.RenderedBlock{}, err
	}
	return context.RenderedBlock{
		Messages: []types.Message{types.SystemMessage(text)},
	}, nil
}

// renderState renders a state block.
func (r *AnthropicRenderer) renderState(block context.ContextBlock) (context.RenderedBlock, error) {
	text, err := extractText(block.Payload)
	if err != nil {
		return context.RenderedBlock{}, err
	}
	return context.RenderedBlock{
		Messages: []types.Message{types.SystemMessage(text)},
	}, nil
}

// renderToolOutput renders a tool output block.
func (r *AnthropicRenderer) renderToolOutput(block context.ContextBlock) (context.RenderedBlock, error) {
	text, err := extractText(block.Payload)
	if err != nil {
		return context.RenderedBlock{}, err
	}
	return context.RenderedBlock{
		Messages: []types.Message{types.SystemMessage(fmt.Sprintf("Tool output: %s", text))},
	}, nil
}

// renderHistory renders a history block.
func (r *AnthropicRenderer) renderHistory(block context.ContextBlock) (context.RenderedBlock, error) {
	var historyPayload codecs.ConversationHistoryPayload
	switch v := block.Payload.(type) {
	case codecs.ConversationHistoryPayload:
		historyPayload = v
	case map[string]any:
		if messages, ok := v["messages"].([]any); ok {
			for _, msg := range messages {
				if msgMap, ok := msg.(map[string]any); ok {
					role, _ := msgMap["role"].(string)
					content, _ := msgMap["content"].(string)
					historyPayload.Messages = append(historyPayload.Messages, codecs.ConversationMessage{
						Role: role, Content: content,
					})
				}
			}
		}
		if summary, ok := v["summary"].(string); ok {
			historyPayload.Summary = summary
		}
	default:
		if data, err := json.Marshal(block.Payload); err == nil {
			_ = json.Unmarshal(data, &historyPayload)
		}
	}
	messages := make([]types.Message, 0, len(historyPayload.Messages))
	for _, msg := range historyPayload.Messages {
		role := types.RoleUser
		switch msg.Role {
		case "user":
			role = types.RoleUser
		case "assistant":
			role = types.RoleAssistant
		case "system":
			role = types.RoleSystem
		case "tool":
			role = types.RoleTool
		}
		messages = append(messages, types.NewMessage(
			types.WithRole(role),
			types.WithContent(msg.Content),
		))
	}
	if historyPayload.Summary != "" {
		messages = append(messages, types.SystemMessage(fmt.Sprintf("Previous conversation summary: %s", historyPayload.Summary)))
	}
	return context.RenderedBlock{Messages: messages}, nil
}

// renderTurn renders a turn block.
func (r *AnthropicRenderer) renderTurn(block context.ContextBlock) (context.RenderedBlock, error) {
	// Parse turn payload (supports both TurnPayload and string for backward compatibility)
	var turnPayload codecs.TurnPayload

	switch v := block.Payload.(type) {
	case codecs.TurnPayload:
		turnPayload = v
	case map[string]any:
		if content, ok := v["content"].(string); ok {
			turnPayload.Content = content
		}
		if attachments, ok := v["attachments"].([]any); ok {
			for _, att := range attachments {
				if attMap, ok := att.(map[string]any); ok {
					fileName := ""
					if v, ok := attMap["fileName"].(string); ok {
						fileName = v
					}
					mimeType := ""
					if v, ok := attMap["mimeType"].(string); ok {
						mimeType = v
					}
					sizeBytes := int64(0)
					if v, ok := attMap["sizeBytes"].(float64); ok {
						sizeBytes = int64(v)
					} else if v, ok := attMap["sizeBytes"].(int64); ok {
						sizeBytes = v
					} else if v, ok := attMap["sizeBytes"].(int); ok {
						sizeBytes = int64(v)
					}
					reference := ""
					if v, ok := attMap["reference"].(string); ok {
						reference = v
					}
					turnPayload.Attachments = append(turnPayload.Attachments, codecs.TurnAttachment{
						FileName:  fileName,
						MimeType:  mimeType,
						SizeBytes: sizeBytes,
						Reference: reference,
					})
				}
			}
		}
	case string:
		turnPayload.Content = v
	default:
		text, err := extractText(block.Payload)
		if err != nil {
			return context.RenderedBlock{}, err
		}
		turnPayload.Content = text
	}

	// Convert attachments
	attachments := make([]types.Attachment, 0, len(turnPayload.Attachments))
	for _, att := range turnPayload.Attachments {
		var attID uuid.UUID
		if len(att.Reference) > 11 && att.Reference[:11] == "attachment:" {
			if id, err := uuid.Parse(att.Reference[11:]); err == nil {
				attID = id
			}
		}
		attachments = append(attachments, types.Attachment{
			ID: attID, FileName: att.FileName, MimeType: att.MimeType,
			SizeBytes: att.SizeBytes, FilePath: att.Reference,
		})
	}

	opts := []types.MessageOption{types.WithRole(types.RoleUser), types.WithContent(turnPayload.Content)}
	if len(attachments) > 0 {
		opts = append(opts, types.WithAttachments(attachments...))
	}
	msg := types.NewMessage(opts...)
	return context.RenderedBlock{Messages: []types.Message{msg}}, nil
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
