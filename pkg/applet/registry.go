package applet

import (
	"fmt"
	"sync"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Registry manages registered applets in the application.
// It provides thread-safe registration and retrieval of applets.
type Registry interface {
	// Register registers an applet with the registry.
	// Returns an error if an applet with the same name is already registered.
	Register(applet Applet) error

	// Get retrieves an applet by name.
	// Returns nil if the applet is not found.
	Get(name string) Applet

	// GetByBasePath retrieves an applet by its base path.
	// Returns nil if no applet is mounted at the given path.
	GetByBasePath(basePath string) Applet

	// All returns all registered applets.
	All() []Applet

	// Has checks if an applet with the given name is registered.
	Has(name string) bool
}

// appletRegistry is the default implementation of Registry.
type appletRegistry struct {
	mu              sync.RWMutex
	appletsByName   map[string]Applet
	appletsByPath   map[string]Applet
	registeredOrder []Applet // Maintains registration order
}

// NewRegistry creates a new applet registry.
func NewRegistry() Registry {
	return &appletRegistry{
		appletsByName: make(map[string]Applet),
		appletsByPath: make(map[string]Applet),
	}
}

// Register implements Registry.Register
func (r *appletRegistry) Register(applet Applet) error {
	const op serrors.Op = "appletRegistry.Register"

	r.mu.Lock()
	defer r.mu.Unlock()

	name := applet.Name()
	basePath := applet.BasePath()

	// Check for duplicate name
	if _, exists := r.appletsByName[name]; exists {
		return serrors.E(op, serrors.KindValidation,
			fmt.Sprintf("applet with name %q already registered", name))
	}

	// Check for duplicate base path
	if _, exists := r.appletsByPath[basePath]; exists {
		return serrors.E(op, serrors.KindValidation,
			fmt.Sprintf("applet with base path %q already registered", basePath))
	}

	// Register applet
	r.appletsByName[name] = applet
	r.appletsByPath[basePath] = applet
	r.registeredOrder = append(r.registeredOrder, applet)

	return nil
}

// Get implements Registry.Get
func (r *appletRegistry) Get(name string) Applet {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.appletsByName[name]
}

// GetByBasePath implements Registry.GetByBasePath
func (r *appletRegistry) GetByBasePath(basePath string) Applet {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.appletsByPath[basePath]
}

// All implements Registry.All
func (r *appletRegistry) All() []Applet {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make([]Applet, len(r.registeredOrder))
	copy(result, r.registeredOrder)

	return result
}

// Has implements Registry.Has
func (r *appletRegistry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.appletsByName[name]
	return exists
}
