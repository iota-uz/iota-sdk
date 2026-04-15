package meiliconfig

import "github.com/iota-uz/iota-sdk/pkg/configuration"

// FromLegacy produces a Config from the monolithic *configuration.Configuration.
// Pure field-for-field mapping — no validation, no derivation.
func FromLegacy(c *configuration.Configuration) Config {
	return Config{
		URL:    c.MeiliURL,
		APIKey: c.MeiliAPIKey,
	}
}
