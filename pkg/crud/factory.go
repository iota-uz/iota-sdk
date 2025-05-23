package crud

import (
	"reflect"
)

// DefaultEntityFactory is the default implementation of EntityFactory
type DefaultEntityFactory[T any] struct{}

// Create instantiates a new entity of type T
func (f DefaultEntityFactory[T]) Create() T {
	// Create a new entity using reflection
	entityType := reflect.TypeOf(*new(T))
	if entityType.Kind() == reflect.Ptr {
		entityType = entityType.Elem()
	}
	return reflect.New(entityType).Interface().(T)
}
