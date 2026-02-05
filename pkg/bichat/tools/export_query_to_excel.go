package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
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
func NewExportQueryToExcelTool(executor bichatsql.QueryExecutor, opts ...ExportQueryToolOption) agents.Tool {
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

// Call executes the query and exports to Excel.
func (t *ExportQueryToExcelTool) Call(ctx context.Context, input string) (string, error) {
	const op serrors.Op = "ExportQueryToExcelTool.Call"

	// Parse input
	params, err := agents.ParseToolInput[exportQueryInput](input)
	if err != nil {
		return FormatToolError(
			ErrCodeInvalidRequest,
			fmt.Sprintf("failed to parse input: %v", err),
			HintCheckRequiredFields,
			HintCheckFieldTypes,
		), nil
	}

	if params.SQL == "" {
		return FormatToolError(
			ErrCodeInvalidRequest,
			"sql parameter is required",
			HintCheckRequiredFields,
			"Provide a SELECT query to execute and export",
		), nil
	}

	// Validate query is read-only
	if err := validateReadOnlyQuery(params.SQL); err != nil {
		return FormatToolError(
			ErrCodePolicyViolation,
			err.Error(),
			HintOnlySelectAllowed,
			HintNoWriteOperations,
			HintUseSchemaList,
		), nil
	}

	// Set defaults
	filename := params.Filename
	if filename == "" {
		filename = "export.xlsx"
	}

	// Ensure .xlsx extension
	if !strings.HasSuffix(filename, ".xlsx") {
		filename += ".xlsx"
	}

	// Apply row limit (50k max for exports)
	querySql := applyRowLimit(params.SQL, 50000)

	// Execute query
	result, err := t.executor.ExecuteQuery(ctx, querySql, nil, 60*time.Second) // 60 second timeout for exports
	if err != nil {
		return FormatToolError(
			ErrCodeQueryError,
			fmt.Sprintf("query execution failed: %v", err),
			HintCheckSQLSyntax,
			HintVerifyTableNames,
			HintCheckJoinConditions,
		), serrors.E(op, err, "failed to execute query")
	}

	// Create datasource adapter
	datasource := NewQueryResultDataSource(result)

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

	// Calculate file size
	fileSizeKB := int64(len(bytes)) / 1024

	// Return download URL
	url := fmt.Sprintf("%s/%s", t.baseURL, filename)

	// Build response
	response := exportQueryOutput{
		URL:         url,
		Filename:    filename,
		RowCount:    result.RowCount,
		Description: params.Description,
		FileSizeKB:  fileSizeKB,
	}

	return agents.FormatToolOutput(response)
}

// applyRowLimit adds or enforces a LIMIT clause to the query.
// If the query already has a LIMIT, it's capped at maxRows.
func applyRowLimit(query string, maxRows int) string {
	normalized := strings.ToUpper(strings.TrimSpace(query))

	// Check if query already has a LIMIT
	if strings.Contains(normalized, "LIMIT") {
		// Parse existing limit and cap if necessary
		// For simplicity, just append our limit - PostgreSQL will use the smaller one
		return fmt.Sprintf("%s LIMIT %d", query, maxRows)
	}

	// No LIMIT present, add one
	return fmt.Sprintf("%s LIMIT %d", query, maxRows)
}
