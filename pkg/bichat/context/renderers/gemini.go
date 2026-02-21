package renderers

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/context"
	"github.com/iota-uz/iota-sdk/pkg/bichat/context/codecs"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
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
func (r *GeminiRenderer) renderSystem(block context.ContextBlock) (context.RenderedBlock, error) {
	text, err := extractText(block.Payload)
	if err != nil {
		return context.RenderedBlock{}, err
	}
	return context.RenderedBlock{
		Messages: []types.Message{types.SystemMessage(text)},
	}, nil
}

// renderToolOutput renders a tool output block.
func (r *GeminiRenderer) renderToolOutput(block context.ContextBlock) (context.RenderedBlock, error) {
	text, err := extractText(block.Payload)
	if err != nil {
		return context.RenderedBlock{}, err
	}
	return context.RenderedBlock{
		Messages: []types.Message{types.SystemMessage(fmt.Sprintf("Tool output: %s", text))},
	}, nil
}

// renderHistory renders a history block.
func (r *GeminiRenderer) renderHistory(block context.ContextBlock) (context.RenderedBlock, error) {
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
func (r *GeminiRenderer) renderTurn(block context.ContextBlock) (context.RenderedBlock, error) {
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
					var uploadID *int64
					if v, ok := attMap["uploadId"].(float64); ok {
						parsed := int64(v)
						if parsed > 0 {
							uploadID = &parsed
						}
					} else if v, ok := attMap["uploadId"].(int64); ok {
						if v > 0 {
							uploadID = &v
						}
					} else if v, ok := attMap["uploadId"].(int); ok {
						parsed := int64(v)
						if parsed > 0 {
							uploadID = &parsed
						}
					}
					turnPayload.Attachments = append(turnPayload.Attachments, codecs.TurnAttachment{
						FileName:  fileName,
						MimeType:  mimeType,
						SizeBytes: sizeBytes,
						Reference: reference,
						UploadID:  uploadID,
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
			UploadID: att.UploadID,
			ID:       attID, FileName: att.FileName, MimeType: att.MimeType,
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
