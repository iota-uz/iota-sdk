package services

import (
	"context"
	"math"
	"sort"

	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// NotificationOutboxConsumerID is the receipt key of the outbox → notification
// bridge. It must stay stable: a new ID redelivers the retained journal.
const NotificationOutboxConsumerID = "notifications"

// NotificationEnqueuer queues the rule-mode delivery of a notification event.
// Enqueue joins the transaction carried by ctx.
type NotificationEnqueuer interface {
	Enqueue(ctx context.Context, event notifications.Event) (int, error)
}

var _ NotificationEnqueuer = (*NotificationDispatchService)(nil)

// NewNotificationOutboxBridge is the Go consumer of the outbox that delivers
// event-backed notification definitions (notifications.FromEvent). Each
// committed event of such a definition is enqueued in the transaction of its
// delivery, so the notice and the dispatch job commit with the delivery
// receipt: a crash after the business commit only delays the notification, and
// a redelivery of the same outbox row never queues a second notice.
func NewNotificationOutboxBridge(catalog *notifications.Catalog, dispatch NotificationEnqueuer) events.Consumer {
	return &notificationOutboxBridge{catalog: catalog, dispatch: dispatch}
}

type notificationOutboxBridge struct {
	catalog  *notifications.Catalog
	dispatch NotificationEnqueuer
}

func (b *notificationOutboxBridge) ID() string { return NotificationOutboxConsumerID }

// Events lists the keys of the definitions built from an event; it is read on
// every dispatcher pass, so definitions registered later are picked up.
func (b *notificationOutboxBridge) Events(context.Context) ([]string, error) {
	var keys []string
	for _, d := range b.catalog.Definitions() {
		if d.Event != nil {
			keys = append(keys, d.Key)
		}
	}
	sort.Strings(keys)
	return keys, nil
}

// AfterSeq is 0: every retained event of a listed key is delivered once.
func (b *notificationOutboxBridge) AfterSeq(context.Context) (int64, error) { return 0, nil }

// Deliver enqueues the notification of one outbox event in tx (carried by ctx).
func (b *notificationOutboxBridge) Deliver(ctx context.Context, _ pgx.Tx, ev events.Envelope) error {
	const op serrors.Op = "NotificationOutboxBridge.Deliver"
	definition, ok := b.catalog.Get(ev.Event)
	if !ok || definition.Event == nil {
		return nil // the definition was removed: nothing to deliver
	}
	data, err := definition.DataOf(ev.Payload)
	if err != nil {
		return serrors.New(serrors.Invalid, err.Error()).WithOp(op)
	}
	event := notifications.Event{
		Key:        ev.Event,
		ID:         ev.ID,
		TenantID:   ev.TenantID,
		Payload:    ev.Payload,
		Data:       data,
		OccurredAt: ev.OccurredAt,
		Producer:   notifications.Producer{EventID: ev.ID},
	}
	if ev.InitiatorUserID != nil && *ev.InitiatorUserID > 0 && *ev.InitiatorUserID <= math.MaxUint32 {
		event.ActorUserID = uint(*ev.InitiatorUserID)
	}
	if ev.Subject != nil {
		event.Subject = notifications.SubjectReference{Type: ev.Subject.Entity, ID: ev.Subject.Key}
	}
	if _, err := b.dispatch.Enqueue(ctx, event); err != nil {
		return serrors.Wrap(op, err)
	}
	return nil
}
