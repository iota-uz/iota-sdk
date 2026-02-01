package hooks

import "context"

// EventHandler processes events published to the EventBus.
type EventHandler interface {
	// Handle processes an event.
	// Errors are logged but do not stop other handlers.
	Handle(ctx context.Context, event Event) error
}

// EventHandlerFunc is a function adapter for EventHandler.
// It allows using ordinary functions as event handlers.
type EventHandlerFunc func(ctx context.Context, event Event) error

// Handle implements EventHandler.
func (f EventHandlerFunc) Handle(ctx context.Context, event Event) error {
	return f(ctx, event)
}
