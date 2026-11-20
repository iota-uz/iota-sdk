package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// Consumer is a Go consumer of the outbox. The dispatcher delivers every
// committed event it asks for exactly once: each delivery runs in one
// transaction with the consumer's receipt (subscription_receipts keyed by
// ID and the event), so a repeated pass, a second process or a restart never
// delivers an event twice.
type Consumer interface {
	// ID is the receipt key (subscription in subscription_receipts). It must
	// be stable across releases (a new ID redelivers the retained journal
	// after AfterSeq) and unique in the process.
	ID() string
	// Events are the event keys the consumer needs now. They are read again
	// on every pass, so the set may change at run time; a key added later
	// receives the events with seq > AfterSeq.
	Events(ctx context.Context) ([]string, error)
	// AfterSeq is the consumer's watermark: events with seq <= AfterSeq are
	// never delivered to it (see Watermark).
	AfterSeq(ctx context.Context) (int64, error)
	// Deliver handles one event in tx, the transaction of the receipt; tx
	// carries the event's tenant in app.tenant_id and the consumer must work
	// only in Envelope.TenantID. nil commits the receipt as done. An error
	// rolls the transaction back: the delivery is retried with backoff and
	// dead-lettered after MaxAttempts, except an error for which
	// host.IsInfrastructure is true (retried without counting an attempt)
	// and one that wraps ErrNoSubject (blocked: re-evaluated every
	// BlockedRecheck without counting an attempt). Use AfterCommit for work
	// that must follow the commit.
	Deliver(ctx context.Context, tx pgx.Tx, ev Envelope) error
}

// Envelope is one outbox event as a consumer receives it.
type Envelope struct {
	// ID is the event's UUID in the outbox.
	ID       string
	Seq      int64
	TenantID uuid.UUID
	// Event is the event key, for example "core.user.created.v1".
	Event   string
	Subject *spi.ObjectRef
	// Payload was validated against the event contract when it was published.
	Payload         json.RawMessage
	InitiatorUserID *int64
	OccurredAt      time.Time
}

// ConsumerSource lists the consumers of a pass. It is read on every pass, so
// a source may follow activations.
type ConsumerSource interface {
	Consumers(ctx context.Context) ([]Consumer, error)
}

// ConsumerSourceFunc adapts a function to ConsumerSource.
type ConsumerSourceFunc func(ctx context.Context) ([]Consumer, error)

func (f ConsumerSourceFunc) Consumers(ctx context.Context) ([]Consumer, error) { return f(ctx) }

// StaticConsumers is a ConsumerSource of a fixed list.
func StaticConsumers(list ...Consumer) ConsumerSource {
	list = append([]Consumer(nil), list...)
	return ConsumerSourceFunc(func(context.Context) ([]Consumer, error) {
		return append([]Consumer(nil), list...), nil
	})
}

// ComposeConsumers concatenates sources in order; nil sources are skipped.
func ComposeConsumers(srcs ...ConsumerSource) ConsumerSource {
	var list []ConsumerSource
	for _, s := range srcs {
		if s != nil {
			list = append(list, s)
		}
	}
	return ConsumerSourceFunc(func(ctx context.Context) ([]Consumer, error) {
		var out []Consumer
		for _, s := range list {
			cs, err := s.Consumers(ctx)
			if err != nil {
				return nil, err
			}
			out = append(out, cs...)
		}
		return out, nil
	})
}

type afterCommitKey struct{}

type afterCommits struct{ fns []func(context.Context) }

// AfterCommit registers fn to run after the delivery transaction of the
// current Consumer.Deliver commits (a wake-up, a cache refresh). It is
// dropped when the delivery rolls back. It reports false, and does not keep
// fn, when ctx is not the context of a delivery.
func AfterCommit(ctx context.Context, fn func(context.Context)) bool {
	ac, ok := ctx.Value(afterCommitKey{}).(*afterCommits)
	if !ok || fn == nil {
		return false
	}
	ac.fns = append(ac.fns, fn)
	return true
}
