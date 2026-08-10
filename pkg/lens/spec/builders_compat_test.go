package spec

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/stretchr/testify/require"
)

func TestPanelBuilderHeightCompatibility(t *testing.T) {
	t.Parallel()

	panel := StackedBar("daily", "Daily", "daily").Height("320px").Build()

	require.Equal(t, "320px", panel.Height)
}

func TestPanelBuilderLegacyPresentationCompatibility(t *testing.T) {
	t.Parallel()

	chart := Pie("products", "Products", "products").
		LegendAt(panel.LegendRight).
		LegendWidth(300).
		LegendOffsetY(48).
		FloatingLegend().
		CircularScale(0.9).
		CircularOffsetX(-130).
		SemanticColors("product", "product").
		Build()

	require.Equal(t, panel.LegendRight, chart.LegendPosition)
	require.Equal(t, 300, chart.LegendWidthPx)
	require.Equal(t, 48, chart.LegendOffsetY)
	require.True(t, chart.LegendFloating)
	require.InDelta(t, 0.9, chart.CircularScale, 0.001)
	require.Equal(t, -130, chart.CircularOffsetX)
	require.Equal(t, "product", chart.ColorScale)
	require.Equal(t, "product", chart.ColorField)
}
