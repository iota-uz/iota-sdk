package config

import (
	"maps"
	"slices"

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

	// Get returns the value stored at the given dot-delimited key.
	// ok is false if the key does not exist.
	Get(key string) (value any, ok bool)

	// Keys returns all keys in the source, sorted and deduplicated.
	Keys() []string

	// Origin returns the name of the provider that last set key.
	// ok is false if the key does not exist.
	// Reports which Provider supplied key. Unrelated to httpconfig.Config.Origin
	// (URL builder) and httpconfig.Config.OriginOverride (env pin).
	Origin(key string) (provider string, ok bool)
}

// Build composes providers into a single immutable Source.
// Providers are applied in order; later providers override earlier ones.
// Each provider's Load() returns a map[string]any which is merged into
// the internal koanf instance via flattened dot-delimited keys.
func Build(providers ...Provider) (Source, error) {
	k := koanf.New(".")
	// flat duplicates koanf's internal state for O(1) key lookups and origin
	// tracking. Config maps are typically tens of keys so the O(n) extra memory
	// cost is negligible.
	flat := make(map[string]any)
	origins := make(map[string]string)

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
		flatMap, _ := koanfmaps.Flatten(m, nil, ".")
		for key, val := range flatMap {
			if err := k.Set(key, val); err != nil {
				return nil, err
			}
			flat[key] = val
			origins[key] = p.Name()
		}
	}
	return &frozenSource{k: k, flat: flat, origins: origins}, nil
}

// frozenSource is the immutable Source implementation returned by Build.
// After Build returns, all fields are read-only; concurrent Unmarshal/Get/Keys/Origin
// calls are safe without additional synchronisation. Mutation would require a new Build.
type frozenSource struct {
	k       *koanf.Koanf
	flat    map[string]any
	origins map[string]string
}

func (s *frozenSource) Unmarshal(prefix string, target any) error {
	return s.k.Unmarshal(prefix, target)
}

func (s *frozenSource) Get(key string) (any, bool) {
	v, ok := s.flat[key]
	return v, ok
}

func (s *frozenSource) Keys() []string {
	ks := slices.Collect(maps.Keys(s.flat))
	slices.Sort(ks)
	return ks
}

func (s *frozenSource) Origin(key string) (string, bool) {
	p, ok := s.origins[key]
	return p, ok
}
