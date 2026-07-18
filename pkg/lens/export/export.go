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
	"github.com/iota-uz/iota-sdk/pkg/lens/explore"
	"github.com/iota-uz/iota-sdk/pkg/lens/exportmeta"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/xuri/excelize/v2"
)

type Labels struct {
	Summary, ChartData, Breakdown, Parameters, Sources, Metric, Value, Parameter, Dataset, Source, DependsOn string
	Explorer, Branch, Perspective, Node, Path, ExportMode                                                    string
}

func DefaultLabels() Labels {
	return Labels{"Summary", "Chart data", "Breakdown", "Parameters", "Sources", "Metric", "Value", "Parameter", "Dataset", "Source", "Depends on", "Explorer", "Branch", "Perspective", "Node", "Path", "Export mode"}
}

func LabelsForLocale(locale string) Labels {
	switch strings.ToLower(strings.TrimSpace(locale)) {
	case "ru", "ru-ru":
		return Labels{"Сводка", "Данные графика", "Разбивка", "Параметры", "Источники", "Показатель", "Значение", "Параметр", "Набор данных", "Источник", "Зависит от", "Исследование", "Раздел", "Представление", "Узел", "Путь", "Режим экспорта"}
	case "uz-cyrl", "uz-cyrl-uz", "oz":
		return Labels{"Хулоса", "График маълумотлари", "Тафсилот", "Параметрлар", "Манбалар", "Кўрсаткич", "Қиймат", "Параметр", "Маълумотлар тўплами", "Манба", "Боғлиқ", "Таҳлил", "Бўлим", "Кўриниш", "Тугун", "Йўл", "Экспорт режими"}
	case "uz", "uz-latn", "uz-latn-uz":
		return Labels{"Xulosa", "Grafik ma’lumotlari", "Tafsilot", "Parametrlar", "Manbalar", "Ko‘rsatkich", "Qiymat", "Parametr", "Ma’lumotlar to‘plami", "Manba", "Bog‘liq", "Tahlil", "Bo‘lim", "Ko‘rinish", "Tugun", "Yo‘l", "Eksport rejimi"}
	default:
		return DefaultLabels()
	}
}

type Request struct {
	Result      *lensruntime.DashboardResult
	PanelID     string
	DrillPath   []string
	Labels      Labels
	Exploration *explore.ExportRequest
}

type sheetTarget struct {
	Sheet string
	Cell  string
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
		labels = LabelsForLocale(req.Result.Locale)
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
	parameters, err := newSheet(labels.Parameters)
	if err != nil {
		return err
	}
	sources, err := newSheet(labels.Sources)
	if err != nil {
		return err
	}
	summaryRows := [][]any{
		{labels.Metric, labels.Value},
		{req.Result.Spec.Title, dashboardTotal(req.Result, req.PanelID)},
		{"Snapshot", req.Result.SnapshotID},
		{"Drill", strings.Join(req.DrillPath, " > ")},
		{labels.Sources, sheetHyperlink(sources, labels.Sources)},
	}
	if req.Exploration != nil {
		if err := req.Exploration.Validate(); err != nil {
			return err
		}
		summaryRows = append(summaryRows, explorationSummaryRows(labels, *req.Exploration)...)
	}
	if err := writeRows(file, summary, summaryRows); err != nil {
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
	datasetTargets := map[string]sheetTarget{}
	exportOrder := make([]string, 0)
	addDataset := func(dataset string) bool {
		dataset = strings.TrimSpace(dataset)
		if dataset == "" || exportedDatasets[dataset] {
			return false
		}
		exportedDatasets[dataset] = true
		exportOrder = append(exportOrder, dataset)
		return true
	}
	if req.PanelID == "" {
		for _, dataset := range req.Result.Spec.Export.EvidenceDatasets {
			addDataset(dataset)
		}
	}
	for _, panelResult := range panels {
		if panelResult == nil || panelResult.Frames == nil {
			continue
		}
		addDataset(panelResult.Panel.Dataset)
		evidence := panelResult.Panel.Export.EvidenceDatasets
		if len(evidence) == 0 {
			evidence = datasetExport(req.Result.Spec, panelResult.Panel.Dataset).EvidenceDatasets
		}
		for _, dataset := range evidence {
			addDataset(dataset)
		}
		if panelResult.Panel.Export.IncludeUpstream || datasetExport(req.Result.Spec, panelResult.Panel.Dataset).IncludeUpstream {
			collectUpstream(req.Result.Spec, panelResult.Panel.Dataset, addDataset)
		}
	}
	for _, dataset := range exportOrder {
		if containsPanelDataset(panels, dataset) {
			continue
		}
		result := req.Result.Datasets[dataset]
		if result == nil || result.Frames == nil {
			return fmt.Errorf("declared export evidence dataset %q is unavailable", dataset)
		}
		datasetSpec := findDataset(req.Result.Spec, dataset)
		sheetName := labels.Breakdown + " " + dataset
		if datasetSpec.Export.SheetName != "" {
			sheetName = datasetSpec.Export.SheetName
		} else if datasetSpec.Title != "" {
			sheetName = datasetSpec.Title
		}
		sheet, sheetErr := newSheet(sheetName)
		if sheetErr != nil {
			return sheetErr
		}
		if err := writeFrameSet(file, sheet, result.Frames); err != nil {
			return err
		}
		if err := configureEvidenceSheet(file, sheet, result.Frames, datasetSpec.Export); err != nil {
			return err
		}
		datasetTargets[dataset] = sheetTarget{Sheet: sheet, Cell: "A1"}
	}
	dashboardRow := len(summaryRows) + 2
	dashboardColumns := 2
	for _, panelResult := range panels {
		if panelResult == nil || panelResult.Frames == nil {
			continue
		}
		panelCell := cell(1, dashboardRow)
		if err := file.SetCellValue(summary, panelCell, panelResult.Panel.Title); err != nil {
			return err
		}
		datasetTargets[panelResult.Panel.Dataset] = sheetTarget{Sheet: summary, Cell: panelCell}
		dashboardRow++
		nextRow, columns, writeErr := writeFrameSetAt(file, summary, panelResult.Frames, dashboardRow)
		if writeErr != nil {
			return writeErr
		}
		if columns > dashboardColumns {
			dashboardColumns = columns
		}
		dashboardRow = nextRow + 2
	}
	if err := setSheetDimension(file, summary, dashboardRow-3, dashboardColumns); err != nil {
		return err
	}
	sourceRows := [][]any{{labels.Dataset, labels.Source, labels.DependsOn}}
	for _, datasetName := range exportOrder {
		dataset := findDataset(req.Result.Spec, datasetName)
		datasetTitle := strings.TrimSpace(dataset.Title)
		if datasetTitle == "" {
			datasetTitle = datasetName
		}
		title := any(datasetTitle)
		if target, ok := datasetTargets[datasetName]; ok {
			title = sheetCellHyperlink(target.Sheet, target.Cell, datasetTitle)
		}
		sourceRows = append(sourceRows, []any{title, dataset.Source, strings.Join(dataset.DependsOn, ", ")})
	}
	if err := writeRows(file, sources, sourceRows); err != nil {
		return err
	}
	if err := file.DeleteSheet(defaultSheet); err != nil {
		return err
	}
	if err := file.SetCalcProps(&excelize.CalcPropsOptions{
		CalcMode:       pointer("auto"),
		FullCalcOnLoad: pointer(true),
		ForceFullCalc:  pointer(true),
	}); err != nil {
		return err
	}
	index, _ := file.GetSheetIndex(summary)
	file.SetActiveSheet(index)
	return file.Write(w)
}

func explorationSummaryRows(labels Labels, req explore.ExportRequest) [][]any {
	value := func(label, fallback string) string {
		if strings.TrimSpace(label) != "" {
			return label
		}
		return fallback
	}
	rows := [][]any{
		{labels.ExportMode, string(req.Mode)},
		{labels.Explorer, value(req.Labels.Explorer, req.ExplorerID)},
		{labels.Branch, value(req.Labels.Branch, req.BranchKey)},
	}
	if req.PerspectiveKey != "" {
		rows = append(rows, []any{labels.Perspective, value(req.Labels.Perspective, req.PerspectiveKey)})
	}
	if req.NodeKey != "" {
		rows = append(rows, []any{labels.Node, value(req.Labels.Node, req.NodeKey)})
	}
	if len(req.Path) > 0 {
		rows = append(rows, []any{labels.Path, strings.Join(req.Path, " > ")})
	}
	return rows
}

func selectedPanels(result *lensruntime.DashboardResult, id string) []*lensruntime.PanelResult {
	if id != "" {
		if panel := result.Panel(id); panel != nil {
			return []*lensruntime.PanelResult{panel}
		}
		return nil
	}
	out := make([]*lensruntime.PanelResult, 0, len(result.Panels))
	seen := make(map[string]bool, len(result.Panels))
	var appendPanel func(panel.Spec)
	appendPanel = func(spec panel.Spec) {
		if panelResult := result.Panels[spec.ID]; panelResult != nil && !panelResult.Panel.Kind.IsContainer() && !seen[spec.ID] {
			out = append(out, panelResult)
			seen[spec.ID] = true
		}
		for _, child := range spec.Children {
			appendPanel(child)
		}
	}
	for _, row := range result.Spec.Rows {
		for _, spec := range row.Panels {
			appendPanel(spec)
		}
	}
	remaining := make([]string, 0, len(result.Panels)-len(seen))
	for id, panelResult := range result.Panels {
		if !seen[id] && !panelResult.Panel.Kind.IsContainer() {
			remaining = append(remaining, id)
		}
	}
	sort.Strings(remaining)
	for _, id := range remaining {
		out = append(out, result.Panels[id])
	}
	return out
}
func datasetExport(spec lens.DashboardSpec, name string) (out struct {
	EvidenceDatasets []string
	IncludeUpstream  bool
}) {
	for _, d := range spec.Datasets {
		if d.Name == name {
			out.EvidenceDatasets = append([]string(nil), d.Export.EvidenceDatasets...)
			out.IncludeUpstream = d.Export.IncludeUpstream
		}
	}
	return
}
func collectUpstream(spec lens.DashboardSpec, name string, add func(string) bool) {
	for _, d := range spec.Datasets {
		if d.Name != name {
			continue
		}
		for _, dep := range d.DependsOn {
			if add(dep) {
				collectUpstream(spec, dep, add)
			}
		}
	}
}

func findDataset(spec lens.DashboardSpec, name string) lens.DatasetSpec {
	for _, dataset := range spec.Datasets {
		if dataset.Name == name {
			return dataset
		}
	}
	return lens.DatasetSpec{}
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
	nextRow, maxColumns, err := writeFrameSetAt(file, sheet, frames, 1)
	if err != nil {
		return err
	}
	return setSheetDimension(file, sheet, nextRow-1, maxColumns)
}
func writeFrameSetAt(file *excelize.File, sheet string, frames *frame.FrameSet, startRow int) (int, int, error) {
	row := startRow
	maxColumns := 1
	for index, fr := range frames.Frames {
		if index > 0 {
			row += 2
		}
		if fr.Meta.Title != "" {
			_ = file.SetCellValue(sheet, cell(1, row), fr.Meta.Title)
			row++
		}
		headers := make([]any, len(fr.Fields))
		if len(fr.Fields) > maxColumns {
			maxColumns = len(fr.Fields)
		}
		for i, f := range fr.Fields {
			headers[i] = fieldLabel(f)
		}
		if err := writeRow(file, sheet, row, headers); err != nil {
			return row, maxColumns, err
		}
		row++
		for i := 0; i < fr.RowCount; i++ {
			values := make([]any, len(fr.Fields))
			for col, f := range fr.Fields {
				values[col] = f.Values[i]
			}
			if err := writeRow(file, sheet, row, values); err != nil {
				return row, maxColumns, err
			}
			row++
		}
	}
	return row, maxColumns, nil
}
func writeRows(file *excelize.File, sheet string, rows [][]any) error {
	maxColumns := 1
	for i, row := range rows {
		if len(row) > maxColumns {
			maxColumns = len(row)
		}
		if err := writeRow(file, sheet, i+1, row); err != nil {
			return err
		}
	}
	return setSheetDimension(file, sheet, len(rows), maxColumns)
}
func writeRow(file *excelize.File, sheet string, row int, values []any) error {
	for col, value := range values {
		if err := writeCell(file, sheet, cell(col+1, row), value); err != nil {
			return err
		}
	}
	return nil
}

func writeCell(file *excelize.File, sheet, coordinate string, value any) error {
	switch item := value.(type) {
	case frame.Formula:
		formula := strings.TrimPrefix(strings.TrimSpace(item.Expression), "=")
		if formula == "" {
			return file.SetCellValue(sheet, coordinate, excelValue(item.Result))
		}
		return file.SetCellFormula(sheet, coordinate, formula)
	case *frame.Formula:
		if item == nil {
			return nil
		}
		return writeCell(file, sheet, coordinate, *item)
	case frame.Hyperlink:
		return file.SetCellFormula(sheet, coordinate, fmt.Sprintf(`HYPERLINK("%s","%s")`, excelFormulaString(item.URL), excelFormulaString(item.Label)))
	case *frame.Hyperlink:
		if item == nil {
			return nil
		}
		return writeCell(file, sheet, coordinate, *item)
	default:
		return file.SetCellValue(sheet, coordinate, excelValue(value))
	}
}

func configureEvidenceSheet(file *excelize.File, sheet string, frames *frame.FrameSet, spec exportmeta.Spec) error {
	if frames == nil || frames.Primary() == nil {
		return nil
	}
	primary := frames.Primary()
	if spec.FreezeHeader {
		if err := file.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
			return err
		}
	}
	if tableName := strings.TrimSpace(spec.TableName); tableName != "" && len(primary.Fields) > 0 {
		endColumn, err := excelize.ColumnNumberToName(len(primary.Fields))
		if err != nil {
			return err
		}
		endRow := primary.RowCount + 1
		if endRow < 2 {
			endRow = 2
		}
		if err := file.AddTable(sheet, &excelize.Table{
			Range:          fmt.Sprintf("A1:%s%d", endColumn, endRow),
			Name:           tableName,
			StyleName:      "TableStyleMedium2",
			ShowRowStripes: pointer(true),
		}); err != nil {
			return err
		}
	}
	return nil
}

func pointer[T any](value T) *T              { return &value }
func excelFormulaString(value string) string { return strings.ReplaceAll(value, `"`, `""`) }
func cell(col, row int) string               { name, _ := excelize.CoordinatesToCellName(col, row); return name }
func sheetHyperlink(sheet, label string) frame.Hyperlink {
	return sheetCellHyperlink(sheet, "A1", label)
}
func sheetCellHyperlink(sheet, coordinate, label string) frame.Hyperlink {
	return frame.Hyperlink{URL: "#'" + strings.ReplaceAll(sheet, "'", "''") + "'!" + coordinate, Label: label}
}
func setSheetDimension(file *excelize.File, sheet string, rows, columns int) error {
	if rows < 1 {
		rows = 1
	}
	if columns < 1 {
		columns = 1
	}
	return file.SetSheetDimension(sheet, "A1:"+cell(columns, rows))
}
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
