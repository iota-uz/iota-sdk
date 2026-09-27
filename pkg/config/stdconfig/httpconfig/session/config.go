// Package session provides typed configuration for session lifetime settings.
// It is a stdconfig package intended to be registered via config.Register[session.Config].
package session

import (
	"errors"
	"time"
)

// Config holds session-lifetime settings.
//
// Env prefix: "http.session" (e.g. SESSION_DURATION → http.session.duration).
type Config struct {
	// Duration is the session lifetime.
	Duration time.Duration `koanf:"duration" default:"720h"`
}

// ConfigPrefix returns the koanf prefix for session ("http.session").
func (Config) ConfigPrefix() string { return "http.session" }

// Validate checks that Duration is positive.
func (c *Config) Validate() error {
	if c.Duration <= 0 {
		return errors.New("sessionconfig: duration must be positive")
	}
	return nil
}
