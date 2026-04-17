// Package httpconfig provides typed configuration for the HTTP server.
// It is a stdconfig package intended to be registered via config.Register[httpconfig.Config].
//
// Sub-packages httpconfig/headers, httpconfig/cookies, httpconfig/session,
// and httpconfig/pagination provide focused configs for their respective areas.
package httpconfig

import (
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/appconfig"
)

// Config holds HTTP server settings.
//
// Env prefix: "http" (e.g. PORT → http.port, DOMAIN → http.domain).
type Config struct {
	Port           int      `koanf:"port"           default:"3200"`
	Domain         string   `koanf:"domain"         default:"localhost"`
	OriginOverride string   `koanf:"origin"`
	AllowedOrigins []string `koanf:"allowedorigins" default:"http://localhost:3000"`
}

// ConfigPrefix returns the koanf prefix for httpconfig ("http").
func (Config) ConfigPrefix() string { return "http" }

// Origin returns the canonical scheme://host[:port] URL.
// Production drops the explicit port; dev keeps it.
// If OriginOverride is non-empty, it wins and app is unused.
func (c *Config) Origin(app *appconfig.Config) string {
	if c.OriginOverride != "" {
		return c.OriginOverride
	}
	if app.IsProduction() {
		return fmt.Sprintf("%s://%s", app.Scheme(), c.Domain)
	}
	return fmt.Sprintf("%s://%s:%d", app.Scheme(), c.Domain, c.Port)
}
