package events

import (
	"context"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// PublishInTx enqueues a native event in the transaction held by ctx
// (composables.UseTx), so the event commits or rolls back with the change it
// reports; it is the publisher of a service that runs inside composables.InTx.
//
// The event belongs to tenantID, the tenant of the entity, not the tenant of
// the caller: an empty tenant is an error, and so is a ctx without a
// transaction, so the caller's change rolls back instead of committing without
// its event. The acting user of ctx, when there is one and the entity belongs
// to the user's tenant, is the event initiator; a change without an acting
// user (a gateway callback, a job) has no initiator, and a consumer that runs
// as the initiator then runs as the owner of its trigger.
func PublishInTx[P any](ctx context.Context, tenantID uuid.UUID, ev *native.EventDef[P], subject *spi.ObjectRef, payload P) error {
	const op = serrors.Op("extruntime.events.PublishInTx")
	if tenantID == uuid.Nil {
		return serrors.New(serrors.Invalid, "event "+ev.ID()+" has no tenant").WithOp(op)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	var initiator int64
	if actor, actorErr := composables.UseUser(ctx); actorErr == nil && actor != nil && actor.TenantID() == tenantID {
		initiator = int64(actor.ID())
	}
	_, err = Publish(ctx, tx, spi.Scope{Subject: spi.Subject{TenantID: tenantID, UserID: initiator, Kind: spi.SubjectInitiator}}, ev, subject, payload)
	return serrors.Wrap(op, err)
}
