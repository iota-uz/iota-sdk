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

// ---------------------------------------------------------------------------
// Tailwind grid helpers
// ---------------------------------------------------------------------------

// spanClasses maps column spans to static Tailwind classes.
// These must be spelled out literally so Tailwind's JIT scanner can find them.
var spanClasses = map[int]string{
	1:  "col-span-12 md:col-span-1",
	2:  "col-span-12 md:col-span-2",
	3:  "col-span-12 md:col-span-3",
	4:  "col-span-12 md:col-span-4",
	5:  "col-span-12 md:col-span-5",
	6:  "col-span-12 md:col-span-6",
	7:  "col-span-12 md:col-span-7",
	8:  "col-span-12 md:col-span-8",
	9:  "col-span-12 md:col-span-9",
	10: "col-span-12 md:col-span-10",
	11: "col-span-12 md:col-span-11",
	12: "col-span-12 md:col-span-12",
}

// spanClass returns the Tailwind col-span class for a panel.
func spanClass(span int) string {
	if c, ok := spanClasses[span]; ok {
		return c
	}
	return spanClasses[6]
}

// ---------------------------------------------------------------------------
// Panel data accessors
// ---------------------------------------------------------------------------

// panelData safely extracts the PanelResult from Results for a given panel.
func panelData(p lens.Panel, results *lens.Results) *lens.PanelResult {
	if results == nil || results.Panels == nil {
		return nil
	}
	return results.Panels[p.ID]
}

// resultData safely extracts the QueryResult from a PanelResult.
func resultData(pr *lens.PanelResult) *lens.QueryResult {
	if pr == nil {
		return nil
	}
	return pr.Data
}

// ---------------------------------------------------------------------------
// Column resolution
// ---------------------------------------------------------------------------

// resolveColumn returns the value for a named column from a row.
// It first checks the explicit ColumnMap mapping, then falls back to
// convention-based detection across common column names.
func resolveColumn(row map[string]any, mapName string, fallbacks []string) (any, bool) {
	// Try the explicit mapping first.
	if mapName != "" {
		v, ok := row[mapName]
		return v, ok
	}
	// Fall back to conventions.
	for _, name := range fallbacks {
		if v, ok := row[name]; ok {
			return v, true
		}
	}
	return nil, false
}

var (
	labelFallbacks    = []string{"label", "category", "name", "timestamp", "date"}
	valueFallbacks    = []string{"value", "amount", "count", "total"}
	seriesFallbacks   = []string{"series", "group"}
	categoryFallbacks = []string{"category", "label"}
)

// resolveLabel returns the label value for a row using column map + fallbacks.
func resolveLabel(row map[string]any, cm lens.ColumnMap) string {
	v, ok := resolveColumn(row, cm.Label, labelFallbacks)
	if !ok {
		return ""
	}
	return formatCellValue(v)
}

// resolveValue returns the numeric value for a row using column map + fallbacks.
func resolveValue(row map[string]any, cm lens.ColumnMap) (any, bool) {
	return resolveColumn(row, cm.Value, valueFallbacks)
}

// resolveSeries returns the series name for a row using column map + fallbacks.
func resolveSeries(row map[string]any, cm lens.ColumnMap) (string, bool) {
	v, ok := resolveColumn(row, cm.Series, seriesFallbacks)
	if !ok {
		return "", false
	}
	return fmt.Sprintf("%v", v), true
}

// resolveCategory returns the category value for a row using column map + fallbacks.
func resolveCategory(row map[string]any, cm lens.ColumnMap) (string, bool) {
	v, ok := resolveColumn(row, cm.Category, categoryFallbacks)
	if !ok {
		return "", false
	}
	return fmt.Sprintf("%v", v), true
}

// ---------------------------------------------------------------------------
// Drill-down URL resolution (unified for charts and tables)
// ---------------------------------------------------------------------------

// resolveDrillURL substitutes all {column_name} placeholders in a URL template
// with URL-encoded values from the given data row.
func resolveDrillURL(tmpl string, row map[string]any) string {
	result := tmpl
	for k, v := range row {
		placeholder := "{" + k + "}"
		if strings.Contains(result, placeholder) {
			result = strings.ReplaceAll(result, placeholder, url.QueryEscape(fmt.Sprintf("%v", v)))
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// Metric helpers
// ---------------------------------------------------------------------------

// MetricData holds the extracted metric value from a query result.
type MetricData struct {
	Value          float64
	FormattedValue string
	HasData        bool
}

// extractMetric reads the metric value from the first row of the result.
func extractMetric(result *lens.QueryResult, cm lens.ColumnMap) MetricData {
	if result == nil || len(result.Rows) == 0 {
		return MetricData{}
	}
	row := result.Rows[0]
	v, ok := resolveValue(row, cm)
	if !ok {
		// Last resort: try first numeric value in the row.
		for _, val := range row {
			if f, fOk := toFloat64(val); fOk {
				return MetricData{Value: f, FormattedValue: formatNumber(f), HasData: true}
			}
		}
		return MetricData{}
	}
	f, fOk := toFloat64(v)
	if !fOk {
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
	if p.Metric != nil {
		if p.Metric.Prefix != "" {
			s = p.Metric.Prefix + s
		}
		if p.Metric.Unit != "" {
			s = s + " " + p.Metric.Unit
		}
	}
	return s
}

// metricColor returns the accent color for a metric panel.
func metricColor(p lens.Panel) string {
	if p.Metric != nil && p.Metric.Color != "" {
		return p.Metric.Color
	}
	return ""
}

// ---------------------------------------------------------------------------
// Chart building
// ---------------------------------------------------------------------------

// panelHeight returns the chart height for a panel, with a default.
func panelHeight(p lens.Panel) string {
	if p.Chart != nil && p.Chart.Height != "" {
		return p.Chart.Height
	}
	return "320px"
}

// buildChartOptions converts a panel + query result into ApexCharts options.
func buildChartOptions(p lens.Panel, result *lens.QueryResult) charts.ChartOptions {
	chartType := panelTypeToChartType(p.Type)
	stacked := p.Chart != nil && p.Chart.Stacked

	opts := charts.ChartOptions{
		Chart: charts.ChartConfig{
			Type:    chartType,
			Height:  panelHeight(p),
			Toolbar: charts.Toolbar{Show: false},
			Stacked: stacked,
		},
		DataLabels: &charts.DataLabels{Enabled: false},
		Colors:     panelColors(p),
	}

	if result == nil || len(result.Rows) == 0 {
		return opts
	}

	cm := p.ColumnMap

	switch p.Type {
	case lens.TypePie, lens.TypeDonut:
		opts.Series = buildPieSeries(result, cm)
		opts.Labels = buildLabels(result, cm)
		addPieOptions(&opts)
	case lens.TypeGauge:
		opts.Series = buildPieSeries(result, cm)
		opts.Labels = buildLabels(result, cm)
		addGaugeOptions(&opts)
	case lens.TypeStackedBar:
		opts.Series = buildGroupedSeries(result, cm)
		opts.XAxis = charts.XAxisConfig{Categories: buildUniqueCategories(result, cm)}
		addBarOptions(&opts)
		opts.Chart.Stacked = true
	case lens.TypeLine, lens.TypeArea:
		// Multi-series: if a series column exists, group by it.
		if hasSeriesColumn(result, cm) {
			opts.Series = buildGroupedSeries(result, cm)
			opts.XAxis = charts.XAxisConfig{Categories: buildUniqueCategories(result, cm)}
		} else {
			opts.Series = buildSingleSeries(result, cm)
			opts.XAxis = charts.XAxisConfig{Categories: buildLabels(result, cm)}
		}
		if p.Type == lens.TypeLine {
			addLineOptions(&opts)
		} else {
			addAreaOptions(&opts)
		}
	case lens.TypeBar, lens.TypeColumn:
		if hasSeriesColumn(result, cm) {
			opts.Series = buildGroupedSeries(result, cm)
			opts.XAxis = charts.XAxisConfig{Categories: buildUniqueCategories(result, cm)}
		} else {
			opts.Series = buildSingleSeries(result, cm)
			opts.XAxis = charts.XAxisConfig{Categories: buildLabels(result, cm)}
		}
		addBarOptions(&opts)
	case lens.TypeMetric, lens.TypeTable:
		// Not chart types — should not reach here.
		return opts
	}

	if p.Chart != nil && p.Chart.ShowLegend {
		pos := charts.LegendPositionBottom
		opts.Legend = &charts.LegendConfig{Position: &pos}
	}

	if p.DrillDown != nil {
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
	case lens.TypeMetric, lens.TypeTable:
		return charts.LineChartType
	}
	return charts.LineChartType
}

func panelColors(p lens.Panel) []string {
	if p.Chart != nil && len(p.Chart.Colors) > 0 {
		return p.Chart.Colors
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
	case lens.TypeMetric, lens.TypeTable:
		return []string{"#6b7280"}
	}
	return []string{"#6b7280"}
}

// ---------------------------------------------------------------------------
// Series builders (column-map aware)
// ---------------------------------------------------------------------------

// hasSeriesColumn returns true if any row has a resolvable series column.
func hasSeriesColumn(result *lens.QueryResult, cm lens.ColumnMap) bool {
	if len(result.Rows) == 0 {
		return false
	}
	_, ok := resolveSeries(result.Rows[0], cm)
	return ok
}

// buildSingleSeries creates a single series from the query result.
func buildSingleSeries(result *lens.QueryResult, cm lens.ColumnMap) []charts.Series {
	data := make([]interface{}, 0, len(result.Rows))
	for _, row := range result.Rows {
		if v, ok := resolveValue(row, cm); ok {
			data = append(data, v)
		} else {
			data = append(data, 0)
		}
	}
	return []charts.Series{{Name: "Data", Data: data}}
}

// buildGroupedSeries groups rows by series column for multi-series/stacked charts.
func buildGroupedSeries(result *lens.QueryResult, cm lens.ColumnMap) []charts.Series {
	var seriesOrder []string
	seriesData := make(map[string][]interface{})

	for _, row := range result.Rows {
		name := "Series"
		if s, ok := resolveSeries(row, cm); ok {
			name = s
		}
		if _, exists := seriesData[name]; !exists {
			seriesOrder = append(seriesOrder, name)
		}
		val, _ := resolveValue(row, cm)
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

// buildPieSeries creates a flat value array for pie/donut/gauge charts.
func buildPieSeries(result *lens.QueryResult, cm lens.ColumnMap) []interface{} {
	data := make([]interface{}, 0, len(result.Rows))
	for _, row := range result.Rows {
		if v, ok := resolveValue(row, cm); ok {
			data = append(data, v)
		}
	}
	return data
}

// buildLabels extracts label values for chart x-axis categories.
func buildLabels(result *lens.QueryResult, cm lens.ColumnMap) []string {
	labels := make([]string, 0, len(result.Rows))
	for _, row := range result.Rows {
		labels = append(labels, resolveLabel(row, cm))
	}
	return labels
}

// buildUniqueCategories extracts unique category values for grouped charts.
func buildUniqueCategories(result *lens.QueryResult, cm lens.ColumnMap) []string {
	seen := make(map[string]bool)
	var cats []string
	for _, row := range result.Rows {
		cat, ok := resolveCategory(row, cm)
		if !ok {
			cat = resolveLabel(row, cm)
		}
		if cat != "" && !seen[cat] {
			seen[cat] = true
			cats = append(cats, cat)
		}
	}
	return cats
}

// ---------------------------------------------------------------------------
// Chart-specific option helpers
// ---------------------------------------------------------------------------

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

func addPieOptions(opts *charts.ChartOptions) {
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

// addDrillDownEvents wires up chart click events to navigate via the drill-down
// URL template. All {column_name} placeholders in the URL are substituted from
// the row data — not just {label}.
func addDrillDownEvents(opts *charts.ChartOptions, p lens.Panel) {
	dd := p.DrillDown
	if dd == nil {
		return
	}

	urlTemplate, err := json.Marshal(dd.URL)
	if err != nil {
		return
	}

	// Build a JS snippet that constructs a data object from the clicked point,
	// then substitutes all {key} placeholders in the URL template.
	replacerJS := `
		var url = urlTmpl;
		for (var key in dataObj) {
			url = url.replace('{' + key + '}', encodeURIComponent(dataObj[key]));
		}
	`

	var actionJS string
	if dd.Target != "" {
		actionJS = fmt.Sprintf(`htmx.ajax('GET', url, {target: %q, swap: 'innerHTML'});`, dd.Target)
	} else {
		actionJS = `window.location.href = url;`
	}

	js := fmt.Sprintf(`function(event, chartContext, opts) {
		var cfg = chartContext.w.config;
		var labels = (cfg.xaxis && cfg.xaxis.categories) ? cfg.xaxis.categories : (cfg.labels || []);
		var idx = opts.dataPointIndex !== undefined && opts.dataPointIndex >= 0
			? opts.dataPointIndex
			: (opts.seriesIndex !== undefined ? opts.seriesIndex : -1);
		if (idx < 0) return;

		var seriesName = '';
		if (cfg.series && cfg.series[opts.seriesIndex] && cfg.series[opts.seriesIndex].name) {
			seriesName = cfg.series[opts.seriesIndex].name;
		}
		var value = '';
		if (cfg.series && cfg.series[opts.seriesIndex]) {
			var s = cfg.series[opts.seriesIndex];
			if (s.data && s.data[opts.dataPointIndex] !== undefined) {
				value = s.data[opts.dataPointIndex];
			} else if (typeof s === 'number') {
				value = s;
			}
		}

		var dataObj = {
			label: labels[idx] || '',
			value: value,
			series: seriesName,
			category: labels[idx] || '',
			index: idx
		};

		var urlTmpl = %s;
		%s
		%s
	}`, string(urlTemplate), replacerJS, actionJS)

	opts.Chart.Events = &charts.ChartEvents{
		DataPointSelection: templ.JSExpression(js),
	}
}

// ---------------------------------------------------------------------------
// Table helpers
// ---------------------------------------------------------------------------

// tableColumns returns column definitions, falling back to query result columns.
func tableColumns(p lens.Panel, result *lens.QueryResult) []lens.TableColumn {
	if p.Table != nil && len(p.Table.Columns) > 0 {
		return p.Table.Columns
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

// ---------------------------------------------------------------------------
// Formatting helpers
// ---------------------------------------------------------------------------

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
