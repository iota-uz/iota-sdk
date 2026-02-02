package applet

import (
	"context"
	"net/http"
	"time"
)

// ErrorContextEnricher enriches error context for frontend error boundaries.
// Implementations can customize error handling metadata based on tenant,
// environment, or application configuration.
type ErrorContextEnricher interface {
	EnrichContext(ctx context.Context, r *http.Request) (*ErrorContext, error)
}

// MetricsRecorder records metrics for context building operations.
// Implementations can send metrics to Prometheus, StatsD, or other monitoring systems.
type MetricsRecorder interface {
	// RecordDuration records the duration of an operation
	RecordDuration(name string, duration time.Duration, labels map[string]string)

	// IncrementCounter increments a counter metric
	IncrementCounter(name string, labels map[string]string)
}

// SessionStore reads session expiry times from the session storage backend.
// This allows ContextBuilder to use actual session expiry instead of configured default.
//
// Example implementation using gorilla/sessions:
//
//	type GorillaSessionStore struct {
//	    store *sessions.CookieStore
//	}
//
//	func (s *GorillaSessionStore) GetSessionExpiry(r *http.Request) time.Time {
//	    session, err := s.store.Get(r, "session-name")
//	    if err != nil || session.IsNew {
//	        return time.Time{} // Zero time = not found
//	    }
//	    if expiresAt, ok := session.Values["expires_at"].(time.Time); ok {
//	        return expiresAt
//	    }
//	    return time.Time{}
//	}
type SessionStore interface {
	// GetSessionExpiry returns the expiry time for the current session.
	// Returns zero time if session not found or error occurs.
	GetSessionExpiry(r *http.Request) time.Time
}
