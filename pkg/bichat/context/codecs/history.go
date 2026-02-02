package codecs

import (
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/bichat/context"
)

// ConversationMessage represents a single message in a conversation.
type ConversationMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ConversationHistoryPayload represents a conversation history block.
type ConversationHistoryPayload struct {
	Messages []ConversationMessage `json:"messages"`
	Summary  string                `json:"summary,omitempty"`
}

// ConversationHistoryCodec handles conversation history blocks.
type ConversationHistoryCodec struct {
	*context.BaseCodec
}

// NewConversationHistoryCodec creates a new conversation history codec.
func NewConversationHistoryCodec() *ConversationHistoryCodec {
	return &ConversationHistoryCodec{
		BaseCodec: context.NewBaseCodec("conversation-history", "1.0.0"),
	}
}

// Validate validates the conversation history payload.
func (c *ConversationHistoryCodec) Validate(payload any) error {
	switch v := payload.(type) {
	case ConversationHistoryPayload:
		if len(v.Messages) == 0 {
			return fmt.Errorf("conversation history must have at least one message")
		}
		for i, msg := range v.Messages {
			if msg.Role == "" {
				return fmt.Errorf("message %d missing role", i)
			}
			if msg.Content == "" {
				return fmt.Errorf("message %d missing content", i)
			}
		}
		return nil
	case map[string]any:
		messages, ok := v["messages"]
		if !ok {
			return fmt.Errorf("messages field not found")
		}
		if msgs, ok := messages.([]any); !ok || len(msgs) == 0 {
			return fmt.Errorf("messages must be a non-empty array")
		}
		return nil
	default:
		return fmt.Errorf("invalid conversation history payload type: %T", payload)
	}
}

// Canonicalize converts the payload to canonical form.
func (c *ConversationHistoryCodec) Canonicalize(payload any) ([]byte, error) {
	var history ConversationHistoryPayload

	switch v := payload.(type) {
	case ConversationHistoryPayload:
		history = v
	case map[string]any:
		// Extract messages
		if messages, ok := v["messages"].([]any); ok {
			for _, msg := range messages {
				if msgMap, ok := msg.(map[string]any); ok {
					role, _ := msgMap["role"].(string)
					content, _ := msgMap["content"].(string)
					history.Messages = append(history.Messages, ConversationMessage{
						Role:    role,
						Content: normalizeWhitespace(content),
					})
				}
			}
		}
		if summary, ok := v["summary"].(string); ok {
			history.Summary = normalizeWhitespace(summary)
		}
	default:
		return nil, fmt.Errorf("invalid conversation history payload type: %T", payload)
	}

	return context.SortedJSONBytes(history)
}
