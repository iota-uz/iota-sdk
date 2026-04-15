// Package introspect provides an HTTP handler that renders the current
// configuration as redacted JSON. It is intended to be mounted at an
// admin-only path (e.g. /debug/config) behind the caller's auth middleware.
//
// Route wiring is a module/application concern; this package ships the handler
// as a reusable library.
package introspect

import (
	"encoding/json"
	"net/http"

	"github.com/iota-uz/iota-sdk/pkg/config"
)

// Handler returns an http.HandlerFunc that renders the snapshot returned by
// snapshot() as redacted JSON (via config.Redact).
//
// authz must return true iff the caller is authorized to view configuration.
// A nil authz always denies access — explicit opt-in is required.
//
// On auth failure the handler responds with HTTP 403 and a plain-text body.
// On success it writes Content-Type: application/json and the redacted
// representation of the snapshot value.
//
// The snapshot function is called on every request so callers can compose the
// snapshot from whatever registered configs are available at request time.
// Example usage:
//
//	dbCfg := ...  // *dbconfig.Config resolved from the container
//	handler := introspect.Handler(
//	    func() any { return struct{ DB *dbconfig.Config }{dbCfg} },
//	    func(r *http.Request) bool { return isSuperAdmin(r) },
//	)
//	router.Handle("/debug/config", handler)
func Handler(snapshot func() any, authz func(*http.Request) bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if authz == nil || !authz(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		v := snapshot()
		redacted := config.Redact(v)

		// config.Redact already produces a valid JSON string; we write it
		// directly rather than re-marshalling to avoid double escaping.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(redacted))
	}
}

// HandlerFromMap returns an http.HandlerFunc that renders a flat map of
// named config values (each individually redacted) as a JSON object.
//
// This is a convenience wrapper when callers have multiple disjoint config
// structs and want them labelled by key in the output.
//
// authz semantics are identical to Handler.
func HandlerFromMap(configs map[string]any, authz func(*http.Request) bool) http.HandlerFunc {
	return Handler(func() any {
		out := make(map[string]json.RawMessage, len(configs))
		for k, v := range configs {
			out[k] = json.RawMessage(config.Redact(v))
		}
		return out
	}, authz)
}
