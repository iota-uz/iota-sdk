package periodics

import (
	"reflect"

	"github.com/iota-uz/iota-sdk/pkg/application"
)

// GetManager retrieves the periodic tasks manager from the application container.
// If multiple Manager implementations are registered, the first one found is returned.
func GetManager(app application.Application) Manager {
	managerType := reflect.TypeOf((*Manager)(nil)).Elem()
	services := app.Services()

	var found Manager
	for _, service := range services {
		if reflect.TypeOf(service).Implements(managerType) {
			if found != nil {
				return found // Return first found, ignore duplicates
			}
			found = service.(Manager)
		}
	}
	return found
}
