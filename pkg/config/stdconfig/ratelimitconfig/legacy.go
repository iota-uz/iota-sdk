package ratelimitconfig

import "github.com/iota-uz/iota-sdk/pkg/configuration"

// FromLegacy produces a Config from the monolithic *configuration.Configuration.
// SetDefaults is called to ensure zero-value fields get sensible defaults when
// the legacy struct was populated from env tags that carry envDefault values.
func FromLegacy(c *configuration.Configuration) Config {
	rl := c.RateLimit
	cfg := Config{
		Enabled:   rl.Enabled,
		GlobalRPS: rl.GlobalRPS,
		Storage:   rl.Storage,
		RedisURL:  rl.RedisURL,
	}
	// Legacy env tags already set defaults via envDefault; mapping is 1:1.
	// SetDefaults is a no-op when all fields are non-zero, so safe to call.
	cfg.SetDefaults()
	return cfg
}
