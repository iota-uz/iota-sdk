package events_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host/hosttest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/triggerowner"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

// The Go consumer tests need the database only: no Node pool, no packages.

type named struct {
	Name string `json:"name"`
}

var (
	goEventA = native.Event[named]("acme.go.alpha.v1", native.EventSpec{})
	goEventB = native.Event[named]("acme.go.beta.v1", native.EventSpec{})
)

type goEnv struct {
	t        *testing.T
	db       *pgxpool.Pool
	tenant   uuid.UUID
	subjects *hosttest.Subjects
	logger   *slog.Logger
}

func newGoEnv(t *testing.T) *goEnv {
	t.Helper()
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:      "extruntime_consumers",
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	e := &goEnv{t: t, db: scope.Pool, tenant: scope.Tenant.ID, subjects: hosttest.NewSubjects(),
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))}
	_, err := e.db.Exec(context.Background(), `CREATE TABLE consumer_effects (
		consumer text NOT NULL, event_id uuid NOT NULL, tenant_id uuid NOT NULL, tenant_setting text NOT NULL, name text NOT NULL,
		PRIMARY KEY (consumer, event_id))`)
	require.NoError(t, err)
	return e
}

func (e *goEnv) newTenant() uuid.UUID {
	e.t.Helper()
	id := uuid.New()
	_, err := e.db.Exec(context.Background(),
		`INSERT INTO tenants (id, name, domain, created_at, updated_at) VALUES ($1, $2, $3, NOW(), NOW())`, id, "t-"+id.String()[:8], id.String()[:8]+".test")
	require.NoError(e.t, err)
	return id
}

func (e *goEnv) publish(tenant uuid.UUID, ev *native.EventDef[named], name string, initiator int64) string {
	e.t.Helper()
	ctx := context.Background()
	tx, err := e.db.Begin(ctx)
	require.NoError(e.t, err)
	id, err := events.Publish(ctx, tx, spi.Scope{Subject: spi.Subject{TenantID: tenant, UserID: initiator}}, ev, nil, named{Name: name})
	require.NoError(e.t, err)
	require.NoError(e.t, tx.Commit(ctx))
	return id
}

func (e *goEnv) dispatcher(src events.ConsumerSource, mods ...func(*events.Config)) *events.Dispatcher {
	e.t.Helper()
	cfg := events.Config{
		DB: e.db, Consumers: src, Subjects: e.subjects, PollInterval: 50 * time.Millisecond,
		BackoffBase: time.Millisecond, BackoffMax: 2 * time.Millisecond, MaxAttempts: 3, BlockedRecheck: time.Millisecond, Logger: e.logger,
	}
	for _, m := range mods {
		m(&cfg)
	}
	d, err := events.NewDispatcher(cfg)
	require.NoError(e.t, err)
	e.t.Cleanup(d.Stop)
	return d
}

// drain runs passes until several deliver nothing and no failed delivery
// awaits a retry.
func (e *goEnv) drain(d *events.Dispatcher) {
	e.t.Helper()
	idle := 0
	for i := 0; i < 500; i++ {
		n, err := d.RunOnce(context.Background())
		require.NoError(e.t, err)
		if n > 0 {
			idle = 0
			continue
		}
		idle++
		var failed int
		require.NoError(e.t, e.db.QueryRow(context.Background(), `SELECT count(*) FROM extruntime.subscription_receipts WHERE status = 'failed'`).Scan(&failed))
		if idle >= 3 && failed == 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	require.Fail(e.t, "the dispatcher did not settle")
}

func (e *goEnv) effects(consumer string) map[string]string {
	e.t.Helper()
	rows, err := e.db.Query(context.Background(), `SELECT name, tenant_id::text, tenant_setting FROM consumer_effects WHERE consumer = $1`, consumer)
	require.NoError(e.t, err)
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var name, tenant, setting string
		require.NoError(e.t, rows.Scan(&name, &tenant, &setting))
		require.Equal(e.t, tenant, setting, "the delivery transaction carries the event tenant in app.tenant_id")
		out[name] = tenant
	}
	require.NoError(e.t, rows.Err())
	return out
}

type goReceipt struct {
	Status   string
	Attempts int
	Error    string
}

func (e *goEnv) receipt(consumer, eventID string) goReceipt {
	e.t.Helper()
	var r goReceipt
	err := e.db.QueryRow(context.Background(), `SELECT status, attempts, last_error FROM extruntime.subscription_receipts WHERE subscription = $1 AND event_id = $2`,
		consumer, eventID).Scan(&r.Status, &r.Attempts, &r.Error)
	if errors.Is(err, pgx.ErrNoRows) {
		return goReceipt{}
	}
	require.NoError(e.t, err)
	return r
}

// goConsumer records its effect in the delivery transaction.
type goConsumer struct {
	id string

	mu    sync.Mutex
	keys  []string
	after int64
	calls map[string]int
	// fail decides the outcome of a delivery before the effect is written.
	fail func(ctx context.Context, ev events.Envelope, call int) error
}

func newGoConsumer(id string, keys ...string) *goConsumer {
	return &goConsumer{id: id, keys: keys, calls: map[string]int{}}
}

func (c *goConsumer) ID() string { return c.id }

func (c *goConsumer) Events(context.Context) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.keys...), nil
}

func (c *goConsumer) AfterSeq(context.Context) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.after, nil
}

func (c *goConsumer) Deliver(ctx context.Context, tx pgx.Tx, ev events.Envelope) error {
	c.mu.Lock()
	c.calls[ev.ID]++
	call := c.calls[ev.ID]
	fail := c.fail
	c.mu.Unlock()
	if fail != nil {
		if err := fail(ctx, ev, call); errors.Is(err, errBail) {
			return nil
		} else if err != nil {
			return err
		}
	}
	var p named
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO consumer_effects (consumer, event_id, tenant_id, tenant_setting, name)
		VALUES ($1, $2, $3, current_setting('app.tenant_id', true), $4)`, c.id, ev.ID, ev.TenantID, p.Name)
	return err
}

func (c *goConsumer) set(f func(c *goConsumer)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	f(c)
}

func TestGoConsumersAcrossTenantsAreExactlyOnceAndAFailingOneIsIsolated(t *testing.T) {
	e := newGoEnv(t)
	other := e.newTenant()
	ids := make([]string, 0, 5)
	for i, tenant := range []uuid.UUID{e.tenant, other, e.tenant, other, e.tenant} {
		ids = append(ids, e.publish(tenant, goEventA, fmt.Sprintf("e%d", i), 0))
	}

	a := newGoConsumer("acme.go.a", goEventA.ID())
	b := newGoConsumer("acme.go.b", goEventA.ID())
	bad := newGoConsumer("acme.go.bad", goEventA.ID())
	bad.fail = func(context.Context, events.Envelope, int) error { return errors.New("handler bug") }

	// b crashes the pass in the middle: its third delivery cancels the pass
	// (a process stop) after a and b delivered part of the journal.
	pass, cancel := context.WithCancel(context.Background())
	b.fail = func(_ context.Context, ev events.Envelope, call int) error {
		if ev.ID == ids[2] && call == 1 {
			cancel()
			return context.Canceled
		}
		return nil
	}
	d := e.dispatcher(events.StaticConsumers(a, b, bad))
	_, err := d.RunOnce(pass)
	require.Error(t, err, "the interrupted pass reports the cancellation")

	e.drain(d)

	for _, c := range []*goConsumer{a, b} {
		got := e.effects(c.id)
		require.Len(t, got, 5, "%s delivered every event once", c.id)
		require.Equal(t, other.String(), got["e1"])
		require.Equal(t, e.tenant.String(), got["e0"])
		for _, id := range ids {
			require.Equal(t, events.StatusDone, e.receipt(c.id, id).Status)
		}
	}
	require.Empty(t, e.effects(bad.id))
	for _, id := range ids {
		r := e.receipt(bad.id, id)
		require.Equal(t, events.StatusDead, r.Status, "the failing consumer dead-letters every event")
		require.Equal(t, 3, r.Attempts)
		require.Contains(t, r.Error, "handler bug")
	}

	// A second dispatcher (another process) delivers nothing again.
	n, err := e.dispatcher(events.StaticConsumers(a, b, bad)).RunOnce(context.Background())
	require.NoError(t, err)
	require.Zero(t, n)
	// A handler interrupted by the cancelled pass may run again, but its
	// rolled back effect never doubles: consumer_effects is keyed by
	// (consumer, event) and holds exactly one row per event above.
}

func TestInfrastructureErrorIsRetriedWithoutCountingAttempts(t *testing.T) {
	e := newGoEnv(t)
	id := e.publish(e.tenant, goEventA, "x", 0)
	c := newGoConsumer("acme.go.infra", goEventA.ID())
	var outage atomic.Bool
	outage.Store(true)
	c.fail = func(context.Context, events.Envelope, int) error {
		if outage.Load() {
			return &host.InfraError{Err: errors.New("database is down")}
		}
		return nil
	}
	d := e.dispatcher(events.StaticConsumers(c))
	for i := 0; i < 6; i++ {
		_, err := d.RunOnce(context.Background())
		require.NoError(t, err)
		time.Sleep(5 * time.Millisecond)
	}
	r := e.receipt(c.id, id)
	require.Equal(t, events.StatusFailed, r.Status, "an outage never dead-letters the event")
	require.Zero(t, r.Attempts)
	require.GreaterOrEqual(t, c.calls[id], 4, "MaxAttempts (3) did not stop the retries")

	outage.Store(false)
	e.drain(d)
	require.Equal(t, events.StatusDone, e.receipt(c.id, id).Status)
	require.Len(t, e.effects(c.id), 1)
}

func TestEventKeysAreReadEveryPassAndTheWatermarkHolds(t *testing.T) {
	e := newGoEnv(t)
	e.publish(e.tenant, goEventA, "a-old", 0)
	mark, err := events.Watermark(context.Background(), e.db)
	require.NoError(t, err)
	e.publish(e.tenant, goEventA, "a-new", 0)
	e.publish(e.tenant, goEventB, "b-new", 0)

	c := newGoConsumer("acme.go.keys", goEventA.ID())
	c.after = mark
	d := e.dispatcher(events.StaticConsumers(c))
	e.drain(d)
	require.Equal(t, map[string]string{"a-new": e.tenant.String()}, e.effects(c.id), "older events and other keys are not delivered")

	c.set(func(c *goConsumer) { c.keys = append(c.keys, goEventB.ID()) })
	e.drain(d)
	require.Len(t, e.effects(c.id), 2, "a key added later receives its events after the watermark")
	require.Contains(t, e.effects(c.id), "b-new")
}

func TestNoSubjectBlocksTheDeliveryUntilAnOwnerIsAssigned(t *testing.T) {
	e := newGoEnv(t)
	id := e.publish(e.tenant, goEventA, "sys", 0)
	c := newGoConsumer("acme.go.owned", goEventA.ID())
	var acted spi.Subject
	c.fail = func(ctx context.Context, ev events.Envelope, _ int) error {
		s, err := events.ResolveSubject(ctx, e.db, e.subjects, ev, "acme.go.owned", events.RunAsInitiator)
		acted = s
		return err
	}
	d := e.dispatcher(events.StaticConsumers(c))
	e.drain(d)
	r := e.receipt(c.id, id)
	require.Equal(t, events.StatusBlocked, r.Status)
	require.Zero(t, r.Attempts, "a blocked delivery counts no attempt")
	require.Contains(t, r.Error, "no owner-user")

	e.subjects.Grant(42)
	require.NoError(t, triggerowner.Assign(context.Background(), e.db, c.id, e.tenant, 42))
	e.drain(d)
	require.Equal(t, events.StatusDone, e.receipt(c.id, id).Status)
	require.Equal(t, int64(42), acted.UserID)
	require.Equal(t, spi.SubjectOwner, acted.Kind)
}

func TestResolveSubjectModes(t *testing.T) {
	e := newGoEnv(t)
	ctx := context.Background()
	e.subjects.Grant(7, "p.initiator")
	e.subjects.Grant(42, "p.owner")
	initiator := int64(7)
	withInitiator := events.Envelope{ID: uuid.NewString(), TenantID: e.tenant, Event: goEventA.ID(), InitiatorUserID: &initiator}
	system := events.Envelope{ID: uuid.NewString(), TenantID: e.tenant, Event: goEventA.ID()}

	// Neither an initiator nor an owner.
	_, err := events.ResolveSubject(ctx, e.db, e.subjects, system, "acme.trigger", events.RunAsInitiator)
	require.ErrorIs(t, err, events.ErrNoSubject)
	_, err = events.ResolveSubject(ctx, e.db, e.subjects, withInitiator, "acme.trigger", events.RunAsOwner)
	require.ErrorIs(t, err, events.ErrNoSubject, "the owner mode does not fall back to the initiator")

	s, err := events.ResolveSubject(ctx, e.db, e.subjects, withInitiator, "acme.trigger", events.RunAsInitiator)
	require.NoError(t, err)
	require.Equal(t, int64(7), s.UserID)
	require.Equal(t, spi.SubjectInitiator, s.Kind)
	require.Equal(t, []string{"p.initiator"}, s.Permissions)

	_, err = e.db.Exec(ctx, `INSERT INTO extruntime.trigger_owners (trigger_id, tenant_id, user_id) VALUES ('acme.trigger', $1, 42)`, e.tenant)
	require.NoError(t, err)
	s, err = events.ResolveSubject(ctx, e.db, e.subjects, withInitiator, "acme.trigger", events.RunAsOwner)
	require.NoError(t, err)
	require.Equal(t, int64(42), s.UserID, "the owner mode ignores the initiator")
	require.Equal(t, spi.SubjectOwner, s.Kind)
	s, err = events.ResolveSubject(ctx, e.db, e.subjects, system, "acme.trigger", events.RunAsInitiator)
	require.NoError(t, err)
	require.Equal(t, int64(42), s.UserID, "without an initiator the owner acts")

	e.subjects.Block(7, "account disabled")
	_, err = events.ResolveSubject(ctx, e.db, e.subjects, withInitiator, "acme.trigger", events.RunAsInitiator)
	require.ErrorIs(t, err, events.ErrNoSubject, "a blocked user is no subject")
	require.ErrorContains(t, err, "account disabled")
	require.ErrorIs(t, err, host.ErrSubjectBlocked)

	_, err = events.ResolveSubject(ctx, e.db, e.subjects, system, "acme.trigger", events.RunAs("nobody"))
	require.Error(t, err)
	require.NotErrorIs(t, err, events.ErrNoSubject)
}

func TestAfterCommitRunsOnlyWhenTheDeliveryCommits(t *testing.T) {
	e := newGoEnv(t)
	e.publish(e.tenant, goEventA, "x", 0)
	c := newGoConsumer("acme.go.after", goEventA.ID())
	var mu sync.Mutex
	var ran []int
	c.fail = func(ctx context.Context, _ events.Envelope, call int) error {
		require.True(t, events.AfterCommit(ctx, func(context.Context) { mu.Lock(); ran = append(ran, call); mu.Unlock() }))
		if call == 1 {
			return errors.New("first try fails")
		}
		return nil
	}
	e.drain(e.dispatcher(events.StaticConsumers(c)))
	require.Equal(t, []int{2}, ran, "the rolled back delivery dropped its after-commit work")
	require.False(t, events.AfterCommit(context.Background(), func(context.Context) {}), "no delivery, nothing registered")
}

func TestDuplicateConsumerIDIsReportedAndDeliveredOnce(t *testing.T) {
	e := newGoEnv(t)
	e.publish(e.tenant, goEventA, "x", 0)
	c := newGoConsumer("acme.go.dup", goEventA.ID())
	twin := newGoConsumer("acme.go.dup", goEventA.ID())
	_, err := e.dispatcher(events.StaticConsumers(c, twin)).RunOnce(context.Background())
	require.ErrorContains(t, err, "registered twice")
	require.Len(t, e.effects(c.id), 1)
	require.Empty(t, twin.calls)
}

func TestPruneKeepsEventsWithAnUnfinishedDeliveryOfAnyConsumer(t *testing.T) {
	e := newGoEnv(t)
	delivered := e.publish(e.tenant, goEventA, "delivered", 0)
	pending := e.publish(e.tenant, goEventB, "pending", 0)
	ok := newGoConsumer("acme.go.ok", goEventA.ID(), goEventB.ID())
	failing := newGoConsumer("acme.go.failing", goEventB.ID())
	failing.fail = func(context.Context, events.Envelope, int) error { return errors.New("not yet") }
	d := e.dispatcher(events.StaticConsumers(ok, failing), func(c *events.Config) { c.Retention = time.Millisecond; c.MaxAttempts = 100 })
	_, err := d.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, events.StatusFailed, e.receipt(failing.id, pending).Status)
	time.Sleep(5 * time.Millisecond)

	n, err := d.Prune(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(1), n)
	var left []string
	rows, err := e.db.Query(context.Background(), `SELECT id::text FROM extruntime.outbox_events`)
	require.NoError(t, err)
	for rows.Next() {
		var id string
		require.NoError(t, rows.Scan(&id))
		left = append(left, id)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []string{pending}, left, "%s was pruned, the event another consumer still owes is kept", delivered)
}

// A delivery whose commit fails (here: the consumer swallowed a SQL error and
// left the transaction aborted) counts as a failed attempt: it backs off and
// dead-letters instead of being fetched first on every pass and holding back
// the later events.
func TestFailedCommitCountsAnAttemptAndDoesNotBlockLaterEvents(t *testing.T) {
	e := newGoEnv(t)
	poisoned := e.publish(e.tenant, goEventA, "poisoned", 0)
	later := e.publish(e.tenant, goEventA, "later", 0)
	c := newGoConsumer("acme.go.commit", goEventA.ID())
	c.fail = func(ctx context.Context, ev events.Envelope, _ int) error {
		if ev.ID != poisoned {
			return nil
		}
		tx, err := composables.UseTx(ctx)
		if err != nil {
			return err
		}
		_, _ = tx.Exec(ctx, `SELECT * FROM no_such_table`) // error swallowed: the tx is aborted
		return errBail
	}
	d := e.dispatcher(events.StaticConsumers(nil, c))
	e.drain(d)
	r := e.receipt(c.id, poisoned)
	require.Equal(t, events.StatusDead, r.Status)
	require.Equal(t, 3, r.Attempts)
	require.Equal(t, events.StatusDone, e.receipt(c.id, later).Status)
	require.Equal(t, map[string]string{"later": e.tenant.String()}, e.effects(c.id))
}

// errBail makes goConsumer.Deliver return nil without writing its effect.
var errBail = errors.New("bail")

// A failing source does not hide the consumers of the other sources: they are
// served in the same pass and the source's error is reported.
func TestFailingSourceDoesNotHoldBackOtherConsumers(t *testing.T) {
	e := newGoEnv(t)
	e.publish(e.tenant, goEventA, "x", 0)
	good := newGoConsumer("acme.go.good", goEventA.ID())
	broken := events.ConsumerSourceFunc(func(context.Context) ([]events.Consumer, error) {
		return nil, errors.New("subscriptions unavailable")
	})
	d := e.dispatcher(events.ComposeConsumers(broken, events.StaticConsumers(good)))
	n, err := d.RunOnce(context.Background())
	require.ErrorContains(t, err, "subscriptions unavailable")
	require.Equal(t, 1, n)
	require.Len(t, e.effects(good.id), 1, "the healthy consumer was delivered to")

	list, err := events.ComposeConsumers(events.StaticConsumers(good), broken, events.StaticConsumers(newGoConsumer("acme.go.other"))).Consumers(context.Background())
	require.Error(t, err)
	require.Len(t, list, 2, "the consumers of every readable source are returned")
}

// On a small pool the dispatcher keeps room for its LISTEN connection and for
// a second connection of every running delivery (a package handler records
// its invocation on the pool before it runs): a backlog for several
// consumers whose deliveries go back to the pool drains without timeouts.
func TestSmallPoolBacklogDrainsWhenDeliveriesUseASecondConnection(t *testing.T) {
	e := newGoEnv(t)
	cfg := e.db.Config().Copy()
	cfg.MaxConns, cfg.MinConns = 4, 0
	small, err := pgxpool.NewWithConfig(context.Background(), cfg)
	require.NoError(t, err)
	t.Cleanup(small.Close)

	const perConsumer = 5
	for i := 0; i < perConsumer; i++ {
		e.publish(e.tenant, goEventA, fmt.Sprintf("e%d", i), 0)
	}
	var consumers []events.Consumer
	var timeouts atomic.Int64
	for i := 0; i < 3; i++ {
		c := newGoConsumer(fmt.Sprintf("acme.go.pool%d", i), goEventA.ID())
		c.fail = func(ctx context.Context, _ events.Envelope, _ int) error {
			qctx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			var one int
			if err := small.QueryRow(qctx, `SELECT 1`).Scan(&one); err != nil { // a second connection, like host.Run's invocation record
				timeouts.Add(1)
				return err
			}
			return nil
		}
		consumers = append(consumers, c)
	}
	d, err := events.NewDispatcher(events.Config{
		DB: small, Consumers: events.StaticConsumers(consumers...), Subjects: e.subjects, PollInterval: 50 * time.Millisecond,
		BackoffBase: time.Millisecond, BackoffMax: 2 * time.Millisecond, Logger: e.logger,
	})
	require.NoError(t, err)
	d.Start(context.Background()) // holds the LISTEN connection
	t.Cleanup(d.Stop)

	require.Eventually(t, func() bool {
		var n int
		require.NoError(t, e.db.QueryRow(context.Background(), `SELECT count(*) FROM consumer_effects`).Scan(&n))
		return n == 3*perConsumer
	}, 30*time.Second, 50*time.Millisecond, "every consumer drained its backlog")
	require.Zero(t, timeouts.Load(), "no delivery waited for a connection until its timeout")
}
