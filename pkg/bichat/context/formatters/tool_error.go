package formatters

import (
	"encoding/json"
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

// ToolErrorFormatter formats tool errors as JSON for LLM consumption.
type ToolErrorFormatter struct{}

// NewToolErrorFormatter creates a new tool error formatter.
func NewToolErrorFormatter() *ToolErrorFormatter {
	return &ToolErrorFormatter{}
}

// Format renders a ToolErrorPayload as JSON.
func (f *ToolErrorFormatter) Format(payload any, opts types.FormatOptions) (string, error) {
	switch p := payload.(type) {
	case types.ToolErrorPayload:
		return formatToolErrorJSON(p.Code, p.Message, p.Hints)
	case types.SQLDiagnosisPayload:
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
		wrapperErr := map[string]interface{}{
			"error": map[string]string{"code": code, "message": message},
		}
		fallback, fallbackErr := json.MarshalIndent(wrapperErr, "", "  ")
		if fallbackErr == nil {
			return string(fallback), err
		}
		return "", err
	}
	return string(data), nil
}

func formatSQLDiagnosisJSON(p types.SQLDiagnosisPayload) (string, error) {
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
		wrapperErr := map[string]interface{}{
			"error": map[string]string{"code": p.Code, "message": p.Message},
		}
		fallback, fallbackErr := json.MarshalIndent(wrapperErr, "", "  ")
		if fallbackErr == nil {
			return string(fallback), err
		}
		return "", err
	}
	return string(data), nil
}
