package tools

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// WebSearchTool provides web search capability for LLM agents.
// This tool is sent to OpenAI and will be called like any other tool.
// Currently returns an error - actual search implementation is pending.
//
// TODO: Implement web search provider integration (Tavily, Serper, Bing API, etc.)
// TODO: Future: Use OpenAI Responses API native web search when available
type WebSearchTool struct{}

// NewWebSearchTool creates a new web search tool.
// The tool is registered with OpenAI and will be called when the LLM needs web search.
// Actual search implementation is pending - currently returns "not implemented" error.
func NewWebSearchTool() agents.Tool {
	return &WebSearchTool{}
}

// Name returns the tool name.
func (t *WebSearchTool) Name() string {
	return "web_search"
}

// Description returns the tool description for the LLM.
func (t *WebSearchTool) Description() string {
	return "Search the web for current information, news, and real-time data. " +
		"Use this when you need up-to-date information beyond your training data, " +
		"such as current events, recent news, stock prices, weather, or any time-sensitive information."
}

// Parameters returns the JSON Schema for tool parameters.
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

// Call executes the web search operation.
// Currently not implemented - returns error informing LLM that search is unavailable.
//
// TODO: Implement actual web search using a provider like:
// - Tavily API (recommended for LLM use cases)
// - Serper API (Google search results)
// - Bing Search API
// - Custom search implementation
func (t *WebSearchTool) Call(ctx context.Context, input string) (string, error) {
	const op serrors.Op = "WebSearchTool.Call"

	// Return formatted error that LLM can understand
	// This allows the agent to gracefully handle unavailable web search
	return FormatToolError(
		ErrCodeServiceUnavailable,
		"Web search is not yet implemented",
		"The web_search tool is enabled but not connected to a search provider",
		"Please use alternative information sources or inform the user that web search is unavailable",
	), serrors.E(op, "web_search implementation pending - needs search provider integration")
}
