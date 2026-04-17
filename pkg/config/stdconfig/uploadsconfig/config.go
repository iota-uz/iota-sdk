// Package uploadsconfig provides typed configuration for file upload settings.
// It is a stdconfig package intended to be registered via config.Register[uploadsconfig.Config].
package uploadsconfig

const (
	defaultPath      = "static"
	defaultMaxSize   = int64(33554432) // 32 MiB
	defaultMaxMemory = int64(33554432) // 32 MiB
)

// Config holds file upload path and size constraints.
//
// Env prefix: "uploads" (e.g. UPLOADS_PATH → uploads.path,
// MAX_UPLOAD_SIZE → uploads.maxsize, MAX_UPLOAD_MEMORY → uploads.maxmemory).
type Config struct {
	Path      string `koanf:"path"`
	MaxSize   int64  `koanf:"maxsize"`
	MaxMemory int64  `koanf:"maxmemory"`
}

// ConfigPrefix returns the koanf prefix for uploadsconfig ("uploads").
func (Config) ConfigPrefix() string { return "uploads" }

// SetDefaults fills zero-value fields with documented defaults.
func (c *Config) SetDefaults() {
	if c.Path == "" {
		c.Path = defaultPath
	}
	if c.MaxSize == 0 {
		c.MaxSize = defaultMaxSize
	}
	if c.MaxMemory == 0 {
		c.MaxMemory = defaultMaxMemory
	}
}
