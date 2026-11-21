package events_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host/hosttest"
)

// goSlots records the slots a Go schedule handler ran, and writes each into
// its transaction so the receipt and the effect are checked together.
type goSlots struct {
	mu   sync.Mutex
	runs []events.Slot
	fail func(events.Slot) error
}

func (g *goSlots) RunSlot(ctx context.Context, tx pgx.Tx, slot events.Slot) error {
	if g.fail != nil {
		if err := g.fail(slot); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO host_items (tenant_id, label) VALUES ($1, $2)`, slot.TenantID, fmt.Sprintf("%s:%s:%s", slot.ScheduleID, slot.TenantID, slot.At.Format(time.RFC3339))); err != nil {
		return err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.runs = append(g.runs, slot)
	return nil
}

func (g *goSlots) slots() []events.Slot {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]events.Slot(nil), g.runs...)
}

type fakeRunner struct{}

func (fakeRunner) Run(context.Context, host.RunRequest) (host.Result, error) {
	return host.Result{}, errors.New("not used")
}

// countingTenants counts how often the scheduler lists tenants.
type countingTenants struct {
	list  []uuid.UUID
	calls int
}

func (c *countingTenants) Tenants(context.Context) ([]uuid.UUID, error) {
	c.calls++
	return c.list, nil
}

// goScheduler builds a scheduler without a Runner: Go schedules need no Node.
func goScheduler(t *testing.T, env *hosttest.Env, src events.ScheduleSource, now *time.Time, mods ...func(*events.SchedulerConfig)) *events.Scheduler {
	t.Helper()
	cfg := events.SchedulerConfig{
		DB: env.DB, Schedules: src, Lookback: time.Minute, BackoffBase: 10 * time.Millisecond,
		BlockedRecheck: 30 * time.Millisecond, MaxAttempts: 3, Logger: env.Logger,
		Now: func() time.Time { return *now },
	}
	for _, m := range mods {
		m(&cfg)
	}
	s, err := events.NewScheduler(cfg)
	require.NoError(t, err)
	t.Cleanup(s.Stop)
	return s
}

func runsOf(t *testing.T, env *hosttest.Env, schedule string) map[string]string {
	t.Helper()
	rows, err := env.DB.Query(context.Background(), `SELECT tenant_id::text || '@' || to_char(slot AT TIME ZONE 'UTC', 'HH24:MI'), status FROM extruntime.schedule_runs WHERE schedule = $1`, schedule)
	require.NoError(t, err)
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, st string
		require.NoError(t, rows.Scan(&k, &st))
		out[k] = st
	}
	require.NoError(t, rows.Err())
	return out
}

func TestGoScheduleOfOneTenantRunsOnlyForItAndNeverListsTenants(t *testing.T) {
	env := hosttest.New(t)
	other := env.NewTenant()
	h := &goSlots{}
	tenant := env.Tenant
	def := events.ScheduleDef{ID: "automation:a1:t1", Cron: "* * * * *", TenantID: &tenant, Handler: h}
	tenants := &countingTenants{list: []uuid.UUID{env.Tenant, other}}
	now := time.Date(2026, 6, 1, 10, 30, 20, 0, time.UTC)
	s := goScheduler(t, env, events.StaticSchedules(def), &now, func(c *events.SchedulerConfig) { c.Tenants = tenants })

	n, err := s.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, []events.Slot{{ScheduleID: def.ID, TenantID: env.Tenant, At: time.Date(2026, 6, 1, 10, 30, 0, 0, time.UTC)}}, h.slots())
	require.Zero(t, tenants.calls, "a tenant-scoped schedule does not consult the TenantLister")
	require.Equal(t, map[string]string{env.Tenant.String() + "@10:30": events.StatusDone}, runsOf(t, env, def.ID), "no run for the second tenant")
}

func TestGoScheduleWithoutTenantRunsForEveryTenant(t *testing.T) {
	env := hosttest.New(t)
	other := env.NewTenant()
	h := &goSlots{}
	def := events.ScheduleDef{ID: "app.tick", Cron: "* * * * *", Handler: h}
	now := time.Date(2026, 6, 1, 10, 30, 20, 0, time.UTC)
	s := goScheduler(t, env, events.StaticSchedules(def), &now, func(c *events.SchedulerConfig) {
		c.Tenants = &countingTenants{list: []uuid.UUID{env.Tenant, other}}
	})
	n, err := s.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, n)
}

func TestDoubleRunOnceAndTwoSchedulersRunASlotOnce(t *testing.T) {
	env := hosttest.New(t)
	h := &goSlots{}
	tenant := env.Tenant
	def := events.ScheduleDef{ID: "automation:a1:t1", Cron: "* * * * *", TenantID: &tenant, Handler: h}
	now := time.Date(2026, 6, 1, 10, 30, 20, 0, time.UTC)
	a := goScheduler(t, env, events.StaticSchedules(def), &now)
	b := goScheduler(t, env, events.StaticSchedules(def), &now)

	var wg sync.WaitGroup
	for _, s := range []*events.Scheduler{a, b} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.RunOnce(context.Background())
			assert.NoError(t, err)
		}()
	}
	wg.Wait()
	n, err := a.RunOnce(context.Background())
	require.NoError(t, err)
	require.Zero(t, n)
	require.Len(t, h.slots(), 1, "RunSlot ran once for the slot")
}

func TestScheduleAddedAndRemovedBetweenPassesTakesEffectWithoutRestart(t *testing.T) {
	env := hosttest.New(t)
	h := &goSlots{}
	tenant := env.Tenant
	def := events.ScheduleDef{ID: "automation:a1:t1", Cron: "* * * * *", TenantID: &tenant, Handler: h}
	src := &schedules{}
	now := time.Date(2026, 6, 1, 10, 30, 20, 0, time.UTC)
	s := goScheduler(t, env, src, &now)

	n, err := s.RunOnce(context.Background())
	require.NoError(t, err)
	require.Zero(t, n)

	src.mu.Lock()
	src.defs = []events.ScheduleDef{def}
	src.mu.Unlock()
	now = now.Add(time.Minute)
	n, err = s.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, n, "the added schedule fires on the nearest slot")

	src.mu.Lock()
	src.defs = nil
	src.mu.Unlock()
	now = now.Add(time.Minute)
	n, err = s.RunOnce(context.Background())
	require.NoError(t, err)
	require.Zero(t, n, "the removed schedule stops")
	require.Len(t, h.slots(), 1)
}

func TestSlotsBeforeActiveFromAreNotRunAndNotRecorded(t *testing.T) {
	env := hosttest.New(t)
	h := &goSlots{}
	tenant := env.Tenant
	now := time.Date(2026, 6, 1, 10, 30, 20, 0, time.UTC)
	def := events.ScheduleDef{
		ID: "automation:a1:t1", Cron: "* * * * *", TenantID: &tenant, Handler: h,
		ActiveFrom: time.Date(2026, 6, 1, 10, 29, 30, 0, time.UTC), MissedAfter: time.Hour,
	}
	s := goScheduler(t, env, events.StaticSchedules(def), &now)
	n, err := s.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, map[string]string{env.Tenant.String() + "@10:30": events.StatusDone}, runsOf(t, env, def.ID),
		"10:29 is before ActiveFrom (10:29:30): it neither runs nor is recorded")
}

func TestSlotOlderThanMissedAfterIsRecordedAsSkippedAndNotRun(t *testing.T) {
	env := hosttest.New(t)
	h := &goSlots{}
	tenant := env.Tenant
	def := events.ScheduleDef{ID: "automation:a1:t1", Cron: "* * * * *", TenantID: &tenant, Handler: h, MissedAfter: 90 * time.Second}
	now := time.Date(2026, 6, 1, 10, 30, 20, 0, time.UTC)
	// The schedule became active at 10:27:30: slots 10:28 .. 10:30 exist.
	def.ActiveFrom = time.Date(2026, 6, 1, 10, 27, 30, 0, time.UTC)
	s := goScheduler(t, env, events.StaticSchedules(def), &now)

	n, err := s.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, n, "10:29 (81s late) and 10:30 run; 10:28 is older than MissedAfter")
	runs := runsOf(t, env, def.ID)
	require.Equal(t, events.StatusSkipped, runs[env.Tenant.String()+"@10:28"])
	require.Equal(t, events.StatusDone, runs[env.Tenant.String()+"@10:29"])
	require.Equal(t, events.StatusDone, runs[env.Tenant.String()+"@10:30"])
	require.Len(t, h.slots(), 2)

	// Skipped slots are not retried and not recorded twice.
	n, err = s.RunOnce(context.Background())
	require.NoError(t, err)
	require.Zero(t, n)
	require.Len(t, runsOf(t, env, def.ID), 3)
}

func TestMissedAfterDefaultsToLookback(t *testing.T) {
	env := hosttest.New(t)
	h := &goSlots{}
	tenant := env.Tenant
	def := events.ScheduleDef{ID: "automation:a1:t1", Cron: "* * * * *", TenantID: &tenant, Handler: h}
	now := time.Date(2026, 6, 1, 10, 30, 20, 0, time.UTC)
	def.ActiveFrom = now.Add(-3 * time.Minute)
	s := goScheduler(t, env, events.StaticSchedules(def), &now) // Lookback is one minute
	n, err := s.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, n)
	runs := runsOf(t, env, def.ID)
	require.Equal(t, events.StatusSkipped, runs[env.Tenant.String()+"@10:28"])
	require.Equal(t, events.StatusSkipped, runs[env.Tenant.String()+"@10:29"])
	require.Equal(t, events.StatusDone, runs[env.Tenant.String()+"@10:30"])
}

func TestFailingGoHandlerRollsBackRetriesThenDeadLetters(t *testing.T) {
	env := hosttest.New(t)
	h := &goSlots{fail: func(events.Slot) error { return errors.New("boom") }}
	tenant := env.Tenant
	def := events.ScheduleDef{ID: "automation:a1:t1", Cron: "* * * * *", TenantID: &tenant, Handler: h}
	now := time.Date(2026, 6, 1, 10, 30, 20, 0, time.UTC)
	s := goScheduler(t, env, events.StaticSchedules(def), &now)
	require.Eventually(t, func() bool {
		_, err := s.RunOnce(context.Background())
		require.NoError(t, err)
		var status string
		var attempts int
		err = env.DB.QueryRow(context.Background(), `SELECT status, attempts FROM extruntime.schedule_runs WHERE tenant_id = $1 AND schedule = $2`, env.Tenant, def.ID).Scan(&status, &attempts)
		return err == nil && status == events.StatusDead && attempts == 3
	}, 10*time.Second, 30*time.Millisecond)
	require.Empty(t, h.slots())
}

func TestGoHandlerWithoutSubjectBlocksTheSlotUntilItResolves(t *testing.T) {
	env := hosttest.New(t)
	var mu sync.Mutex
	ready := false
	h := &goSlots{fail: func(events.Slot) error {
		mu.Lock()
		defer mu.Unlock()
		if !ready {
			return fmt.Errorf("%w: nobody to run as", events.ErrNoSubject)
		}
		return nil
	}}
	tenant := env.Tenant
	def := events.ScheduleDef{ID: "automation:a1:t1", Cron: "* * * * *", TenantID: &tenant, Handler: h}
	now := time.Date(2026, 6, 1, 10, 30, 20, 0, time.UTC)
	s := goScheduler(t, env, events.StaticSchedules(def), &now)
	_, err := s.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, events.StatusBlocked, runsOf(t, env, def.ID)[env.Tenant.String()+"@10:30"])
	mu.Lock()
	ready = true
	mu.Unlock()
	require.Eventually(t, func() bool {
		_, err := s.RunOnce(context.Background())
		require.NoError(t, err)
		return len(h.slots()) == 1
	}, 10*time.Second, 40*time.Millisecond)
}

func TestPanickingGoHandlerFailsItsSlotOnly(t *testing.T) {
	env := hosttest.New(t)
	tenant := env.Tenant
	bad := events.ScheduleDef{ID: "bad", Cron: "* * * * *", TenantID: &tenant, Handler: events.SlotHandlerFunc(func(context.Context, pgx.Tx, events.Slot) error { panic("kaboom") })}
	good := &goSlots{}
	def := events.ScheduleDef{ID: "good", Cron: "* * * * *", TenantID: &tenant, Handler: good}
	now := time.Date(2026, 6, 1, 10, 30, 20, 0, time.UTC)
	s := goScheduler(t, env, events.StaticSchedules(bad, def), &now)
	n, err := s.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, events.StatusFailed, runsOf(t, env, "bad")[env.Tenant.String()+"@10:30"])
}

func TestScheduleOfAMissingTenantDoesNotStopOtherSchedules(t *testing.T) {
	env := hosttest.New(t)
	gone := uuid.New()
	tenant := env.Tenant
	good := &goSlots{}
	now := time.Date(2026, 6, 1, 10, 30, 20, 0, time.UTC)
	s := goScheduler(t, env, events.StaticSchedules(
		events.ScheduleDef{ID: "orphan", Cron: "* * * * *", TenantID: &gone, Handler: &goSlots{}},
		events.ScheduleDef{ID: "good", Cron: "* * * * *", TenantID: &tenant, Handler: good},
	), &now)
	n, err := s.RunOnce(context.Background())
	require.Error(t, err, "the orphaned schedule is reported")
	require.Equal(t, 1, n)
	require.Len(t, good.slots(), 1)
}

func TestPackageScheduleWithoutRunnerIsBlockedWithADiagnostic(t *testing.T) {
	env := hosttest.New(t)
	tenant := env.Tenant
	def := events.ScheduleDef{ID: "acme.sched.tick", Package: "acme.sched.tick", Version: "1.0.0", Export: "handler", Cron: "* * * * *", TenantID: &tenant}
	now := time.Date(2026, 6, 1, 10, 30, 20, 0, time.UTC)
	s := goScheduler(t, env, events.StaticSchedules(def), &now)
	n, err := s.RunOnce(context.Background())
	require.NoError(t, err)
	require.Zero(t, n)
	var status, diag string
	require.NoError(t, env.DB.QueryRow(context.Background(), `SELECT status, last_error FROM extruntime.schedule_runs WHERE tenant_id = $1 AND schedule = $2`, env.Tenant, def.ID).Scan(&status, &diag))
	require.Equal(t, events.StatusBlocked, status)
	require.Contains(t, diag, "runtime is not enabled")
}

func TestComposeSchedulesKeepsEverySourceAndReportsDuplicatesAndFailures(t *testing.T) {
	a := events.StaticSchedules(events.ScheduleDef{ID: "a"}, events.ScheduleDef{ID: "dup"})
	b := events.StaticSchedules(events.ScheduleDef{ID: "b"}, events.ScheduleDef{ID: "dup"})
	failing := events.ScheduleSourceFunc(func(context.Context) ([]events.ScheduleDef, error) { return nil, errors.New("source down") })
	defs, err := events.ComposeSchedules(a, nil, failing, b).ActiveSchedules(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "source down")
	require.Contains(t, err.Error(), `duplicate schedule ID "dup"`)
	ids := make([]string, 0, len(defs))
	for _, d := range defs {
		ids = append(ids, d.ID)
	}
	require.Equal(t, []string{"a", "dup", "b"}, ids)
}

func TestSchedulerRequiresSubjectsOnlyWithARunner(t *testing.T) {
	env := hosttest.New(t)
	_, err := events.NewScheduler(events.SchedulerConfig{DB: env.DB, Schedules: events.StaticSchedules()})
	require.NoError(t, err)
	_, err = events.NewScheduler(events.SchedulerConfig{DB: env.DB, Schedules: events.StaticSchedules(), Runner: fakeRunner{}})
	require.Error(t, err)
}

func TestScheduleWithoutActiveFromRecordsNoSkippedSlots(t *testing.T) {
	env := hosttest.New(t)
	tenant := env.Tenant
	def := events.ScheduleDef{ID: "automation:a1:t1", Cron: "* * * * *", TenantID: &tenant, Handler: &goSlots{}}
	now := time.Date(2026, 6, 1, 10, 30, 20, 0, time.UTC)
	s := goScheduler(t, env, events.StaticSchedules(def), &now)
	n, err := s.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Len(t, runsOf(t, env, def.ID), 1, "no known start of activity: the past is not accounted for")
}
