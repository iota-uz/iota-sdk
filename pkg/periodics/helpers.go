package periodics

import (
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
)

// Deprecated: GetManager retrieves a single periodic tasks manager from the application container.
// Use GetManagerRegistry instead, which supports multiple named managers.
func GetManager(app application.Application) Manager {
	registry, ok, err := composition.ResolveOptionalForApp[ManagerRegistry](app)
	if err != nil || !ok || registry == nil {
		return nil
	}
	for _, manager := range registry.All() {
		return manager
	}
	return nil
}
