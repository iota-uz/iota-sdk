package formatters

import (
	"encoding/json"
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/bichat/context"
)

// ToolErrorFormatter formats tool errors as JSON for LLM consumption.
type ToolErrorFormatter struct{}

// NewToolErrorFormatter creates a new tool error formatter.
func NewToolErrorFormatter() *ToolErrorFormatter {
	return &ToolErrorFormatter{}
}

// Format renders a ToolErrorPayload as JSON.
func (f *ToolErrorFormatter) Format(payload any, opts context.FormatOptions) (string, error) {
	switch p := payload.(type) {
	case ToolErrorPayload:
		return formatToolErrorJSON(p.Code, p.Message, p.Hints)
	case SQLDiagnosisPayload:
		return formatSQLDiagnosisJSON(p)
	default:
		return "", fmt.Errorf("ToolErrorFormatter: expected ToolErrorPayload or SQLDiagnosisPayload, got %T", payload)
	}
}

func formatToolErrorJSON(code, message string, hints []string) (string, error) {
	wrapper := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
			"hints":   hints,
		},
	}

	data, err := json.MarshalIndent(wrapper, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": {"code": "%s", "message": "%s"}}`, code, message), nil
	}
	return string(data), nil
}

func formatSQLDiagnosisJSON(p SQLDiagnosisPayload) (string, error) {
	wrapper := map[string]interface{}{
		"error": map[string]interface{}{
			"code":       p.Code,
			"message":    p.Message,
			"table":      p.Table,
			"column":     p.Column,
			"suggestion": p.Suggestion,
			"hints":      p.Hints,
		},
	}

	data, err := json.MarshalIndent(wrapper, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": {"code": "%s", "message": "%s"}}`, p.Code, p.Message), nil
	}
	return string(data), nil
}
