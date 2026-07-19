package templ

import (
	"bytes"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/format"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func TestSegmentBar_HeadlineOverrideAndRowActions(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("allocation",
		frame.Field{Name: "segment", Type: frame.FieldTypeString, Values: []any{"Reserve", "Earned"}},
		frame.Field{Name: "amount", Type: frame.FieldTypeNumber, Values: []any{40.0, 60.0}},
		frame.Field{Name: "action_url", Type: frame.FieldTypeString, Values: []any{"/drill/reserve", "/drill/earned"}},
	)
	require.NoError(t, err)
	set, err := frame.NewFrameSet(fr)
	require.NoError(t, err)

	spec := panel.SegmentBar("allocation", "Premium allocation", "allocation").
		LabelField("segment").
		ValueField("amount").
		HeadlineValue(40).
		Action(action.HtmxSwap("", "#drawer").WithFieldURL("action_url")).
		Build()
	result := &runtime.PanelResult{Panel: spec, Frames: set, Locale: "en"}

	view := buildSegmentBarView(spec, result)
	require.Equal(t, "40.00", view.Total)
	require.Len(t, view.Segments, 2)
	require.Equal(t, "/drill/reserve", view.Segments[0].Href)
	require.Equal(t, "/drill/earned", view.Segments[1].Href)

	var html bytes.Buffer
	err = SegmentBarPanel(spec, result, nil).Render(metricInfoContext(t, language.English), &html)
	require.NoError(t, err)
	rendered := html.String()
	assert.Equal(t, 2, strings.Count(rendered, `href="/drill/reserve"`), "track and legend must both open the reserve drill")
	assert.Equal(t, 2, strings.Count(rendered, `href="/drill/earned"`), "track and legend must both open the earned drill")
	assert.Contains(t, rendered, "window.__lensDrillAjax")
}

func TestPie_HeadlineValueAndDescription(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("allocation",
		frame.Field{Name: "segment", Type: frame.FieldTypeString, Values: []any{"Reserve", "Earned"}},
		frame.Field{Name: "amount", Type: frame.FieldTypeNumber, Values: []any{40.0, 60.0}},
	)
	require.NoError(t, err)
	set, err := frame.NewFrameSet(fr)
	require.NoError(t, err)

	spec := panel.Pie("allocation", "Premium allocation", "allocation").
		LabelField("segment").
		ValueField("amount").
		HeadlineValue(40).
		Description("Remaining in reserve · allocation of received premium").
		Build()
	result := &runtime.PanelResult{Panel: spec, Frames: set, Locale: "en"}

	var html bytes.Buffer
	err = ChartPanel(spec, result, nil).Render(metricInfoContext(t, language.English), &html)
	require.NoError(t, err)
	rendered := html.String()
	assert.Contains(t, rendered, "40.00")
	assert.Contains(t, rendered, "Remaining in reserve · allocation of received premium")
}

// A row carrying a Heading renders as a section band (label + hairline rule),
// not a panel grid.
func TestDashboard_RendersSectionHeading(t *testing.T) {
	t.Parallel()

	spec := lens.DashboardSpec{
		ID: "sectioned",
		Rows: []lens.RowSpec{
			{Heading: "Премии"},
		},
	}

	var html bytes.Buffer
	err := Dashboard(DashboardProps{Spec: spec}).Render(metricInfoContext(t, language.English), &html)
	require.NoError(t, err)

	rendered := html.String()
	assert.Contains(t, rendered, "Премии")
	assert.Contains(t, rendered, "lens-microlabel")
}

func TestPanelExportButton_RendersDownloadHandshakeState(t *testing.T) {
	t.Parallel()

	spec := panel.Bar("premium", "Premium", "premium").
		Export("/analytics/export?dashboard=profitability", "").
		Build()

	var html bytes.Buffer
	err := PanelExportButton(spec).Render(metricInfoContext(t, language.English), &html)
	require.NoError(t, err)

	rendered := html.String()
	assert.Contains(t, rendered, "data-lens-export-button")
	assert.Contains(t, rendered, "data-lens-export-idle")
	assert.Contains(t, rendered, "data-lens-export-loading")
	assert.Contains(t, rendered, "Generating Excel report…")
	assert.Contains(t, rendered, "animate-spin")
}

func TestDashboardExportButton_UsesCanonicalDownloadHandshake(t *testing.T) {
	t.Parallel()

	var html bytes.Buffer
	err := DashboardExportButton(DashboardExportButtonProps{
		URL:          "/analytics/export?format=excel",
		ParamsFormID: "analytics-filters",
	}).Render(metricInfoContext(t, language.English), &html)
	require.NoError(t, err)

	rendered := html.String()
	assert.Contains(t, rendered, "data-lens-export-button")
	assert.Contains(t, rendered, `data-lens-export-params-form="analytics-filters"`)
	assert.Contains(t, rendered, "Export to Excel")
	assert.Contains(t, rendered, "Generating Excel report…")
	assert.Contains(t, rendered, "Export failed. Please try again.")
}

func TestDashboardScripts_SerializesExportFiltersAndHandlesFailure(t *testing.T) {
	t.Parallel()

	var html bytes.Buffer
	require.NoError(t, DashboardScripts().Render(metricInfoContext(t, language.English), &html))

	rendered := html.String()
	assert.Contains(t, rendered, "button.dataset.lensExportParamsForm")
	assert.Contains(t, rendered, "new FormData(form)")
	assert.Contains(t, rendered, "finish(signal)")
	assert.Contains(t, rendered, "signal !== 'started'")
}

func TestDashboard_RendersPanelsWhenHeadingAlsoPresent(t *testing.T) {
	t.Parallel()

	spec := lens.DashboardSpec{
		ID: "sectioned",
		Rows: []lens.RowSpec{
			{
				Heading: " Summary ",
				Panels: []panel.Spec{
					panel.Stat("total", "Total", "stats").Build(),
				},
			},
		},
	}

	var html bytes.Buffer
	err := Dashboard(DashboardProps{Spec: spec}).Render(metricInfoContext(t, language.English), &html)
	require.NoError(t, err)

	rendered := html.String()
	assert.Contains(t, rendered, "Summary")
	assert.Contains(t, rendered, `data-lens-panel-id="total"`)
	assert.Contains(t, rendered, "grid grid-cols-12 gap-3")
}

// Stat card v2: one .lens-stat layout with a label row (accent swatch +
// microlabel) and a lens-value-lg headline — no icon-tile / accent-bar chrome
// variants.
func TestStatPanel_V2SwatchLabelValue(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("kpis",
		frame.Field{Name: "earned_premium", Type: frame.FieldTypeNumber, Values: []any{1000.0}},
	)
	require.NoError(t, err)
	set, err := frame.NewFrameSet(fr)
	require.NoError(t, err)

	spec := panel.Stat("kpi-earned", "Заработанная премия", "kpis").
		AccentColor("#2563eb").
		ValueField("earned_premium").
		Build()

	result := &runtime.PanelResult{Panel: spec, Frames: set, Locale: "en"}

	var html bytes.Buffer
	err = StatPanel(spec, result, nil).Render(metricInfoContext(t, language.English), &html)
	require.NoError(t, err)

	rendered := html.String()
	assert.Contains(t, rendered, "Заработанная премия")
	assert.Contains(t, rendered, "lens-stat__label-row")
	// 8x8 accent swatch instead of icon-tile / accent-bar chrome
	assert.Contains(t, rendered, "lens-stat__swatch")
	assert.Contains(t, rendered, "background-color:#2563eb")
	assert.Contains(t, rendered, "lens-value-lg")
	assert.Contains(t, rendered, "1000.00")
	assert.NotContains(t, rendered, "h-10 w-10")
	assert.NotContains(t, rendered, "lens-stat--zero")
}

// A zero primary value demotes the card and suppresses trend + sparkline.
func TestStatPanel_ZeroValueDemotes(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("kpis",
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{0.0}},
	)
	require.NoError(t, err)
	set, err := frame.NewFrameSet(fr)
	require.NoError(t, err)

	spec := panel.Stat("kpi-zero", "Убытки", "kpis").
		Trend(12.5, "vs last month").
		Sparkline([]float64{1, 2, 3}).
		Build()
	result := &runtime.PanelResult{Panel: spec, Frames: set, Locale: "en"}

	var html bytes.Buffer
	err = StatPanel(spec, result, nil).Render(metricInfoContext(t, language.English), &html)
	require.NoError(t, err)

	rendered := html.String()
	assert.Contains(t, rendered, "lens-stat--zero")
	assert.NotContains(t, rendered, "lens-trend")
	assert.NotContains(t, rendered, "lens-spark")
}

// TrendWithInvert flips the good/bad color while the arrow follows the sign;
// the sparkline renders as a native polyline stroked with the accent token.
func TestStatPanel_TrendInvertAndSparkline(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("kpis",
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{87.5}},
	)
	require.NoError(t, err)
	set, err := frame.NewFrameSet(fr)
	require.NoError(t, err)

	spec := panel.Stat("kpi-loss-ratio", "Loss ratio", "kpis").
		TrendWithInvert(-5.0, "vs LY", true).
		Sparkline([]float64{4, 2, 6, 3}).
		Status("ON TRACK", panel.StatusWarning).
		Build()
	result := &runtime.PanelResult{Panel: spec, Frames: set, Locale: "en"}

	var html bytes.Buffer
	err = StatPanel(spec, result, nil).Render(metricInfoContext(t, language.English), &html)
	require.NoError(t, err)

	rendered := html.String()
	// down-is-good: negative percent renders the positive (up) color class
	assert.Contains(t, rendered, "lens-trend lens-trend--up")
	assert.Contains(t, rendered, "▼")
	assert.Contains(t, rendered, "-5.0")
	assert.Contains(t, rendered, "vs LY")
	// sparkline: native svg polyline, accent stroke by default
	assert.Contains(t, rendered, `class="lens-spark"`)
	assert.Contains(t, rendered, "<polyline")
	assert.Contains(t, rendered, "stroke:var(--lens-accent-500)")
	// status chip
	assert.Contains(t, rendered, "lens-chip lens-chip--warning")
	assert.Contains(t, rendered, "ON TRACK")
}

// StatGroup renders its children inside ONE card body, separated by hairline
// dividers, with each child resolving its own dataset result.
func TestStatGroupPanel_RendersChildrenInOneCard(t *testing.T) {
	t.Parallel()

	frA, err := frame.New("a",
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{100.0}},
	)
	require.NoError(t, err)
	setA, err := frame.NewFrameSet(frA)
	require.NoError(t, err)
	frB, err := frame.New("b",
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{250.0}},
	)
	require.NoError(t, err)
	setB, err := frame.NewFrameSet(frB)
	require.NoError(t, err)

	childA := panel.Stat("kpi-a", "Premium", "a").Build()
	childB := panel.Stat("kpi-b", "Claims", "b").Build()
	group := panel.StatGroup("kpi-group", "KPIs", childA, childB).Build()

	result := &runtime.Result{
		Spec: lens.DashboardSpec{Rows: []lens.RowSpec{{Panels: []panel.Spec{group}}}},
		Panels: map[string]*runtime.PanelResult{
			"kpi-a": {Panel: childA, Frames: setA, Locale: "en"},
			"kpi-b": {Panel: childB, Frames: setB, Locale: "en"},
		},
	}

	var html bytes.Buffer
	err = Panel(group, result, nil).Render(metricInfoContext(t, language.English), &html)
	require.NoError(t, err)

	rendered := html.String()
	// one card for the whole group (children carry no card chrome)
	assert.Equal(t, 1, strings.Count(rendered, "lens-card lens-card--overflow"))
	assert.Contains(t, rendered, "Premium")
	assert.Contains(t, rendered, "Claims")
	assert.Contains(t, rendered, "100.00")
	assert.Contains(t, rendered, "250.00")
	// columns layout: vertical hairline before the second child only
	assert.Equal(t, 1, strings.Count(rendered, "border-left:1px solid var(--lens-divider)"))
	assert.Contains(t, rendered, "repeat(auto-fit,minmax(180px,1fr))")
}

func TestStatGroupPanel_RowsLayoutUsesHorizontalDividers(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("a",
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{1.0}},
	)
	require.NoError(t, err)
	set, err := frame.NewFrameSet(fr)
	require.NoError(t, err)

	childA := panel.Stat("kpi-a", "A", "a").Build()
	childB := panel.Stat("kpi-b", "B", "a").Build()
	group := panel.StatGroup("kpi-group", "KPIs", childA, childB).Layout(panel.GroupRows).Build()

	result := &runtime.Result{
		Spec: lens.DashboardSpec{Rows: []lens.RowSpec{{Panels: []panel.Spec{group}}}},
		Panels: map[string]*runtime.PanelResult{
			"kpi-a": {Panel: childA, Frames: set, Locale: "en"},
			"kpi-b": {Panel: childB, Frames: set, Locale: "en"},
		},
	}

	var html bytes.Buffer
	err = StatGroupPanel(group, result, nil).Render(metricInfoContext(t, language.English), &html)
	require.NoError(t, err)

	rendered := html.String()
	assert.Contains(t, rendered, "flex-direction:column")
	assert.Equal(t, 1, strings.Count(rendered, "border-top:1px solid var(--lens-divider)"))
	assert.NotContains(t, rendered, "border-left:1px solid")
}

// A donut whose slices are all zero routes to the empty state instead of a
// blank white disc.
func TestChartPanel_AllZeroDonutRendersEmptyState(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("mix",
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"A", "B"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{0.0, 0.0}},
	)
	require.NoError(t, err)
	set, err := frame.NewFrameSet(fr)
	require.NoError(t, err)

	spec := panel.Donut("mix", "Mix", "mix").Build()
	result := &runtime.PanelResult{Panel: spec, Frames: set, Locale: "en"}

	var html bytes.Buffer
	err = ChartPanel(spec, result, nil).Render(metricInfoContext(t, language.English), &html)
	require.NoError(t, err)

	rendered := html.String()
	assert.Contains(t, rendered, "lens-empty")
	assert.Contains(t, rendered, "lens-empty__icon")
	assert.NotContains(t, rendered, "apexcharts")
}

func TestCascadePanel_RendersNativeBridgeRows(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("bridge",
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"Collected", "After commission", "Remainder"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{4850.0, 4230.0, 2890.0}},
		frame.Field{Name: "cut", Type: frame.FieldTypeNumber, Values: []any{0.0, 620.0, 1340.0}},
		frame.Field{Name: "cutLabel", Type: frame.FieldTypeString, Values: []any{"", "Commission", "Claims paid"}},
		frame.Field{Name: "final", Type: frame.FieldTypeBoolean, Values: []any{false, false, true}},
	)
	require.NoError(t, err)
	set, err := frame.NewFrameSet(fr)
	require.NoError(t, err)

	spec := panel.Cascade("cash-bridge", "Cash bridge", "bridge").
		Format(format.MoneyCompact("UZS")).
		Build()
	result := &runtime.PanelResult{Panel: spec, Frames: set, Locale: "en"}

	var html bytes.Buffer
	err = CascadePanel(spec, result, nil).Render(metricInfoContext(t, language.English), &html)
	require.NoError(t, err)

	rendered := html.String()
	assert.Contains(t, rendered, "Collected")
	assert.Contains(t, rendered, "After commission")
	// sign-aware cut connectors: a positive cut renders with a real minus sign
	// (U+2212) and red text, not the old "-> - Label:" arrow row.
	assert.Contains(t, rendered, "Commission</span>")
	assert.Contains(t, rendered, "−620.00 UZS")
	assert.Contains(t, rendered, "Claims paid</span>")
	assert.Contains(t, rendered, "−1.34K UZS")
	assert.NotContains(t, rendered, "-&gt;")
	// final stage uses the consumer-palette green, not emerald (absent there)
	assert.Contains(t, rendered, "bg-green-500")
	assert.NotContains(t, rendered, "bg-emerald-500")
	assert.Contains(t, rendered, "width:59.5876%")
	// top-aligned flex column, no vertical stretch/centering
	assert.Contains(t, rendered, "flex flex-col gap-1\"")
	assert.NotContains(t, rendered, "apexcharts")
	assert.NotContains(t, rendered, "<canvas")
}

func TestCascadePanel_RendersTrendChipInHeader(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("bridge",
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"Collected"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{4850.0}},
	)
	require.NoError(t, err)
	set, err := frame.NewFrameSet(fr)
	require.NoError(t, err)

	spec := panel.Cascade("cash-bridge", "Cash bridge", "bridge").
		Format(format.MoneyCompact("UZS")).
		Trend(-14.2, "vs last month").
		Build()

	result := &runtime.Result{
		Spec: lens.DashboardSpec{
			Rows: []lens.RowSpec{{Panels: []panel.Spec{spec}}},
		},
		Panels: map[string]*runtime.PanelResult{
			"cash-bridge": {Panel: spec, Frames: set, Locale: "en"},
		},
	}

	var html bytes.Buffer
	err = Panel(spec, result, nil).Render(metricInfoContext(t, language.English), &html)
	require.NoError(t, err)

	rendered := html.String()
	assert.Contains(t, rendered, "vs last month")
	assert.Contains(t, rendered, "-14.2%")
	assert.Contains(t, rendered, "▼")
	assert.Contains(t, rendered, "text-red-600")
}

func TestTablePanel_RendersBarAndDeltaCells(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("products",
		frame.Field{Name: "product", Type: frame.FieldTypeString, Values: []any{"OSAGO", "Kasko"}},
		frame.Field{Name: "result", Type: frame.FieldTypeNumber, Values: []any{1000.0, -500.0}},
		frame.Field{Name: "delta", Type: frame.FieldTypeNumber, Values: []any{200.0, -50.0}},
		frame.Field{Name: "delta_pct", Type: frame.FieldTypeNumber, Values: []any{20.0, -9.1}},
	)
	require.NoError(t, err)
	set, err := frame.NewFrameSet(fr)
	require.NoError(t, err)

	moneySpec := format.MoneyCompact("UZS")
	spec := panel.Table("bridge-products", "Products", "products").
		Columns(
			panel.TableColumn{Field: "product", Label: "Product"},
			panel.TableColumn{
				Field: "result", Label: "Result", Formatter: &moneySpec, Align: "right",
				Cell: &panel.TableCellSpec{Kind: panel.TableCellBar},
			},
			panel.TableColumn{
				Field: "delta", Label: "Delta", Formatter: &moneySpec, Align: "right",
				Cell: &panel.TableCellSpec{Kind: panel.TableCellDelta, PercentField: "delta_pct"},
			},
		).
		Build()

	result := &runtime.PanelResult{Panel: spec, Frames: set, Locale: "en"}

	var html bytes.Buffer
	err = TablePanel(spec, result, nil).Render(metricInfoContext(t, language.English), &html)
	require.NoError(t, err)

	rendered := html.String()
	// bar cell: value + a proportional fill scaled against the column max,
	// colored via the lens status vars
	assert.Contains(t, rendered, "width:100.0000%;background-color:var(--lens-pos)")
	assert.Contains(t, rendered, "width:50.0000%;background-color:var(--lens-neg)")
	assert.NotContains(t, rendered, "bg-green-500")
	// delta cell: signed percent as the primary line, signed amount beneath
	assert.Contains(t, rendered, "+20.0%")
	assert.Contains(t, rendered, "+200.00 UZS")
	assert.Contains(t, rendered, "−9.1%")
	assert.Contains(t, rendered, "−50.00 UZS")
	// lens table classes: header/body cells, numeric right-align, strong first column
	assert.Contains(t, rendered, "lens-th whitespace-nowrap lens-th--num")
	assert.Contains(t, rendered, "lens-td--num lens-num")
	assert.Contains(t, rendered, "lens-td--strong")
	assert.Contains(t, rendered, "lens-tr")
}

// A lens-table-sticky-first ClassName token passes through to the <table>,
// and a column Width renders as an inline min-width on its cells.
func TestTablePanel_StickyFirstTokenAndColumnWidth(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("rows",
		frame.Field{Name: "name", Type: frame.FieldTypeString, Values: []any{"OSAGO"}},
		frame.Field{Name: "total", Type: frame.FieldTypeNumber, Values: []any{10.0}},
	)
	require.NoError(t, err)
	set, err := frame.NewFrameSet(fr)
	require.NoError(t, err)

	spec := panel.Table("wide-table", "Wide", "rows").
		ClassName("lens-table-sticky-first").
		Columns(
			panel.TableColumn{Field: "name", Label: "Name"}.Width(160),
			panel.TableColumn{Field: "total", Label: "Total", Align: "right"},
		).
		Build()
	result := &runtime.PanelResult{Panel: spec, Frames: set, Locale: "en"}

	var html bytes.Buffer
	err = TablePanel(spec, result, nil).Render(metricInfoContext(t, language.English), &html)
	require.NoError(t, err)

	rendered := html.String()
	assert.Contains(t, rendered, `<table class="min-w-full w-full text-[13px] lens-table-sticky-first">`)
	assert.Contains(t, rendered, "min-width:160px")
}

// The skeleton mirrors the prepared layout: a heading band plus card-shaped
// shimmer per panel, all under one pulse.
func TestDashboardSkeleton_MirrorsLayout(t *testing.T) {
	t.Parallel()

	statSpec := panel.Stat("kpi", "KPI", "ds").AccentColor("#2563eb").Build()
	chartSpec := panel.Bar("bar", "Bar", "ds").Build()
	spec := lens.DashboardSpec{
		ID: "skeleton",
		Rows: []lens.RowSpec{
			{Heading: "Премии"},
			{Panels: []panel.Spec{statSpec, chartSpec}},
		},
	}

	var html bytes.Buffer
	err := DashboardSkeleton(spec).Render(metricInfoContext(t, language.English), &html)
	require.NoError(t, err)

	rendered := html.String()
	assert.Contains(t, rendered, "lens-skeleton-shimmer")
	// stat-shaped skeleton card (stat v2 geometry)
	assert.Contains(t, rendered, "min-height:96px")
	// chart/table-shaped skeleton block
	assert.Contains(t, rendered, "min-height:260px")
	// skeleton cards use the lens card chrome
	assert.Contains(t, rendered, "lens-card")
	// no live spinner
	assert.NotContains(t, rendered, "animate-spin")
}

func TestDashboardSkeleton_PreservesRowClass(t *testing.T) {
	spec := lens.DashboardSpec{
		Rows: []lens.RowSpec{{
			Class: "hidden lg:grid",
			Panels: []panel.Spec{panel.Stat("hidden-until-large", "Hidden", "ds").
				ClassName("invisible-panel").Build()},
		}},
	}

	var html strings.Builder
	err := DashboardSkeleton(spec).Render(metricInfoContext(t, language.English), &html)
	require.NoError(t, err)
	require.Contains(t, html.String(), "hidden lg:grid")
	require.Contains(t, html.String(), "invisible-panel")
}

// The skeleton mirrors a Tabs panel nesting a Grid of Cascade+Table children
// (the profitability bridge shape): one outer card with a pill-nav shimmer
// per tab, then the active tab's Grid rendered as its own bare child cards —
// not a single generic box for the whole composite.
func TestDashboardSkeleton_MirrorsNestedTabsGridComposite(t *testing.T) {
	t.Parallel()

	cascadeSpec := panel.Cascade("cash-cascade", "Cash bridge", "bridge").Span(4).Build()
	tableSpec := panel.Table("cash-table", "Products", "products").Span(8).Build()
	gridSpec := panel.Grid("cash-tab", "Cash", cascadeSpec, tableSpec).Build()
	gridSpec.Span = 12
	otherGridSpec := panel.Grid("underwriting-tab", "Underwriting", cascadeSpec, tableSpec).Build()
	otherGridSpec.Span = 12
	tabsSpec := panel.Tabs("bridges", "Мост премия → результат", gridSpec, otherGridSpec).Build()
	tabsSpec.Span = 12

	spec := lens.DashboardSpec{
		ID:   "skeleton-nested",
		Rows: []lens.RowSpec{{Panels: []panel.Spec{tabsSpec}}},
	}

	var html bytes.Buffer
	err := DashboardSkeleton(spec).Render(metricInfoContext(t, language.English), &html)
	require.NoError(t, err)

	rendered := html.String()
	// one pill shimmer per tab, not per nested leaf
	assert.Equal(t, 2, strings.Count(rendered, "lens-skeleton-shimmer h-7 flex-1 rounded-md"))
	// the active tab's Grid renders as a bare child grid (cascade card + table card)...
	assert.Contains(t, rendered, "min-height:88px")  // cascade-shaped card
	assert.Contains(t, rendered, "min-height:260px") // table-shaped card
	// ...nested inside the outer row grid (2 total: the dashboard row + the tab's Grid)
	assert.Equal(t, 2, strings.Count(rendered, `grid grid-cols-12 gap-3`))
}
