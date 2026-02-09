package context

import "github.com/iota-uz/iota-sdk/pkg/bichat/types"

// FormatterRegistry maps codec IDs to Formatters.
// The executor uses this to convert StructuredTool results to strings.
type FormatterRegistry struct {
	formatters map[string]types.Formatter
}

// NewFormatterRegistry creates an empty FormatterRegistry.
func NewFormatterRegistry() *FormatterRegistry {
	return &FormatterRegistry{
		formatters: make(map[string]types.Formatter),
	}
}

// Register adds a formatter for the given codec ID.
// If a formatter is already registered for that ID, it is replaced.
func (r *FormatterRegistry) Register(codecID string, f types.Formatter) {
	r.formatters[codecID] = f
}

// Get returns the formatter for the given codec ID, or nil if none is registered.
func (r *FormatterRegistry) Get(codecID string) types.Formatter {
	return r.formatters[codecID]
}

// Has returns true if a formatter is registered for the given codec ID.
func (r *FormatterRegistry) Has(codecID string) bool {
	_, ok := r.formatters[codecID]
	return ok
}
