package application

import (
	"fmt"
	"github.com/iota-agency/iota-erp/internal/domain/entities/permission"
	"github.com/iota-agency/iota-erp/pkg/event"
	"gorm.io/gorm"
	"reflect"
)

// Application with a dynamically extendable service registry
type Application struct {
	DD             *gorm.DB
	EventPublisher event.Publisher
	Rbac           *permission.Rbac
	services       map[reflect.Type]interface{}
}

// RegisterService registers a new service in the application by its type
func (app *Application) RegisterService(service interface{}) {
	serviceType := reflect.TypeOf(service).Elem()
	app.services[serviceType] = service
}

// Service retrieves a service by its type
func (app *Application) Service(service interface{}) interface{} {
	serviceType := reflect.TypeOf(service)
	svc, exists := app.services[serviceType]
	if !exists {
		panic(fmt.Sprintf("service %s not found", serviceType.Name()))
	}
	return svc
}

func New(db *gorm.DB, eventPublisher event.Publisher) *Application {
	return &Application{
		DD:             db,
		EventPublisher: eventPublisher,
		Rbac:           permission.NewRbac(),
		services:       make(map[reflect.Type]interface{}),
	}
}
