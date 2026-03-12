package periodics

import (
	"context"
	"fmt"
	"sync"

	"github.com/iota-uz/iota-sdk/pkg/application"
)

// ManagerRegistry collects named Manager instances so that multiple modules
// can each register their own periodic-task manager without overwriting
// each other in the application service container.
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

func (r *managerRegistry) Register(name string, m Manager) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.managers[name]; exists {
		return fmt.Errorf("periodic task manager with name '%s' is already registered", name)
	}
	r.managers[name] = m
	return nil
}

func (r *managerRegistry) StopAll(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var lastErr error
	for name, m := range r.managers {
		if err := m.Stop(ctx); err != nil {
			lastErr = fmt.Errorf("failed to stop manager '%s': %w", name, err)
		}
	}
	return lastErr
}

func (r *managerRegistry) All() map[string]Manager {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]Manager, len(r.managers))
	for k, v := range r.managers {
		result[k] = v
	}
	return result
}

// GetManagerRegistry retrieves the ManagerRegistry from the application container.
// Returns nil if no registry has been registered.
func GetManagerRegistry(app application.Application) ManagerRegistry {
	for _, svc := range app.Services() {
		if reg, ok := svc.(ManagerRegistry); ok {
			return reg
		}
	}
	return nil
}

var registryMu sync.Mutex

// GetOrCreateManagerRegistry returns the existing ManagerRegistry from the
// application container, or creates a new one, registers it, and returns it.
// It is safe to call concurrently from multiple goroutines.
func GetOrCreateManagerRegistry(app application.Application) ManagerRegistry {
	registryMu.Lock()
	defer registryMu.Unlock()
	if reg := GetManagerRegistry(app); reg != nil {
		return reg
	}
	reg := NewManagerRegistry()
	app.RegisterServices(reg)
	return reg
}
