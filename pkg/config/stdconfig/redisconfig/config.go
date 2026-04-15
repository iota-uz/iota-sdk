// Package redisconfig provides typed configuration for the top-level Redis connection.
// It is a stdconfig package intended to be registered via config.Register[redisconfig.Config].
//
// Note: rate-limit-specific Redis settings live in ratelimitconfig, not here.
package redisconfig

const defaultURL = "localhost:6379"

// Config holds the general-purpose Redis connection URL.
//
// Env prefix: "redis" (e.g. REDIS_URL → redis.url).
type Config struct {
	URL string `koanf:"url"`
}

// SetDefaults fills zero-value fields with documented defaults.
func (c *Config) SetDefaults() {
	if c.URL == "" {
		c.URL = defaultURL
	}
}
