// Package export turns an executed Lens dashboard into a typed, multi-sheet
// Excel workbook. It consumes runtime results, never re-queries a datasource.
package export

import (
	"context"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	"github.com/iota-uz/iota-sdk/pkg/lens/explore"
	"github.com/iota-uz/iota-sdk/pkg/lens/exportmeta"
	"github.com/iota-uz/iota-sdk/pkg/lens/format"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/xuri/excelize/v2"
)

type Labels struct {
	Summary, ChartData, Breakdown, Parameters, Sources, Metric, Value, Parameter, Dataset, Source, DependsOn string
	Explorer, Branch, Perspective, Node, Path, ExportMode                                                    string
	Snapshot, Drill                                                                                          string
	AllTime, Yes, No                                                                                         string
}

func DefaultLabels() Labels {
	return Labels{Summary: "Summary", ChartData: "Chart data", Breakdown: "Breakdown", Parameters: "Parameters", Sources: "Sources", Metric: "Metric", Value: "Value", Parameter: "Parameter", Dataset: "Dataset", Source: "Source", DependsOn: "Depends on", Explorer: "Explorer", Branch: "Branch", Perspective: "Perspective", Node: "Node", Path: "Path", ExportMode: "Export mode", Snapshot: "Snapshot", Drill: "Drill", AllTime: "All time", Yes: "Yes", No: "No"}
}

func LabelsForLocale(locale string) Labels {
	switch strings.ToLower(strings.TrimSpace(locale)) {
	case "ru", "ru-ru":
		labels := DefaultLabels()
		labels.Summary, labels.ChartData, labels.Breakdown, labels.Parameters, labels.Sources = "Сводка", "Данные графика", "Разбивка", "Параметры", "Источники"
		labels.Metric, labels.Value, labels.Parameter, labels.Dataset, labels.Source, labels.DependsOn = "Показатель", "Значение", "Параметр", "Набор данных", "Источник", "Зависит от"
		labels.Explorer, labels.Branch, labels.Perspective, labels.Node, labels.Path, labels.ExportMode, labels.Snapshot, labels.Drill = "Исследование", "Раздел", "Представление", "Узел", "Путь", "Режим экспорта", "Снимок", "Детализация"
		labels.AllTime, labels.Yes, labels.No = "За всё время", "Да", "Нет"
		return labels
	case "uz-cyrl", "uz-cyrl-uz", "oz":
		labels := DefaultLabels()
		labels.Summary, labels.ChartData, labels.Breakdown, labels.Parameters, labels.Sources = "Хулоса", "График маълумотлари", "Тафсилот", "Параметрлар", "Манбалар"
		labels.Metric, labels.Value, labels.Parameter, labels.Dataset, labels.Source, labels.DependsOn = "Кўрсаткич", "Қиймат", "Параметр", "Маълумотлар тўплами", "Манба", "Боғлиқ"
		labels.Explorer, labels.Branch, labels.Perspective, labels.Node, labels.Path, labels.ExportMode, labels.Snapshot, labels.Drill = "Таҳлил", "Бўлим", "Кўриниш", "Тугун", "Йўл", "Экспорт режими", "Сурат", "Тафсилот"
		labels.AllTime, labels.Yes, labels.No = "Бутун давр", "Ҳа", "Йўқ"
		return labels
	case "uz", "uz-latn", "uz-latn-uz":
		labels := DefaultLabels()
		labels.Summary, labels.ChartData, labels.Breakdown, labels.Parameters, labels.Sources = "Xulosa", "Grafik ma’lumotlari", "Tafsilot", "Parametrlar", "Manbalar"
		labels.Metric, labels.Value, labels.Parameter, labels.Dataset, labels.Source, labels.DependsOn = "Ko‘rsatkich", "Qiymat", "Parametr", "Ma’lumotlar to‘plami", "Manba", "Bog‘liq"
		labels.Explorer, labels.Branch, labels.Perspective, labels.Node, labels.Path, labels.ExportMode, labels.Snapshot, labels.Drill = "Tahlil", "Bo‘lim", "Ko‘rinish", "Tugun", "Yo‘l", "Eksport rejimi", "Surat", "Tafsilot"
		labels.AllTime, labels.Yes, labels.No = "Butun davr", "Ha", "Yo‘q"
		return labels
	case "pt", "pt-br", "pt_br":
		labels := DefaultLabels()
		labels.Summary, labels.ChartData, labels.Breakdown, labels.Parameters, labels.Sources = "Resumo", "Dados do gráfico", "Detalhamento", "Parâmetros", "Fontes"
		labels.Metric, labels.Value, labels.Parameter, labels.Dataset, labels.Source, labels.DependsOn = "Métrica", "Valor", "Parâmetro", "Conjunto de dados", "Fonte", "Depende de"
		labels.Explorer, labels.Branch, labels.Perspective, labels.Node, labels.Path, labels.ExportMode, labels.Snapshot, labels.Drill = "Exploração", "Ramo", "Perspectiva", "Nó", "Caminho", "Modo de exportação", "Instantâneo", "Detalhamento"
		labels.AllTime, labels.Yes, labels.No = "Todo o período", "Sim", "Não"
		return labels
	default:
		return DefaultLabels()
	}
}

type Request struct {
	Result          *lensruntime.DashboardResult
	PanelID         string
	DrillPath       []string
	Labels          Labels
	Exploration     *explore.ExportRequest
	EvidenceSources map[string]EvidenceSource
	Spool           *SpoolOptions
}

type sheetTarget struct {
	Sheet string
	Cell  string
}

type Exporter struct{}

func New() *Exporter { return &Exporter{} }

// Write streams the archive to w; archive errors may leave partial output.
// Use Prepare or WriteHTTP when failed generation must not reach a response.
// SpoolOptions.MaxBytes bounds combined XML/archive work (default 256 MiB).
func (e *Exporter) Write(ctx context.Context, w io.Writer, req Request) (err error) {
	op := serrors.Op("lens/export.Exporter.Write")
	defer func() {
		if err != nil {
			err = serrors.Wrap(op, err)
		}
	}()
	if req.Result == nil {
		return fmt.Errorf("lens export result is required")
	}
	if req.PanelID != "" && req.Result.Panel(req.PanelID) == nil {
		return fmt.Errorf("panel %q not found", req.PanelID)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	work, err := newExportWork(ctx, req.Spool)
	if err != nil {
		return err
	}
	defer work.close()
	labels := req.Labels
	if labels.Summary == "" {
		labels = LabelsForLocale(req.Result.Locale)
	}
	file := excelize.NewFile(excelize.Options{TmpDir: work.dir})
	defer func() { _ = file.Close() }()
	book := newWorkbook(file)
	defaultSheet := file.GetSheetName(0)
	used := map[string]int{}
	newSheet := func(name string) (string, error) {
		name = uniqueSheetName(safeSheetName(name), used)
		_, err := file.NewSheet(name)
		return name, err
	}
	includeLineage := !req.Result.Spec.Export.OmitLineage
	summary, err := newSheet(labels.Summary)
	if err != nil {
		return err
	}
	parameters, err := newSheet(labels.Parameters)
	if err != nil {
		return err
	}
	sources := ""
	if includeLineage {
		if sources, err = newSheet(labels.Sources); err != nil {
			return err
		}
	}
	panels := selectedPanels(req.Result, req.PanelID)
	summaryRows := make([][]any, 0, 8)
	if title := strings.TrimSpace(req.Result.Spec.Title); title != "" {
		summaryRows = append(summaryRows, []any{title})
	}
	if req.PanelID != "" && len(panels) == 1 && !isSingleValuePanel(panels[0]) {
		if total, ok := panelTotal(panels[0]); ok {
			summaryRows = append(summaryRows, []any{panels[0].Panel.Title, formattedValue{Value: total, Format: panels[0].Panel.Formatter}})
		}
	}
	if len(req.DrillPath) > 0 {
		summaryRows = append(summaryRows, []any{labels.Drill, strings.Join(req.DrillPath, " > ")})
	}
	if includeLineage {
		summaryRows = append(summaryRows, []any{labels.Sources, sheetHyperlink(sources, labels.Sources)})
	}
	if req.Exploration != nil {
		if err := req.Exploration.Validate(); err != nil {
			return err
		}
		summaryRows = append(summaryRows, explorationSummaryRows(labels, *req.Exploration)...)
	}
	if err := book.writeRows(summary, summaryRows); err != nil {
		return err
	}
	if err := book.writeRows(parameters, parameterRows(req.Result, labels)); err != nil {
		return err
	}

	datasetTargets := map[string]sheetTarget{}
	exportOrder := exportDatasetNames(req.Result.Spec, panels, req.PanelID, includeLineage)
	for _, dataset := range exportOrder {
		if containsPanelDataset(panels, dataset) {
			continue
		}
		result := req.Result.Datasets[dataset]
		source, streamed := req.EvidenceSources[dataset]
		if !streamed && (result == nil || result.Frames == nil) {
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
		if streamed {
			if err := work.writeEvidence(ctx, book, sheet, source, datasetSpec.Export); err != nil {
				return err
			}
		} else {
			if err := book.writeFrameSet(sheet, result.Frames, hintFormats); err != nil {
				return err
			}
			if err := configureEvidenceSheet(file, sheet, result.Frames, datasetSpec.Export); err != nil {
				return err
			}
		}
		datasetTargets[dataset] = sheetTarget{Sheet: sheet, Cell: "A1"}
	}
	dashboardRow := len(summaryRows) + 2
	if len(summaryRows) == 0 {
		dashboardRow = 1
	}
	dashboardColumns := 2
	statRows := [][]any{{labels.Metric, labels.Value}}
	for _, panelResult := range panels {
		if !isSingleValuePanel(panelResult) {
			continue
		}
		value, _ := singleValue(panelResult)
		statRows = append(statRows, []any{panelResult.Panel.Title, formattedValue{Value: value, Format: panelFormats(panelResult.Panel)(valueField(panelResult))}})
		datasetTargets[panelResult.Panel.Dataset] = sheetTarget{Sheet: summary, Cell: cell(1, dashboardRow+len(statRows)-1)}
	}
	if len(statRows) > 1 {
		for i, row := range statRows {
			if err := book.writeRow(summary, dashboardRow+i, row); err != nil {
				return err
			}
		}
		dashboardRow += len(statRows) + 1
	}
	for _, panelResult := range panels {
		if panelResult == nil || panelResult.Frames == nil || isSingleValuePanel(panelResult) {
			continue
		}
		panelCell := cell(1, dashboardRow)
		if err := book.writeCell(summary, 1, dashboardRow, panelResult.Panel.Title); err != nil {
			return err
		}
		datasetTargets[panelResult.Panel.Dataset] = sheetTarget{Sheet: summary, Cell: panelCell}
		dashboardRow++
		nextRow, columns, writeErr := book.writeFrameSetAt(summary, panelResult.Frames, dashboardRow, panelFormats(panelResult.Panel))
		if writeErr != nil {
			return writeErr
		}
		if columns > dashboardColumns {
			dashboardColumns = columns
		}
		dashboardRow = nextRow + 1
	}
	if err := setSheetDimension(file, summary, dashboardRow-2, dashboardColumns); err != nil {
		return err
	}
	if includeLineage {
		sourceRows := [][]any{{labels.Dataset, labels.Source, labels.DependsOn}}
		for _, datasetName := range exportOrder {
			dataset := findDataset(req.Result.Spec, datasetName)
			datasetTitle := datasetDisplayName(req.Result.Spec, datasetName)
			title := any(datasetTitle)
			if target, ok := datasetTargets[datasetName]; ok {
				title = sheetCellHyperlink(target.Sheet, target.Cell, datasetTitle)
			}
			dependsOn := make([]string, 0, len(dataset.DependsOn))
			for _, dependency := range dataset.DependsOn {
				dependsOn = append(dependsOn, datasetDisplayName(req.Result.Spec, dependency))
			}
			sourceRows = append(sourceRows, []any{title, dataset.Source, strings.Join(dependsOn, ", ")})
		}
		if err := book.writeRows(sources, sourceRows); err != nil {
			return err
		}
	}
	if err := book.applyColumnWidths(); err != nil {
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
	// Excelize's default ZIP writer builds a whole-file bytes.Buffer. Its public
	// ZIP factory allows the archive itself to flow to the bounded destination.
	file.SetZipWriter(func(io.Writer) excelize.ZipWriter { return work.zipWriter(ctx, w) })
	return file.Write(io.Discard)
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

type datasetExportOptions struct {
	EvidenceDatasets []string
	IncludeUpstream  bool
}

func datasetExport(spec lens.DashboardSpec, name string) datasetExportOptions {
	out := datasetExportOptions{}
	for _, d := range spec.Datasets {
		if d.Name == name {
			out.EvidenceDatasets = append([]string(nil), d.Export.EvidenceDatasets...)
			out.IncludeUpstream = d.Export.IncludeUpstream
		}
	}
	return out
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
func isSingleValuePanel(p *lensruntime.PanelResult) bool {
	if p == nil || p.Frames == nil || (p.Panel.Kind != panel.KindStat && p.Panel.Kind != panel.KindGauge) {
		return false
	}
	primary := p.Frames.Primary()
	if primary == nil || primary.RowCount > 1 {
		return false
	}
	_, ok := primary.Field(p.Panel.Fields.Value.Name())
	return ok
}

func singleValue(p *lensruntime.PanelResult) (any, bool) {
	field := valueField(p)
	if len(field.Values) == 0 {
		return nil, false
	}
	return field.Values[0], true
}

func valueField(p *lensruntime.PanelResult) frame.Field {
	if p == nil || p.Frames == nil || p.Frames.Primary() == nil {
		return frame.Field{}
	}
	field, ok := p.Frames.Primary().Field(p.Panel.Fields.Value.Name())
	if !ok || field == nil {
		return frame.Field{}
	}
	return *field
}

func panelTotal(p *lensruntime.PanelResult) (float64, bool) {
	field := valueField(p)
	total, found := 0.0, false
	for _, v := range field.Values {
		if n, ok := numeric(v); ok {
			total += n
			found = true
		}
	}
	return total, found
}

type fieldFormats func(frame.Field) *format.Spec

func hintFormats(field frame.Field) *format.Spec {
	switch strings.ToLower(strings.TrimSpace(field.FormatterHint)) {
	case string(format.KindInteger), "count":
		return pointer(format.Count())
	case string(format.KindMoney):
		return pointer(format.Money("", 2))
	case string(format.KindAbbreviatedMoney), "money_compact", "number_compact":
		return pointer(format.NumberCompact())
	case string(format.KindPercent):
		return pointer(format.Percent(1))
	case string(format.KindDate):
		return pointer(format.Date(""))
	default:
		return nil
	}
}

func panelFormats(spec panel.Spec) fieldFormats {
	return func(field frame.Field) *format.Spec {
		for _, column := range spec.Columns {
			if column.Formatter != nil && column.Field.Name() == field.Name {
				return column.Formatter
			}
		}
		if spec.Formatter != nil && !spec.Fields.Value.Empty() && spec.Fields.Value.Name() == field.Name {
			return spec.Formatter
		}
		return hintFormats(field)
	}
}

func parameterRows(result *lensruntime.DashboardResult, labels Labels) [][]any {
	rows := [][]any{{labels.Parameter, labels.Value}}
	seen := map[string]bool{}
	for _, variable := range result.Spec.Variables {
		value, ok := result.Variables[variable.Name]
		if !ok {
			continue
		}
		display := variableDisplay(variable, value, labels)
		if display == nil || display == "" {
			continue
		}
		label := strings.TrimSpace(variable.Label)
		if label == "" {
			label = variable.Name
		}
		key := label + "\x00" + fmt.Sprint(display)
		if seen[key] {
			continue
		}
		seen[key] = true
		rows = append(rows, []any{label, display})
	}
	return rows
}

func variableDisplay(variable lens.VariableSpec, value any, labels Labels) any {
	switch v := value.(type) {
	case nil:
		return nil
	case lens.DateRangeValue:
		return dateRangeDisplay(v, labels)
	case *lens.DateRangeValue:
		if v == nil {
			return nil
		}
		return dateRangeDisplay(*v, labels)
	case lens.CompareValue:
		if v.Mode == "" || v.Mode == lens.CompareOff {
			return nil
		}
		period := dateRangeDisplay(v.Range, labels)
		if label := optionLabel(variable, string(v.Mode)); label != string(v.Mode) {
			if period == "" {
				return label
			}
			return label + " (" + period + ")"
		}
		return period
	case string:
		return optionLabel(variable, v)
	case []string:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if item = strings.TrimSpace(item); item != "" {
				out = append(out, optionLabel(variable, item))
			}
		}
		return strings.Join(out, ", ")
	case bool:
		if v {
			return fallbackLabel(labels.Yes, DefaultLabels().Yes)
		}
		return fallbackLabel(labels.No, DefaultLabels().No)
	case time.Time:
		return v.Format("02.01.2006")
	default:
		return scalar(v)
	}
}

func dateRangeDisplay(value lens.DateRangeValue, labels Labels) string {
	if value.Mode == string(datasource.TimeRangeModeAll) {
		return fallbackLabel(labels.AllTime, DefaultLabels().AllTime)
	}
	if value.Start == nil || value.End == nil {
		return ""
	}
	return value.Start.Format("02.01.2006") + " – " + value.End.Format("02.01.2006")
}

func optionLabel(variable lens.VariableSpec, value string) string {
	for _, option := range variable.Options {
		if option.Value == value && strings.TrimSpace(option.Label) != "" {
			return option.Label
		}
	}
	return value
}

func fallbackLabel(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func datasetDisplayName(spec lens.DashboardSpec, name string) string {
	if title := strings.TrimSpace(findDataset(spec, name).Title); title != "" {
		return title
	}
	return name
}

// formattedValue carries an explicit Lens formatter for a single cell.
type formattedValue struct {
	Value  any
	Format *format.Spec
}

type workbook struct {
	file   *excelize.File
	styles map[string]int
	widths map[string]map[int]int
	sheets []string
}

func newWorkbook(file *excelize.File) *workbook {
	return &workbook{file: file, styles: map[string]int{}, widths: map[string]map[int]int{}}
}

func (b *workbook) writeFrameSet(sheet string, frames *frame.FrameSet, formats fieldFormats) error {
	nextRow, maxColumns, err := b.writeFrameSetAt(sheet, frames, 1, formats)
	if err != nil {
		return err
	}
	return setSheetDimension(b.file, sheet, nextRow-1, maxColumns)
}

func (b *workbook) writeFrameSetAt(sheet string, frames *frame.FrameSet, startRow int, formats fieldFormats) (int, int, error) {
	row := startRow
	maxColumns := 1
	for index, fr := range frames.Frames {
		if index > 0 {
			row += 2
		}
		if fr.Meta.Title != "" {
			if err := b.writeCell(sheet, 1, row, fr.Meta.Title); err != nil {
				return row, maxColumns, err
			}
			row++
		}
		headers := make([]any, len(fr.Fields))
		if len(fr.Fields) > maxColumns {
			maxColumns = len(fr.Fields)
		}
		specs := make([]*format.Spec, len(fr.Fields))
		for i, f := range fr.Fields {
			headers[i] = fieldLabel(f)
			specs[i] = formats(f)
		}
		if err := b.writeRow(sheet, row, headers); err != nil {
			return row, maxColumns, err
		}
		row++
		for i := 0; i < fr.RowCount; i++ {
			for col, f := range fr.Fields {
				if err := b.writeCell(sheet, col+1, row, formattedValue{Value: f.Values[i], Format: specs[col]}); err != nil {
					return row, maxColumns, err
				}
			}
			row++
		}
	}
	return row, maxColumns, nil
}

func (b *workbook) writeRows(sheet string, rows [][]any) error {
	maxColumns := 1
	for i, row := range rows {
		if len(row) > maxColumns {
			maxColumns = len(row)
		}
		if err := b.writeRow(sheet, i+1, row); err != nil {
			return err
		}
	}
	return setSheetDimension(b.file, sheet, len(rows), maxColumns)
}

func (b *workbook) writeRow(sheet string, row int, values []any) error {
	for col, value := range values {
		if err := b.writeCell(sheet, col+1, row, value); err != nil {
			return err
		}
	}
	return nil
}

func (b *workbook) writeCell(sheet string, col, row int, value any) error {
	var spec *format.Spec
	if formatted, ok := value.(formattedValue); ok {
		value, spec = formatted.Value, formatted.Format
	}
	exact := exactValue(value)
	var err error
	value, err = normalizeExactValue(value)
	if err != nil {
		return err
	}
	coordinate := cell(col, row)
	if err := writeCell(b.file, sheet, coordinate, value); err != nil {
		return err
	}
	b.track(sheet, col, displayWidth(value))
	numFmt := numberFormat(spec, value)
	if exact && spec == nil {
		numFmt = "General"
	}
	if numFmt == "" {
		return nil
	}
	style, err := b.style(numFmt)
	if err != nil {
		return err
	}
	return b.file.SetCellStyle(sheet, coordinate, coordinate, style)
}

func (b *workbook) style(numFmt string) (int, error) {
	if id, ok := b.styles[numFmt]; ok {
		return id, nil
	}
	id, err := b.file.NewStyle(&excelize.Style{CustomNumFmt: pointer(numFmt)})
	if err != nil {
		return 0, err
	}
	b.styles[numFmt] = id
	return id, nil
}

func (b *workbook) track(sheet string, col, width int) {
	widths, ok := b.widths[sheet]
	if !ok {
		widths = map[int]int{}
		b.widths[sheet] = widths
		b.sheets = append(b.sheets, sheet)
	}
	if width > widths[col] {
		widths[col] = width
	}
}

func (b *workbook) applyColumnWidths() error {
	const minWidth, maxWidth = 10, 60
	for _, sheet := range b.sheets {
		for col, width := range b.widths[sheet] {
			width = min(max(width+2, minWidth), maxWidth)
			name, err := excelize.ColumnNumberToName(col)
			if err != nil {
				return err
			}
			if err := b.file.SetColWidth(sheet, name, name, float64(width)); err != nil {
				return err
			}
		}
	}
	return nil
}

func displayWidth(value any) int {
	switch v := value.(type) {
	case nil:
		return 0
	case string:
		return utf8.RuneCountInString(v)
	case frame.Hyperlink:
		return utf8.RuneCountInString(v.Label)
	case *frame.Hyperlink:
		if v == nil {
			return 0
		}
		return utf8.RuneCountInString(v.Label)
	case frame.Formula:
		return displayWidth(v.Result)
	case *frame.Formula:
		if v == nil {
			return 0
		}
		return displayWidth(v.Result)
	case time.Time, *time.Time:
		return 16
	}
	if n, ok := numeric(value); ok {
		digits := len(strconv.FormatFloat(math.Abs(math.Trunc(n)), 'f', 0, 64))
		return digits + digits/3 + 4
	}
	return utf8.RuneCountInString(fmt.Sprint(value))
}

func numberFormat(spec *format.Spec, value any) string {
	switch v := value.(type) {
	case frame.Formula:
		if spec == nil {
			return ""
		}
		value = v.Result
	case *frame.Formula:
		if v == nil || spec == nil {
			return ""
		}
		value = v.Result
	case time.Time:
		return dateFormat(v)
	case *time.Time:
		if v == nil {
			return ""
		}
		return dateFormat(*v)
	}
	n, ok := numeric(value)
	if !ok {
		return ""
	}
	if spec == nil {
		if n == math.Trunc(n) {
			return "#,##0"
		}
		return "#,##0.00"
	}
	switch spec.Kind {
	case format.KindInteger:
		return "#,##0"
	case format.KindPercent:
		// Lens percent values are already scaled (45.3 means 45.3%), so the
		// sign is a literal instead of Excel's multiply-by-100 percent format.
		return decimalFormat(spec.Precision) + `"%"`
	case format.KindMoney, format.KindAbbreviatedMoney:
		return decimalFormat(spec.Precision)
	case format.KindDate, format.KindMonthLabel, format.KindDuration, format.KindLocalizedString:
		return numberFormat(nil, value)
	default:
		return numberFormat(nil, value)
	}
}

func dateFormat(value time.Time) string {
	if value.Hour() == 0 && value.Minute() == 0 && value.Second() == 0 {
		return "dd.mm.yyyy"
	}
	return "dd.mm.yyyy hh:mm"
}

func decimalFormat(precision int) string {
	if precision <= 0 {
		return "#,##0"
	}
	return "#,##0." + strings.Repeat("0", precision)
}

func numeric(value any) (float64, bool) {
	switch n := value.(type) {
	case float64:
		return n, !math.IsNaN(n) && !math.IsInf(n, 0)
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	default:
		return 0, false
	}
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
	headerRow := 1
	if primary.Meta.Title != "" {
		headerRow++
	}
	if spec.FreezeHeader {
		if err := file.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: headerRow, TopLeftCell: cell(1, headerRow+1), ActivePane: "bottomLeft"}); err != nil {
			return err
		}
	}
	if tableName := strings.TrimSpace(spec.TableName); tableName != "" && len(primary.Fields) > 0 {
		endColumn, err := excelize.ColumnNumberToName(len(primary.Fields))
		if err != nil {
			return err
		}
		endRow := headerRow + primary.RowCount
		if endRow <= headerRow {
			endRow = headerRow + 1
		}
		if err := file.AddTable(sheet, &excelize.Table{
			Range:          fmt.Sprintf("A%d:%s%d", headerRow, endColumn, endRow),
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
	name = strings.TrimSpace(strings.Trim(strings.TrimSpace(invalidSheetChars.ReplaceAllString(name, " ")), "'"))
	if name == "" {
		name = "Data"
	}
	r := []rune(name)
	if len(r) > 31 {
		name = strings.TrimSpace(strings.Trim(string(r[:31]), "'"))
	}
	return name
}
func uniqueSheetName(name string, used map[string]int) string {
	base := name
	for {
		key := strings.ToLower(name)
		if _, ok := used[key]; !ok {
			used[key] = 1
			return name
		}
		baseKey := strings.ToLower(base)
		used[baseKey]++
		suffix := fmt.Sprintf(" %d", used[baseKey])
		r := []rune(base)
		if len(r)+len([]rune(suffix)) > 31 {
			r = r[:31-len([]rune(suffix))]
		}
		name = string(r) + suffix
	}
}
