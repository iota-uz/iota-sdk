package tools

import (
	"context"
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/xuri/excelize/v2"
)

// ExcelExporter defines the interface for exporting data to Excel.
// Consumers can implement this interface to customize Excel generation.
type ExcelExporter interface {
	// ExportToExcel exports query results to an Excel file and returns the file path or URL.
	ExportToExcel(ctx context.Context, data *QueryResult, filename string) (string, error)
}

// ExportToExcelTool exports query results to Excel format.
// It generates an Excel file with formatted data and returns the file path/URL.
type ExportToExcelTool struct {
	exporter ExcelExporter
}

// NewExportToExcelTool creates a new export to Excel tool.
func NewExportToExcelTool(exporter ExcelExporter) agents.Tool {
	return &ExportToExcelTool{
		exporter: exporter,
	}
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

	// Export to Excel
	url, err := t.exporter.ExportToExcel(ctx, params.Data, filename)
	if err != nil {
		return "", serrors.E(op, err, "Excel export failed")
	}

	// Build response
	response := excelExportOutput{
		URL:      url,
		Filename: filename,
		RowCount: params.Data.RowCount,
	}

	return agents.FormatToolOutput(response)
}

// DefaultExcelExporter is a default implementation using excelize.
// It creates Excel files with formatted headers and data.
type DefaultExcelExporter struct {
	outputDir string // Directory to save Excel files
	baseURL   string // Base URL for download links
}

// NewDefaultExcelExporter creates a new default Excel exporter.
// outputDir is the directory where Excel files will be saved.
// baseURL is the base URL for download links (e.g., "https://example.com/exports").
func NewDefaultExcelExporter(outputDir, baseURL string) ExcelExporter {
	return &DefaultExcelExporter{
		outputDir: outputDir,
		baseURL:   baseURL,
	}
}

// ExportToExcel exports query results to an Excel file.
func (e *DefaultExcelExporter) ExportToExcel(ctx context.Context, data *QueryResult, filename string) (string, error) {
	const op serrors.Op = "DefaultExcelExporter.ExportToExcel"

	// Create new Excel file
	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			// TODO: log Excel file close error
			_ = err
		}
	}()

	sheetName := "Sheet1"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return "", serrors.E(op, err, "failed to create sheet")
	}

	// Set headers
	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#E0E0E0"},
			Pattern: 1,
		},
	})
	if err != nil {
		return "", serrors.E(op, err, "failed to create header style")
	}

	// Write headers
	for i, col := range data.Columns {
		cell := fmt.Sprintf("%s1", string(rune('A'+i)))
		if err := f.SetCellValue(sheetName, cell, col); err != nil {
			return "", serrors.E(op, err, "failed to set header")
		}
		if err := f.SetCellStyle(sheetName, cell, cell, headerStyle); err != nil {
			return "", serrors.E(op, err, "failed to set header style")
		}
	}

	// Write data rows
	for rowIdx, row := range data.Rows {
		for colIdx, col := range data.Columns {
			cell := fmt.Sprintf("%s%d", string(rune('A'+colIdx)), rowIdx+2)
			value := row[col]
			if err := f.SetCellValue(sheetName, cell, value); err != nil {
				return "", serrors.E(op, err, "failed to set cell value")
			}
		}
	}

	// Auto-fit columns
	for i := range data.Columns {
		col := string(rune('A' + i))
		if err := f.SetColWidth(sheetName, col, col, 15); err != nil {
			return "", serrors.E(op, err, "failed to set column width")
		}
	}

	// Set active sheet
	f.SetActiveSheet(index)

	// Save file
	filePath := fmt.Sprintf("%s/%s", e.outputDir, filename)
	if err := f.SaveAs(filePath); err != nil {
		return "", serrors.E(op, err, "failed to save Excel file")
	}

	// Return download URL
	url := fmt.Sprintf("%s/%s", e.baseURL, filename)
	return url, nil
}
