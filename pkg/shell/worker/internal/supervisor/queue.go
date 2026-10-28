package supervisor

import (
	"context"
	"errors"
)

var errBusy = errors.New("shell worker queue is full")

// queue admits one active run and a bounded number of waiting ones. A waiting
// caller that goes away leaves the queue immediately.
type queue struct {
	waiting chan struct{}
	active  chan struct{}
}

func newQueue(size int) *queue {
	return &queue{waiting: make(chan struct{}, size), active: make(chan struct{}, 1)}
}

func (q *queue) acquire(ctx context.Context) (func(), error) {
	select {
	case q.waiting <- struct{}{}:
	default:
		return nil, errBusy
	}
	defer func() { <-q.waiting }()
	select {
	case q.active <- struct{}{}:
		return func() { <-q.active }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
