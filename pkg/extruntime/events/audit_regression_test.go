package events_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host/hosttest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// outageOnItems makes inserts into host_items fail like an exhausted
// connection limit (SQLSTATE 53300) until off is called.
func (f *fixture) outageOnItems() func() {
	f.t.Helper()
	ctx := context.Background()
	for _, q := range []string{
		`CREATE TABLE outage_switch (x int)`,
		`INSERT INTO outage_switch VALUES (1)`,
		`CREATE FUNCTION outage_fn() RETURNS trigger AS $$
			BEGIN IF EXISTS (SELECT 1 FROM outage_switch) THEN RAISE EXCEPTION 'simulated outage' USING ERRCODE = '53300'; END IF; RETURN NEW; END $$ LANGUAGE plpgsql`,
		`CREATE TRIGGER outage BEFORE INSERT ON host_items FOR EACH ROW EXECUTE FUNCTION outage_fn()`,
	} {
		_, err := f.env.DB.Exec(ctx, q)
		require.NoError(f.t, err, q)
	}
	return func() {
		_, err := f.env.DB.Exec(ctx, `DELETE FROM outage_switch`)
		require.NoError(f.t, err)
	}
}

func TestInfrastructureFailureDoesNotCountTowardDeadLettering(t *testing.T) {
	f := newFixture(t)
	sub := f.subscribe("a", `export const handler = async (tx, e) => { await tx.insert(item, { label: "got:" + e.payload.name }); };`)
	f.subs.set(sub)
	require.NoError(t, f.emit(7, "x", false))
	off := f.outageOnItems()

	// Many more passes than MaxAttempts (3), each past the backoff.
	deadline := time.Now().Add(20 * time.Second)
	passes := 0
	for passes < 8 && time.Now().Before(deadline) {
		_, err := f.disp.RunOnce(context.Background())
		require.NoError(t, err)
		if r, ok := f.receipts(sub.ID)["x"]; ok {
			require.NotEqual(t, events.StatusDead, r.Status, "an outage must not dead-letter the event")
			passes++
		}
		time.Sleep(30 * time.Millisecond)
	}
	require.Equal(t, 8, passes, "the delivery was attempted repeatedly")
	r := f.receipts(sub.ID)["x"]
	require.Equal(t, events.StatusFailed, r.Status)
	require.Zero(t, r.Attempts, "infrastructure failures are not counted as attempts")
	require.Contains(t, r.LastError, "simulated outage")

	off()
	f.deliverUntil(20*time.Second, func() bool { return f.receipts(sub.ID)["x"].Status == events.StatusDone }, "delivered once the database is healthy")
	require.Contains(t, f.env.Items(), "got:x")
}

func TestInfrastructureFailureOfAScheduledRunDoesNotCountTowardDeadLettering(t *testing.T) {
	f := newFixture(t)
	f.env.Subjects.Grant(42)
	def := f.scheduleDef("outage", `export const handler = async (ctx) => {
  await ctx.transaction(async (tx) => { await tx.insert(item, { label: "ran" }); });
};`)
	_, err := f.env.DB.Exec(context.Background(), `INSERT INTO extruntime.trigger_owners (trigger_id, tenant_id, user_id) VALUES ($1, $2, 42)`, def.ID, f.env.Tenant)
	require.NoError(t, err)
	now := time.Date(2026, 6, 1, 10, 30, 20, 0, time.UTC)
	s := f.scheduler(&schedules{defs: []events.ScheduleDef{def}}, &now)
	off := f.outageOnItems()

	for i := 0; i < 8; i++ {
		_, err := s.RunOnce(context.Background())
		require.NoError(t, err)
		var status string
		require.NoError(t, f.env.DB.QueryRow(context.Background(), `SELECT status FROM extruntime.schedule_runs WHERE tenant_id = $1 AND schedule = $2`, f.env.Tenant, def.ID).Scan(&status))
		require.NotEqual(t, events.StatusDead, status)
		time.Sleep(30 * time.Millisecond)
	}
	var attempts int
	require.NoError(t, f.env.DB.QueryRow(context.Background(), `SELECT attempts FROM extruntime.schedule_runs WHERE tenant_id = $1 AND schedule = $2`, f.env.Tenant, def.ID).Scan(&attempts))
	require.Zero(t, attempts)

	off()
	require.Eventually(t, func() bool {
		_, err := s.RunOnce(context.Background())
		require.NoError(t, err)
		return len(f.allItems()) == 1
	}, 10*time.Second, 30*time.Millisecond)
}

func TestRequeueGivesDeadAndBlockedReceiptsAFreshStart(t *testing.T) {
	f := newFixture(t)
	bad := f.subscribe("a", `export const handler = async () => { throw new Error("poison pill"); };`)
	f.subs.set(bad)
	require.NoError(t, f.emit(7, "one", false))
	require.NoError(t, f.emit(7, "two", false))
	f.deliverUntil(20*time.Second, func() bool {
		r := f.receipts(bad.ID)
		return r["one"].Status == events.StatusDead && r["two"].Status == events.StatusDead
	}, "both dead-lettered")

	// The handler is fixed in a new version; the dead letters are still dead.
	fixed := f.subscribe("a2", `export const handler = async (tx, e) => { await tx.insert(item, { label: "got:" + e.payload.name }); };`, func(s *events.Subscription) { s.ID = bad.ID })
	f.subs.set(fixed)
	n, err := f.disp.RunOnce(context.Background())
	require.NoError(t, err)
	require.Zero(t, n, "dead receipts are not retried by themselves")

	ctx := context.Background()
	got, err := events.RequeueReceipts(ctx, f.env.DB, f.env.NewTenant(), bad.ID, "")
	require.NoError(t, err)
	require.Zero(t, got, "another tenant's receipts are untouched")

	var oneID string
	require.NoError(t, f.env.DB.QueryRow(ctx, `SELECT event_id::text FROM extruntime.subscription_receipts r JOIN extruntime.outbox_events e ON e.id = r.event_id WHERE e.payload->>'name' = 'one'`).Scan(&oneID))
	got, err = events.RequeueReceipts(ctx, f.env.DB, f.env.Tenant, bad.ID, oneID)
	require.NoError(t, err)
	require.EqualValues(t, 1, got, "only the named event was requeued")
	f.deliverUntil(20*time.Second, func() bool { return f.receipts(bad.ID)["one"].Status == events.StatusDone }, "requeued event delivered")
	require.Equal(t, events.StatusDead, f.receipts(bad.ID)["two"].Status)

	// A blocked delivery is requeued too; delivered ones never are.
	_, err = f.env.DB.Exec(ctx, `UPDATE extruntime.subscription_receipts SET status = 'blocked', next_at = now() + interval '1 hour' WHERE subscription = $1 AND event_id = $2::uuid`, bad.ID, oneID)
	require.NoError(t, err)
	got, err = events.RequeueReceipts(ctx, f.env.DB, f.env.Tenant, bad.ID, "")
	require.NoError(t, err)
	require.EqualValues(t, 2, got, "the dead and the blocked receipt")
	f.deliverUntil(20*time.Second, func() bool {
		r := f.receipts(bad.ID)
		return r["one"].Status == events.StatusDone && r["two"].Status == events.StatusDone
	}, "everything delivered")
	got, err = events.RequeueReceipts(ctx, f.env.DB, f.env.Tenant, bad.ID, "")
	require.NoError(t, err)
	require.Zero(t, got)

	_, err = events.RequeueReceipts(ctx, f.env.DB, f.env.Tenant, bad.ID, "not-a-uuid")
	require.Error(t, err)
	_, err = events.RequeueReceipts(ctx, f.env.DB, f.env.Tenant, "", "")
	require.Error(t, err)
}

func TestScheduleDoesNotFireSlotsBeforeItsActivation(t *testing.T) {
	f := newFixture(t)
	f.env.Subjects.Grant(42)
	def := f.scheduleDef("fresh", `export const handler = async (ctx) => {
  await ctx.transaction(async (tx) => { await tx.insert(item, { label: "slot:" + ctx.slot }); });
};`)
	def.ActiveFrom = time.Date(2026, 6, 1, 10, 29, 30, 0, time.UTC)
	_, err := f.env.DB.Exec(context.Background(), `INSERT INTO extruntime.trigger_owners (trigger_id, tenant_id, user_id) VALUES ($1, $2, 42)`, def.ID, f.env.Tenant)
	require.NoError(t, err)
	now := time.Date(2026, 6, 1, 10, 30, 20, 0, time.UTC)
	s := f.scheduler(&schedules{defs: []events.ScheduleDef{def}}, &now, func(c *events.SchedulerConfig) { c.Lookback = 5 * time.Minute })
	n, err := s.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, []string{"slot:2026-06-01T10:30:00Z"}, f.allItems(), "the 10:26-10:29 slots precede the activation")
}

// A subscription handler's transaction is committed by the dispatcher with
// the receipt. A tx.call the handler did not await must not slip into that
// commit half-done: the delivery fails and rolls back.
func TestUnawaitedTxCallInASubscriptionHandlerRollsTheDeliveryBack(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.env.Artifacts.Add(t, "acme.ops", "1.0.0", prelude+`
export const slow = async (tx) => {
  await tx.insert(item, { label: "op-1" });
  await new Promise((resolve) => setTimeout(resolve, 400));
  await tx.insert(item, { label: "op-2" });
  return null;
};`)
	f.env.Catalog.Add(spi.ResolvedResource{Package: "acme.ops", Version: "1.0.0", Resource: hosttest.Operation("acme.test.slow", nil, nil)})
	_, err := f.env.Bindings.Activate(ctx, "acme.test.slow", "acme.ops", "1.0.0", "slow", nil)
	require.NoError(t, err)
	sub := f.subscribe("a", `export const handler = async (tx, e) => {
  await tx.insert(item, { label: "handler:" + e.payload.name });
  void tx.call({ id: "acme.test.slow" }, null);
  await new Promise((resolve) => setTimeout(resolve, 150)); // the call is under way when the handler returns
};`)
	f.subs.set(sub)
	require.NoError(t, f.emit(7, "x", false))

	f.deliverUntil(20*time.Second, func() bool { return f.receipts(sub.ID)["x"].Status == events.StatusFailed }, "the delivery failed")
	require.Contains(t, f.receipts(sub.ID)["x"].LastError, "runtime.nestedInFlight")
	require.Equal(t, []string{"row:x"}, f.env.Items(), "neither the handler's write nor the first write of the un-awaited call committed")
	time.Sleep(600 * time.Millisecond) // the abandoned call finishes in its worker before the next test starts
}
