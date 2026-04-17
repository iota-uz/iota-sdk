// Package static provides a config.Provider backed by an in-memory map.
// Intended for tests. Accepts dot-keyed flat maps as well as nested map[string]any values.
package static

import (
	"github.com/iota-uz/iota-sdk/pkg/config"
)

// Ensure *staticProvider implements config.Provider at compile time.
var _ config.Provider = (*staticProvider)(nil)

// New returns a Provider loaded from values.
// Keys may be flat dot-delimited strings ("db.host") or nested map[string]any.
// An empty or nil map is a no-op.
func New(values map[string]any) config.Provider {
	return &staticProvider{values: values}
}

type staticProvider struct {
	values map[string]any
}

// Name returns "static".
func (p *staticProvider) Name() string {
	return "static"
}

// Load returns a copy of the stored values map.
func (p *staticProvider) Load() (map[string]any, error) {
	if len(p.values) == 0 {
		return nil, nil
	}
	// Return a shallow copy so mutations after New don't affect the source.
	out := make(map[string]any, len(p.values))
	for k, v := range p.values {
		out[k] = v
	}
	return out, nil
}
