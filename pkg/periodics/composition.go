package periodics

import "github.com/iota-uz/iota-sdk/pkg/composition"

// ProvideManagerRegistry registers a periodic manager registry in the
// composition container.
func ProvideManagerRegistry(
	builder *composition.Builder,
	registry ManagerRegistry,
) ManagerRegistry {
	if registry == nil {
		registry = NewManagerRegistry()
	}
	composition.Provide[ManagerRegistry](builder, registry)
	return registry
}
