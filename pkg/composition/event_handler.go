package composition

import (
	"context"
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

// ContributeEventHandler registers an event-bus handler that is auto-subscribed
// at engine Start and auto-unsubscribed at engine Stop. Handlers are arbitrary
// functions matching the eventbus.Subscribe signature; the underlying event
// bus is resolved from the container (auto-provided by the engine).
//
// Compared to inline `eventBus.Subscribe(handler)` chains inside hooks, this
// declaration takes one line and the unsubscribe is automatic.
//
// Example:
//
//	composition.ContributeEventHandler(builder, readModelHandler.OnClientCreated)
//	composition.ContributeEventHandler(builder, readModelHandler.OnPolicyPurchased)
func ContributeEventHandler(builder *Builder, handler interface{}) {
	if builder == nil {
		panic("composition: builder is nil")
	}
	if handler == nil {
		panic("composition: ContributeEventHandler: handler is nil")
	}
	name := fmt.Sprintf("event-handler/%s/%T", builder.descriptor.Name, handler)
	ContributeHooks(builder, func(container *Container) ([]Hook, error) {
		bus, err := Resolve[eventbus.EventBus](container)
		if err != nil {
			return nil, err
		}
		return []Hook{{
			Name: name,
			Start: func(context.Context) (StopFn, error) {
				unsubscribe := bus.Subscribe(handler)
				return func(context.Context) error {
					if unsubscribe != nil {
						unsubscribe()
					}
					return nil
				}, nil
			},
		}}, nil
	})
}

// ContributeEventHandlers is a convenience for the (very common) case of
// subscribing many handlers from the same component.
func ContributeEventHandlers(builder *Builder, handlers ...interface{}) {
	for _, h := range handlers {
		ContributeEventHandler(builder, h)
	}
}
