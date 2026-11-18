package events_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

const gotHandler = `export const handler = async (tx, e) => { await tx.insert(item, { label: "got:" + e.payload.name + ":" + tx.user.userId + ":" + tx.user.kind }); };`

func TestRolledBackEmitIsNeverDeliveredAndCommittedEventRunsEachSubscriptionOnce(t *testing.T) {
	f := newFixture(t)
	a := f.subscribe("a", gotHandler)
	b := f.subscribe("b", `export const handler = async (tx, e) => { await tx.insert(item, { label: "b:" + e.payload.name }); };`)
	f.subs.set(a, b)

	require.Error(t, f.emit(7, "lost", true))
	require.Zero(t, f.outboxCount(), "the outbox row rolled back with the emitting transaction")
	n, err := f.disp.RunOnce(context.Background())
	require.NoError(t, err)
	require.Zero(t, n)
	require.Empty(t, f.env.Items())

	require.NoError(t, f.emit(7, "kept", false))
	f.deliverUntil(20*time.Second, func() bool { return len(f.env.Items()) == 3 }, "both subscriptions ran")
	require.ElementsMatch(t, []string{"row:kept", "got:kept:7:initiator", "b:kept"}, f.env.Items(), "each subscription ran once, as the event initiator")

	// Redelivery after commit (a restarted dispatcher, a second node) is a no-op.
	other, err := events.NewDispatcher(events.Config{DB: f.env.DB, Runner: f.host, Subscriptions: f.subs, Subjects: f.env.Subjects, Logger: f.env.Logger})
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		n, err := other.RunOnce(context.Background())
		require.NoError(t, err)
		require.Zero(t, n)
	}
	require.Len(t, f.env.Items(), 3)
	for _, sub := range []string{a.ID, b.ID} {
		r := f.receipts(sub)["kept"]
		require.Equal(t, events.StatusDone, r.Status)
		require.NotNil(t, r.InvocationID, "the receipt points at the subscription invocation")
	}
}

func TestSubscriptionsAreIndependentAndAFailingOneDoesNotHoldBackTheOther(t *testing.T) {
	f := newFixture(t)
	good := f.subscribe("good", gotHandler)
	bad := f.subscribe("bad", `export const handler = async () => { throw new Error("always fails"); };`)
	f.subs.set(bad, good)
	require.NoError(t, f.emit(7, "x", false))
	f.deliverUntil(20*time.Second, func() bool { return len(f.env.Items()) == 2 }, "good subscription delivered")
	require.Equal(t, events.StatusFailed, f.receipts(bad.ID)["x"].Status)
	require.Equal(t, events.StatusDone, f.receipts(good.ID)["x"].Status)
}

func TestOutOfOrderCommitIsDeliveredWithoutWaitingForOpenTransactions(t *testing.T) {
	f := newFixture(t)
	sub := f.subscribe("a", `export const handler = async (tx, e) => { await tx.insert(item, { label: e.payload.name }); };`)
	f.subs.set(sub)
	ctx := context.Background()
	scope := spi.Scope{Subject: f.env.Subject(7)}

	txA, err := f.env.DB.Begin(ctx)
	require.NoError(t, err)
	_, err = f.outbox.Emit(ctx, txA, scope, spi.EventIntent{Event: thingEvent, Payload: []byte(`{"name":"A"}`)})
	require.NoError(t, err) // A takes the first seq and stays open
	txB, err := f.env.DB.Begin(ctx)
	require.NoError(t, err)
	_, err = f.outbox.Emit(ctx, txB, scope, spi.EventIntent{Event: thingEvent, Payload: []byte(`{"name":"B"}`)})
	require.NoError(t, err)
	require.NoError(t, txB.Commit(ctx)) // B commits first

	f.deliverUntil(20*time.Second, func() bool { return len(f.env.Items()) == 1 }, "B delivered while A is still open")
	require.Equal(t, []string{"B"}, f.env.Items(), "an open transaction elsewhere does not hold back committed events")

	require.NoError(t, txA.Commit(ctx))
	f.deliverUntil(20*time.Second, func() bool { return len(f.env.Items()) == 2 }, "A delivered after it commits")
	require.ElementsMatch(t, []string{"A", "B"}, f.env.Items(), "the lower seq that committed late is not skipped")
}

func TestPoisonedEventGoesDeadWithoutBlockingLaterEvents(t *testing.T) {
	f := newFixture(t)
	sub := f.subscribe("a", `export const handler = async (tx, e) => {
  if (e.payload.name === "bad") throw new Error("poison pill");
  await tx.insert(item, { label: e.payload.name });
};`)
	f.subs.set(sub)
	require.NoError(t, f.emit(7, "bad", false))
	require.NoError(t, f.emit(7, "good", false))

	f.deliverUntil(20*time.Second, func() bool {
		r := f.receipts(sub.ID)
		return r["bad"].Status == events.StatusDead && r["good"].Status == events.StatusDone
	}, "bad is dead-lettered, good delivered")
	r := f.receipts(sub.ID)
	require.Equal(t, 3, r["bad"].Attempts)
	require.Contains(t, r["bad"].LastError, "poison pill")
	require.Equal(t, []string{"row:bad", "row:good", "good"}, f.env.Items(), "the later event was delivered on the first pass, the poisoned one did not block it")
	n, err := f.disp.RunOnce(context.Background())
	require.NoError(t, err)
	require.Zero(t, n, "a dead event is not retried")
}

func TestEventWithoutInitiatorRunsAsTheTriggerOwnerAndMissingOwnerBlocksWithADiagnostic(t *testing.T) {
	f := newFixture(t)
	type named struct {
		Name string `json:"name"`
	}
	ev := native.Event[named]("acme.test.native.v1", native.EventSpec{})
	f.env.Catalog.Add(spi.ResolvedResource{Native: true, Resource: ev.Resource()})
	sub := f.subscribe("owner", `export const handler = async (tx, e) => { await tx.insert(item, { label: "owner-run:" + tx.user.userId + ":" + tx.user.kind }); };`,
		func(s *events.Subscription) { s.Event = "acme.test.native.v1" })
	f.subs.set(sub)

	publish := func() {
		tx, err := f.env.DB.Begin(context.Background())
		require.NoError(t, err)
		_, err = events.Publish(context.Background(), tx, spi.Scope{Subject: spi.Subject{TenantID: f.env.Tenant}}, ev, nil, named{Name: "sys"})
		require.NoError(t, err)
		require.NoError(t, tx.Commit(context.Background()))
	}
	publish()
	require.Eventually(t, func() bool {
		_, err := f.disp.RunOnce(context.Background())
		require.NoError(t, err)
		return f.receipts(sub.ID)["sys"].Status == events.StatusBlocked
	}, 20*time.Second, 30*time.Millisecond)
	r := f.receipts(sub.ID)["sys"]
	require.Contains(t, r.LastError, "no owner-user")
	require.Empty(t, f.env.Items())

	f.env.Subjects.Grant(42)
	_, err := f.env.DB.Exec(context.Background(), `INSERT INTO extruntime.trigger_owners (trigger_id, tenant_id, user_id) VALUES ($1, $2, 42)`, sub.ID, f.env.Tenant)
	require.NoError(t, err)
	f.deliverUntil(20*time.Second, func() bool { return len(f.env.Items()) == 1 }, "delivered once an owner is assigned")
	require.Equal(t, []string{"owner-run:42:owner"}, f.env.Items())
	require.Equal(t, events.StatusDone, f.receipts(sub.ID)["sys"].Status)
}

func TestInitiatorWithDisabledAccountBlocksDeliveryAndCurrentRightsAreUsed(t *testing.T) {
	f := newFixture(t)
	sub := f.subscribe("rights", `export const handler = async (tx) => { await tx.insert(item, { label: "can:" + tx.user.has("later.granted") }); };`)
	f.subs.set(sub)
	require.NoError(t, f.emit(7, "x", false))
	f.env.Subjects.Block(7, "account disabled")
	require.Eventually(t, func() bool {
		_, err := f.disp.RunOnce(context.Background())
		require.NoError(t, err)
		return f.receipts(sub.ID)["x"].Status == events.StatusBlocked
	}, 20*time.Second, 30*time.Millisecond)
	require.Contains(t, f.receipts(sub.ID)["x"].LastError, "account disabled")

	f.env.Subjects.Grant(7, "later.granted") // rights changed after the event was emitted
	f.deliverUntil(20*time.Second, func() bool { return len(f.env.Items()) == 2 }, "delivered after the account is enabled")
	require.Contains(t, f.env.Items(), "can:true", "the handler ran with the initiator's rights as they are now")
}

func TestActivationWatermarkExcludesOlderEventsOnly(t *testing.T) {
	f := newFixture(t)
	require.NoError(t, f.emit(7, "before", false))
	mark, err := events.Watermark(context.Background(), f.env.DB)
	require.NoError(t, err)
	require.NoError(t, f.emit(7, "after", false))
	sub := f.subscribe("late", `export const handler = async (tx, e) => { await tx.insert(item, { label: "late:" + e.payload.name }); };`,
		func(s *events.Subscription) { s.AfterSeq = mark })
	f.subs.set(sub)
	f.deliverUntil(20*time.Second, func() bool { return f.receipts(sub.ID)["after"].Status == events.StatusDone }, "post-activation event delivered")
	require.NotContains(t, f.env.Items(), "late:before", "no automatic replay of the journal on activation")
	require.Contains(t, f.env.Items(), "late:after")
}

func TestListenNotifyWakesTheDispatcherBeforeThePollFallback(t *testing.T) {
	f := newFixture(t, func(c *events.Config) { c.PollInterval = 30 * time.Second })
	sub := f.subscribe("a", gotHandler)
	f.subs.set(sub)
	f.disp.Start(context.Background())
	time.Sleep(300 * time.Millisecond) // LISTEN is established and the first pass is done
	require.NoError(t, f.emit(7, "wake", false))
	f.env.Eventually(20*time.Second, func() bool { return f.receipts(sub.ID)["wake"].Status == events.StatusDone }, "NOTIFY wakes the dispatcher")
}

func TestPruneDeletesOnlyDeliveredOldEvents(t *testing.T) {
	f := newFixture(t, func(c *events.Config) { c.Retention = time.Millisecond })
	sub := f.subscribe("a", gotHandler)
	f.subs.set(sub)
	require.NoError(t, f.emit(7, "done", false))
	f.deliverUntil(20*time.Second, func() bool { return f.receipts(sub.ID)["done"].Status == events.StatusDone }, "delivered")
	mark, err := events.Watermark(context.Background(), f.env.DB)
	require.NoError(t, err)
	f.subs.set(f.subscribe("bad", `export const handler = async () => { throw new Error("x"); };`, func(s *events.Subscription) { s.AfterSeq = mark }))
	require.NoError(t, f.emit(7, "pending", false))
	f.deliverUntil(20*time.Second, func() bool { return f.receipts("acme.sub.bad")["pending"].Status == events.StatusFailed }, "failed once")

	time.Sleep(20 * time.Millisecond)
	var removed int64
	require.Eventually(t, func() bool {
		n, err := f.disp.Prune(context.Background())
		require.NoError(t, err)
		removed += n
		return removed >= 1
	}, 20*time.Second, 50*time.Millisecond)
	require.Equal(t, 1, f.outboxCount(), "the event with a pending delivery is kept")
}

func TestOutboxAndPublishValidateEventsAndPayloads(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	scope := spi.Scope{Subject: f.env.Subject(7)}
	tx, err := f.env.DB.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = f.outbox.Emit(ctx, tx, scope, spi.EventIntent{Event: "acme.test.undeclared.v1", Payload: []byte(`{}`)})
	require.ErrorContains(t, err, "not declared")
	sp, err := tx.Begin(ctx) // savepoint: a failed validation must not poison the transaction
	require.NoError(t, err)
	_, err = f.outbox.Emit(ctx, sp, scope, spi.EventIntent{Event: thingEvent, Payload: []byte(`{"name":5}`)})
	require.ErrorContains(t, err, "payload")
	require.NoError(t, sp.Rollback(ctx))
	id, err := f.outbox.Emit(ctx, tx, scope, spi.EventIntent{Event: thingEvent, Payload: []byte(`{"name":"ok"}`), Producer: spi.Producer{InvocationID: uuid.NewString()}})
	require.NoError(t, err)
	require.NotEmpty(t, id)

	type thing struct {
		Name string `json:"name"`
	}
	declared := native.Event[thing]("core.thing.created.v1", native.EventSpec{Subject: "core.user"})
	_, err = events.Publish(ctx, tx, scope, declared, nil, thing{Name: "x"})
	require.ErrorContains(t, err, "needs a subject of core.user")
	_, err = events.Publish(ctx, tx, scope, declared, &spi.ObjectRef{Entity: "crm.client", Key: "1"}, thing{Name: "x"})
	require.ErrorContains(t, err, "subject must be a core.user ref")
	id, err = events.Publish(ctx, tx, scope, declared, &spi.ObjectRef{Entity: "core.user", Key: "1"}, thing{Name: "x"})
	require.NoError(t, err)
	require.NotEmpty(t, id)
	_, err = native.NewEvent[thing]("core.noversion", native.EventSpec{})
	require.Error(t, err, "a native event ID ends in a version")
	res := declared.Resource()
	require.Equal(t, manifest.ResEvent, res.Kind)
	require.Equal(t, manifest.ContractHashOf(res), res.ContractHash)
}

var _ pgx.Tx
