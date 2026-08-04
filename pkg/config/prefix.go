package config

// Prefixed is implemented by config types that know their own koanf prefix.
// Register[T Prefixed] uses this to derive the prefix automatically, so
// callers never repeat the string.
type Prefixed interface {
	ConfigPrefix() string
}
