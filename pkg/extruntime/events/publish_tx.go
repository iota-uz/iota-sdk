package events

import (
	"context"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// PublishInTx enqueues a native event in the transaction held by ctx
// (composables.InTx), so the event commits or rolls back with the change it
// reports. It is the publisher of the SDK modules' services.
//
// tenantID is the tenant of the changed entity; the nil UUID is an error, so a
// publication never lands in a wrong tenant and its transaction rolls back.
// The initiator is the acting user of ctx when that user belongs to the
// event's tenant (a superadmin of another tenant is no subject of it); a
// system or job context (history.IsJob) has none, even when it carries the
// user it is authorized as. A context without a transaction is an
// error: a published event must never outlive a failed change.
func PublishInTx[P any](ctx context.Context, tenantID uuid.UUID, ev *native.EventDef[P], subject *spi.ObjectRef, payload P) error {
	const op = serrors.Op("extruntime.events.PublishInTx")
	if tenantID == uuid.Nil {
		return serrors.New(serrors.Invalid, "event "+ev.ID()+" has no tenant").WithOp(op)
	}
	if ctx.Value(constants.TxKey) == nil {
		return serrors.New(serrors.Invalid, "event "+ev.ID()+" must be published in the transaction of the change").WithOp(op)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	var initiator int64
	if actor, actorErr := composables.UseUser(ctx); actorErr == nil && actor != nil && actor.TenantID() == tenantID && !history.IsJob(ctx) {
		initiator = int64(actor.ID())
	}
	_, err = Publish(ctx, tx, spi.Scope{Subject: spi.Subject{TenantID: tenantID, UserID: initiator, Kind: spi.SubjectInitiator}}, ev, subject, payload)
	return serrors.Wrap(op, err)
}
