package chart

import (
	"context"
	"fmt"
	tools "github.com/iota-uz/iota-sdk/pkg/bichat/tools"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

// DrawChartTool creates chart visualizations for data analysis.
// It generates chart specifications that can be rendered by frontend visualization libraries.
// This tool does NOT render charts - it only returns specifications.
type DrawChartTool struct{}

const (
	defaultChartType       = "line"
	defaultChartHeight     = 360
	minChartHeight         = 100
	maxChartHeight         = 1000
	maxSeriesCount         = 20
	maxDataPointsPerSeries = 10000
	downsampleTargetPoints = 500
	autoLogRatioThreshold  = 100
)

var (
	validHexColorPattern = regexp.MustCompile(`^#([A-Fa-f0-9]{3}|[A-Fa-f0-9]{4}|[A-Fa-f0-9]{6}|[A-Fa-f0-9]{8})$`)
	defaultPalette       = []any{"#2563EB", "#059669", "#EA580C", "#DC2626", "#7C3AED", "#0891B2"}
	supportedChartTypes  = map[string]struct{}{
		"line":      {},
		"area":      {},
		"bar":       {},
		"pie":       {},
		"donut":     {},
		"radialbar": {},
		"scatter":   {},
		"bubble":    {},
		"heatmap":   {},
		"radar":     {},
		"polararea": {},
		"treemap":   {},
	}
)

// NewDrawChartTool creates a new draw chart tool.
func NewDrawChartTool() agents.Tool {
	return &DrawChartTool{}
}

// Name returns the tool name.
func (t *DrawChartTool) Name() string {
	return "draw_chart"
}

// Description returns the tool description for the LLM.
func (t *DrawChartTool) Description() string {
	return "Create chart visualizations using ApexCharts options. " +
		"Pass a single options object that mirrors ApexCharts configuration " +
		"(chart, series, xaxis, yaxis, colors, etc). " +
		"The tool applies smart defaults, validates chart safety/quality, " +
		"and auto-enables logarithmic y-axis for highly scattered positive data when appropriate."
}

// Parameters returns the JSON Schema for tool parameters.
func (t *DrawChartTool) Parameters() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Create an ApexCharts chart using canonical arguments: {\"options\": {...}}.",
		"properties": map[string]any{
			"options": map[string]any{
				"type":                 "object",
				"description":          "ApexCharts options object with series and chart/type (plus title, xaxis, yaxis, colors, etc.).",
				"additionalProperties": true,
			},
		},
		"required":             []string{"options"},
		"additionalProperties": true,
	}
}

// chartToolInput represents the parsed input parameters.
type chartToolInput struct {
	Options map[string]any `json:"options"`
}

type seriesStats struct {
	seriesCount        int
	maxPointsPerSeries int
	minPositive        float64
	maxValue           float64
	hasValue           bool
	hasNonPositive     bool
	singlePointType    bool
}

// CallStructured executes the chart creation and returns a structured result.
func (t *DrawChartTool) CallStructured(ctx context.Context, input string) (*types.ToolResult, error) {
	params, err := agents.ParseToolInput[chartToolInput](input)
	if err != nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: fmt.Sprintf("failed to parse input: %v", err),
				Hints:   []string{tools.HintCheckRequiredFields, tools.HintCheckFieldTypes},
			},
		}, nil
	}

	options := cloneMap(params.Options)

	if len(options) == 0 {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: "options parameter is required and must not be empty",
				Hints:   []string{tools.HintCheckRequiredFields, "Provide a valid ApexCharts options object"},
			},
		}, nil
	}

	if err := t.validateNoLegacyFields(options); err != nil {
		//nolint:nilerr // validation error is surfaced as a structured ToolResult; callers receive CodecToolError payload instead of a Go error
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: err.Error(),
				Hints:   []string{tools.HintCheckFieldFormat, "Use ApexCharts-native keys (chart.type, chart.height, title.text, series)"},
			},
		}, nil
	}

	chartType, err := t.ensureChartType(options)
	if err != nil {
		//nolint:nilerr // validation error is surfaced as a structured ToolResult; callers receive CodecToolError payload instead of a Go error
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: err.Error(),
				Hints:   []string{tools.HintCheckFieldFormat, "Use ApexCharts chart.type values (line, bar, area, pie, donut, etc.)"},
			},
		}, nil
	}

	stats, err := t.normalizeAndValidateSeries(options, chartType)
	if err != nil {
		return &types.ToolResult{ //nolint:nilerr // validation error is surfaced as a structured ToolResult; callers receive CodecToolError payload instead of a Go error
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: err.Error(),
				Hints:   []string{tools.HintCheckFieldFormat, "Ensure series/data matches ApexCharts expectations"},
			},
		}, nil
	}

	if err := t.validateOptions(options, chartType, stats); err != nil {
		return &types.ToolResult{ //nolint:nilerr // validation error is surfaced as a structured ToolResult; callers receive CodecToolError payload instead of a Go error
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: err.Error(),
				Hints:   []string{tools.HintCheckFieldFormat, "Verify options.chart, options.series, colors, and axis values"},
			},
		}, nil
	}

	t.applySmartDefaults(options, chartType, stats)

	warnings := make([]string, 0)
	warnings = append(warnings, t.applyAxisInferenceAndNormalize(options, chartType)...)
	warnings = append(warnings, t.applyDownsampling(options, chartType)...)

	logWarning, err := t.applyLogScale(options, chartType, stats)
	if err != nil {
		//nolint:nilerr // validation error is surfaced as a structured ToolResult; callers receive CodecToolError payload instead of a Go error
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: err.Error(),
				Hints:   []string{tools.HintCheckFieldFormat, "Use logarithmic scale only when all y values are strictly positive"},
			},
		}, nil
	}
	if logWarning != "" {
		warnings = append(warnings, logWarning)
	}
	warnings = dedupeWarnings(warnings)

	chartTitle := t.extractTitle(options)
	if chartTitle == "" {
		chartTitle = "Chart"
	}

	metadata := map[string]any{
		"spec": options,
	}
	if len(warnings) > 0 {
		metadata["warnings"] = warnings
	}

	return &types.ToolResult{
		CodecID: types.CodecJSON,
		Payload: types.JSONPayload{Output: options},
		Artifacts: []types.ToolArtifact{
			{
				Type:        "chart",
				Name:        chartTitle,
				Description: fmt.Sprintf("%s chart", chartType),
				MimeType:    "application/json",
				Metadata:    metadata,
			},
		},
	}, nil
}

// Call executes the chart creation and returns a chart specification.
func (t *DrawChartTool) Call(ctx context.Context, input string) (string, error) {
	return tools.FormatStructuredResult(t.CallStructured(ctx, input))
}

func (t *DrawChartTool) validateNoLegacyFields(options map[string]any) error {
	if _, ok := options["chartType"]; ok {
		return fmt.Errorf("options.chartType is not supported; use options.chart.type")
	}
	if _, ok := options["type"]; ok {
		return fmt.Errorf("options.type is not supported; use options.chart.type")
	}
	if _, ok := options["height"]; ok {
		return fmt.Errorf("options.height is not supported; use options.chart.height")
	}
	return nil
}

func (t *DrawChartTool) ensureChartType(options map[string]any) (string, error) {
	chartCfg, err := ensureMap(options, "chart")
	if err != nil {
		return "", err
	}

	if _, exists := chartCfg["type"]; !exists {
		chartCfg["type"] = defaultChartType
	}

	rawType, ok := chartCfg["type"].(string)
	if !ok {
		return "", fmt.Errorf("options.chart.type must be a string")
	}

	chartType := strings.ToLower(strings.TrimSpace(rawType))
	if chartType == "" {
		chartType = defaultChartType
	}
	if _, ok := supportedChartTypes[chartType]; !ok {
		return "", fmt.Errorf("unsupported chart type: %s", chartType)
	}
	chartCfg["type"] = chartType

	return chartType, nil
}

func (t *DrawChartTool) normalizeAndValidateSeries(options map[string]any, chartType string) (seriesStats, error) {
	var stats seriesStats
	stats.minPositive = math.MaxFloat64
	stats.singlePointType = true

	rawSeries, ok := options["series"]
	if !ok {
		return stats, fmt.Errorf("options.series is required")
	}

	seriesSlice, ok := rawSeries.([]any)
	if !ok {
		return stats, fmt.Errorf("options.series must be an array")
	}
	if len(seriesSlice) == 0 {
		return stats, fmt.Errorf("options.series must not be empty")
	}
	if len(seriesSlice) > maxSeriesCount {
		return stats, fmt.Errorf("options.series exceeds maximum %d series: %d", maxSeriesCount, len(seriesSlice))
	}
	stats.seriesCount = len(seriesSlice)

	if isPieLikeChart(chartType) {
		if isMapLikeSlice(seriesSlice) {
			return stats, fmt.Errorf("%s charts require numeric options.series values", chartType)
		}

		if len(seriesSlice) > maxDataPointsPerSeries {
			return stats, fmt.Errorf("options.series exceeds maximum %d data points: %d", maxDataPointsPerSeries, len(seriesSlice))
		}

		stats.maxPointsPerSeries = len(seriesSlice)
		for i, v := range seriesSlice {
			f, ok := toFloat(v)
			if !ok {
				return stats, fmt.Errorf("options.series[%d] is not a number", i)
			}
			if math.IsNaN(f) || math.IsInf(f, 0) {
				return stats, fmt.Errorf("options.series[%d] is NaN or Infinity", i)
			}
			if f < 0 {
				return stats, fmt.Errorf("options.series[%d] is negative (%.2f); %s charts require non-negative values", i, f, chartType)
			}
			stats.consumeValue(f)
		}
		return stats, nil
	}

	if !isMapLikeSlice(seriesSlice) {
		return stats, fmt.Errorf("%s charts require options.series as an array of objects with data arrays", chartType)
	}

	pointCounts := make([]int, 0, len(seriesSlice))
	for i, seriesItem := range seriesSlice {
		seriesMap, ok := seriesItem.(map[string]any)
		if !ok {
			return stats, fmt.Errorf("options.series[%d] must be an object with a data array", i)
		}
		dataRaw, ok := seriesMap["data"]
		if !ok {
			return stats, fmt.Errorf("options.series[%d].data is required", i)
		}
		dataSlice, ok := dataRaw.([]any)
		if !ok {
			return stats, fmt.Errorf("options.series[%d].data must be an array", i)
		}
		if len(dataSlice) == 0 {
			return stats, fmt.Errorf("options.series[%d].data has no data points", i)
		}
		if len(dataSlice) > maxDataPointsPerSeries {
			return stats, fmt.Errorf("options.series[%d].data exceeds maximum %d data points: %d", i, maxDataPointsPerSeries, len(dataSlice))
		}
		pointCounts = append(pointCounts, len(dataSlice))
		if len(dataSlice) > stats.maxPointsPerSeries {
			stats.maxPointsPerSeries = len(dataSlice)
		}

		for j, point := range dataSlice {
			if pointMap, ok := point.(map[string]any); ok {
				stats.singlePointType = false
				y, ok := pointMap["y"]
				if !ok {
					return stats, fmt.Errorf("options.series[%d].data[%d].y is required for object points", i, j)
				}
				f, ok := toFloat(y)
				if !ok {
					return stats, fmt.Errorf("options.series[%d].data[%d].y is not a number", i, j)
				}
				if math.IsNaN(f) || math.IsInf(f, 0) {
					return stats, fmt.Errorf("options.series[%d].data[%d].y is NaN or Infinity", i, j)
				}
				stats.consumeValue(f)
				continue
			}

			f, ok := toFloat(point)
			if !ok {
				return stats, fmt.Errorf("options.series[%d].data[%d] is not a number", i, j)
			}
			if math.IsNaN(f) || math.IsInf(f, 0) {
				return stats, fmt.Errorf("options.series[%d].data[%d] is NaN or Infinity", i, j)
			}
			stats.consumeValue(f)
		}

		if _, ok := seriesMap["name"]; !ok {
			seriesMap["name"] = fmt.Sprintf("Series %d", i+1)
		}
	}

	if stats.singlePointType && len(pointCounts) > 1 {
		first := pointCounts[0]
		for i := 1; i < len(pointCounts); i++ {
			if pointCounts[i] != first {
				return stats, fmt.Errorf("options.series[%d].data has %d data points but options.series[0].data has %d; all series must have equal length",
					i, pointCounts[i], first)
			}
		}
	}

	return stats, nil
}

func (t *DrawChartTool) validateOptions(options map[string]any, chartType string, stats seriesStats) error {
	if stats.seriesCount == 0 {
		return fmt.Errorf("options.series must not be empty")
	}
	if stats.maxPointsPerSeries == 0 {
		return fmt.Errorf("options.series must include at least one data point")
	}

	if err := validateChartHeight(options); err != nil {
		return err
	}

	if err := validateColors(options); err != nil {
		return err
	}
	if err := validateTitle(options); err != nil {
		return err
	}

	if isPieLikeChart(chartType) {
		if labelsRaw, ok := options["labels"]; ok {
			labels, ok := labelsRaw.([]any)
			if !ok {
				return fmt.Errorf("options.labels must be an array when provided")
			}
			series, _ := options["series"].([]any)
			if len(labels) > 0 && len(labels) != len(series) {
				return fmt.Errorf("labels count (%d) does not match data points (%d)", len(labels), len(series))
			}
		}
		return nil
	}

	xaxisRaw, hasXaxis := options["xaxis"]
	if !hasXaxis {
		return nil
	}

	xaxis, ok := xaxisRaw.(map[string]any)
	if !ok {
		return fmt.Errorf("options.xaxis must be an object")
	}
	categoriesRaw, ok := xaxis["categories"]
	if !ok {
		return nil
	}
	categories, ok := categoriesRaw.([]any)
	if !ok {
		return fmt.Errorf("options.xaxis.categories must be an array")
	}

	if stats.singlePointType && stats.maxPointsPerSeries > 0 && len(categories) > 0 && len(categories) != stats.maxPointsPerSeries {
		return fmt.Errorf("options.xaxis.categories count (%d) does not match data points (%d)", len(categories), stats.maxPointsPerSeries)
	}

	return nil
}

func (t *DrawChartTool) applySmartDefaults(options map[string]any, chartType string, stats seriesStats) {
	chartCfg, _ := ensureMap(options, "chart")
	if _, ok := chartCfg["height"]; !ok {
		chartCfg["height"] = defaultChartHeight
	}
	toolbar, _ := ensureMap(chartCfg, "toolbar")
	if _, ok := toolbar["show"]; !ok {
		toolbar["show"] = true
	}
	zoom, _ := ensureMap(chartCfg, "zoom")
	if _, ok := zoom["enabled"]; !ok {
		zoom["enabled"] = !isPieLikeChart(chartType)
	}

	titleCfg, titleIsMap := options["title"].(map[string]any)
	if !titleIsMap {
		titleCfg = map[string]any{}
		options["title"] = titleCfg
	}
	if _, ok := titleCfg["text"]; !ok || strings.TrimSpace(fmt.Sprintf("%v", titleCfg["text"])) == "" {
		titleCfg["text"] = "Chart"
	}

	dataLabels, _ := ensureMap(options, "dataLabels")
	if _, ok := dataLabels["enabled"]; !ok {
		dataLabels["enabled"] = isPieLikeChart(chartType)
	}

	legend, _ := ensureMap(options, "legend")
	if _, ok := legend["position"]; !ok {
		legend["position"] = "top"
	}

	grid, _ := ensureMap(options, "grid")
	if _, ok := grid["strokeDashArray"]; !ok {
		grid["strokeDashArray"] = 3
	}

	theme, _ := ensureMap(options, "theme")
	if _, ok := theme["mode"]; !ok {
		theme["mode"] = "light"
	}

	if _, ok := options["colors"]; !ok {
		options["colors"] = cloneSlice(defaultPalette)
	}

	if chartType == "line" || chartType == "area" {
		stroke, _ := ensureMap(options, "stroke")
		if _, ok := stroke["width"]; !ok {
			stroke["width"] = 2
		}
		if _, ok := stroke["curve"]; !ok {
			stroke["curve"] = "smooth"
		}

		markers, _ := ensureMap(options, "markers")
		if _, ok := markers["size"]; !ok {
			if stats.maxPointsPerSeries <= 40 {
				markers["size"] = 3
			} else {
				markers["size"] = 0
			}
		}
	}

	if stats.seriesCount > 1 && (chartType == "line" || chartType == "area" || chartType == "bar") {
		tooltip, _ := ensureMap(options, "tooltip")
		if _, ok := tooltip["shared"]; !ok {
			tooltip["shared"] = true
		}
	}
}

func (t *DrawChartTool) applyLogScale(options map[string]any, chartType string, stats seriesStats) (string, error) {
	if isPieLikeChart(chartType) || !stats.hasValue {
		return "", nil
	}

	explicitLog, hasExplicit, multiAxis := getLogConfig(options)
	if hasExplicit && explicitLog && stats.hasNonPositive {
		return "", fmt.Errorf("options.yaxis.logarithmic=true requires all y values to be strictly positive")
	}
	if hasExplicit || multiAxis || stats.hasNonPositive || stats.minPositive <= 0 {
		return "", nil
	}

	if stats.maxValue/stats.minPositive >= autoLogRatioThreshold {
		yaxis, _ := ensureMap(options, "yaxis")
		yaxis["logarithmic"] = true
		return "Automatically enabled logarithmic y-axis for widely scattered positive values.", nil
	}
	return "", nil
}

func (t *DrawChartTool) extractTitle(options map[string]any) string {
	titleRaw, ok := options["title"]
	if !ok {
		return ""
	}
	titleMap, ok := titleRaw.(map[string]any)
	if !ok {
		return ""
	}
	textRaw, ok := titleMap["text"]
	if !ok {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%v", textRaw))
}

func validateChartHeight(options map[string]any) error {
	chartRaw, ok := options["chart"]
	if !ok {
		return nil
	}
	chartCfg, ok := chartRaw.(map[string]any)
	if !ok {
		return fmt.Errorf("options.chart must be an object")
	}
	heightRaw, ok := chartCfg["height"]
	if !ok {
		return nil
	}

	switch h := heightRaw.(type) {
	case float64:
		if h < minChartHeight || h > maxChartHeight {
			return fmt.Errorf("chart height must be between %d-%d pixels, got %.2f", minChartHeight, maxChartHeight, h)
		}
	case int:
		if h < minChartHeight || h > maxChartHeight {
			return fmt.Errorf("chart height must be between %d-%d pixels, got %d", minChartHeight, maxChartHeight, h)
		}
	case string:
		s := strings.TrimSpace(h)
		if s == "" {
			return fmt.Errorf("options.chart.height must not be empty")
		}
		if strings.HasSuffix(s, "%") {
			return nil
		}
		value, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return fmt.Errorf("options.chart.height must be a number or percentage string")
		}
		if value < minChartHeight || value > maxChartHeight {
			return fmt.Errorf("chart height must be between %d-%d pixels, got %.2f", minChartHeight, maxChartHeight, value)
		}
	default:
		return fmt.Errorf("options.chart.height must be a number or percentage string")
	}

	return nil
}

func validateColors(options map[string]any) error {
	rawColors, ok := options["colors"]
	if !ok {
		return nil
	}
	colorSlice, ok := rawColors.([]any)
	if !ok {
		return fmt.Errorf("options.colors must be an array")
	}
	for i, colorRaw := range colorSlice {
		colorStr, ok := colorRaw.(string)
		if !ok {
			return fmt.Errorf("options.colors[%d] must be a string", i)
		}
		colorStr = strings.TrimSpace(colorStr)
		if colorStr == "" {
			return fmt.Errorf("options.colors[%d] must not be empty", i)
		}
		if strings.HasPrefix(colorStr, "#") && !validHexColorPattern.MatchString(colorStr) {
			return fmt.Errorf("options.colors[%d] is not a valid hex color: %s", i, colorStr)
		}
	}
	return nil
}

func validateTitle(options map[string]any) error {
	titleRaw, ok := options["title"]
	if !ok {
		return nil
	}
	titleMap, ok := titleRaw.(map[string]any)
	if !ok {
		return fmt.Errorf("options.title must be an object with a text field")
	}
	if textRaw, ok := titleMap["text"]; ok {
		if _, ok := textRaw.(string); !ok {
			return fmt.Errorf("options.title.text must be a string")
		}
	}
	return nil
}

func getLogConfig(options map[string]any) (bool, bool, bool) {
	yaxisRaw, ok := options["yaxis"]
	if !ok {
		return false, false, false
	}

	if yaxisMap, ok := yaxisRaw.(map[string]any); ok {
		if rawLog, ok := yaxisMap["logarithmic"]; ok {
			if flag, ok := rawLog.(bool); ok {
				return flag, true, false
			}
		}
		return false, false, false
	}

	if axes, ok := yaxisRaw.([]any); ok {
		hasLog := false
		isLog := false
		for _, axis := range axes {
			axisMap, ok := axis.(map[string]any)
			if !ok {
				continue
			}
			rawLog, ok := axisMap["logarithmic"]
			if !ok {
				continue
			}
			flag, ok := rawLog.(bool)
			if !ok {
				continue
			}
			hasLog = true
			if flag {
				isLog = true
			}
		}
		return isLog, hasLog, true
	}

	return false, false, false
}

func (s *seriesStats) consumeValue(v float64) {
	s.hasValue = true
	if v <= 0 {
		s.hasNonPositive = true
	} else if v < s.minPositive {
		s.minPositive = v
	}
	if v > s.maxValue {
		s.maxValue = v
	}
}

func isPieLikeChart(chartType string) bool {
	switch chartType {
	case "pie", "donut", "polararea", "radialbar":
		return true
	default:
		return false
	}
}

func toFloat(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint64:
		return float64(v), true
	case uint32:
		return float64(v), true
	default:
		return 0, false
	}
}

func ensureMap(parent map[string]any, key string) (map[string]any, error) {
	if raw, ok := parent[key]; ok {
		existing, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("options.%s must be an object", key)
		}
		return existing, nil
	}
	created := make(map[string]any)
	parent[key] = created
	return created, nil
}

func isMapLikeSlice(values []any) bool {
	if len(values) == 0 {
		return false
	}
	_, ok := values[0].(map[string]any)
	return ok
}

func cloneMap(src map[string]any) map[string]any {
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = cloneValue(v)
	}
	return out
}

func cloneSlice(src []any) []any {
	out := make([]any, 0, len(src))
	for _, v := range src {
		out = append(out, cloneValue(v))
	}
	return out
}

func cloneValue(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		return cloneMap(typed)
	case []any:
		return cloneSlice(typed)
	default:
		return typed
	}
}

func (t *DrawChartTool) applyAxisInferenceAndNormalize(options map[string]any, chartType string) []string {
	if isPieLikeChart(chartType) {
		return nil
	}

	seriesSlice, ok := options["series"].([]any)
	if !ok || len(seriesSlice) == 0 {
		return nil
	}

	var xaxis map[string]any
	if rawXaxis, hasXaxis := options["xaxis"]; hasXaxis {
		existing, ok := rawXaxis.(map[string]any)
		if !ok {
			return []string{"Skipped x-axis normalization because options.xaxis is not an object."}
		}
		xaxis = existing
	}

	explicitType := ""
	if xaxis != nil {
		if rawType, ok := xaxis["type"].(string); ok {
			explicitType = strings.ToLower(strings.TrimSpace(rawType))
		}
	}
	explicitDatetime := explicitType == "datetime"

	inferFromCategories := false
	if explicitType == "" && xaxis != nil {
		if categories, ok := xaxis["categories"].([]any); ok {
			if shouldInferDatetimeFromValues(categories) {
				inferFromCategories = true
			}
		}
	}

	inferFromPointX := false
	if explicitType == "" && !inferFromCategories {
		xCount := 0
		allParseable := true
		for _, seriesItem := range seriesSlice {
			seriesMap, ok := seriesItem.(map[string]any)
			if !ok {
				continue
			}
			dataSlice, ok := seriesMap["data"].([]any)
			if !ok {
				continue
			}
			for _, point := range dataSlice {
				pointMap, ok := point.(map[string]any)
				if !ok {
					continue
				}
				xRaw, ok := pointMap["x"]
				if !ok {
					continue
				}
				xStr, ok := xRaw.(string)
				if !ok || strings.TrimSpace(xStr) == "" {
					continue
				}
				xCount++
				if _, ok := normalizeDatetimeValue(xRaw); !ok {
					allParseable = false
				}
			}
		}
		if xCount >= 2 && allParseable {
			inferFromPointX = true
		}
	}

	if !explicitDatetime && !inferFromCategories && !inferFromPointX {
		return nil
	}

	if xaxis == nil {
		xaxis = map[string]any{}
		options["xaxis"] = xaxis
	}
	xaxis["type"] = "datetime"

	warnings := make([]string, 0, 3)
	if inferFromCategories {
		warnings = append(warnings, "Inferred datetime x-axis from category labels.")
	}
	if inferFromPointX {
		warnings = append(warnings, "Inferred datetime x-axis from series x-values.")
	}

	categoryConversions := 0
	categoryFailures := 0
	if categoriesRaw, ok := xaxis["categories"]; ok {
		if categories, ok := categoriesRaw.([]any); ok {
			normalized := make([]any, len(categories))
			for i, value := range categories {
				if ms, ok := normalizeDatetimeValue(value); ok {
					normalized[i] = ms
					categoryConversions++
				} else {
					normalized[i] = value
					categoryFailures++
				}
			}
			if categoryConversions > 0 {
				xaxis["categories"] = normalized
			}
		}
	}

	pointFailures := 0
	for _, seriesItem := range seriesSlice {
		seriesMap, ok := seriesItem.(map[string]any)
		if !ok {
			continue
		}
		dataSlice, ok := seriesMap["data"].([]any)
		if !ok {
			continue
		}
		for _, point := range dataSlice {
			pointMap, ok := point.(map[string]any)
			if !ok {
				continue
			}
			xRaw, ok := pointMap["x"]
			if !ok {
				continue
			}
			ms, ok := normalizeDatetimeValue(xRaw)
			if !ok {
				pointFailures++
				continue
			}
			pointMap["x"] = ms
		}
	}

	if categoryFailures > 0 || pointFailures > 0 {
		warnings = append(warnings, "Some datetime labels could not be parsed and were left unchanged.")
	}

	return warnings
}

func (t *DrawChartTool) applyDownsampling(options map[string]any, chartType string) []string {
	if isPieLikeChart(chartType) {
		return nil
	}

	seriesSlice, ok := options["series"].([]any)
	if !ok || len(seriesSlice) == 0 {
		return nil
	}

	downsampledSeries := 0
	maxOriginalPoints := 0
	sharedSourceLen := 0
	sharedIndices := []int(nil)
	canTrimCategories := true

	for _, seriesItem := range seriesSlice {
		seriesMap, ok := seriesItem.(map[string]any)
		if !ok {
			continue
		}
		dataSlice, ok := seriesMap["data"].([]any)
		if !ok {
			continue
		}
		if len(dataSlice) <= downsampleTargetPoints {
			continue
		}

		indices := buildDownsampleIndices(len(dataSlice), downsampleTargetPoints)
		if len(indices) == 0 || len(indices) >= len(dataSlice) {
			continue
		}

		seriesMap["data"] = selectByIndices(dataSlice, indices)
		downsampledSeries++
		if len(dataSlice) > maxOriginalPoints {
			maxOriginalPoints = len(dataSlice)
		}

		if sharedIndices == nil {
			sharedIndices = indices
			sharedSourceLen = len(dataSlice)
		} else if sharedSourceLen != len(dataSlice) {
			canTrimCategories = false
		}
	}

	if downsampledSeries == 0 {
		return nil
	}

	if canTrimCategories && len(sharedIndices) > 0 {
		if xaxis, ok := options["xaxis"].(map[string]any); ok {
			if categories, ok := xaxis["categories"].([]any); ok && len(categories) == sharedSourceLen {
				xaxis["categories"] = selectByIndices(categories, sharedIndices)
			}
		}
	}

	return []string{
		fmt.Sprintf(
			"Downsampled %d dense series to %d points (largest series had %d points) for readability and performance.",
			downsampledSeries,
			downsampleTargetPoints,
			maxOriginalPoints,
		),
	}
}

func shouldInferDatetimeFromValues(values []any) bool {
	if len(values) < 2 {
		return false
	}
	hasString := false
	for _, value := range values {
		raw, ok := value.(string)
		if !ok || strings.TrimSpace(raw) == "" {
			return false
		}
		hasString = true
		if _, ok := normalizeDatetimeValue(raw); !ok {
			return false
		}
	}
	return hasString
}

func normalizeDatetimeValue(value any) (int64, bool) {
	raw, ok := value.(string)
	if ok {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return 0, false
		}

		if parsed, ok := parseDateStringToMillis(raw); ok {
			return parsed, true
		}

		n, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return 0, false
		}
		if !looksLikeUnixEpoch(n) {
			return 0, false
		}
		return normalizeEpochToMillis(n), true
	}

	if f, ok := toFloat(value); ok {
		if !looksLikeUnixEpoch(f) {
			return 0, false
		}
		return normalizeEpochToMillis(f), true
	}
	return 0, false
}

func parseDateStringToMillis(raw string) (int64, bool) {
	if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return parsed.UnixMilli(), true
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return parsed.UnixMilli(), true
	}
	// Use a single slash format to avoid ambiguous month/day (e.g. 03/04/2025). US style: MM/DD/2006.
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		"2006/01/02",
		"01/02/2006",
		"2006-01",
		"2006",
	}
	for _, layout := range layouts {
		if parsed, err := time.ParseInLocation(layout, raw, time.UTC); err == nil {
			return parsed.UnixMilli(), true
		}
	}
	return 0, false
}

// looksLikeUnixEpoch returns true only for values in a plausible Unix epoch range
// (seconds 1e9–1e10 or milliseconds 1e12–1e13) to avoid misclassifying large numerics (e.g. revenue).
func looksLikeUnixEpoch(value float64) bool {
	abs := math.Abs(value)
	return (abs >= 1e9 && abs < 1e11) || (abs >= 1e12 && abs < 1e14)
}

func normalizeEpochToMillis(value float64) int64 {
	abs := math.Abs(value)
	switch {
	case abs < 1e11:
		return int64(math.Round(value * 1000))
	case abs < 1e14:
		return int64(math.Round(value))
	case abs < 1e17:
		return int64(math.Round(value / 1000))
	default:
		return int64(math.Round(value / 1e6))
	}
}

func buildDownsampleIndices(length, target int) []int {
	if length <= target || target <= 1 {
		return nil
	}
	if target == 2 {
		return []int{0, length - 1}
	}

	step := float64(length-1) / float64(target-1)
	indices := make([]int, 0, target)
	last := -1
	for i := 0; i < target; i++ {
		candidate := int(math.Round(float64(i) * step))
		if candidate <= last {
			candidate = last + 1
		}
		if candidate >= length {
			candidate = length - 1
		}
		indices = append(indices, candidate)
		last = candidate
	}
	indices[len(indices)-1] = length - 1
	return indices
}

func selectByIndices(values []any, indices []int) []any {
	if len(indices) == 0 || len(values) == 0 {
		return values
	}
	out := make([]any, 0, len(indices))
	for _, idx := range indices {
		if idx >= 0 && idx < len(values) {
			out = append(out, values[idx])
		}
	}
	return out
}

func dedupeWarnings(warnings []string) []string {
	if len(warnings) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(warnings))
	result := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		warning = strings.TrimSpace(warning)
		if warning == "" {
			continue
		}
		if _, ok := seen[warning]; ok {
			continue
		}
		seen[warning] = struct{}{}
		result = append(result, warning)
	}
	return result
}
