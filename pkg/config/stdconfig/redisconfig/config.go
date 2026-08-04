// Package redisconfig provides typed configuration for the top-level Redis connection.
// It is a stdconfig package intended to be registered via config.Register[redisconfig.Config].
//
// Redis is an optional integration: leaving REDIS_URL unset keeps it off.
// Consumers wanting local-dev Redis should set REDIS_URL=localhost:6379
// explicitly — the previous tag default has been removed so that unset
// truly means "not configured" and gate helpers can detect the off state.
//
// Note: rate-limit-specific Redis settings live in ratelimitconfig, not here.
package redisconfig

// Config holds the general-purpose Redis connection URL.
//
// Env prefix: "redis" (e.g. REDIS_URL → redis.url).
type Config struct {
	URL string `koanf:"url"`
}

// ConfigPrefix returns the koanf prefix for redisconfig ("redis").
func (Config) ConfigPrefix() string { return "redis" }

// IsConfigured returns true when a Redis URL has been supplied. Implements
// config.Configured so composition.SkipIfDisabled / GatedRegister can gate
// Redis-dependent wiring.
func (c *Config) IsConfigured() bool { return c.URL != "" }

// DisabledReason explains why Redis is off when IsConfigured returns false.
func (c *Config) DisabledReason() string { return "REDIS_URL not set" }
