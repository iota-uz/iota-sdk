// Package memory implements single-instance realtime transport.
package memory

import (
	"context"
	"errors"
	"sync"

	"github.com/iota-uz/iota-sdk/pkg/realtime"
)

type Backend struct {
	mu       sync.RWMutex
	handlers map[uint]realtime.Handler
	next     uint
	closed   bool
	done     chan struct{}
}

var _ realtime.Backend = (*Backend)(nil)

func New() *Backend {
	return &Backend{handlers: make(map[uint]realtime.Handler), done: make(chan struct{})}
}
func (b *Backend) Publish(ctx context.Context, e realtime.Envelope) error {
	if err := e.Validate(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return errors.New("realtime backend closed")
	}
	handlers := make([]realtime.Handler, 0, len(b.handlers))
	for _, h := range b.handlers {
		handlers = append(handlers, h)
	}
	b.mu.RUnlock()
	for _, h := range handlers {
		if err := h(ctx, e); err != nil {
			return err
		}
	}
	return nil
}
func (b *Backend) Subscribe(ctx context.Context, h realtime.Handler) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return errors.New("realtime backend closed")
	}
	b.next++
	id := b.next
	b.handlers[id] = h
	go func() {
		select {
		case <-ctx.Done():
		case <-b.done:
		}
		b.mu.Lock()
		delete(b.handlers, id)
		b.mu.Unlock()
	}()
	return nil
}
func (b *Backend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.closed {
		close(b.done)
	}
	b.closed = true
	clear(b.handlers)
	return nil
}
