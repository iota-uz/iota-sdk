package learning

import (
	"context"
	"fmt"
	tools "github.com/iota-uz/iota-sdk/pkg/bichat/tools"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/learning"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

// SearchLearningsTool searches past learnings about SQL errors, type mismatches, and corrections.
type SearchLearningsTool struct {
	store learning.LearningStore
}

// NewSearchLearningsTool creates a new search learnings tool.
func NewSearchLearningsTool(store learning.LearningStore) *SearchLearningsTool {
	return &SearchLearningsTool{store: store}
}

// Name returns the tool name.
func (t *SearchLearningsTool) Name() string {
	return "search_learnings"
}

// Description returns the tool description for the LLM.
func (t *SearchLearningsTool) Description() string {
	return "Search past learnings about SQL errors, type mismatches, and corrections for this database. " +
		"Use this before writing SQL to check for known patterns, gotchas, and fixes. " +
		"Learnings help you avoid repeating mistakes from previous conversations."
}

// Parameters returns the JSON Schema for tool parameters.
func (t *SearchLearningsTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Search terms (e.g., 'sales table', 'date column error', 'customer_id type')",
			},
			"table_name": map[string]any{
				"type":        "string",
				"description": "Optional: Filter learnings for a specific table",
			},
			"category": map[string]any{
				"type":        "string",
				"enum":        []string{"sql_error", "type_mismatch", "user_correction", "business_rule"},
				"description": "Optional: Filter by learning type",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of results (default: 10, max: 50)",
				"default":     10,
			},
		},
		"required": []string{"query"},
	}
}

// searchLearningsInput represents the parsed input parameters.
type searchLearningsInput struct {
	Query     string `json:"query"`
	TableName string `json:"table_name,omitempty"`
	Category  string `json:"category,omitempty"`
	Limit     int    `json:"limit,omitempty"`
}

// searchLearningsOutput represents the formatted output.
type searchLearningsOutput struct {
	Query       string                      `json:"query"`
	ResultCount int                         `json:"result_count"`
	Learnings   []searchLearningsResultItem `json:"learnings"`
}

type searchLearningsResultItem struct {
	ID        string `json:"id"`
	Category  string `json:"category"`
	Trigger   string `json:"trigger"`
	Lesson    string `json:"lesson"`
	TableName string `json:"table_name,omitempty"`
	SQLPatch  string `json:"sql_patch,omitempty"`
	UsedCount int    `json:"used_count"`
}

// CallStructured executes the search learnings tool and returns a structured result.
func (t *SearchLearningsTool) CallStructured(ctx context.Context, input string) (*types.ToolResult, error) {
	params, err := agents.ParseToolInput[searchLearningsInput](input)
	if err != nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: fmt.Sprintf("failed to parse input: %v", err),
				Hints:   []string{tools.HintCheckRequiredFields},
			},
		}, nil
	}

	if params.Query == "" {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: "query parameter is required",
				Hints:   []string{tools.HintCheckRequiredFields, "Provide search terms for learning query"},
			},
		}, nil
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		// Return ToolResult so executor formats the payload; err is conveyed in the payload.
		return &types.ToolResult{ //nolint:nilerr
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeServiceUnavailable),
				Message: "tenant context not available",
				Hints:   []string{tools.HintServiceMayBeDown},
			},
		}, nil
	}

	limit := params.Limit
	if limit == 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	opts := learning.SearchOpts{
		TenantID:  tenantID,
		TableName: params.TableName,
		Limit:     limit,
	}

	if params.Category != "" {
		cat := learning.Category(params.Category)
		opts.Category = &cat
	}

	learnings, err := t.store.Search(ctx, params.Query, opts)
	if err != nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeServiceUnavailable),
				Message: fmt.Sprintf("learning search failed: %v", err),
				Hints:   []string{tools.HintServiceMayBeDown, tools.HintRetryLater},
			},
		}, nil
	}

	if len(learnings) == 0 {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeNoData),
				Message: fmt.Sprintf("no learnings found for query: %s", params.Query),
				Hints: []string{
					tools.HintTryDifferentTerms,
					"Try broader search terms or remove table filter",
					"This might be a new pattern - proceed carefully and save learnings if you discover issues",
				},
			},
		}, nil
	}

	results := make([]searchLearningsResultItem, len(learnings))
	for i, l := range learnings {
		results[i] = searchLearningsResultItem{
			ID:        l.ID.String(),
			Category:  string(l.Category),
			Trigger:   l.Trigger,
			Lesson:    l.Lesson,
			TableName: l.TableName,
			SQLPatch:  l.SQLPatch,
			UsedCount: l.UsedCount,
		}
	}

	response := searchLearningsOutput{
		Query:       params.Query,
		ResultCount: len(results),
		Learnings:   results,
	}

	return &types.ToolResult{
		CodecID: types.CodecSearchResults,
		Payload: types.JSONPayload{Output: response},
	}, nil
}

// Call executes the search learnings tool.
func (t *SearchLearningsTool) Call(ctx context.Context, input string) (string, error) {
	return tools.FormatStructuredResult(t.CallStructured(ctx, input))
}

// SaveLearningTool saves a new learning when the agent discovers an error pattern or important insight.
type SaveLearningTool struct {
	store learning.LearningStore
}

// NewSaveLearningTool creates a new save learning tool.
func NewSaveLearningTool(store learning.LearningStore) *SaveLearningTool {
	return &SaveLearningTool{store: store}
}

// Name returns the tool name.
func (t *SaveLearningTool) Name() string {
	return "save_learning"
}

// Description returns the tool description for the LLM.
func (t *SaveLearningTool) Description() string {
	return "Save a new learning when you discover an error pattern, type mismatch, or important insight about the database. " +
		"This helps you and future conversations avoid repeating the same mistakes. " +
		"Use this after resolving SQL errors, discovering type casting issues, or receiving user corrections."
}

// Parameters returns the JSON Schema for tool parameters.
func (t *SaveLearningTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"category": map[string]any{
				"type":        "string",
				"enum":        []string{"sql_error", "type_mismatch", "user_correction", "business_rule"},
				"description": "Type of learning being saved",
			},
			"trigger": map[string]any{
				"type":        "string",
				"description": "What caused this learning (error message, user input, etc.)",
			},
			"lesson": map[string]any{
				"type":        "string",
				"description": "What to do/avoid next time (be specific and actionable)",
			},
			"table_name": map[string]any{
				"type":        "string",
				"description": "Optional: Related table for schema-specific learnings",
			},
			"sql_patch": map[string]any{
				"type":        "string",
				"description": "Optional: SQL fix or pattern to apply (e.g., 'CAST(column AS TEXT)')",
			},
		},
		"required": []string{"category", "trigger", "lesson"},
	}
}

// saveLearningInput represents the parsed input parameters.
type saveLearningInput struct {
	Category  string `json:"category"`
	Trigger   string `json:"trigger"`
	Lesson    string `json:"lesson"`
	TableName string `json:"table_name,omitempty"`
	SQLPatch  string `json:"sql_patch,omitempty"`
}

// saveLearningOutput represents the formatted output.
type saveLearningOutput struct {
	ID        string `json:"id"`
	Category  string `json:"category"`
	Message   string `json:"message"`
	TableName string `json:"table_name,omitempty"`
}

// CallStructured executes the save learning tool and returns a structured result.
func (t *SaveLearningTool) CallStructured(ctx context.Context, input string) (*types.ToolResult, error) {
	params, err := agents.ParseToolInput[saveLearningInput](input)
	if err != nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: fmt.Sprintf("failed to parse input: %v", err),
				Hints:   []string{tools.HintCheckRequiredFields},
			},
		}, nil
	}

	if params.Category == "" {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: "category parameter is required",
				Hints:   []string{tools.HintCheckRequiredFields, "Valid categories: sql_error, type_mismatch, user_correction, business_rule"},
			},
		}, nil
	}
	if params.Trigger == "" {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: "trigger parameter is required",
				Hints:   []string{tools.HintCheckRequiredFields, "Describe what caused this learning (error message, user feedback, etc.)"},
			},
		}, nil
	}
	if params.Lesson == "" {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: "lesson parameter is required",
				Hints:   []string{tools.HintCheckRequiredFields, "Describe what to do/avoid next time (be specific and actionable)"},
			},
		}, nil
	}

	validCategories := map[string]bool{
		"sql_error":       true,
		"type_mismatch":   true,
		"user_correction": true,
		"business_rule":   true,
	}
	if !validCategories[params.Category] {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: fmt.Sprintf("invalid category: %s", params.Category),
				Hints:   []string{tools.HintCheckFieldTypes, "Valid categories: sql_error, type_mismatch, user_correction, business_rule"},
			},
		}, nil
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		// Return ToolResult so executor formats the payload; err is conveyed in the payload.
		return &types.ToolResult{ //nolint:nilerr
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeServiceUnavailable),
				Message: "tenant context not available",
				Hints:   []string{tools.HintServiceMayBeDown},
			},
		}, nil
	}

	l := learning.Learning{
		ID:        uuid.New(),
		TenantID:  tenantID,
		Category:  learning.Category(params.Category),
		Trigger:   params.Trigger,
		Lesson:    params.Lesson,
		TableName: params.TableName,
		SQLPatch:  params.SQLPatch,
		UsedCount: 0,
		CreatedAt: time.Now(),
	}

	err = t.store.Save(ctx, l)
	if err != nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeServiceUnavailable),
				Message: fmt.Sprintf("failed to save learning: %v", err),
				Hints:   []string{tools.HintServiceMayBeDown, tools.HintRetryLater},
			},
		}, nil
	}

	message := fmt.Sprintf("Learning saved successfully. This %s pattern will help avoid similar issues in the future.", params.Category)
	if params.TableName != "" {
		message = fmt.Sprintf("Learning saved for table '%s'. This %s pattern will help avoid similar issues in the future.", params.TableName, params.Category)
	}

	response := saveLearningOutput{
		ID:        l.ID.String(),
		Category:  params.Category,
		Message:   message,
		TableName: params.TableName,
	}

	return &types.ToolResult{
		CodecID: types.CodecJSON,
		Payload: types.JSONPayload{Output: response},
	}, nil
}

// Call executes the save learning tool.
func (t *SaveLearningTool) Call(ctx context.Context, input string) (string, error) {
	return tools.FormatStructuredResult(t.CallStructured(ctx, input))
}
