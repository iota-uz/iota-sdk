package tools

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
)

// WebSearchTool is a marker tool that tells the Model layer to enable
// native web search via the OpenAI Responses API.
// The API runs the search internally and returns citations in annotations.
// Call() should never be invoked — if it is, it returns an error.
type WebSearchTool struct{}

// NewWebSearchTool creates a new web search marker tool.
func NewWebSearchTool() agents.Tool {
	return &WebSearchTool{}
}

func (t *WebSearchTool) Name() string { return "web_search" }

func (t *WebSearchTool) Description() string {
	return "Search the web for current information, news, and real-time data. " +
		"Use this when you need up-to-date information beyond your training data."
}

func (t *WebSearchTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The search query to execute on the web",
			},
		},
		"required": []string{"query"},
	}
}

// Call returns an error because web search is handled natively by the API.
func (t *WebSearchTool) Call(_ context.Context, _ string) (string, error) {
	return FormatToolError(
		ErrCodeServiceUnavailable,
		"web_search is handled natively by the API",
		"This tool should not be called directly; it is executed by the model provider",
		"This is a marker tool — no action needed",
	), nil
}
