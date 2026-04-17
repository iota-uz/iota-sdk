// Package httpconfig provides typed configuration for the HTTP server,
// cookies, headers, session, and pagination settings.
// It is a stdconfig package intended to be registered via config.Register[httpconfig.Config].
package httpconfig

import (
	"fmt"
	"time"
)

const production = "production"

// HeadersConfig groups HTTP header name settings.
type HeadersConfig struct {
	// RequestID is the header name SDK looks for to propagate request IDs.
	// Defaults to "X-Request-ID".
	RequestID string `koanf:"requestid"`
	// RealIP is the header name SDK uses to extract the real client IP.
	// Defaults to "X-Real-IP".
	RealIP string `koanf:"realip"`
}

// CookiesConfig groups cookie key names.
type CookiesConfig struct {
	// SID is the session-ID cookie key. Defaults to "sid".
	SID string `koanf:"sid"`
	// OAuthState is the OAuth state cookie key. Defaults to "oauthState".
	OAuthState string `koanf:"oauthstate"`
}

// SessionConfig groups session-lifetime settings.
type SessionConfig struct {
	// Duration is the session lifetime. Defaults to 720h.
	Duration time.Duration `koanf:"duration"`
}

// PaginationConfig groups page-size settings.
type PaginationConfig struct {
	// PageSize is the default number of items per page. Defaults to 25.
	PageSize int `koanf:"pagesize"`
	// MaxPageSize is the maximum allowed page size. Defaults to 100.
	MaxPageSize int `koanf:"maxpagesize"`
}

// Config holds all HTTP server, cookie, header, session, and pagination settings.
//
// Env prefix: "http" (e.g. PORT → http.port, GO_APP_ENV → http.environment).
// Note: SocketAddress is derived at runtime via SocketAddress(); it is never stored.
type Config struct {
	Port           int              `koanf:"port"`
	Domain         string           `koanf:"domain"`
	Origin         string           `koanf:"origin"`
	AllowedOrigins []string         `koanf:"allowedorigins"`
	Environment    string           `koanf:"environment"`
	Headers        HeadersConfig    `koanf:"headers"`
	Cookies        CookiesConfig    `koanf:"cookies"`
	Session        SessionConfig    `koanf:"session"`
	Pagination     PaginationConfig `koanf:"pagination"`
}

// ConfigPrefix returns the koanf prefix for httpconfig ("http").
func (Config) ConfigPrefix() string { return "http" }

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

// SocketAddress returns ":<port>" in production, "localhost:<port>" otherwise.
// This mirrors the legacy load() logic (lines 401-406 of environment.go).
func (c *Config) SocketAddress() string {
	if c.IsProduction() {
		return fmt.Sprintf(":%d", c.Port)
	}
	return fmt.Sprintf("localhost:%d", c.Port)
}

// SetDefaults fills zero-valued fields with fallback values that match the
// legacy envDefault tags from pkg/configuration. Called automatically by
// config.Register after Unmarshal.
func (c *Config) SetDefaults() {
	if c.Port == 0 {
		c.Port = 3200
	}
	if c.Domain == "" {
		c.Domain = "localhost"
	}
	if c.Origin == "" {
		if c.IsProduction() {
			c.Origin = fmt.Sprintf("%s://%s", c.Scheme(), c.Domain)
		} else {
			c.Origin = fmt.Sprintf("%s://%s:%d", c.Scheme(), c.Domain, c.Port)
		}
	}
	if len(c.AllowedOrigins) == 0 {
		c.AllowedOrigins = []string{"http://localhost:3000"}
	}
	if c.Environment == "" {
		c.Environment = "development"
	}
	if c.Headers.RequestID == "" {
		c.Headers.RequestID = "X-Request-ID"
	}
	if c.Headers.RealIP == "" {
		c.Headers.RealIP = "X-Real-IP"
	}
	if c.Cookies.SID == "" {
		c.Cookies.SID = "sid"
	}
	if c.Cookies.OAuthState == "" {
		c.Cookies.OAuthState = "oauthState"
	}
	if c.Session.Duration == 0 {
		c.Session.Duration = 720 * time.Hour
	}
	if c.Pagination.PageSize == 0 {
		c.Pagination.PageSize = 25
	}
	if c.Pagination.MaxPageSize == 0 {
		c.Pagination.MaxPageSize = 100
	}
}
