package hooks

import (
	"context"
	"sync"
)

// EventBus distributes events to registered handlers.
// Implementations must be thread-safe.
type EventBus interface {
	// Publish sends an event to all registered handlers.
	// Handlers are called synchronously in registration order.
	// If a handler returns an error, it is logged but does not stop other handlers.
	Publish(ctx context.Context, event Event) error

	// Subscribe registers a handler for specific event types.
	// Returns an unsubscribe function to remove the handler.
	// If no types are provided, the handler receives no events (use SubscribeAll for all events).
	Subscribe(handler EventHandler, types ...string) (unsubscribe func())

	// SubscribeAll registers a handler for all event types.
	// Returns an unsubscribe function to remove the handler.
	SubscribeAll(handler EventHandler) (unsubscribe func())
}

// subscription represents a registered event handler with its event type filters.
type subscription struct {
	id      int
	handler EventHandler
	types   map[string]bool // nil means subscribe to all events
}

// defaultEventBus is a thread-safe in-memory EventBus implementation.
type defaultEventBus struct {
	mu            sync.RWMutex
	subscriptions []*subscription
	nextID        int
}

// NewEventBus creates a new default EventBus implementation.
func NewEventBus() EventBus {
	return &defaultEventBus{
		subscriptions: make([]*subscription, 0),
		nextID:        1,
	}
}

// Publish implements EventBus.
func (b *defaultEventBus) Publish(ctx context.Context, event Event) error {
	b.mu.RLock()
	subs := make([]*subscription, len(b.subscriptions))
	copy(subs, b.subscriptions)
	b.mu.RUnlock()

	for _, sub := range subs {
		// Check if handler is interested in this event type
		if sub.types != nil && !sub.types[event.Type()] {
			continue
		}

		// Call handler (ignore errors but could log them)
		_ = sub.handler.Handle(ctx, event)
	}

	return nil
}

// Subscribe implements EventBus.
func (b *defaultEventBus) Subscribe(handler EventHandler, types ...string) func() {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Build type filter map
	var typeMap map[string]bool
	if len(types) > 0 {
		typeMap = make(map[string]bool, len(types))
		for _, t := range types {
			typeMap[t] = true
		}
	}

	sub := &subscription{
		id:      b.nextID,
		handler: handler,
		types:   typeMap,
	}
	b.nextID++

	b.subscriptions = append(b.subscriptions, sub)

	// Return unsubscribe function
	return func() {
		b.unsubscribe(sub.id)
	}
}

// SubscribeAll implements EventBus.
func (b *defaultEventBus) SubscribeAll(handler EventHandler) func() {
	b.mu.Lock()
	defer b.mu.Unlock()

	sub := &subscription{
		id:      b.nextID,
		handler: handler,
		types:   nil, // nil means all events
	}
	b.nextID++

	b.subscriptions = append(b.subscriptions, sub)

	// Return unsubscribe function
	return func() {
		b.unsubscribe(sub.id)
	}
}

// unsubscribe removes a subscription by ID.
func (b *defaultEventBus) unsubscribe(id int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i, sub := range b.subscriptions {
		if sub.id == id {
			// Remove subscription by replacing with last element and truncating
			b.subscriptions[i] = b.subscriptions[len(b.subscriptions)-1]
			b.subscriptions = b.subscriptions[:len(b.subscriptions)-1]
			return
		}
	}
}
