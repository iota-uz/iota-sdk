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

func facetFilter() Filter {
	return Filter{
		ID:    "facet-region",
		Kind:  FilterKindFacet,
		Label: "Region",
		Facet: &FacetFilter{
			Dimension:       "region",
			OptionsEndpoint: "/reports/sales/facets?_facet=region",
			SearchParam:     "_facet_search",
			Selections: []FacetSelection{{
				Label:     "Tashkent",
				RemoveURL: "/reports/sales?_f=product%3Aosago",
			}},
			ClearURL: "/reports/sales?period=2026",
		},
	}
}

func segmentedFilter() Filter {
	return Filter{
		ID:    "grain",
		Kind:  FilterKindSegmented,
		Label: "Periodicity",
		Segmented: &SegmentedFilter{
			Param: "PeriodGrain",
			Value: "quarter",
			Options: []SegmentedOption{
				{Value: "year", Label: "By year"},
				{Value: "quarter", Label: "By quarter"},
			},
		},
	}
}

func TestDashboardDocumentValidate_SegmentedFilter(t *testing.T) {
	t.Run("valid segmented filter passes beside a period", func(t *testing.T) {
		doc := testDocument()
		doc.Filters = []Filter{periodFilter(), segmentedFilter()}
		require.NoError(t, doc.Validate())
	})

	for _, test := range []struct {
		name    string
		mutate  func(*Filter)
		message string
	}{
		{name: "payload required", mutate: func(filter *Filter) { filter.Segmented = nil }, message: "requires a segmented payload"},
		{name: "param required", mutate: func(filter *Filter) { filter.Segmented.Param = " " }, message: "requires a param"},
		{
			name:    "single option rejected",
			mutate:  func(filter *Filter) { filter.Segmented.Options = filter.Segmented.Options[1:] },
			message: "at least two options",
		},
		{
			name:    "option value required",
			mutate:  func(filter *Filter) { filter.Segmented.Options[0].Value = " " },
			message: "option requires a value",
		},
		{
			name:    "option label required",
			mutate:  func(filter *Filter) { filter.Segmented.Options[0].Label = " " },
			message: "requires a label",
		},
		{
			name:    "duplicate option rejected",
			mutate:  func(filter *Filter) { filter.Segmented.Options[0].Value = "quarter" },
			message: "duplicate option",
		},
		{
			name:    "value must be one of the options",
			mutate:  func(filter *Filter) { filter.Segmented.Value = "month" },
			message: "is not one of its options",
		},
		{
			name:    "mixed payload rejected",
			mutate:  func(filter *Filter) { filter.Period = periodFilter().Period },
			message: "cannot mix segmented",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			doc := testDocument()
			filter := segmentedFilter()
			test.mutate(&filter)
			doc.Filters = []Filter{filter}
			require.ErrorContains(t, doc.Validate(), test.message)
		})
	}

	t.Run("survives clone and JSON round trip", func(t *testing.T) {
		doc := testDocument()
		segmented := segmentedFilter()
		segmented.Placement = &FilterPlacement{GroupID: "result", Tab: "Underwriting"}
		doc.Filters = []Filter{periodFilter(), segmented}
		require.NoError(t, doc.Validate())

		cloned := cloneFilters(doc.Filters)
		require.Equal(t, doc.Filters, cloned)
		cloned[1].Segmented.Options[0].Label = "mutated"
		cloned[1].Placement.Tab = "mutated"
		require.Equal(t, "By year", doc.Filters[1].Segmented.Options[0].Label)
		require.Equal(t, "Underwriting", doc.Filters[1].Placement.Tab)

		encoded, err := doc.MarshalJSON()
		require.NoError(t, err)
		decoded := &DashboardDocument{}
		require.NoError(t, json.Unmarshal(encoded, decoded))
		require.Equal(t, doc.Filters, decoded.Filters)
	})

	for _, test := range []struct {
		name      string
		placement FilterPlacement
		message   string
	}{
		{name: "group required", placement: FilterPlacement{Tab: "Underwriting"}, message: "requires a group id"},
		{name: "tab required", placement: FilterPlacement{GroupID: "result"}, message: "requires a tab"},
	} {
		t.Run("placement "+test.name, func(t *testing.T) {
			doc := testDocument()
			filter := segmentedFilter()
			filter.Placement = &test.placement
			doc.Filters = []Filter{filter}
			require.ErrorContains(t, doc.Validate(), test.message)
		})
	}
}

func TestDashboardDocumentValidate_Filters(t *testing.T) {
	t.Run("valid period filter passes", func(t *testing.T) {
		doc := testDocument()
		doc.Filters = []Filter{periodFilter()}
		require.NoError(t, doc.Validate())
	})

	t.Run("valid facet filter passes", func(t *testing.T) {
		doc := testDocument()
		doc.Filters = []Filter{periodFilter(), facetFilter()}
		require.NoError(t, doc.Validate())
	})

	for _, test := range []struct {
		name    string
		mutate  func(*Filter)
		message string
	}{
		{name: "payload required", mutate: func(filter *Filter) { filter.Facet = nil }, message: "requires a facet payload"},
		{name: "absolute options endpoint rejected", mutate: func(filter *Filter) { filter.Facet.OptionsEndpoint = "https://example.com/options" }, message: "same-origin options endpoint"},
		{name: "network-path options endpoint rejected", mutate: func(filter *Filter) { filter.Facet.OptionsEndpoint = "//example.com/options" }, message: "same-origin options endpoint"},
		{name: "backslash network-path options endpoint rejected", mutate: func(filter *Filter) { filter.Facet.OptionsEndpoint = `/\\example.com/options` }, message: "same-origin options endpoint"},
		{name: "network-path clear URL rejected", mutate: func(filter *Filter) { filter.Facet.ClearURL = "//example.com/clear" }, message: "invalid clear URL"},
		{name: "network-path remove URL rejected", mutate: func(filter *Filter) { filter.Facet.Selections[0].RemoveURL = "//example.com/remove" }, message: "invalid remove URL"},
		{name: "selection label required", mutate: func(filter *Filter) { filter.Facet.Selections[0].Label = " " }, message: "requires a label"},
	} {
		t.Run("facet "+test.name, func(t *testing.T) {
			doc := testDocument()
			filter := facetFilter()
			test.mutate(&filter)
			doc.Filters = []Filter{filter}
			require.ErrorContains(t, doc.Validate(), test.message)
		})
	}

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
	source := []Filter{periodFilter(), facetFilter()}
	cloned := cloneFilters(source)
	require.Equal(t, source, cloned)

	cloned[0].Period.Presets[0].Label = "mutated"
	cloned[0].Period.Value.Start = "1999-01-01"
	require.Equal(t, "2026", source[0].Period.Presets[0].Label)
	require.Equal(t, "2026-01-01", source[0].Period.Value.Start)
	cloned[1].Facet.Selections[0].Label = "mutated"
	require.Equal(t, "Tashkent", source[1].Facet.Selections[0].Label)
}

func TestFilterJSONRoundTrip(t *testing.T) {
	doc := testDocument()
	doc.Filters = []Filter{periodFilter(), facetFilter()}
	require.NoError(t, doc.Validate())

	encoded, err := doc.MarshalJSON()
	require.NoError(t, err)

	decoded := &DashboardDocument{}
	require.NoError(t, json.Unmarshal(encoded, decoded))
	require.Equal(t, doc.Filters, decoded.Filters)
}
