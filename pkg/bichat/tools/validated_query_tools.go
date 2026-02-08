package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	bichatctx "github.com/iota-uz/iota-sdk/pkg/bichat/context"
	"github.com/iota-uz/iota-sdk/pkg/bichat/context/formatters"
	"github.com/iota-uz/iota-sdk/pkg/bichat/learning"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// SearchValidatedQueriesTool searches the library of validated SQL queries for patterns matching the question.
type SearchValidatedQueriesTool struct {
	store learning.ValidatedQueryStore
}

// NewSearchValidatedQueriesTool creates a new search validated queries tool.
func NewSearchValidatedQueriesTool(store learning.ValidatedQueryStore) agents.Tool {
	return &SearchValidatedQueriesTool{store: store}
}

// Name returns the tool name.
func (t *SearchValidatedQueriesTool) Name() string {
	return "search_validated_queries"
}

// Description returns the tool description for the LLM.
func (t *SearchValidatedQueriesTool) Description() string {
	return "Search the library of validated SQL queries for patterns matching your question. " +
		"Use this before writing new SQL to reuse proven patterns. " +
		"Results are ordered by relevance and usage frequency."
}

// Parameters returns the JSON Schema for tool parameters.
func (t *SearchValidatedQueriesTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"question": map[string]any{
				"type":        "string",
				"description": "Search terms describing what you want to query (e.g., 'sales by customer', 'monthly revenue trend')",
			},
			"tables": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "string"},
				"description": "Optional: Filter to queries using specific tables (e.g., ['sales', 'customers'])",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of results (default: 10, max: 50)",
				"default":     10,
			},
		},
		"required": []string{"question"},
	}
}

// searchValidatedQueriesInput represents the parsed input parameters.
type searchValidatedQueriesInput struct {
	Question string   `json:"question"`
	Tables   []string `json:"tables,omitempty"`
	Limit    int      `json:"limit,omitempty"`
}

// searchValidatedQueriesOutput represents the formatted output.
type searchValidatedQueriesOutput struct {
	Question    string                             `json:"question"`
	ResultCount int                                `json:"result_count"`
	Queries     []searchValidatedQueriesResultItem `json:"queries"`
}

type searchValidatedQueriesResultItem struct {
	ID               string   `json:"id"`
	Question         string   `json:"question"`
	SQL              string   `json:"sql"`
	Summary          string   `json:"summary"`
	TablesUsed       []string `json:"tables_used"`
	DataQualityNotes []string `json:"data_quality_notes,omitempty"`
	UsedCount        int      `json:"used_count"`
}

// CallStructured executes the search validated queries tool and returns a structured result.
func (t *SearchValidatedQueriesTool) CallStructured(ctx context.Context, input string) (*agents.ToolResult, error) {
	const op serrors.Op = "SearchValidatedQueriesTool.CallStructured"

	params, err := agents.ParseToolInput[searchValidatedQueriesInput](input)
	if err != nil {
		return &agents.ToolResult{
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
				Code:    string(ErrCodeInvalidRequest),
				Message: fmt.Sprintf("failed to parse input: %v", err),
				Hints:   []string{HintCheckRequiredFields},
			},
		}, nil
	}

	if params.Question == "" {
		return &agents.ToolResult{
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
				Code:    string(ErrCodeInvalidRequest),
				Message: "question parameter is required",
				Hints:   []string{HintCheckRequiredFields, "Provide search terms for validated query lookup"},
			},
		}, nil
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return &agents.ToolResult{
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
				Code:    string(ErrCodeServiceUnavailable),
				Message: "tenant context not available",
				Hints:   []string{HintServiceMayBeDown},
			},
		}, serrors.E(op, err)
	}

	limit := params.Limit
	if limit == 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	opts := learning.ValidatedQuerySearchOpts{
		TenantID: tenantID,
		Tables:   params.Tables,
		Limit:    limit,
	}

	queries, err := t.store.Search(ctx, params.Question, opts)
	if err != nil {
		return &agents.ToolResult{
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
				Code:    string(ErrCodeServiceUnavailable),
				Message: fmt.Sprintf("validated query search failed: %v", err),
				Hints:   []string{HintServiceMayBeDown, HintRetryLater},
			},
		}, serrors.E(op, err, "validated query search failed")
	}

	if len(queries) == 0 {
		return &agents.ToolResult{
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
				Code:    string(ErrCodeNoData),
				Message: fmt.Sprintf("no validated queries found for question: %s", params.Question),
				Hints: []string{
					HintTryDifferentTerms,
					"Try broader search terms or remove table filter",
					"This might be a new query pattern - write SQL carefully and save if successful",
				},
			},
		}, nil
	}

	results := make([]searchValidatedQueriesResultItem, len(queries))
	for i, q := range queries {
		results[i] = searchValidatedQueriesResultItem{
			ID:               q.ID.String(),
			Question:         q.Question,
			SQL:              q.SQL,
			Summary:          q.Summary,
			TablesUsed:       q.TablesUsed,
			DataQualityNotes: q.DataQualityNotes,
			UsedCount:        q.UsedCount,
		}
	}

	response := searchValidatedQueriesOutput{
		Question:    params.Question,
		ResultCount: len(results),
		Queries:     results,
	}

	return &agents.ToolResult{
		CodecID: formatters.CodecSearchResults,
		Payload: formatters.SearchResultsPayload{Output: response},
	}, nil
}

// Call executes the search validated queries tool.
func (t *SearchValidatedQueriesTool) Call(ctx context.Context, input string) (string, error) {
	result, err := t.CallStructured(ctx, input)
	if err != nil {
		if result != nil {
			registry := formatters.DefaultFormatterRegistry()
			if f := registry.Get(result.CodecID); f != nil {
				formatted, fmtErr := f.Format(result.Payload, bichatctx.DefaultFormatOptions())
				if fmtErr == nil {
					return formatted, err
				}
			}
		}
		return "", err
	}

	registry := formatters.DefaultFormatterRegistry()
	f := registry.Get(result.CodecID)
	if f == nil {
		return agents.FormatToolOutput(result.Payload)
	}
	return f.Format(result.Payload, bichatctx.DefaultFormatOptions())
}

// SaveValidatedQueryTool saves a successful SQL query to the library for future reuse.
type SaveValidatedQueryTool struct {
	store learning.ValidatedQueryStore
}

// NewSaveValidatedQueryTool creates a new save validated query tool.
func NewSaveValidatedQueryTool(store learning.ValidatedQueryStore) agents.Tool {
	return &SaveValidatedQueryTool{store: store}
}

// Name returns the tool name.
func (t *SaveValidatedQueryTool) Name() string {
	return "save_validated_query"
}

// Description returns the tool description for the LLM.
func (t *SaveValidatedQueryTool) Description() string {
	return "Save a successful SQL query to the library for future reuse. " +
		"Only save queries that produced correct results and answer a meaningful business question. " +
		"This helps you and future conversations reuse proven query patterns."
}

// Parameters returns the JSON Schema for tool parameters.
func (t *SaveValidatedQueryTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"question": map[string]any{
				"type":        "string",
				"description": "The business question this query answers (e.g., 'What are total sales by customer?')",
			},
			"sql": map[string]any{
				"type":        "string",
				"description": "The validated SQL query (SELECT or WITH...SELECT only)",
			},
			"summary": map[string]any{
				"type":        "string",
				"description": "Brief description of what the query does and what data it returns",
			},
			"tables_used": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "string"},
				"description": "List of tables referenced in the query (e.g., ['sales', 'customers', 'products'])",
			},
			"data_quality_notes": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "string"},
				"description": "Optional: Known data quality issues or caveats (e.g., ['NULL values in discount column', 'Historical data only'])",
			},
		},
		"required": []string{"question", "sql", "summary", "tables_used"},
	}
}

// saveValidatedQueryInput represents the parsed input parameters.
type saveValidatedQueryInput struct {
	Question         string   `json:"question"`
	SQL              string   `json:"sql"`
	Summary          string   `json:"summary"`
	TablesUsed       []string `json:"tables_used"`
	DataQualityNotes []string `json:"data_quality_notes,omitempty"`
}

// saveValidatedQueryOutput represents the formatted output.
type saveValidatedQueryOutput struct {
	ID       string   `json:"id"`
	Question string   `json:"question"`
	Message  string   `json:"message"`
	Tables   []string `json:"tables"`
}

// CallStructured executes the save validated query tool and returns a structured result.
func (t *SaveValidatedQueryTool) CallStructured(ctx context.Context, input string) (*agents.ToolResult, error) {
	const op serrors.Op = "SaveValidatedQueryTool.CallStructured"

	params, err := agents.ParseToolInput[saveValidatedQueryInput](input)
	if err != nil {
		return &agents.ToolResult{
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
				Code:    string(ErrCodeInvalidRequest),
				Message: fmt.Sprintf("failed to parse input: %v", err),
				Hints:   []string{HintCheckRequiredFields},
			},
		}, nil
	}

	if params.Question == "" {
		return &agents.ToolResult{
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
				Code:    string(ErrCodeInvalidRequest),
				Message: "question parameter is required",
				Hints:   []string{HintCheckRequiredFields, "Describe the business question this query answers"},
			},
		}, nil
	}
	if params.SQL == "" {
		return &agents.ToolResult{
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
				Code:    string(ErrCodeInvalidRequest),
				Message: "sql parameter is required",
				Hints:   []string{HintCheckRequiredFields, "Provide the validated SQL query"},
			},
		}, nil
	}
	if params.Summary == "" {
		return &agents.ToolResult{
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
				Code:    string(ErrCodeInvalidRequest),
				Message: "summary parameter is required",
				Hints:   []string{HintCheckRequiredFields, "Provide a brief description of what the query does"},
			},
		}, nil
	}
	if len(params.TablesUsed) == 0 {
		return &agents.ToolResult{
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
				Code:    string(ErrCodeInvalidRequest),
				Message: "tables_used parameter is required",
				Hints:   []string{HintCheckRequiredFields, "Provide list of tables referenced in the query"},
			},
		}, nil
	}

	normalizedSQL := strings.TrimSpace(strings.ToUpper(params.SQL))
	if !strings.HasPrefix(normalizedSQL, "SELECT") && !strings.HasPrefix(normalizedSQL, "WITH") {
		return &agents.ToolResult{
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
				Code:    string(ErrCodeInvalidRequest),
				Message: "only SELECT and WITH queries can be saved",
				Hints:   []string{HintCheckFieldTypes, "Ensure the SQL query starts with SELECT or WITH"},
			},
		}, nil
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return &agents.ToolResult{
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
				Code:    string(ErrCodeServiceUnavailable),
				Message: "tenant context not available",
				Hints:   []string{HintServiceMayBeDown},
			},
		}, serrors.E(op, err)
	}

	query := learning.ValidatedQuery{
		ID:               uuid.New(),
		TenantID:         tenantID,
		Question:         params.Question,
		SQL:              params.SQL,
		Summary:          params.Summary,
		TablesUsed:       params.TablesUsed,
		DataQualityNotes: params.DataQualityNotes,
		UsedCount:        0,
		CreatedAt:        time.Now(),
	}

	err = t.store.Save(ctx, query)
	if err != nil {
		return &agents.ToolResult{
			CodecID: formatters.CodecToolError,
			Payload: formatters.ToolErrorPayload{
				Code:    string(ErrCodeServiceUnavailable),
				Message: fmt.Sprintf("failed to save validated query: %v", err),
				Hints:   []string{HintServiceMayBeDown, HintRetryLater},
			},
		}, serrors.E(op, err, "failed to save validated query")
	}

	message := "Query saved successfully. This pattern will be available for future searches when similar questions arise."

	response := saveValidatedQueryOutput{
		ID:       query.ID.String(),
		Question: params.Question,
		Message:  message,
		Tables:   params.TablesUsed,
	}

	return &agents.ToolResult{
		CodecID: formatters.CodecJSON,
		Payload: formatters.GenericJSONPayload{Output: response},
	}, nil
}

// Call executes the save validated query tool.
func (t *SaveValidatedQueryTool) Call(ctx context.Context, input string) (string, error) {
	result, err := t.CallStructured(ctx, input)
	if err != nil {
		if result != nil {
			registry := formatters.DefaultFormatterRegistry()
			if f := registry.Get(result.CodecID); f != nil {
				formatted, fmtErr := f.Format(result.Payload, bichatctx.DefaultFormatOptions())
				if fmtErr == nil {
					return formatted, err
				}
			}
		}
		return "", err
	}

	registry := formatters.DefaultFormatterRegistry()
	f := registry.Get(result.CodecID)
	if f == nil {
		return agents.FormatToolOutput(result.Payload)
	}
	return f.Format(result.Payload, bichatctx.DefaultFormatOptions())
}
