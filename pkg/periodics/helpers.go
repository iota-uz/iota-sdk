package periodics

import (
	"reflect"

	"github.com/iota-uz/iota-sdk/pkg/application"
)

// GetManager retrieves the periodic tasks manager from the application container
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
