package supervisor

import (
	"context"
	"errors"
)

var errBusy = errors.New("shell worker queue is full")

// queue admits one active run and a bounded number of waiting ones. A request
// reserves its waiting slot before its body is read, so at most size+1 bodies
// are held at once. A waiting caller that goes away leaves the queue
// immediately.
type queue struct {
	waiting chan struct{}
	active  chan struct{}
}

func newQueue(size int) *queue {
	return &queue{waiting: make(chan struct{}, size), active: make(chan struct{}, 1)}
}

func (q *queue) reserve() error {
	select {
	case q.waiting <- struct{}{}:
		return nil
	default:
		return errBusy
	}
}

func (q *queue) leave() { <-q.waiting }

// acquire waits for the active slot and gives up the reserved waiting slot.
func (q *queue) acquire(ctx context.Context) (func(), error) {
	defer q.leave()
	select {
	case q.active <- struct{}{}:
		return func() { <-q.active }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
