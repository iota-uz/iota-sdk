package chart

import (
	"context"
	"fmt"
	tools "github.com/iota-uz/iota-sdk/pkg/bichat/tools"
	"math"
	"regexp"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
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

	if params.ChartType == "" {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: "chartType parameter is required",
				Hints:   []string{tools.HintCheckRequiredFields, "Valid chart types: line, bar, pie, area, donut"},
			},
		}, nil
	}
	if params.Title == "" {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: "title parameter is required",
				Hints:   []string{tools.HintCheckRequiredFields, "Provide a descriptive title for the chart"},
			},
		}, nil
	}
	if len(params.Series) == 0 {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: "series parameter is required and must not be empty",
				Hints:   []string{tools.HintCheckRequiredFields, "Provide at least one data series with name and data array"},
			},
		}, nil
	}

	if params.Height == 0 {
		params.Height = 350
	}

	if err := t.validateChartType(params.ChartType); err != nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: err.Error(),
				Hints:   []string{"Valid chart types: line, bar, pie, area, donut", tools.HintCheckFieldFormat},
			},
		}, nil
	}

	if err := t.validate(params.ChartType, params.Series, params.Labels, params.Colors, params.Height); err != nil {
		return &types.ToolResult{
			CodecID: types.CodecToolError,
			Payload: types.ToolErrorPayload{
				Code:    string(tools.ErrCodeInvalidRequest),
				Message: err.Error(),
				Hints:   []string{tools.HintCheckFieldFormat, "Verify data arrays, labels, and color codes"},
			},
		}, nil
	}

	return &types.ToolResult{
		CodecID: types.CodecJSON,
		Payload: types.JSONPayload{Output: params},
		Artifacts: []types.ToolArtifact{
			{
				Type:        "chart",
				Name:        params.Title,
				Description: fmt.Sprintf("%s chart", params.ChartType),
				MimeType:    "application/json",
				Metadata: map[string]any{
					"spec": params,
				},
			},
		},
	}, nil
}

// Call executes the chart creation and returns a chart specification.
func (t *DrawChartTool) Call(ctx context.Context, input string) (string, error) {
	return tools.FormatStructuredResult(t.CallStructured(ctx, input))
}

// validateChartType validates the chart type is supported.
func (t *DrawChartTool) validateChartType(chartType string) error {
	validTypes := map[string]bool{
		"line":  true,
		"bar":   true,
		"pie":   true,
		"area":  true,
		"donut": true,
	}

	if !validTypes[chartType] {
		return fmt.Errorf("unsupported chart type: %s", chartType)
	}

	return nil
}

// validate validates the chart parameters.
func (t *DrawChartTool) validate(chartType string, series []ChartSeries, labels []string, colors []string, height int) error {
	// Validate pie/donut charts require single series
	if chartType == "pie" || chartType == "donut" {
		if len(series) != 1 {
			return fmt.Errorf("%s charts require exactly one series, got %d", chartType, len(series))
		}
	}

	// Validate per-series constraints
	for i, s := range series {
		// Fix 3: Reject empty data arrays
		if len(s.Data) == 0 {
			return fmt.Errorf("series[%d] has no data points", i)
		}

		// Validate max data points per series
		if len(s.Data) > 1000 {
			return fmt.Errorf("series[%d] exceeds maximum 1000 data points: %d", i, len(s.Data))
		}

		// Fix 2: Validate data values are numeric
		for j, v := range s.Data {
			f, ok := v.(float64)
			if !ok {
				return fmt.Errorf("series[%d].data[%d] is not a number", i, j)
			}
			if math.IsNaN(f) || math.IsInf(f, 0) {
				return fmt.Errorf("series[%d].data[%d] is NaN or Infinity", i, j)
			}
		}

		// Fix 4: Reject negative values for pie/donut
		if chartType == "pie" || chartType == "donut" {
			for j, v := range s.Data {
				if f, ok := v.(float64); ok && f < 0 {
					return fmt.Errorf("series[%d].data[%d] is negative (%.2f); pie/donut charts require non-negative values", i, j, f)
				}
			}
		}
	}

	// Fix 6: Validate cross-series data length consistency
	if chartType != "pie" && chartType != "donut" && len(series) > 1 {
		expectedLen := len(series[0].Data)
		for i := 1; i < len(series); i++ {
			if len(series[i].Data) != expectedLen {
				return fmt.Errorf("series[%d] has %d data points but series[0] has %d; all series must have equal length",
					i, len(series[i].Data), expectedLen)
			}
		}
	}

	// Fix 5: Validate labels count for all chart types
	if len(labels) > 0 && len(series) > 0 {
		dataPointCount := len(series[0].Data)
		if len(labels) != dataPointCount {
			return fmt.Errorf("labels count (%d) does not match data points (%d)", len(labels), dataPointCount)
		}
	}

	// Validate color hex codes
	for i, color := range colors {
		if !t.isValidHexColor(color) {
			return fmt.Errorf("colors[%d] is not a valid hex color: %s", i, color)
		}
	}

	// Validate height bounds
	if height < 100 || height > 1000 {
		return fmt.Errorf("height must be between 100-1000 pixels, got %d", height)
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
