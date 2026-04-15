// Package appconfig provides typed configuration for cross-cutting app-level settings
// that do not belong to any specific infrastructure concern.
// It is a stdconfig package intended to be registered via config.Register[appconfig.Config].
package appconfig

const (
	production         = "production"
	defaultEnvironment = "development"
)

// Config holds app-level settings shared across the application.
//
// Env prefix: "app" (e.g. GO_APP_ENV → app.environment,
// TELEGRAM_BOT_TOKEN → app.telegrambottoken,
// ENABLE_TEST_ENDPOINTS → app.enabletestendpoints).
//
// Note: httpconfig.Config also exposes Environment for HTTP-specific helpers
// (Scheme, IsDev, IsProduction, SocketAddress). Both packages read the same
// source value independently — intentional duplication of an independent slice.
type Config struct {
	Environment         string `koanf:"environment"`
	TelegramBotToken    string `koanf:"telegrambottoken" secret:"true"`
	EnableTestEndpoints bool   `koanf:"enabletestendpoints"`
}

// SetDefaults fills zero-value fields with documented defaults.
func (c *Config) SetDefaults() {
	if c.Environment == "" {
		c.Environment = defaultEnvironment
	}
}

// IsProduction reports whether the environment is "production".
func (c *Config) IsProduction() bool {
	return c.Environment == production
}

// IsDev reports whether the environment is not "production".
func (c *Config) IsDev() bool {
	return c.Environment != production
}
