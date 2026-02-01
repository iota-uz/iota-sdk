package codecs

import (
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/bichat/context"
)

// ChartDataPayload represents chart data block (BI-specific).
type ChartDataPayload struct {
	ChartType string           `json:"chart_type"`
	Title     string           `json:"title"`
	Data      []map[string]any `json:"data"`
	Config    map[string]any   `json:"config,omitempty"`
}

// ChartDataCodec handles chart data blocks for BI use cases.
type ChartDataCodec struct {
	*context.BaseCodec
}

// NewChartDataCodec creates a new chart data codec.
func NewChartDataCodec() *ChartDataCodec {
	return &ChartDataCodec{
		BaseCodec: context.NewBaseCodec("chart-data", "1.0.0"),
	}
}

// Validate validates the chart data payload.
func (c *ChartDataCodec) Validate(payload any) error {
	switch v := payload.(type) {
	case ChartDataPayload:
		if v.ChartType == "" {
			return fmt.Errorf("chart type cannot be empty")
		}
		if len(v.Data) == 0 {
			return fmt.Errorf("chart data cannot be empty")
		}
		return nil
	case map[string]any:
		if chartType, ok := v["chart_type"].(string); !ok || chartType == "" {
			return fmt.Errorf("chart type cannot be empty")
		}
		if data, ok := v["data"].([]any); !ok || len(data) == 0 {
			return fmt.Errorf("chart data cannot be empty")
		}
		return nil
	default:
		return fmt.Errorf("invalid chart data payload type: %T", payload)
	}
}

// Canonicalize converts the payload to canonical form.
func (c *ChartDataCodec) Canonicalize(payload any) ([]byte, error) {
	var chart ChartDataPayload

	switch v := payload.(type) {
	case ChartDataPayload:
		chart = v
	case map[string]any:
		if chartType, ok := v["chart_type"].(string); ok {
			chart.ChartType = chartType
		}
		if title, ok := v["title"].(string); ok {
			chart.Title = normalizeWhitespace(title)
		}
		if data, ok := v["data"].([]any); ok {
			for _, item := range data {
				if itemMap, ok := item.(map[string]any); ok {
					chart.Data = append(chart.Data, itemMap)
				}
			}
		}
		if config, ok := v["config"].(map[string]any); ok {
			chart.Config = config
		}
	default:
		return nil, fmt.Errorf("invalid chart data payload type: %T", payload)
	}

	return context.SortedJSONBytes(chart)
}
