package config

// Provider contributes keys to a Source during Build.
// Later providers passed to Build override earlier ones.
//
// Load returns a flat or nested map[string]any of configuration values.
// The Build function merges provider maps in order (later providers override
// earlier ones) using dot-notation for nested keys.
//
// Name returns a human-readable identifier for the provider, used when
// recording key origins (see [Source.Origin]). The format is provider-specific:
// env providers include the files they loaded, yamlfile providers include the
// file path, and so on.
type Provider interface {
	Load() (map[string]any, error)
	Name() string
}
