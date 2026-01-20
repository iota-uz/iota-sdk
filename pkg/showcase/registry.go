// Package showcase provides extensibility for IOTA SDK's component showcase system.
// Child projects can register custom component categories that appear alongside
// built-in showcase categories.
//
// Usage example in child project's init():
//
//	import sdkshowcase "github.com/iota-uz/iota-sdk/pkg/showcase"
//	import "my-project/components"
//
//	func init() {
//	    err := sdkshowcase.GlobalRegistry().RegisterCategory(&sdkshowcase.Category{
//	        Name: "Custom Components",
//	        Path: "custom",
//	        Components: []sdkshowcase.Component{
//	            {
//	                Name:     "My Custom Button",
//	                Template: components.MyCustomButton(),
//	                Source:   `<button class="custom">Click Me</button>`, // optional
//	            },
//	        },
//	    })
//	    if err != nil {
//	        panic(fmt.Sprintf("failed to register showcase category: %v", err))
//	    }
//	}
//
// Reserved Paths:
// The following paths are reserved for built-in categories and cannot be used:
// "form", "loaders", "charts", "tooltips", "other", "kanban", "subscription"
package showcase

import (
	"regexp"
	"strconv"
	"sync"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Component represents a single component in the showcase
type Component struct {
	Name     string          // Display name (e.g., "My Custom Button")
	Template templ.Component // Templ component to render
	Source   string          // Source code (optional, for code tab)
}

// Category represents a group of related components
type Category struct {
	Name       string      // Display name (e.g., "Custom Components")
	Path       string      // URL path segment (e.g., "custom")
	Components []Component // List of components in this category
}

// Registry manages all showcase categories
type Registry struct {
	mu         sync.RWMutex
	categories map[string]*Category // key: path
	order      []string             // registration order for sidebar
	reserved   map[string]bool      // reserved paths (built-in categories)
}

var globalRegistry = &Registry{
	categories: make(map[string]*Category),
	order:      make([]string, 0),
	reserved: map[string]bool{
		"form":         true,
		"loaders":      true,
		"charts":       true,
		"tooltips":     true,
		"other":        true,
		"kanban":       true,
		"subscription": true,
	},
}

// GlobalRegistry returns the singleton registry instance
func GlobalRegistry() *Registry {
	return globalRegistry
}

// isValidURLPath validates that a path contains only valid URL characters
// (alphanumeric, hyphens, underscores)
var validPathPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func isValidURLPath(path string) bool {
	return validPathPattern.MatchString(path)
}

// RegisterCategory adds a category with components to the showcase.
// This method is typically called from child project init() functions.
//
// Validation rules:
//   - Category must not be nil
//   - Path must not be empty and must contain only alphanumeric, hyphens, underscores
//   - Name must not be empty
//   - Components slice must have at least one component
//   - Each component must have non-empty name and non-nil template
//   - Path must not conflict with reserved built-in categories
//
// Returns error if validation fails or path conflicts with built-in categories.
//
// Note: When calling from init(), wrap errors appropriately:
//
//	if err := sdkshowcase.GlobalRegistry().RegisterCategory(cat); err != nil {
//	    panic(fmt.Sprintf("failed to register showcase category: %v", err))
//	}
func (r *Registry) RegisterCategory(cat *Category) error {
	op := serrors.Op("showcase.Registry.RegisterCategory")
	r.mu.Lock()
	defer r.mu.Unlock()

	// Validate category is not nil
	if cat == nil {
		return serrors.E(op, serrors.Invalid, "category cannot be nil")
	}

	// Validate path is not empty
	if cat.Path == "" {
		return serrors.E(op, serrors.Invalid, "category path cannot be empty")
	}

	// Validate path contains only valid URL characters
	if !isValidURLPath(cat.Path) {
		return serrors.E(op, serrors.Invalid, "category path contains invalid characters (only alphanumeric, hyphens, and underscores allowed)", "path", cat.Path)
	}

	// Validate name is not empty
	if cat.Name == "" {
		return serrors.E(op, serrors.Invalid, "category name cannot be empty", "path", cat.Path)
	}

	// Validate components slice has at least one component
	if len(cat.Components) == 0 {
		return serrors.E(op, serrors.Invalid, "category must have at least one component", "path", cat.Path)
	}

	// Validate each component
	for i, comp := range cat.Components {
		if comp.Name == "" {
			return serrors.E(op, serrors.Invalid, "component has empty name", "path", cat.Path, "index", strconv.Itoa(i))
		}
		if comp.Template == nil {
			return serrors.E(op, serrors.Invalid, "component has nil template", "path", cat.Path, "name", comp.Name)
		}
	}

	// Validate path doesn't conflict with reserved paths
	if r.reserved[cat.Path] {
		return serrors.E(op, serrors.Invalid, "category path conflicts with built-in showcase category", "path", cat.Path)
	}

	// Check if already registered
	if _, exists := r.categories[cat.Path]; !exists {
		r.order = append(r.order, cat.Path)
	}

	r.categories[cat.Path] = cloneCategory(cat)
	return nil
}

// cloneCategory deep-copies a category including its components slice,
// so callers cannot mutate registry state without holding the mutex.
func cloneCategory(cat *Category) *Category {
	if cat == nil {
		return nil
	}
	cp := *cat
	cp.Components = append([]Component(nil), cat.Components...)
	return &cp
}

// GetCategory retrieves a category by path
func (r *Registry) GetCategory(path string) (*Category, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cat, ok := r.categories[path]
	if !ok {
		return nil, false
	}
	return cloneCategory(cat), true
}

// GetAllCategories returns all registered categories in registration order
func (r *Registry) GetAllCategories() []*Category {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cats := make([]*Category, 0, len(r.order))
	for _, path := range r.order {
		if cat, ok := r.categories[path]; ok {
			cats = append(cats, cloneCategory(cat))
		}
	}
	return cats
}
