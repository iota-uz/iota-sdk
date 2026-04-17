// Package redisconfig provides typed configuration for the top-level Redis connection.
// It is a stdconfig package intended to be registered via config.Register[redisconfig.Config].
//
// Note: rate-limit-specific Redis settings live in ratelimitconfig, not here.
package redisconfig

// Config holds the general-purpose Redis connection URL.
//
// Env prefix: "redis" (e.g. REDIS_URL → redis.url).
type Config struct {
	URL string `koanf:"url" default:"localhost:6379"`
}

// ConfigPrefix returns the koanf prefix for redisconfig ("redis").
func (Config) ConfigPrefix() string { return "redis" }
