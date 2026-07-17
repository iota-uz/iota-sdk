package apex

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/stretchr/testify/require"
)

func TestChainChartEvent_PreservesBothHandlers(t *testing.T) {
	t.Parallel()

	combined := string(chainChartEvent("function(){ window.first = true; }", "function(){ window.second = true; }"))
	require.Contains(t, combined, "window.first = true")
	require.Contains(t, combined, "window.second = true")
	require.Contains(t, combined, "first.apply(this, arguments)")
	require.Contains(t, combined, "second.apply(this, arguments)")
}

func TestPie_SupportsRightLegendWithMobileBottomFallback(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("composition",
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"A", "B"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{70.0, 30.0}},
	)
	require.NoError(t, err)
	frameSet, err := frame.NewFrameSet(fr)
	require.NoError(t, err)
	spec := panel.Pie("composition-panel", "Composition", "composition").
		LegendAt(panel.LegendRight).
		LegendWidth(300).
		Build()

	opts := Options(spec, &runtime.PanelResult{Frames: frameSet, Locale: "ru"})
	require.NotNil(t, opts.Legend)
	require.NotNil(t, opts.Legend.Position)
	require.Equal(t, "right", string(*opts.Legend.Position))
	require.NotNil(t, opts.Legend.Width)
	require.Equal(t, 300, *opts.Legend.Width)
	require.Len(t, opts.Responsive, 1)
	require.NotNil(t, opts.Responsive[0].Options.Legend)
	require.Equal(t, "bottom", string(*opts.Responsive[0].Options.Legend.Position))
}
