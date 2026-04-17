// Package ratelimitconfig provides typed configuration for HTTP rate limiting.
// It is a stdconfig package intended to be registered via config.Register[ratelimitconfig.Config].
package ratelimitconfig

import "fmt"

// Config holds all rate-limit settings.
// Env prefix: "ratelimit" (e.g. RATE_LIMIT_ENABLED → ratelimit.enabled).
//
// The Enabled field is a *bool so an explicit RATE_LIMIT_ENABLED=false from
// the environment is distinguishable from "absent". When absent the tag engine
// allocates the pointer and sets it to true. Use IsEnabled() to read the value
// safely without a nil-check.
type Config struct {
	// Enabled controls whether rate limiting is active.
	// Pointer so explicit `false` from env distinguishes from absent.
	// Use IsEnabled() to read.
	Enabled   *bool  `koanf:"enabled"   default:"true"`
	GlobalRPS int    `koanf:"globalrps" default:"1000"`
	Storage   string `koanf:"storage"   default:"memory"`
	RedisURL  string `koanf:"redisurl"`
}

// ConfigPrefix returns the koanf prefix for ratelimitconfig ("ratelimit").
func (Config) ConfigPrefix() string { return "ratelimit" }

// IsEnabled reports whether rate-limiting is active.
// A nil Enabled pointer is treated as true (tag-default path not yet run).
func (c *Config) IsEnabled() bool {
	if c.Enabled == nil {
		return true
	}
	return *c.Enabled
}

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
