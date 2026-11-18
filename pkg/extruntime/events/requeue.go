package events

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const opRequeue = serrors.Op("extruntime.events.RequeueReceipts")

// RequeueReceipts gives the dead-lettered and blocked deliveries of one
// subscription in a tenant a fresh start: they become `failed` with no
// attempts counted and are due now, so the dispatcher delivers them on its
// next pass (a blocked one is re-evaluated: it blocks again if its cause
// remains). eventID, when set, narrows the requeue to one event. It returns
// how many receipts were requeued. Delivered receipts are never touched.
func RequeueReceipts(ctx context.Context, q Execer, tenant uuid.UUID, subscription, eventID string) (int64, error) {
	if subscription == "" {
		return 0, serrors.New(serrors.Invalid, "subscription is required").WithOp(opRequeue)
	}
	var event any
	if eventID != "" {
		id, err := uuid.Parse(eventID)
		if err != nil {
			return 0, serrors.New(serrors.Invalid, fmt.Sprintf("event %q is not an event id", eventID)).WithOp(opRequeue)
		}
		event = id
	}
	tag, err := q.Exec(ctx, `
		UPDATE extruntime.subscription_receipts
		SET status = 'failed', attempts = 0, next_at = now(), updated_at = now()
		WHERE tenant_id = $1 AND subscription = $2 AND status IN ('dead', 'blocked') AND ($3::uuid IS NULL OR event_id = $3::uuid)`,
		tenant, subscription, event)
	if err != nil {
		return 0, serrors.FromDB(opRequeue, err)
	}
	if tag.RowsAffected() > 0 {
		if _, err := q.Exec(ctx, `SELECT pg_notify($1, '')`, NotifyChannel); err != nil {
			return 0, serrors.FromDB(opRequeue, err)
		}
	}
	return tag.RowsAffected(), nil
}
