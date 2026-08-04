package periodics

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// ManagerRegistry collects named Manager instances so that multiple modules
// can each register their own periodic-task manager without overwriting
// each other in the composition container.
type ManagerRegistry interface {
	// Register adds a manager under the given name (e.g. "insurance").
	// Returns an error if a manager with the same name is already registered.
	Register(name string, m Manager) error
	// All returns every registered manager keyed by name.
	All() map[string]Manager
	// StopAll stops all registered managers gracefully
	StopAll(ctx context.Context) error
}

type managerRegistry struct {
	mu       sync.RWMutex
	managers map[string]Manager
}

// NewManagerRegistry creates a new empty ManagerRegistry.
func NewManagerRegistry() ManagerRegistry {
	return &managerRegistry{
		managers: make(map[string]Manager),
	}
}

// Register adds a manager under the given name. Returns an error if a manager
// with the same name is already registered.
func (r *managerRegistry) Register(name string, m Manager) error {
	const op serrors.Op = "periodics.managerRegistry.Register"
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.managers[name]; exists {
		return serrors.E(op, fmt.Errorf("periodic task manager with name '%s' is already registered", name))
	}
	r.managers[name] = m
	return nil
}

// StopAll stops all registered managers gracefully, aggregating any errors.
func (r *managerRegistry) StopAll(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var errs []error
	for name, m := range r.managers {
		if err := m.Stop(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to stop manager '%s': %w", name, err))
		}
	}
	return errors.Join(errs...)
}

// All returns a copy of every registered manager keyed by name.
func (r *managerRegistry) All() map[string]Manager {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]Manager, len(r.managers))
	maps.Copy(result, r.managers)
	return result
}
