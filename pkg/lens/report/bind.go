package report

import (
	"github.com/iota-uz/iota-sdk/pkg/lens"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
)

// BindRequest freezes normalized source inputs before placement namespacing.
// The composition's one runtime still executes and caches all datasets.
func BindRequest(spec lens.DashboardSpec, request lensruntime.Request) (lens.DashboardSpec, error) {
	values, err := lensruntime.ResolveVariables(spec.Variables, request)
	if err != nil {
		return lens.DashboardSpec{}, err
	}
	bound := clone(spec)
	primary := ""
	for i := range bound.Variables {
		variable := &bound.Variables[i]
		variable.Default = values[variable.Name]
		variable.Bound = true
		if variable.Kind == lens.VariableDateRange && primary == "" {
			primary = variable.Name
		}
	}
	for i := range bound.Datasets {
		if bound.Datasets[i].TimeRangeVariable == "" {
			bound.Datasets[i].TimeRangeVariable = primary
		}
	}
	return bound, nil
}
