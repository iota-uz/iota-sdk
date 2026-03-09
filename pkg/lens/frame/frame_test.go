package frame

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFrameNormalizeRejectsUnevenFields(t *testing.T) {
	t.Parallel()

	fr := &Frame{
		Name: "broken",
		Fields: []Field{
			{Name: "label", Values: []any{"a", "b"}},
			{Name: "value", Values: []any{1}},
		},
	}

	err := fr.Normalize()
	require.Error(t, err)
}

func TestFrameRowsRoundTrip(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Second)
	fr, err := New("report",
		Field{Name: "label", Values: []any{"one", "two"}},
		Field{Name: "value", Values: []any{1.0, 2.0}},
		Field{Name: "at", Values: []any{now, now.Add(time.Hour)}},
	)
	require.NoError(t, err)

	rows := fr.Rows()
	require.Len(t, rows, 2)
	require.Equal(t, "one", rows[0]["label"])
	require.Equal(t, 2.0, rows[1]["value"])
	require.Equal(t, now.Add(time.Hour), rows[1]["at"])
}

func TestBuilderLongSeriesPreservesExtraFields(t *testing.T) {
	t.Parallel()

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
	require.Equal(t, "2026-01-01", rows[0]["category"])
	require.Equal(t, "OSAGO", rows[0]["series"])
	require.Equal(t, "prod-1", rows[0]["product_id"])
}

func TestBuilderAppendStrictRequiresDeclaredFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		row  Row
	}{
		{
			name: "missing declared field",
			row:  Row{"label": "Revenue"},
		},
		{
			name: "unexpected extra field",
			row:  Row{"label": "Revenue", "value": 12.5, "extra": "nope"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			builder := NewBuilder("strict").
				String("label", RoleDimension).
				Number("value", RoleMetric)

			err := builder.AppendStrict(tt.row)
			require.Error(t, err)
		})
	}
}

func TestFieldCloneCopiesNestedValues(t *testing.T) {
	t.Parallel()

	field := Field{
		Name: "payload",
		Values: []any{
			map[string]any{
				"product": "osago",
				"meta":    []any{"a", "b"},
			},
		},
	}

	cloned := field.Clone()
	clonedMap := cloned.Values[0].(map[string]any)
	clonedMap["product"] = "kasko"
	clonedMap["meta"].([]any)[0] = "changed"

	originalMap := field.Values[0].(map[string]any)
	require.Equal(t, "osago", originalMap["product"])
	require.Equal(t, "a", originalMap["meta"].([]any)[0])
}

func TestInferFieldTypeSupportsPointerTime(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	require.Equal(t, FieldTypeTime, InferFieldType(&now))
}
