package jobs

import (
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/jobs"
)

// RegisterHandler wires a job kind into the shared registry. Call it from a
// component Build function; registration completes during graph
// materialization, before the HTTP server starts serving, so any job
// enqueued through the API always finds its handler.
func RegisterHandler(builder *composition.Builder, kind string, handler jobs.HandlerFunc) {
	composition.ContributeHooks(builder, func(container *composition.Container) ([]composition.Hook, error) {
		registry, err := composition.Resolve[*jobs.Registry](container)
		if err != nil {
			return nil, err
		}
		registry.Register(kind, handler)
		return nil, nil
	})
}
