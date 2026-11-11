package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	opPublish = serrors.Op("extruntime.events.Publish")
	opEmit    = serrors.Op("extruntime.events.Outbox.Emit")

	// NotifyChannel wakes dispatchers after a commit that published events.
	NotifyChannel = "extruntime_events"
)

// Execer is the part of a transaction Publish needs; pgx.Tx and repo.Tx
// satisfy it. The row commits or rolls back with the caller's transaction.
type Execer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// Publish enqueues an event in the caller's transaction. It is the Go API of
// native modules: the event must be registered in the Default native catalog
// (RegisterNative) and the payload must match its versioned payload type.
// The scope's subject is the event initiator when it carries a user.
func Publish(ctx context.Context, tx Execer, sc spi.Scope, in spi.EventIntent) (string, error) {
	res, ok := Default.Lookup(in.Event)
	if !ok {
		return "", serrors.Wrap(opPublish, fmt.Errorf("event %q is not registered in the native event catalog", in.Event))
	}
	if err := checkEvent(res, in); err != nil {
		return "", serrors.Wrap(opPublish, err)
	}
	id, err := insertEvent(ctx, tx, sc, in)
	if err != nil {
		return "", serrors.Wrap(opPublish, err)
	}
	return id, nil
}

func checkEvent(res manifest.Resource, in spi.EventIntent) error {
	if res.Kind != manifest.ResEvent {
		return fmt.Errorf("%s is a %s, not an event", res.ID, res.Kind)
	}
	if res.Payload != nil {
		if err := manifest.ValidateValue(*res.Payload, in.Payload); err != nil {
			return fmt.Errorf("payload of %s: %w", res.ID, err)
		}
	}
	switch {
	case res.Subject != "" && in.Subject == nil:
		return fmt.Errorf("%s needs a subject of %s", res.ID, res.Subject)
	case res.Subject == "" && in.Subject != nil:
		return fmt.Errorf("%s has no subject", res.ID)
	case res.Subject != "" && in.Subject.Entity != res.Subject:
		return fmt.Errorf("%s subject must be a %s ref, got %s", res.ID, res.Subject, in.Subject.Entity)
	}
	return nil
}

func insertEvent(ctx context.Context, tx Execer, sc spi.Scope, in spi.EventIntent) (string, error) {
	if sc.Subject.TenantID == uuid.Nil {
		return "", errors.New("event scope has no tenant")
	}
	payload := in.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`null`)
	}
	var subjectEntity, subjectKey any
	if in.Subject != nil {
		subjectEntity, subjectKey = in.Subject.Entity, in.Subject.Key
	}
	var initiator any
	if sc.Subject.UserID > 0 {
		initiator = sc.Subject.UserID
	}
	var producer any
	if in.Producer.InvocationID != "" {
		producer = in.Producer.InvocationID
	}
	id := uuid.NewString()
	if _, err := tx.Exec(ctx, `
		INSERT INTO extruntime.outbox_events
			(id, tenant_id, event, subject_entity, subject_key, payload, initiator_user_id, producer_invocation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8::uuid)`,
		id, sc.Subject.TenantID, in.Event, subjectEntity, subjectKey, []byte(payload), initiator, producer); err != nil {
		return "", err
	}
	// Delivered to listeners only when this transaction commits.
	if _, err := tx.Exec(ctx, `SELECT pg_notify($1, '')`, NotifyChannel); err != nil {
		return "", err
	}
	return id, nil
}

// Outbox implements spi.Outbox for package events (`tx.emit`): the event must
// be declared in the catalog and the payload and subject must match.
type Outbox struct {
	catalog spi.Catalog
}

// NewOutbox builds the outbox over the catalog that resolves package and
// native events.
func NewOutbox(catalog spi.Catalog) *Outbox { return &Outbox{catalog: catalog} }

var _ spi.Outbox = (*Outbox)(nil)

// Emit inserts the event in the caller's transaction with the producing
// invocation, so the event commits atomically with the data.
func (o *Outbox) Emit(ctx context.Context, tx pgx.Tx, sc spi.Scope, e spi.EventIntent) (string, error) {
	res, ok, err := o.catalog.Resolve(ctx, e.Event)
	if err != nil {
		return "", serrors.Wrap(opEmit, err)
	}
	if !ok {
		return "", &pool.WireError{Kind: pool.ErrKindInternal, Code: "events.unknownEvent", Message: "event " + e.Event + " is not declared in the catalog"}
	}
	if err := checkEvent(res.Resource, e); err != nil {
		return "", &pool.WireError{Kind: pool.ErrKindInternal, Code: "events.invalidEvent", Message: err.Error()}
	}
	id, err := insertEvent(ctx, tx, sc, e)
	if err != nil {
		return "", serrors.Wrap(opEmit, err)
	}
	return id, nil
}

// Watermark returns the sequence position to store as a subscription's
// AfterSeq at activation: events published up to now are never delivered to
// it, later ones always are, regardless of commit order.
func Watermark(ctx context.Context, q interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}) (int64, error) {
	var seq int64
	err := q.QueryRow(ctx, `SELECT CASE WHEN is_called THEN last_value ELSE 0 END FROM extruntime.outbox_events_seq_seq`).Scan(&seq)
	return seq, serrors.FromDB(serrors.Op("extruntime.events.Watermark"), err)
}
