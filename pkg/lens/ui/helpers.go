package ui

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/components/charts"
	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/evaluation"
	"github.com/iota-uz/iota-sdk/pkg/lens/executor"
)

// Helper functions for templ components

func generateGridCSS(layout *evaluation.Layout) string {
	css := "display: grid; "
	css += "grid-template-columns: " + layout.CSS.GridTemplate.Columns + "; "
	css += "grid-auto-rows: " + layout.CSS.GridTemplate.Rows + "; "
	css += "gap: 1rem; "
	css += "padding: 1rem; "
	return css
}

func generateLayoutCSS(layout *evaluation.Layout) string {
	css := "display: grid; "
	css += "grid-template-columns: " + layout.CSS.GridTemplate.Columns + "; "
	css += "grid-auto-rows: " + layout.CSS.GridTemplate.Rows + "; "
	css += "gap: 1rem; "
	return css
}

func generatePanelGridCSS(panel *evaluation.EvaluatedPanel) string {
	// For desktop and larger screens, use CSS custom properties for positioning
	// Mobile layout is handled by responsive CSS classes
	pos := panel.Config.Position
	dim := panel.Config.Dimensions

	return fmt.Sprintf("--panel-grid-area: %d / %d / %d / %d;",
		pos.Y+1, pos.X+1, pos.Y+dim.Height+1, pos.X+dim.Width+1)
}

func generateConfigPanelGridCSS(config lens.PanelConfig) string {
	// For desktop and larger screens, use CSS custom properties for positioning
	// Mobile layout is handled by responsive CSS classes
	pos := config.Position
	dim := config.Dimensions

	return fmt.Sprintf("--panel-grid-area: %d / %d / %d / %d;",
		pos.Y+1, pos.X+1, pos.Y+dim.Height+1, pos.X+dim.Width+1)
}

func generateDashboardGridCSS(config lens.DashboardConfig) string {
	// Return minimal inline styles - responsive layout is handled by CSS classes
	return ""
}

func formatValue(value interface{}) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return v
	case int, int32, int64:
		return fmt.Sprintf("%d", v)
	case float32, float64:
		return fmt.Sprintf("%.2f", v)
	case bool:
		return strconv.FormatBool(v)
	case time.Time:
		return v.Format("2006-01-02 15:04:05")
	default:
		return fmt.Sprintf("%v", v)
	}
}

func buildChartOptionsFromPanel(panel *evaluation.EvaluatedPanel) charts.ChartOptions {
	// Build chart options from evaluated panel
	options := charts.ChartOptions{
		Chart: charts.ChartConfig{
			Type:    convertLensToChartsType(panel.Config.Type),
			Height:  "100%",
			Toolbar: charts.Toolbar{Show: false},
		},
		Series: []charts.Series{
			{
				Name: panel.Config.Title,
				Data: []interface{}{}, // Will be populated via HTMX
			},
		},
		DataLabels: &charts.DataLabels{Enabled: false},
		Colors:     []string{"#10b981"},
	}

	// Apply custom options from panel config
	mergeCustomOptions(&options, panel.Config.Options)

	return options
}

func buildChartOptionsFromResult(config lens.PanelConfig, result *executor.ExecutionResult) charts.ChartOptions {
	// Build chart options from executor result data
	chartType := convertLensToChartsType(config.Type)

	options := charts.ChartOptions{
		Chart: charts.ChartConfig{
			Type:    chartType,
			Height:  "100%",
			Toolbar: charts.Toolbar{Show: false},
			Stacked: config.Type == lens.ChartTypeStackedBar,
		},
		DataLabels: &charts.DataLabels{Enabled: false},
		Colors:     getChartColors(config.Type),
	}

	// Add event handlers if configured
	addEventHandlers(&options, config)

	// Handle pie and gauge charts differently
	if config.Type == lens.ChartTypePie {
		options.Series = buildPieSeriesFromResult(result)
		options.Labels = buildCategoriesFromResult(result)
	} else if config.Type == lens.ChartTypeGauge {
		// Gauge charts (radial bars) use pie-like series format
		options.Series = buildPieSeriesFromResult(result)
		options.Labels = buildCategoriesFromResult(result)
	} else if config.Type == lens.ChartTypeStackedBar {
		options.Series = buildStackedSeriesFromResult(result)
		options.XAxis = charts.XAxisConfig{
			Categories: buildCategoriesFromResult(result),
		}
	} else {
		options.Series = buildSeriesFromResult(result)
		options.XAxis = charts.XAxisConfig{
			Categories: buildCategoriesFromResult(result),
		}
	}

	// Add chart-specific options
	addChartSpecificOptions(&options, config.Type)

	// Apply custom options from panel config
	mergeCustomOptions(&options, config.Options)

	return options
}

func convertLensToChartsType(lensType lens.ChartType) charts.ChartType {
	switch lensType {
	case lens.ChartTypeLine:
		return charts.LineChartType
	case lens.ChartTypeBar:
		return charts.BarChartType
	case lens.ChartTypeStackedBar:
		return charts.BarChartType
	case lens.ChartTypeColumn:
		return charts.BarChartType
	case lens.ChartTypePie:
		return charts.PieChartType
	case lens.ChartTypeArea:
		return charts.AreaChartType
	case lens.ChartTypeGauge:
		return charts.RadialBarChartType
	case lens.ChartTypeTable:
		return charts.LineChartType // Table doesn't need a chart type
	case lens.ChartTypeMetric:
		return charts.LineChartType // Metric doesn't need a chart type
	default:
		return charts.LineChartType
	}
}

func buildSeriesFromResult(result *executor.ExecutionResult) []charts.Series {
	if len(result.Data) == 0 {
		return []charts.Series{}
	}

	dataPoints := make([]interface{}, 0, len(result.Data))
	for _, point := range result.Data {
		// For time-series format, use the Value field
		if point.Value != nil {
			dataPoints = append(dataPoints, point.Value)
		} else {
			// For table format, look for 'value' in Fields
			if val, exists := point.Fields["value"]; exists {
				dataPoints = append(dataPoints, val)
			} else {
				// Fallback: use 0 if no value found
				dataPoints = append(dataPoints, 0)
			}
		}
	}

	return []charts.Series{
		{
			Name: "Data",
			Data: dataPoints,
		},
	}
}

func buildPieSeriesFromResult(result *executor.ExecutionResult) []interface{} {
	if len(result.Data) == 0 {
		return []interface{}{}
	}

	dataPoints := make([]interface{}, 0, len(result.Data))
	for _, point := range result.Data {
		// For time-series format, use the Value field
		if point.Value != nil {
			dataPoints = append(dataPoints, point.Value)
		} else {
			// For table format, look for 'value' in Fields
			if val, exists := point.Fields["value"]; exists {
				dataPoints = append(dataPoints, val)
			} else {
				// Fallback: use 0 if no value found
				dataPoints = append(dataPoints, 0)
			}
		}
	}

	return dataPoints
}

// buildStackedSeriesFromResult builds multiple series for stacked bar charts.
// For stacked charts, the query should return data in format:
// SELECT category, series_name, value FROM ... ORDER BY category, series_name
func buildStackedSeriesFromResult(result *executor.ExecutionResult) []charts.Series {
	if len(result.Data) == 0 {
		return []charts.Series{}
	}

	// Group data by series name
	seriesMap := make(map[string][]interface{})
	categorySet := make(map[string]bool)

	for _, point := range result.Data {
		var category, seriesName string
		var value interface{}

		// Extract category (x-axis)
		if cat, exists := point.Fields["category"]; exists {
			if strCat, ok := cat.(string); ok {
				category = strCat
			}
		} else if cat, exists := point.Labels["category"]; exists {
			category = cat
		}

		// Extract series name (for grouping)
		if series, exists := point.Fields["series"]; exists {
			if strSeries, ok := series.(string); ok {
				seriesName = strSeries
			}
		} else if series, exists := point.Labels["series"]; exists {
			seriesName = series
		} else {
			seriesName = "Series 1" // Default series name
		}

		// Extract value
		if point.Value != nil {
			value = point.Value
		} else if val, exists := point.Fields["value"]; exists {
			value = val
		} else {
			value = 0
		}

		// Track category (for deduplication)
		if !categorySet[category] {
			categorySet[category] = true
		}

		// Initialize series if it doesn't exist
		if _, exists := seriesMap[seriesName]; !exists {
			seriesMap[seriesName] = make([]interface{}, 0)
		}

		seriesMap[seriesName] = append(seriesMap[seriesName], value)
	}

	// Convert map to slice
	series := make([]charts.Series, 0, len(seriesMap))
	for seriesName, data := range seriesMap {
		series = append(series, charts.Series{
			Name: seriesName,
			Data: data,
		})
	}

	return series
}

// buildCategoriesFromResult extracts category labels from query results.
//
// Standard Query Format:
// - For categorical charts (bar, pie): SELECT label, value FROM ...
// - For time-series charts (line, area): SELECT timestamp, value FROM ...
//
// The 'label' column should contain string values for chart categories.
// The 'timestamp' column should contain time values for time-series data.
func buildCategoriesFromResult(result *executor.ExecutionResult) []string {
	categories := make([]string, 0, len(result.Data))

	for _, point := range result.Data {
		var category string
		found := false

		// First check Labels map (for time-series format)
		if cat, exists := point.Labels["label"]; exists {
			category = cat
			found = true
		} else if cat, exists := point.Labels["category"]; exists {
			category = cat
			found = true
		} else if cat, exists := point.Labels["name"]; exists {
			category = cat
			found = true
		} else if cat, exists := point.Labels["timestamp"]; exists {
			category = cat
			found = true
		}

		// If not found in Labels, check Fields map (for table format)
		if !found {
			if cat, exists := point.Fields["label"]; exists {
				if strCat, ok := cat.(string); ok {
					category = strCat
					found = true
				}
			} else if cat, exists := point.Fields["category"]; exists {
				if strCat, ok := cat.(string); ok {
					category = strCat
					found = true
				}
			} else if cat, exists := point.Fields["name"]; exists {
				if strCat, ok := cat.(string); ok {
					category = strCat
					found = true
				}
			}
		}

		if found {
			categories = append(categories, category)
		} else {
			// Log warning with available fields and labels
			log.Printf("Warning: Chart query missing 'label' column. Expected format: SELECT label, value FROM ... Available labels: %v, Available fields: %v",
				getAvailableLabels(point.Labels), getAvailableFields(point.Fields))

			// Last resort: use timestamp format with warning
			log.Printf("Warning: No suitable label column found, falling back to timestamp format")
			categories = append(categories, point.Timestamp.Format("15:04"))
		}
	}

	return categories
}

func getAvailableLabels(labels map[string]string) []string {
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	return keys
}

func getAvailableFields(fields map[string]interface{}) []string {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	return keys
}

func getChartColors(chartType lens.ChartType) []string {
	switch chartType {
	case lens.ChartTypeLine:
		return []string{"#10b981"}
	case lens.ChartTypeBar, lens.ChartTypeColumn:
		return []string{"#3b82f6"}
	case lens.ChartTypeStackedBar:
		return []string{"#3b82f6", "#10b981", "#f59e0b", "#ef4444", "#8b5cf6", "#06b6d4", "#84cc16", "#f97316"}
	case lens.ChartTypePie:
		return []string{"#10b981", "#3b82f6", "#f59e0b", "#ef4444", "#8b5cf6"}
	case lens.ChartTypeArea:
		return []string{"#06b6d4"}
	case lens.ChartTypeGauge:
		return []string{"#f59e0b"}
	case lens.ChartTypeTable:
		return []string{"#6b7280"}
	case lens.ChartTypeMetric:
		return []string{"#6b7280"}
	default:
		return []string{"#6b7280"}
	}
}

func addChartSpecificOptions(options *charts.ChartOptions, chartType lens.ChartType) {
	switch chartType {
	case lens.ChartTypeLine:
		options.Stroke = &charts.StrokeConfig{
			Curve: charts.StrokeCurveSmooth,
			Width: 2,
		}
		options.Markers = &charts.MarkersConfig{
			Size: 4,
		}
	case lens.ChartTypeBar, lens.ChartTypeColumn:
		options.PlotOptions = &charts.PlotOptions{
			Bar: &charts.BarConfig{
				ColumnWidth:  "55%",
				BorderRadius: 2,
			},
		}
	case lens.ChartTypeStackedBar:
		options.PlotOptions = &charts.PlotOptions{
			Bar: &charts.BarConfig{
				ColumnWidth:  "70%",
				BorderRadius: 2,
			},
		}
		// Enable legend for stacked bars to show series names
		position := charts.LegendPositionBottom
		options.Legend = &charts.LegendConfig{
			Position: &position,
		}
	case lens.ChartTypePie:
		position := charts.LegendPositionBottom
		options.Legend = &charts.LegendConfig{
			Position: &position,
		}
	case lens.ChartTypeArea:
		options.Stroke = &charts.StrokeConfig{
			Curve: charts.StrokeCurveSmooth,
		}
		options.Fill = &charts.FillConfig{
			Type: charts.FillTypeGradient,
			Gradient: &charts.FillGradient{
				ShadeIntensity: floatPtr(1),
				OpacityFrom:    floatPtr(0.7),
				OpacityTo:      floatPtr(0.3),
			},
		}
	case lens.ChartTypeGauge:
		startAngle := -135
		endAngle := 225
		margin := 0
		size := "70%"
		background := "#fff"
		image := ""
		position := "front"
		strokeWidth := "70%"
		fontSize16 := "16px"
		fontSize14 := "14px"
		fontWeight600 := "600"
		fontWeight400 := "400"
		color := "#373d3f"
		show := true
		label := "Total"
		offsetY120 := 120
		offsetY76 := 76

		options.PlotOptions = &charts.PlotOptions{
			RadialBar: &charts.RadialBarConfig{
				StartAngle: &startAngle,
				EndAngle:   &endAngle,
				Hollow: &charts.RadialBarHollow{
					Margin:     &margin,
					Size:       &size,
					Background: &background,
					Image:      &image,
					Position:   &position,
					DropShadow: &charts.DropShadow{
						Enabled: true,
						Top:     3,
						Left:    0,
						Blur:    4,
						Opacity: 0.24,
					},
				},
				Track: &charts.RadialBarTrack{
					Background:  &background,
					StrokeWidth: &strokeWidth,
					Margin:      &margin,
					DropShadow: &charts.DropShadow{
						Enabled: true,
						Top:     -3,
						Left:    0,
						Blur:    4,
						Opacity: 0.35,
					},
				},
				DataLabels: &charts.RadialBarDataLabels{
					Name: &charts.LabelNameValue{
						Show:       &show,
						FontSize:   &fontSize16,
						FontWeight: &fontWeight600,
						OffsetY:    &offsetY120,
					},
					Value: &charts.LabelNameValue{
						Show:       &show,
						FontSize:   &fontSize14,
						FontWeight: &fontWeight400,
						OffsetY:    &offsetY76,
					},
					Total: &charts.LabelTotal{
						Show:       &show,
						Label:      &label,
						FontSize:   &fontSize16,
						FontWeight: &fontWeight600,
						Color:      &color,
					},
				},
			},
		}
	case lens.ChartTypeTable:
		// Table doesn't need chart-specific options
	case lens.ChartTypeMetric:
		// Metric doesn't need chart-specific options
	}
}

func mergeCustomOptions(options *charts.ChartOptions, customOptions map[string]interface{}) {
	if customOptions == nil {
		return
	}

	// Apply custom colors
	if colors, ok := customOptions["colors"].([]string); ok {
		options.Colors = colors
	}

	// Apply custom title
	if title, ok := customOptions["title"].(string); ok {
		options.Title = &charts.TitleConfig{
			Text: &title,
		}
	}

	// Apply height
	if height, ok := customOptions["height"].(string); ok {
		options.Chart.Height = height
	}
}

// buildMetricFromResult builds a MetricValue from executor result
func buildMetricFromResult(config lens.PanelConfig, result *executor.ExecutionResult) lens.MetricValue {
	if len(result.Data) == 0 {
		return lens.MetricValue{
			Label: config.Title,
			Value: 0,
		}
	}

	// Use the first data point for the metric value
	dataPoint := result.Data[0]

	// Convert value to float64
	var value float64
	if val, ok := dataPoint.Value.(float64); ok {
		value = val
	} else if val, ok := dataPoint.Value.(int); ok {
		value = float64(val)
	} else if val, ok := dataPoint.Value.(int64); ok {
		value = float64(val)
	} else {
		value = 0.0
	}

	metric := lens.MetricValue{
		Label: config.Title,
		Value: value,
	}

	// Extract additional metric properties from config options
	if config.Options != nil {
		if unit, ok := config.Options["unit"].(string); ok {
			metric.Unit = unit
		}
		if color, ok := config.Options["color"].(string); ok {
			metric.Color = color
		}
		if icon, ok := config.Options["icon"].(string); ok {
			metric.Icon = icon
		}
		if formattedValue, ok := config.Options["formattedValue"].(string); ok {
			metric.FormattedValue = formattedValue
		}

		// Extract trend information
		if trendData, ok := config.Options["trend"].(map[string]interface{}); ok {
			trend := &lens.Trend{}
			if direction, ok := trendData["direction"].(string); ok {
				trend.Direction = direction
			}
			if percentage, ok := trendData["percentage"].(float64); ok {
				trend.Percentage = percentage
			}
			if isPositive, ok := trendData["isPositive"].(bool); ok {
				trend.IsPositive = isPositive
			}
			metric.Trend = trend
		}
	}

	return metric
}

// generateMetricCardStyle generates CSS styles for metric cards
func generateMetricCardStyle(metric lens.MetricValue) string {
	if metric.Color == "" {
		return ""
	}
	return fmt.Sprintf("--metric-color: %s;", metric.Color)
}

// formatMetricValue formats a numeric value with unit
func formatMetricValue(value float64, unit string) string {
	formatted := formatNumericValue(value)
	if unit != "" {
		return formatted + " " + unit
	}
	return formatted
}

// formatNumericValue formats a numeric value with appropriate precision
func formatNumericValue(value float64) string {
	// Handle large numbers with appropriate suffixes
	abs := value
	if abs < 0 {
		abs = -abs
	}

	if abs >= 1000000000 {
		return fmt.Sprintf("%.1fB", value/1000000000)
	} else if abs >= 1000000 {
		return fmt.Sprintf("%.1fM", value/1000000)
	} else if abs >= 1000 {
		return fmt.Sprintf("%.1fK", value/1000)
	} else if abs >= 1 {
		return fmt.Sprintf("%.0f", value)
	} else {
		return fmt.Sprintf("%.2f", value)
	}
}

// getTrendClass returns CSS class for trend styling
func getTrendClass(trend *lens.Trend) string {
	if trend == nil {
		return ""
	}

	baseClass := "metric-card__trend--"
	if trend.IsPositive {
		return baseClass + "positive"
	}
	return baseClass + "negative"
}

// getTrendIcon returns the appropriate icon for trend direction
func getTrendIcon(direction string) string {
	switch direction {
	case "up":
		return "↗"
	case "down":
		return "↘"
	case "stable":
		return "→"
	default:
		return "→"
	}
}

// formatPercentage formats a percentage value
func formatPercentage(value float64) string {
	return fmt.Sprintf("%.1f%%", value)
}

// Helper function to create float64 pointer
func floatPtr(f float64) *float64 {
	return &f
}

// addEventHandlers adds event handlers to chart options based on panel configuration
func addEventHandlers(options *charts.ChartOptions, config lens.PanelConfig) {
	if config.Events == nil {
		return
	}

	events := &charts.ChartEvents{}

	// Add general click handler
	if config.Events.Click != nil {
		events.Click = templ.JSExpression(fmt.Sprintf(
			"function(event, chartContext, opts) { handleChartEvent('%s', 'click', event, chartContext, opts, %s); }",
			config.ID,
			buildEventContextJS(config.Events.Click.Action),
		))
	}

	// Add data point selection handler
	if config.Events.DataPoint != nil {
		events.DataPointSelection = templ.JSExpression(fmt.Sprintf(
			"function(event, chartContext, opts) { handleChartEvent('%s', 'dataPoint', event, chartContext, opts, %s); }",
			config.ID,
			buildEventContextJS(config.Events.DataPoint.Action),
		))
	}

	// Add legend click handler
	if config.Events.Legend != nil {
		events.LegendClick = templ.JSExpression(fmt.Sprintf(
			"function(event, chartContext, opts) { handleChartEvent('%s', 'legend', event, chartContext, opts, %s); }",
			config.ID,
			buildEventContextJS(config.Events.Legend.Action),
		))
	}

	// Add marker click handler
	if config.Events.Marker != nil {
		events.MarkerClick = templ.JSExpression(fmt.Sprintf(
			"function(event, chartContext, opts) { handleChartEvent('%s', 'marker', event, chartContext, opts, %s); }",
			config.ID,
			buildEventContextJS(config.Events.Marker.Action),
		))
	}

	// Add X-axis label click handler
	if config.Events.XAxisLabel != nil {
		events.XAxisLabelClick = templ.JSExpression(fmt.Sprintf(
			"function(event, chartContext, opts) { handleChartEvent('%s', 'xAxisLabel', event, chartContext, opts, %s); }",
			config.ID,
			buildEventContextJS(config.Events.XAxisLabel.Action),
		))
	}

	// Only set events if any are configured
	if events.Click != "" || events.DataPointSelection != "" || events.LegendClick != "" ||
		events.MarkerClick != "" || events.XAxisLabelClick != "" {
		options.Chart.Events = events
	}
}

// buildEventContextJS creates a JavaScript object from ActionConfig for client-side use
func buildEventContextJS(action lens.ActionConfig) string {
	switch action.Type {
	case lens.ActionTypeNavigation:
		if action.Navigation != nil {
			return fmt.Sprintf(`{
				type: 'navigation',
				navigation: {
					url: '%s',
					target: '%s',
					variables: %s
				}
			}`,
				action.Navigation.URL,
				action.Navigation.Target,
				mapToJS(action.Navigation.Variables),
			)
		}
	case lens.ActionTypeDrillDown:
		if action.DrillDown != nil {
			return fmt.Sprintf(`{
				type: 'drillDown',
				drillDown: {
					dashboard: '%s',
					filters: %s,
					variables: %s
				}
			}`,
				action.DrillDown.Dashboard,
				mapToJS(action.DrillDown.Filters),
				mapToJS(action.DrillDown.Variables),
			)
		}
	case lens.ActionTypeModal:
		if action.Modal != nil {
			return fmt.Sprintf(`{
				type: 'modal',
				modal: {
					title: '%s',
					content: '%s',
					url: '%s',
					variables: %s
				}
			}`,
				action.Modal.Title,
				action.Modal.Content,
				action.Modal.URL,
				mapToJS(action.Modal.Variables),
			)
		}
	case lens.ActionTypeCustom:
		if action.Custom != nil {
			return fmt.Sprintf(`{
				type: 'custom',
				custom: {
					function: '%s',
					variables: %s
				}
			}`,
				action.Custom.Function,
				mapToJS(action.Custom.Variables),
			)
		}
	}
	return "{}"
}

// mapToJS converts a map[string]string to a JavaScript object string
func mapToJS(m map[string]string) string {
	if len(m) == 0 {
		return "{}"
	}

	result := "{"
	first := true
	for key, value := range m {
		if !first {
			result += ","
		}
		result += fmt.Sprintf("'%s': '%s'", key, value)
		first = false
	}
	result += "}"
	return result
}
