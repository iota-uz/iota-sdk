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
	fontFamily := "'Inter', 'Helvetica Neue', Arial, sans-serif"
	axisFontSize := "11px"
	axisColor := "#9ca3af" // gray-400
	gridColor := "#f0f0f3" // subtle neutral grid

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
			BorderColor: gridColor,
			Padding:     &charts.Padding{Top: mapping.Pointer(4), Right: mapping.Pointer(12), Bottom: mapping.Pointer(0), Left: mapping.Pointer(12)},
		},
		Tooltip: &charts.TooltipConfig{
			Theme:     mapping.Pointer("dark"),
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
		if panelSpec.Kind == panel.KindDonut {
			size := "72%"
			position := charts.LegendPositionBottom
			legendFontSize := "11px"
			options.Legend = &charts.LegendConfig{
				Position:   &position,
				FontSize:   &legendFontSize,
				FontFamily: &fontFamily,
				ItemMargin: &charts.LegendItemMargin{Horizontal: mapping.Pointer(6), Vertical: mapping.Pointer(2)},
			}
			options.PlotOptions = &charts.PlotOptions{Pie: &charts.PieDonutConfig{Donut: &charts.DonutSpecifics{Size: &size}}}
		}
		if panelSpec.Kind == panel.KindPie {
			position := charts.LegendPositionBottom
			legendFontSize := "11px"
			options.Legend = &charts.LegendConfig{
				Position:   &position,
				FontSize:   &legendFontSize,
				FontFamily: &fontFamily,
				ItemMargin: &charts.LegendItemMargin{Horizontal: mapping.Pointer(6), Vertical: mapping.Pointer(2)},
			}
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
	case panel.KindStat, panel.KindTimeSeries, panel.KindBar, panel.KindHorizontalBar, panel.KindStackedBar, panel.KindTable, panel.KindTabs, panel.KindGrid, panel.KindSplit, panel.KindRepeat:
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
	case panel.KindStat, panel.KindTimeSeries, panel.KindBar, panel.KindHorizontalBar, panel.KindStackedBar, panel.KindTable, panel.KindTabs, panel.KindGrid, panel.KindSplit, panel.KindRepeat:
	}

	if panelSpec.Kind == panel.KindHorizontalBar {
		horizontal := true
		options.PlotOptions = &charts.PlotOptions{Bar: &charts.BarConfig{Horizontal: &horizontal, BorderRadius: 4, ColumnWidth: "50%"}}
		ensureHorizontalBarLabelPadding(&options)
	}
	if panelSpec.Kind == panel.KindBar || panelSpec.Kind == panel.KindStackedBar {
		if options.PlotOptions == nil {
			options.PlotOptions = &charts.PlotOptions{Bar: &charts.BarConfig{BorderRadius: 4, ColumnWidth: "48%"}}
		}
	}
	if options.PlotOptions != nil && options.PlotOptions.Bar != nil && usesDistributedBarColors(panelSpec, panelResult) {
		options.PlotOptions.Bar.Distributed = mapping.Pointer(true)
	}
	if panelSpec.Kind == panel.KindTimeSeries {
		curve := charts.StrokeCurveSmooth
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
		options.Fill = &charts.FillConfig{
			Type:    "gradient",
			Opacity: 1,
			Gradient: &charts.FillGradient{
				ShadeIntensity: mapping.Pointer(1.0),
				OpacityFrom:    mapping.Pointer(0.25),
				OpacityTo:      mapping.Pointer(0.05),
				Stops:          []float64{0, 90, 100},
			},
		}
	}
	applyCategoryLabelFormatting(&options, panelSpec)
	if panelSpec.ShowLegend {
		position := charts.LegendPositionBottom
		legendFontSize := "11px"
		if options.Legend == nil {
			options.Legend = &charts.LegendConfig{}
		}
		options.Legend.Show = mapping.Pointer(true)
		if options.Legend.Position == nil {
			options.Legend.Position = &position
		}
		if options.Legend.FontSize == nil {
			options.Legend.FontSize = &legendFontSize
		}
		if options.Legend.FontFamily == nil {
			options.Legend.FontFamily = &fontFamily
		}
		if options.Legend.ItemMargin == nil {
			options.Legend.ItemMargin = &charts.LegendItemMargin{Horizontal: mapping.Pointer(8), Vertical: mapping.Pointer(2)}
		}
	}
	logPlan, manualLogScaleApplied := applyValueScale(&options, panelSpec)
	applyValueFormatter(&options, panelSpec, panelResult, manualLogScaleApplied, logPlan)
	var chartEvents charts.ChartEvents
	if syncTooltip := distributedTooltipMarkerSyncJS(panelSpec, rows, fields); syncTooltip != "" {
		chartEvents.DataPointMouseEnter = syncTooltip
	}
	if panelSpec.Action != nil {
		chartEvents.DataPointSelection = buildActionJS(panelSpec.Action, fr, fields, panelResult)
	}
	if chartEvents.DataPointMouseEnter != "" || chartEvents.DataPointSelection != "" {
		options.Chart.Events = &chartEvents
	}
	appendResponsiveDefaults(&options, panelSpec.Kind)
	return options
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
		panel.KindTable,
		panel.KindTabs,
		panel.KindGrid,
		panel.KindSplit,
		panel.KindRepeat:
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

func applyValueFormatter(options *charts.ChartOptions, panelSpec panel.Spec, panelResult *runtime.PanelResult, manualLogScaleApplied bool, logPlan logarithmicAxisPlan) {
	if options == nil || panelResult == nil {
		return
	}
	axisFormatter, tooltipFormatter := chartValueFormatters(panelSpec.Formatter, panelResult.Locale)
	valueAxis := normalizedValueAxis(panelSpec.ValueAxis)
	if valueAxis.Scale == panel.AxisScaleLogarithmic && manualLogScaleApplied {
		axisFormatter = wrapLogarithmicAxisFormatter(axisFormatter, panelResult.Locale, logPlan)
		tooltipFormatter = wrapLogarithmicTooltipFormatter(tooltipFormatter, panelResult.Locale, valueAxis.LogBase)
	}
	if axisFormatter == "" && tooltipFormatter == "" {
		return
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
	maxExponent := int(math.Ceil(math.Log(maxPositive) / logBase))
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
	}`, plan.MinExponent, plan.Step, plan.Base, inner, inner, locale))
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
				htmx.ajax(cfg.method || 'GET', nextURL, {source: source || target, target: target, swap: 'innerHTML'});
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
		actionJS = "htmx.ajax(cfg.method || 'GET', nextURL, {target: cfg.target, swap: 'innerHTML'});"
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
				params.append('_f', cfg.drill.dimension + ':' + String(drillValue));
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
		panel.KindTable,
		panel.KindGauge,
		panel.KindTabs,
		panel.KindGrid,
		panel.KindSplit,
		panel.KindRepeat:
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
	case panel.KindStat, panel.KindBar, panel.KindHorizontalBar, panel.KindStackedBar, panel.KindTable, panel.KindTabs, panel.KindGrid, panel.KindSplit, panel.KindRepeat:
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
	return lenscolor.Sequence(string(panelSpec.Kind), fallbackPanelColorCount(panelSpec, panelResult))
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
		panel.KindPie,
		panel.KindDonut,
		panel.KindTable,
		panel.KindGauge,
		panel.KindTabs,
		panel.KindGrid,
		panel.KindSplit,
		panel.KindRepeat:
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
		panel.KindTable,
		panel.KindGauge,
		panel.KindTabs,
		panel.KindGrid,
		panel.KindSplit,
		panel.KindRepeat:
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
		panel.KindPie,
		panel.KindDonut,
		panel.KindTable,
		panel.KindGauge,
		panel.KindTabs,
		panel.KindGrid,
		panel.KindSplit,
		panel.KindRepeat:
		return false
	}
	if hasSeries(rows, fields.Series.Name()) {
		return false
	}
	if panelSpec.Distributed {
		return true
	}
	if len(rows) <= 1 {
		return false
	}
	return strings.TrimSpace(panelSpec.ColorScale) == "" && len(panelSpec.Colors) == 0
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
