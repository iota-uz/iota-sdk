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
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/excel"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
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
	return "Run a read-only SQL query and render an interactive, paginated table in chat. " +
		"Use this when users need an interactive table (not just a chart). " +
		"Accepts optional title and headers. Returns rows for frontend pagination with an Export to Excel action."
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
				"type":        "array",
				"description": "Optional ordered display headers aligned with selected columns.",
				"items": map[string]any{
					"type": "string",
				},
			},
		},
		"required": []string{"sql"},
	}
}

type renderTableInput struct {
	SQL     string   `json:"sql"`
	Title   string   `json:"title,omitempty"`
	Headers []string `json:"headers,omitempty"`
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
	ColumnTypes     []string           `json:"column_types,omitempty"`
	Headers         []string           `json:"headers"`
	Rows            [][]any            `json:"rows"`
	TotalRows       int                `json:"total_rows"`
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

	executedSQL := normalizedQuery

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
	if result.Truncated {
		truncated = true
		truncatedReason = "executor_cap"
	}

	headers := resolveRenderTableHeaders(result.Columns, params.Headers)

	output := renderTableOutput{
		Title:           strings.TrimSpace(params.Title),
		Query:           normalizedQuery,
		Columns:         append([]string(nil), result.Columns...),
		ColumnTypes:     append([]string(nil), result.ColumnTypes...),
		Headers:         headers,
		Rows:            rows,
		TotalRows:       len(rows),
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

		filename := buildRenderTableFilename()
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

	// Emit a table artifact with structural metadata for overlay preview; full row data is in the tool result payload.
	name := strings.TrimSpace(output.Title)
	if name == "" {
		name = "Table"
	}
	artifacts := []types.ToolArtifact{{
		Type:        string(domain.ArtifactTypeTable),
		Name:        name,
		Description: fmt.Sprintf("Table: %d rows", output.TotalRows),
		MimeType:    "application/json",
		Metadata: map[string]any{
			"query":        output.Query,
			"columns":      output.Columns,
			"headers":      output.Headers,
			"column_types": output.ColumnTypes,
			"total_rows":   output.TotalRows,
			"truncated":    output.Truncated,
			"title":        output.Title,
		},
	}}

	return &types.ToolResult{
		CodecID:   types.CodecJSON,
		Payload:   types.JSONPayload{Output: output},
		Artifacts: artifacts,
	}, nil
}

// Call executes render_table and returns JSON output.
func (t *RenderTableTool) Call(ctx context.Context, input string) (string, error) {
	return tools.FormatStructuredResult(t.CallStructured(ctx, input))
}

func resolveRenderTableHeaders(columns []string, headers []string) []string {
	out := make([]string, len(columns))
	for i, column := range columns {
		label := ""
		if i < len(headers) {
			label = strings.TrimSpace(headers[i])
		}
		if label == "" {
			label = column
		}
		out[i] = label
	}

	return out
}

func buildRenderTableFilename() string {
	return fmt.Sprintf("render_table_%s.xlsx", time.Now().UTC().Format("20060102_150405.000000000"))
}
