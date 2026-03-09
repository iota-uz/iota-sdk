package filter

import (
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/stretchr/testify/require"
)

func TestBuildNormalizesDateRangeAndAllTime(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.March, 9, 23, 59, 59, 0, time.UTC)

	model := Build([]lens.VariableSpec{
		lens.DateRangeVariable("range", "Range", 24*time.Hour),
	}, map[string]any{
		"range": lens.DateRangeValue{
			Mode:  "all",
			Start: &start,
			End:   &end,
		},
	})

	require.Len(t, model.Inputs, 1)
	require.Equal(t, "all", model.Inputs[0].DateRange.Mode)
	require.True(t, model.Inputs[0].DateRange.AllowAllTime)
	require.Equal(t, "2026-03-01", model.Inputs[0].DateRange.Start)
	require.Equal(t, "2026-03-09", model.Inputs[0].DateRange.End)
}

func TestBuildNormalizesOptionSelection(t *testing.T) {
	t.Parallel()

	model := Build([]lens.VariableSpec{
		{
			Name:  "products",
			Label: "Products",
			Kind:  lens.VariableMultiSelect,
			Options: []lens.VariableOption{
				{Label: "OSAGO", Value: "osago"},
				{Label: "Travel", Value: "travel"},
			},
		},
	}, map[string]any{
		"products": []string{"travel"},
	})

	require.Len(t, model.Inputs, 1)
	require.Equal(t, []string{"travel"}, model.Inputs[0].Values)
	require.False(t, model.Inputs[0].Options[0].Selected)
	require.True(t, model.Inputs[0].Options[1].Selected)
}

func TestBuildNormalizesToggleAndScalarValues(t *testing.T) {
	t.Parallel()

	model := Build([]lens.VariableSpec{
		{Name: "active_only", Label: "Active", Kind: lens.VariableToggle},
		{Name: "limit", Label: "Limit", Kind: lens.VariableNumber},
	}, map[string]any{
		"active_only": "true",
		"limit":       25.5,
	})

	require.True(t, model.Inputs[0].Checked)
	require.Equal(t, "25.5", model.Inputs[1].Value)
}
