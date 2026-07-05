package apex

import (
	"math"
	"testing"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/components/charts"
	"github.com/iota-uz/iota-sdk/pkg/lens/format"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/stretchr/testify/require"
)

func TestBuildDrillHierarchyJSReturnsEmptyForNilSpec(t *testing.T) {
	t.Parallel()

	require.Equal(t, templ.JSExpression(""), buildDrillHierarchyJS(nil, nil, "ru"))
}

func TestBuildDrillHierarchyJSEmbedsExpectedConfig(t *testing.T) {
	t.Parallel()

	spec := &panel.DrillHierarchy{
		Sources:     []string{"direct", "reinsurance"},
		OthersLabel: "Остальные",
		OthersYears: []int{2015, 2016},
		Years: map[string]float64{
			"2015|direct":      10,
			"2015|reinsurance": 20,
			"2016|direct":      30,
			"2016|reinsurance": 40,
		},
		Quarters: map[string]panel.QuarterBreakdown{
			"2015|direct": {
				Amounts:      [4]float64{1, 2, 3, 4},
				NavigateURLs: [4]string{"/a?q=1", "/a?q=2", "/a?q=3", "/a?q=4"},
			},
		},
	}
	formatterSpec := format.Money("UZS", 0)

	js := string(buildDrillHierarchyJS(spec, &formatterSpec, "ru"))

	require.Contains(t, js, "__lensDrillHierarchyClick")
	require.Contains(t, js, `"othersLabel":"Остальные"`)
	require.Contains(t, js, `"backLabel":"Назад"`)
	require.Contains(t, js, `"2015|direct"`)
	require.Contains(t, js, "/a?q=1")
	require.Contains(t, js, `"sources":["direct","reinsurance"]`)
}

func TestFloorAndScaleFloorsNonPositiveValuesBeforeScaling(t *testing.T) {
	t.Parallel()

	// Non-positive points get floored to a small positive epsilon before any
	// log transform, so the bar renders as a sliver rather than vanishing or
	// producing NaN/Inf once log-transformed.
	series, axis := floorAndScale([]charts.Series{
		{Data: []any{0.0, -5.0, 3.0}},
	})
	require.Equal(t, "log", axis.Scale)
	for _, point := range series[0].Data {
		value := numericValue(point)
		require.False(t, math.IsNaN(value) || math.IsInf(value, 0))
	}
}

func TestFloorAndScaleAppliesLogTransformForWideRangeData(t *testing.T) {
	t.Parallel()

	series, axis := floorAndScale([]charts.Series{
		{Data: []any{100.0, 1_000_000.0}},
	})
	require.Equal(t, "log", axis.Scale)
	require.Equal(t, 10, axis.Base)
	require.Positive(t, axis.TickAmount)
	require.NotEmpty(t, series)
}

func TestOptionsWiresDrillHierarchyDataPointSelection(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("premium",
		frame.Field{Name: "category", Type: frame.FieldTypeString, Values: []any{"2025", "2025"}},
		frame.Field{Name: "series", Type: frame.FieldTypeString, Values: []any{"Direct", "Reinsurance"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{120.0, 880.0}},
	)
	require.NoError(t, err)
	frameSet, err := frame.NewFrameSet(fr)
	require.NoError(t, err)

	spec := panel.Bar("premium-panel", "Premium", "premium").
		CategoryField("category").
		SeriesField("series").
		ValueField("value").
		DrillHierarchy(panel.DrillHierarchy{Sources: []string{"direct", "reinsurance"}}).
		Build()

	opts := Options(spec, &runtime.PanelResult{Frames: frameSet, Locale: "ru"})
	require.NotNil(t, opts.Chart.Events)
	require.Contains(t, string(opts.Chart.Events.DataPointSelection), "__lensDrillHierarchyClick")
	require.Contains(t, string(opts.Chart.Events.Mounted), "__lensDrillHierarchyMount")
}
