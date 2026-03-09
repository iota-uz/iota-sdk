package frame

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFrameBehaviors_Scenarios(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Second)
	cases := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "normalize_rejects_uneven_fields",
			run: func(t *testing.T) {
				fr := &Frame{
					Name: "broken",
					Fields: []Field{
						{Name: "label", Values: []any{"a", "b"}},
						{Name: "value", Values: []any{1}},
					},
				}

				require.Error(t, fr.Normalize())
			},
		},
		{
			name: "rows_round_trip",
			run: func(t *testing.T) {
				fr, err := New("report",
					Field{Name: "label", Values: []any{"one", "two"}},
					Field{Name: "value", Values: []any{1.0, 2.0}},
					Field{Name: "at", Values: []any{now, now.Add(time.Hour)}},
				)
				require.NoError(t, err)

				rows := fr.Rows()
				require.Len(t, rows, 2)
				assert.Equal(t, "one", rows[0]["label"])
				assert.Equal(t, 2.0, rows[1]["value"])
				assert.Equal(t, now.Add(time.Hour), rows[1]["at"])
			},
		},
		{
			name: "long_series_preserves_extra_fields",
			run: func(t *testing.T) {
				set, err := LongSeries("sales",
					LongSeriesRow{
						Category: "2026-01-01",
						Series:   "OSAGO",
						Value:    12.5,
						Extra: map[string]any{
							"bucket_start": "2026-01-01",
							"product_id":   "prod-1",
						},
					},
				)
				require.NoError(t, err)

				rows := set.Primary().Rows()
				require.Len(t, rows, 1)
				assert.Equal(t, "2026-01-01", rows[0]["category"])
				assert.Equal(t, "OSAGO", rows[0]["series"])
				assert.Equal(t, "prod-1", rows[0]["product_id"])
			},
		},
		{
			name: "append_strict_requires_declared_fields",
			run: func(t *testing.T) {
				builder := NewBuilder("strict").
					String("label", RoleDimension).
					Number("value", RoleMetric)

				err := builder.AppendStrict(Row{"label": "Revenue"})
				require.Error(t, err)
				assert.Contains(t, err.Error(), "row missing field")
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.run(t)
		})
	}
}

func TestBuilderAppendStrictRejectsUnexpectedFields(t *testing.T) {
	t.Parallel()

	builder := NewBuilder("strict").
		String("label", RoleDimension).
		Number("value", RoleMetric)

	err := builder.AppendStrict(Row{"label": "Revenue", "value": 12.5, "extra": "nope"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected field")
}

func TestInferFieldTypeSupportsPointerTime(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	require.Equal(t, FieldTypeTime, InferFieldType(&now))
}
