package transform

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFillMissing_ZeroFillsSparseSeries(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("sales")
	require.NoError(t, err)
	require.NoError(t, fr.AppendRow(map[string]any{"category": "2025-01", "series": "A", "value": 10.0}))
	require.NoError(t, fr.AppendRow(map[string]any{"category": "2025-01", "series": "B", "value": 20.0}))
	require.NoError(t, fr.AppendRow(map[string]any{"category": "2025-02", "series": "A", "value": 15.0}))
	require.NoError(t, fr.Normalize())

	set, err := frame.NewFrameSet(fr)
	require.NoError(t, err)

	next, err := Apply(set, nil, []Spec{{
		Kind: KindFillMissing,
		FillMissing: &FillMissingConfig{
			CategoryField: "category",
			SeriesField:   "series",
			ValueField:    "value",
			FillValue:     0.0,
		},
	}})
	require.NoError(t, err)

	rows := next.Primary().Rows()
	require.Len(t, rows, 4)
	assert.Contains(t, rows, map[string]any{"category": "2025-02", "series": "B", "value": 0.0})
}

func TestGroupBy_AggregatesRows(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("sales")
	require.NoError(t, err)
	require.NoError(t, fr.AppendRow(map[string]any{"region": "Tashkent", "amount": 10.0}))
	require.NoError(t, fr.AppendRow(map[string]any{"region": "Tashkent", "amount": 20.0}))
	require.NoError(t, fr.AppendRow(map[string]any{"region": "Samarkand", "amount": 5.0}))
	require.NoError(t, fr.Normalize())

	set, err := frame.NewFrameSet(fr)
	require.NoError(t, err)

	next, err := Apply(set, nil, []Spec{{
		Kind:    KindGroupBy,
		GroupBy: []string{"region"},
		Aggregates: []Aggregate{
			{Field: "amount", As: "total", Func: "sum"},
		},
	}})
	require.NoError(t, err)

	rows := next.Primary().Rows()
	require.Len(t, rows, 2)
	assert.Contains(t, rows, map[string]any{"region": "Tashkent", "total": 30.0})
	assert.Contains(t, rows, map[string]any{"region": "Samarkand", "total": 5.0})
}

func TestMoneyScaleAndAgeRangeTransforms(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("contracts")
	require.NoError(t, err)
	require.NoError(t, fr.AppendRow(map[string]any{"amount_minor": 12345.0, "age_range": "65+"}))
	require.NoError(t, fr.Normalize())

	set, err := frame.NewFrameSet(fr)
	require.NoError(t, err)

	next, err := Apply(set, nil, []Spec{
		MoneyScale("amount_minor", "amount", 100),
		ParseAgeRange("age_range", "age_min", "age_max"),
	})
	require.NoError(t, err)

	rows := next.Primary().Rows()
	require.Len(t, rows, 1)
	require.InDelta(t, 123.45, rows[0]["amount"].(float64), 0.001)
	require.Equal(t, 65, rows[0]["age_min"])
	require.Equal(t, 999, rows[0]["age_max"])
}

func TestBucketBoundsTransformAddsDateWindow(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("buckets")
	require.NoError(t, err)
	require.NoError(t, fr.AppendRow(map[string]any{"bucket_at": time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)}))
	require.NoError(t, fr.Normalize())

	set, err := frame.NewFrameSet(fr)
	require.NoError(t, err)

	next, err := Apply(set, nil, []Spec{{
		Kind: KindBucketBounds,
		BucketBounds: &BucketBoundsConfig{
			Field:       "bucket_at",
			Granularity: "month",
			StartAs:     "bucket_start",
			EndAs:       "bucket_end",
		},
	}})
	require.NoError(t, err)

	rows := next.Primary().Rows()
	require.Len(t, rows, 1)
	require.Equal(t, "2026-03-01", rows[0]["bucket_start"])
	require.Equal(t, "2026-03-31", rows[0]["bucket_end"])
}

func TestJoin_LeftPreservesRowsAndExpandsDuplicateMatches(t *testing.T) {
	t.Parallel()

	left, err := frame.FromRows("left",
		frame.Row{"id": "a", "label": "Alpha"},
		frame.Row{"id": "b", "label": "Beta"},
	)
	require.NoError(t, err)
	right, err := frame.FromRows("right",
		frame.Row{"id": "a", "score": 1.0},
		frame.Row{"id": "a", "score": 2.0},
	)
	require.NoError(t, err)

	next, err := Apply(left, map[string]*frame.FrameSet{"right": right}, []Spec{{
		Kind: KindJoin,
		Join: &JoinConfig{Other: "right", On: []string{"id"}, How: "left"},
	}})
	require.NoError(t, err)

	rows := next.Primary().Rows()
	require.Len(t, rows, 3)
	assert.Contains(t, rows, map[string]any{"id": "a", "label": "Alpha", "right_id": "a", "score": 1.0})
	assert.Contains(t, rows, map[string]any{"id": "a", "label": "Alpha", "right_id": "a", "score": 2.0})
	assert.Contains(t, rows, map[string]any{"id": "b", "label": "Beta", "right_id": nil, "score": nil})
}

func TestFormulaDivisionMarksZeroBaselineUnavailable(t *testing.T) {
	t.Parallel()
	set, err := frame.FromRows("metrics",
		frame.Row{"delta": 12.0, "baseline": 0.0},
		frame.Row{"delta": 12.0, "baseline": 4.0},
	)
	require.NoError(t, err)
	next, err := Apply(set, nil, []Spec{{
		Kind: KindFormula, Formula: &Formula{As: "ratio", Op: "/", Left: "delta", Right: "baseline"},
	}})
	require.NoError(t, err)
	rows := next.Primary().Rows()
	require.Nil(t, rows[0]["ratio"], "a zero baseline is unavailable, not an authoritative zero change")
	require.Equal(t, 3.0, rows[1]["ratio"])
}

func TestFormulaDivisionCanUseAbsoluteBaseline(t *testing.T) {
	t.Parallel()
	set, err := frame.FromRows("metrics", frame.Row{"delta": 40.0, "baseline": -100.0})
	require.NoError(t, err)
	next, err := Apply(set, nil, []Spec{{
		Kind: KindFormula, Formula: &Formula{
			As: "ratio", Op: "/", Left: "delta", Right: "baseline", AbsoluteRight: true,
		},
	}})
	require.NoError(t, err)
	require.Equal(t, 0.4, next.Primary().Rows()[0]["ratio"])
}

func TestFilterRows_ParsesNumericStrings(t *testing.T) {
	t.Parallel()

	set, err := frame.FromRows("metrics",
		frame.Row{"label": "a", "value": "10.5"},
		frame.Row{"label": "b", "value": "2"},
	)
	require.NoError(t, err)

	next, err := Apply(set, nil, []Spec{{
		Kind: KindFilterRows,
		Predicates: []Predicate{{
			Field: "value",
			Op:    ">",
			Value: 5,
		}},
	}})
	require.NoError(t, err)

	rows := next.Primary().Rows()
	require.Len(t, rows, 1)
	require.Equal(t, "a", rows[0]["label"])
}

func TestTopN_AggregatesOverflowIntoOtherBucket(t *testing.T) {
	t.Parallel()

	set, err := frame.FromRows("products",
		frame.Row{"filter_value": "osago", "label": "OSAGO", "color_value": "osago", "total_policies": 50.0, "total_revenue": 100.0},
		frame.Row{"filter_value": "travel", "label": "Travel", "color_value": "travel", "total_policies": 30.0, "total_revenue": 60.0},
		frame.Row{"filter_value": "kasko", "label": "KASKO", "color_value": "kasko", "total_policies": 20.0, "total_revenue": 40.0},
		frame.Row{"filter_value": "osgor", "label": "OSGOR", "color_value": "osgor", "total_policies": 10.0, "total_revenue": 20.0},
	)
	require.NoError(t, err)

	next, err := Apply(set, nil, []Spec{
		TopN("total_policies", 2, "Other"),
	})
	require.NoError(t, err)

	rows := next.Primary().Rows()
	require.Len(t, rows, 3)
	assert.Equal(t, map[string]any{
		"filter_value":   "osago",
		"label":          "OSAGO",
		"color_value":    "osago",
		"total_policies": 50.0,
		"total_revenue":  100.0,
	}, rows[0])
	assert.Equal(t, map[string]any{
		"filter_value":   "travel",
		"label":          "Travel",
		"color_value":    "travel",
		"total_policies": 30.0,
		"total_revenue":  60.0,
	}, rows[1])
	assert.Equal(t, map[string]any{
		"filter_value":   "",
		"label":          "Other",
		"color_value":    "Other",
		"total_policies": 30.0,
		"total_revenue":  60.0,
	}, rows[2])
}

func TestTopN_RankDatasetKeepsPrimaryPeriodMembershipAndOrder(t *testing.T) {
	t.Parallel()

	current, err := frame.FromRows("current",
		frame.Row{"filter_value": "a", "label": "A", "value": 100.0},
		frame.Row{"filter_value": "b", "label": "B", "value": 90.0},
		frame.Row{"filter_value": "", "label": "Other", "value": 10.0},
	)
	require.NoError(t, err)
	previous, err := frame.FromRows("previous",
		frame.Row{"filter_value": "c", "label": "C", "value": 1000.0},
		frame.Row{"filter_value": "b", "label": "B", "value": 2.0},
		frame.Row{"filter_value": "a", "label": "A", "value": 1.0},
		frame.Row{"filter_value": "d", "label": "D", "value": 500.0},
	)
	require.NoError(t, err)

	next, err := Apply(previous, map[string]*frame.FrameSet{"current": current}, []Spec{{
		Kind: KindTopN,
		TopN: &TopNConfig{
			Field: "value", N: 2, Other: "Other", AdditiveFields: map[string]bool{"value": true},
			RankByDataset: "current", KeyFields: []string{"filter_value", "label"},
		},
	}})
	require.NoError(t, err)
	require.Equal(t, []map[string]any{
		{"filter_value": "a", "label": "A", "value": 1.0},
		{"filter_value": "b", "label": "B", "value": 2.0},
		{"filter_value": "", "label": "Other", "value": 1500.0},
	}, next.Primary().Rows())
}

func TestExpandableTopN_PreservesAndTagsRemainderRows(t *testing.T) {
	t.Parallel()

	set, err := frame.FromRows("products",
		frame.Row{"label": "OSAGO", "value": 100.0},
		frame.Row{"label": "Travel", "value": 10.0},
		frame.Row{"label": "KASKO", "value": 2.0},
	)
	require.NoError(t, err)

	next, err := Apply(set, nil, []Spec{ExpandableTopN("value", 1, "Other")})
	require.NoError(t, err)
	require.Equal(t, []any{nil, "Other", "Other"}, next.Primary().MustField(TopNGroupField).Values)
	require.Len(t, next.Primary().Rows(), 3, "expandable mode must retain the tail source rows")
}

func TestTopN_DoesNotFabricateOtherForOneRemainingCategory(t *testing.T) {
	t.Parallel()

	set, err := frame.FromRows("products",
		frame.Row{"label": "OSAGO", "value": 100.0},
		frame.Row{"label": "Travel", "value": 10.0},
		frame.Row{"label": "KASKO", "value": 1.0},
	)
	require.NoError(t, err)

	collapsed, err := Apply(set, nil, []Spec{TopN("value", 2, "Other")})
	require.NoError(t, err)
	require.Equal(t, []any{"OSAGO", "Travel", "KASKO"}, collapsed.Primary().MustField("label").Values)

	expandable, err := Apply(set, nil, []Spec{ExpandableTopN("value", 2, "Other")})
	require.NoError(t, err)
	require.Equal(t, []any{nil, nil, nil}, expandable.Primary().MustField(TopNGroupField).Values)
}

func TestTopN_PreservesNonAdditiveMeasuresInOtherBucket(t *testing.T) {
	t.Parallel()

	set, err := frame.FromRows("products",
		frame.Row{"filter_value": "osago", "label": "OSAGO", "avg_ticket": 50.0, "total_policies": 5.0},
		frame.Row{"filter_value": "travel", "label": "Travel", "avg_ticket": 30.0, "total_policies": 4.0},
		frame.Row{"filter_value": "kasko", "label": "KASKO", "avg_ticket": 20.0, "total_policies": 3.0},
	)
	require.NoError(t, err)

	next, err := Apply(set, nil, []Spec{{
		Kind: KindTopN,
		TopN: &TopNConfig{
			Field: "total_policies",
			N:     1,
			Other: "Other",
			AdditiveFields: map[string]bool{
				"total_policies": true,
				"avg_ticket":     false,
			},
		},
	}})
	require.NoError(t, err)

	rows := next.Primary().Rows()
	require.Len(t, rows, 2)
	assert.InDelta(t, 50.0, rows[0]["avg_ticket"], 1e-9)
	assert.Nil(t, rows[1]["avg_ticket"])
	assert.InDelta(t, 7.0, rows[1]["total_policies"], 1e-9)
}

func TestMovingAverage_AppendsExportableSMAAndKeepsSeriesIndependent(t *testing.T) {
	t.Parallel()

	set, err := frame.FromRows("daily",
		frame.Row{"series": "A", "value": 3.0},
		frame.Row{"series": "B", "value": 30.0},
		frame.Row{"series": "A", "value": 6.0},
		frame.Row{"series": "B", "value": 60.0},
		frame.Row{"series": "A", "value": 9.0},
		frame.Row{"series": "B", "value": 90.0},
	)
	require.NoError(t, err)

	next, err := Apply(set, nil, []Spec{MovingAverageBy("value", "series", "sma_3", 3)})
	require.NoError(t, err)
	require.Equal(t, []any{nil, nil, nil, nil, 6.0, 60.0}, next.Primary().MustField("sma_3").Values)
	require.Equal(t, []any{3.0, 30.0, 6.0, 60.0, 9.0, 90.0}, next.Primary().MustField("value").Values)
}

func TestLinearRegression_AppendsLeastSquaresTrend(t *testing.T) {
	t.Parallel()

	set, err := frame.FromRows("trend",
		frame.Row{"value": 2.0},
		frame.Row{"value": 5.0},
		frame.Row{"value": 8.0},
		frame.Row{"value": 11.0},
	)
	require.NoError(t, err)

	next, err := Apply(set, nil, []Spec{LinearRegression("value", "regression")})
	require.NoError(t, err)
	require.Equal(t, []any{2.0, 5.0, 8.0, 11.0}, next.Primary().MustField("regression").Values)
}

func TestLinearForecast_AppendsDeterministicProjectionAndPredictionInterval(t *testing.T) {
	t.Parallel()

	set, err := frame.FromRows("monthly",
		frame.Row{"month": "2026-01-01T00:00:00Z", "series": "Observed", "value": 10.0},
		frame.Row{"month": "2026-02-01T00:00:00Z", "series": "Observed", "value": 12.0},
		frame.Row{"month": "2026-03-01T00:00:00Z", "series": "Observed", "value": 13.0},
		frame.Row{"month": "2026-04-01T00:00:00Z", "series": "Observed", "value": 17.0},
		frame.Row{"month": "2026-05-01T00:00:00Z", "series": "Observed", "value": 18.0},
	)
	require.NoError(t, err)

	next, err := Apply(set, nil, []Spec{
		LinearForecast("month", "value", "series", "forecast", "forecast_lower", "forecast_upper", "month", 3),
	})
	require.NoError(t, err)
	fr := next.Primary()
	require.Equal(t, 8, fr.RowCount)
	require.Equal(t, []any{
		"2026-06-01T00:00:00Z", "2026-07-01T00:00:00Z", "2026-08-01T00:00:00Z",
	}, fr.MustField("month").Values[5:])
	require.Equal(t, []any{nil, nil, nil}, fr.MustField("value").Values[5:])
	require.Equal(t, []any{"Observed", "Observed", "Observed"}, fr.MustField("series").Values[5:])

	forecast := fr.MustField("forecast").Values
	lower := fr.MustField("forecast_lower").Values
	upper := fr.MustField("forecast_upper").Values
	require.InDelta(t, 20.3, forecast[5], 1e-9)
	require.InDelta(t, 22.4, forecast[6], 1e-9)
	require.InDelta(t, 24.5, forecast[7], 1e-9)
	require.InDelta(t, 18.039, lower[5], 1e-3)
	require.InDelta(t, 22.561, upper[5], 1e-3)
	require.Less(t, lower[7].(float64), forecast[7].(float64))
	require.Greater(t, upper[7].(float64), forecast[7].(float64))
	for index := 0; index < 5; index++ {
		require.Nil(t, forecast[index], "observed rows must not be relabelled as forecast")
	}

	fixtureBytes, err := os.ReadFile(filepath.Join("..", "..", "..", "web", "lens", "fixtures", "linear-forecast.json"))
	require.NoError(t, err)
	var fixture struct {
		Rows [][]any `json:"rows"`
	}
	require.NoError(t, json.Unmarshal(fixtureBytes, &fixture))
	actualRows := make([][]any, fr.RowCount)
	for row := range actualRows {
		actualRows[row] = []any{
			fr.MustField("month").Values[row], fr.MustField("series").Values[row], fr.MustField("value").Values[row],
			forecast[row], lower[row], upper[row],
		}
	}
	require.Equal(t, fixture.Rows, actualRows, "the visible story fixture must be the transform's exact deterministic output")
}

func TestTemporalTransforms_ValidateConfiguration(t *testing.T) {
	t.Parallel()

	set, err := frame.FromRows("trend", frame.Row{"value": 1.0})
	require.NoError(t, err)
	_, err = Apply(set, nil, []Spec{MovingAverage("value", "sma", 0)})
	require.ErrorContains(t, err, "positive window")
	_, err = Apply(set, nil, []Spec{LinearRegression("missing", "trend")})
	require.ErrorContains(t, err, "not found")
	_, err = Apply(set, nil, []Spec{LinearForecast("missing", "value", "", "forecast", "lower", "upper", "month", 2)})
	require.ErrorContains(t, err, "category field")
}
