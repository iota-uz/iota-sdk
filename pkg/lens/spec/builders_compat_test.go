package spec

import (
	"encoding/json"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPanelBuilderHeightCompatibility(t *testing.T) {
	t.Parallel()

	panel := StackedBar("daily", "Daily", "daily").Height("320px").Build()

	require.Equal(t, "320px", panel.Height)
	payload, err := json.Marshal(panel)
	require.NoError(t, err)
	var wire map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(payload, &wire))
	height, ok := wire["height"]
	require.True(t, ok)
	assert.Equal(t, json.RawMessage(`"320px"`), height)
}

func TestPanelBuilderLegacyPresentationCompatibility(t *testing.T) {
	t.Parallel()

	legendCases := []struct {
		name     string
		position panel.LegendPosition
	}{
		{name: "top", position: panel.LegendTop},
		{name: "right", position: panel.LegendRight},
		{name: "bottom", position: panel.LegendBottom},
		{name: "left", position: panel.LegendLeft},
	}
	for _, tc := range legendCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			chart := Pie("products", "Products", "products").LegendAt(tc.position).Build()
			assert.Equal(t, tc.position, chart.LegendPosition)
		})
	}

	chart := Pie("products", "Products", "products").
		LegendWidth(300).
		LegendOffsetY(48).
		FloatingLegend().
		CircularScale(0.9).
		CircularOffsetX(-130).
		SemanticColors("product", "product").
		Build()

	require.Equal(t, 300, chart.LegendWidthPx)
	require.Equal(t, 48, chart.LegendOffsetY)
	require.True(t, chart.LegendFloating)
	require.InDelta(t, 0.9, chart.CircularScale, 0.001)
	require.Equal(t, -130, chart.CircularOffsetX)
	require.Equal(t, "product", chart.ColorScale)
	require.Equal(t, "product", chart.ColorField)
}
