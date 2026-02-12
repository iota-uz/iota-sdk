package testharness

import (
	"encoding/json"
	"strings"

	"github.com/openai/openai-go"
)

func extractChatMessageContent(msg openai.ChatCompletionMessage) string {
	if content := normalizeTextField(msg.Content); content != "" {
		return content
	}
	return extractTextFieldFromRawMessage(msg.RawJSON(), "content")
}

func extractChatMessageRefusal(msg openai.ChatCompletionMessage) string {
	if refusal := normalizeTextField(msg.Refusal); refusal != "" {
		return refusal
	}
	return extractTextFieldFromRawMessage(msg.RawJSON(), "refusal")
}

func extractTextFieldFromRawMessage(rawJSON string, key string) string {
	rawJSON = strings.TrimSpace(rawJSON)
	if rawJSON == "" {
		return ""
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(rawJSON), &payload); err != nil {
		return ""
	}

	return strings.TrimSpace(extractTextValue(payload[key]))
}

func extractTextValue(v any) string {
	switch value := v.(type) {
	case string:
		return value
	case []any:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			text := strings.TrimSpace(extractTextValue(item))
			if text != "" {
				parts = append(parts, text)
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	case map[string]any:
		parts := make([]string, 0, 2)
		if text := strings.TrimSpace(extractTextValue(value["text"])); text != "" {
			parts = append(parts, text)
		}
		if inner := strings.TrimSpace(extractTextValue(value["content"])); inner != "" {
			parts = append(parts, inner)
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	default:
		return ""
	}
}

func normalizeTextField(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	if strings.HasPrefix(value, "{") || strings.HasPrefix(value, "[") {
		var parsed any
		if err := json.Unmarshal([]byte(value), &parsed); err == nil {
			if extracted := strings.TrimSpace(extractTextValue(parsed)); extracted != "" {
				return extracted
			}
		}
	}

	return value
}
