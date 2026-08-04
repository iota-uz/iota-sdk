package bootstrap

import (
	"github.com/iota-uz/iota-sdk/pkg/health"
)

// WithCapabilityRegistry attaches a shared health.CapabilityRegistry to the
// Runtime. Gate helpers inside component Build() register CapabilityProbes
// here so the /system/info Capabilities panel reflects every optional
// feature's enabled/disabled state. When omitted, the BuildContext lazily
// creates a private registry and the probes remain SDK-internal (nothing
// on /system/info reads them). Most server binaries attach the same registry
// they hand to the HealthUIController options so the UI and the gate layer
// share one list.
func WithCapabilityRegistry(registry health.CapabilityRegistry) Option {
	return func(o *options) {
		o.capabilityRegistry = registry
	}
}
