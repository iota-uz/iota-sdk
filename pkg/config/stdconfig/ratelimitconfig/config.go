// Package ratelimitconfig provides typed configuration for HTTP rate limiting.
// It is a stdconfig package intended to be registered via config.Register[ratelimitconfig.Config].
package ratelimitconfig

import "fmt"

// Config holds all rate-limit settings.
// Env prefix: "ratelimit" (e.g. RATE_LIMIT_ENABLED → ratelimit.enabled).
//
// Semantic note: Enabled defaults to true independently of other fields via the
// default:"true" struct tag. The old all-zero gate silently left Enabled=false
// whenever any ratelimit var was set without RATE_LIMIT_ENABLED — that was a
// correctness bug. The tag-based default fixes it.
//
// Limitation: because false is the zero value for bool, the tag engine cannot
// distinguish "RATE_LIMIT_ENABLED=false" from "absent". The default fires in
// both cases. If you need to disable rate limiting, prefer setting Enabled to
// false programmatically after registration, or use a non-zero sentinel and
// Validate to detect it.
type Config struct {
	Enabled   bool   `koanf:"enabled"   default:"true"`
	GlobalRPS int    `koanf:"globalrps" default:"1000"`
	Storage   string `koanf:"storage"   default:"memory"`
	RedisURL  string `koanf:"redisurl"`
}

// ConfigPrefix returns the koanf prefix for ratelimitconfig ("ratelimit").
func (Config) ConfigPrefix() string { return "ratelimit" }

// Validate checks rate-limit configuration for errors.
// Implements config.Validatable so config.Register invokes it automatically.
func (c *Config) Validate() error {
	if c.GlobalRPS < 0 {
		return fmt.Errorf("ratelimitconfig: globalrps must be non-negative, got %d", c.GlobalRPS)
	}
	if c.GlobalRPS > 1000000 {
		return fmt.Errorf("ratelimitconfig: globalrps too high, maximum is 1,000,000, got %d", c.GlobalRPS)
	}
	if c.Storage != "memory" && c.Storage != "redis" {
		return fmt.Errorf("ratelimitconfig: storage must be 'memory' or 'redis', got %q", c.Storage)
	}
	if c.Storage == "redis" && c.RedisURL == "" {
		return fmt.Errorf("ratelimitconfig: redisurl is required when storage is 'redis'")
	}
	return nil
}
