package events

import (
	"context"
	"runtime"
	"sync"
	"weak"

	"github.com/jackc/pgx/v5/pgxpool"
)

// connSlots shares the pool's capacity among the transactions that need two of
// its connections at once: an outbox delivery and a schedule slot each hold a
// transaction while a package handler records its invocation on the pool. Half
// of the pool (at least one) can be in such a transaction at a time, so
// neither can hold the connection the other waits for, whatever the pool size
// (the runtime needs at least two).
//
// The registry holds the pool weakly and drops its entry when the pool is
// garbage collected, so replacing pools in a long-lived process leaks nothing.
var connSlots sync.Map // weak.Pointer[pgxpool.Pool] -> chan struct{}

// acquireConnSlot takes a share of the pool. The returned release is safe to
// call more than once: take it as soon as the transaction ends, before any
// callback that may call back into the scheduler or dispatcher.
func acquireConnSlot(ctx context.Context, db *pgxpool.Pool) (func(), error) {
	key := weak.Make(db)
	v, ok := connSlots.Load(key)
	if !ok {
		var loaded bool
		v, loaded = connSlots.LoadOrStore(key, make(chan struct{}, deliverySlots(1<<30, db.Config().MaxConns)))
		if !loaded {
			runtime.AddCleanup(db, func(k weak.Pointer[pgxpool.Pool]) { connSlots.Delete(k) }, key)
		}
	}
	slots := v.(chan struct{})
	select {
	case slots <- struct{}{}:
		var once sync.Once
		return func() { once.Do(func() { <-slots }) }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
