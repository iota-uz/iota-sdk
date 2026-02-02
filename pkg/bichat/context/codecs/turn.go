package codecs

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/context"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

// TurnPayload represents a user turn with content and optional attachments.
type TurnPayload struct {
	Content     string           `json:"content"`               // The user's message content
	Attachments []TurnAttachment `json:"attachments,omitempty"` // Optional file attachments
}

// TurnAttachment represents a file attachment in a turn.
// This is a stable, LLM-oriented representation (no UUIDs/timestamps).
type TurnAttachment struct {
	FileName  string `json:"fileName"`  // Original filename
	MimeType  string `json:"mimeType"`  // MIME type (e.g., "image/png", "application/pdf")
	SizeBytes int64  `json:"sizeBytes"` // File size in bytes
	Reference string `json:"reference"` // URL or path reference (safe for LLM context)
}

// TurnCodec handles user turn blocks with content and attachments.
type TurnCodec struct {
	*context.BaseCodec
}

// NewTurnCodec creates a new turn codec.
func NewTurnCodec() *TurnCodec {
	return &TurnCodec{
		BaseCodec: context.NewBaseCodec("turn", "1.0.0"),
	}
}

// Validate validates the turn payload.
func (c *TurnCodec) Validate(payload any) error {
	switch v := payload.(type) {
	case TurnPayload:
		if v.Content == "" {
			return fmt.Errorf("turn content cannot be empty")
		}
		return nil
	case map[string]any:
		if content, ok := v["content"].(string); !ok || content == "" {
			return fmt.Errorf("turn content cannot be empty")
		}
		return nil
	case string:
		if v == "" {
			return fmt.Errorf("turn content cannot be empty")
		}
		return nil
	default:
		return fmt.Errorf("invalid turn payload type: %T", payload)
	}
}

// Canonicalize converts the payload to canonical form.
func (c *TurnCodec) Canonicalize(payload any) ([]byte, error) {
	var turn TurnPayload

	switch v := payload.(type) {
	case TurnPayload:
		turn = v
	case map[string]any:
		if content, ok := v["content"].(string); ok {
			turn.Content = normalizeWhitespace(content)
		} else {
			return nil, fmt.Errorf("content field not found")
		}
		// Extract attachments if present
		if attachments, ok := v["attachments"].([]any); ok {
			turn.Attachments = make([]TurnAttachment, 0, len(attachments))
			for _, att := range attachments {
				if attMap, ok := att.(map[string]any); ok {
					turn.Attachments = append(turn.Attachments, TurnAttachment{
						FileName:  getString(attMap, "fileName"),
						MimeType:  getString(attMap, "mimeType"),
						SizeBytes: getInt64(attMap, "sizeBytes"),
						Reference: getString(attMap, "reference"),
					})
				}
			}
		}
	case string:
		// Simple string payload (backward compatibility)
		turn.Content = normalizeWhitespace(v)
	default:
		return nil, fmt.Errorf("invalid turn payload type: %T", payload)
	}

	return context.SortedJSONBytes(turn)
}

// getString safely extracts a string from a map.
func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// getInt64 safely extracts an int64 from a map.
func getInt64(m map[string]any, key string) int64 {
	if v, ok := m[key].(float64); ok {
		return int64(v)
	}
	if v, ok := m[key].(int64); ok {
		return v
	}
	if v, ok := m[key].(int); ok {
		return int64(v)
	}
	return 0
}

// ConvertAttachmentsToTurnAttachments converts types.Attachment slice to TurnAttachment slice.
func ConvertAttachmentsToTurnAttachments(attachments []types.Attachment) []TurnAttachment {
	result := make([]TurnAttachment, 0, len(attachments))
	for _, att := range attachments {
		// Use FilePath as reference if available, otherwise construct from ID
		reference := att.FilePath
		if reference == "" && att.ID != (uuid.UUID{}) {
			reference = fmt.Sprintf("attachment:%s", att.ID.String())
		}

		result = append(result, TurnAttachment{
			FileName:  att.FileName,
			MimeType:  att.MimeType,
			SizeBytes: att.SizeBytes,
			Reference: reference,
		})
	}
	return result
}
