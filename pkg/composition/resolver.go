package composition

import "fmt"

type Resolver[T any] struct {
	key Key
}

func Use[T any]() Resolver[T] {
	return Resolver[T]{key: KeyFor[T]()}
}

func UseNamed[T any](name string) Resolver[T] {
	return Resolver[T]{key: NamedKeyFor[T](name)}
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

type Optional[T any] struct {
	resolver Resolver[T]
}

func UseOptional[T any]() Optional[T] {
	return Optional[T]{resolver: Use[T]()}
}

func UseOptionalNamed[T any](name string) Optional[T] {
	return Optional[T]{resolver: UseNamed[T](name)}
}

func (o Optional[T]) Resolve(container *Container) (T, bool, error) {
	value, err := o.resolver.Resolve(container)
	if err == nil {
		return value, true, nil
	}
	if IsNotProvided(err) {
		var zero T
		return zero, false, nil
	}
	var zero T
	return zero, false, err
}
