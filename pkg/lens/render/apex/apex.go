// Package apex renders Lens panel results into ApexCharts options.
package apex

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/components/charts"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	lenscolor "github.com/iota-uz/iota-sdk/pkg/lens/color"
	"github.com/iota-uz/iota-sdk/pkg/lens/format"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
)

func Options(panelSpec panel.Spec, panelResult *runtime.PanelResult) charts.ChartOptions {
	return options(panelSpec, panelResult, "")
}

func OptionsWithHeight(panelSpec panel.Spec, panelResult *runtime.PanelResult, height string) charts.ChartOptions {
	options := options(panelSpec, panelResult, height)
	enable := true
	disable := false
	zoomMode := "x"
	autoSelected := "zoom"
	options.Chart.Toolbar = charts.Toolbar{
		Show:         true,
		AutoSelected: &autoSelected,
		Tools: &charts.ToolbarTools{
			Download:  &enable,
			Selection: &disable,
			Zoom:      &enable,
			ZoomIn:    &enable,
			ZoomOut:   &enable,
			Pan:       &enable,
			Reset:     &enable,
		},
	}
	options.Chart.Zoom = &charts.ZoomConfig{
		Enabled:        &enable,
		Type:           &zoomMode,
		AutoScaleYaxis: &disable,
	}
	return options
}

func options(panelSpec panel.Spec, panelResult *runtime.PanelResult, heightOverride string) charts.ChartOptions {
	fontFamily := apexTheme.FontFamily
	axisFontSize := apexTheme.AxisFontSize
	axisColor := apexTheme.AxisLabelColor

	options := charts.ChartOptions{
		Chart: charts.ChartConfig{
			Type:    chartType(panelSpec.Kind),
			Height:  panelHeight(panelSpec, heightOverride),
			Toolbar: charts.Toolbar{Show: false},
			Stacked: panelSpec.Kind == panel.KindStackedBar,
		},
		DataLabels: &charts.DataLabels{Enabled: false},
		Colors:     panelColors(panelSpec, panelResult),
		Grid: &charts.GridConfig{
			BorderColor: apexTheme.GridColor,
			// Horizontal gridlines only; horizontal_bar panels invert this below.
			XAxis:   &charts.GridAxisConfig{Lines: &charts.GridLinesConfig{Show: mapping.BoolPointer(false)}},
			YAxis:   &charts.GridAxisConfig{Lines: &charts.GridLinesConfig{Show: mapping.BoolPointer(true)}},
			Padding: &charts.Padding{Top: mapping.Pointer(4), Right: mapping.Pointer(12), Bottom: mapping.Pointer(0), Left: mapping.Pointer(12)},
		},
		Tooltip: &charts.TooltipConfig{
			Theme:     mapping.Pointer("light"),
			CSSClass:  mapping.Pointer(apexTheme.TooltipCSSClass),
			Shared:    mapping.Pointer(true),
			Intersect: mapping.BoolPointer(false),
			Style:     &charts.TooltipStyleConfig{FontSize: mapping.Pointer("12px"), FontFamily: &fontFamily},
		},
		XAxis: charts.XAxisConfig{
			Labels: &charts.XAxisLabelsConfig{
				HideOverlappingLabels: mapping.Pointer(true),
				Trim:                  mapping.Pointer(true),
				Style: &charts.XAxisLabelStyleConfig{
					FontSize:   &axisFontSize,
					FontFamily: &fontFamily,
					Colors:     axisColor,
					CSSClass:   mapping.Pointer(apexTheme.NumeralCSSClass),
				},
			},
			AxisBorder: &charts.XAxisBorderConfig{Show: mapping.Pointer(false)},
			AxisTicks:  &charts.XAxisTicksConfig{Show: mapping.Pointer(false)},
		},
		YAxis: []charts.YAxisConfig{
			{
				Labels: &charts.YAxisLabelsConfig{
					Style: &charts.YAxisLabelStyleConfig{
						FontSize:   &axisFontSize,
						FontFamily: &fontFamily,
						Colors:     axisColor,
						CSSClass:   mapping.Pointer(apexTheme.NumeralCSSClass),
					},
				},
			},
		},
		Legend: &charts.LegendConfig{
			Show: mapping.BoolPointer(false),
		},
	}
	if panelResult == nil || panelResult.Frames == nil || panelResult.Frames.Primary() == nil {
		return options
	}
	fr := panelResult.Frames.Primary()
	rows := fr.Rows()
	fields := panelSpec.Fields

	switch panelSpec.Kind {
	case panel.KindPie, panel.KindDonut, panel.KindGauge:
		labels := make([]string, 0, len(rows))
		values := make([]any, 0, len(rows))
		for _, row := range rows {
			labels = append(labels, displayValue(firstNonEmpty(row[fields.Label.Name()], row[fields.Category.Name()])))
			values = append(values, numericValue(row[fields.Value.Name()]))
		}
		options.Labels = labels
		options.Series = values
		if panelSpec.Kind == panel.KindDonut || panelSpec.Kind == panel.KindPie {
			position := charts.LegendPositionBottom
			options.Legend = &charts.LegendConfig{Position: &position}
			applyThemedLegendDefaults(options.Legend)
			// White hairline between slices so adjacent categories separate
			// on the light card surface.
			options.Stroke = &charts.StrokeConfig{
				Show:   mapping.Pointer(true),
				Width:  2,
				Colors: []string{apexTheme.Surface},
			}
		}
		if panelSpec.Kind == panel.KindDonut {
			size := donutSize
			options.PlotOptions = &charts.PlotOptions{Pie: &charts.PieDonutConfig{Donut: &charts.DonutSpecifics{Size: &size}}}
		}
		if panelSpec.Kind == panel.KindGauge {
			startAngle := -135
			endAngle := 225
			hollowSize := "70%"
			options.PlotOptions = &charts.PlotOptions{
				RadialBar: &charts.RadialBarConfig{
					StartAngle: &startAngle,
					EndAngle:   &endAngle,
					Hollow:     &charts.RadialBarHollow{Size: &hollowSize},
				},
			}
		}
	case panel.KindStat, panel.KindTimeSeries, panel.KindBar, panel.KindHorizontalBar, panel.KindStackedBar, panel.KindSegmentBar, panel.KindCascade, panel.KindTable, panel.KindTabs, panel.KindGrid, panel.KindSplit, panel.KindRepeat, panel.KindStatGroup:
		if hasSeries(rows, fields.Series.Name()) {
			categories, series := groupedSeries(rows, fields)
			options.Series = series
			options.XAxis.Categories = categories
		} else {
			categories := make([]string, 0, len(rows))
			values := make([]any, 0, len(rows))
			for _, row := range rows {
				categories = append(categories, displayValue(firstNonEmpty(row[fields.Label.Name()], row[fields.Category.Name()])))
				values = append(values, numericValue(row[fields.Value.Name()]))
			}
			options.Series = []charts.Series{{Name: panelSpec.Title, Data: values}}
			options.XAxis.Categories = categories
		}
	}

	switch panelSpec.Kind {
	case panel.KindPie, panel.KindDonut, panel.KindGauge:
		if options.Grid == nil {
			options.Grid = &charts.GridConfig{}
		}
		options.Grid.BorderColor = "transparent"
		options.XAxis.Labels = nil
		options.XAxis.AxisBorder = nil
		options.XAxis.AxisTicks = nil
		options.YAxis = nil
	case panel.KindStat, panel.KindTimeSeries, panel.KindBar, panel.KindHorizontalBar, panel.KindStackedBar, panel.KindSegmentBar, panel.KindCascade, panel.KindTable, panel.KindTabs, panel.KindGrid, panel.KindSplit, panel.KindRepeat, panel.KindStatGroup:
	}

	if panelSpec.Kind == panel.KindHorizontalBar {
		horizontal := true
		options.PlotOptions = &charts.PlotOptions{Bar: &charts.BarConfig{
			Horizontal:              &horizontal,
			BorderRadius:            barBorderRadius,
			BorderRadiusApplication: mapping.Pointer("end"),
			BarHeight:               mapping.Pointer(horizontalBarHeight),
		}}
		// Bars run along X, so only vertical (x-axis) gridlines carry signal.
		if options.Grid != nil {
			options.Grid.XAxis = &charts.GridAxisConfig{Lines: &charts.GridLinesConfig{Show: mapping.BoolPointer(true)}}
			options.Grid.YAxis = &charts.GridAxisConfig{Lines: &charts.GridLinesConfig{Show: mapping.BoolPointer(false)}}
		}
		ensureHorizontalBarLabelPadding(&options)
	}
	if panelSpec.Kind == panel.KindBar || panelSpec.Kind == panel.KindStackedBar {
		if options.PlotOptions == nil {
			options.PlotOptions = &charts.PlotOptions{Bar: &charts.BarConfig{
				BorderRadius:            barBorderRadius,
				BorderRadiusApplication: mapping.Pointer("end"),
				ColumnWidth:             adaptiveColumnWidth(len(options.XAxis.Categories)),
			}}
		}
	}
	if options.PlotOptions != nil && options.PlotOptions.Bar != nil && usesDistributedBarColors(panelSpec, panelResult) {
		options.PlotOptions.Bar.Distributed = mapping.Pointer(true)
	}
	applyBarHoverStates(&options, panelSpec)
	if panelSpec.Kind == panel.KindTimeSeries {
		curve := charts.StrokeCurveStraight
		options.Stroke = &charts.StrokeConfig{
			Curve:   curve,
			Width:   2,
			LineCap: charts.StrokeLineCapRound,
		}
		options.Markers = &charts.MarkersConfig{
			Size:        0,
			StrokeWidth: 0,
			Hover:       &charts.MarkerHover{SizeOffset: mapping.Pointer(5)},
		}
		// Pure lines: no gradient area wash under the stroke.
		options.Fill = &charts.FillConfig{Type: "solid", Opacity: 1}
	}
	applyCategoryLabelFormatting(&options, panelSpec)
	if panelSpec.ShowLegend {
		position := charts.LegendPositionBottom
		if options.Legend == nil {
			options.Legend = &charts.LegendConfig{}
		}
		options.Legend.Show = mapping.Pointer(true)
		if options.Legend.Position == nil {
			options.Legend.Position = &position
		}
		applyThemedLegendDefaults(options.Legend)
	}
	logPlan, manualLogScaleApplied := applyValueScale(&options, panelSpec)
	tooltipFormatter := applyValueFormatter(&options, panelSpec, panelResult, manualLogScaleApplied, logPlan)
	if panelSpec.Kind == panel.KindDonut {
		applyDonutTotalLabels(&options, panelResult.Locale, tooltipFormatter)
	}
	if panelSpec.Kind == panel.KindDonut || panelSpec.Kind == panel.KindPie {
		if options.Legend != nil && options.Legend.Formatter == "" {
			options.Legend.Formatter = pieLegendFormatterJS(panelResult.Locale, tooltipFormatter)
		}
	}
	var chartEvents charts.ChartEvents
	if syncTooltip := distributedTooltipMarkerSyncJS(panelSpec, rows, fields); syncTooltip != "" {
		chartEvents.DataPointMouseEnter = syncTooltip
	}
	if panelSpec.Action != nil {
		chartEvents.DataPointSelection = buildActionJS(panelSpec.Action, fr, fields, panelResult)
	} else if panelSpec.DrillHierarchy != nil {
		chartEvents.DataPointSelection = buildDrillHierarchyJS(panelSpec.DrillHierarchy, panelSpec.Formatter, panelResult.Locale)
		// Mounted re-derives the same cfg blob so the shared JS state machine can
		// fast-forward a freshly-mounted chart (e.g. the lazy fullscreen instance)
		// to whatever zoom level its sibling in the same rerender scope is at,
		// without waiting for a click.
		chartEvents.Mounted = buildDrillHierarchyMountJS(panelSpec.DrillHierarchy, panelSpec.Formatter, panelResult.Locale)
	}
	if panelSpec.Kind == panel.KindStackedBar || panelSpec.ShowTotalBadge {
		applyStackedBarTotalBadgeEvents(&chartEvents, panelResult.Locale, tooltipFormatter, staticTotalBadgeText(panelSpec, panelResult.Locale))
		// The total badge floats at the top-right of the plot (the drill-back
		// overlay owns the top-left). Reserve a header band above the plot so it
		// clears the top y-axis label and the badge itself.
		if options.Grid != nil && options.Grid.Padding != nil {
			options.Grid.Padding.Top = mapping.Pointer(34)
		}
	}
	if hasChartEvents(chartEvents) {
		options.Chart.Events = &chartEvents
	}
	appendResponsiveDefaults(&options, panelSpec.Kind)
	return options
}

// applyBarHoverStates disables ApexCharts' own hover filter on bar panels.
// Under a shared tooltip Apex applies the hover state to EVERY series at the
// hovered category (all bars of the year darken together), which reads as a
// group highlight rather than "this is the bar you're on". The per-bar
// highlight is instead a plain CSS :hover brightness on .apexcharts-bar-area
// (see DashboardScripts in render/templ/dashboard.templ), which only ever
// matches the single path under the pointer. The active (clicked) state keeps
// an explicit darken so drill clicks give press feedback.
func applyBarHoverStates(options *charts.ChartOptions, panelSpec panel.Spec) {
	switch panelSpec.Kind {
	case panel.KindBar, panel.KindHorizontalBar, panel.KindStackedBar, panel.KindSegmentBar, panel.KindCascade:
	case panel.KindStat, panel.KindTimeSeries, panel.KindPie, panel.KindDonut, panel.KindGauge,
		panel.KindTable, panel.KindTabs, panel.KindGrid, panel.KindSplit, panel.KindRepeat, panel.KindStatGroup:
		return
	default:
		return
	}
	if options.States != nil {
		return
	}
	options.States = &charts.StatesConfig{
		Hover: &charts.StateFilterConfig{Filter: &charts.StateFilter{
			Type: mapping.Pointer("none"),
		}},
		Active: &charts.StateActiveConfig{Filter: &charts.StateFilter{
			Type:  mapping.Pointer("darken"),
			Value: mapping.Pointer(0.2),
		}},
	}
}

// applyThemedLegendDefaults fills unset legend styling with the Lens v2
// legend look: 11px muted labels, small square markers, airy item margins.
// Explicitly-set values are left alone.
func applyThemedLegendDefaults(legend *charts.LegendConfig) {
	if legend == nil {
		return
	}
	if legend.FontSize == nil {
		legend.FontSize = mapping.Pointer(apexTheme.LegendFontSize)
	}
	if legend.FontFamily == nil {
		legend.FontFamily = mapping.Pointer(apexTheme.FontFamily)
	}
	if legend.Labels == nil {
		legend.Labels = &charts.LegendLabelsConfig{Colors: apexTheme.LegendColor}
	}
	if legend.Markers == nil {
		// mapping.Pointer(0) would collapse to nil (zero value) and Apex's
		// default marker stroke (1px) would leak back in.
		markerStrokeWidth := 0
		legend.Markers = &charts.LegendMarkersConfig{
			Size:        mapping.Pointer(5),
			StrokeWidth: &markerStrokeWidth,
			Shape:       mapping.Pointer("square"),
		}
	}
	if legend.ItemMargin == nil {
		legend.ItemMargin = &charts.LegendItemMargin{Horizontal: mapping.Pointer(8), Vertical: mapping.Pointer(4)}
	}
}

// applyDonutTotalLabels enables the donut's hollow-center labels: the
// localized "Total" caption plus the summed value formatted with the panel's
// own tooltip formatter (falling back to a locale-aware integer).
func applyDonutTotalLabels(options *charts.ChartOptions, locale string, tooltipFormatter templ.JSExpression) {
	if options == nil || options.PlotOptions == nil || options.PlotOptions.Pie == nil || options.PlotOptions.Pie.Donut == nil {
		return
	}
	locale = normalizedChartLocale(locale)
	valueFormatter := "null"
	if tooltipFormatter != "" {
		valueFormatter = "(" + string(tooltipFormatter) + ")"
	}
	show := true
	totalFormatter := templ.JSExpression(fmt.Sprintf(`function(w) {
		const fmt = %s;
		const locale = %q;
		const totals = (w && w.globals && w.globals.seriesTotals) || [];
		let total = 0;
		totals.forEach(function(value) {
			const number = Number(value);
			if (Number.isFinite(number)) {
				total += number;
			}
		});
		if (fmt) {
			return fmt(total);
		}
		return Math.round(total).toLocaleString(locale);
	}`, valueFormatter, locale))
	sliceFormatter := templ.JSExpression(fmt.Sprintf(`function(value) {
		const fmt = %s;
		const locale = %q;
		const number = Number(value);
		if (!Number.isFinite(number)) {
			return value == null ? '' : String(value);
		}
		if (fmt) {
			return fmt(number);
		}
		return Math.round(number).toLocaleString(locale);
	}`, valueFormatter, locale))
	options.PlotOptions.Pie.Donut.Labels = &charts.Labels{
		Show: &show,
		Name: &charts.LabelNameValue{
			Show:       &show,
			FontSize:   mapping.Pointer(apexTheme.LegendFontSize),
			FontFamily: mapping.Pointer(apexTheme.FontFamily),
			Color:      mapping.Pointer(apexTheme.LegendColor),
		},
		Value: &charts.LabelNameValue{
			Show:       &show,
			FontSize:   mapping.Pointer("18px"),
			FontFamily: mapping.Pointer(apexTheme.FontFamily),
			FontWeight: mapping.Pointer("600"),
			Color:      mapping.Pointer(apexTheme.TextStrong),
			Formatter:  sliceFormatter,
		},
		Total: &charts.LabelTotal{
			Show:       &show,
			Label:      mapping.Pointer(localizedTotalLabel(locale)),
			FontSize:   mapping.Pointer(apexTheme.LegendFontSize),
			FontFamily: mapping.Pointer(apexTheme.FontFamily),
			FontWeight: mapping.Pointer("500"),
			Color:      mapping.Pointer(apexTheme.LegendColor),
			Formatter:  totalFormatter,
		},
	}
}

// pieLegendFormatterJS renders pie/donut legend entries as
// "label · value (pct%)" using the panel's tooltip formatter for the value.
func pieLegendFormatterJS(locale string, tooltipFormatter templ.JSExpression) templ.JSExpression {
	locale = normalizedChartLocale(locale)
	valueFormatter := "null"
	if tooltipFormatter != "" {
		valueFormatter = "(" + string(tooltipFormatter) + ")"
	}
	return templ.JSExpression(fmt.Sprintf(`function(seriesName, opts) {
		const fmt = %s;
		const locale = %q;
		const w = opts && opts.w;
		const index = opts && typeof opts.seriesIndex === 'number' ? opts.seriesIndex : -1;
		const series = (w && w.globals && w.globals.series) || [];
		const raw = Number(series[index]);
		if (!Number.isFinite(raw)) {
			return seriesName;
		}
		let total = 0;
		series.forEach(function(value) {
			const number = Number(value);
			if (Number.isFinite(number)) {
				total += number;
			}
		});
		const formatted = fmt ? fmt(raw) : Math.round(raw).toLocaleString(locale);
		const pct = total > 0 ? (raw / total) * 100 : 0;
		return seriesName + ' · ' + formatted + ' (' + pct.toFixed(1) + '%%)';
	}`, valueFormatter, locale))
}

func appendResponsiveDefaults(options *charts.ChartOptions, kind panel.Kind) {
	// Skip for pie/donut/gauge — they scale via SVG naturally
	switch kind {
	case panel.KindPie, panel.KindDonut, panel.KindGauge:
		return
	case panel.KindStat,
		panel.KindTimeSeries,
		panel.KindBar,
		panel.KindHorizontalBar,
		panel.KindStackedBar,
		panel.KindSegmentBar, panel.KindCascade,
		panel.KindTable,
		panel.KindTabs,
		panel.KindGrid,
		panel.KindSplit,
		panel.KindRepeat,
		panel.KindStatGroup:
		// Responsive defaults apply to cartesian and table-like layouts.
	}

	tabletFontSize := "10px"
	mobileFontSize := "9px"
	mobileRotate := -45

	// Tablet breakpoint (768px)
	tabletOpts := charts.ResponsiveOptions{
		XAxis: &charts.XAxisConfig{
			Labels: &charts.XAxisLabelsConfig{
				Style: &charts.XAxisLabelStyleConfig{FontSize: &tabletFontSize},
			},
		},
		YAxis: []charts.YAxisConfig{
			{Labels: &charts.YAxisLabelsConfig{Style: &charts.YAxisLabelStyleConfig{FontSize: &tabletFontSize}}},
		},
		Grid: &charts.GridConfig{
			Padding: &charts.Padding{Left: mapping.Pointer(4), Right: mapping.Pointer(4)},
		},
	}
	if options.Legend != nil {
		tabletOpts.Legend = &charts.LegendConfig{FontSize: &tabletFontSize}
	}

	// Mobile breakpoint (480px)
	mobileOpts := charts.ResponsiveOptions{
		XAxis: &charts.XAxisConfig{
			Labels: &charts.XAxisLabelsConfig{
				Style:  &charts.XAxisLabelStyleConfig{FontSize: &mobileFontSize},
				Rotate: &mobileRotate,
			},
		},
		YAxis: []charts.YAxisConfig{
			{Labels: &charts.YAxisLabelsConfig{Style: &charts.YAxisLabelStyleConfig{FontSize: &mobileFontSize}}},
		},
		Grid: &charts.GridConfig{
			Padding: &charts.Padding{Left: mapping.Pointer(0), Right: mapping.Pointer(0)},
		},
	}
	if options.Legend != nil {
		mobileOpts.Legend = &charts.LegendConfig{FontSize: &mobileFontSize}
	}

	options.Responsive = []charts.ResponsiveBreakpoint{
		{Breakpoint: 769, Options: tabletOpts},
		{Breakpoint: 481, Options: mobileOpts},
	}
}

func applyValueScale(options *charts.ChartOptions, panelSpec panel.Spec) (logarithmicAxisPlan, bool) {
	if options == nil {
		return logarithmicAxisPlan{}, false
	}
	axis := normalizedValueAxis(panelSpec.ValueAxis)
	if axis.Scale != panel.AxisScaleLogarithmic {
		return logarithmicAxisPlan{}, false
	}
	series, ok := options.Series.([]charts.Series)
	if !ok || len(series) == 0 {
		return logarithmicAxisPlan{}, false
	}
	if !supportsManualLogScale(panelSpec, series) {
		return logarithmicAxisPlan{}, false
	}
	plan, ok := buildLogarithmicAxisPlan(series, axis.LogBase)
	if !ok {
		return logarithmicAxisPlan{}, false
	}
	for i := range series {
		series[i].Data = logarithmicSeriesData(series[i].Data, axis.LogBase)
	}
	options.Series = series
	step := plan.Step
	if panelSpec.Kind == panel.KindHorizontalBar {
		options.XAxis.Min = mapping.Pointer(plan.MinExponent)
		options.XAxis.Max = mapping.Pointer(plan.MaxExponent)
		options.XAxis.StepSize = &step
		options.XAxis.DecimalsInFloat = mapping.Pointer(0)
		return plan, true
	}
	if len(options.YAxis) == 0 {
		options.YAxis = []charts.YAxisConfig{{}}
	}
	options.YAxis[0].Logarithmic = nil
	options.YAxis[0].LogBase = nil
	options.YAxis[0].Min = plan.MinExponent
	options.YAxis[0].Max = plan.MaxExponent
	options.YAxis[0].StepSize = &step
	options.YAxis[0].TickAmount = mapping.Pointer(plan.TickAmount)
	options.YAxis[0].ForceNiceScale = mapping.Pointer(false)
	options.YAxis[0].DecimalsInFloat = mapping.Pointer(0)
	return plan, true
}

func applyValueFormatter(options *charts.ChartOptions, panelSpec panel.Spec, panelResult *runtime.PanelResult, manualLogScaleApplied bool, logPlan logarithmicAxisPlan) templ.JSExpression {
	if options == nil || panelResult == nil {
		return ""
	}
	axisFormatter, tooltipFormatter := chartValueFormatters(panelSpec.Formatter, panelResult.Locale)
	valueAxis := normalizedValueAxis(panelSpec.ValueAxis)
	if valueAxis.Scale == panel.AxisScaleLogarithmic && manualLogScaleApplied {
		axisFormatter = wrapLogarithmicAxisFormatter(axisFormatter, panelResult.Locale, logPlan)
		tooltipFormatter = wrapLogarithmicTooltipFormatter(tooltipFormatter, panelResult.Locale, valueAxis.LogBase)
	}
	if panelSpec.Kind == panel.KindStackedBar {
		if options.Tooltip == nil {
			options.Tooltip = &charts.TooltipConfig{}
		}
		options.Tooltip.Custom = stackedBarTooltipWithTotal(panelResult.Locale, tooltipFormatter)
	}
	if axisFormatter == "" && tooltipFormatter == "" {
		return tooltipFormatter
	}
	if axisFormatter != "" {
		if panelSpec.Kind == panel.KindHorizontalBar {
			if options.XAxis.Labels != nil {
				options.XAxis.Labels.Formatter = axisFormatter
			}
		} else if len(options.YAxis) > 0 && options.YAxis[0].Labels != nil {
			options.YAxis[0].Labels.Formatter = axisFormatter
		}
	}
	if options.Tooltip == nil {
		options.Tooltip = &charts.TooltipConfig{}
	}
	if tooltipFormatter != "" {
		options.Tooltip.Y = &charts.TooltipYConfig{Formatter: tooltipFormatter}
	}
	return tooltipFormatter
}

func normalizedValueAxis(axis panel.ValueAxis) panel.ValueAxis {
	if axis.Scale == "" {
		axis.Scale = panel.AxisScaleLinear
	}
	if axis.LogBase <= 1 {
		axis.LogBase = 10
	}
	return axis
}

type logarithmicAxisPlan struct {
	Base        int
	MinExponent float64
	MaxExponent float64
	Step        float64
	TickAmount  int
}

func logarithmicAxisPlanForOptions(options charts.ChartOptions, base int) (logarithmicAxisPlan, bool) {
	series, ok := options.Series.([]charts.Series)
	if !ok || len(series) == 0 {
		return logarithmicAxisPlan{}, false
	}
	return buildLogarithmicAxisPlan(series, base)
}

func logarithmicAxisPlanFromAxisOptions(options charts.ChartOptions, kind panel.Kind, base int) (logarithmicAxisPlan, bool) {
	if base <= 1 {
		base = 10
	}
	if kind == panel.KindHorizontalBar {
		return logarithmicAxisPlanFromAxisConfig(options.XAxis.Min, options.XAxis.Max, options.XAxis.StepSize, options.XAxis.TickAmount, base)
	}
	if len(options.YAxis) > 0 {
		axis := options.YAxis[0]
		return logarithmicAxisPlanFromAxisConfig(axis.Min, axis.Max, axis.StepSize, axis.TickAmount, base)
	}
	return logarithmicAxisPlanForOptions(options, base)
}

func logarithmicAxisPlanFromAxisConfig(minValue, maxValue any, step *float64, tickAmount any, base int) (logarithmicAxisPlan, bool) {
	minExponent, okMin := numericAxisValue(minValue)
	maxExponent, okMax := numericAxisValue(maxValue)
	if !okMin || !okMax {
		return logarithmicAxisPlan{}, false
	}
	plan := logarithmicAxisPlan{
		Base:        base,
		MinExponent: minExponent,
		MaxExponent: maxExponent,
		TickAmount:  max(1, int(maxExponent-minExponent)+1),
	}
	if step != nil && *step > 0 {
		plan.Step = *step
	}
	if ticks, ok := numericAxisIntValue(tickAmount); ok && ticks > 0 {
		plan.TickAmount = ticks
	}
	if plan.Step == 0 {
		plan.Step = 1
	}
	return plan, true
}

func numericAxisValue(value any) (float64, bool) {
	switch current := value.(type) {
	case nil:
		return 0, false
	case float64:
		return current, true
	case float32:
		return float64(current), true
	case int:
		return float64(current), true
	case int64:
		return float64(current), true
	case int32:
		return float64(current), true
	case *float64:
		if current == nil {
			return 0, false
		}
		return *current, true
	case *int:
		if current == nil {
			return 0, false
		}
		return float64(*current), true
	default:
		return 0, false
	}
}

func numericAxisIntValue(value any) (int, bool) {
	switch current := value.(type) {
	case nil:
		return 0, false
	case int:
		return current, true
	case int64:
		return int(current), true
	case int32:
		return int(current), true
	case float64:
		return int(current), true
	case float32:
		return int(current), true
	case *int:
		if current == nil {
			return 0, false
		}
		return *current, true
	default:
		return 0, false
	}
}

func buildLogarithmicAxisPlan(series []charts.Series, base int) (logarithmicAxisPlan, bool) {
	if base <= 1 {
		base = 10
	}
	minPositive := 0.0
	maxPositive := 0.0
	for _, entry := range series {
		for _, point := range entry.Data {
			value := numericValue(point)
			if value <= 0 {
				continue
			}
			if minPositive == 0 || value < minPositive {
				minPositive = value
			}
			if value > maxPositive {
				maxPositive = value
			}
		}
	}
	if minPositive <= 0 || maxPositive <= 0 {
		return logarithmicAxisPlan{}, false
	}
	logBase := math.Log(float64(base))
	minExponent := int(math.Floor(math.Log(minPositive) / logBase))
	rawMaxExponent := math.Log(maxPositive) / logBase
	maxExponent := int(math.Ceil(rawMaxExponent))
	if maxExponent <= minExponent {
		maxExponent = minExponent + 1
	}
	span := maxExponent - minExponent
	step := 1
	const maxLabels = 5
	for span/step+1 > maxLabels {
		step++
	}
	maxTickExponent := minExponent + step*int(math.Ceil(float64(span)/float64(step)))
	tickAmount := (maxTickExponent - minExponent) / step
	if tickAmount < 1 {
		tickAmount = 1
	}
	// Rounding the axis top up to a full decade can leave most of a decade
	// empty above the tallest bar (e.g. a 256-billion max stretches the axis
	// to 1 trillion). When labels sit on every decade anyway, cap the axis at
	// the next half-decade (×√base) instead: gridlines land on half-decade
	// steps, the label formatter blanks the non-decade ones, and Apex's tick
	// generator stays exact because the range remains a multiple of stepSize.
	if step == 1 {
		halfMax := math.Ceil(rawMaxExponent*2) / 2
		// Keep a sliver of headroom so the tallest bar never touches the
		// axis top (0.04 of a decade ≈ 10% in value terms).
		if halfMax-rawMaxExponent < 0.04 {
			halfMax += 0.5
		}
		if halfMax < float64(maxTickExponent) && halfMax > float64(minExponent) {
			halfTicks := int(math.Round((halfMax - float64(minExponent)) * 2))
			return logarithmicAxisPlan{
				Base:        base,
				MinExponent: float64(minExponent),
				MaxExponent: halfMax,
				Step:        0.5,
				TickAmount:  halfTicks,
			}, true
		}
	}
	return logarithmicAxisPlan{
		Base:        base,
		MinExponent: float64(minExponent),
		MaxExponent: float64(maxTickExponent),
		Step:        float64(step),
		TickAmount:  tickAmount,
	}, true
}

func logarithmicSeriesData(values []any, base int) []any {
	if len(values) == 0 {
		return values
	}
	scaled := make([]any, len(values))
	for i, value := range values {
		scaled[i] = logarithmicValue(numericValue(value), base)
	}
	return scaled
}

func supportsManualLogScale(panelSpec panel.Spec, series []charts.Series) bool {
	if panelSpec.Kind == panel.KindStackedBar {
		return false
	}
	for _, entry := range series {
		if len(entry.Data) == 0 {
			return false
		}
		for _, point := range entry.Data {
			if numericValue(point) <= 0 {
				return false
			}
		}
	}
	return true
}

func logarithmicValue(value float64, base int) float64 {
	if value <= 0 {
		return 0
	}
	if base <= 1 {
		base = 10
	}
	return math.Log(value) / math.Log(float64(base))
}

func wrapLogarithmicAxisFormatter(formatter templ.JSExpression, locale string, plan logarithmicAxisPlan) templ.JSExpression {
	if strings.TrimSpace(locale) == "" {
		locale = "en-US"
	}
	inner := "null"
	if formatter != "" {
		inner = "(" + string(formatter) + ")"
	}
	// Labels always sit on whole decades: a sub-decade grid step (the
	// half-decade axis cap) only adds unlabeled gridlines.
	labelStep := math.Max(1, plan.Step)
	return templ.JSExpression(fmt.Sprintf(`function(value) {
		const scaled = Number(value);
		if (!Number.isFinite(scaled)) {
			return '';
		}
		const minExponent = %f;
		const step = %f;
		const slot = (scaled - minExponent) / step;
		if (!Number.isFinite(slot) || Math.abs(slot - Math.round(slot)) > 0.001) {
			return '';
		}
		const actual = Math.pow(%d, scaled);
		const normalized = Math.abs(actual) < 1e-9 ? 0 : actual;
		if (%s) {
			return %s(normalized);
		}
		return Math.round(normalized).toLocaleString(%q);
	}`, plan.MinExponent, labelStep, plan.Base, inner, inner, locale))
}

func wrapLogarithmicTooltipFormatter(formatter templ.JSExpression, locale string, base int) templ.JSExpression {
	if strings.TrimSpace(locale) == "" {
		locale = "en-US"
	}
	if base <= 1 {
		base = 10
	}
	inner := "null"
	if formatter != "" {
		inner = "(" + string(formatter) + ")"
	}
	return templ.JSExpression(fmt.Sprintf(`function(value) {
		const scaled = Number(value);
		if (!Number.isFinite(scaled)) {
			return '';
		}
		const actual = Math.pow(%d, scaled);
		const normalized = Math.abs(actual) < 1e-9 ? 0 : actual;
		if (%s) {
			return %s(normalized);
		}
		return Math.round(normalized).toLocaleString(%q);
	}`, base, inner, inner, locale))
}

func chartValueFormatters(spec *format.Spec, locale string) (templ.JSExpression, templ.JSExpression) {
	if spec == nil {
		return "", ""
	}
	if strings.TrimSpace(locale) == "" {
		locale = "en-US"
	}
	switch spec.Kind {
	case format.KindMoney:
		return charts.FullCurrency(locale, spec.Currency), charts.FullCurrency(locale, spec.Currency)
	case format.KindAbbreviatedMoney:
		return charts.AbbreviatedCurrency(locale, spec.Currency), charts.FullCurrency(locale, spec.Currency)
	case format.KindInteger:
		return charts.Count(locale), charts.Count(locale)
	case format.KindPercent:
		return charts.Percentage(spec.Precision), charts.Percentage(spec.Precision)
	case format.KindDate, format.KindMonthLabel, format.KindDuration, format.KindLocalizedString:
		return "", ""
	default:
		return "", ""
	}
}

func stackedBarTooltipWithTotal(locale string, formatter templ.JSExpression) templ.JSExpression {
	locale = normalizedChartLocale(locale)
	label := localizedTotalLabel(locale)
	valueFormatter := "null"
	if formatter != "" {
		valueFormatter = "(" + string(formatter) + ")"
	}
	return templ.JSExpression(fmt.Sprintf(`function({ series, dataPointIndex, w }) {
		const valueFormatter = %s;
		const locale = %q;
		const totalLabel = %q;
		const fallbackMarkerColor = %q;
		const dividerColor = %q;
		const totalTextColor = %q;
		const globals = (w && w.globals) || {};
		const names = globals.seriesNames || [];
		const colors = globals.colors || [];
		const hiddenSeriesIndices = new Set([].concat(globals.collapsedSeriesIndices || [], globals.hiddenSeriesIndices || []));
		const hiddenSeriesNames = new Set([].concat(globals.collapsedSeries || [], globals.hiddenSeries || [])
			.map((entry) => {
				if (entry && typeof entry === 'object' && 'name' in entry) {
					return String(entry.name);
				}
				return String(entry);
			}));
		const isSeriesHidden = (seriesIndex) => hiddenSeriesIndices.has(seriesIndex) || hiddenSeriesNames.has(String(names[seriesIndex] || ''));
		const categories = ((w && w.config && w.config.xaxis && w.config.xaxis.categories) || globals.categoryLabels || globals.labels || []);
		const escapeHTML = (value) => String(value == null ? '' : value)
			.replace(/&/g, '&amp;')
			.replace(/</g, '&lt;')
			.replace(/>/g, '&gt;')
			.replace(/"/g, '&quot;')
			.replace(/'/g, '&#39;');
		const formatValue = (value, seriesIndex) => {
			const number = Number(value);
			const normalized = Number.isFinite(number) ? number : value;
			if (valueFormatter) {
				return valueFormatter(normalized, { seriesIndex, dataPointIndex, w });
			}
			return Number.isFinite(number) ? number.toLocaleString(locale) : String(value == null ? '' : value);
		};
		let total = 0;
		let hasTotal = false;
		const rows = (series || []).map((points, seriesIndex) => {
			if (isSeriesHidden(seriesIndex)) {
				return '';
			}
			const value = points && points[dataPointIndex];
			if (value == null || value === '') {
				return '';
			}
			const number = Number(value);
			if (Number.isFinite(number) && number === 0) {
				return '';
			}
			if (Number.isFinite(number)) {
				total += number;
				hasTotal = true;
			}
			const color = colors[seriesIndex] || fallbackMarkerColor;
			const name = names[seriesIndex] || '';
			return '<div class="apexcharts-tooltip-series-group" style="display:flex;align-items:center;">'
				+ '<span class="apexcharts-tooltip-marker" style="background-color:' + escapeHTML(color) + ';"></span>'
				+ '<div class="apexcharts-tooltip-text">'
				+ '<div class="apexcharts-tooltip-y-group"><span class="apexcharts-tooltip-text-y-label">' + escapeHTML(name) + ': </span>'
				+ '<span class="apexcharts-tooltip-text-y-value">' + escapeHTML(formatValue(value, seriesIndex)) + '</span></div>'
				+ '</div></div>';
		}).join('');
		const totalRow = hasTotal
			? '<div class="apexcharts-tooltip-series-group" style="display:flex;align-items:center;font-weight:600;color:' + totalTextColor + ';border-top:1px solid ' + dividerColor + ';margin-top:4px;padding-top:4px;">'
				+ '<span class="apexcharts-tooltip-marker" style="background-color:transparent;"></span>'
				+ '<div class="apexcharts-tooltip-text"><div class="apexcharts-tooltip-y-group"><span class="apexcharts-tooltip-text-y-label">' + escapeHTML(totalLabel) + ': </span>'
				+ '<span class="apexcharts-tooltip-text-y-value">' + escapeHTML(formatValue(total, -1)) + '</span></div></div></div>'
			: '';
		return '<div class="apexcharts-tooltip-title">' + escapeHTML(categories[dataPointIndex] || '') + '</div>' + rows + totalRow;
	}`, valueFormatter, locale, label, apexTheme.TextFaint, apexTheme.TooltipDividerColor, apexTheme.TextStrong))
}

// staticTotalBadgeText formats TotalBadgeValue server-side with the panel's
// own formatter. The client-side path cannot be reused: on log-scaled panels
// both the plotted points (floored exponents) and the tooltip formatter
// (inverse log transform) operate in exponent space, so neither summing the
// points nor running the raw total through that formatter yields the total.
func staticTotalBadgeText(panelSpec panel.Spec, locale string) string {
	if panelSpec.TotalBadgeValue == nil {
		return ""
	}
	return format.Apply(panelSpec.Formatter, *panelSpec.TotalBadgeValue, normalizedChartLocale(locale), "")
}

func applyStackedBarTotalBadgeEvents(events *charts.ChartEvents, locale string, formatter templ.JSExpression, staticTotalText string) {
	if events == nil {
		return
	}
	totalBadge := stackedBarTotalBadgeJS(locale, formatter, staticTotalText)
	events.Mounted = totalBadge
	events.Updated = totalBadge
	events.LegendClick = totalBadge
}

func stackedBarTotalBadgeJS(locale string, formatter templ.JSExpression, staticTotalText string) templ.JSExpression {
	locale = normalizedChartLocale(locale)
	label := localizedTotalLabel(locale)
	valueFormatter := "null"
	if formatter != "" {
		valueFormatter = "(" + string(formatter) + ")"
	}
	return templ.JSExpression(fmt.Sprintf(`function(chartContext) {
		const valueFormatter = %s;
		const locale = %q;
		const totalLabel = %q;
		const staticTotalText = %q;
		const chipBorderColor = %q;
		const chipBackground = %q;
		const chipTextColor = %q;
		const chipFontFamily = %q;
		const update = () => {
			const ctx = chartContext || null;
			const el = ctx && ctx.el ? ctx.el : null;
			const w = ctx && ctx.w ? ctx.w : null;
			if (!el || !w) {
				return;
			}
			// Bail if the chart was destroyed after this callback was queued.
			// The total-badge JS is bound to mounted/updated/legendClick and
			// defers via setTimeout(update, 0|80); a re-render (filter/drill
			// HTMX swap, or the two-pass readiness render) can destroy the
			// chart inside that window. Apex's Destroy.clear() nulls ctx.series
			// but leaves ctx.el/ctx.w intact, so the guard above cannot catch a
			// dead context — the pending timer would then call
			// ctx.isSeriesHidden() -> this.series.isSeriesHidden -> null deref
			// ("Cannot read properties of null (reading 'isSeriesHidden')").
			// ctx.series is truthy on every live (mounted/updated) chart, so it
			// is the reliable destroyed-marker.
			if (!ctx.series) {
				return;
			}
			if (window.getComputedStyle && window.getComputedStyle(el).position === 'static') {
				el.style.position = 'relative';
			}
			let badge = el.querySelector('[data-lens-stacked-total]');
			if (!badge) {
				badge = document.createElement('div');
				badge.setAttribute('data-lens-stacked-total', 'true');
				badge.style.position = 'absolute';
				// Anchored top-right: the drill-back ("← Back") overlay owns the
				// top-left corner, the y-axis labels live on the left, and the
				// legend sits at the bottom. The vertical offset is (re)computed
				// on every update below so the badge drops clear of the
				// ApexCharts toolbar when one is present.
				badge.style.right = '12px';
				badge.style.zIndex = '5';
				badge.style.padding = '4px 8px';
				badge.style.borderRadius = '6px';
				badge.style.border = '1px solid ' + chipBorderColor;
				badge.style.background = chipBackground;
				badge.style.boxShadow = '0 1px 2px rgba(15, 23, 42, 0.08)';
				badge.style.color = chipTextColor;
				badge.style.fontWeight = '600';
				badge.style.fontSize = '12px';
				badge.style.fontFamily = chipFontFamily;
				badge.style.lineHeight = '16px';
				badge.style.pointerEvents = 'none';
				el.appendChild(badge);
			}
			// In fullscreen the chart mounts with the ApexCharts toolbar (the
			// download / zoom "hamburger" menu), which Apex also anchors to the
			// top-right — this badge's corner. The earlier fix that moved the
			// badge here from the top-left (to clear the drill-back overlay)
			// re-introduced the collision against that toolbar. Rather than
			// relocate to yet another fixed corner, measure the toolbar and drop
			// the badge just below it whenever a visible one is present; the
			// in-page chart disables the toolbar, so the badge stays at the top.
			// Recomputed each update because Apex mounts the toolbar after the
			// first tick and the fullscreen instance mounts lazily.
			const toolbar = el.querySelector('.apexcharts-toolbar');
			let badgeTop = 6;
			if (toolbar) {
				const toolbarStyle = window.getComputedStyle ? window.getComputedStyle(toolbar) : null;
				const toolbarVisible = !toolbarStyle || (toolbarStyle.display !== 'none' && toolbarStyle.visibility !== 'hidden');
				const toolbarHeight = toolbar.offsetHeight || 0;
				if (toolbarVisible && toolbarHeight > 0) {
					badgeTop = toolbarHeight + 10;
				}
			}
			badge.style.top = badgeTop + 'px';
			if (staticTotalText) {
				badge.textContent = totalLabel + ': ' + staticTotalText;
				return;
			}
			const globals = w.globals || {};
			const seriesNames = globals.seriesNames || [];
			const configSeries = w.config && Array.isArray(w.config.series) ? w.config.series : [];
			const runtimeSeries = Array.isArray(globals.series) ? globals.series : [];
			const seriesHiddenResult = (seriesName) => {
				if (!seriesName) {
					return null;
				}
				if (ctx && typeof ctx.isSeriesHidden === 'function') {
					const result = ctx.isSeriesHidden(seriesName);
					if (result != null) {
						return result;
					}
				}
				if (ctx && ctx.series && typeof ctx.series.isSeriesHidden === 'function') {
					return ctx.series.isSeriesHidden(seriesName);
				}
				return null;
			};
			const isSeriesHidden = (seriesName) => {
				const result = seriesHiddenResult(seriesName);
				if (typeof result === 'boolean') {
					return result;
				}
				return Boolean(result && result.isHidden);
			};
			let total = 0;
			const addValue = (value) => {
				if (value && typeof value === 'object' && 'y' in value) {
					value = value.y;
				}
				const number = Number(value);
				if (Number.isFinite(number)) {
					total += number;
				}
			};
			configSeries.forEach((entry, seriesIndex) => {
				const name = seriesNames[seriesIndex] || (entry && entry.name) || '';
				if (isSeriesHidden(String(name))) {
					return;
				}
				const points = entry && Array.isArray(entry.data)
					? entry.data
					: (Array.isArray(runtimeSeries[seriesIndex]) ? runtimeSeries[seriesIndex] : []);
				points.forEach(addValue);
			});
			const formatValue = (value) => {
				if (valueFormatter) {
					return valueFormatter(value, { seriesIndex: -1, dataPointIndex: -1, w });
				}
				return Number.isFinite(value) ? value.toLocaleString(locale) : String(value == null ? '' : value);
			};
			badge.textContent = totalLabel + ': ' + formatValue(total);
		};
		setTimeout(update, 0);
		setTimeout(update, 80);
	}`, valueFormatter, locale, label, staticTotalText, apexTheme.Border, apexTheme.Surface, apexTheme.Text, apexTheme.FontFamily))
}

func normalizedChartLocale(locale string) string {
	if strings.TrimSpace(locale) == "" {
		return "en-US"
	}
	return locale
}

// localizedTotalLabel returns the "Total" caption used by the stacked-bar
// tooltip/badge and the donut center label (this render layer only has a
// locale string, not the page i18n localizer).
func localizedTotalLabel(locale string) string {
	switch {
	case strings.HasPrefix(locale, "ru"):
		return "Итого"
	case strings.HasPrefix(locale, "uz-Cyrl"):
		return "Жами"
	case strings.HasPrefix(locale, "uz"):
		return "Jami"
	default:
		return "Total"
	}
}

func hasChartEvents(events charts.ChartEvents) bool {
	return events.AnimationEnd != "" ||
		events.BeforeMount != "" ||
		events.Mounted != "" ||
		events.Updated != "" ||
		events.MouseMove != "" ||
		events.MouseLeave != "" ||
		events.Click != "" ||
		events.LegendClick != "" ||
		events.MarkerClick != "" ||
		events.XAxisLabelClick != "" ||
		events.Selection != "" ||
		events.DataPointSelection != "" ||
		events.DataPointMouseEnter != "" ||
		events.DataPointMouseLeave != "" ||
		events.BeforeZoom != "" ||
		events.BeforeResetZoom != "" ||
		events.Zoomed != "" ||
		events.Scrolled != ""
}

func buildActionJS(spec *action.Spec, fr *frame.Frame, fields panel.FieldMapping, panelResult *runtime.PanelResult) templ.JSExpression {
	method := spec.Method
	if method == "" {
		method = "GET"
	}
	variables := map[string]any(nil)
	baseQuery := map[string][]string(nil)
	if panelResult != nil {
		variables = panelResult.Variables
		baseQuery = copiedQueryMap(panelResult.Request)
	}
	configJS := mustJSONJS(chartActionConfig{
		Rows:           fr.Rows(),
		Variables:      variables,
		Kind:           string(spec.Kind),
		URL:            spec.URL,
		Method:         method,
		Target:         spec.Target,
		Event:          spec.Event,
		CategoryField:  fields.Category.Name(),
		LabelField:     fields.Label.Name(),
		IDField:        fields.ID.Name(),
		StartTimeField: fields.StartTime.Name(),
		SeriesField:    fields.Series.Name(),
		BaseQuery:      baseQuery,
		Drill:          chartDrillConfig(spec),
	})
	var actionJS string
	switch spec.Kind {
	case action.KindNavigate:
		actionJS = "window.location.href = nextURL;"
	case action.KindCubeDrill:
		actionJS = `const source = (chartContext && chartContext.el) ? chartContext.el : null;
		const target = (source && source.closest) ? source.closest('[data-lens-swap-target]') : document.querySelector('[data-lens-swap-target]');
		if (typeof htmx !== 'undefined' && target) {
			if (target.dataset && target.dataset.lensDrillPending === 'true') {
				return;
			}
			if (target.dataset) {
				target.dataset.lensDrillPending = 'true';
			}
			const clearPending = function(evt) {
				const detail = evt && evt.detail ? evt.detail : {};
				const requestTarget = detail.target || null;
				const requestSource = detail.elt || null;
				if (requestTarget !== target && requestSource !== (source || target)) {
					return;
				}
				if (target.dataset) {
					delete target.dataset.lensDrillPending;
				}
				document.removeEventListener('htmx:afterRequest', clearPending);
				document.removeEventListener('htmx:sendError', clearPending);
				document.removeEventListener('htmx:sendAbort', clearPending);
				document.removeEventListener('htmx:timeout', clearPending);
			};
			document.addEventListener('htmx:afterRequest', clearPending);
			document.addEventListener('htmx:sendError', clearPending);
			document.addEventListener('htmx:sendAbort', clearPending);
			document.addEventListener('htmx:timeout', clearPending);
			if (source && source.setAttribute) {
				source.setAttribute('hx-push-url', 'true');
			}
			if (window.__lensSetSwapTargetLoading) {
				window.__lensSetSwapTargetLoading(target, true);
			}
			try {
				// Route through __lensDrillAjax so the htmx source is always
				// set (here the clicked chart element, falling back to the swap
				// target). htmx.ajax otherwise defaults source to document.body,
				// scoping the htmx-request loading state to the whole page.
				window.__lensDrillAjax(cfg.method || 'GET', nextURL, target, source);
			} catch (error) {
				if (target.dataset) {
					delete target.dataset.lensDrillPending;
				}
				if (window.__lensSetSwapTargetLoading) {
					window.__lensSetSwapTargetLoading(target, false);
				}
				throw error;
			}
		} else {
			window.location.href = nextURL;
		};`
	case action.KindHtmxSwap:
		// Route through __lensDrillAjax so the htmx `source` is always set
		// (here the swap target subtree). htmx.ajax otherwise defaults source
		// to document.body and the in-flight `htmx-request` loading state
		// cascades onto every .btn on the page (nav tabs, sidebar, etc.).
		actionJS = "window.__lensDrillAjax(cfg.method || 'GET', nextURL, cfg.target, cfg.target);"
	case action.KindEmitEvent:
		actionJS = "document.dispatchEvent(new CustomEvent(cfg.event, {detail: payload}));"
	}
	js := fmt.Sprintf(`function(event, chartContext, opts) {
		const cfg = %s;
		const rows = cfg.rows || [];
		const variables = cfg.variables || {};
		const config = chartContext.w.config;
		const categories = (config.xaxis && config.xaxis.categories) ? config.xaxis.categories : [];
		const seriesName = config.series && config.series[opts.seriesIndex] ? config.series[opts.seriesIndex].name : '';
		const categoryName = categories[opts.dataPointIndex] || '';
		const normalizeCategoryValue = function(value) {
			if (value === undefined || value === null || value === '') {
				return '';
			}
			const stringValue = String(value);
			if (/^\d{4}-\d{2}-\d{2}$/.test(stringValue)) {
				return stringValue;
			}
			const parsed = new Date(stringValue);
			if (!Number.isNaN(parsed.getTime())) {
				return parsed.toISOString().slice(0, 10);
			}
			return stringValue;
		};
		const resolveValue = function(value, fallbackValue) {
			if (value === undefined || value === null || value === '') {
				return fallbackValue;
			}
			return value;
		};
		const safeActionURL = function(value) {
			if (value === undefined || value === null) {
				return '';
			}
			const raw = String(value).trim();
			if (!raw || raw.includes('\\\\') || raw.startsWith('//')) {
				return '';
			}
			try {
				const parsed = new URL(raw, window.location.href);
				if ((parsed.protocol !== 'http:' && parsed.protocol !== 'https:') || parsed.origin !== window.location.origin) {
					return '';
				}
				return parsed.pathname + parsed.search + parsed.hash;
			} catch (_) {
				return '';
			}
		};
		const replaceParam = function(name, value) {
			params.delete(name);
			if (Array.isArray(value)) {
				value.forEach(function(item) {
					if (item !== undefined && item !== null && item !== '') {
						params.append(name, String(item));
					}
				});
				return;
			}
			if (value !== undefined && value !== null && value !== '') {
				params.append(name, String(value));
			}
		};
		let row = rows[opts.dataPointIndex] || {};
		const groupedMatch = rows.find(function(item) {
			const categoryValue = item[cfg.categoryField] || item[cfg.labelField] || item[cfg.startTimeField];
			const seriesValue = item[cfg.seriesField] || '';
			return normalizeCategoryValue(categoryValue) === normalizeCategoryValue(categoryName) && String(seriesValue) === String(seriesName);
		});
		if (groupedMatch) {
			row = groupedMatch;
		}
		let nextURL = cfg.url;
		const payload = {};
		const params = new URLSearchParams();
		if (cfg.baseQuery) {
			Object.entries(cfg.baseQuery).forEach(function(entry) {
				const key = entry[0];
				const values = Array.isArray(entry[1]) ? entry[1] : [];
				values.forEach(function(item) {
					if (item !== undefined && item !== null && item !== '') {
						params.append(key, String(item));
					}
				});
			});
		}
	`, configJS)
	if spec.URLSource != nil {
		expr := actionValueJS(*spec.URLSource, fields)
		js += fmt.Sprintf("const resolvedURL = safeActionURL(%s);\nif (!resolvedURL) { return; }\nnextURL = resolvedURL;\n", expr)
	}
	js += "if (!nextURL) { return; }\n"
	for idx, param := range spec.Params {
		expr := actionValueJS(param.Source, fields)
		js += fmt.Sprintf("const paramValue%d = %s;\nif (paramValue%d !== undefined) { replaceParam(%q, paramValue%d); payload[%q] = paramValue%d; }\n", idx, expr, idx, param.Name, idx, param.Name, idx)
	}
	payloadIndex := 0
	for key, source := range spec.Payload {
		expr := actionValueJS(source, fields)
		js += fmt.Sprintf("const payloadValue%d = %s;\nif (payloadValue%d !== undefined) { payload[%q] = payloadValue%d; }\n", payloadIndex, expr, payloadIndex, key, payloadIndex)
		payloadIndex++
	}
	if spec.Kind == action.KindCubeDrill {
		drillExpr := "undefined"
		if spec.Drill != nil {
			drillExpr = actionValueJS(spec.Drill.Value, fields)
		}
		js += fmt.Sprintf(`let drillValue;
		if (cfg.drill) {
			drillValue = %s;
			if (drillValue !== undefined && drillValue !== null && drillValue !== '') {
				const dimension = cfg.drill.dimension;
				const value = String(drillValue);
				const grouped = new Map();
				const passthrough = [];
				for (const entry of Array.from(params.getAll('_f'))) {
					const sep = entry.indexOf(':');
					if (sep <= 0) {
						passthrough.push(entry);
						continue;
					}
					const dim = entry.slice(0, sep);
					const filterValue = entry.slice(sep + 1).trim();
					if (!filterValue) {
						continue;
					}
					if (!grouped.has(dim)) {
						grouped.set(dim, []);
					}
					if (!grouped.get(dim).includes(filterValue)) {
						grouped.get(dim).push(filterValue);
					}
				}
				const current = grouped.get(dimension) || [];
				const existingIdx = current.indexOf(value);
				if (existingIdx >= 0) {
					current.splice(existingIdx, 1);
				} else {
					current.push(value);
				}
				grouped.set(dimension, current);
				params.delete('_f');
				passthrough.forEach(function(entry) { params.append('_f', entry); });
				grouped.forEach(function(values, dim) {
					values.forEach(function(item) {
						params.append('_f', dim + ':' + item);
					});
				});
			} else {
				return;
			}
		}
		`, drillExpr)
	}
	js += `const query = params.toString();
		if (query) {
			nextURL = nextURL + (nextURL.includes('?') ? '&' : '?') + query;
		}
	` + actionJS + `}`
	return templ.JSExpression(js)
}

func applyCategoryLabelFormatting(options *charts.ChartOptions, panelSpec panel.Spec) {
	if options == nil {
		return
	}
	categories := options.XAxis.Categories
	if len(categories) == 0 {
		return
	}
	switch panelSpec.Kind {
	case panel.KindBar, panel.KindStackedBar:
		applyVerticalCategoryLabelFormatting(options, categories)
	case panel.KindHorizontalBar:
		applyHorizontalCategoryLabelFormatting(options, categories)
	case panel.KindStat,
		panel.KindTimeSeries,
		panel.KindPie,
		panel.KindDonut,
		panel.KindSegmentBar, panel.KindCascade,
		panel.KindTable,
		panel.KindGauge,
		panel.KindTabs,
		panel.KindGrid,
		panel.KindSplit,
		panel.KindRepeat,
		panel.KindStatGroup:
		return
	}
}

func applyVerticalCategoryLabelFormatting(options *charts.ChartOptions, categories []string) {
	if options.XAxis.Labels == nil {
		options.XAxis.Labels = &charts.XAxisLabelsConfig{}
	}
	maxLength := maxCategoryLength(categories)
	if maxLength <= 16 {
		return
	}
	rotate := -45
	rotateAlways := true
	maxHeight := 96
	options.XAxis.Labels.Rotate = &rotate
	options.XAxis.Labels.RotateAlways = &rotateAlways
	options.XAxis.Labels.MaxHeight = &maxHeight
	options.XAxis.Labels.Formatter = truncateCategoryLabelFormatter(16)
	if options.Tooltip == nil {
		options.Tooltip = &charts.TooltipConfig{}
	}
	options.Tooltip.X = &charts.TooltipXConfig{Formatter: fullCategoryTooltipXFormatter()}
}

func applyHorizontalCategoryLabelFormatting(options *charts.ChartOptions, categories []string) {
	if len(options.YAxis) == 0 {
		options.YAxis = []charts.YAxisConfig{{}}
	}
	if options.YAxis[0].Labels == nil {
		options.YAxis[0].Labels = &charts.YAxisLabelsConfig{}
	}
	maxLength := maxCategoryLength(categories)
	if maxLength <= 24 {
		return
	}
	maxWidth := 220
	options.YAxis[0].Labels.MaxWidth = &maxWidth
	options.YAxis[0].Labels.Formatter = truncateCategoryLabelFormatter(24)
	if options.Tooltip == nil {
		options.Tooltip = &charts.TooltipConfig{}
	}
	options.Tooltip.X = &charts.TooltipXConfig{Formatter: fullCategoryTooltipXFormatter()}
}

func maxCategoryLength(categories []string) int {
	maxLength := 0
	for _, category := range categories {
		if length := len([]rune(strings.TrimSpace(category))); length > maxLength {
			maxLength = length
		}
	}
	return maxLength
}

func truncateCategoryLabelFormatter(limit int) templ.JSExpression {
	if limit <= 3 {
		return templ.JSExpression(fmt.Sprintf(`function(value) {
		if (value === undefined || value === null) {
			return '';
		}
		const text = String(value).trim();
		if (text.length <= %d) {
			return text;
		}
		return text.slice(0, %d);
	}`, limit, limit))
	}
	return templ.JSExpression(fmt.Sprintf(`function(value) {
		if (value === undefined || value === null) {
			return '';
		}
		const text = String(value).trim();
		if (text.length <= %d) {
			return text;
		}
		return text.slice(0, %d) + '...';
	}`, limit, limit-3))
}

// fullCategoryTooltipXFormatter returns the untruncated category label for the
// hovered data point by reading it directly from w.config.xaxis.categories
// (which retains full names) instead of the axis-formatted, truncated value
// ApexCharts passes in. Keeps axis labels truncated while tooltips show full names.
func fullCategoryTooltipXFormatter() templ.JSExpression {
	return templ.JSExpression(`function(value, opts) {
		const w = opts && opts.w;
		const cats = (w && w.config && w.config.xaxis && w.config.xaxis.categories) || [];
		const idx = opts && typeof opts.dataPointIndex === 'number' ? opts.dataPointIndex : -1;
		if (idx >= 0 && idx < cats.length && cats[idx] != null) {
			return String(cats[idx]);
		}
		return value == null ? '' : String(value);
	}`)
}

func chartDrillConfig(spec *action.Spec) *chartDrill {
	if spec == nil || spec.Kind != action.KindCubeDrill || spec.Drill == nil {
		return nil
	}
	return &chartDrill{
		Dimension: spec.Drill.Dimension,
	}
}

func copiedQueryMap(values map[string][]string) map[string][]string {
	if values == nil {
		return nil
	}
	out := map[string][]string{}
	for key, items := range values {
		out[key] = append([]string(nil), items...)
	}
	return out
}

func actionValueJS(source action.ValueSource, fields panel.FieldMapping) string {
	switch source.Kind {
	case action.SourceField:
		return fmt.Sprintf("resolveValue(row[%q], %s)", source.Name, jsFallbackLiteral(source.Fallback))
	case action.SourceVariable:
		return fmt.Sprintf("resolveValue(variables[%q], %s)", source.Name, jsFallbackLiteral(source.Fallback))
	case action.SourceLiteral:
		return mustJSONJS(source.Value)
	default:
		return "undefined"
	}
}

func jsFallbackLiteral(value any) string {
	if value == nil {
		return "undefined"
	}
	return mustJSONJS(value)
}

func mustJSONJS(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "null"
	}
	return string(encoded)
}

type chartActionConfig struct {
	Rows           []map[string]any    `json:"rows"`
	Variables      map[string]any      `json:"variables"`
	Kind           string              `json:"kind"`
	URL            string              `json:"url"`
	Method         string              `json:"method,omitempty"`
	Target         string              `json:"target,omitempty"`
	Event          string              `json:"event,omitempty"`
	CategoryField  string              `json:"categoryField"`
	LabelField     string              `json:"labelField"`
	IDField        string              `json:"idField"`
	StartTimeField string              `json:"startTimeField"`
	SeriesField    string              `json:"seriesField"`
	BaseQuery      map[string][]string `json:"baseQuery,omitempty"`
	Drill          *chartDrill         `json:"drill,omitempty"`
}

type chartDrill struct {
	Dimension string `json:"dimension,omitempty"`
}

func chartType(kind panel.Kind) charts.ChartType {
	switch kind {
	case panel.KindTimeSeries:
		return charts.LineChartType
	case panel.KindPie:
		return charts.PieChartType
	case panel.KindDonut:
		return charts.DonutChartType
	case panel.KindGauge:
		return charts.RadialBarChartType
	case panel.KindStat, panel.KindBar, panel.KindHorizontalBar, panel.KindStackedBar, panel.KindSegmentBar, panel.KindCascade, panel.KindTable, panel.KindTabs, panel.KindGrid, panel.KindSplit, panel.KindRepeat, panel.KindStatGroup:
		return charts.BarChartType
	}
	return charts.BarChartType
}

func panelHeight(panelSpec panel.Spec, heightOverride string) string {
	if heightOverride != "" {
		return heightOverride
	}
	if panelSpec.Height != "" {
		return panelSpec.Height
	}
	return "320px"
}

func ensureHorizontalBarLabelPadding(options *charts.ChartOptions) {
	if options == nil {
		return
	}
	if options.Grid == nil {
		options.Grid = &charts.GridConfig{}
	}
	if options.Grid.Padding == nil {
		options.Grid.Padding = &charts.Padding{}
	}
	rightPadding := 40
	if options.Grid.Padding.Right == nil || *options.Grid.Padding.Right < rightPadding {
		options.Grid.Padding.Right = mapping.Pointer(rightPadding)
	}
}

func panelColors(panelSpec panel.Spec, panelResult *runtime.PanelResult) []string {
	if colors := semanticPanelColors(panelSpec, panelResult); len(colors) > 0 {
		return colors
	}
	if len(panelSpec.Colors) > 0 {
		return panelSpec.Colors
	}
	count := fallbackPanelColorCount(panelSpec, panelResult)
	if count <= 1 {
		// Single-series charts with no explicit colors render in the one
		// Lens accent, not a per-category rainbow.
		return []string{lenscolor.Accent()}
	}
	return lenscolor.Categorical(count)
}

func distributedTooltipMarkerSyncJS(panelSpec panel.Spec, rows []map[string]any, fields panel.FieldMapping) templ.JSExpression {
	if !usesDistributedBarColorsForRows(panelSpec, rows, fields) {
		return ""
	}
	switch panelSpec.Kind {
	case panel.KindBar, panel.KindHorizontalBar:
	case panel.KindStat,
		panel.KindTimeSeries,
		panel.KindStackedBar,
		panel.KindSegmentBar, panel.KindCascade,
		panel.KindPie,
		panel.KindDonut,
		panel.KindTable,
		panel.KindGauge,
		panel.KindTabs,
		panel.KindGrid,
		panel.KindSplit,
		panel.KindRepeat,
		panel.KindStatGroup:
	default:
		return ""
	}
	return templ.JSExpression(`function(event, chartContext, config) {
		requestAnimationFrame(function() {
			const normalizeColor = function(value) {
				if (typeof value !== 'string') {
					return '';
				}
				const color = value.trim();
				if (!color || color === 'none' || color === 'transparent' || color === 'rgba(0, 0, 0, 0)') {
					return '';
				}
				return color;
			};
			const chartRoot = chartContext && chartContext.el instanceof Element ? chartContext.el : null;
			const scopedTooltip = chartRoot ? chartRoot.querySelector('.apexcharts-tooltip') : null;
			const tooltips = scopedTooltip ? [scopedTooltip] : Array.from(document.querySelectorAll('.apexcharts-tooltip'));
			const tooltip = tooltips.find(function(node) {
				if (!(node instanceof HTMLElement)) {
					return false;
				}
				const style = window.getComputedStyle(node);
				return style.display !== 'none' && style.visibility !== 'hidden' && node.offsetWidth > 0 && node.offsetHeight > 0;
			});
			if (!tooltip) {
				return;
			}
			const marker = tooltip.querySelector('.apexcharts-tooltip-marker');
			if (!marker) {
				return;
			}
			const hoveredElement = event && event.target instanceof Element ? event.target.closest('path, rect, circle, line, polygon, polyline') : null;
			let color = '';
			if (hoveredElement) {
				color = normalizeColor(hoveredElement.getAttribute('fill'));
				if (!color) {
					color = normalizeColor(hoveredElement.getAttribute('stroke'));
				}
				if (!color) {
					const hoveredStyle = window.getComputedStyle(hoveredElement);
					color = normalizeColor(hoveredStyle.fill) || normalizeColor(hoveredStyle.stroke);
				}
			}
			const resolveIndexedColor = function(values, index) {
				if (!Array.isArray(values) || values.length === 0) {
					return '';
				}
				const fallbackIndex = Number.isFinite(index) && index >= 0 ? index : 0;
				const candidate = values[fallbackIndex] || values[0];
				return normalizeColor(candidate);
			};
			const chartConfig = config && config.w && config.w.config;
			const globals = config && config.w && config.w.globals;
			let index = Number(config && config.dataPointIndex);
			if (!Number.isFinite(index) && hoveredElement) {
				const domIndex = Number(hoveredElement.getAttribute('j'));
				if (Number.isFinite(domIndex)) {
					index = domIndex;
				}
			}
			if (!color) {
				color = resolveIndexedColor(chartConfig && chartConfig.colors, index);
			}
			if (!color) {
				color = resolveIndexedColor(globals && globals.fill && globals.fill.colors, index);
			}
			if (!color) {
				color = resolveIndexedColor(globals && globals.stroke && globals.stroke.colors, index);
			}
			if (!color) {
				color = resolveIndexedColor(globals && globals.colors, index);
			}
			if (!color) {
				return;
			}
			marker.style.backgroundColor = color;
			marker.style.borderColor = color;
		});
	}`)
}

func semanticPanelColors(panelSpec panel.Spec, panelResult *runtime.PanelResult) []string {
	if panelResult == nil || panelResult.Frames == nil || panelResult.Frames.Primary() == nil {
		return nil
	}
	if strings.TrimSpace(panelSpec.ColorScale) == "" || panelSpec.ColorField.Empty() {
		return nil
	}
	rows := panelResult.Frames.Primary().Rows()
	if len(rows) == 0 {
		return nil
	}
	keys := make([]string, 0, len(rows))
	seen := map[string]bool{}
	seriesField := panelSpec.Fields.Series.Name()
	groupedBySeries := hasSeries(rows, seriesField)
	for _, row := range rows {
		seriesName := ""
		if groupedBySeries {
			seriesName = displayValue(row[seriesField])
			if seen[seriesName] {
				continue
			}
		}
		key := displayValue(firstNonEmpty(
			row[panelSpec.ColorField.Name()],
			seriesName,
			row[panelSpec.Fields.ID.Name()],
			row[panelSpec.Fields.Category.Name()],
			row[panelSpec.Fields.Label.Name()],
		))
		keys = append(keys, key)
		if groupedBySeries {
			seen[seriesName] = true
		}
	}
	return lenscolor.Palette(panelSpec.ColorScale, keys)
}

func fallbackPanelColorCount(panelSpec panel.Spec, panelResult *runtime.PanelResult) int {
	if panelResult == nil || panelResult.Frames == nil || panelResult.Frames.Primary() == nil {
		return 1
	}
	rows := panelResult.Frames.Primary().Rows()
	if len(rows) == 0 {
		return 1
	}
	if hasSeries(rows, panelSpec.Fields.Series.Name()) {
		return len(uniqueDisplayValues(rows, panelSpec.Fields.Series.Name()))
	}
	switch panelSpec.Kind {
	case panel.KindPie, panel.KindDonut, panel.KindStackedBar:
		return len(rows)
	case panel.KindBar, panel.KindHorizontalBar:
		if usesDistributedBarColorsForRows(panelSpec, rows, panelSpec.Fields) {
			return len(rows)
		}
	case panel.KindStat,
		panel.KindTimeSeries,
		panel.KindSegmentBar, panel.KindCascade,
		panel.KindTable,
		panel.KindGauge,
		panel.KindTabs,
		panel.KindGrid,
		panel.KindSplit,
		panel.KindRepeat,
		panel.KindStatGroup:
		return 1
	}
	return 1
}

func usesDistributedBarColors(panelSpec panel.Spec, panelResult *runtime.PanelResult) bool {
	if panelResult == nil || panelResult.Frames == nil || panelResult.Frames.Primary() == nil {
		return panelSpec.Distributed
	}
	return usesDistributedBarColorsForRows(panelSpec, panelResult.Frames.Primary().Rows(), panelSpec.Fields)
}

func usesDistributedBarColorsForRows(panelSpec panel.Spec, rows []map[string]any, fields panel.FieldMapping) bool {
	switch panelSpec.Kind {
	case panel.KindBar, panel.KindHorizontalBar:
	case panel.KindStat,
		panel.KindTimeSeries,
		panel.KindStackedBar,
		panel.KindSegmentBar, panel.KindCascade,
		panel.KindPie,
		panel.KindDonut,
		panel.KindTable,
		panel.KindGauge,
		panel.KindTabs,
		panel.KindGrid,
		panel.KindSplit,
		panel.KindRepeat,
		panel.KindStatGroup:
		return false
	}
	if hasSeries(rows, fields.Series.Name()) {
		return false
	}
	if panelSpec.Distributed {
		return true
	}
	// A semantic color scale assigns one color per category, which only
	// renders when Apex distributes colors across data points. There is no
	// implicit "multi-row single-series ⇒ rainbow" fallback: without an
	// explicit Distributed flag or a semantic scale, a single-series bar
	// stays single-colored (lens accent).
	return strings.TrimSpace(panelSpec.ColorScale) != "" && !panelSpec.ColorField.Empty() && len(rows) > 1
}

func uniqueDisplayValues(rows []map[string]any, field string) []string {
	if field == "" {
		return nil
	}
	values := make([]string, 0, len(rows))
	seen := map[string]bool{}
	for _, row := range rows {
		value := displayValue(row[field])
		if seen[value] {
			continue
		}
		seen[value] = true
		values = append(values, value)
	}
	return values
}

func hasSeries(rows []map[string]any, field string) bool {
	if field == "" {
		return false
	}
	for _, row := range rows {
		value, ok := row[field]
		if !ok || value == nil {
			continue
		}
		if strings.TrimSpace(fmt.Sprint(value)) != "" {
			return true
		}
	}
	return false
}

func groupedSeries(rows []map[string]any, fields panel.FieldMapping) ([]string, []charts.Series) {
	categoryOrder := make([]string, 0)
	categorySeen := map[string]bool{}
	seriesOrder := make([]string, 0)
	seriesSeen := map[string]bool{}
	index := map[string]map[string]float64{}
	for _, row := range rows {
		category := displayValue(firstNonEmpty(row[fields.Category.Name()], row[fields.Label.Name()]))
		series := displayValue(row[fields.Series.Name()])
		if !categorySeen[category] {
			categorySeen[category] = true
			categoryOrder = append(categoryOrder, category)
		}
		if !seriesSeen[series] {
			seriesSeen[series] = true
			seriesOrder = append(seriesOrder, series)
		}
		if _, ok := index[series]; !ok {
			index[series] = make(map[string]float64)
		}
		index[series][category] = numericValue(row[fields.Value.Name()])
	}
	series := make([]charts.Series, 0, len(seriesOrder))
	for _, name := range seriesOrder {
		values := make([]any, len(categoryOrder))
		for i, category := range categoryOrder {
			values[i] = index[name][category]
		}
		series = append(series, charts.Series{Name: name, Data: values})
	}
	return categoryOrder, series
}

func firstNonEmpty(values ...any) any {
	for _, value := range values {
		if value == nil {
			continue
		}
		if strings.TrimSpace(displayValue(value)) != "" {
			return value
		}
	}
	return ""
}

func numericValue(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case int32:
		return float64(v)
	case uint:
		return float64(v)
	case uint64:
		return float64(v)
	case uint32:
		return float64(v)
	case uint16:
		return float64(v)
	case uint8:
		return float64(v)
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err == nil {
			return parsed
		}
		return 0
	default:
		return 0
	}
}

func displayValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case *string:
		if v == nil {
			return ""
		}
		return *v
	case time.Time:
		return v.Format("2006-01-02")
	case *time.Time:
		if v == nil {
			return ""
		}
		return v.Format("2006-01-02")
	default:
		return fmt.Sprint(v)
	}
}
