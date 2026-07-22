package document

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func periodFilter() Filter {
	return Filter{
		ID:    "period",
		Kind:  FilterKindPeriod,
		Label: "Period",
		Period: &PeriodFilter{
			StartParam: "ActualRangeStart",
			EndParam:   "ActualRangeEnd",
			Value:      PeriodValue{Start: "2026-01-01", End: "2026-07-22"},
			AllowEmpty: true,
			Presets: []PeriodPreset{
				{ID: "year-2026", Label: "2026", Value: PeriodValue{Start: "2026-01-01", End: "2026-12-31"}},
				{ID: "all", Label: "All time", Value: PeriodValue{}},
			},
		},
	}
}

func TestDashboardDocumentValidate_Filters(t *testing.T) {
	t.Run("valid period filter passes", func(t *testing.T) {
		doc := testDocument()
		doc.Filters = []Filter{periodFilter()}
		require.NoError(t, doc.Validate())
	})

	t.Run("id required and unique", func(t *testing.T) {
		doc := testDocument()
		filter := periodFilter()
		filter.ID = " "
		doc.Filters = []Filter{filter}
		require.ErrorContains(t, doc.Validate(), "filter id is required")

		doc.Filters = []Filter{periodFilter(), periodFilter()}
		require.ErrorContains(t, doc.Validate(), "duplicate filter")
	})

	t.Run("unknown kind rejected", func(t *testing.T) {
		doc := testDocument()
		filter := periodFilter()
		filter.Kind = "enum"
		doc.Filters = []Filter{filter}
		require.ErrorContains(t, doc.Validate(), "unsupported kind")
	})

	t.Run("period payload required", func(t *testing.T) {
		doc := testDocument()
		filter := periodFilter()
		filter.Period = nil
		doc.Filters = []Filter{filter}
		require.ErrorContains(t, doc.Validate(), "requires a period payload")
	})

	t.Run("parameter names required and distinct", func(t *testing.T) {
		doc := testDocument()
		filter := periodFilter()
		filter.Period.EndParam = ""
		doc.Filters = []Filter{filter}
		require.ErrorContains(t, doc.Validate(), "start and end parameter names")

		filter = periodFilter()
		filter.Period.EndParam = filter.Period.StartParam
		doc.Filters = []Filter{filter}
		require.ErrorContains(t, doc.Validate(), "must differ")
	})

	t.Run("dates must be wire layout", func(t *testing.T) {
		doc := testDocument()
		filter := periodFilter()
		filter.Period.Value.Start = "01.02.2026"
		doc.Filters = []Filter{filter}
		require.ErrorContains(t, doc.Validate(), "2006-01-02")

		filter = periodFilter()
		filter.Period.Value.Start = "2026-2-1"
		doc.Filters = []Filter{filter}
		require.ErrorContains(t, doc.Validate(), "2006-01-02")
	})

	t.Run("inverted ranges rejected", func(t *testing.T) {
		doc := testDocument()
		filter := periodFilter()
		filter.Period.Value = PeriodValue{Start: "2026-07-22", End: "2026-01-01"}
		doc.Filters = []Filter{filter}
		require.ErrorContains(t, doc.Validate(), "end precedes start")

		filter = periodFilter()
		filter.Period.Min = "2026-01-01"
		filter.Period.Max = "2025-01-01"
		doc.Filters = []Filter{filter}
		require.ErrorContains(t, doc.Validate(), "max precedes min")
	})

	t.Run("open boundaries need allowEmpty", func(t *testing.T) {
		doc := testDocument()
		filter := periodFilter()
		filter.Period.AllowEmpty = false
		filter.Period.Presets = nil
		filter.Period.Value = PeriodValue{Start: "", End: "2026-07-22"}
		doc.Filters = []Filter{filter}
		require.ErrorContains(t, doc.Validate(), "does not allow empty")
	})

	t.Run("preset invariants", func(t *testing.T) {
		doc := testDocument()
		filter := periodFilter()
		filter.Period.Presets[1].ID = filter.Period.Presets[0].ID
		doc.Filters = []Filter{filter}
		require.ErrorContains(t, doc.Validate(), "duplicate preset")

		filter = periodFilter()
		filter.Period.Presets[0].Label = " "
		doc.Filters = []Filter{filter}
		require.ErrorContains(t, doc.Validate(), "requires a label")
	})
}

func TestCloneFilters_Isolation(t *testing.T) {
	source := []Filter{periodFilter()}
	cloned := cloneFilters(source)
	require.Equal(t, source, cloned)

	cloned[0].Period.Presets[0].Label = "mutated"
	cloned[0].Period.Value.Start = "1999-01-01"
	require.Equal(t, "2026", source[0].Period.Presets[0].Label)
	require.Equal(t, "2026-01-01", source[0].Period.Value.Start)
}

func TestFilterJSONRoundTrip(t *testing.T) {
	doc := testDocument()
	doc.Filters = []Filter{periodFilter()}
	require.NoError(t, doc.Validate())

	encoded, err := doc.MarshalJSON()
	require.NoError(t, err)

	decoded := &DashboardDocument{}
	require.NoError(t, json.Unmarshal(encoded, decoded))
	require.Equal(t, doc.Filters, decoded.Filters)
}
