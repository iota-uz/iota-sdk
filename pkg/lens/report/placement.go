package report

import (
	"github.com/iota-uz/iota-sdk/pkg/lens/filtercontext"
	"reflect"
)

// PlaceAnalysis preserves the opened selection locally, including relative periods.
// PlaceLibraryReport starts with inheritance and never copies an execution snapshot.
func PlaceAnalysis(id string, definition Definition, effective filtercontext.EffectiveContext) (Placement, error) {
	if err := definition.Validate(); err != nil {
		return Placement{}, err
	}
	if !identifier.MatchString(id) || !effective.Ready() {
		return Placement{}, invalid("opened analysis is not placeable")
	}
	resolved, err := filtercontext.Resolve(effective.Input)
	if err != nil {
		return Placement{}, err
	}
	if !reflect.DeepEqual(resolved, effective) {
		return Placement{}, invalid("opened analysis has inconsistent context")
	}
	source := effective.Input.Source
	if source == nil || source.ID != definition.Source.ID || source.Version != definition.Source.Version || source.View != definition.View {
		return Placement{}, invalid("opened analysis belongs to another source")
	}
	frozen := clone(effective)
	return Placement{ID: id, ReportID: definition.ID, Filters: filtercontext.ContextFilters{Period: &frozen.Period.Selection, Constraints: frozen.Intersection}}, nil
}

func PlaceLibraryReport(id, reportID string) Placement { return Placement{ID: id, ReportID: reportID} }
