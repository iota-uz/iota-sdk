package formatters

import (
	"encoding/json"
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/bichat/context"
)

// JSONFormatter serializes any payload as JSON.
// Used as a fallback for payloads that don't need custom formatting.
type JSONFormatter struct{}

// NewJSONFormatter creates a new JSON formatter.
func NewJSONFormatter() *JSONFormatter {
	return &JSONFormatter{}
}

// Format renders the payload as JSON.
func (f *JSONFormatter) Format(payload any, opts context.FormatOptions) (string, error) {
	// Handle SearchResultsPayload, TimePayload, GenericJSONPayload by unwrapping
	switch p := payload.(type) {
	case SearchResultsPayload:
		return marshalJSON(p.Output)
	case TimePayload:
		return marshalJSON(p.Output)
	case GenericJSONPayload:
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
