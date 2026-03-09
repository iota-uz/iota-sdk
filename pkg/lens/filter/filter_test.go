package filter

import (
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	lensbuild "github.com/iota-uz/iota-sdk/pkg/lens/build"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuild_Scenarios(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.March, 9, 23, 59, 59, 0, time.UTC)

	tests := []struct {
		name   string
		specs  []lens.VariableSpec
		values map[string]any
		assert func(t *testing.T, model Model)
	}{
		{
			name: "normalizes date range and keeps all time",
			specs: []lens.VariableSpec{
				lensbuild.DateRangeVariable("range", "Range", 24*time.Hour),
			},
			values: map[string]any{
				"range": lens.DateRangeValue{
					Mode:  "all",
					Start: &start,
					End:   &end,
				},
			},
			assert: func(t *testing.T, model Model) {
				t.Helper()
				require.Len(t, model.Inputs, 1)
				assert.Equal(t, "all", model.Inputs[0].DateRange.Mode)
				assert.True(t, model.Inputs[0].DateRange.AllowAllTime)
				assert.Equal(t, "2026-03-01", model.Inputs[0].DateRange.Start)
				assert.Equal(t, "2026-03-09", model.Inputs[0].DateRange.End)
			},
		},
		{
			name: "clamps all time when variable disallows it",
			specs: []lens.VariableSpec{
				{
					Name:         "range",
					Label:        "Range",
					Kind:         lens.VariableDateRange,
					AllowAllTime: false,
				},
			},
			values: map[string]any{
				"range": lens.DateRangeValue{Mode: " all "},
			},
			assert: func(t *testing.T, model Model) {
				t.Helper()
				require.Len(t, model.Inputs, 1)
				assert.Equal(t, "default", model.Inputs[0].DateRange.Mode)
			},
		},
		{
			name: "uses configured defaults when runtime values are missing",
			specs: []lens.VariableSpec{
				{Name: "active_only", Label: "Active", Kind: lens.VariableToggle, Default: true},
				{Name: "limit", Label: "Limit", Kind: lens.VariableNumber, Default: 25.5},
			},
			values: nil,
			assert: func(t *testing.T, model Model) {
				t.Helper()
				require.Len(t, model.Inputs, 2)
				assert.True(t, model.Inputs[0].Checked)
				assert.Equal(t, "25.5", model.Inputs[1].Value)
			},
		},
		{
			name: "normalizes option selection",
			specs: []lens.VariableSpec{
				{
					Name:  "products",
					Label: "Products",
					Kind:  lens.VariableMultiSelect,
					Options: []lens.VariableOption{
						{Label: "OSAGO", Value: "osago"},
						{Label: "Travel", Value: "travel"},
					},
				},
			},
			values: map[string]any{
				"products": []string{"travel"},
			},
			assert: func(t *testing.T, model Model) {
				t.Helper()
				require.Len(t, model.Inputs, 1)
				assert.Equal(t, []string{"travel"}, model.Inputs[0].Values)
				assert.False(t, model.Inputs[0].Options[0].Selected)
				assert.True(t, model.Inputs[0].Options[1].Selected)
			},
		},
		{
			name: "normalizes toggle and scalar values",
			specs: []lens.VariableSpec{
				{Name: "active_only", Label: "Active", Kind: lens.VariableToggle},
				{Name: "limit", Label: "Limit", Kind: lens.VariableNumber},
			},
			values: map[string]any{
				"active_only": "true",
				"limit":       25.5,
			},
			assert: func(t *testing.T, model Model) {
				t.Helper()
				require.Len(t, model.Inputs, 2)
				assert.True(t, model.Inputs[0].Checked)
				assert.Equal(t, "25.5", model.Inputs[1].Value)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := Build(tt.specs, tt.values)
			tt.assert(t, model)
		})
	}
}
