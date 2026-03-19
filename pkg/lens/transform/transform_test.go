package transform

import (
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
	assert.Equal(t, 50.0, rows[0]["avg_ticket"])
	assert.Nil(t, rows[1]["avg_ticket"])
	assert.Equal(t, 7.0, rows[1]["total_policies"])
}
