// Package appconfig provides typed configuration for cross-cutting app-level settings
// that do not belong to any specific infrastructure concern.
// It is a stdconfig package intended to be registered via config.Register[appconfig.Config].
package appconfig

import "fmt"

// Config holds app-level settings shared across the application.
//
// Env prefix: "app" (e.g. APP_ENVIRONMENT → app.environment,
// APP_TELEGRAMBOTTOKEN → app.telegrambottoken,
// APP_ENABLETESTENDPOINTS → app.enabletestendpoints).
type Config struct {
	Environment         string `koanf:"environment"         default:"development"`
	TelegramBotToken    string `koanf:"telegrambottoken"    secret:"true"`
	EnableTestEndpoints bool   `koanf:"enabletestendpoints"`
}

// ConfigPrefix returns the koanf prefix for appconfig ("app").
func (Config) ConfigPrefix() string { return "app" }

// Env returns the parsed deployment environment. Unrecognised values yield
// EnvironmentUnknown; see ParseEnvironment for the accepted spellings.
func (c *Config) Env() Environment {
	env, _ := ParseEnvironment(c.Environment)
	return env
}

// IsDevelopment reports whether the environment is development.
// It is false on staging, test and production.
func (c *Config) IsDevelopment() bool { return c.Env().IsDevelopment() }

// IsTest reports whether the environment is test.
func (c *Config) IsTest() bool { return c.Env().IsTest() }

// IsStaging reports whether the environment is staging.
func (c *Config) IsStaging() bool { return c.Env().IsStaging() }

// IsProduction reports whether the environment is production.
func (c *Config) IsProduction() bool { return c.Env().IsProduction() }

// IsDev reports whether the environment is not production.
//
// Deprecated: IsDev is also true on staging, preprod and test, which enables
// development behaviour on shared deployments. Use IsDevelopment for
// development only, Env().IsLocal() for development or test, or
// !IsProduction() to keep the old meaning explicitly.
func (c *Config) IsDev() bool {
	return !c.IsProduction()
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
