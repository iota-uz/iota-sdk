package composition

import (
	"context"
	"fmt"
	"reflect"

	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

// ContributeEventHandler registers an event-bus handler that is
// auto-subscribed at engine Start and auto-unsubscribed at engine Stop.
// The handler value is either a direct function compatible with
// eventbus.EventBus.Subscribe, or a method reference taken from a service
// resolved earlier in the container.
//
// Compared to inline `eventBus.Subscribe(handler)` chains inside hooks,
// this declaration takes one line and unsubscribe is automatic.
//
// Example — direct function:
//
//	composition.ContributeEventHandler(builder, readModelHandler.OnClientCreated)
//
// Example — resolving a handler service first:
//
//	composition.ProvideFunc(builder, handlers.NewClientHandler)
//	composition.ContributeEventHandlerFunc(builder, func(h *handlers.ClientHandler) any {
//	    return h.OnCreated
//	})
func ContributeEventHandler(builder *Builder, handler any) {
	if builder == nil {
		panic("composition: builder is nil")
	}
	if handler == nil {
		panic("composition: ContributeEventHandler: handler is nil")
	}
	builder.eventHandlerSeq++
	name := fmt.Sprintf("event-handler/%s/%T/%d", builder.descriptor.Name, handler, builder.eventHandlerSeq)
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

// ContributeEventHandlerFunc registers an event-bus subscription whose
// handler is built from a service resolved out of the container. The
// factory is invoked during Engine.Compile.materialize — the same phase
// that resolves every other contribution — so dependency-resolution
// errors, nil-handler panics, and construction side effects all surface
// at compile time, not lazily at Start. Only the eventbus.Subscribe call
// is deferred to Hook.Start; the matching Unsubscribe runs from StopFn.
//
// This is the typical path when the handler is a method on a typed
// service built via ProvideFunc:
//
//	composition.ProvideFunc(builder, handlers.NewClientHandler)
//	composition.ContributeEventHandlerFunc(builder,
//	    func(h *handlers.ClientHandler) any { return h.OnCreated })
//
// The factory's single parameter is resolved from the container by type;
// constructors with multiple dependencies should provide an intermediate
// service and use ProvideFunc for it.
func ContributeEventHandlerFunc[T any](builder *Builder, factory func(T) any) {
	if builder == nil {
		panic("composition: builder is nil")
	}
	if factory == nil {
		panic("composition: ContributeEventHandlerFunc: factory is nil")
	}
	serviceKey := keyFor(reflect.TypeOf((*T)(nil)).Elem(), "")
	builder.eventHandlerSeq++
	name := fmt.Sprintf("event-handler/%s/%s/%d", builder.descriptor.Name, serviceKey, builder.eventHandlerSeq)
	ContributeHooks(builder, func(container *Container) ([]Hook, error) {
		bus, err := Resolve[eventbus.EventBus](container)
		if err != nil {
			return nil, err
		}
		svc, err := Resolve[T](container)
		if err != nil {
			return nil, err
		}
		handler := factory(svc)
		if handler == nil {
			return nil, fmt.Errorf("composition: ContributeEventHandlerFunc: factory returned nil handler")
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
