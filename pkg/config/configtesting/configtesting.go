// Package configtesting provides helpers for testing code that consumes
// stdconfig packages. It wires a static provider, builds a Source, and
// registers the requested config type — all in one call.
package configtesting

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
)

// Populated builds a source from overrides (flat dot-key map), registers T,
// and returns the populated *T. Panics (not t.Fatal) so it works as an
// initialiser in table-driven tests where t.Fatal is unavailable.
//
// Example:
//
//	cfg := configtesting.Populated[httpconfig.Config](t, map[string]any{
//	    "http.port": 9090,
//	})
//	// cfg.Port == 9090, all other fields have their defaults
func Populated[T config.Prefixed](t testing.TB, overrides map[string]any) *T {
	t.Helper()
	src, err := config.Build(static.New(overrides))
	if err != nil {
		panic("configtesting.Populated: Build: " + err.Error())
	}
	r := config.NewRegistry(src)
	cfg, err := config.Register[T](r)
	if err != nil {
		panic("configtesting.Populated: Register: " + err.Error())
	}
	return cfg
}
