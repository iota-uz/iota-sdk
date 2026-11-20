package events

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Subscription is one active subscription of an activated package version.
type Subscription struct {
	ID       string
	Package  string
	Version  string
	Export   string
	Event    string
	Artifact pool.Artifact
	Secrets  []string
	// AfterSeq is the activation watermark (see Watermark): events with a
	// lower or equal sequence number are never delivered to this
	// subscription. A handler update does not replay the journal.
	AfterSeq int64
}

// SubscriptionSource lists the active subscriptions; the management slice
// implements it over activations.
type SubscriptionSource interface {
	ActiveSubscriptions(ctx context.Context) ([]Subscription, error)
}

// Runner executes an export; *host.Host implements it.
type Runner interface {
	Run(ctx context.Context, req host.RunRequest) (host.Result, error)
}

// PackageConsumers adapts the active package subscriptions to consumers: each
// subscription is a consumer of its one event whose receipt key is the
// subscription ID. The handler acts as the event initiator with current
// rights, else as the subscription's owner in the tenant; without either the
// delivery is blocked. The handler runs in the delivery transaction, and the
// receipt records its invocation.
func PackageConsumers(subs SubscriptionSource, runner Runner, subjects host.SubjectLoader) ConsumerSource {
	return ConsumerSourceFunc(func(ctx context.Context) ([]Consumer, error) {
		list, err := subs.ActiveSubscriptions(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]Consumer, 0, len(list))
		for _, s := range list {
			out = append(out, &packageConsumer{sub: s, runner: runner, subjects: subjects})
		}
		return out, nil
	})
}

type packageConsumer struct {
	sub      Subscription
	runner   Runner
	subjects host.SubjectLoader
}

func (c *packageConsumer) ID() string { return c.sub.ID }
func (c *packageConsumer) Events(context.Context) ([]string, error) {
	return []string{c.sub.Event}, nil
}
func (c *packageConsumer) AfterSeq(context.Context) (int64, error) { return c.sub.AfterSeq, nil }

func (c *packageConsumer) Deliver(ctx context.Context, tx pgx.Tx, ev Envelope) error {
	const op = serrors.Op("extruntime.events.packageConsumer.Deliver")
	subject, err := ResolveSubject(ctx, tx, c.subjects, ev, c.sub.ID, RunAsInitiator)
	if err != nil {
		return err
	}
	input, err := packageInput(ev)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	sub := c.sub
	res, runErr := c.runner.Run(ctx, host.RunRequest{
		Export: host.Export{
			Kind:     pool.KindSubscription,
			Resource: sub.ID,
			Binding: host.Binding{
				Contract: sub.ID, Package: sub.Package, Version: sub.Version, Export: sub.Export,
				Artifact: sub.Artifact, Secrets: sub.Secrets,
			},
		},
		Input:        input,
		Subject:      subject,
		Trigger:      host.Trigger{Kind: host.TriggerSubscription, Ref: sub.ID},
		Tx:           tx,
		Producer:     spi.Producer{EventID: ev.ID},
		StartKeyBase: sub.ID + ":" + ev.ID,
	})
	if runErr != nil {
		if res.InvocationID != "" {
			return &invocationError{err: runErr, invocationID: res.InvocationID}
		}
		return runErr
	}
	if res.InvocationID != "" {
		if _, err := tx.Exec(ctx, `UPDATE extruntime.subscription_receipts SET invocation_id = $3::uuid WHERE subscription = $1 AND event_id = $2::uuid`,
			sub.ID, ev.ID, res.InvocationID); err != nil {
			return serrors.FromDB(op, err)
		}
	}
	if res.AfterCommit != nil {
		AfterCommit(ctx, res.AfterCommit)
	}
	return nil
}

// invocationError carries the invocation of a failed handler run to the
// failure receipt.
type invocationError struct {
	err          error
	invocationID string
}

func (e *invocationError) Error() string { return e.err.Error() }
func (e *invocationError) Unwrap() error { return e.err }

// packageInput is the event object a package handler receives.
func packageInput(ev Envelope) (json.RawMessage, error) {
	var initiator *string
	if ev.InitiatorUserID != nil {
		s := strconv.FormatInt(*ev.InitiatorUserID, 10)
		initiator = &s
	}
	payload := ev.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`null`)
	}
	return json.Marshal(struct {
		ID         string          `json:"id"`
		Event      string          `json:"event"`
		OccurredAt string          `json:"occurredAt"`
		Subject    *spi.ObjectRef  `json:"subject"`
		Payload    json.RawMessage `json:"payload"`
		Initiator  *string         `json:"initiator"`
	}{ev.ID, ev.Event, ev.OccurredAt.UTC().Format(time.RFC3339Nano), ev.Subject, payload, initiator})
}
