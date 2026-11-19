package composition

import (
	"context"
	"errors"
	"sync"
)

// ErrResourceClosing is returned by OwnedResource.Acquire once Close has begun.
var ErrResourceClosing = errors.New("composition: resource is closing")

// ErrResourceInUse is returned by OwnedResource.Close when its context ends
// while holders still use the resource. The close is not abandoned: it runs as
// soon as the last holder releases its lease.
var ErrResourceInUse = errors.New("composition: resource still in use; close deferred until released")

// OwnedResource guards a shared resource (a Redis client, a broker
// connection) that background work borrows. Close never tears the resource
// down underneath an active holder: it waits for leases until its context
// ends and otherwise defers the close to the last Release. This keeps a
// bounded shutdown bounded without closing a client that an in-flight job is
// still using for its terminal bookkeeping.
type OwnedResource struct {
	mu       sync.Mutex
	close    func() error
	holders  int
	closing  bool
	closed   bool
	done     chan struct{}
	closeErr error
}

// NewOwnedResource wraps closeFn. A nil closeFn makes Close a no-op.
func NewOwnedResource(closeFn func() error) *OwnedResource {
	return &OwnedResource{close: closeFn, done: make(chan struct{})}
}

// Acquire registers a holder. The returned release is idempotent and must be
// called when the holder no longer uses the resource.
func (r *OwnedResource) Acquire() (func(), error) {
	if r == nil {
		return func() {}, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closing {
		return nil, ErrResourceClosing
	}
	r.holders++
	var once sync.Once
	return func() { once.Do(r.release) }, nil
}

func (r *OwnedResource) release() {
	r.mu.Lock()
	r.holders--
	last := r.holders == 0 && r.closing
	r.mu.Unlock()
	if last {
		r.finish()
	}
}

func (r *OwnedResource) finish() {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.closed = true
	closeFn := r.close
	r.mu.Unlock()
	var err error
	if closeFn != nil {
		err = closeFn()
	}
	r.mu.Lock()
	r.closeErr = err
	r.mu.Unlock()
	close(r.done)
}

// Close refuses new holders and closes the resource once every lease is
// released. It waits for that until ctx ends; on expiry it returns
// ErrResourceInUse (joined with ctx's error) and the last Release performs the
// close. Close is idempotent; later calls wait for the same close.
func (r *OwnedResource) Close(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	first := !r.closing
	r.closing = true
	idle := r.holders == 0
	r.mu.Unlock()
	if first && idle {
		r.finish()
	}
	select {
	case <-r.done:
		return r.result()
	default:
	}
	select {
	case <-r.done:
		return r.result()
	case <-ctx.Done():
		return errors.Join(ErrResourceInUse, ctx.Err())
	}
}

func (r *OwnedResource) result() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closeErr
}

// Done is closed after the underlying resource has been closed.
func (r *OwnedResource) Done() <-chan struct{} {
	if r == nil {
		closed := make(chan struct{})
		close(closed)
		return closed
	}
	return r.done
}
