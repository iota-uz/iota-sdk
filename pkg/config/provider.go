package config

import "github.com/knadh/koanf/v2"

// Provider contributes keys to a Source during Build.
// Later providers passed to Build override earlier ones.
type Provider interface {
	Load(k *koanf.Koanf) error
}
