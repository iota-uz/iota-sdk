package config

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
)

// FeatureState describes the resolved runtime state of a registered config.
// State is computed at Register time from the combination of the Configured
// interface, the Source's key coverage, and the registry's StrictMode.
type FeatureState int

const (
	// StateActive means the feature is on and will be wired up downstream.
	// Either the config does not implement Configured (always-on core), or
	// IsConfigured returned true.
	StateActive FeatureState = iota

	// StateDisabled means the feature is off and downstream wiring must skip it.
	// The config implements Configured, IsConfigured returned false, and the
	// operator did not set any key under its prefix (clean opt-out).
	StateDisabled

	// StatePartiallyConfigured means the operator set one or more fields under
	// the config's prefix but not enough for IsConfigured to return true. This
	// is almost always a typo or a half-finished deployment. In strict mode
	// Register/Seal error; in lax mode the registry treats it as Disabled and
	// logs a warning via Seal.
	StatePartiallyConfigured
)

// String returns a stable, log-friendly name for s.
func (s FeatureState) String() string {
	switch s {
	case StateActive:
		return "active"
	case StateDisabled:
		return "disabled"
	case StatePartiallyConfigured:
		return "partially_configured"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// entry is the registry's per-type bookkeeping. ptr is *T for the registered
// type T; state is the resolved FeatureState; prefix caches T.ConfigPrefix()
// (or the explicit prefix passed to RegisterAt) so Seal can re-emit it in
// error messages without reloading the source.
type entry struct {
	ptr    any
	state  FeatureState
	prefix string
}

// Registry is a typed store keyed by reflect.Type, backed by a Source.
// All operations are safe for concurrent use.
type Registry struct {
	mu      sync.RWMutex
	entries map[reflect.Type]*entry
	src     Source
	sealed  bool
	strict  StrictMode
}

// NewRegistry creates a new Registry backed by src. The registry defaults to
// StrictDefault, which resolves to StrictYes in production (app.environment =
// "production") and StrictLax elsewhere. Override with SetStrict before the
// first Register call.
func NewRegistry(src Source) *Registry {
	return &Registry{
		entries: make(map[reflect.Type]*entry),
		src:     src,
		strict:  StrictDefault,
	}
}

// SetStrict overrides the registry's strict mode. Must be called before the
// first Register/RegisterAt to be observed during state resolution.
func (r *Registry) SetStrict(mode StrictMode) {
	r.mu.Lock()
	r.strict = mode
	r.mu.Unlock()
}

// RegisterAt loads T at prefix from the registry's Source, applies tag-based
// defaults, resolves FeatureState via the Configured interface + Source
// key coverage, optionally validates, stores, and returns a pointer to the
// populated config struct. It is the escape hatch for non-Prefixed types or tests.
//
// Order of operations:
//  1. Source.Unmarshal fills fields from env / yaml / etc.
//  2. applyTagDefaults fills zero-valued fields from `default:"X"` struct tags.
//  3. resolveState computes FeatureState using Configured + Source.HasPrefix.
//  4. If state is StatePartiallyConfigured and strict mode is on, abort with error.
//  5. If state is StateActive and T implements Validatable, Validate checks values.
//
// A non-nil Validate error aborts registration and returns an error.
//
// Calling RegisterAt[T] twice with the same T silently overwrites the previous
// entry — callers should register each type once at startup.
func RegisterAt[T any](r *Registry, prefix string) (*T, error) {
	r.mu.Lock()
	sealed := r.sealed
	strict := r.strict.resolve(r)
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

	state := resolveState(r.src, prefix, &cfg)

	if state == StatePartiallyConfigured && strict == StrictYes {
		return nil, partialConfigError(prefix, &cfg)
	}

	if state == StateActive {
		if v, ok := any(&cfg).(Validatable); ok {
			if err := v.Validate(); err != nil {
				return nil, fmt.Errorf("config: validate %T: %w", cfg, err)
			}
		}
	}

	ptr := &cfg
	t := reflect.TypeOf(cfg)

	r.mu.Lock()
	r.entries[t] = &entry{ptr: ptr, state: state, prefix: prefix}
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
	e, ok := r.entries[t]
	r.mu.RUnlock()

	if !ok {
		return nil, false
	}
	return e.ptr.(*T), true
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

// State returns the resolved FeatureState for t, or StateActive with ok=false
// when t is not registered. Gate helpers use this to decide whether to wire
// up or skip a feature without re-reading the Source.
func (r *Registry) State(t reflect.Type) (FeatureState, bool) {
	r.mu.RLock()
	e, ok := r.entries[t]
	r.mu.RUnlock()
	if !ok {
		return StateActive, false
	}
	return e.state, true
}

// StateOf is the generic sugar over Registry.State for T.
func StateOf[T any](r *Registry) (FeatureState, bool) {
	var zero T
	return r.State(reflect.TypeOf(zero))
}

// resolveState computes the FeatureState for a just-unmarshaled config value.
// cfg must be a non-nil pointer. The returned state depends on whether T
// implements Configured and whether the Source has any keys under prefix.
func resolveState(src Source, prefix string, cfg any) FeatureState {
	c, ok := cfg.(Configured)
	if !ok {
		return StateActive
	}
	if c.IsConfigured() {
		return StateActive
	}
	// IsConfigured returned false. Distinguish clean-disabled from partial.
	if prefix != "" && src != nil && src.HasPrefix(prefix) {
		return StatePartiallyConfigured
	}
	return StateDisabled
}

// partialConfigError builds a strict-mode error for a PartiallyConfigured
// entry, including the DisabledReason when the config provides one.
func partialConfigError(prefix string, cfg any) error {
	reason := "required fields not set"
	if d, ok := cfg.(DisabledReason); ok {
		if r := d.DisabledReason(); r != "" {
			reason = r
		}
	}
	return fmt.Errorf("config: %q partially configured: %s", prefix, reason)
}

// Seal validates all registered entries that implement Validatable, then
// locks the registry against further registrations. Only Active entries are
// revalidated; Disabled entries are skipped. In strict mode any
// PartiallyConfigured entries still present (already-errored entries never
// get stored, but Register can fail lazily if SetStrict(StrictLax) was used
// at the time) are joined into the returned error. In lax mode Seal logs a
// warning via the supplied slog default, downgrades them to Disabled, and
// returns nil for those entries.
// Returns a joined error of all validation failures; the registry is sealed
// regardless of whether validation passed.
func (r *Registry) Seal() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sealed = true
	strict := r.strict.resolve(r)

	var errs []error
	for _, e := range r.entries {
		switch e.state {
		case StateDisabled:
			continue
		case StatePartiallyConfigured:
			if strict == StrictYes {
				errs = append(errs, partialConfigError(e.prefix, e.ptr))
				continue
			}
			// Lax mode: downgrade to Disabled and move on. Callers observing
			// State(t) will see StateDisabled from this point; they are spared
			// wiring up something that would malfunction at runtime.
			e.state = StateDisabled
			continue
		case StateActive:
			if val, ok := e.ptr.(Validatable); ok {
				if err := val.Validate(); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}

	return errors.Join(errs...)
}
