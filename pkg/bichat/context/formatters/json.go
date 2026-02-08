package formatters

import (
	"encoding/json"
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

// JSONFormatter serializes any payload as JSON.
// Used as a fallback for payloads that don't need custom formatting.
type JSONFormatter struct{}

// NewJSONFormatter creates a new JSON formatter.
func NewJSONFormatter() *JSONFormatter {
	return &JSONFormatter{}
}

// Format renders the payload as JSON.
func (f *JSONFormatter) Format(payload any, opts types.FormatOptions) (string, error) {
	// Handle JSONPayload by unwrapping
	switch p := payload.(type) {
	case types.JSONPayload:
		return marshalJSON(p.Output)
	default:
		return marshalJSON(payload)
	}
}

func marshalJSON(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("JSONFormatter: %w", err)
	}
	return string(data), nil
}
