package crud

import (
	"context"
	"sync"

	"github.com/a-h/templ"
)

// FieldRenderer defines the interface for custom field renderers
// Each renderer provides three rendering modes for different contexts
type FieldRenderer interface {
	// RenderTableCell renders the field value for display in table rows
	RenderTableCell(ctx context.Context, field Field, value FieldValue) templ.Component

	// RenderDetails renders the field value for the details/view page
	RenderDetails(ctx context.Context, field Field, value FieldValue) templ.Component

	// RenderFormControl renders the field as an editable form input
	RenderFormControl(ctx context.Context, field Field, value FieldValue) templ.Component
}

// RendererRegistry manages the registration and retrieval of custom field renderers
// It provides thread-safe access to renderer mappings
type RendererRegistry struct {
	mu        sync.RWMutex
	renderers map[string]FieldRenderer
}

// NewRendererRegistry creates a new empty renderer registry
func NewRendererRegistry() *RendererRegistry {
	return &RendererRegistry{
		renderers: make(map[string]FieldRenderer),
	}
}

// Register associates a renderer with a field type identifier
// typeName should match the value returned by Field.RendererType()
func (r *RendererRegistry) Register(typeName string, renderer FieldRenderer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.renderers[typeName] = renderer
}

// Get retrieves a renderer for the given type name
// Returns the renderer and true if found, nil and false otherwise
func (r *RendererRegistry) Get(typeName string) (FieldRenderer, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	renderer, exists := r.renderers[typeName]
	return renderer, exists
}

// Has checks if a renderer is registered for the given type name
func (r *RendererRegistry) Has(typeName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.renderers[typeName]
	return exists
}

// Unregister removes a renderer for the given type name
func (r *RendererRegistry) Unregister(typeName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.renderers, typeName)
}

// RegisteredTypes returns a slice of all registered type names
func (r *RendererRegistry) RegisteredTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.renderers))
	for typeName := range r.renderers {
		types = append(types, typeName)
	}
	return types
}
