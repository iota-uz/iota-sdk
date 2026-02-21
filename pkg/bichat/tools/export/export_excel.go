package export

import (
	tools "github.com/iota-uz/iota-sdk/pkg/bichat/tools"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
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

// CallStructured executes the Excel export operation and returns a structured result.
func (t *ExportToExcelTool) CallStructured(ctx context.Context, input string) (*types.ToolResult, error) {
	const op serrors.Op = "ExportToExcelTool.Call"

	params, err := agents.ParseToolInput[excelExportInput](input)
	if err != nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: fmt.Sprintf("failed to parse input: %v", err),
				Hints:   []string{tools.HintCheckRequiredFields, "Provide data parameter with query results"},
			},
		}, nil
	}

	if params.Data == nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: "data parameter is required",
				Hints:   []string{tools.HintCheckRequiredFields, "Provide query result data to export"},
			},
		}, nil
	}

	if params.Data.RowCount > 100000 {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeDataTooLarge),
				Message: fmt.Sprintf("data too large for Excel export: %d rows (max: 100000)", params.Data.RowCount),
				Hints:   []string{tools.HintAddLimitClause, tools.HintFilterWithWhere, "Consider exporting filtered subsets instead"},
			},
		}, nil
	}

	filename := params.Filename
	if filename == "" {
		filename = "export.xlsx"
	}

	datasource := NewQueryResultDataSource(params.Data)
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

	url := buildDownloadURL(ctx, t.baseURL, filename)
	response := excelExportOutput{
		URL:      url,
		Filename: filename,
		RowCount: params.Data.RowCount,
	}

	return &types.ToolResult{
		CodecID: types.CodecJSON,
		Payload: types.JSONPayload{Output: response},
		Artifacts: []types.ToolArtifact{
			{
				Type:      "export",
				Name:      filename,
				MimeType:  "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
				URL:       url,
				SizeBytes: int64(len(bytes)),
				Metadata: map[string]any{
					"row_count": params.Data.RowCount,
				},
			},
		},
	}, nil
}

// Call executes the Excel export operation.
func (t *ExportToExcelTool) Call(ctx context.Context, input string) (string, error) {
	return tools.FormatStructuredResult(t.CallStructured(ctx, input))
}
