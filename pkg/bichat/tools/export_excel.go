package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/excel"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// ExportToExcelTool exports query results to Excel format.
// It generates an Excel file with formatted data and returns the file path/URL.
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
	return "export_to_excel"
}

// Description returns the tool description for the LLM.
func (t *ExportToExcelTool) Description() string {
	return "Export query results to Excel format. " +
		"Returns a download URL for the generated Excel file."
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
	Data     *QueryResult `json:"data"`
	Filename string       `json:"filename,omitempty"`
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
		return "", serrors.E(op, err, "failed to parse input")
	}

	if params.Data == nil {
		return "", serrors.E(op, "data parameter is required")
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
		return "", serrors.E(op, err, "failed to export Excel")
	}

	// Save to file
	filePath := filepath.Join(t.outputDir, filename)
	if err := os.WriteFile(filePath, bytes, 0644); err != nil {
		return "", serrors.E(op, err, "failed to save Excel file")
	}

	// Return download URL
	url := fmt.Sprintf("%s/%s", t.baseURL, filename)

	// Build response
	response := excelExportOutput{
		URL:      url,
		Filename: filename,
		RowCount: params.Data.RowCount,
	}

	return agents.FormatToolOutput(response)
}
