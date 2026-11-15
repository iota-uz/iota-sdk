package serve

import (
	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/filter"
)

func wireVariableFilters(model filter.Model) []document.Filter {
	filters := make([]document.Filter, 0)
	for _, input := range model.Inputs {
		if input.Kind == lens.VariableDateRange {
			filters = append(filters, periodFilter(input))
			continue
		}
		if input.Kind != lens.VariableCompare {
			continue
		}
		filters = append(filters, document.Filter{
			ID: input.Name, Kind: document.FilterKindCompare, Label: input.Label,
			Compare: &document.CompareFilter{
				ModeParam: input.ModeName, StartParam: input.Compare.StartName, EndParam: input.Compare.EndName,
				CompareTo: input.Compare.CompareTo,
				Value: document.CompareValue{
					Mode: document.CompareMode(input.Compare.Mode), Start: input.Compare.Start, End: input.Compare.End,
				},
			},
		})
	}
	return filters
}

// periodFilter turns a date-range variable into the document's period widget:
// the calendar, the preset catalogue and the custom range. The value is the
// range the engine resolved, so the widget shows what the panels were queried
// for; an all-time range is an unbounded selection.
func periodFilter(input filter.Input) document.Filter {
	value := document.PeriodValue{Start: input.DateRange.Start, End: input.DateRange.End}
	if input.DateRange.Mode == "all" {
		value = document.PeriodValue{}
	}
	return document.Filter{
		ID: input.Name, Kind: document.FilterKindPeriod, Label: input.Label,
		Period: &document.PeriodFilter{
			StartParam: input.DateRange.StartName, EndParam: input.DateRange.EndName,
			Value:      value,
			AllowEmpty: input.DateRange.AllowAllTime || value.Start == "" || value.End == "",
		},
	}
}
