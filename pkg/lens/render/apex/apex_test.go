package apex

import (
	"fmt"
	"testing"
	"time"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/components/charts"
	"github.com/iota-uz/iota-sdk/pkg/js"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	lenscolor "github.com/iota-uz/iota-sdk/pkg/lens/color"
	"github.com/iota-uz/iota-sdk/pkg/lens/format"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/iota-uz/iota-sdk/pkg/lens/theme"
	"github.com/stretchr/testify/require"
)

func TestBuildActionJSNormalizesTimeCategories(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("sales",
		frame.Field{Name: "category", Type: frame.FieldTypeTime, Values: []any{"2026-03-09T00:00:00Z"}},
		frame.Field{Name: "series", Type: frame.FieldTypeString, Values: []any{"Revenue"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{42.0}},
	)
	require.NoError(t, err)

	js := string(buildActionJS(
		&action.Spec{Kind: action.KindNavigate, URL: "/reports"},
		fr,
		panel.FieldMapping{Category: "category", Series: "series", Value: "value"},
		&runtime.PanelResult{},
	))

	require.Contains(t, js, "normalizeCategoryValue")
	require.Contains(t, js, "toISOString().slice(0, 10)")
}

func TestBuildActionJSPreservesTimeValuesInConfig(t *testing.T) {
	t.Parallel()

	timestamp := time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC)
	fr, err := frame.New("sales",
		frame.Field{Name: "category", Type: frame.FieldTypeTime, Values: []any{timestamp}},
		frame.Field{Name: "series", Type: frame.FieldTypeString, Values: []any{"Revenue"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{42.0}},
	)
	require.NoError(t, err)

	js := string(buildActionJS(
		&action.Spec{Kind: action.KindNavigate, URL: "/reports"},
		fr,
		panel.FieldMapping{Category: "category", Series: "series", Value: "value"},
		&runtime.PanelResult{Variables: map[string]any{"from": timestamp}},
	))

	require.Contains(t, js, `"2026-03-09T00:00:00Z"`)
}

func TestBuildActionJSHonorsFallbacks(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("sales",
		frame.Field{Name: "category", Type: frame.FieldTypeString, Values: []any{"March"}},
		frame.Field{Name: "series", Type: frame.FieldTypeString, Values: []any{"Revenue"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{42.0}},
	)
	require.NoError(t, err)

	js := string(buildActionJS(
		&action.Spec{
			Kind: action.KindNavigate,
			URL:  "/reports",
			Params: []action.Param{
				{
					Name: "product",
					Source: action.ValueSource{
						Kind:     action.SourceField,
						Name:     "product_id",
						Fallback: "default-product",
					},
				},
			},
			Payload: map[string]action.ValueSource{
				"active_only": {
					Kind:     action.SourceVariable,
					Name:     "active_only",
					Fallback: true,
				},
			},
		},
		fr,
		panel.FieldMapping{Category: "category", Series: "series", Value: "value"},
		&runtime.PanelResult{},
	))

	require.Contains(t, js, `resolveValue(row["product_id"], "default-product")`)
	require.Contains(t, js, `resolveValue(variables["active_only"], true)`)
}

func TestBuildActionJSEmitEventDoesNotRequireURL(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("premium-composition",
		frame.Field{Name: "segment", Type: frame.FieldTypeString, Values: []any{"Earned"}},
		frame.Field{Name: "metric", Type: frame.FieldTypeString, Values: []any{"earned_premium"}},
		frame.Field{Name: "amount", Type: frame.FieldTypeNumber, Values: []any{42.0}},
	)
	require.NoError(t, err)

	js := string(buildActionJS(
		&action.Spec{
			Kind:  action.KindEmitEvent,
			Event: "analytics:premium-year-breakdown",
			Payload: map[string]action.ValueSource{
				"metric": action.FieldValue("metric"),
			},
		},
		fr,
		panel.FieldMapping{Category: "segment", Value: "amount"},
		&runtime.PanelResult{},
	))

	require.NotContains(t, js, "if (!nextURL) { return; }")
	require.Contains(t, js, `document.dispatchEvent(new CustomEvent(cfg.event, {detail: payload}));`)
	require.Contains(t, js, `resolveValue(row["metric"], undefined)`)
}

func TestBuildActionJSCircularChartsUseSliceIndexForClickedRow(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("premium-composition",
		frame.Field{Name: "segment", Type: frame.FieldTypeString, Values: []any{"Earned", "Unearned"}},
		frame.Field{Name: "metric", Type: frame.FieldTypeString, Values: []any{"earned_premium", "unearned_premium"}},
		frame.Field{Name: "amount", Type: frame.FieldTypeNumber, Values: []any{42.0, 8.0}},
	)
	require.NoError(t, err)

	js := string(buildActionJS(
		&action.Spec{
			Kind:  action.KindEmitEvent,
			Event: "analytics:premium-year-breakdown",
			Payload: map[string]action.ValueSource{
				"metric": action.FieldValue("metric"),
			},
		},
		fr,
		panel.FieldMapping{Category: "segment", Value: "amount"},
		&runtime.PanelResult{},
	))

	require.Contains(t, js, `const isCircularChart = chartType === 'pie' || chartType === 'donut' || chartType === 'radialBar';`)
	require.Contains(t, js, `event.target.closest('.apexcharts-pie-area')`)
	require.Contains(t, js, `Number(sliceTarget.getAttribute('j'))`)
	require.Contains(t, js, `isCircularChart && Number.isInteger(sliceIndex)`)
	require.Contains(t, js, `let row = rows[rowIndex] || {};`)
}

func TestBuildActionJSResolvesFullURLFromClickedRow(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("costs",
		frame.Field{Name: "segment", Type: frame.FieldTypeString, Values: []any{"Acquisition"}},
		frame.Field{Name: "action_url", Type: frame.FieldTypeString, Values: []any{"/analytics/drill/acquisition_cost?token=signed-token"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{42.0}},
	)
	require.NoError(t, err)

	spec := action.Navigate("").WithFieldURL("action_url")
	js := string(buildActionJS(
		&spec,
		fr,
		panel.FieldMapping{Label: "segment", Value: "value"},
		&runtime.PanelResult{},
	))

	require.Contains(t, js, `resolveValue(row["action_url"], undefined)`)
	require.Contains(t, js, "const resolvedURL = safeActionURL")
	require.Contains(t, js, "if (!resolvedURL) { return; }")
	require.Contains(t, js, "window.location.href = nextURL")
}

func TestBuildActionJSResolvesFullURLFromClickedRowForHtmxSwap(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("costs",
		frame.Field{Name: "segment", Type: frame.FieldTypeString, Values: []any{"Acquisition"}},
		frame.Field{Name: "action_url", Type: frame.FieldTypeString, Values: []any{"/analytics/drill/acquisition_cost?token=signed-token"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{42.0}},
	)
	require.NoError(t, err)

	spec := action.HtmxSwap("", "#drawer").WithFieldURL("action_url")
	js := string(buildActionJS(
		&spec,
		fr,
		panel.FieldMapping{Label: "segment", Value: "value"},
		&runtime.PanelResult{},
	))

	require.Contains(t, js, `resolveValue(row["action_url"], undefined)`)
	require.Contains(t, js, "const resolvedURL = safeActionURL")
	require.Contains(t, js, "if (!resolvedURL) { return; }")
	require.Contains(t, js, "window.__lensDrillAjax(cfg.method || 'GET', nextURL, cfg.target, cfg.target)")
}

func TestBuildActionJSRejectsUnsafeURLSourceForNavigateAndHtmx(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("costs",
		frame.Field{Name: "action_url", Type: frame.FieldTypeString, Values: []any{"javascript:alert(1)"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{42.0}},
	)
	require.NoError(t, err)

	for _, spec := range []action.Spec{
		action.Navigate("").WithFieldURL("action_url"),
		action.HtmxSwap("", "#drawer").WithFieldURL("action_url"),
	} {
		js := string(buildActionJS(&spec, fr, panel.FieldMapping{Value: "value"}, &runtime.PanelResult{}))
		require.Contains(t, js, "const resolvedURL = safeActionURL")
		require.Contains(t, js, "raw.startsWith('//')")
		require.Contains(t, js, "parsed.origin !== window.location.origin")
		require.Contains(t, js, "if (!resolvedURL) { return; }")
	}
}

func TestBuildActionJSUsesHtmxSwapForCubeDrill(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("sales",
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"OSAGO"}},
		frame.Field{Name: "filter_value", Type: frame.FieldTypeString, Values: []any{"osago"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{42.0}},
	)
	require.NoError(t, err)

	js := string(buildActionJS(
		&action.Spec{
			Kind: action.KindCubeDrill,
			URL:  "/crm/reports/sales",
			Drill: &action.DrillSpec{
				Dimension: "product",
				Value:     action.FieldValue("filter_value"),
			},
		},
		fr,
		panel.FieldMapping{Label: "label", Category: "label", Value: "value", ID: "filter_value"},
		&runtime.PanelResult{},
	))

	require.Contains(t, js, "closest('[data-lens-swap-target]')")
	require.Contains(t, js, "target.dataset.lensDrillPending === 'true'")
	require.Contains(t, js, "document.addEventListener('htmx:afterRequest', clearPending)")
	require.Contains(t, js, "source.setAttribute('hx-push-url', 'true')")
	require.Contains(t, js, "return;")
	// Drill requests go through the scoped helper, which always passes an
	// explicit htmx source (never document.body). See DashboardScripts().
	require.Contains(t, js, "window.__lensDrillAjax(cfg.method || 'GET', nextURL, target, source)")
}

func TestOptionsResponsiveOverridesDoNotSerializeNilSeries(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("sales",
		frame.Field{Name: "category", Type: frame.FieldTypeString, Values: []any{"March", "April"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{42.0, 18.0}},
	)
	require.NoError(t, err)

	options := Options(
		panel.Bar("sales-by-month", "Sales by Month", "sales").
			CategoryField("category").
			ValueField("value").
			Build(),
		&runtime.PanelResult{Frames: mustFrameSet(t, fr)},
	)

	encoded, err := js.ToJS(options)
	require.NoError(t, err)
	require.Contains(t, encoded, "responsive")
	require.NotContains(t, encoded, "series: null")
	require.NotContains(t, encoded, "chart: {}")
}

func TestOptionsFallsBackToCategoryForPieLabels(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("sales",
		frame.Field{Name: "category", Type: frame.FieldTypeString, Values: []any{"OSAGO", "Travel"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{42.0, 18.0}},
	)
	require.NoError(t, err)

	options := Options(
		panel.Pie("sales-by-product", "Sales by Product", "sales").
			CategoryField("category").
			ValueField("value").
			Build(),
		&runtime.PanelResult{Frames: mustFrameSet(t, fr)},
	)

	require.Equal(t, []string{"OSAGO", "Travel"}, options.Labels)
}

func TestOptionsFallsBackToCategoryForUngroupedBarCategories(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("sales",
		frame.Field{Name: "category", Type: frame.FieldTypeString, Values: []any{"March", "April"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{42.0, 18.0}},
	)
	require.NoError(t, err)

	options := Options(
		panel.Bar("sales-by-month", "Sales by Month", "sales").
			CategoryField("category").
			ValueField("value").
			Build(),
		&runtime.PanelResult{Frames: mustFrameSet(t, fr)},
	)

	require.Equal(t, []string{"March", "April"}, options.XAxis.Categories)
}

func TestOptionsPanelEnhancements(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name        string
		panelSpec   panel.Spec
		panelResult *runtime.PanelResult
		height      string
		assertions  func(t *testing.T, options charts.ChartOptions)
	}

	regionFrame, err := frame.New("regions",
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"Tashkent", "Region"}},
		frame.Field{Name: "revenue", Type: frame.FieldTypeNumber, Values: []any{757_350_000.0, 1_250.0}},
	)
	require.NoError(t, err)

	liabilityFrame, err := frame.New("liability",
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"Travel", "OSAGO", "KASKO"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{68_800.0, 357_100_000_000.0, 472_900_000.0}},
	)
	require.NoError(t, err)

	productsFrame, err := frame.New("products",
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"TRAVEL", "OSAGO", "KASKO"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{10.0, 25.0, 5.0}},
	)
	require.NoError(t, err)

	heightFrame, err := frame.New("sales",
		frame.Field{Name: "category", Type: frame.FieldTypeString, Values: []any{"March"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{42.0}},
	)
	require.NoError(t, err)

	tests := []testCase{
		{
			name: "height override",
			panelSpec: panel.Bar("sales-by-month", "Sales by Month", "sales").
				CategoryField("category").
				ValueField("value").
				Height("360px").
				Build(),
			panelResult: &runtime.PanelResult{Frames: mustFrameSet(t, heightFrame)},
			height:      "100%",
			assertions: func(t *testing.T, options charts.ChartOptions) {
				t.Helper()
				require.Equal(t, "100%", options.Chart.Height)
			},
		},
		{
			name: "logarithmic horizontal bar",
			panelSpec: panel.HorizontalBar("revenue-by-region", "Revenue by Region", "regions").
				LabelField("label").
				ValueField("revenue").
				Format(format.MoneyCompact("UZS")).
				LogarithmicValueAxis(10).
				Build(),
			panelResult: &runtime.PanelResult{Frames: mustFrameSet(t, regionFrame), Locale: "ru"},
			assertions: func(t *testing.T, options charts.ChartOptions) {
				t.Helper()
				series, ok := options.Series.([]charts.Series)
				require.True(t, ok)
				require.Len(t, series, 1)
				require.Equal(t, []string{"Tashkent", "Region"}, options.XAxis.Categories)
				require.IsType(t, float64(0), series[0].Data[0])
				require.IsType(t, float64(0), series[0].Data[1])
				require.Greater(t, series[0].Data[0].(float64), series[0].Data[1].(float64))
				require.NotNil(t, options.XAxis.Min)
				require.NotNil(t, options.XAxis.Max)
				require.NotNil(t, options.XAxis.StepSize)
				require.InDelta(t, 3.0, *options.XAxis.Min, 1e-9)
				require.InDelta(t, 9.0, *options.XAxis.Max, 1e-9)
				require.InDelta(t, 2.0, *options.XAxis.StepSize, 1e-9)
				require.NotEmpty(t, options.XAxis.Labels.Formatter)
				require.NotNil(t, options.Tooltip)
				require.NotNil(t, options.Tooltip.Y)
				tooltipY, ok := options.Tooltip.Y.(*charts.TooltipYConfig)
				require.True(t, ok)
				require.NotEmpty(t, tooltipY.Formatter)
			},
		},
		{
			name: "smart logarithmic vertical bar",
			panelSpec: panel.Bar("liability-by-type", "Liability by Type", "liability").
				LabelField("label").
				ValueField("value").
				Format(format.MoneyCompact("UZS")).
				LogarithmicValueAxis(10).
				Build(),
			panelResult: &runtime.PanelResult{Frames: mustFrameSet(t, liabilityFrame), Locale: "ru"},
			assertions: func(t *testing.T, options charts.ChartOptions) {
				t.Helper()
				series, ok := options.Series.([]charts.Series)
				require.True(t, ok)
				require.Len(t, series, 1)
				require.Len(t, options.YAxis, 1)
				require.Nil(t, options.YAxis[0].Logarithmic)
				require.InDelta(t, 4.0, options.YAxis[0].Min, 1e-9)
				require.InDelta(t, 12.0, options.YAxis[0].Max, 1e-9)
				require.NotNil(t, options.YAxis[0].StepSize)
				require.InDelta(t, 2.0, *options.YAxis[0].StepSize, 1e-9)
				require.NotNil(t, options.YAxis[0].TickAmount)
				require.Equal(t, 4, *options.YAxis[0].TickAmount)
				require.NotNil(t, options.SDK)
				require.NotNil(t, options.SDK.ManualLogScale)
				require.Equal(t, 10, options.SDK.ManualLogScale.Base)
				require.False(t, options.SDK.ManualLogScale.Horizontal)
				require.NotNil(t, options.YAxis[0].Labels)
				require.NotEmpty(t, options.YAxis[0].Labels.Formatter)
				require.NotNil(t, options.Tooltip)
				require.NotNil(t, options.Tooltip.Y)
				tooltipY, ok := options.Tooltip.Y.(*charts.TooltipYConfig)
				require.True(t, ok)
				require.NotEmpty(t, tooltipY.Formatter)
			},
		},
		{
			name: "distributed colors",
			panelSpec: panel.Bar("premium-by-type", "Average Premium", "products").
				LabelField("label").
				ValueField("value").
				Colors("#3B82F6", "#10B981", "#EF4444").
				DistributedColors().
				Build(),
			panelResult: &runtime.PanelResult{Frames: mustFrameSet(t, productsFrame)},
			assertions: func(t *testing.T, options charts.ChartOptions) {
				t.Helper()
				require.NotNil(t, options.PlotOptions)
				require.NotNil(t, options.PlotOptions.Bar)
				require.NotNil(t, options.PlotOptions.Bar.Distributed)
				require.True(t, *options.PlotOptions.Bar.Distributed)
				require.Equal(t, []string{"#3B82F6", "#10B981", "#EF4444"}, options.Colors)
				require.NotNil(t, options.Chart.Events)
				require.NotEmpty(t, options.Chart.Events.DataPointMouseEnter)
			},
		},
		{
			name: "dense category labels hide legend and truncate",
			panelSpec: panel.HorizontalBar("agency", "Agency", "agency").
				LabelField("label").
				ValueField("value").
				Colors("#3B82F6", "#10B981").
				DistributedColors().
				Build(),
			panelResult: func() *runtime.PanelResult {
				fr, err := frame.New("agency",
					frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"Very Long Agency Name That Needs Truncation", "Another Long Agency Name"}},
					frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{42.0, 18.0}},
				)
				require.NoError(t, err)
				return &runtime.PanelResult{Frames: mustFrameSet(t, fr)}
			}(),
			assertions: func(t *testing.T, options charts.ChartOptions) {
				t.Helper()
				require.NotNil(t, options.Legend)
				require.NotNil(t, options.Legend.Show)
				require.False(t, *options.Legend.Show)
				require.Len(t, options.YAxis, 1)
				require.NotNil(t, options.YAxis[0].Labels)
				require.NotEmpty(t, options.YAxis[0].Labels.Formatter)
				require.NotNil(t, options.YAxis[0].Labels.MaxWidth)
			},
		},
		{
			name: "semantic colors deduplicate grouped series",
			panelSpec: panel.StackedBar("sales-by-product", "Sales by Product", "sales").
				CategoryField("category").
				SeriesField("series").
				ValueField("value").
				SemanticColors("PRODUCT", panel.Ref("color_value")).
				Build(),
			panelResult: func() *runtime.PanelResult {
				fr, err := frame.New("sales",
					frame.Field{Name: "category", Type: frame.FieldTypeString, Values: []any{"Jan", "Jan", "Feb", "Feb"}},
					frame.Field{Name: "series", Type: frame.FieldTypeString, Values: []any{"OSAGO", "TRAVEL", "OSAGO", "TRAVEL"}},
					frame.Field{Name: "color_value", Type: frame.FieldTypeString, Values: []any{"osago", "travel", "osago", "travel"}},
					frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{10.0, 5.0, 12.0, 8.0}},
				)
				require.NoError(t, err)
				return &runtime.PanelResult{Frames: mustFrameSet(t, fr)}
			}(),
			assertions: func(t *testing.T, options charts.ChartOptions) {
				t.Helper()
				require.Len(t, options.Colors, 2)
				require.Equal(t, []string{"#7C3AED", "#2563EB"}, options.Colors)
			},
		},
		{
			name: "stacked bar tooltip includes localized total",
			panelSpec: panel.StackedBar("sales-by-product", "Sales by Product", "sales").
				CategoryField("category").
				SeriesField("series").
				ValueField("value").
				Format(format.MoneyCompact("UZS")).
				Build(),
			panelResult: func() *runtime.PanelResult {
				fr, err := frame.New("sales",
					frame.Field{Name: "category", Type: frame.FieldTypeString, Values: []any{"Jan", "Jan"}},
					frame.Field{Name: "series", Type: frame.FieldTypeString, Values: []any{"OSAGO", "TRAVEL"}},
					frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{10.0, 5.0}},
				)
				require.NoError(t, err)
				return &runtime.PanelResult{Frames: mustFrameSet(t, fr), Locale: "ru"}
			}(),
			assertions: func(t *testing.T, options charts.ChartOptions) {
				t.Helper()
				require.NotNil(t, options.Tooltip)
				require.NotEmpty(t, options.Tooltip.Custom)
				custom := string(options.Tooltip.Custom.(templ.JSExpression))
				require.Contains(t, custom, "Итого")
				// Light-theme totals divider + strong text, not the old
				// white-on-dark hairline.
				require.Contains(t, custom, "rgba(15,23,42,0.08)")
				require.NotContains(t, custom, "rgba(255,255,255,0.18)")
				require.Contains(t, custom, theme.TextStrong)
				require.Contains(t, custom, "collapsedSeriesIndices")
				require.Contains(t, custom, "hiddenSeriesIndices")
				require.Contains(t, custom, "hiddenSeriesNames")
				require.Contains(t, custom, "number === 0")
				require.Contains(t, custom, "formatValue(total, -1)")
				require.NotNil(t, options.Chart.Events)
				require.NotEmpty(t, options.Chart.Events.Mounted)
				require.NotEmpty(t, options.Chart.Events.Updated)
				require.NotEmpty(t, options.Chart.Events.LegendClick)
				badge := string(options.Chart.Events.LegendClick)
				require.Contains(t, badge, "data-lens-stacked-total")
				require.Contains(t, badge, "ctx.isSeriesHidden")
				require.Contains(t, badge, "ctx.series.isSeriesHidden")
				require.Contains(t, badge, "collapsedSeriesIndices")
				require.Contains(t, badge, "hiddenSeriesIndices")
				require.NotContains(t, badge, "collapsedSeries || []")
				require.NotContains(t, badge, "hiddenSeries || []")
				require.Contains(t, badge, "badge.textContent = totalLabel + ': ' + formatValue(total)")
				require.Contains(t, badge, `const staticTotalText = "";`)
			},
		},
		{
			name: "grouped bar with server total renders static badge",
			panelSpec: panel.Bar("premium-by-year", "Premium by Year", "premium").
				CategoryField("category").
				SeriesField("series").
				ValueField("value").
				Format(format.MoneyCompact("UZS")).
				TotalBadgeValue(393069670322.96).
				Build(),
			panelResult: func() *runtime.PanelResult {
				fr, err := frame.New("premium",
					frame.Field{Name: "category", Type: frame.FieldTypeString, Values: []any{"2024", "2024"}},
					frame.Field{Name: "series", Type: frame.FieldTypeString, Values: []any{"OSAGO", "TRAVEL"}},
					frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{10.0, 5.0}},
				)
				require.NoError(t, err)
				return &runtime.PanelResult{Frames: mustFrameSet(t, fr), Locale: "ru"}
			}(),
			assertions: func(t *testing.T, options charts.ChartOptions) {
				t.Helper()
				require.NotNil(t, options.Chart.Events)
				require.NotEmpty(t, options.Chart.Events.Mounted)
				badge := string(options.Chart.Events.Mounted)
				require.Contains(t, badge, "data-lens-stacked-total")
				// The total is formatted server-side with the panel's own
				// formatter — the client-side sum/formatter operate in
				// (log-transformed) plot space and must not be used.
				spec := format.MoneyCompact("UZS")
				expected := format.Apply(&spec, 393069670322.96, "ru", "")
				require.NotEmpty(t, expected)
				require.Contains(t, badge, fmt.Sprintf("const staticTotalText = %q;", expected))
				require.Contains(t, badge, "badge.textContent = totalLabel + ': ' + staticTotalText")
			},
		},
		{
			name: "apex theme tokens on axes grid and tooltip",
			panelSpec: panel.Bar("sales-by-month", "Sales by Month", "sales").
				CategoryField("category").
				ValueField("value").
				Build(),
			panelResult: &runtime.PanelResult{Frames: mustFrameSet(t, heightFrame)},
			assertions: func(t *testing.T, options charts.ChartOptions) {
				t.Helper()
				require.NotNil(t, options.Tooltip)
				require.NotNil(t, options.Tooltip.Theme)
				require.Equal(t, "light", *options.Tooltip.Theme)
				require.NotNil(t, options.Tooltip.CSSClass)
				require.Equal(t, "lens-tooltip", *options.Tooltip.CSSClass)
				require.NotNil(t, options.Tooltip.FillSeriesColor)
				require.False(t, *options.Tooltip.FillSeriesColor)
				require.NotNil(t, options.Grid)
				require.Equal(t, theme.Divider, options.Grid.BorderColor)
				require.NotNil(t, options.Grid.XAxis)
				require.NotNil(t, options.Grid.XAxis.Lines)
				require.False(t, *options.Grid.XAxis.Lines.Show)
				require.NotNil(t, options.Grid.YAxis)
				require.NotNil(t, options.Grid.YAxis.Lines)
				require.True(t, *options.Grid.YAxis.Lines.Show)
				require.NotNil(t, options.XAxis.Labels)
				require.NotNil(t, options.XAxis.Labels.Style)
				require.Equal(t, theme.TextMuted, options.XAxis.Labels.Style.Colors)
				require.NotNil(t, options.XAxis.Labels.Style.CSSClass)
				require.Equal(t, "lens-num", *options.XAxis.Labels.Style.CSSClass)
				require.Len(t, options.YAxis, 1)
				require.NotNil(t, options.YAxis[0].Labels)
				require.NotNil(t, options.YAxis[0].Labels.Style)
				require.Equal(t, theme.TextMuted, options.YAxis[0].Labels.Style.Colors)
				require.NotNil(t, options.YAxis[0].Labels.Style.CSSClass)
				require.Equal(t, "lens-num", *options.YAxis[0].Labels.Style.CSSClass)
				require.NotNil(t, options.PlotOptions)
				require.NotNil(t, options.PlotOptions.Bar)
				require.Equal(t, 3, options.PlotOptions.Bar.BorderRadius)
				require.NotNil(t, options.PlotOptions.Bar.BorderRadiusApplication)
				require.Equal(t, "end", *options.PlotOptions.Bar.BorderRadiusApplication)
				// One category → sparse layout keeps bars slim.
				require.Equal(t, "32%", options.PlotOptions.Bar.ColumnWidth)
			},
		},
		{
			name: "adaptive column width widens with category count",
			panelSpec: panel.Bar("monthly", "Monthly", "monthly").
				CategoryField("category").
				ValueField("value").
				Build(),
			panelResult: func() *runtime.PanelResult {
				categories := make([]any, 13)
				values := make([]any, 13)
				for i := range categories {
					categories[i] = fmt.Sprintf("2024-%02d", i+1)
					values[i] = float64(i + 1)
				}
				fr, err := frame.New("monthly",
					frame.Field{Name: "category", Type: frame.FieldTypeString, Values: categories},
					frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: values},
				)
				require.NoError(t, err)
				return &runtime.PanelResult{Frames: mustFrameSet(t, fr)}
			}(),
			assertions: func(t *testing.T, options charts.ChartOptions) {
				t.Helper()
				require.NotNil(t, options.PlotOptions)
				require.NotNil(t, options.PlotOptions.Bar)
				require.Equal(t, "62%", options.PlotOptions.Bar.ColumnWidth)
				require.Equal(t, "48%", adaptiveColumnWidth(6))
			},
		},
		{
			name: "horizontal bar geometry and gridline inversion",
			panelSpec: panel.HorizontalBar("by-region", "By Region", "regions").
				LabelField("label").
				ValueField("revenue").
				Build(),
			panelResult: &runtime.PanelResult{Frames: mustFrameSet(t, regionFrame)},
			assertions: func(t *testing.T, options charts.ChartOptions) {
				t.Helper()
				require.NotNil(t, options.PlotOptions)
				require.NotNil(t, options.PlotOptions.Bar)
				require.NotNil(t, options.PlotOptions.Bar.BarHeight)
				require.Equal(t, "55%", *options.PlotOptions.Bar.BarHeight)
				require.NotNil(t, options.PlotOptions.Bar.BorderRadiusApplication)
				require.Equal(t, "end", *options.PlotOptions.Bar.BorderRadiusApplication)
				// Horizontal bars invert gridlines: vertical lines only.
				require.NotNil(t, options.Grid)
				require.True(t, *options.Grid.XAxis.Lines.Show)
				require.False(t, *options.Grid.YAxis.Lines.Show)
			},
		},
		{
			name: "time series uses straight solid strokes",
			panelSpec: panel.TimeSeries("trend", "Trend", "sales").
				CategoryField("category").
				ValueField("value").
				Build(),
			panelResult: &runtime.PanelResult{Frames: mustFrameSet(t, heightFrame)},
			assertions: func(t *testing.T, options charts.ChartOptions) {
				t.Helper()
				require.NotNil(t, options.Stroke)
				require.Equal(t, charts.StrokeCurveStraight, options.Stroke.Curve)
				require.Equal(t, 2, options.Stroke.Width)
				require.NotNil(t, options.Fill)
				require.Equal(t, "solid", options.Fill.Type)
				require.Nil(t, options.Fill.Gradient)
			},
		},
		{
			name: "donut renders center total labels and rich legend",
			panelSpec: panel.Donut("by-product", "By Product", "products").
				LabelField("label").
				ValueField("value").
				Format(format.MoneyCompact("UZS")).
				Build(),
			panelResult: &runtime.PanelResult{Frames: mustFrameSet(t, productsFrame), Locale: "ru"},
			assertions: func(t *testing.T, options charts.ChartOptions) {
				t.Helper()
				require.NotNil(t, options.PlotOptions)
				require.NotNil(t, options.PlotOptions.Pie)
				require.NotNil(t, options.PlotOptions.Pie.Donut)
				require.NotNil(t, options.PlotOptions.Pie.Donut.Size)
				require.Equal(t, "78%", *options.PlotOptions.Pie.Donut.Size)
				labels := options.PlotOptions.Pie.Donut.Labels
				require.NotNil(t, labels)
				require.True(t, *labels.Show)
				require.NotNil(t, labels.Total)
				require.True(t, *labels.Total.Show)
				require.Equal(t, "Итого", *labels.Total.Label)
				require.Contains(t, string(labels.Total.Formatter), "seriesTotals")
				require.NotNil(t, labels.Value)
				require.NotEmpty(t, labels.Value.Formatter)
				// White hairline separates slices on the light surface.
				require.NotNil(t, options.Stroke)
				require.Equal(t, []string{theme.BgCard}, options.Stroke.Colors)
				require.Equal(t, 2, options.Stroke.Width)
				// Percentages live on slices; the legend carries exact values only.
				require.NotNil(t, options.DataLabels)
				require.True(t, options.DataLabels.Enabled)
				require.NotEmpty(t, options.DataLabels.Formatter)
				dataLabelJS := string(options.DataLabels.Formatter)
				require.Contains(t, dataLabelJS, "maximumFractionDigits: 1")
				require.Contains(t, dataLabelJS, "+ '%'")
				require.NotNil(t, options.DataLabels.Style)
				require.Equal(t, []string{theme.BgCard}, options.DataLabels.Style.Colors)
				require.NotNil(t, options.DataLabels.DropShadow)
				require.True(t, options.DataLabels.DropShadow.Enabled)
				require.NotNil(t, options.Legend)
				require.NotNil(t, options.Legend.Position)
				require.Equal(t, charts.LegendPositionBottom, *options.Legend.Position)
				require.NotNil(t, options.Legend.Markers)
				require.Equal(t, "square", *options.Legend.Markers.Shape)
				require.NotNil(t, options.Legend.OnItemClick)
				require.True(t, *options.Legend.OnItemClick.ToggleDataSeries)
				require.NotEmpty(t, options.Legend.Formatter)
				legendJS := string(options.Legend.Formatter)
				require.Contains(t, legendJS, "seriesName + ' · ' + formatted")
				require.NotContains(t, legendJS, "toFixed")
				require.NotContains(t, legendJS, "pct")
				totalJS := string(labels.Total.Formatter)
				require.Contains(t, totalJS, "collapsedSeriesIndices")
				require.Contains(t, totalJS, "hiddenIndices.has(index)")
			},
		},
		{
			name: "money pie renders total badge",
			panelSpec: panel.Pie("premium-mix", "Premium Mix", "products").
				LabelField("label").
				ValueField("value").
				Format(format.MoneyCompact("UZS")).
				Build(),
			panelResult: &runtime.PanelResult{Frames: mustFrameSet(t, productsFrame), Locale: "ru"},
			assertions: func(t *testing.T, options charts.ChartOptions) {
				t.Helper()
				require.NotNil(t, options.Chart.Events)
				require.NotEmpty(t, options.Chart.Events.Mounted)
				badge := string(options.Chart.Events.Mounted)
				require.Contains(t, badge, "data-lens-stacked-total")
				require.Contains(t, badge, `const totalLabel = "Итого";`)
				require.Contains(t, badge, "collapsedSeriesIndices")
				require.Contains(t, badge, "!hiddenSeriesIndices.has(seriesIndex)")
				require.Contains(t, badge, "const isFlatSeries = configSeries.length > 0")
				require.Contains(t, badge, "configSeries.forEach((value, seriesIndex)")
				require.NotNil(t, options.Grid)
				require.NotNil(t, options.Grid.Padding)
				require.Equal(t, 34, *options.Grid.Padding.Top)
			},
		},
		{
			name: "percentage pie omits additive total",
			panelSpec: panel.Pie("share-mix", "Share Mix", "products").
				LabelField("label").
				ValueField("value").
				Format(format.Percent(1)).
				Build(),
			panelResult: &runtime.PanelResult{Frames: mustFrameSet(t, productsFrame), Locale: "ru"},
			assertions: func(t *testing.T, options charts.ChartOptions) {
				t.Helper()
				require.Nil(t, options.Chart.Events)
				require.NotNil(t, options.Grid)
				require.NotNil(t, options.Grid.Padding)
				require.Equal(t, 4, *options.Grid.Padding.Top)
			},
		},
		{
			name: "percentage donut omits center total",
			panelSpec: panel.Donut("share-donut", "Share Donut", "products").
				LabelField("label").
				ValueField("value").
				Format(format.Percent(1)).
				Build(),
			panelResult: &runtime.PanelResult{Frames: mustFrameSet(t, productsFrame), Locale: "ru"},
			assertions: func(t *testing.T, options charts.ChartOptions) {
				t.Helper()
				require.NotNil(t, options.PlotOptions)
				require.NotNil(t, options.PlotOptions.Pie)
				require.NotNil(t, options.PlotOptions.Pie.Donut)
				require.Nil(t, options.PlotOptions.Pie.Donut.Labels)
			},
		},
		{
			name: "truncate formatter keeps output within limit",
			panelSpec: panel.Bar("formatter-check", "Formatter Check", "sales").
				CategoryField("category").
				ValueField("value").
				Build(),
			panelResult: &runtime.PanelResult{Frames: mustFrameSet(t, heightFrame)},
			assertions: func(t *testing.T, options charts.ChartOptions) {
				t.Helper()
				formatter := string(truncateCategoryLabelFormatter(16))
				require.Contains(t, formatter, "text.slice(0, 13) + '...';")
				require.NotContains(t, formatter, "if (16 <= 3)")

				shortFormatter := string(truncateCategoryLabelFormatter(3))
				require.Contains(t, shortFormatter, "return text.slice(0, 3);")
				require.NotContains(t, shortFormatter, "text.slice(0, 0) + '...';")
				require.NotNil(t, options)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			options := Options(tc.panelSpec, tc.panelResult)
			if tc.height != "" {
				options = OptionsWithHeight(tc.panelSpec, tc.panelResult, tc.height)
			}

			tc.assertions(t, options)
		})
	}
}

func TestBuildLogarithmicAxisPlanCapsTopAtHalfDecade(t *testing.T) {
	t.Parallel()

	// 575M..256B spans exponents 8.76..11.41: rounding the top up to a full
	// decade (1T) would waste most of a decade above the tallest bar, so the
	// plan caps at the next half-decade (11.5 ≈ 316B) with half-decade grid
	// steps. Labels stay on whole decades via the formatter's slot check.
	plan, ok := buildLogarithmicAxisPlan([]charts.Series{
		{Name: "Direct", Data: []any{5.75e8, 4.7e10}},
		{Name: "Inward", Data: []any{9.1e9, 2.56e11}},
	}, 10)
	require.True(t, ok)
	require.InDelta(t, 8.0, plan.MinExponent, 1e-9)
	require.InDelta(t, 11.5, plan.MaxExponent, 1e-9)
	require.InDelta(t, 0.5, plan.Step, 1e-9)
	require.Equal(t, 7, plan.TickAmount)
}

func TestBuildLogarithmicAxisPlanKeepsFullDecadeWhenMaxIsNearIt(t *testing.T) {
	t.Parallel()

	// 9.9e11 is within 0.04 exponent of the decade boundary — the half-decade
	// cap would leave the tallest bar touching the axis top, so the plan keeps
	// the classic full-decade ceiling.
	plan, ok := buildLogarithmicAxisPlan([]charts.Series{
		{Name: "Direct", Data: []any{5.75e8, 9.9e11}},
	}, 10)
	require.True(t, ok)
	require.InDelta(t, 8.0, plan.MinExponent, 1e-9)
	require.InDelta(t, 12.0, plan.MaxExponent, 1e-9)
	require.InDelta(t, 1.0, plan.Step, 1e-9)
	require.Equal(t, 4, plan.TickAmount)
}

func TestLogarithmicAxisPlanFromAxisOptionsUsesAxisConfig(t *testing.T) {
	t.Parallel()

	minValue := 3.0
	maxValue := 9.0
	step := 2.0
	options := charts.ChartOptions{
		XAxis: charts.XAxisConfig{
			Min:      &minValue,
			Max:      &maxValue,
			StepSize: &step,
		},
		Series: []charts.Series{
			{
				Name: "Revenue",
				Data: []any{0.0, 1.0, 2.0},
			},
		},
	}

	plan, ok := logarithmicAxisPlanFromAxisOptions(options, panel.KindHorizontalBar, 10)
	require.True(t, ok)
	require.Equal(t, 10, plan.Base)
	require.InDelta(t, 3.0, plan.MinExponent, 1e-9)
	require.InDelta(t, 9.0, plan.MaxExponent, 1e-9)
	require.InDelta(t, 2.0, plan.Step, 1e-9)
}

func TestLogarithmicAxisFormatterDoesNotBakeInitialAxisBounds(t *testing.T) {
	t.Parallel()

	formatter := wrapLogarithmicAxisFormatter("", "ru", logarithmicAxisPlan{
		Base:        10,
		MinExponent: 5,
		MaxExponent: 11,
		Step:        2,
		TickAmount:  3,
	})

	js := string(formatter)
	require.Contains(t, js, "Math.abs(scaled - Math.round(scaled))")
	require.NotContains(t, js, "minExponent", "legend-driven replanning must not retain the initial axis bounds")
}

func TestOptionsDoesNotWrapLogFormattersWhenManualScaleIsSkipped(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("sales",
		frame.Field{Name: "category", Type: frame.FieldTypeString, Values: []any{"March", "April", "May"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{0.0, 100.0, 1000.0}},
	)
	require.NoError(t, err)

	options := Options(
		panel.Bar("sales-by-month", "Sales by Month", "sales").
			CategoryField("category").
			ValueField("value").
			Format(format.MoneyCompact("UZS")).
			LogarithmicValueAxis(10).
			Build(),
		&runtime.PanelResult{Frames: mustFrameSet(t, fr), Locale: "ru", Timezone: "Asia/Tashkent"},
	)

	require.NotNil(t, options.Tooltip)
	tooltipY, ok := options.Tooltip.Y.(*charts.TooltipYConfig)
	require.True(t, ok)
	require.NotNil(t, tooltipY)
	require.NotContains(t, string(tooltipY.Formatter), "Math.pow")
	require.Len(t, options.YAxis, 1)
	require.NotNil(t, options.YAxis[0].Labels)
	require.NotContains(t, string(options.YAxis[0].Labels.Formatter), "Math.pow")
}

func TestOptionsWithHeightEnablesFullscreenToolbar(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("sales",
		frame.Field{Name: "category", Type: frame.FieldTypeString, Values: []any{"March"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{42.0}},
	)
	require.NoError(t, err)

	panelSpec := panel.Bar("sales-by-month", "Sales by Month", "sales").
		CategoryField("category").
		ValueField("value").
		Build()

	inlineOptions := Options(panelSpec, &runtime.PanelResult{Frames: mustFrameSet(t, fr)})
	fullscreenOptions := OptionsWithHeight(panelSpec, &runtime.PanelResult{Frames: mustFrameSet(t, fr)}, "100%")

	require.False(t, inlineOptions.Chart.Toolbar.Show)
	require.True(t, fullscreenOptions.Chart.Toolbar.Show)
	require.Equal(t, "100%", fullscreenOptions.Chart.Height)
	require.NotNil(t, fullscreenOptions.Chart.Toolbar.Tools)
	require.NotNil(t, fullscreenOptions.Chart.Toolbar.Tools.Download)
	require.True(t, *fullscreenOptions.Chart.Toolbar.Tools.Download)
	require.NotNil(t, fullscreenOptions.Chart.Toolbar.Tools.Zoom)
	require.True(t, *fullscreenOptions.Chart.Toolbar.Tools.Zoom)
	require.NotNil(t, fullscreenOptions.Chart.Toolbar.Tools.ZoomIn)
	require.True(t, *fullscreenOptions.Chart.Toolbar.Tools.ZoomIn)
	require.NotNil(t, fullscreenOptions.Chart.Toolbar.Tools.ZoomOut)
	require.True(t, *fullscreenOptions.Chart.Toolbar.Tools.ZoomOut)
	require.NotNil(t, fullscreenOptions.Chart.Toolbar.Tools.Pan)
	require.True(t, *fullscreenOptions.Chart.Toolbar.Tools.Pan)
	require.NotNil(t, fullscreenOptions.Chart.Toolbar.Tools.Reset)
	require.True(t, *fullscreenOptions.Chart.Toolbar.Tools.Reset)
	require.NotNil(t, fullscreenOptions.Chart.Toolbar.Tools.Selection)
	require.False(t, *fullscreenOptions.Chart.Toolbar.Tools.Selection)
	require.NotNil(t, fullscreenOptions.Chart.Toolbar.AutoSelected)
	require.Equal(t, "zoom", *fullscreenOptions.Chart.Toolbar.AutoSelected)
	require.NotNil(t, fullscreenOptions.Chart.Zoom)
	require.NotNil(t, fullscreenOptions.Chart.Zoom.Enabled)
	require.True(t, *fullscreenOptions.Chart.Zoom.Enabled)
	require.NotNil(t, fullscreenOptions.Chart.Zoom.Type)
	require.Equal(t, "x", *fullscreenOptions.Chart.Zoom.Type)
}

func TestOptionsSingleSeriesBarStaysAccentColored(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("vehicle-types",
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"Sedan", "SUV", "Truck"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{42.0, 30.0, 18.0}},
	)
	require.NoError(t, err)

	options := Options(
		panel.HorizontalBar("vehicle-type", "Vehicle Type", "vehicle-types").
			LabelField("label").
			ValueField("value").
			Build(),
		&runtime.PanelResult{Frames: mustFrameSet(t, fr)},
	)

	// No explicit Distributed flag and no semantic scale: single-series bars
	// keep one color (the Lens accent), never an implicit per-row rainbow.
	require.NotNil(t, options.PlotOptions)
	require.NotNil(t, options.PlotOptions.Bar)
	require.Nil(t, options.PlotOptions.Bar.Distributed)
	require.Equal(t, []string{lenscolor.Accent()}, options.Colors)
	if options.Chart.Events != nil {
		require.Empty(t, options.Chart.Events.DataPointMouseEnter)
	}
	require.NotNil(t, options.Grid)
	require.NotNil(t, options.Grid.Padding)
	require.NotNil(t, options.Grid.Padding.Right)
	require.GreaterOrEqual(t, *options.Grid.Padding.Right, 40)
}

func TestOptionsSemanticScaleDistributesBarColors(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("products",
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"OSAGO", "TRAVEL"}},
		frame.Field{Name: "color_value", Type: frame.FieldTypeString, Values: []any{"osago", "travel"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{42.0, 30.0}},
	)
	require.NoError(t, err)

	options := Options(
		panel.Bar("premium-by-product", "Premium by Product", "products").
			LabelField("label").
			ValueField("value").
			SemanticColors("PRODUCT", panel.Ref("color_value")).
			Build(),
		&runtime.PanelResult{Frames: mustFrameSet(t, fr)},
	)

	require.NotNil(t, options.PlotOptions)
	require.NotNil(t, options.PlotOptions.Bar)
	require.NotNil(t, options.PlotOptions.Bar.Distributed)
	require.True(t, *options.PlotOptions.Bar.Distributed)
	require.Equal(t, []string{"#7C3AED", "#2563EB"}, options.Colors)
}

func TestDistributedTooltipMarkerSyncJSPrefersConfiguredPointColors(t *testing.T) {
	t.Parallel()

	rows := []map[string]any{
		{"label": "Sedan", "value": 42.0},
		{"label": "SUV", "value": 30.0},
		{"label": "Metro", "value": 18.0},
	}

	script := string(distributedTooltipMarkerSyncJS(
		panel.HorizontalBar("vehicle-type", "Vehicle Type", "vehicle-types").
			LabelField("label").
			ValueField("value").
			DistributedColors().
			Build(),
		rows,
		panel.FieldMapping{Label: "label", Value: "value"},
	))

	require.Contains(t, script, "config && config.w && config.w.config")
	require.Contains(t, script, "event && event.target instanceof Element")
	require.Contains(t, script, "hoveredElement.getAttribute('fill')")
	require.Contains(t, script, "hoveredElement.getAttribute('j')")
	require.Contains(t, script, "chartConfig && chartConfig.colors")
	require.Contains(t, script, "globals && globals.fill && globals.fill.colors")
	require.Contains(t, script, "globals && globals.stroke && globals.stroke.colors")
	require.Contains(t, script, "globals && globals.colors")
}

func mustFrameSet(t *testing.T, fr *frame.Frame) *frame.FrameSet {
	t.Helper()

	set, err := frame.NewFrameSet(fr)
	require.NoError(t, err)
	return set
}
