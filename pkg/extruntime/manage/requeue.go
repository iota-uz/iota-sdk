package manage

import (
	"context"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// ActionEventsRequeue is the audit action of RequeueReceipts.
const ActionEventsRequeue = "events.requeue"

// RequeueReceipts gives the dead-lettered and blocked deliveries of a
// subscription in the actor's tenant a fresh start (all of them, or the one
// event when eventID is set) and returns how many were requeued. The
// dispatcher delivers them on its next pass, with the subscription's current
// handler. Requires the operate permission; audited.
func (s *Service) RequeueReceipts(ctx context.Context, actor spi.Subject, subscription, eventID string) (int64, error) {
	e := &auditEntry{Action: ActionEventsRequeue, Target: subscription, Detail: map[string]any{"event": eventID}}
	var n int64
	err := s.mutate(ctx, actor, PermOperate, e, func(tx pgx.Tx) error {
		got, err := events.RequeueReceipts(ctx, tx, actor.TenantID, subscription, eventID)
		n = got
		e.Detail["requeued"] = got
		return err
	})
	return n, err
}
