package templ

import (
	"bytes"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/format"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

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
	assert.Contains(t, rendered, "uppercase tracking-wider")
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
	assert.Contains(t, rendered, "Total")
	assert.Contains(t, rendered, "grid grid-cols-12")
}

// A Stat with an AccentColor but no Icon renders the icon-less accent chrome: a
// solid family-color bar, the value, and NO colored icon badge.
func TestStatPanel_AccentChromeWithoutIcon(t *testing.T) {
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
	// solid family-color accent bar (statAccentStyle)
	assert.Contains(t, rendered, "background-color: #2563eb;")
	// no translucent icon-badge chrome (badgeStyle uses rgba(...))
	assert.NotContains(t, rendered, "rgba(37, 101, 235")
	assert.NotContains(t, rendered, "h-10 w-10")
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
	// bar cell: value + a proportional fill scaled against the column max
	assert.Contains(t, rendered, "bg-green-500")
	assert.Contains(t, rendered, "bg-red-500")
	assert.Contains(t, rendered, "width:100.0000%")
	assert.Contains(t, rendered, "width:50.0000%")
	// delta cell: signed percent as the primary line, signed amount beneath
	assert.Contains(t, rendered, "+20.0%")
	assert.Contains(t, rendered, "+200.00 UZS")
	assert.Contains(t, rendered, "−9.1%")
	assert.Contains(t, rendered, "−50.00 UZS")
	// right-aligned header + cell
	assert.Contains(t, rendered, "text-right")
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
	// stat-shaped skeleton card (min height of the accent stat card)
	assert.Contains(t, rendered, "min-height:136px")
	// chart/table-shaped skeleton block
	assert.Contains(t, rendered, "min-height:260px")
	// no live spinner
	assert.NotContains(t, rendered, "animate-spin")
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
	assert.Equal(t, 2, strings.Count(rendered, "lens-skeleton-shimmer h-8 flex-1 rounded-xl"))
	// the active tab's Grid renders as a bare child grid (cascade card + table card)...
	assert.Contains(t, rendered, "min-height:88px")  // cascade-shaped card
	assert.Contains(t, rendered, "min-height:260px") // table-shaped card
	// ...nested inside the outer row grid (2 total: the dashboard row + the tab's Grid)
	assert.Equal(t, 2, strings.Count(rendered, "grid grid-cols-12 gap-3 md:gap-5"))
}
