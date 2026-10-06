package reporting

import (
	"context"
	"encoding/json"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/bichat/tools"
	"github.com/iota-uz/iota-sdk/pkg/bichat/tools/export"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/excel"
)

// ReportTableTool generates the complete workbook before limiting the model's
// preview. A preview budget must never become a download budget.
type ReportTableTool struct{ delegate agents.StructuredTool }

func NewReportTableTool(executor bichatsql.QueryExecutor, dir, baseURL string) *ReportTableTool {
	options := excel.DefaultOptions()
	options.DateTimeFormat = ""
	return &ReportTableTool{delegate: export.NewRenderTableTool(NewReportExecutor(executor), export.WithRenderTableOutputDir(dir), export.WithRenderTableBaseURL(baseURL), export.WithRenderTableExportOptions(options)).(agents.StructuredTool)}
}
func (t *ReportTableTool) Name() string { return t.delegate.Name() }
func (t *ReportTableTool) Description() string {
	return "Render an exploratory SQL result as a 200-row preview with a complete typed Excel download. No partial workbook is published. Use execute_report_plan for registered report semantics."
}
func (t *ReportTableTool) Parameters() map[string]any { return t.delegate.Parameters() }
func (t *ReportTableTool) Call(ctx context.Context, input string) (string, error) {
	return tools.FormatStructuredResult(t.CallStructured(ctx, input))
}
func (t *ReportTableTool) CallStructured(ctx context.Context, input string) (*types.ToolResult, error) {
	r, err := t.delegate.CallStructured(ctx, input)
	if err != nil {
		return r, err
	}
	if r.CodecID != types.CodecJSON {
		return r, nil
	}
	payload, ok := r.Payload.(types.JSONPayload)
	if !ok {
		return nil, ErrReportShape
	}
	data, err := json.Marshal(payload.Output)
	if err != nil {
		return nil, err
	}
	var output map[string]any
	if err = json.Unmarshal(data, &output); err != nil {
		return nil, err
	}
	rows, ok := output["rows"].([]any)
	if !ok {
		return nil, ErrReportShape
	}
	output["complete_export"] = true
	if len(rows) > 200 {
		output["rows"] = rows[:200]
		output["truncated"] = true
		output["truncated_reason"] = "preview_only; Excel contains the complete query result"
		for i := range r.Artifacts {
			r.Artifacts[i].Metadata["rows"] = rows[:200]
			r.Artifacts[i].Metadata["truncated"] = true
			r.Artifacts[i].Metadata["complete_export"] = true
		}
	}
	r.Payload = types.JSONPayload{Output: output}
	return r, nil
}
