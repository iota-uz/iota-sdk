package tools

import (
	"context"
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// KBSearcher defines the interface for searching the knowledge base.
// Consumers should implement this interface to provide knowledge base access.
type KBSearcher interface {
	// Search searches the knowledge base and returns relevant documents.
	// The limit parameter specifies the maximum number of results to return.
	Search(ctx context.Context, query string, limit int) ([]SearchResult, error)

	// IsAvailable checks if the knowledge base is available.
	IsAvailable() bool
}

// SearchResult represents a search result from the knowledge base.
type SearchResult struct {
	ID        string                 `json:"id"`
	Title     string                 `json:"title"`
	Content   string                 `json:"content"`
	Score     float64                `json:"score"`
	Excerpt   string                 `json:"excerpt,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp string                 `json:"timestamp,omitempty"`
}

// KBSearchTool searches the knowledge base for relevant documents.
// It's useful for retrieving documentation, FAQs, and how-to guides.
type KBSearchTool struct {
	searcher KBSearcher
}

// NewKBSearchTool creates a new knowledge base search tool.
func NewKBSearchTool(searcher KBSearcher) agents.Tool {
	return &KBSearchTool{
		searcher: searcher,
	}
}

// Name returns the tool name.
func (t *KBSearchTool) Name() string {
	return "kb_search"
}

// Description returns the tool description for the LLM.
func (t *KBSearchTool) Description() string {
	return "Search the knowledge base for documentation, FAQs, and how-to guides. " +
		"Use this when the user asks about business rules, procedures, or domain knowledge."
}

// Parameters returns the JSON Schema for tool parameters.
func (t *KBSearchTool) Parameters() map[string]any {
	return agents.ToolSchema[kbSearchInput]()
}

// kbSearchInput represents the parsed input parameters.
type kbSearchInput struct {
	Query string `json:"query" jsonschema:"description=The search query"`
	Limit int    `json:"limit,omitempty" jsonschema:"description=Maximum number of results to return (default: 5, max: 20);default=5;minimum=1;maximum=20"`
}

// kbSearchOutput represents the formatted output.
type kbSearchOutput struct {
	Query       string         `json:"query"`
	ResultCount int            `json:"result_count"`
	Results     []SearchResult `json:"results"`
}

// Call executes the knowledge base search.
func (t *KBSearchTool) Call(ctx context.Context, input string) (string, error) {
	const op serrors.Op = "KBSearchTool.Call"

	// Parse input
	params, err := agents.ParseToolInput[kbSearchInput](input)
	if err != nil {
		return FormatToolError(
			ErrCodeInvalidRequest,
			fmt.Sprintf("failed to parse input: %v", err),
			HintCheckRequiredFields,
		), nil
	}

	if params.Query == "" {
		return FormatToolError(
			ErrCodeInvalidRequest,
			"query parameter is required",
			HintCheckRequiredFields,
			"Provide search terms for knowledge base query",
		), nil
	}

	// Set defaults
	limit := params.Limit
	if limit == 0 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}

	// Check if KB is available
	if !t.searcher.IsAvailable() {
		return FormatToolError(
			ErrCodeServiceUnavailable,
			"knowledge base is not available",
			HintServiceMayBeDown,
			HintRetryLater,
			"Contact administrator to enable knowledge base",
		), serrors.E(op, "knowledge base is not available")
	}

	// Search knowledge base
	results, err := t.searcher.Search(ctx, params.Query, limit)
	if err != nil {
		return FormatToolError(
			ErrCodeServiceUnavailable,
			fmt.Sprintf("knowledge base search failed: %v", err),
			HintServiceMayBeDown,
			HintRetryLater,
		), serrors.E(op, err, "knowledge base search failed")
	}

	// Check if no results found
	if len(results) == 0 {
		return FormatToolError(
			ErrCodeNoData,
			fmt.Sprintf("no knowledge base results found for query: %s", params.Query),
			HintTryDifferentTerms,
			"Try broader or more specific search terms",
			"Check spelling and terminology",
		), nil
	}

	// Build response
	response := kbSearchOutput{
		Query:       params.Query,
		ResultCount: len(results),
		Results:     results,
	}

	return agents.FormatToolOutput(response)
}
