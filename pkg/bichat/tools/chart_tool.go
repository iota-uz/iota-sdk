package tools

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// DrawChartTool creates chart visualizations for data analysis.
// It generates chart specifications that can be rendered by frontend visualization libraries.
// This tool does NOT render charts - it only returns specifications.
type DrawChartTool struct{}

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
	return "Create chart visualizations for data analysis. " +
		"Supports line, bar, pie, area, and donut charts. " +
		"Pie and donut charts require a single series. " +
		"Maximum 1000 data points per series. " +
		"Colors must be valid hex codes. " +
		"Height must be between 100-1000 pixels."
}

// Parameters returns the JSON Schema for tool parameters.
func (t *DrawChartTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"chartType": map[string]any{
				"type":        "string",
				"description": "Chart type: line, bar, pie, area, or donut",
				"enum":        []string{"line", "bar", "pie", "area", "donut"},
			},
			"title": map[string]any{
				"type":        "string",
				"description": "Chart title",
			},
			"series": map[string]any{
				"type":        "array",
				"description": "Array of data series with name and data array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{
							"type":        "string",
							"description": "Series name",
						},
						"data": map[string]any{
							"type":        "array",
							"description": "Array of numeric values",
							"items": map[string]any{
								"type": "number",
							},
						},
					},
					"required": []string{"name", "data"},
				},
			},
			"labels": map[string]any{
				"type":        "array",
				"description": "X-axis labels (optional, auto-indexed if omitted)",
				"items": map[string]any{
					"type": "string",
				},
			},
			"colors": map[string]any{
				"type":        "array",
				"description": "Hex color codes for series (optional, uses default palette)",
				"items": map[string]any{
					"type": "string",
				},
			},
			"height": map[string]any{
				"type":        "integer",
				"description": "Chart height in pixels (default: 350, range: 100-1000)",
				"default":     350,
			},
		},
		"required": []string{"chartType", "title", "series"},
	}
}

// ChartSeries represents a data series in a chart.
type ChartSeries struct {
	Name string        `json:"name"`
	Data []interface{} `json:"data"`
}

// chartToolInput represents the parsed input parameters.
type chartToolInput struct {
	ChartType string        `json:"chartType"`
	Title     string        `json:"title"`
	Series    []ChartSeries `json:"series"`
	Labels    []string      `json:"labels,omitempty"`
	Colors    []string      `json:"colors,omitempty"`
	Height    int           `json:"height,omitempty"`
}

// Call executes the chart creation and returns a chart specification.
func (t *DrawChartTool) Call(ctx context.Context, input string) (string, error) {
	const op serrors.Op = "DrawChartTool.Call"

	// Parse input
	params, err := agents.ParseToolInput[chartToolInput](input)
	if err != nil {
		return "", serrors.E(op, err, "failed to parse input")
	}

	// Validate required fields
	if params.ChartType == "" {
		return "", serrors.E(op, "chartType parameter is required")
	}
	if params.Title == "" {
		return "", serrors.E(op, "title parameter is required")
	}
	if len(params.Series) == 0 {
		return "", serrors.E(op, "series parameter is required and must not be empty")
	}

	// Set defaults
	if params.Height == 0 {
		params.Height = 350
	}

	// Validate chart type
	if err := t.validateChartType(params.ChartType); err != nil {
		return "", serrors.E(op, err)
	}

	// Validate inputs
	if err := t.validate(params.ChartType, params.Series, params.Labels, params.Colors, params.Height); err != nil {
		return "", serrors.E(op, err)
	}

	// Return chart specification as JSON
	return agents.FormatToolOutput(params)
}

// validateChartType validates the chart type is supported.
func (t *DrawChartTool) validateChartType(chartType string) error {
	const op serrors.Op = "DrawChartTool.validateChartType"

	validTypes := map[string]bool{
		"line":  true,
		"bar":   true,
		"pie":   true,
		"area":  true,
		"donut": true,
	}

	if !validTypes[chartType] {
		return serrors.E(op, fmt.Sprintf("unsupported chart type: %s", chartType))
	}

	return nil
}

// validate validates the chart parameters.
func (t *DrawChartTool) validate(chartType string, series []ChartSeries, labels []string, colors []string, height int) error {
	const op serrors.Op = "DrawChartTool.validate"

	// Validate pie/donut charts require single series
	if chartType == "pie" || chartType == "donut" {
		if len(series) != 1 {
			return serrors.E(op, fmt.Sprintf("%s charts require exactly one series, got %d", chartType, len(series)))
		}
	}

	// Validate max data points per series
	for i, s := range series {
		if len(s.Data) > 1000 {
			return serrors.E(op, fmt.Sprintf("series[%d] exceeds maximum 1000 data points: %d", i, len(s.Data)))
		}
	}

	// Validate labels count matches data points (if provided and not pie/donut)
	if len(labels) > 0 && chartType != "pie" && chartType != "donut" {
		if len(series) > 0 {
			dataPointCount := len(series[0].Data)
			if len(labels) != dataPointCount {
				return serrors.E(op, fmt.Sprintf("labels count (%d) does not match data points (%d)", len(labels), dataPointCount))
			}
		}
	}

	// Validate color hex codes
	for i, color := range colors {
		if !t.isValidHexColor(color) {
			return serrors.E(op, fmt.Sprintf("colors[%d] is not a valid hex color: %s", i, color))
		}
	}

	// Validate height bounds
	if height < 100 || height > 1000 {
		return serrors.E(op, fmt.Sprintf("height must be between 100-1000 pixels, got %d", height))
	}

	return nil
}

// isValidHexColor validates a hex color string.
func (t *DrawChartTool) isValidHexColor(color string) bool {
	// Remove whitespace
	color = strings.TrimSpace(color)
	// Hex color regex: #RGB, #RGBA, #RRGGBB, #RRGGBBAA
	hexColorPattern := regexp.MustCompile(`^#([A-Fa-f0-9]{3}|[A-Fa-f0-9]{4}|[A-Fa-f0-9]{6}|[A-Fa-f0-9]{8})$`)
	return hexColorPattern.MatchString(color)
}
