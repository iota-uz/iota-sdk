package events

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SetLiveLimit lowers the number of live slots one pass enumerates.
func SetLiveLimit(s *Scheduler, n int) { s.liveLimit = n }

// SetRetryLimit lowers the number of due retries one pass handles.
func SetRetryLimit(s *Scheduler, n int) { s.retryLimit = n }

// CommitRejected exposes the commit-failure classification.
func CommitRejected(err error) bool { return commitRejected(err) }

// FailSlotOutside records a failed slot without its transaction, the way the
// scheduler does when the claim is already gone; claimed is the attempt count
// the claim returned.
func FailSlotOutside(s *Scheduler, ctx context.Context, def ScheduleDef, tenant uuid.UUID, slot time.Time, claimed int, generation *int64, runErr error) error {
	return s.failSlot(ctx, nil, def, tenant, slot, claimed, generation, "", runErr)
}

// SetCommit replaces how the scheduler finishes a slot transaction.
func SetCommit(s *Scheduler, commit func(context.Context, pgx.Tx) error) { s.commit = commit }

// ConnSlotPools counts the pools the shared limiter registry knows.
func ConnSlotPools() int {
	n := 0
	connSlots.Range(func(_, _ any) bool { n++; return true })
	return n
}

// AcquireConnSlot takes and releases a share of db's capacity.
func AcquireConnSlot(ctx context.Context, db *pgxpool.Pool) error {
	release, err := acquireConnSlot(ctx, db)
	if err != nil {
		return err
	}
	release()
	return nil
}
