package periodics

import (
	"reflect"

	"github.com/iota-uz/iota-sdk/pkg/application"
)

// Deprecated: GetManager retrieves a single periodic tasks manager from the application container.
// Use GetManagerRegistry instead, which supports multiple named managers.
func GetManager(app application.Application) Manager {
	managerType := reflect.TypeOf((*Manager)(nil)).Elem()
	services := app.Services()

	for _, service := range services {
		if reflect.TypeOf(service).Implements(managerType) {
			return service.(Manager)
		}
	}
	return nil
}
