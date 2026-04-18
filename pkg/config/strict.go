package config

import (
	"os"
	"strconv"
	"strings"
)

// StrictMode controls how the registry treats StatePartiallyConfigured
// entries: configs whose operator-set keys exist under their prefix but
// whose Configured.IsConfigured still returns false. This is almost
// always a typo or a half-finished deployment.
//
//   - StrictYes    — Register returns an error; Seal joins all errors.
//                    Prevents boot when any optional feature is half-configured.
//   - StrictLax    — Register succeeds with state downgraded to Disabled;
//                    Seal logs a warning per entry and returns nil for them.
//                    Appropriate for development where partial configs are
//                    common during iteration.
//   - StrictDefault — resolves to StrictYes when app.environment=="production"
//                     (either via the Source or the APP_ENVIRONMENT env var
//                     fallback) and StrictLax elsewhere. Overridable at the
//                     registry via SetStrict, or via APP_STRICT_CONFIG=true|false
//                     in the environment.
type StrictMode int

const (
	StrictDefault StrictMode = iota
	StrictYes
	StrictLax
)

// resolve materialises StrictDefault into StrictYes or StrictLax based on
// the registry's source and process environment. StrictYes / StrictLax pass
// through unchanged. Callers hold r.mu for reading (at least) — resolve
// does not touch registry state.
func (m StrictMode) resolve(r *Registry) StrictMode {
	if m != StrictDefault {
		return m
	}
	// Explicit override wins over any environment heuristic.
	if v, ok := os.LookupEnv("APP_STRICT_CONFIG"); ok {
		if b, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
			if b {
				return StrictYes
			}
			return StrictLax
		}
	}
	// Production → strict. Consult the Source first; fall back to env.
	env := ""
	if r != nil && r.src != nil {
		if v, ok := r.src.Get("app.environment"); ok {
			if s, ok := v.(string); ok {
				env = s
			}
		}
	}
	if env == "" {
		env = os.Getenv("APP_ENVIRONMENT")
	}
	if strings.EqualFold(env, "production") {
		return StrictYes
	}
	return StrictLax
}
