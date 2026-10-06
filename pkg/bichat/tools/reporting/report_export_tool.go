package reporting

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/bichat/tools"
	"github.com/iota-uz/iota-sdk/pkg/bichat/tools/export"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/excel"
	"github.com/jackc/pgx/v5/pgtype"
)

type ReportExportTool struct {
	executor     bichatsql.QueryExecutor
	dir, baseURL string
}

func NewReportExportTool(executor bichatsql.QueryExecutor, dir, baseURL string) *ReportExportTool {
	return &ReportExportTool{executor: NewReportExecutor(executor), dir: dir, baseURL: baseURL}
}

func (t *ReportExportTool) Name() string { return "export_query_to_excel" }
func (t *ReportExportTool) Description() string {
	return "Export the complete result of a read-only SQL query to typed Excel. No preview limit is appended. Fails without a file if the result exceeds 50000 rows or numeric precision is unsafe. This is exploratory SQL, not an approved financial report. Discover registered report definitions and use execute_report_plan when the requested semantics are registered."
}
func (t *ReportExportTool) Parameters() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{
		"sql": map[string]any{"type": "string"}, "filename": map[string]any{"type": "string"},
		"description": map[string]any{"type": "string"},
	}, "required": []string{"sql"}}
}
func (t *ReportExportTool) Call(ctx context.Context, input string) (string, error) {
	return tools.FormatStructuredResult(t.CallStructured(ctx, input))
}
func (t *ReportExportTool) CallStructured(ctx context.Context, input string) (*types.ToolResult, error) {
	params, err := agents.ParseToolInput[struct {
		SQL         string `json:"sql"`
		Filename    string `json:"filename"`
		Description string `json:"description"`
	}](input)
	if err != nil {
		return nil, err
	}
	// Fetch the original SQL exactly once. The SDK export helper receives a
	// frozen result, so its legacy appended LIMIT cannot truncate this dataset.
	r, err := t.executor.ExecuteQuery(ctx, params.SQL, nil, 60*time.Second)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSuffix(filepath.Base(params.Filename), ".xlsx")
	if name == "" || name == "." {
		name = "export"
	}
	params.Filename = name + "_" + uuid.NewString() + ".xlsx"
	excelResult, err := excelReportResult(r)
	if err != nil {
		return nil, err
	}
	frozen := &reportResultExecutor{result: excelResult}
	options := excel.DefaultOptions()
	options.DateTimeFormat = ""
	delegate := export.NewExportQueryToExcelTool(frozen, export.WithQueryOutputDir(t.dir), export.WithQueryBaseURL(t.baseURL), export.WithQueryExportOptions(options))
	request, err := agents.FormatToolOutput(params)
	if err != nil {
		return nil, err
	}
	output, err := delegate.CallStructured(ctx, request)
	if err != nil {
		return output, err
	}
	for i := range output.Artifacts {
		output.Artifacts[i].Metadata["complete"] = true
		output.Artifacts[i].Metadata["basis"] = "query_result"
	}
	return output, nil
}

// pgx TIME values survive SDK JSON formatting as pgtype.Time, which Excelize
// otherwise writes as text. Convert only the workbook copy to native duration.
func excelReportResult(r *bichatsql.QueryResult) (*bichatsql.QueryResult, error) {
	out := *r
	out.Rows = make([][]any, len(r.Rows))
	for i, row := range r.Rows {
		out.Rows[i] = append([]any(nil), row...)
		for j, value := range row {
			if r.ColumnTypes[j] != "date" {
				continue
			}
			switch cell := value.(type) {
			case pgtype.Time:
				if !cell.Valid {
					out.Rows[i][j] = nil
					continue
				}
				if cell.Microseconds < 0 || cell.Microseconds > int64(24*time.Hour/time.Microsecond) {
					return nil, ErrReportShape
				}
				out.Rows[i][j] = time.Duration(cell.Microseconds) * time.Microsecond
			case time.Time:
				// A parsed time-only string has no date; serialize it as a time
				// fraction rather than a year-zero date unsupported by Excel.
				if cell.Year() == 0 {
					out.Rows[i][j] = time.Duration(cell.Hour())*time.Hour + time.Duration(cell.Minute())*time.Minute + time.Duration(cell.Second())*time.Second + time.Duration(cell.Nanosecond())
				}
			}
		}
	}
	return &out, nil
}

type reportResultExecutor struct{ result *bichatsql.QueryResult }

func (e *reportResultExecutor) ExecuteQuery(context.Context, string, []any, time.Duration) (*bichatsql.QueryResult, error) {
	return e.result, nil
}
