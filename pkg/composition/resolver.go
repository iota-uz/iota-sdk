package composition

import (
	"fmt"
	"reflect"
)

type Resolver[T any] struct {
	key Key
}

func Use[T any]() Resolver[T] {
	return Resolver[T]{key: KeyFor[T]()}
}

func KeyForType(t reflect.Type) Key {
	return keyFor(t, "")
}

func (c *Container) ResolveType(t reflect.Type) (any, error) {
	if c == nil {
		return nil, fmt.Errorf("composition: container is nil")
	}
	return c.resolveAny(keyFor(t, ""))
}

func (r Resolver[T]) Key() Key {
	return r.key
}

func (r Resolver[T]) Resolve(container *Container) (T, error) {
	if container == nil {
		var zero T
		return zero, fmt.Errorf("composition: container is nil")
	}
	return ResolveKey[T](container, r.key)
}

func (r Resolver[T]) MustResolve(container *Container) T {
	value, err := r.Resolve(container)
	if err != nil {
		panic(err)
	}
	return value
}
