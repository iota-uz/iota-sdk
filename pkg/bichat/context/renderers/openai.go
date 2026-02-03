package renderers

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/context"
	"github.com/iota-uz/iota-sdk/pkg/bichat/context/codecs"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
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

// renderSystem renders system-level blocks.
func (r *OpenAIRenderer) renderSystem(block context.ContextBlock) (context.RenderedBlock, error) {
	text, err := extractText(block.Payload)
	if err != nil {
		return context.RenderedBlock{}, err
	}

	// Emit canonical system message
	return context.RenderedBlock{
		Messages: []types.Message{types.SystemMessage(text)},
	}, nil
}

// renderToolOutput renders a tool output block.
func (r *OpenAIRenderer) renderToolOutput(block context.ContextBlock) (context.RenderedBlock, error) {
	text, err := extractText(block.Payload)
	if err != nil {
		return context.RenderedBlock{}, err
	}

	// Tool outputs as system messages with clear prefix
	return context.RenderedBlock{
		Messages: []types.Message{types.SystemMessage(fmt.Sprintf("Tool output: %s", text))},
	}, nil
}

// renderHistory renders a history block.
func (r *OpenAIRenderer) renderHistory(block context.ContextBlock) (context.RenderedBlock, error) {
	// Parse conversation history payload
	var historyPayload codecs.ConversationHistoryPayload

	switch v := block.Payload.(type) {
	case codecs.ConversationHistoryPayload:
		historyPayload = v
	case map[string]any:
		// Extract messages
		if messages, ok := v["messages"].([]any); ok {
			for _, msg := range messages {
				if msgMap, ok := msg.(map[string]any); ok {
					role, _ := msgMap["role"].(string)
					content, _ := msgMap["content"].(string)
					historyPayload.Messages = append(historyPayload.Messages, codecs.ConversationMessage{
						Role:    role,
						Content: content,
					})
				}
			}
		}
		if summary, ok := v["summary"].(string); ok {
			historyPayload.Summary = summary
		}
	default:
		// Try JSON unmarshal as fallback
		if data, err := json.Marshal(block.Payload); err == nil {
			_ = json.Unmarshal(data, &historyPayload)
		}
	}

	// Emit one message per history entry
	var messages []types.Message
	for _, msg := range historyPayload.Messages {
		role := types.RoleUser // Default
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

	// If summary exists, append a system message
	if historyPayload.Summary != "" {
		messages = append(messages, types.SystemMessage(fmt.Sprintf("Previous conversation summary: %s", historyPayload.Summary)))
	}

	return context.RenderedBlock{
		Messages: messages,
	}, nil
}

// renderTurn renders a turn block.
func (r *OpenAIRenderer) renderTurn(block context.ContextBlock) (context.RenderedBlock, error) {
	// Parse turn payload (supports both TurnPayload and string for backward compatibility)
	var turnPayload codecs.TurnPayload

	switch v := block.Payload.(type) {
	case codecs.TurnPayload:
		turnPayload = v
	case map[string]any:
		// Extract from map format
		if content, ok := v["content"].(string); ok {
			turnPayload.Content = content
		}
		if attachments, ok := v["attachments"].([]any); ok {
			turnPayload.Attachments = make([]codecs.TurnAttachment, 0, len(attachments))
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
		// Backward compatibility: simple string payload
		turnPayload.Content = v
	default:
		// Try extractText as fallback
		text, err := extractText(block.Payload)
		if err != nil {
			return context.RenderedBlock{}, err
		}
		turnPayload.Content = text
	}

	// Convert attachments to types.Attachment
	attachments := make([]types.Attachment, 0, len(turnPayload.Attachments))
	for _, att := range turnPayload.Attachments {
		// Parse reference to extract ID if it's in format "attachment:uuid"
		var attID uuid.UUID
		if len(att.Reference) > 11 && att.Reference[:11] == "attachment:" {
			if id, err := uuid.Parse(att.Reference[11:]); err == nil {
				attID = id
			}
		}

		attachments = append(attachments, types.Attachment{
			ID:        attID,
			FileName:  att.FileName,
			MimeType:  att.MimeType,
			SizeBytes: att.SizeBytes,
			FilePath:  att.Reference,
		})
	}

	// Emit canonical user message with attachments
	opts := []types.MessageOption{types.WithRole(types.RoleUser), types.WithContent(turnPayload.Content)}
	if len(attachments) > 0 {
		opts = append(opts, types.WithAttachments(attachments...))
	}
	msg := types.NewMessage(opts...)

	return context.RenderedBlock{
		Messages: []types.Message{msg},
	}, nil
}
