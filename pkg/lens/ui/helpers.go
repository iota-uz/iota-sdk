package ui

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/components/charts"
	"github.com/iota-uz/iota-sdk/pkg/lens"
)

// panelHeight returns the chart height for a panel, with a default.
func panelHeight(p lens.Panel) string {
	if p.Options.Height != "" {
		return p.Options.Height
	}
	return "320px"
}

// spanClass returns the Tailwind col-span class for a panel.
func spanClass(span int) string {
	if span <= 0 || span > 12 {
		span = 6
	}
	return fmt.Sprintf("col-span-12 md:col-span-%d", span)
}

// --- Metric helpers ---

// MetricData holds the extracted metric value from a query result.
type MetricData struct {
	Value          float64
	FormattedValue string
	HasData        bool
}

// extractMetric reads the first row's "value" column from the result.
func extractMetric(result *lens.QueryResult) MetricData {
	if result == nil || len(result.Rows) == 0 {
		return MetricData{}
	}
	row := result.Rows[0]
	val, ok := row["value"]
	if !ok {
		// Try first numeric column
		for _, v := range row {
			if f, ok := toFloat64(v); ok {
				return MetricData{Value: f, FormattedValue: formatNumber(f), HasData: true}
			}
		}
		return MetricData{}
	}
	f, ok := toFloat64(val)
	if !ok {
		return MetricData{}
	}
	return MetricData{Value: f, FormattedValue: formatNumber(f), HasData: true}
}

// formatMetricDisplay formats a metric value with optional prefix and unit.
func formatMetricDisplay(m MetricData, p lens.Panel) string {
	if !m.HasData {
		return "-"
	}
	s := m.FormattedValue
	if p.Options.Prefix != "" {
		s = p.Options.Prefix + s
	}
	if p.Options.Unit != "" {
		s = s + " " + p.Options.Unit
	}
	return s
}

// --- Chart helpers ---

// buildChartOptions converts a panel + query result into ApexCharts options.
func buildChartOptions(p lens.Panel, result *lens.QueryResult) charts.ChartOptions {
	chartType := panelTypeToChartType(p.Type)

	opts := charts.ChartOptions{
		Chart: charts.ChartConfig{
			Type:    chartType,
			Height:  panelHeight(p),
			Toolbar: charts.Toolbar{Show: false},
			Stacked: p.Options.Stacked,
		},
		DataLabels: &charts.DataLabels{Enabled: false},
		Colors:     panelColors(p),
	}

	if result == nil || len(result.Rows) == 0 {
		return opts
	}

	switch p.Type {
	case lens.TypePie, lens.TypeDonut:
		opts.Series = buildPieSeries(result)
		opts.Labels = buildLabels(result)
		addPieOptions(&opts, p)
	case lens.TypeGauge:
		opts.Series = buildPieSeries(result)
		opts.Labels = buildLabels(result)
		addGaugeOptions(&opts)
	case lens.TypeStackedBar:
		opts.Series = buildStackedSeries(result)
		opts.XAxis = charts.XAxisConfig{Categories: buildCategories(result)}
		addBarOptions(&opts)
		opts.Chart.Stacked = true
	case lens.TypeLine:
		opts.Series = buildSeries(result)
		opts.XAxis = charts.XAxisConfig{Categories: buildLabels(result)}
		addLineOptions(&opts)
	case lens.TypeArea:
		opts.Series = buildSeries(result)
		opts.XAxis = charts.XAxisConfig{Categories: buildLabels(result)}
		addAreaOptions(&opts)
	default: // bar, column, etc.
		opts.Series = buildSeries(result)
		opts.XAxis = charts.XAxisConfig{Categories: buildLabels(result)}
		addBarOptions(&opts)
	}

	if p.Options.ShowLegend {
		pos := charts.LegendPositionBottom
		opts.Legend = &charts.LegendConfig{Position: &pos}
	}

	// Add drill-down event handler
	if p.Options.DrillDown != nil {
		addDrillDownEvents(&opts, p)
	}

	return opts
}

func panelTypeToChartType(t lens.PanelType) charts.ChartType {
	switch t {
	case lens.TypeLine:
		return charts.LineChartType
	case lens.TypeBar, lens.TypeStackedBar, lens.TypeColumn:
		return charts.BarChartType
	case lens.TypePie:
		return charts.PieChartType
	case lens.TypeDonut:
		return charts.DonutChartType
	case lens.TypeArea:
		return charts.AreaChartType
	case lens.TypeGauge:
		return charts.RadialBarChartType
	default:
		return charts.LineChartType
	}
}

func panelColors(p lens.Panel) []string {
	if len(p.Options.Colors) > 0 {
		return p.Options.Colors
	}
	if p.Options.Color != "" {
		return []string{p.Options.Color}
	}
	switch p.Type {
	case lens.TypeLine:
		return []string{"#10b981"}
	case lens.TypeBar, lens.TypeColumn:
		return []string{"#3b82f6"}
	case lens.TypeStackedBar:
		return []string{"#3b82f6", "#10b981", "#f59e0b", "#ef4444", "#8b5cf6", "#06b6d4"}
	case lens.TypePie, lens.TypeDonut:
		return []string{"#10b981", "#3b82f6", "#f59e0b", "#ef4444", "#8b5cf6"}
	case lens.TypeArea:
		return []string{"#06b6d4"}
	case lens.TypeGauge:
		return []string{"#f59e0b"}
	default:
		return []string{"#6b7280"}
	}
}

// buildSeries creates a single series from query result.
// Expects columns: label/category, value
func buildSeries(result *lens.QueryResult) []charts.Series {
	data := make([]interface{}, 0, len(result.Rows))
	for _, row := range result.Rows {
		if v, ok := row["value"]; ok {
			data = append(data, v)
		} else {
			data = append(data, 0)
		}
	}
	return []charts.Series{{Name: "Data", Data: data}}
}

// buildPieSeries creates a flat value array for pie/donut/gauge charts.
func buildPieSeries(result *lens.QueryResult) []interface{} {
	data := make([]interface{}, 0, len(result.Rows))
	for _, row := range result.Rows {
		if v, ok := row["value"]; ok {
			data = append(data, v)
		}
	}
	return data
}

// buildStackedSeries groups rows by "series" column for stacked charts.
// Expects columns: category, series, value
func buildStackedSeries(result *lens.QueryResult) []charts.Series {
	// Preserve insertion order for series names.
	var seriesOrder []string
	seriesData := make(map[string][]interface{})

	for _, row := range result.Rows {
		name := "Series"
		if s, ok := row["series"]; ok {
			name = fmt.Sprintf("%v", s)
		}
		if _, exists := seriesData[name]; !exists {
			seriesOrder = append(seriesOrder, name)
		}
		val := row["value"]
		if val == nil {
			val = 0
		}
		seriesData[name] = append(seriesData[name], val)
	}

	series := make([]charts.Series, 0, len(seriesOrder))
	for _, name := range seriesOrder {
		series = append(series, charts.Series{Name: name, Data: seriesData[name]})
	}
	return series
}

// buildCategories extracts unique category values for stacked charts.
func buildCategories(result *lens.QueryResult) []string {
	seen := make(map[string]bool)
	var cats []string
	for _, row := range result.Rows {
		cat := ""
		if v, ok := row["category"]; ok {
			cat = fmt.Sprintf("%v", v)
		} else if v, ok := row["label"]; ok {
			cat = fmt.Sprintf("%v", v)
		}
		if cat != "" && !seen[cat] {
			seen[cat] = true
			cats = append(cats, cat)
		}
	}
	return cats
}

// buildLabels extracts label values for chart x-axis categories.
func buildLabels(result *lens.QueryResult) []string {
	labels := make([]string, 0, len(result.Rows))
	for _, row := range result.Rows {
		label := ""
		if v, ok := row["label"]; ok {
			label = formatCellValue(v)
		} else if v, ok := row["category"]; ok {
			label = formatCellValue(v)
		} else if v, ok := row["name"]; ok {
			label = formatCellValue(v)
		} else if v, ok := row["timestamp"]; ok {
			label = formatCellValue(v)
		} else if v, ok := row["date"]; ok {
			label = formatCellValue(v)
		} else {
			// Use first string column
			for _, col := range result.Columns {
				if col.Type == "string" || col.Type == "timestamp" {
					if v, ok := row[col.Name]; ok {
						label = formatCellValue(v)
						break
					}
				}
			}
		}
		labels = append(labels, label)
	}
	return labels
}

// --- Chart-specific option helpers ---

func addLineOptions(opts *charts.ChartOptions) {
	opts.Stroke = &charts.StrokeConfig{Curve: charts.StrokeCurveSmooth, Width: 2}
	opts.Markers = &charts.MarkersConfig{Size: 4}
}

func addAreaOptions(opts *charts.ChartOptions) {
	opts.Stroke = &charts.StrokeConfig{Curve: charts.StrokeCurveSmooth}
	opts.Fill = &charts.FillConfig{
		Type: charts.FillTypeGradient,
		Gradient: &charts.FillGradient{
			ShadeIntensity: floatPtr(1),
			OpacityFrom:    floatPtr(0.7),
			OpacityTo:      floatPtr(0.3),
		},
	}
}

func addBarOptions(opts *charts.ChartOptions) {
	opts.PlotOptions = &charts.PlotOptions{
		Bar: &charts.BarConfig{ColumnWidth: "55%", BorderRadius: 2},
	}
}

func addPieOptions(opts *charts.ChartOptions, p lens.Panel) {
	pos := charts.LegendPositionBottom
	opts.Legend = &charts.LegendConfig{Position: &pos}
}

func addGaugeOptions(opts *charts.ChartOptions) {
	startAngle := -135
	endAngle := 225
	size := "70%"
	opts.PlotOptions = &charts.PlotOptions{
		RadialBar: &charts.RadialBarConfig{
			StartAngle: &startAngle,
			EndAngle:   &endAngle,
			Hollow:     &charts.RadialBarHollow{Size: &size},
		},
	}
}

func addDrillDownEvents(opts *charts.ChartOptions, p lens.Panel) {
	dd := p.Options.DrillDown
	if dd == nil {
		return
	}

	// Escape the URL template for safe embedding in JS.
	urlTemplate, _ := json.Marshal(dd.URL)

	var js string
	if dd.Target != "" {
		// Use HTMX for partial page update
		js = fmt.Sprintf(`function(event, chartContext, opts) {
			var labels = chartContext.w.config.xaxis && chartContext.w.config.xaxis.categories ? chartContext.w.config.xaxis.categories : (chartContext.w.config.labels || []);
			var idx = opts.dataPointIndex !== undefined ? opts.dataPointIndex : (opts.seriesIndex !== undefined ? opts.seriesIndex : -1);
			if (idx < 0) return;
			var label = labels[idx] || '';
			var url = %s.replace('{label}', encodeURIComponent(label));
			for (var key in opts.w.config.series) {
				url = url.replace('{' + key + '}', encodeURIComponent(opts.w.config.series[key]));
			}
			htmx.ajax('GET', url, {target: %q, swap: 'innerHTML'});
		}`, string(urlTemplate), dd.Target)
	} else {
		// Full page navigation
		js = fmt.Sprintf(`function(event, chartContext, opts) {
			var labels = chartContext.w.config.xaxis && chartContext.w.config.xaxis.categories ? chartContext.w.config.xaxis.categories : (chartContext.w.config.labels || []);
			var idx = opts.dataPointIndex !== undefined ? opts.dataPointIndex : (opts.seriesIndex !== undefined ? opts.seriesIndex : -1);
			if (idx < 0) return;
			var label = labels[idx] || '';
			var url = %s.replace('{label}', encodeURIComponent(label));
			window.location.href = url;
		}`, string(urlTemplate))
	}

	opts.Chart.Events = &charts.ChartEvents{
		DataPointSelection: templ.JSExpression(js),
	}
}

// --- Table helpers ---

// tableColumns returns column definitions, falling back to query result columns.
func tableColumns(p lens.Panel, result *lens.QueryResult) []lens.TableColumn {
	if len(p.Options.Columns) > 0 {
		return p.Options.Columns
	}
	if result == nil {
		return nil
	}
	cols := make([]lens.TableColumn, len(result.Columns))
	for i, c := range result.Columns {
		cols[i] = lens.TableColumn{Key: c.Name, Label: c.Name, Format: c.Type}
	}
	return cols
}

// drillDownURL builds a concrete URL from a template and row data.
func drillDownURL(tmpl string, row map[string]any) string {
	result := tmpl
	for k, v := range row {
		placeholder := "{" + k + "}"
		result = strings.ReplaceAll(result, placeholder, url.QueryEscape(fmt.Sprintf("%v", v)))
	}
	return result
}

// --- Formatting helpers ---

func formatCellValue(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case time.Time:
		return val.Format("2006-01-02")
	case float64:
		if val == math.Trunc(val) {
			return fmt.Sprintf("%.0f", val)
		}
		return fmt.Sprintf("%.2f", val)
	case float32:
		f := float64(val)
		if f == math.Trunc(f) {
			return fmt.Sprintf("%.0f", f)
		}
		return fmt.Sprintf("%.2f", f)
	case int, int32, int64:
		return fmt.Sprintf("%d", val)
	case bool:
		if val {
			return "Yes"
		}
		return "No"
	default:
		return fmt.Sprintf("%v", val)
	}
}

func formatNumber(v float64) string {
	abs := v
	if abs < 0 {
		abs = -abs
	}
	switch {
	case abs >= 1_000_000_000:
		return fmt.Sprintf("%.1fB", v/1_000_000_000)
	case abs >= 1_000_000:
		return fmt.Sprintf("%.1fM", v/1_000_000)
	case abs >= 1_000:
		return fmt.Sprintf("%.1fK", v/1_000)
	case abs >= 1:
		return fmt.Sprintf("%.0f", v)
	default:
		return fmt.Sprintf("%.2f", v)
	}
}

func toFloat64(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	default:
		return 0, false
	}
}

func floatPtr(f float64) *float64 {
	return &f
}
