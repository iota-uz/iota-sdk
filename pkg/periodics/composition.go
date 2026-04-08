package periodics

import "github.com/iota-uz/iota-sdk/pkg/composition"

// ProvideManagerRegistry registers a periodic manager registry for both
// composition-based components and legacy app.Service consumers.
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
