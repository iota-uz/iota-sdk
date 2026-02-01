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
	metrics    hooks.MetricsRecorder
	wg         sync.WaitGroup
	stopOnce   sync.Once
	stopCh     chan struct{}
}

// eventWithContext pairs an event with its context for async processing.
type eventWithContext struct {
	ctx   context.Context
	event hooks.Event
}

// AsyncHandlerOption configures an AsyncHandler.
type AsyncHandlerOption func(*AsyncHandler)

// WithMetrics sets a metrics recorder for the async handler.
func WithMetrics(metrics hooks.MetricsRecorder) AsyncHandlerOption {
	return func(h *AsyncHandler) {
		h.metrics = metrics
	}
}

// NewAsyncHandler creates a new AsyncHandler that processes events asynchronously.
// The bufferSize determines how many events can be queued before blocking.
// Call Close() when done to wait for pending events and cleanup resources.
func NewAsyncHandler(handler hooks.EventHandler, bufferSize int, opts ...AsyncHandlerOption) *AsyncHandler {
	if bufferSize <= 0 {
		bufferSize = 100 // Default buffer size
	}

	h := &AsyncHandler{
		handler:    handler,
		eventQueue: make(chan eventWithContext, bufferSize),
		metrics:    hooks.NewNoOpMetricsRecorder(), // Default to no-op
		stopCh:     make(chan struct{}),
	}

	for _, opt := range opts {
		opt(h)
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
		// Record queue depth
		h.metrics.RecordGauge("bichat.async_handler.queue_depth",
			float64(len(h.eventQueue)),
			map[string]string{"event_type": event.Type()})
		return nil
	case <-h.stopCh:
		// Handler is stopped, drop the event
		h.metrics.IncrementCounter("bichat.async_handler.dropped_events", 1,
			map[string]string{
				"event_type": event.Type(),
				"reason":     "handler_stopped",
			})
		return nil
	default:
		// Queue is full, drop the event
		h.metrics.IncrementCounter("bichat.async_handler.dropped_events", 1,
			map[string]string{
				"event_type": event.Type(),
				"reason":     "queue_full",
			})
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
