package application

import (
	"errors"
	"fmt"
	"github.com/iota-agency/iota-erp/sdk/event"
	"gorm.io/gorm"
	"reflect"
)

// ErrServiceNotFound Custom error for when a service is not found
var ErrServiceNotFound = errors.New("service not found")

// Application with a dynamically extendable service registry
type Application struct {
	DD             *gorm.DB
	EventPublisher event.Publisher
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
		services:       make(map[reflect.Type]interface{}),
	}
}
