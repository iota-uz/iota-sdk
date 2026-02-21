package export

import (
	tools "github.com/iota-uz/iota-sdk/pkg/bichat/tools"
	toolsql "github.com/iota-uz/iota-sdk/pkg/bichat/tools/sql"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/excel"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// ExportQueryToExcelTool executes a SQL query and exports results directly to Excel.
// This is a query-driven approach that executes fresh SQL and generates Excel in one step.
type ExportQueryToExcelTool struct {
	executor   bichatsql.QueryExecutor
	outputDir  string
	baseURL    string
	exportOpts *excel.ExportOptions
	styleOpts  *excel.StyleOptions
}

// ExportQueryToolOption configures the export query tool.
type ExportQueryToolOption func(*ExportQueryToExcelTool)

// WithQueryOutputDir sets the directory where Excel files will be saved.
func WithQueryOutputDir(dir string) ExportQueryToolOption {
	return func(t *ExportQueryToExcelTool) {
		t.outputDir = dir
	}
}

// WithQueryBaseURL sets the base URL for download links.
func WithQueryBaseURL(url string) ExportQueryToolOption {
	return func(t *ExportQueryToExcelTool) {
		t.baseURL = url
	}
}

// WithQueryExportOptions sets custom export options.
func WithQueryExportOptions(opts *excel.ExportOptions) ExportQueryToolOption {
	return func(t *ExportQueryToExcelTool) {
		t.exportOpts = opts
	}
}

// WithQueryStyleOptions sets custom style options.
func WithQueryStyleOptions(opts *excel.StyleOptions) ExportQueryToolOption {
	return func(t *ExportQueryToExcelTool) {
		t.styleOpts = opts
	}
}

// NewExportQueryToExcelTool creates a new export query to Excel tool.
func NewExportQueryToExcelTool(executor bichatsql.QueryExecutor, opts ...ExportQueryToolOption) *ExportQueryToExcelTool {
	tool := &ExportQueryToExcelTool{
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
func (t *ExportQueryToExcelTool) Name() string {
	return "export_query_to_excel"
}

// Description returns the tool description for the LLM.
func (t *ExportQueryToExcelTool) Description() string {
	return "Execute a SQL query and export results directly to Excel format. " +
		"Use this when you need to export large datasets that haven't been fetched yet. " +
		"Applies automatic 50k row limit for performance. Returns download URL."
}

// Parameters returns the JSON Schema for tool parameters.
func (t *ExportQueryToExcelTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"sql": map[string]any{
				"type":        "string",
				"description": "SQL SELECT query to execute. Must be read-only. LIMIT will be auto-appended if missing (max 50000 rows).",
			},
			"filename": map[string]any{
				"type":        "string",
				"description": "Filename for the Excel file (default: 'export.xlsx')",
				"default":     "export.xlsx",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "Optional description of the export for user reference",
				"default":     "",
			},
		},
		"required": []string{"sql"},
	}
}

// exportQueryInput represents the parsed input parameters.
type exportQueryInput struct {
	SQL         string `json:"sql"`
	Filename    string `json:"filename,omitempty"`
	Description string `json:"description,omitempty"`
}

// exportQueryOutput represents the formatted output.
type exportQueryOutput struct {
	URL         string `json:"url"`
	Filename    string `json:"filename"`
	RowCount    int    `json:"row_count"`
	Description string `json:"description,omitempty"`
	FileSizeKB  int64  `json:"file_size_kb"`
}

// CallStructured executes the query and exports to Excel, returning a structured result.
func (t *ExportQueryToExcelTool) CallStructured(ctx context.Context, input string) (*types.ToolResult, error) {
	const op serrors.Op = "ExportQueryToExcelTool.CallStructured"

	params, err := agents.ParseToolInput[exportQueryInput](input)
	if err != nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: fmt.Sprintf("failed to parse input: %v", err),
				Hints:   []string{tools.HintCheckRequiredFields, tools.HintCheckFieldTypes},
			},
		}, agents.ErrStructuredToolOutput
	}

	if params.SQL == "" {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: "sql parameter is required",
				Hints:   []string{tools.HintCheckRequiredFields, "Provide a SELECT query to execute and export"},
			},
		}, nil
	}

	if err := toolsql.ValidateReadOnlyQuery(params.SQL); err != nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodePolicyViolation),
				Message: err.Error(),
				Hints:   []string{tools.HintOnlySelectAllowed, tools.HintNoWriteOperations, tools.HintUseSchemaList},
			},
		}, agents.ErrStructuredToolOutput
	}

	filename := params.Filename
	if filename == "" {
		filename = "export.xlsx"
	}

	if !strings.HasSuffix(filename, ".xlsx") {
		filename += ".xlsx"
	}

	querySql := applyRowLimit(params.SQL, 50000)

	result, err := t.executor.ExecuteQuery(ctx, querySql, nil, 60*time.Second)
	if err != nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeQueryError),
				Message: fmt.Sprintf("query execution failed: %v", err),
				Hints:   []string{tools.HintCheckSQLSyntax, tools.HintVerifyTableNames, tools.HintCheckJoinConditions},
			},
		}, serrors.E(op, err, "failed to execute query")
	}

	datasource := NewQueryResultDataSource(result)
	exporter := excel.NewExcelExporter(t.exportOpts, t.styleOpts)

	bytes, err := exporter.Export(ctx, datasource)
	if err != nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeQueryError),
				Message: fmt.Sprintf("failed to export Excel: %v", err),
				Hints:   []string{"Verify data format is valid", "Check for special characters in data"},
			},
		}, serrors.E(op, err, "failed to export Excel")
	}

	filePath := filepath.Join(t.outputDir, filename)
	if err := os.WriteFile(filePath, bytes, 0644); err != nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeServiceUnavailable),
				Message: fmt.Sprintf("failed to save Excel file: %v", err),
				Hints:   []string{"File system may be full or permissions issue", tools.HintRetryLater},
			},
		}, serrors.E(op, err, "failed to save Excel file")
	}

	fileSizeKB := int64(len(bytes)) / 1024
	url := buildDownloadURL(ctx, t.baseURL, filename)

	response := exportQueryOutput{
		URL:         url,
		Filename:    filename,
		RowCount:    result.RowCount,
		Description: params.Description,
		FileSizeKB:  fileSizeKB,
	}

	return &types.ToolResult{
		CodecID: types.CodecJSON,
		Payload: types.JSONPayload{Output: response},
		Artifacts: []types.ToolArtifact{
			{
				Type:        "export",
				Name:        filename,
				Description: params.Description,
				MimeType:    "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
				URL:         url,
				SizeBytes:   int64(len(bytes)),
				Metadata: map[string]any{
					"row_count":    result.RowCount,
					"file_size_kb": fileSizeKB,
				},
			},
		},
	}, nil
}

// Call executes the query and exports to Excel.
func (t *ExportQueryToExcelTool) Call(ctx context.Context, input string) (string, error) {
	return tools.FormatStructuredResult(t.CallStructured(ctx, input))
}

// applyRowLimit applies a LIMIT to the query. If the query already has a top-level
// LIMIT clause it is returned unchanged. For CTE queries (starting with "WITH"),
// it appends LIMIT directly to avoid breaking the CTE syntax. For other queries,
// it wraps the query as a subquery with an outer LIMIT.
func applyRowLimit(query string, maxRows int) string {
	q := strings.TrimSpace(query)
	q = strings.TrimRight(q, ";")
	if hasTopLevelLimit(q) {
		return q
	}
	if strings.HasPrefix(strings.ToUpper(q), "WITH ") {
		return fmt.Sprintf("%s LIMIT %d", q, maxRows)
	}
	return fmt.Sprintf("SELECT * FROM (%s) AS _bichat_export LIMIT %d", q, maxRows)
}

// trailingLimitRe matches a top-level LIMIT <number> (with optional OFFSET) at the
// end of a query, avoiding false positives from LIMIT inside subqueries.
var trailingLimitRe = regexp.MustCompile(`(?i)\bLIMIT\s+\d+(\s+OFFSET\s+\d+)?\s*$`)

// hasTopLevelLimit reports whether q already ends with a LIMIT clause.
func hasTopLevelLimit(q string) bool {
	return trailingLimitRe.MatchString(q)
}
