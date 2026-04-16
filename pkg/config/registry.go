package config

import (
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
}

// NewRegistry creates a new Registry backed by src.
func NewRegistry(src Source) *Registry {
	return &Registry{
		entries: make(map[reflect.Type]any),
		src:     src,
	}
}

// Defaulter is optionally implemented by config types that need to apply
// fallback values for zero-valued fields. SetDefaults is called on the
// pointer receiver AFTER Unmarshal and BEFORE Validate — it must not
// overwrite fields that the source already populated.
type Defaulter interface {
	SetDefaults()
}

// Register loads T at prefix from the registry's Source, applies defaults,
// optionally validates, stores, and returns a pointer to the populated
// config struct.
//
// Order of operations:
//  1. Source.Unmarshal fills fields from env / yaml / etc.
//  2. If T implements Defaulter, SetDefaults fills zero-valued fields.
//  3. If T implements Validatable, Validate checks the final values.
//
// A non-nil Validate error aborts registration and returns an error.
//
// Calling Register[T] twice with the same T silently overwrites the previous
// entry — callers should register each type once at startup.
func Register[T any](r *Registry, prefix string) (*T, error) {
	var cfg T
	if err := r.src.Unmarshal(prefix, &cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshal %T at %q: %w", cfg, prefix, err)
	}

	if d, ok := any(&cfg).(Defaulter); ok {
		d.SetDefaults()
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

// Get retrieves the previously registered *T, returning false when not found.
func Get[T any](r *Registry) (*T, bool) {
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

// MustGet retrieves the previously registered *T or panics when not found.
func MustGet[T any](r *Registry) *T {
	ptr, ok := Get[T](r)
	if !ok {
		var zero T
		panic(fmt.Sprintf("config: type %T not registered in registry", zero))
	}
	return ptr
}
