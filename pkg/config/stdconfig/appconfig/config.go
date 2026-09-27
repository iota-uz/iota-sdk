// Package appconfig provides typed configuration for cross-cutting app-level settings
// that do not belong to any specific infrastructure concern.
// It is a stdconfig package intended to be registered via config.Register[appconfig.Config].
package appconfig

import "fmt"

const production = "production"

// Config holds app-level settings shared across the application.
//
// Env prefix: "app" (e.g. GO_APP_ENV → app.environment,
// TELEGRAM_BOT_TOKEN → app.telegrambottoken,
// ENABLE_TEST_ENDPOINTS → app.enabletestendpoints).
type Config struct {
	Environment         string `koanf:"environment"         default:"development"`
	TelegramBotToken    string `koanf:"telegrambottoken"    secret:"true"`
	EnableTestEndpoints bool   `koanf:"enabletestendpoints"`
}

// ConfigPrefix returns the koanf prefix for appconfig ("app").
func (Config) ConfigPrefix() string { return "app" }

// IsProduction reports whether the environment is "production".
func (c *Config) IsProduction() bool {
	return c.Environment == production
}

// IsDev reports whether the environment is not "production".
func (c *Config) IsDev() bool {
	return c.Environment != production
}

// Scheme returns "https" in production, "http" otherwise.
func (c *Config) Scheme() string {
	if c.IsProduction() {
		return "https"
	}
	return "http"
}

// SocketAddress returns the bind address for the given port.
// Production: ":<port>". Dev: "localhost:<port>".
func (c *Config) SocketAddress(port int) string {
	if c.IsProduction() {
		return fmt.Sprintf(":%d", port)
	}
	return fmt.Sprintf("localhost:%d", port)
}
