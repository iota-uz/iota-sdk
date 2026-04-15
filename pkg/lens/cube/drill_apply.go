package cube

import (
	"github.com/sirupsen/logrus"
)

// DrillApplier applies a single drill filter value to application-specific params.
// The closure captures the target struct by pointer — no type assertions needed.
type DrillApplier func(value string)

// ApplyDrillFilters iterates drill context filters and calls the matching applier.
// Logs a warning for any filter whose dimension exists in the cube spec but has
// no applier, catching forgotten mappings at runtime.
// Silently skips filters whose dimension is not in the cube spec.
func ApplyDrillFilters(spec CubeSpec, ctx DrillContext, appliers map[string]DrillApplier) {
	for _, filter := range ctx.Filters {
		if _, ok := spec.Dimension(filter.Dimension); !ok {
			continue
		}
		applier, ok := appliers[filter.Dimension]
		if !ok {
			logrus.WithFields(logrus.Fields{
				"cube":      spec.ID,
				"dimension": filter.Dimension,
				"value":     filter.Value,
			}).Warn("cube: drill filter has no applier — dimension will be ignored in detail query")
			continue
		}
		applier(filter.Value)
	}
}
