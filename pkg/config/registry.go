package config

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
)

// Registry is a typed store keyed by reflect.Type, backed by a Source.
// All operations are safe for concurrent use.
type Registry struct {
	mu      sync.RWMutex
	entries map[reflect.Type]any
	src     Source
	sealed  bool
}

// NewRegistry creates a new Registry backed by src.
func NewRegistry(src Source) *Registry {
	return &Registry{
		entries: make(map[reflect.Type]any),
		src:     src,
	}
}

// RegisterAt loads T at prefix from the registry's Source, applies tag-based
// defaults, optionally validates, stores, and returns a pointer to the
// populated config struct. It is the escape hatch for non-Prefixed types or tests.
//
// Order of operations:
//  1. Source.Unmarshal fills fields from env / yaml / etc.
//  2. applyTagDefaults fills zero-valued fields from `default:"X"` struct tags.
//  3. If T implements Validatable, Validate checks the final values.
//
// A non-nil Validate error aborts registration and returns an error.
//
// Calling RegisterAt[T] twice with the same T silently overwrites the previous
// entry — callers should register each type once at startup.
func RegisterAt[T any](r *Registry, prefix string) (*T, error) {
	r.mu.Lock()
	sealed := r.sealed
	r.mu.Unlock()
	if sealed {
		return nil, fmt.Errorf("config: registry sealed")
	}

	var cfg T
	if err := r.src.Unmarshal(prefix, &cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshal %T at %q: %w", cfg, prefix, err)
	}

	if err := applyTagDefaults(&cfg); err != nil {
		return nil, fmt.Errorf("config: apply defaults %T: %w", cfg, err)
	}

	if v, ok := any(&cfg).(Validatable); ok {
		if err := v.Validate(); err != nil {
			return nil, fmt.Errorf("config: validate %T: %w", cfg, err)
		}
	}

	ptr := &cfg
	t := reflect.TypeOf(cfg)

	r.mu.Lock()
	r.entries[t] = ptr
	r.mu.Unlock()

	return ptr, nil
}

// Register loads T using the prefix declared by T.ConfigPrefix().
// T must implement Prefixed (i.e. have a ConfigPrefix() string method).
// This is the preferred API for all stdconfig types.
func Register[T Prefixed](r *Registry) (*T, error) {
	var zero T
	prefix := any(zero).(Prefixed).ConfigPrefix()
	return RegisterAt[T](r, prefix)
}

// Lookup retrieves the previously registered *T, returning false when not found.
// This is the low-level accessor; prefer Get for Prefixed types.
func Lookup[T any](r *Registry) (*T, bool) {
	var zero T
	t := reflect.TypeOf(zero)

	r.mu.RLock()
	v, ok := r.entries[t]
	r.mu.RUnlock()

	if !ok {
		return nil, false
	}
	return v.(*T), true
}

// Get retrieves the previously registered *T or panics when not found.
// T must implement Prefixed.
func Get[T Prefixed](r *Registry) *T {
	ptr, ok := Lookup[T](r)
	if !ok {
		var zero T
		panic(fmt.Sprintf("config: type %T not registered in registry", zero))
	}
	return ptr
}

// Seal validates all registered entries that implement Validatable, then
// locks the registry against further registrations.
// Returns a joined error of all validation failures; the registry is sealed
// regardless of whether validation passed.
func (r *Registry) Seal() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sealed = true

	var errs []error
	for _, v := range r.entries {
		if val, ok := v.(Validatable); ok {
			if err := val.Validate(); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errors.Join(errs...)
}
