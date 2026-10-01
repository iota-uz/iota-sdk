package jobs

import (
	jobsservices "github.com/iota-uz/iota-sdk/modules/jobs/services"
	"github.com/iota-uz/iota-sdk/pkg/composition"
)

// RegisterHandler wires a job kind into the shared registry. Call it from a
// component Build function; the registration itself runs in a lifecycle hook
// that completes before the HTTP server starts serving, so any job enqueued
// through the API always finds its handler.
func RegisterHandler(builder *composition.Builder, kind string, handler jobsservices.HandlerFunc) {
	composition.ContributeHooks(builder, func(container *composition.Container) ([]composition.Hook, error) {
		registry, err := composition.Resolve[*jobsservices.Registry](container)
		if err != nil {
			return nil, err
		}
		registry.Register(kind, handler)
		return nil, nil
	})
}
