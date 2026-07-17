// Package export turns an executed Lens dashboard into a typed, multi-sheet
// Excel workbook. It consumes runtime results, never re-queries a datasource.
package export

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/xuri/excelize/v2"
)

type Labels struct{ Summary, ChartData, Breakdown, Parameters, Sources, Metric, Value, Parameter, Dataset, Source, DependsOn string }

func DefaultLabels() Labels {
	return Labels{"Summary", "Chart data", "Breakdown", "Parameters", "Sources", "Metric", "Value", "Parameter", "Dataset", "Source", "Depends on"}
}

type Request struct {
	Result    *lensruntime.DashboardResult
	PanelID   string
	DrillPath []string
	Labels    Labels
}

type Exporter struct{}

func New() *Exporter { return &Exporter{} }

func (e *Exporter) Write(ctx context.Context, w io.Writer, req Request) error {
	if req.Result == nil {
		return fmt.Errorf("lens export result is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	labels := req.Labels
	if labels.Summary == "" {
		labels = DefaultLabels()
	}
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()
	defaultSheet := file.GetSheetName(0)
	used := map[string]int{}
	newSheet := func(name string) (string, error) {
		name = uniqueSheetName(safeSheetName(name), used)
		_, err := file.NewSheet(name)
		return name, err
	}
	summary, err := newSheet(labels.Summary)
	if err != nil {
		return err
	}
	if err := writeRows(file, summary, [][]any{{labels.Metric, labels.Value}, {req.Result.Spec.Title, dashboardTotal(req.Result, req.PanelID)}, {"Snapshot", req.Result.SnapshotID}, {"Drill", strings.Join(req.DrillPath, " > ")}}); err != nil {
		return err
	}
	parameters, err := newSheet(labels.Parameters)
	if err != nil {
		return err
	}
	parameterRows := [][]any{{labels.Parameter, labels.Value}}
	keys := make([]string, 0, len(req.Result.Variables))
	for key := range req.Result.Variables {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		parameterRows = append(parameterRows, []any{key, scalar(req.Result.Variables[key])})
	}
	if err := writeRows(file, parameters, parameterRows); err != nil {
		return err
	}

	panels := selectedPanels(req.Result, req.PanelID)
	exportedDatasets := map[string]bool{}
	if req.PanelID == "" {
		for _, dataset := range req.Result.Spec.Export.EvidenceDatasets {
			exportedDatasets[dataset] = true
		}
	}
	for _, panelResult := range panels {
		if panelResult == nil || panelResult.Frames == nil {
			continue
		}
		sheet, sheetErr := newSheet(panelResult.Panel.Title)
		if sheetErr != nil {
			return sheetErr
		}
		if err := writeFrameSet(file, sheet, panelResult.Frames); err != nil {
			return err
		}
		exportedDatasets[panelResult.Panel.Dataset] = true
		evidence := panelResult.Panel.Export.EvidenceDataset
		if evidence == "" {
			evidence = datasetExport(req.Result.Spec, panelResult.Panel.Dataset).EvidenceDataset
		}
		if evidence != "" {
			exportedDatasets[evidence] = true
		}
		if panelResult.Panel.Export.IncludeUpstream || datasetExport(req.Result.Spec, panelResult.Panel.Dataset).IncludeUpstream {
			collectUpstream(req.Result.Spec, panelResult.Panel.Dataset, exportedDatasets)
		}
	}
	for dataset := range exportedDatasets {
		if containsPanelDataset(panels, dataset) {
			continue
		}
		result := req.Result.Datasets[dataset]
		if result == nil || result.Frames == nil {
			continue
		}
		sheet, sheetErr := newSheet(labels.Breakdown + " " + dataset)
		if sheetErr != nil {
			return sheetErr
		}
		if err := writeFrameSet(file, sheet, result.Frames); err != nil {
			return err
		}
	}
	sources, err := newSheet(labels.Sources)
	if err != nil {
		return err
	}
	sourceRows := [][]any{{labels.Dataset, labels.Source, labels.DependsOn}}
	for _, dataset := range req.Result.Spec.Datasets {
		if !exportedDatasets[dataset.Name] {
			continue
		}
		sourceRows = append(sourceRows, []any{dataset.Title, dataset.Source, strings.Join(dataset.DependsOn, ", ")})
	}
	if err := writeRows(file, sources, sourceRows); err != nil {
		return err
	}
	if err := file.DeleteSheet(defaultSheet); err != nil {
		return err
	}
	index, _ := file.GetSheetIndex(summary)
	file.SetActiveSheet(index)
	return file.Write(w)
}

func selectedPanels(result *lensruntime.DashboardResult, id string) []*lensruntime.PanelResult {
	if id != "" {
		if panel := result.Panel(id); panel != nil {
			return []*lensruntime.PanelResult{panel}
		}
		return nil
	}
	ids := make([]string, 0, len(result.Panels))
	for id := range result.Panels {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]*lensruntime.PanelResult, 0, len(ids))
	for _, id := range ids {
		if !result.Panels[id].Panel.Kind.IsContainer() {
			out = append(out, result.Panels[id])
		}
	}
	return out
}
func datasetExport(spec lens.DashboardSpec, name string) (out struct {
	EvidenceDataset string
	IncludeUpstream bool
}) {
	for _, d := range spec.Datasets {
		if d.Name == name {
			out.EvidenceDataset = d.Export.EvidenceDataset
			out.IncludeUpstream = d.Export.IncludeUpstream
		}
	}
	return
}
func collectUpstream(spec lens.DashboardSpec, name string, set map[string]bool) {
	for _, d := range spec.Datasets {
		if d.Name != name {
			continue
		}
		for _, dep := range d.DependsOn {
			if !set[dep] {
				set[dep] = true
				collectUpstream(spec, dep, set)
			}
		}
	}
}
func containsPanelDataset(panels []*lensruntime.PanelResult, name string) bool {
	for _, p := range panels {
		if p.Panel.Dataset == name {
			return true
		}
	}
	return false
}
func dashboardTotal(result *lensruntime.DashboardResult, panelID string) float64 {
	total := 0.0
	for _, p := range selectedPanels(result, panelID) {
		if p.Frames == nil {
			continue
		}
		f := p.Frames.Primary()
		if f == nil {
			continue
		}
		field, ok := f.Field(p.Panel.Fields.Value.Name())
		if !ok {
			continue
		}
		for _, v := range field.Values {
			switch n := v.(type) {
			case float64:
				total += n
			case float32:
				total += float64(n)
			case int:
				total += float64(n)
			case int64:
				total += float64(n)
			}
		}
	}
	return total
}

func writeFrameSet(file *excelize.File, sheet string, frames *frame.FrameSet) error {
	row := 1
	for _, fr := range frames.Frames {
		if row > 1 {
			row += 2
		}
		if fr.Meta.Title != "" {
			_ = file.SetCellValue(sheet, cell(1, row), fr.Meta.Title)
			row++
		}
		headers := make([]any, len(fr.Fields))
		for i, f := range fr.Fields {
			headers[i] = fieldLabel(f)
		}
		if err := writeRow(file, sheet, row, headers); err != nil {
			return err
		}
		row++
		for i := 0; i < fr.RowCount; i++ {
			values := make([]any, len(fr.Fields))
			for col, f := range fr.Fields {
				values[col] = excelValue(f.Values[i])
			}
			if err := writeRow(file, sheet, row, values); err != nil {
				return err
			}
			row++
		}
	}
	return nil
}
func writeRows(file *excelize.File, sheet string, rows [][]any) error {
	for i, row := range rows {
		if err := writeRow(file, sheet, i+1, row); err != nil {
			return err
		}
	}
	return nil
}
func writeRow(file *excelize.File, sheet string, row int, values []any) error {
	for col, value := range values {
		if err := file.SetCellValue(sheet, cell(col+1, row), excelValue(value)); err != nil {
			return err
		}
	}
	return nil
}
func cell(col, row int) string { name, _ := excelize.CoordinatesToCellName(col, row); return name }
func fieldLabel(f frame.Field) string {
	if label := f.Labels["default"]; label != "" {
		return label
	}
	return f.Name
}
func excelValue(value any) any {
	switch v := value.(type) {
	case time.Time:
		return v
	case *time.Time:
		if v == nil {
			return nil
		}
		return *v
	default:
		return scalar(v)
	}
}
func scalar(value any) any {
	switch value.(type) {
	case nil, string, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, time.Time:
		return value
	default:
		return fmt.Sprint(value)
	}
}

var invalidSheetChars = regexp.MustCompile(`[\\/:?*\[\]]`)

func safeSheetName(name string) string {
	name = strings.TrimSpace(invalidSheetChars.ReplaceAllString(name, " "))
	if name == "" {
		name = "Data"
	}
	r := []rune(name)
	if len(r) > 31 {
		name = string(r[:31])
	}
	return name
}
func uniqueSheetName(name string, used map[string]int) string {
	base := name
	for {
		if _, ok := used[name]; !ok {
			used[name] = 1
			return name
		}
		used[base]++
		suffix := fmt.Sprintf(" %d", used[base])
		r := []rune(base)
		if len(r)+len([]rune(suffix)) > 31 {
			r = r[:31-len([]rune(suffix))]
		}
		name = string(r) + suffix
	}
}
