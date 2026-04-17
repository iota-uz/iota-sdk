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

// SetDefaults applies the same defaults as the legacy RateLimitOptions env tags.
// Called from FromLegacy so zero-value structs get sensible defaults.
func (c *Config) SetDefaults() {
	if !c.Enabled && c.GlobalRPS == 0 && c.Storage == "" {
		// All zero — apply defaults
		c.Enabled = true
		c.GlobalRPS = 1000
		c.Storage = "memory"
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
