package serve

import (
	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/filter"
)

func wireVariableFilters(model filter.Model) []document.Filter {
	filters := make([]document.Filter, 0)
	for _, input := range model.Inputs {
		if input.Kind != lens.VariableCompare {
			continue
		}
		filters = append(filters, document.Filter{
			ID: input.Name, Kind: document.FilterKindCompare, Label: input.Label,
			Compare: &document.CompareFilter{
				ModeParam: input.Name, StartParam: input.Compare.StartName, EndParam: input.Compare.EndName,
				CompareTo: input.Compare.CompareTo,
				Value: document.CompareValue{
					Mode: document.CompareMode(input.Compare.Mode), Start: input.Compare.Start, End: input.Compare.End,
				},
			},
		})
	}
	return filters
}
