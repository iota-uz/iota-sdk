// Package apex renders Lens panel results into ApexCharts options.
package apex

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/components/charts"
	"github.com/iota-uz/iota-sdk/pkg/js"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
)

func Options(panelSpec panel.Spec, panelResult *runtime.PanelResult) charts.ChartOptions {
	fontFamily := "'Inter', 'Helvetica Neue', Arial, sans-serif"
	axisFontSize := "11px"
	axisColor := "#9ca3af" // gray-400
	gridColor := "#f0f0f3" // subtle neutral grid

	options := charts.ChartOptions{
		Chart: charts.ChartConfig{
			Type:    chartType(panelSpec.Kind),
			Height:  panelHeight(panelSpec),
			Toolbar: charts.Toolbar{Show: false},
			Stacked: panelSpec.Kind == panel.KindStackedBar,
		},
		DataLabels: &charts.DataLabels{Enabled: false},
		Colors:     panelColors(panelSpec),
		Grid: &charts.GridConfig{
			BorderColor: gridColor,
			Padding:     &charts.Padding{Top: mapping.Pointer(4), Right: mapping.Pointer(12), Bottom: mapping.Pointer(0), Left: mapping.Pointer(12)},
		},
		Tooltip: &charts.TooltipConfig{
			Theme: mapping.Pointer("dark"),
			Style: &charts.TooltipStyleConfig{FontSize: mapping.Pointer("12px"), FontFamily: &fontFamily},
		},
		XAxis: charts.XAxisConfig{
			Labels: &charts.XAxisLabelsConfig{
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
	default:
	}

	if panelSpec.Kind == panel.KindHorizontalBar {
		horizontal := true
		options.PlotOptions = &charts.PlotOptions{Bar: &charts.BarConfig{Horizontal: &horizontal, BorderRadius: 4, ColumnWidth: "50%"}}
	}
	if panelSpec.Kind == panel.KindBar || panelSpec.Kind == panel.KindStackedBar {
		if options.PlotOptions == nil {
			options.PlotOptions = &charts.PlotOptions{Bar: &charts.BarConfig{BorderRadius: 4, ColumnWidth: "48%"}}
		}
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
	if panelSpec.Action != nil {
		options.Chart.Events = &charts.ChartEvents{DataPointSelection: buildActionJS(panelSpec.Action, fr, fields, panelResult.Variables)}
	}
	return options
}

func buildActionJS(spec *action.Spec, fr *frame.Frame, fields panel.FieldMapping, variables map[string]any) templ.JSExpression {
	method := spec.Method
	if method == "" {
		method = "GET"
	}
	configJS := js.MustToJS(chartActionConfig{
		Rows:           fr.Rows(),
		Variables:      variables,
		URL:            spec.URL,
		Method:         method,
		Target:         spec.Target,
		Event:          spec.Event,
		CategoryField:  fields.Category.Name(),
		LabelField:     fields.Label.Name(),
		StartTimeField: fields.StartTime.Name(),
		SeriesField:    fields.Series.Name(),
	})
	var actionJS string
	switch spec.Kind {
	case action.KindNavigate:
		actionJS = "window.location.href = nextURL;"
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
	`, configJS)
	for idx, param := range spec.Params {
		expr := actionValueJS(param.Source, fields)
		js += fmt.Sprintf("const paramValue%d = %s;\nif (paramValue%d !== undefined) { params.append(%q, paramValue%d); payload[%q] = paramValue%d; }\n", idx, expr, idx, param.Name, idx, param.Name, idx)
	}
	payloadIndex := 0
	for key, source := range spec.Payload {
		expr := actionValueJS(source, fields)
		js += fmt.Sprintf("const payloadValue%d = %s;\nif (payloadValue%d !== undefined) { payload[%q] = payloadValue%d; }\n", payloadIndex, expr, payloadIndex, key, payloadIndex)
		payloadIndex++
	}
	js += `const query = params.toString();
		if (query) {
			nextURL = nextURL + (nextURL.includes('?') ? '&' : '?') + query;
		}
	` + actionJS + `}`
	return templ.JSExpression(js)
}

func actionValueJS(source action.ValueSource, fields panel.FieldMapping) string {
	switch source.Kind {
	case action.SourceField:
		return fmt.Sprintf("resolveValue(row[%q], %s)", source.Name, jsFallbackLiteral(source.Fallback))
	case action.SourcePoint:
		return fmt.Sprintf("resolveValue(%s, %s)", pointValueJS(source.Name, fields), jsFallbackLiteral(source.Fallback))
	case action.SourceVariable:
		return fmt.Sprintf("resolveValue(variables[%q], %s)", source.Name, jsFallbackLiteral(source.Fallback))
	case action.SourceLiteral:
		return js.MustToJS(source.Value)
	default:
		return "undefined"
	}
}

func pointValueJS(name string, fields panel.FieldMapping) string {
	switch name {
	case "label":
		return fmt.Sprintf("row[%q] || row[%q] || categoryName", fields.Label.Name(), fields.Category.Name())
	case "value":
		return fmt.Sprintf("row[%q]", fields.Value.Name())
	case "series":
		return "seriesName"
	case "category":
		return "categoryName"
	default:
		return fmt.Sprintf("row[%q]", name)
	}
}

func jsFallbackLiteral(value any) string {
	if value == nil {
		return "undefined"
	}
	return js.MustToJS(value)
}

type chartActionConfig struct {
	Rows           []map[string]any `json:"rows"`
	Variables      map[string]any   `json:"variables"`
	URL            string           `json:"url"`
	Method         string           `json:"method,omitempty"`
	Target         string           `json:"target,omitempty"`
	Event          string           `json:"event,omitempty"`
	CategoryField  string           `json:"categoryField"`
	LabelField     string           `json:"labelField"`
	StartTimeField string           `json:"startTimeField"`
	SeriesField    string           `json:"seriesField"`
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

func panelHeight(panelSpec panel.Spec) string {
	if panelSpec.Height != "" {
		return panelSpec.Height
	}
	return "320px"
}

func panelColors(panelSpec panel.Spec) []string {
	if len(panelSpec.Colors) > 0 {
		return panelSpec.Colors
	}
	switch panelSpec.Kind {
	case panel.KindTimeSeries:
		return []string{"#3b82f6"} // blue-500 — consistent primary
	case panel.KindStackedBar:
		return []string{"#3b82f6", "#10b981", "#f59e0b", "#8b5cf6", "#ef4444", "#06b6d4", "#6366f1", "#14b8a6"}
	case panel.KindPie, panel.KindDonut:
		return []string{"#3b82f6", "#10b981", "#f59e0b", "#8b5cf6", "#ef4444", "#06b6d4", "#6366f1", "#14b8a6"}
	case panel.KindGauge:
		return []string{"#3b82f6"} // blue-500 — unified accent
	case panel.KindStat, panel.KindBar, panel.KindHorizontalBar, panel.KindTable, panel.KindTabs, panel.KindGrid, panel.KindSplit, panel.KindRepeat:
		return []string{"#3b82f6"} // blue-500
	}
	return []string{"#3b82f6"}
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
