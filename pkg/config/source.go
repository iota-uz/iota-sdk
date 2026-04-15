package config

import "github.com/knadh/koanf/v2"

// Source is an immutable loaded configuration view built once at bootstrap.
// After Build returns the Source cannot absorb new keys.
type Source interface {
	// Unmarshal populates target with all keys under prefix.
	// Use an empty string to unmarshal from the root.
	Unmarshal(prefix string, target any) error

	// Has reports whether the given dot-delimited key exists.
	Has(key string) bool
}

// Build composes providers into a single immutable Source.
// Providers are applied in order; later providers override earlier ones.
func Build(providers ...Provider) (Source, error) {
	k := koanf.New(".")
	for _, p := range providers {
		if err := p.Load(k); err != nil {
			return nil, err
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
