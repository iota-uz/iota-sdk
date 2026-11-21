package events

import (
	"context"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

// connSlots shares the pool's capacity among the transactions that need two of
// its connections at once: an outbox delivery and a schedule slot each hold a
// transaction while a package handler records its invocation on the pool. Half
// of the pool (at least one) can be in such a transaction at a time, so
// neither can hold the connection the other waits for, whatever the pool size
// (the runtime needs at least two).
var connSlots sync.Map // *pgxpool.Pool -> chan struct{}

func acquireConnSlot(ctx context.Context, db *pgxpool.Pool) (func(), error) {
	v, ok := connSlots.Load(db)
	if !ok {
		v, _ = connSlots.LoadOrStore(db, make(chan struct{}, deliverySlots(1<<30, db.Config().MaxConns)))
	}
	slots := v.(chan struct{})
	select {
	case slots <- struct{}{}:
		return func() { <-slots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
