package appconfig

import "github.com/iota-uz/iota-sdk/pkg/configuration"

// FromLegacy produces a Config from the monolithic *configuration.Configuration.
// SetDefaults is called so that zero-value fields receive documented defaults.
func FromLegacy(c *configuration.Configuration) Config {
	cfg := Config{
		Environment:         c.GoAppEnvironment,
		TelegramBotToken:    c.TelegramBotToken,
		EnableTestEndpoints: c.EnableTestEndpoints,
	}
	cfg.SetDefaults()
	return cfg
}
