// Package uploadsconfig provides typed configuration for file upload settings.
// It is a stdconfig package intended to be registered via config.Register[uploadsconfig.Config].
package uploadsconfig

// Config holds file upload path and size constraints.
//
// Env prefix: "uploads" (e.g. UPLOADS_PATH → uploads.path,
// MAX_UPLOAD_SIZE → uploads.maxsize, MAX_UPLOAD_MEMORY → uploads.maxmemory).
type Config struct {
	Path      string `koanf:"path"      default:"static"`
	MaxSize   int64  `koanf:"maxsize"   default:"33554432"`
	MaxMemory int64  `koanf:"maxmemory" default:"33554432"`
}

// ConfigPrefix returns the koanf prefix for uploadsconfig ("uploads").
func (Config) ConfigPrefix() string { return "uploads" }
