package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/excel"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// ExportToExcelTool exports already-fetched query results to Excel format.
// This operates on QueryResult data that was previously fetched by sql_execute.
// For query-driven export (execute SQL + export), use ExportQueryToExcelTool instead.
type ExportToExcelTool struct {
	outputDir  string
	baseURL    string
	exportOpts *excel.ExportOptions
	styleOpts  *excel.StyleOptions
}

// ExcelToolOption configures the Excel export tool.
type ExcelToolOption func(*ExportToExcelTool)

// WithOutputDir sets the directory where Excel files will be saved.
func WithOutputDir(dir string) ExcelToolOption {
	return func(t *ExportToExcelTool) {
		t.outputDir = dir
	}
}

// WithBaseURL sets the base URL for download links.
func WithBaseURL(url string) ExcelToolOption {
	return func(t *ExportToExcelTool) {
		t.baseURL = url
	}
}

// WithExportOptions sets custom export options.
func WithExportOptions(opts *excel.ExportOptions) ExcelToolOption {
	return func(t *ExportToExcelTool) {
		t.exportOpts = opts
	}
}

// WithStyleOptions sets custom style options.
func WithStyleOptions(opts *excel.StyleOptions) ExcelToolOption {
	return func(t *ExportToExcelTool) {
		t.styleOpts = opts
	}
}

// NewExportToExcelTool creates a new export to Excel tool.
func NewExportToExcelTool(opts ...ExcelToolOption) agents.Tool {
	tool := &ExportToExcelTool{
		exportOpts: excel.DefaultOptions(),
		styleOpts:  excel.DefaultStyleOptions(),
	}

	for _, opt := range opts {
		opt(tool)
	}

	return tool
}

// Name returns the tool name.
func (t *ExportToExcelTool) Name() string {
	return "export_data_to_excel"
}

// Description returns the tool description for the LLM.
func (t *ExportToExcelTool) Description() string {
	return "Export already-fetched query results to Excel format. " +
		"Use this when you have data from sql_execute that you want to convert to Excel. " +
		"For query-driven export (execute SQL fresh + export), use export_query_to_excel instead. " +
		"Maximum 100,000 rows. Returns download URL for the generated Excel file."
}

// Parameters returns the JSON Schema for tool parameters.
func (t *ExportToExcelTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"data": map[string]any{
				"type":        "object",
				"description": "Query result data to export (columns and rows)",
			},
			"filename": map[string]any{
				"type":        "string",
				"description": "Filename for the Excel file (default: 'export.xlsx')",
				"default":     "export.xlsx",
			},
		},
		"required": []string{"data"},
	}
}

// excelExportInput represents the parsed input parameters.
type excelExportInput struct {
	Data     *bichatsql.QueryResult `json:"data"`
	Filename string                 `json:"filename,omitempty"`
}

// excelExportOutput represents the formatted output.
type excelExportOutput struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
	RowCount int    `json:"row_count"`
}

// Call executes the Excel export operation.
func (t *ExportToExcelTool) Call(ctx context.Context, input string) (string, error) {
	const op serrors.Op = "ExportToExcelTool.Call"

	// Parse input
	params, err := agents.ParseToolInput[excelExportInput](input)
	if err != nil {
		return FormatToolError(
			ErrCodeInvalidRequest,
			fmt.Sprintf("failed to parse input: %v", err),
			HintCheckRequiredFields,
			"Provide data parameter with query results",
		), nil
	}

	if params.Data == nil {
		return FormatToolError(
			ErrCodeInvalidRequest,
			"data parameter is required",
			HintCheckRequiredFields,
			"Provide query result data to export",
		), nil
	}

	// Check if data is too large
	if params.Data.RowCount > 100000 {
		return FormatToolError(
			ErrCodeDataTooLarge,
			fmt.Sprintf("data too large for Excel export: %d rows (max: 100000)", params.Data.RowCount),
			HintAddLimitClause,
			HintFilterWithWhere,
			"Consider exporting filtered subsets instead",
		), nil
	}

	// Set defaults
	filename := params.Filename
	if filename == "" {
		filename = "export.xlsx"
	}

	// Create datasource adapter
	datasource := NewQueryResultDataSource(params.Data)

	// Use SDK exporter with configured options
	exporter := excel.NewExcelExporter(t.exportOpts, t.styleOpts)

	// Export to bytes
	bytes, err := exporter.Export(ctx, datasource)
	if err != nil {
		return FormatToolError(
			ErrCodeQueryError,
			fmt.Sprintf("failed to export Excel: %v", err),
			"Verify data format is valid",
			"Check for special characters in data",
		), serrors.E(op, err, "failed to export Excel")
	}

	// Save to file
	filePath := filepath.Join(t.outputDir, filename)
	if err := os.WriteFile(filePath, bytes, 0644); err != nil {
		return FormatToolError(
			ErrCodeServiceUnavailable,
			fmt.Sprintf("failed to save Excel file: %v", err),
			"File system may be full or permissions issue",
			HintRetryLater,
		), serrors.E(op, err, "failed to save Excel file")
	}

	// Return download URL
	url := buildDownloadURL(ctx, t.baseURL, filename)

	// Build response
	response := excelExportOutput{
		URL:      url,
		Filename: filename,
		RowCount: params.Data.RowCount,
	}

	return agents.FormatToolOutput(response)
}
