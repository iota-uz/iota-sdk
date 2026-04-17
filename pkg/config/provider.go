package config

// Provider contributes keys to a Source during Build.
// Later providers passed to Build override earlier ones.
//
// Load returns a flat or nested map[string]any of configuration values.
// The Build function merges provider maps in order (later providers override
// earlier ones) using dot-notation for nested keys.
type Provider interface {
	Load() (map[string]any, error)
}
