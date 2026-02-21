package export

import (
	"context"
	"fmt"
	tools "github.com/iota-uz/iota-sdk/pkg/bichat/tools"
	toolsql "github.com/iota-uz/iota-sdk/pkg/bichat/tools/sql"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/excel"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	renderTableDefaultPageSize = 25
	renderTableMaxPageSize     = 200
	renderTableDefaultMaxRows  = 2000
	renderTableHardMaxRows     = 50000
)

// RenderTableTool executes read-only SQL and returns a frontend-renderable table payload.
//
// It is intended for BI chat UIs that can render interactive/paginated tables and provide
// an inline Excel export action.
type RenderTableTool struct {
	executor   bichatsql.QueryExecutor
	outputDir  string
	baseURL    string
	exportOpts *excel.ExportOptions
	styleOpts  *excel.StyleOptions
}

// RenderTableToolOption configures RenderTableTool.
type RenderTableToolOption func(*RenderTableTool)

// WithRenderTableOutputDir sets where generated Excel files are stored.
func WithRenderTableOutputDir(dir string) RenderTableToolOption {
	return func(t *RenderTableTool) {
		t.outputDir = dir
	}
}

// WithRenderTableBaseURL sets the public base URL for generated Excel files.
func WithRenderTableBaseURL(url string) RenderTableToolOption {
	return func(t *RenderTableTool) {
		t.baseURL = url
	}
}

// WithRenderTableExportOptions sets custom Excel export options.
func WithRenderTableExportOptions(opts *excel.ExportOptions) RenderTableToolOption {
	return func(t *RenderTableTool) {
		t.exportOpts = opts
	}
}

// WithRenderTableStyleOptions sets custom Excel style options.
func WithRenderTableStyleOptions(opts *excel.StyleOptions) RenderTableToolOption {
	return func(t *RenderTableTool) {
		t.styleOpts = opts
	}
}

// NewRenderTableTool creates a new render_table tool.
func NewRenderTableTool(executor bichatsql.QueryExecutor, opts ...RenderTableToolOption) agents.Tool {
	tool := &RenderTableTool{
		executor:   executor,
		exportOpts: excel.DefaultOptions(),
		styleOpts:  excel.DefaultStyleOptions(),
	}

	for _, opt := range opts {
		opt(tool)
	}

	return tool
}

// Name returns the tool name.
func (t *RenderTableTool) Name() string {
	return "render_table"
}

// Description returns the tool description for the LLM.
func (t *RenderTableTool) Description() string {
	return "Run a read-only SQL query and render an interactive table card in chat. " +
		"Accepts custom column header names, returns rows for client-side pagination, and includes an Export to Excel action. " +
		"Users see a scrollable table with page controls plus an Export to Excel button. " +
		"Query must be SELECT/WITH-only and results are capped by max_rows (default 2000, max 50000)."
}

// Parameters returns the JSON Schema for tool parameters.
func (t *RenderTableTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"sql": map[string]any{
				"type":        "string",
				"description": "Read-only SQL query (SELECT or WITH...SELECT).",
			},
			"title": map[string]any{
				"type":        "string",
				"description": "Optional title displayed above the rendered table.",
			},
			"headers": map[string]any{
				"type":        "object",
				"description": "Optional map of source column name to display header (e.g. {\"policy_id\":\"Policy ID\"}).",
				"additionalProperties": map[string]any{
					"type": "string",
				},
			},
			"headerNames": map[string]any{
				"type":        "array",
				"description": "Optional ordered display headers aligned with selected columns.",
				"items": map[string]any{
					"type": "string",
				},
			},
			"page_size": map[string]any{
				"type":        "integer",
				"description": "Rows per page in the table UI (default 25, max 200).",
				"default":     renderTableDefaultPageSize,
				"minimum":     1,
				"maximum":     renderTableMaxPageSize,
			},
			"max_rows": map[string]any{
				"type":        "integer",
				"description": "Maximum rows fetched for rendering and export (default 2000, max 50000).",
				"default":     renderTableDefaultMaxRows,
				"minimum":     1,
				"maximum":     renderTableHardMaxRows,
			},
			"filename": map[string]any{
				"type":        "string",
				"description": "Optional Excel filename for the Export to Excel button.",
			},
		},
		"required": []string{"sql"},
	}
}

type renderTableInput struct {
	SQL         string            `json:"sql"`
	Title       string            `json:"title,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	HeaderNames []string          `json:"headerNames,omitempty"`
	PageSize    int               `json:"page_size,omitempty"`
	MaxRows     int               `json:"max_rows,omitempty"`
	Filename    string            `json:"filename,omitempty"`
}

type renderTableExport struct {
	URL        string `json:"url"`
	Filename   string `json:"filename"`
	RowCount   int    `json:"row_count"`
	FileSizeKB int64  `json:"file_size_kb"`
}

type renderTableOutput struct {
	Title           string             `json:"title,omitempty"`
	Query           string             `json:"query"`
	Columns         []string           `json:"columns"`
	Headers         []string           `json:"headers"`
	Rows            [][]any            `json:"rows"`
	TotalRows       int                `json:"total_rows"`
	PageSize        int                `json:"page_size"`
	Truncated       bool               `json:"truncated"`
	TruncatedReason string             `json:"truncated_reason,omitempty"`
	Export          *renderTableExport `json:"export,omitempty"`
	ExportPrompt    string             `json:"export_prompt,omitempty"`
}

// CallStructured executes SQL and returns a render-table JSON payload.
func (t *RenderTableTool) CallStructured(ctx context.Context, input string) (*types.ToolResult, error) {
	const op serrors.Op = "RenderTableTool.CallStructured"

	params, err := agents.ParseToolInput[renderTableInput](input)
	if err != nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: fmt.Sprintf("failed to parse input: %v", err),
				Hints:   []string{tools.HintCheckRequiredFields, tools.HintCheckFieldTypes},
			},
		}, nil
	}

	normalizedQuery := toolsql.NormalizeSQL(params.SQL)
	if normalizedQuery == "" {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: "sql parameter is required",
				Hints:   []string{tools.HintCheckRequiredFields, tools.HintCheckSQLSyntax},
			},
		}, nil
	}

	if err := toolsql.ValidateReadOnlyQuery(normalizedQuery); err != nil {
		return &types.ToolResult{ //nolint:nilerr // validation error is surfaced as a structured ToolResult; callers receive CodecToolError payload instead of a Go error
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodePolicyViolation),
				Message: err.Error(),
				Hints:   []string{tools.HintOnlySelectAllowed, tools.HintNoWriteOperations},
			},
		}, nil
	}

	pageSize := params.PageSize
	if pageSize == 0 {
		pageSize = renderTableDefaultPageSize
	}
	if pageSize < 1 || pageSize > renderTableMaxPageSize {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: fmt.Sprintf("page_size must be between 1 and %d", renderTableMaxPageSize),
				Hints:   []string{tools.HintCheckFieldTypes},
			},
		}, nil
	}

	maxRows := params.MaxRows
	if maxRows == 0 {
		maxRows = renderTableDefaultMaxRows
	}
	if maxRows < 1 || maxRows > renderTableHardMaxRows {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: fmt.Sprintf("max_rows must be between 1 and %d", renderTableHardMaxRows),
				Hints:   []string{tools.HintCheckFieldTypes, tools.HintAddLimitClause},
			},
		}, nil
	}

	fetchLimit := maxRows + 1
	executedSQL := toolsql.WrapQueryWithLimit(normalizedQuery, fetchLimit)

	result, err := t.executor.ExecuteQuery(ctx, executedSQL, nil, 45*time.Second)
	if err != nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeQueryError),
				Message: fmt.Sprintf("query execution failed: %v", err),
				Hints:   []string{tools.HintCheckSQLSyntax, tools.HintVerifyTableNames, tools.HintCheckJoinConditions},
			},
		}, serrors.E(op, err)
	}

	rows := result.Rows
	truncated := false
	truncatedReason := ""
	if len(rows) > maxRows {
		rows = rows[:maxRows]
		truncated = true
		truncatedReason = "max_rows"
	} else if result.Truncated {
		truncated = true
		truncatedReason = "executor_cap"
	}

	headers := resolveRenderTableHeaders(result.Columns, params.Headers, params.HeaderNames)

	output := renderTableOutput{
		Title:           strings.TrimSpace(params.Title),
		Query:           normalizedQuery,
		Columns:         append([]string(nil), result.Columns...),
		Headers:         headers,
		Rows:            rows,
		TotalRows:       len(rows),
		PageSize:        pageSize,
		Truncated:       truncated,
		TruncatedReason: truncatedReason,
		ExportPrompt:    fmt.Sprintf("Export this SQL query to Excel and share the file: %s", normalizedQuery),
	}

	if t.outputDir != "" && len(rows) > 0 {
		exportData := &bichatsql.QueryResult{
			Columns:  append([]string(nil), result.Columns...),
			Rows:     rows,
			RowCount: len(rows),
		}

		exporter := excel.NewExcelExporter(t.exportOpts, t.styleOpts)
		bytes, exportErr := exporter.Export(ctx, NewQueryResultDataSource(exportData))
		if exportErr != nil {
			return &types.ToolResult{
				CodecID: types.CodecToolError,
				Payload: types.ToolErrorPayload{
					Code:    string(tools.ErrCodeServiceUnavailable),
					Message: fmt.Sprintf("failed to generate Excel export: %v", exportErr),
					Hints:   []string{tools.HintRetryLater, tools.HintServiceMayBeDown},
				},
			}, serrors.E(op, exportErr)
		}

		filename := buildRenderTableFilename(params.Filename)
		if err := os.MkdirAll(t.outputDir, 0o755); err != nil {
			return &types.ToolResult{
				CodecID: types.CodecToolError,
				Payload: types.ToolErrorPayload{
					Code:    string(tools.ErrCodeServiceUnavailable),
					Message: fmt.Sprintf("failed to prepare export directory: %v", err),
					Hints:   []string{tools.HintRetryLater},
				},
			}, serrors.E(op, err)
		}

		filePath := filepath.Join(t.outputDir, filename)
		if err := os.WriteFile(filePath, bytes, 0o644); err != nil {
			return &types.ToolResult{
				CodecID: types.CodecToolError,
				Payload: types.ToolErrorPayload{
					Code:    string(tools.ErrCodeServiceUnavailable),
					Message: fmt.Sprintf("failed to save Excel export: %v", err),
					Hints:   []string{tools.HintRetryLater},
				},
			}, serrors.E(op, err)
		}

		output.Export = &renderTableExport{
			URL:        buildDownloadURL(ctx, t.baseURL, filename),
			Filename:   filename,
			RowCount:   len(rows),
			FileSizeKB: int64(len(bytes)) / 1024,
		}
	}

	return &types.ToolResult{
		CodecID: types.CodecJSON,
		Payload: types.JSONPayload{Output: output},
	}, nil
}

// Call executes render_table and returns JSON output.
func (t *RenderTableTool) Call(ctx context.Context, input string) (string, error) {
	return tools.FormatStructuredResult(t.CallStructured(ctx, input))
}

func resolveRenderTableHeaders(columns []string, headers map[string]string, headerNames []string) []string {
	out := make([]string, len(columns))
	lower := make(map[string]string, len(headers))
	for key, value := range headers {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		lower[strings.ToLower(strings.TrimSpace(key))] = trimmed
	}

	for i, column := range columns {
		label := ""
		if raw, ok := headers[column]; ok {
			label = strings.TrimSpace(raw)
		}
		if label == "" {
			label = lower[strings.ToLower(column)]
		}
		if label == "" && i < len(headerNames) {
			label = strings.TrimSpace(headerNames[i])
		}
		if label == "" {
			label = column
		}
		out[i] = label
	}

	return out
}

func buildRenderTableFilename(raw string) string {
	filename := strings.TrimSpace(raw)
	if filename == "" {
		filename = fmt.Sprintf("render_table_%s.xlsx", time.Now().UTC().Format("20060102_150405"))
	}
	filename = filepath.Base(filename)
	if filename == "." || filename == "/" || filename == "" {
		filename = fmt.Sprintf("render_table_%s.xlsx", time.Now().UTC().Format("20060102_150405"))
	}
	if !strings.HasSuffix(strings.ToLower(filename), ".xlsx") {
		filename += ".xlsx"
	}
	return filename
}
