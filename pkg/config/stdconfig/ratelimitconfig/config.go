// Package ratelimitconfig provides typed configuration for HTTP rate limiting.
// It is a stdconfig package intended to be registered via config.Register[ratelimitconfig.Config].
package ratelimitconfig

import "fmt"

// Config holds all rate-limit settings.
// Env prefix: "ratelimit" (e.g. RATE_LIMIT_ENABLED → ratelimit.enabled).
type Config struct {
	Enabled   bool   `koanf:"enabled"`
	GlobalRPS int    `koanf:"globalrps"`
	Storage   string `koanf:"storage"`
	RedisURL  string `koanf:"redisurl"`
}

// ConfigPrefix returns the koanf prefix for ratelimitconfig ("ratelimit").
func (Config) ConfigPrefix() string { return "ratelimit" }

// SetDefaults applies per-field defaults for zero-valued fields, matching the
// pattern used by all other stdconfig packages.
//
// Enabled: defaults to true only when the config is completely zero-valued
// (i.e. no field was set by any provider). This preserves legacy semantics
// while allowing callers to set Enabled=false explicitly.
func (c *Config) SetDefaults() {
	allZero := !c.Enabled && c.GlobalRPS == 0 && c.Storage == "" && c.RedisURL == ""
	if c.GlobalRPS == 0 {
		c.GlobalRPS = 1000
	}
	if c.Storage == "" {
		c.Storage = "memory"
	}
	if allZero {
		c.Enabled = true
	}
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
