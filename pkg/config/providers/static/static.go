// Package static provides a config.Provider backed by an in-memory map.
// Intended for tests. Accepts dot-keyed flat maps as well as nested map[string]any values.
package static

import (
	"github.com/knadh/koanf/v2"

	"github.com/iota-uz/iota-sdk/pkg/config"
)

// New returns a Provider loaded from values.
// Keys may be flat dot-delimited strings ("db.host") or nested map[string]any.
// An empty or nil map is a no-op.
func New(values map[string]any) config.Provider {
	return &staticProvider{values: values}
}

type staticProvider struct {
	values map[string]any
}

// Load merges values into k. Each top-level key is set individually so that
// dot-delimited flat keys (e.g. "db.host") are expanded into nested maps,
// consistent with how env and yaml providers behave.
func (p *staticProvider) Load(k *koanf.Koanf) error {
	if len(p.values) == 0 {
		return nil
	}
	for key, val := range p.values {
		if err := k.Set(key, val); err != nil {
			return err
		}
	}
	return nil
}
