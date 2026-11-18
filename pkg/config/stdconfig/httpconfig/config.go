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

// Config holds HTTP server settings.
//
// Env prefix: "http" (e.g. PORT → http.port, DOMAIN → http.domain).
type Config struct {
	Port   int    `koanf:"port"           default:"3200"`
	Domain string `koanf:"domain"         default:"localhost"`
	// OriginOverride pins the return value of Origin(), bypassing scheme/domain/port
	// computation. Maps from ORIGIN env var. Unrelated to Source.Origin (provider provenance).
	OriginOverride string   `koanf:"origin"`
	AllowedOrigins []string `koanf:"allowedorigins" default:"http://localhost:3000"`

	// Timeouts bounds every connection of the HTTP server
	// (HTTP_TIMEOUTS_READHEADER, HTTP_TIMEOUTS_READ, HTTP_TIMEOUTS_WRITE,
	// HTTP_TIMEOUTS_IDLE). A negative value such as "-1s" disables a timeout.
	Timeouts TimeoutsConfig `koanf:"timeouts"`

	// Shutdown controls graceful shutdown on SIGTERM/SIGINT
	// (HTTP_SHUTDOWN_TIMEOUT, HTTP_SHUTDOWN_DRAINDELAY).
	Shutdown ShutdownConfig `koanf:"shutdown"`
}

// TimeoutsConfig mirrors the net/http server timeouts. Responses that switch to
// Server-Sent Events, and hijacked websocket connections, are exempt from the
// write timeout.
type TimeoutsConfig struct {
	ReadHeader time.Duration `koanf:"readheader" default:"10s"`
	Read       time.Duration `koanf:"read"       default:"5m"`
	Write      time.Duration `koanf:"write"      default:"5m"`
	Idle       time.Duration `koanf:"idle"       default:"2m"`
}

// ShutdownConfig controls the graceful shutdown of the HTTP server.
type ShutdownConfig struct {
	// Timeout is how long in-flight requests may take to finish after a
	// shutdown signal before their connections are closed.
	Timeout time.Duration `koanf:"timeout" default:"25s"`
	// DrainDelay keeps serving new requests (while readiness already reports
	// 503) so a load balancer can stop routing traffic before the listener
	// closes. Zero closes the listener right away.
	DrainDelay time.Duration `koanf:"draindelay"`
}

// ConfigPrefix returns the koanf prefix for httpconfig ("http").
func (Config) ConfigPrefix() string { return "http" }

// Origin builds the scheme://host[:port] URL for this server.
// Production drops the explicit port; dev keeps it.
// If OriginOverride is non-empty, it wins and app is unused.
// Unrelated to Source.Origin (provider provenance).
func (c *Config) Origin(app *appconfig.Config) string {
	if c.OriginOverride != "" {
		return c.OriginOverride
	}
	if app.IsProduction() {
		return fmt.Sprintf("%s://%s", app.Scheme(), c.Domain)
	}
	return fmt.Sprintf("%s://%s:%d", app.Scheme(), c.Domain, c.Port)
}
