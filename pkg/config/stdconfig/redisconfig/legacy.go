package redisconfig

import "github.com/iota-uz/iota-sdk/pkg/configuration"

// FromLegacy produces a Config from the monolithic *configuration.Configuration.
// SetDefaults is called so that zero-value fields receive documented defaults.
func FromLegacy(c *configuration.Configuration) Config {
	cfg := Config{
		URL: c.RedisURL,
	}
	cfg.SetDefaults()
	return cfg
}
