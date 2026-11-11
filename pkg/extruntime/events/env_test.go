package events_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host/hosttest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

const (
	thingEvent = "acme.test.thing.v1"
	prelude    = `const item = { id: "acme.test.item" };
const Thing = { id: "acme.test.thing.v1" };
`
)

// subs is the SubscriptionSource double.
type subs struct {
	mu   sync.Mutex
	list []events.Subscription
}

func (s *subs) set(list ...events.Subscription) {
	s.mu.Lock()
	s.list = list
	s.mu.Unlock()
}

func (s *subs) ActiveSubscriptions(context.Context) ([]events.Subscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]events.Subscription{}, s.list...), nil
}

// fixture is an env with a host, the thing event, an emitter command and a
// dispatcher over the subs double.
type fixture struct {
	t      *testing.T
	env    *hosttest.Env
	host   *host.Host
	subs   *subs
	disp   *events.Dispatcher
	outbox *events.Outbox
}

type fixtureOption func(*events.Config)

func newFixture(t *testing.T, opts ...fixtureOption) *fixture {
	t.Helper()
	env := hosttest.New(t)
	env.Subjects.Grant(7, "p1")
	env.Catalog.Add(spi.ResolvedResource{Package: "acme.a", Version: "1.0.0", Resource: manifest.Resource{
		ID: thingEvent, Kind: manifest.ResEvent, Payload: hosttest.Obj("name", hosttest.Str()),
	}})
	env.Artifacts.Add(t, "acme.a", "1.0.0", prelude+`
export const emit = async (ctx, input) => {
  await ctx.transaction(async (tx) => {
    await tx.insert(item, { label: "row:" + input.name });
    await tx.emit(Thing, { payload: { name: input.name } });
    if (input.rollback) throw new Error("rolled back after emit");
  });
  return { ok: true };
};`)
	env.Catalog.Add(spi.ResolvedResource{Package: "acme.a", Version: "1.0.0", Resource: hosttest.Command("acme.test.emit", false,
		hosttest.Obj("name", hosttest.Str(), "rollback", hosttest.Optional(hosttest.Bool())), hosttest.Obj("ok", hosttest.Bool()))})
	_, err := env.Bindings.Activate(context.Background(), "acme.test.emit", "acme.a", "1.0.0", "emit", nil)
	require.NoError(t, err)

	outbox := events.NewOutbox(env.Catalog)
	h, err := host.New(host.Config{
		Pool: env.NewPool(1), DB: env.DB, Bindings: env.Bindings, Catalog: env.Catalog, Data: env.Store,
		Artifacts: env.Artifacts, Outbox: outbox, Logger: env.Logger,
	})
	require.NoError(t, err)
	f := &fixture{t: t, env: env, host: h, subs: &subs{}, outbox: outbox}
	cfg := events.Config{
		DB: env.DB, Runner: h, Subscriptions: f.subs, Subjects: env.Subjects,
		PollInterval: 100 * time.Millisecond, BackoffBase: 10 * time.Millisecond, BackoffMax: 20 * time.Millisecond,
		MaxAttempts: 3, BlockedRecheck: 30 * time.Millisecond, Logger: env.Logger,
	}
	for _, o := range opts {
		o(&cfg)
	}
	f.disp, err = events.NewDispatcher(cfg)
	require.NoError(t, err)
	t.Cleanup(f.disp.Stop)
	return f
}

// subscribe registers a subscription whose handler source is given.
func (f *fixture) subscribe(id, source string, mods ...func(*events.Subscription)) events.Subscription {
	f.t.Helper()
	art := f.env.Artifacts.Add(f.t, "acme.sub."+id, "1.0.0", prelude+source)
	sub := events.Subscription{ID: "acme.sub." + id, Package: "acme.sub." + id, Version: "1.0.0", Export: "handler", Event: thingEvent, Artifact: art}
	for _, m := range mods {
		m(&sub)
	}
	return sub
}

// emit runs the emitter command as the user.
func (f *fixture) emit(user int64, name string, rollback bool) error {
	in, err := json.Marshal(struct {
		Name     string `json:"name"`
		Rollback bool   `json:"rollback"`
	}{Name: name, Rollback: rollback})
	if err != nil {
		return err
	}
	_, err = f.host.Invoke(context.Background(), host.Request{
		Contract: "acme.test.emit", Input: in, Subject: f.env.Subject(user, "p1"), Trigger: host.Trigger{Kind: host.TriggerUI, Ref: "test"},
	})
	return err
}

// deliverUntil runs the dispatcher until cond holds. Delivery is asynchronous
// work to be picked up by the poll loop, so it polls.
func (f *fixture) deliverUntil(timeout time.Duration, cond func() bool, msg string) {
	f.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_, err := f.disp.RunOnce(context.Background())
		require.NoError(f.t, err)
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.Fail(f.t, "condition not met: "+msg)
}

type receipt struct {
	Status, LastError string
	Attempts          int
	InvocationID      *string
}

func (f *fixture) receipts(sub string) map[string]receipt {
	rows, err := f.env.DB.Query(context.Background(), `
		SELECT e.payload->>'name', r.status, r.attempts, r.last_error, r.invocation_id::text
		FROM extruntime.subscription_receipts r JOIN extruntime.outbox_events e ON e.id = r.event_id
		WHERE r.subscription = $1`, sub)
	require.NoError(f.t, err)
	defer rows.Close()
	out := map[string]receipt{}
	for rows.Next() {
		var name string
		var r receipt
		require.NoError(f.t, rows.Scan(&name, &r.Status, &r.Attempts, &r.LastError, &r.InvocationID))
		out[name] = r
	}
	require.NoError(f.t, rows.Err())
	return out
}

func (f *fixture) outboxCount() int {
	var n int
	require.NoError(f.t, f.env.DB.QueryRow(context.Background(), `SELECT count(*) FROM extruntime.outbox_events`).Scan(&n))
	return n
}

var _ = uuid.Nil
