package panel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// allKinds lists the panel kinds covered by the predicate tests. Keep it in
// sync with the const block above so the table and partition checks cover every
// known kind.
var allKinds = []Kind{
	KindStat,
	KindTimeSeries,
	KindBar,
	KindHorizontalBar,
	KindStackedBar,
	KindSegmentBar,
	KindCascade,
	KindPie,
	KindDonut,
	KindRadial,
	KindTable,
	KindGauge,
	KindHistogram,
	KindBoxPlot,
	KindHeatmap,
	KindMap,
	KindTabs,
	KindGrid,
	KindSplit,
	KindRepeat,
}

func TestKindPredicates_ClassifiesAllKnownKinds(t *testing.T) {
	cases := []struct {
		kind        Kind
		isContainer bool
	}{
		{KindStat, false}, {KindTimeSeries, false}, {KindBar, false}, {KindHorizontalBar, false},
		{KindStackedBar, false}, {KindSegmentBar, false}, {KindCascade, false}, {KindPie, false},
		{KindDonut, false}, {KindRadial, false}, {KindTable, false}, {KindGauge, false},
		{KindHistogram, false}, {KindBoxPlot, false}, {KindHeatmap, false}, {KindMap, false},
		{KindTabs, true}, {KindGrid, true}, {KindSplit, true}, {KindRepeat, true},
	}

	require.Len(t, cases, len(allKinds), "predicate table should cover allKinds")
	seen := make(map[Kind]int, len(cases))
	for _, tc := range cases {
		seen[tc.kind]++
	}
	for _, kind := range allKinds {
		require.Equalf(t, 1, seen[kind], "predicate table should include %q exactly once", kind)
	}

	for _, tc := range cases {
		t.Run(string(tc.kind), func(t *testing.T) {
			assert.Equal(t, tc.isContainer, tc.kind.IsContainer())
		})
	}
}

func TestDistributionBuildersCarryExplicitFields(t *testing.T) {
	histogram := Histogram("claims", "Claim amount", "buckets").CategoryField("bucket").ValueField("count").Build()
	require.Equal(t, KindHistogram, histogram.Kind)

	boxplot := BoxPlot("settlement", "Settlement days", "summary").
		CategoryField("product").BoxFields("min", "q1", "median", "q3", "max").Build()
	require.Equal(t, KindBoxPlot, boxplot.Kind)
	require.Equal(t, FieldRef("median"), boxplot.Fields.Median)

	heatmap := Heatmap("region-product", "Region × product", "cells").
		CategoryField("region").SeriesField("product").ValueField("premium").Build()
	require.Equal(t, KindHeatmap, heatmap.Kind)
}

func TestChoroplethBuilderCarriesExplicitJoin(t *testing.T) {
	geometry := &GeoJSONFeatureCollection{Type: "FeatureCollection", Features: []GeoJSONFeature{{
		Type: "Feature", Properties: map[string]any{"code": "north"},
		Geometry: map[string]any{"type": "Polygon", "coordinates": []any{}},
	}}}
	spec := Choropleth("regions", "Regions", "regional", GeoJSONSource{Inline: geometry}, "code").
		MapAttribution("© Example Maps").ComparisonUnsupported().
		IDField("region_code").ValueField("premium").Build()

	require.Equal(t, KindMap, spec.Kind)
	require.Equal(t, "code", spec.Map.FeatureProperty)
	require.Equal(t, "© Example Maps", spec.Map.Attribution)
	require.True(t, spec.ComparisonUnsupported)
	require.Same(t, geometry, spec.Map.Source.Inline)
}

func TestTrendDeclaresPercentagePointAbsoluteDelta(t *testing.T) {
	t.Parallel()
	spec := Stat("loss-ratio", "Loss ratio", "ratio").
		AutoTrend("ratio_delta", "ratio_delta_percent", "Comparison", true).
		TrendAbsoluteDeltaUnit(TrendDeltaPercentagePoints).
		Build()
	require.Equal(t, TrendDeltaPercentagePoints, spec.Trend.AbsoluteDeltaUnit)
}

func TestRadialNodeKey_ContainsRingAndCategoryIdentity(t *testing.T) {
	require.Equal(t, `radial:["year/2026","direct"]`, RadialNodeKey("year/2026", "direct"))
	require.NotEqual(t, RadialNodeKey("year/2026", "direct"), RadialNodeKey("year/2025", "direct"))
}

func TestPresentationHints_DataLabelsAreOptIn(t *testing.T) {
	require.True(t, PresentationHints{}.IsZero())
	require.False(t, PresentationHints{DataLabels: true}.IsZero())
}

func TestTimeSeriesTemporalBuilderCarriesGenericOverlays(t *testing.T) {
	spec := TimeSeries("trend", "Trend", "daily").
		Regression(FieldRef("regression"), "Trend").
		MovingAverage(7, FieldRef("sma_7"), "SMA 7").
		ReferenceLine(100, "Target").
		IncompletePeriod("2026", TemporalPeriodYTD, "YTD", FieldRef("annualized")).
		AnnotateTime("2026-03-01", "Product launch").
		Forecast("2026-08-01", FieldRef("forecast"), FieldRef("lower"), FieldRef("upper"), "Forecast").
		Build()

	require.NotNil(t, spec.Temporal)
	require.Equal(t, FieldRef("regression"), spec.Temporal.RegressionField)
	require.Equal(t, 7, spec.Temporal.MovingAverages[0].Window)
	require.InDelta(t, 100.0, spec.Temporal.ReferenceLines[0].Value, 1e-9)
	require.Equal(t, TemporalPeriodYTD, spec.Temporal.Period.State)
	require.Equal(t, "Product launch", spec.Temporal.Annotations[0].Label)
	require.Equal(t, FieldRef("upper"), spec.Temporal.Forecast.UpperField)
}

// TestRadialNodeKeyMatchesJSONStringify pins the key format against the one
// the runtime builds with JSON.stringify. encoding/json escapes `<`, `>` and
// `&` by default, which would give the two sides different identities for the
// same segment.
func TestRadialNodeKeyMatchesJSONStringify(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		ring     string
		category string
		want     string
	}{
		{name: "plain", ring: "plan", category: "north", want: `radial:["plan","north"]`},
		{name: "ampersand", ring: "plan", category: "A&B", want: `radial:["plan","A&B"]`},
		{name: "angle brackets", ring: "<plan>", category: "north", want: `radial:["<plan>","north"]`},
		{name: "quote", ring: "plan", category: `"q"`, want: `radial:["plan","\"q\""]`},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.want, RadialNodeKey(testCase.ring, testCase.category))
		})
	}
}
