package codecs

import (
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/bichat/context"
)

// SystemRulesPayload represents a system prompt/rules block.
type SystemRulesPayload struct {
	Text string `json:"text"`
}

// SystemRulesCodec handles system rules blocks.
type SystemRulesCodec struct {
	*context.BaseCodec
}

// NewSystemRulesCodec creates a new system rules codec.
func NewSystemRulesCodec() *SystemRulesCodec {
	return &SystemRulesCodec{
		BaseCodec: context.NewBaseCodec("system-rules", "1.0.0"),
	}
}

// Validate validates the system rules payload.
func (c *SystemRulesCodec) Validate(payload any) error {
	switch v := payload.(type) {
	case SystemRulesPayload:
		if v.Text == "" {
			return fmt.Errorf("system rules text cannot be empty")
		}
		return nil
	case map[string]any:
		if text, ok := v["text"].(string); !ok || text == "" {
			return fmt.Errorf("system rules text cannot be empty")
		}
		return nil
	case string:
		if v == "" {
			return fmt.Errorf("system rules text cannot be empty")
		}
		return nil
	default:
		return fmt.Errorf("invalid system rules payload type: %T", payload)
	}
}

// Canonicalize converts the payload to canonical form.
func (c *SystemRulesCodec) Canonicalize(payload any) ([]byte, error) {
	// Extract text
	var text string
	switch v := payload.(type) {
	case SystemRulesPayload:
		text = v.Text
	case map[string]any:
		if t, ok := v["text"].(string); ok {
			text = t
		} else {
			return nil, fmt.Errorf("text field not found")
		}
	case string:
		text = v
	default:
		return nil, fmt.Errorf("invalid system rules payload type: %T", payload)
	}

	// Normalize whitespace
	canonical := SystemRulesPayload{
		Text: normalizeWhitespace(text),
	}

	return context.SortedJSONBytes(canonical)
}
