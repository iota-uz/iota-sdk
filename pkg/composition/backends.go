package composition

import (
	"fmt"
	"slices"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/appletengine/handlers"
)

type BackendFactory[T any] struct {
	Validate func(BuildContext) error
	Build    func(*Container) (T, error)
}

type BackendRegistry struct {
	KV      *TypedBackendRegistry[handlers.KVStore]
	DB      *TypedBackendRegistry[handlers.DBStore]
	Jobs    *TypedBackendRegistry[handlers.JobsStore]
	Files   *TypedBackendRegistry[handlers.FilesStore]
	Secrets *TypedBackendRegistry[handlers.SecretsStore]
}

type TypedBackendRegistry[T any] struct {
	kind      string
	factories map[string]BackendFactory[T]
}

func NewBackendRegistry() *BackendRegistry {
	return &BackendRegistry{
		KV:      NewTypedBackendRegistry[handlers.KVStore]("kv"),
		DB:      NewTypedBackendRegistry[handlers.DBStore]("db"),
		Jobs:    NewTypedBackendRegistry[handlers.JobsStore]("jobs"),
		Files:   NewTypedBackendRegistry[handlers.FilesStore]("files"),
		Secrets: NewTypedBackendRegistry[handlers.SecretsStore]("secrets"),
	}
}

func NewTypedBackendRegistry[T any](kind string) *TypedBackendRegistry[T] {
	return &TypedBackendRegistry[T]{
		kind:      strings.TrimSpace(kind),
		factories: make(map[string]BackendFactory[T]),
	}
}

func (r *TypedBackendRegistry[T]) Register(name string, factory BackendFactory[T]) error {
	if r == nil {
		return fmt.Errorf("composition: backend registry is nil")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("composition: %s backend name is required", r.kind)
	}
	if _, exists := r.factories[name]; exists {
		return fmt.Errorf("composition: %s backend %q already registered", r.kind, name)
	}
	if factory.Build == nil {
		return fmt.Errorf("composition: %s backend %q requires a build function", r.kind, name)
	}
	r.factories[name] = factory
	return nil
}

func (r *TypedBackendRegistry[T]) Lookup(name string) (BackendFactory[T], bool) {
	if r == nil {
		var zero BackendFactory[T]
		return zero, false
	}
	factory, ok := r.factories[strings.TrimSpace(name)]
	return factory, ok
}

func (r *TypedBackendRegistry[T]) Names() []string {
	if r == nil {
		return nil
	}
	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func (r *TypedBackendRegistry[T]) Validate(name string, ctx BuildContext) error {
	factory, ok := r.Lookup(name)
	if !ok {
		return fmt.Errorf("composition: %s backend %q not registered", r.kind, strings.TrimSpace(name))
	}
	if factory.Validate == nil {
		return nil
	}
	return factory.Validate(ctx)
}

func (r *TypedBackendRegistry[T]) Build(name string, container *Container) (T, error) {
	factory, ok := r.Lookup(name)
	if !ok {
		var zero T
		return zero, fmt.Errorf("composition: %s backend %q not registered", r.kind, strings.TrimSpace(name))
	}
	return factory.Build(container)
}
