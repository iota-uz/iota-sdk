package config

import (
	koanfmaps "github.com/knadh/koanf/maps"
	"github.com/knadh/koanf/v2"
)

// Source is an immutable loaded configuration view built once at bootstrap.
// After Build returns the Source cannot absorb new keys.
// There is no Reload, Watch, OnChange, or Subscribe API — and none will be added.
// To pick up new configuration values, restart the process.
type Source interface {
	// Unmarshal populates target with all keys under prefix.
	// Use an empty string to unmarshal from the root.
	Unmarshal(prefix string, target any) error

	// Has reports whether the given dot-delimited key exists.
	Has(key string) bool
}

// Build composes providers into a single immutable Source.
// Providers are applied in order; later providers override earlier ones.
// Each provider's Load() returns a map[string]any which is merged into
// the internal koanf instance via flattened dot-delimited keys.
func Build(providers ...Provider) (Source, error) {
	k := koanf.New(".")
	for _, p := range providers {
		m, err := p.Load()
		if err != nil {
			return nil, err
		}
		if len(m) == 0 {
			continue
		}
		// Flatten nested maps to dot-delimited keys so koanf.Set handles them
		// correctly regardless of nesting depth.
		flat, _ := koanfmaps.Flatten(m, nil, ".")
		for key, val := range flat {
			if err := k.Set(key, val); err != nil {
				return nil, err
			}
		}
	}
	return &frozenSource{k: k}, nil
}

// frozenSource wraps a *koanf.Koanf and exposes no mutation surface.
type frozenSource struct {
	k *koanf.Koanf
}

func (s *frozenSource) Unmarshal(prefix string, target any) error {
	return s.k.Unmarshal(prefix, target)
}

func (s *frozenSource) Has(key string) bool {
	return s.k.Exists(key)
}
