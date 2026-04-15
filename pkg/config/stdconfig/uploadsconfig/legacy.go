package uploadsconfig

import "github.com/iota-uz/iota-sdk/pkg/configuration"

// FromLegacy produces a Config from the monolithic *configuration.Configuration.
// SetDefaults is called so that zero-value fields receive documented defaults.
func FromLegacy(c *configuration.Configuration) Config {
	cfg := Config{
		Path:      c.UploadsPath,
		MaxSize:   c.MaxUploadSize,
		MaxMemory: c.MaxUploadMemory,
	}
	cfg.SetDefaults()
	return cfg
}
