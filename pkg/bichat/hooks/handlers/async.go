package handlers

import (
	"context"
	"sync"

	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks"
)

// AsyncHandler wraps an EventHandler for asynchronous processing.
// Events are queued in a buffered channel and processed in a background goroutine.
// This prevents slow handlers from blocking event publishers.
type AsyncHandler struct {
	handler    hooks.EventHandler
	eventQueue chan eventWithContext
	wg         sync.WaitGroup
	stopOnce   sync.Once
	stopCh     chan struct{}
}

// eventWithContext pairs an event with its context for async processing.
type eventWithContext struct {
	ctx   context.Context
	event hooks.Event
}

// NewAsyncHandler creates a new AsyncHandler that processes events asynchronously.
// The bufferSize determines how many events can be queued before blocking.
// Call Close() when done to wait for pending events and cleanup resources.
func NewAsyncHandler(handler hooks.EventHandler, bufferSize int) *AsyncHandler {
	if bufferSize <= 0 {
		bufferSize = 100 // Default buffer size
	}

	h := &AsyncHandler{
		handler:    handler,
		eventQueue: make(chan eventWithContext, bufferSize),
		stopCh:     make(chan struct{}),
	}

	// Start background processor
	h.wg.Add(1)
	go h.processEvents()

	return h
}

// Handle implements EventHandler by queuing events for async processing.
func (h *AsyncHandler) Handle(ctx context.Context, event hooks.Event) error {
	select {
	case h.eventQueue <- eventWithContext{ctx: ctx, event: event}:
		return nil
	case <-h.stopCh:
		// Handler is stopped, drop the event
		return nil
	default:
		// Queue is full, drop the event (could also block or return error)
		// TODO: Consider adding metrics for dropped events
		return nil
	}
}

// processEvents runs in a background goroutine and processes queued events.
func (h *AsyncHandler) processEvents() {
	defer h.wg.Done()

	for {
		select {
		case ev := <-h.eventQueue:
			// Process event (ignore errors)
			_ = h.handler.Handle(ev.ctx, ev.event)
		case <-h.stopCh:
			// Drain remaining events before stopping
			for {
				select {
				case ev := <-h.eventQueue:
					_ = h.handler.Handle(ev.ctx, ev.event)
				default:
					return
				}
			}
		}
	}
}

// Close stops the async handler and waits for pending events to be processed.
// This should be called when shutting down to ensure no events are lost.
func (h *AsyncHandler) Close() {
	h.stopOnce.Do(func() {
		close(h.stopCh)
		h.wg.Wait()
	})
}
