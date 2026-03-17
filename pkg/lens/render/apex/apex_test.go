package apex

import (
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/components/charts"
	"github.com/iota-uz/iota-sdk/pkg/js"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/format"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
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
	require.Contains(t, js, "htmx.ajax")
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

func mustFrameSet(t *testing.T, fr *frame.Frame) *frame.FrameSet {
	t.Helper()

	set, err := frame.NewFrameSet(fr)
	require.NoError(t, err)
	return set
}
