package apex

import (
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/stretchr/testify/require"
)

func TestBuildActionJSNormalizesTimeCategories(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("sales",
		frame.Field{Name: "category", Type: frame.FieldTypeTime, Values: []any{"2026-03-09T00:00:00Z"}},
		frame.Field{Name: "series", Type: frame.FieldTypeString, Values: []any{"Revenue"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{42.0}},
	)
	require.NoError(t, err)

	js := string(buildActionJS(
		&action.Spec{Kind: action.KindNavigate, URL: "/reports"},
		fr,
		panel.FieldMapping{Category: "category", Series: "series", Value: "value"},
		nil,
	))

	require.Contains(t, js, "normalizeCategoryValue")
	require.Contains(t, js, "toISOString().slice(0, 10)")
}

func TestBuildActionJSPreservesTimeValuesInConfig(t *testing.T) {
	t.Parallel()

	timestamp := time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC)
	fr, err := frame.New("sales",
		frame.Field{Name: "category", Type: frame.FieldTypeTime, Values: []any{timestamp}},
		frame.Field{Name: "series", Type: frame.FieldTypeString, Values: []any{"Revenue"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{42.0}},
	)
	require.NoError(t, err)

	js := string(buildActionJS(
		&action.Spec{Kind: action.KindNavigate, URL: "/reports"},
		fr,
		panel.FieldMapping{Category: "category", Series: "series", Value: "value"},
		map[string]any{"from": timestamp},
	))

	require.Contains(t, js, `"2026-03-09T00:00:00Z"`)
}

func TestBuildActionJSHonorsFallbacks(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("sales",
		frame.Field{Name: "category", Type: frame.FieldTypeString, Values: []any{"March"}},
		frame.Field{Name: "series", Type: frame.FieldTypeString, Values: []any{"Revenue"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{42.0}},
	)
	require.NoError(t, err)

	js := string(buildActionJS(
		&action.Spec{
			Kind: action.KindNavigate,
			URL:  "/reports",
			Params: []action.Param{
				{
					Name: "product",
					Source: action.ValueSource{
						Kind:     action.SourceField,
						Name:     "product_id",
						Fallback: "default-product",
					},
				},
			},
			Payload: map[string]action.ValueSource{
				"active_only": {
					Kind:     action.SourceVariable,
					Name:     "active_only",
					Fallback: true,
				},
			},
		},
		fr,
		panel.FieldMapping{Category: "category", Series: "series", Value: "value"},
		nil,
	))

	require.Contains(t, js, `resolveValue(row["product_id"], "default-product")`)
	require.Contains(t, js, `resolveValue(variables["active_only"], true)`)
}

func TestOptionsFallsBackToCategoryForPieLabels(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("sales",
		frame.Field{Name: "category", Type: frame.FieldTypeString, Values: []any{"OSAGO", "Travel"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{42.0, 18.0}},
	)
	require.NoError(t, err)

	options := Options(
		panel.Pie("sales-by-product", "Sales by Product", "sales").
			CategoryField("category").
			ValueField("value").
			Build(),
		&runtime.PanelResult{Frames: mustFrameSet(t, fr)},
	)

	require.Equal(t, []string{"OSAGO", "Travel"}, options.Labels)
}

func TestOptionsFallsBackToCategoryForUngroupedBarCategories(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("sales",
		frame.Field{Name: "category", Type: frame.FieldTypeString, Values: []any{"March", "April"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{42.0, 18.0}},
	)
	require.NoError(t, err)

	options := Options(
		panel.Bar("sales-by-month", "Sales by Month", "sales").
			CategoryField("category").
			ValueField("value").
			Build(),
		&runtime.PanelResult{Frames: mustFrameSet(t, fr)},
	)

	require.Equal(t, []string{"March", "April"}, options.XAxis.Categories)
}

func mustFrameSet(t *testing.T, fr *frame.Frame) *frame.FrameSet {
	t.Helper()

	set, err := frame.NewFrameSet(fr)
	require.NoError(t, err)
	return set
}
