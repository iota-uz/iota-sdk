package bootstrap

import (
	"github.com/iota-uz/iota-sdk/pkg/config"
)

// WithSource attaches a config.Source to the Runtime. When set, the Runtime's
// BuildContext exposes this source so components can call
// composition.ProvideConfig[T] and the auto-provider block can populate typed
// stdconfig values from the source instead of falling back to FromLegacy.
//
// WithSource is orthogonal to WithConfig / IotaConfig: both a Source and a
// legacy *configuration.Configuration can coexist during the migration period.
// The legacy path continues to work unchanged when no Source is provided.
func WithSource(src config.Source) Option {
	return func(o *options) {
		o.source = src
	}
}
