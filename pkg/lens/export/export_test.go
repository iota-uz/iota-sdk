package export

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/explore"
	"github.com/iota-uz/iota-sdk/pkg/lens/exportmeta"
	"github.com/iota-uz/iota-sdk/pkg/lens/format"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func TestExporterWrite_PanelAndEvidenceWithoutDatasourceQueries(t *testing.T) {
	t.Parallel()
	chart := mustFrames(t, "chart", frame.Field{Name: "label", Values: []any{"A"}}, frame.Field{Name: "value", Values: []any{42.0}})
	evidence := mustFrames(t, "evidence",
		frame.Field{Name: "policy", Values: []any{frame.Hyperlink{URL: "https://example.test/policies/1", Label: "P-1"}}},
		frame.Field{Name: "amount", Values: []any{42.0}},
		frame.Field{Name: "verified", Values: []any{frame.Formula{Expression: "[@amount]", Result: 42.0}}},
	)
	panelSpec := panel.Spec{ID: "premium", Title: "Premium", Kind: panel.KindPie, Dataset: "chart", Fields: panel.FieldMapping{Label: panel.Ref("label"), Value: panel.Ref("value")}}
	result := &lensruntime.DashboardResult{SnapshotID: "snap", Spec: lens.DashboardSpec{Title: "Dashboard", Datasets: []lens.DatasetSpec{{Name: "chart", Export: exportmeta.Spec{EvidenceDatasets: []string{"evidence"}}}, {Name: "evidence", Title: "Policy evidence", Export: exportmeta.Spec{SheetName: "Policies", TableName: "Policies", FreezeHeader: true}}}}, Variables: map[string]any{"year": 2026}, Datasets: map[string]*lensruntime.DatasetResult{"chart": {Frames: chart}, "evidence": {Frames: evidence}}, Panels: map[string]*lensruntime.PanelResult{"premium": {Panel: panelSpec, Frames: chart}}}
	var out bytes.Buffer
	require.NoError(t, New().Write(context.Background(), &out, Request{Result: result, PanelID: "premium"}))
	book, err := excelize.OpenReader(bytes.NewReader(out.Bytes()))
	require.NoError(t, err)
	defer func() { _ = book.Close() }()
	require.NotContains(t, book.GetSheetList(), "Premium")
	require.Contains(t, book.GetSheetList(), "Policies")
	require.Equal(t, []string{"Summary", "Parameters", "Sources", "Policies"}, book.GetSheetList())
	formula, err := book.GetCellFormula("Summary", "B3")
	require.NoError(t, err)
	require.Equal(t, `HYPERLINK("#'Sources'!A1","Sources")`, formula)
	total, err := book.GetCellValue("Summary", "B2")
	require.NoError(t, err)
	require.Equal(t, "42", total)
	formula, err = book.GetCellFormula("Sources", "A2")
	require.NoError(t, err)
	require.Equal(t, `HYPERLINK("#'Summary'!A5","chart")`, formula)
	formula, err = book.GetCellFormula("Sources", "A3")
	require.NoError(t, err)
	require.Equal(t, `HYPERLINK("#'Policies'!A1","Policy evidence")`, formula)
	value, err := book.GetCellValue("Summary", "A5")
	require.NoError(t, err)
	require.Equal(t, "Premium", value)
	value, err = book.GetCellValue("Summary", "A6")
	require.NoError(t, err)
	require.Equal(t, "label", value)
	value, err = book.GetCellValue("Summary", "B7")
	require.NoError(t, err)
	require.Equal(t, "42", value)
	dimension, err := book.GetSheetDimension("Policies")
	require.NoError(t, err)
	require.Equal(t, "A1:C2", dimension)
	formula, err = book.GetCellFormula("Policies", "C2")
	require.NoError(t, err)
	require.Equal(t, "[@amount]", formula)
	formula, err = book.GetCellFormula("Policies", "A2")
	require.NoError(t, err)
	require.Equal(t, `HYPERLINK("https://example.test/policies/1","P-1")`, formula)
	tables, err := book.GetTables("Policies")
	require.NoError(t, err)
	require.Len(t, tables, 1)
	require.Equal(t, "Policies", tables[0].Name)
}

func TestLabelsForLocale_SupportedLocales(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, locale, summary string }{
		{"Russian", "ru", "Сводка"}, {"Uzbek Latin", "uz", "Xulosa"},
		{"Uzbek Cyrillic", "uz-Cyrl", "Хулоса"}, {"English", "en", "Summary"},
		{"Portuguese Brazil", "pt-BR", "Resumo"},
	} {
		t.Run(tt.name, func(t *testing.T) { t.Parallel(); require.Equal(t, tt.summary, LabelsForLocale(tt.locale).Summary) })
	}
}

func TestSelectedPanelsFollowsDashboardLayout(t *testing.T) {
	t.Parallel()
	result := &lensruntime.DashboardResult{
		Spec: lens.DashboardSpec{Rows: []lens.RowSpec{{Panels: []panel.Spec{
			{ID: "second", Kind: panel.KindStat},
			{ID: "group", Kind: panel.KindStatGroup, Children: []panel.Spec{{ID: "first", Kind: panel.KindStat}}},
		}}}},
		Panels: map[string]*lensruntime.PanelResult{
			"first":  {Panel: panel.Spec{ID: "first", Kind: panel.KindStat}},
			"second": {Panel: panel.Spec{ID: "second", Kind: panel.KindStat}},
		},
	}

	panels := selectedPanels(result, "")
	require.Len(t, panels, 2)
	require.Equal(t, "second", panels[0].Panel.ID)
	require.Equal(t, "first", panels[1].Panel.ID)
}

func TestExporterRejectsMissingDeclaredEvidence(t *testing.T) {
	t.Parallel()
	chart := mustFrames(t, "chart", frame.Field{Name: "value", Values: []any{1}})
	panelSpec := panel.Spec{ID: "premium", Title: "Premium", Dataset: "chart", Export: exportmeta.Spec{EvidenceDatasets: []string{"missing"}}}
	result := &lensruntime.DashboardResult{
		Spec:     lens.DashboardSpec{Datasets: []lens.DatasetSpec{{Name: "chart"}}},
		Datasets: map[string]*lensruntime.DatasetResult{"chart": {Frames: chart}},
		Panels:   map[string]*lensruntime.PanelResult{"premium": {Panel: panelSpec, Frames: chart}},
	}
	err := New().Write(context.Background(), io.Discard, Request{Result: result, PanelID: "premium"})
	require.ErrorContains(t, err, `declared export evidence dataset "missing" is unavailable`)
}

func TestExporterWrite_RejectsUnknownPanel(t *testing.T) {
	t.Parallel()
	err := New().Write(context.Background(), io.Discard, Request{Result: &lensruntime.DashboardResult{}, PanelID: "missing"})
	require.ErrorContains(t, err, `panel "missing" not found`)
}

func TestExporterWrite_OffsetsTitledEvidenceTable(t *testing.T) {
	t.Parallel()
	evidence := mustFrames(t, "evidence", frame.Field{Name: "policy", Values: []any{"P-1"}})
	evidence.Primary().Meta.Title = "Policy evidence"
	chart := mustFrames(t, "chart", frame.Field{Name: "value", Values: []any{1}})
	panelSpec := panel.Spec{ID: "premium", Dataset: "chart", Export: exportmeta.Spec{EvidenceDatasets: []string{"evidence"}}}
	result := &lensruntime.DashboardResult{Spec: lens.DashboardSpec{Datasets: []lens.DatasetSpec{{Name: "chart"}, {Name: "evidence", Export: exportmeta.Spec{TableName: "Evidence", FreezeHeader: true}}}}, Datasets: map[string]*lensruntime.DatasetResult{"chart": {Frames: chart}, "evidence": {Frames: evidence}}, Panels: map[string]*lensruntime.PanelResult{"premium": {Panel: panelSpec, Frames: chart}}}
	var out bytes.Buffer
	require.NoError(t, New().Write(context.Background(), &out, Request{Result: result, PanelID: "premium"}))
	book, err := excelize.OpenReader(bytes.NewReader(out.Bytes()))
	require.NoError(t, err)
	defer func() { _ = book.Close() }()
	tables, err := book.GetTables("Breakdown evidence")
	require.NoError(t, err)
	require.Len(t, tables, 1)
	require.Equal(t, "A2:A3", tables[0].Range)
	panes, err := book.GetPanes("Breakdown evidence")
	require.NoError(t, err)
	require.True(t, panes.Freeze)
	require.Equal(t, 2, panes.YSplit)
}

func TestSheetNames_AreExcelSafeAndCaseInsensitive(t *testing.T) {
	t.Parallel()
	require.Equal(t, "quoted", safeSheetName("'quoted'"))
	used := map[string]int{}
	require.Equal(t, "Summary", uniqueSheetName("Summary", used))
	require.Equal(t, "summary 2", uniqueSheetName("summary", used))
}

func TestExporterWritesExplorationMetadataWithoutChangingLegacyLayout(t *testing.T) {
	t.Parallel()

	chart := mustFrames(t, "chart", frame.Field{Name: "label", Values: []any{"A"}}, frame.Field{Name: "value", Values: []any{42.0}})
	panelSpec := panel.Spec{ID: "premium", Title: "Premium", Kind: panel.KindPie, Dataset: "chart", Fields: panel.FieldMapping{Label: panel.Ref("label"), Value: panel.Ref("value")}}
	result := &lensruntime.DashboardResult{
		Spec:     lens.DashboardSpec{Title: "Dashboard", Datasets: []lens.DatasetSpec{{Name: "chart"}}},
		Datasets: map[string]*lensruntime.DatasetResult{"chart": {Frames: chart}},
		Panels:   map[string]*lensruntime.PanelResult{"premium": {Panel: panelSpec, Frames: chart}},
	}
	selection := &explore.ExportRequest{
		Mode:           explore.ExportCurrentView,
		ExplorerID:     "premium-explorer",
		BranchKey:      "unearned",
		PerspectiveKey: "products",
		Path:           []string{"root", "property"},
		NodeKey:        "property",
		Labels: explore.ExportLabels{
			Explorer: "Premium analysis", Branch: "Unearned", Perspective: "Products", Node: "Property",
		},
	}
	var out bytes.Buffer
	require.NoError(t, New().Write(context.Background(), &out, Request{Result: result, PanelID: "premium", Exploration: selection}))
	book, err := excelize.OpenReader(bytes.NewReader(out.Bytes()))
	require.NoError(t, err)
	defer func() { _ = book.Close() }()

	value, err := book.GetCellValue("Summary", "B7")
	require.NoError(t, err)
	require.Equal(t, "Products", value)
	value, err = book.GetCellValue("Summary", "B8")
	require.NoError(t, err)
	require.Equal(t, "Property", value)
	value, err = book.GetCellValue("Summary", "A11")
	require.NoError(t, err)
	require.Equal(t, "Premium", value)
}

func TestExporterWrite_ReadableBusinessWorkbook(t *testing.T) {
	t.Parallel()

	policies := mustFrames(t, "policies", frame.Field{Name: "value", Labels: map[string]string{"default": "Полисов"}, Values: []any{1234.0}})
	premium := mustFrames(t, "premium", frame.Field{Name: "value", Labels: map[string]string{"default": "Премия"}, Values: []any{1063542000.5}})
	byProduct := mustFrames(t, "cube_dim_product",
		frame.Field{Name: "label", Values: []any{"ОСАГО", "КАСКО"}},
		frame.Field{Name: "value", Values: []any{1000000000.0, 63542000.5}},
		frame.Field{Name: "share", FormatterHint: "percent", Values: []any{94.0, 6.0}},
		frame.Field{Name: "issued", Values: []any{time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)}},
	)
	money := format.Money("UZS", 2)
	policiesPanel := panel.Spec{ID: "policies", Title: "Полисов", Kind: panel.KindStat, Dataset: "policies", Fields: panel.FieldMapping{Value: panel.Ref("value")}, Formatter: pointer(format.Count())}
	premiumPanel := panel.Spec{ID: "premium", Title: "Премия", Kind: panel.KindStat, Dataset: "premium", Fields: panel.FieldMapping{Value: panel.Ref("value")}, Formatter: &money}
	productPanel := panel.Spec{ID: "product", Title: "По продуктам", Kind: panel.KindBar, Dataset: "cube_dim_product", Fields: panel.FieldMapping{Label: panel.Ref("label"), Value: panel.Ref("value")}, Formatter: &money}
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 10, 21, 23, 59, 59, 0, time.UTC)
	result := &lensruntime.DashboardResult{
		SnapshotID: "snapshot-123",
		Locale:     "ru",
		Spec: lens.DashboardSpec{
			Title: "Продажи",
			Rows: []lens.RowSpec{
				{Panels: []panel.Spec{{ID: "kpis", Kind: panel.KindStatGroup, Children: []panel.Spec{policiesPanel, premiumPanel}}}},
				{Panels: []panel.Spec{productPanel}},
			},
			Variables: []lens.VariableSpec{
				{Name: "period", Label: "Период", Kind: lens.VariableDateRange},
				{Name: "compare", Label: "Сравнение", Kind: lens.VariableCompare, CompareTo: "period"},
				{Name: "channel", Label: "Канал", Kind: lens.VariableSingleSelect, Options: []lens.VariableOption{{Label: "Онлайн", Value: "online"}}},
				{Name: "all", Kind: lens.VariableDateRange},
			},
			Datasets: []lens.DatasetSpec{
				{Name: "policies", Title: "Полисов", Source: "primary"},
				{Name: "premium", Source: "primary"},
				{Name: "cube_dim_product", Title: "Продукт", Source: "primary", DependsOn: []string{"policies"}},
			},
		},
		Variables: map[string]any{
			"period":   lens.DateRangeValue{Mode: "bounded", Start: &start, End: &end},
			"compare":  lens.CompareValue{Mode: lens.CompareOff},
			"channel":  "online",
			"all":      lens.DateRangeValue{Mode: "all"},
			"timezone": "Asia/Tashkent",
		},
		Datasets: map[string]*lensruntime.DatasetResult{"policies": {Frames: policies}, "premium": {Frames: premium}, "cube_dim_product": {Frames: byProduct}},
		Panels: map[string]*lensruntime.PanelResult{
			"policies": {Panel: policiesPanel, Frames: policies},
			"premium":  {Panel: premiumPanel, Frames: premium},
			"product":  {Panel: productPanel, Frames: byProduct},
		},
	}
	var out bytes.Buffer
	require.NoError(t, New().Write(context.Background(), &out, Request{Result: result}))
	book, err := excelize.OpenReader(bytes.NewReader(out.Bytes()))
	require.NoError(t, err)
	defer func() { _ = book.Close() }()

	summary := sheetRows(t, book, "Сводка")
	require.Equal(t, [][]string{
		{"Продажи"},
		{"Источники", ""},
		nil,
		{"Показатель", "Значение"},
		{"Полисов", "1,234"},
		{"Премия", "1,063,542,000.50"},
		nil,
		{"По продуктам"},
		{"label", "value", "share", "issued"},
		{"ОСАГО", "1,000,000,000.00", "94.0%", "01.10.2026"},
		{"КАСКО", "63,542,000.50", "6.0%", "02.10.2026"},
	}, summary)
	require.Equal(t, "#,##0", cellNumFmt(t, book, "Сводка", "B5"))
	require.Equal(t, "#,##0.00", cellNumFmt(t, book, "Сводка", "B6"))
	require.Equal(t, `#,##0.0"%"`, cellNumFmt(t, book, "Сводка", "C10"))
	width, err := book.GetColWidth("Сводка", "B")
	require.NoError(t, err)
	require.GreaterOrEqual(t, width, 15.0)

	require.Equal(t, [][]string{
		{"Параметр", "Значение"},
		{"Период", "01.10.2026 – 21.10.2026"},
		{"Канал", "Онлайн"},
		{"all", "За всё время"},
	}, sheetRows(t, book, "Параметры"))

	sources := sheetRows(t, book, "Источники")
	require.Len(t, sources, 4)
	require.Equal(t, []string{"", "primary", "Полисов"}, sources[3])
	formula, err := book.GetCellFormula("Источники", "A4")
	require.NoError(t, err)
	require.Equal(t, `HYPERLINK("#'Сводка'!A8","Продукт")`, formula)
	formula, err = book.GetCellFormula("Источники", "A3")
	require.NoError(t, err)
	require.Equal(t, `HYPERLINK("#'Сводка'!A6","premium")`, formula)
	for _, sheet := range book.GetSheetList() {
		for _, row := range sheetRows(t, book, sheet) {
			for _, value := range row {
				for _, raw := range []string{"<nil>", "{off", "bounded", "cube_dim_", "snapshot-123", "Asia/Tashkent"} {
					require.NotContains(t, value, raw, "sheet %s", sheet)
				}
			}
		}
	}
}

func TestExporterWrite_OmitLineage(t *testing.T) {
	t.Parallel()
	chart := mustFrames(t, "chart", frame.Field{Name: "value", Values: []any{1}})
	panelSpec := panel.Spec{ID: "total", Title: "Total", Kind: panel.KindStat, Dataset: "chart", Fields: panel.FieldMapping{Value: panel.Ref("value")}}
	scope := mustFrames(t, "scope", frame.Field{Name: "identity", Values: []any{"8720f763"}})
	rawRows := mustFrames(t, "raw", frame.Field{Name: "number", Values: []any{"SR-001"}})
	period := lens.DateRangeValue{Mode: "bounded", Start: pointer(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)), End: pointer(time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC))}
	result := &lensruntime.DashboardResult{
		Spec: lens.DashboardSpec{
			Export: exportmeta.Spec{OmitLineage: true, EvidenceDatasets: []string{"scope", "raw"}},
			Variables: []lens.VariableSpec{
				{Name: "a__period", Label: "Period", Kind: lens.VariableDateRange},
				{Name: "b__period", Label: "Period", Kind: lens.VariableDateRange},
			},
			Datasets: []lens.DatasetSpec{
				{Name: "chart"},
				{Name: "scope", Export: exportmeta.Spec{Enabled: true, Lineage: true, SheetName: "Scope a"}},
				{Name: "raw", Export: exportmeta.Spec{Enabled: true, SheetName: "Rows"}},
			},
		},
		Variables: map[string]any{"a__period": period, "b__period": period},
		Datasets:  map[string]*lensruntime.DatasetResult{"chart": {Frames: chart}, "scope": {Frames: scope}, "raw": {Frames: rawRows}},
		Panels:    map[string]*lensruntime.PanelResult{"total": {Panel: panelSpec, Frames: chart}},
	}
	var out bytes.Buffer
	require.NoError(t, New().Write(context.Background(), &out, Request{Result: result}))
	book, err := excelize.OpenReader(bytes.NewReader(out.Bytes()))
	require.NoError(t, err)
	defer func() { _ = book.Close() }()
	require.Equal(t, []string{"Summary", "Parameters", "Rows"}, book.GetSheetList(), "lineage goes, business evidence stays")
	require.Equal(t, [][]string{{"Metric", "Value"}, {"Total", "1"}}, sheetRows(t, book, "Summary"))
	require.Equal(t, [][]string{{"Parameter", "Value"}, {"Period", "01.06.2026 – 30.06.2026"}}, sheetRows(t, book, "Parameters"))
}

func sheetRows(t *testing.T, book *excelize.File, sheet string) [][]string {
	t.Helper()
	rows, err := book.GetRows(sheet)
	require.NoError(t, err)
	for i, row := range rows {
		if strings.Join(row, "") == "" {
			rows[i] = nil
		}
	}
	return rows
}

func cellNumFmt(t *testing.T, book *excelize.File, sheet, coordinate string) string {
	t.Helper()
	id, err := book.GetCellStyle(sheet, coordinate)
	require.NoError(t, err)
	style, err := book.GetStyle(id)
	require.NoError(t, err)
	require.NotNil(t, style.CustomNumFmt)
	return *style.CustomNumFmt
}

func mustFrames(t *testing.T, name string, fields ...frame.Field) *frame.FrameSet {
	t.Helper()
	fr, err := frame.New(name, fields...)
	require.NoError(t, err)
	fs, err := frame.NewFrameSet(fr)
	require.NoError(t, err)
	return fs
}

func TestExporterWrite_SinglePanelExportKeepsWholeFrame(t *testing.T) {
	t.Parallel()
	kpis := mustFrames(t, "kpis",
		frame.Field{Name: "expense", Values: []any{nil}},
		frame.Field{Name: "revenue", Values: []any{132.3}},
	)
	panelSpec := panel.Spec{ID: "expense", Title: "Expense", Kind: panel.KindStat, Dataset: "kpis", Fields: panel.FieldMapping{Value: panel.Ref("expense")}}
	result := &lensruntime.DashboardResult{
		Spec:     lens.DashboardSpec{Datasets: []lens.DatasetSpec{{Name: "kpis"}}},
		Datasets: map[string]*lensruntime.DatasetResult{"kpis": {Frames: kpis}},
		Panels:   map[string]*lensruntime.PanelResult{"expense": {Panel: panelSpec, Frames: kpis}},
	}
	var out bytes.Buffer
	require.NoError(t, New().Write(context.Background(), &out, Request{Result: result, PanelID: "expense"}))
	book, err := excelize.OpenReader(bytes.NewReader(out.Bytes()))
	require.NoError(t, err)
	defer func() { _ = book.Close() }()
	rows, err := book.GetRows("Summary", excelize.Options{RawCellValue: true})
	require.NoError(t, err)
	require.Contains(t, rows, []string{"expense", "revenue"})
	require.Contains(t, rows, []string{"", "132.3"})
}
