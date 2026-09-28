package bootstrap

import (
	"github.com/iota-uz/iota-sdk/pkg/config"
)

// WithSource attaches a config.Source to the Runtime. When set, the Runtime's
// BuildContext exposes this source so components can call
// composition.ProvideConfig[T] and the auto-provider block can populate typed
// stdconfig values from the source.
func WithSource(src config.Source) Option {
	return func(o *options) {
		o.source = src
	}
}
