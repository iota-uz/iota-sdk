// Package httpconfig provides typed configuration for the HTTP server.
// It is a stdconfig package intended to be registered via config.Register[httpconfig.Config].
//
// Sub-packages httpconfig/headers, httpconfig/cookies, httpconfig/session,
// and httpconfig/pagination provide focused configs for their respective areas.
package httpconfig

import (
	"fmt"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/appconfig"
)

// HeadersConfig groups HTTP header name settings (retained for backward compatibility).
// Prefer httpconfig/headers.Config for new code.
type HeadersConfig struct {
	RequestID string `koanf:"requestid"`
	RealIP    string `koanf:"realip"`
}

// CookiesConfig groups cookie key names (retained for backward compatibility).
// Prefer httpconfig/cookies.Config for new code.
type CookiesConfig struct {
	SID        string `koanf:"sid"`
	OAuthState string `koanf:"oauthstate"`
}

// SessionConfig groups session-lifetime settings (retained for backward compatibility).
// Prefer httpconfig/session.Config for new code.
type SessionConfig struct {
	Duration time.Duration `koanf:"duration"`
}

// PaginationConfig groups page-size settings (retained for backward compatibility).
// Prefer httpconfig/pagination.Config for new code.
type PaginationConfig struct {
	PageSize    int `koanf:"pagesize"`
	MaxPageSize int `koanf:"maxpagesize"`
}

// Config holds HTTP server settings.
//
// Env prefix: "http" (e.g. PORT → http.port, DOMAIN → http.domain).
type Config struct {
	Port           int              `koanf:"port"           default:"3200"`
	Domain         string           `koanf:"domain"         default:"localhost"`
	OriginOverride string           `koanf:"origin"`
	AllowedOrigins []string         `koanf:"allowedorigins" default:"http://localhost:3000"`
	// Headers/Cookies/Session/Pagination retained for backward compatibility.
	// Prefer the dedicated sub-packages for new code.
	Headers    HeadersConfig    `koanf:"headers"`
	Cookies    CookiesConfig    `koanf:"cookies"`
	Session    SessionConfig    `koanf:"session"`
	Pagination PaginationConfig `koanf:"pagination"`
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
