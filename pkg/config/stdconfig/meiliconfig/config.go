// Package meiliconfig provides typed configuration for the MeiliSearch connection.
// It is a stdconfig package intended to be registered via config.Register[meiliconfig.Config].
package meiliconfig

// Config holds MeiliSearch connection settings.
//
// Env prefix: "meili" (e.g. MEILI_URL → meili.url, MEILI_API_KEY → meili.apikey).
type Config struct {
	URL    string `koanf:"url"`
	APIKey string `koanf:"apikey" secret:"true"`
}

// ConfigPrefix returns the koanf prefix for meiliconfig ("meili").
func (Config) ConfigPrefix() string { return "meili" }

// IsConfigured returns true when both URL and APIKey are set.
// MeiliSearch activation is implicit — no explicit enable flag required.
// Implements config.Configured for composition.SkipIfDisabled gating.
func (c *Config) IsConfigured() bool {
	return c.URL != "" && c.APIKey != ""
}

// DisabledReason explains why Meili is off when IsConfigured returns false.
func (c *Config) DisabledReason() string {
	return "MEILI_URL and MEILI_APIKEY required"
}
