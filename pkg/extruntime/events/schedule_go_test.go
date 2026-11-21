package events_test

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
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
	h := &goSlots{}
	tenant := env.Tenant
	// The handler writes, then fails: the write must roll back with the receipt.
	var mu sync.Mutex
	written := 0
	def := events.ScheduleDef{ID: "automation:a1:t1", Cron: "* * * * *", TenantID: &tenant,
		Handler: events.SlotHandlerFunc(func(ctx context.Context, tx pgx.Tx, slot events.Slot) error {
			if _, err := tx.Exec(ctx, `INSERT INTO host_items (tenant_id, label) VALUES ($1, 'partial')`, slot.TenantID); err != nil {
				t.Errorf("the handler's write failed: %v", err)
				return err
			}
			mu.Lock()
			written++
			mu.Unlock()
			return errors.New("boom")
		})}
	now := time.Date(2026, 6, 1, 10, 30, 20, 0, time.UTC)
	s := goScheduler(t, env, events.StaticSchedules(def), &now)
	waitFor(t, func() bool {
		_, err := s.RunOnce(context.Background())
		require.NoError(t, err)
		var status string
		var attempts int
		err = env.DB.QueryRow(context.Background(), `SELECT status, attempts FROM extruntime.schedule_runs WHERE tenant_id = $1 AND schedule = $2`, env.Tenant, def.ID).Scan(&status, &attempts)
		return err == nil && status == events.StatusDead && attempts == 3
	}, 10*time.Second, 30*time.Millisecond)
	require.Empty(t, h.slots())
	var rows int
	require.NoError(t, env.DB.QueryRow(context.Background(), `SELECT count(*) FROM host_items`).Scan(&rows))
	require.Zero(t, rows, "the handler's write rolled back with the failed attempts")
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 3, written, "every attempt wrote successfully before failing")
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
	waitFor(t, func() bool {
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

func statusOf(t *testing.T, env *hosttest.Env, schedule string, at time.Time) string {
	t.Helper()
	var status string
	err := env.DB.QueryRow(context.Background(), `SELECT status FROM extruntime.schedule_runs WHERE tenant_id = $1 AND schedule = $2 AND slot = $3`, env.Tenant, schedule, at).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ""
	}
	require.NoError(t, err)
	return status
}

func countStatus(t *testing.T, env *hosttest.Env, schedule, status string) int {
	t.Helper()
	var n int
	require.NoError(t, env.DB.QueryRow(context.Background(), `SELECT count(*) FROM extruntime.schedule_runs WHERE schedule = $1 AND status = $2`, schedule, status).Scan(&n))
	return n
}

func TestActivationAndMissedWindowBoundariesAreInclusive(t *testing.T) {
	env := hosttest.New(t)
	h := &goSlots{}
	tenant := env.Tenant
	def := events.ScheduleDef{
		ID: "automation:a1:t1", Cron: "* * * * *", TenantID: &tenant, Handler: h,
		ActiveFrom: time.Date(2026, 6, 1, 10, 30, 0, 0, time.UTC), MissedAfter: time.Minute,
	}
	now := time.Date(2026, 6, 1, 10, 31, 0, 0, time.UTC)
	s := goScheduler(t, env, events.StaticSchedules(def), &now)
	n, err := s.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, n, "the slot at ActiveFrom and the slot exactly MissedAfter old both run")
	require.Equal(t, events.StatusDone, statusOf(t, env, def.ID, time.Date(2026, 6, 1, 10, 30, 0, 0, time.UTC)))
	require.Equal(t, events.StatusDone, statusOf(t, env, def.ID, now))
}

func TestMissedSlotsAfterLongDowntimeAreRecordedEvenWithoutActiveFrom(t *testing.T) {
	env := hosttest.New(t)
	tenant := env.Tenant
	def := events.ScheduleDef{ID: "automation:daily", Cron: "0 9 * * *", TenantID: &tenant, Handler: &goSlots{}}
	now := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	s := goScheduler(t, env, events.StaticSchedules(def), &now)
	_, err := s.RunOnce(context.Background())
	require.NoError(t, err)
	require.Zero(t, countStatus(t, env, def.ID, events.StatusSkipped), "first seen: nothing before is known")

	now = time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC) // 48 hours of downtime
	_, err = s.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, events.StatusSkipped, statusOf(t, env, def.ID, time.Date(2026, 6, 2, 9, 0, 0, 0, time.UTC)))
	require.Equal(t, events.StatusSkipped, statusOf(t, env, def.ID, time.Date(2026, 6, 3, 9, 0, 0, 0, time.UTC)))
	_, err = s.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, countStatus(t, env, def.ID, events.StatusSkipped), "accounted once")
}

func TestFailedSlotKeepsRetryingAfterItsMissedWindow(t *testing.T) {
	env := hosttest.New(t)
	var mu sync.Mutex
	calls := 0
	h := &goSlots{fail: func(events.Slot) error {
		mu.Lock()
		defer mu.Unlock()
		calls++
		if calls == 1 {
			return errors.New("transient")
		}
		return nil
	}}
	tenant := env.Tenant
	def := events.ScheduleDef{ID: "automation:a1:t1", Cron: "* * * * *", TenantID: &tenant, Handler: h}
	slot := time.Date(2026, 6, 1, 10, 30, 0, 0, time.UTC)
	now := slot.Add(59 * time.Second) // one second left in the one-minute window
	s := goScheduler(t, env, events.StaticSchedules(def), &now)
	_, err := s.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, events.StatusFailed, statusOf(t, env, def.ID, slot))

	now = slot.Add(5 * time.Minute) // the retry is due only after the window closed
	waitFor(t, func() bool {
		_, err := s.RunOnce(context.Background())
		require.NoError(t, err)
		return statusOf(t, env, def.ID, slot) == events.StatusDone
	}, 10*time.Second, 30*time.Millisecond, "the failed slot is retried up to MaxAttempts whatever its age")
}

func TestBlockedSlotExpiresIntoSkippedAfterItsMissedWindow(t *testing.T) {
	env := hosttest.New(t)
	h := &goSlots{fail: func(events.Slot) error { return fmt.Errorf("%w: nobody", events.ErrNoSubject) }}
	tenant := env.Tenant
	def := events.ScheduleDef{ID: "automation:a1:t1", Cron: "* * * * *", TenantID: &tenant, Handler: h}
	slot := time.Date(2026, 6, 1, 10, 30, 0, 0, time.UTC)
	now := slot.Add(20 * time.Second)
	s := goScheduler(t, env, events.StaticSchedules(def), &now)
	_, err := s.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, events.StatusBlocked, statusOf(t, env, def.ID, slot))

	now = slot.Add(3 * time.Minute)
	waitFor(t, func() bool {
		_, err := s.RunOnce(context.Background())
		require.NoError(t, err)
		return statusOf(t, env, def.ID, slot) == events.StatusSkipped
	}, 10*time.Second, 40*time.Millisecond)
	var msg string
	require.NoError(t, env.DB.QueryRow(context.Background(), `SELECT last_error FROM extruntime.schedule_runs WHERE tenant_id = $1 AND schedule = $2 AND slot = $3`, env.Tenant, def.ID, slot).Scan(&msg))
	require.Contains(t, msg, "nobody")
	require.Contains(t, msg, "expired")
}

func TestLongHistoryIsRecordedInBatchesAndNeverStarvesTheCurrentSlot(t *testing.T) {
	env := hosttest.New(t)
	h := &goSlots{}
	tenant := env.Tenant
	now := time.Date(2026, 6, 21, 10, 30, 20, 0, time.UTC)
	def := events.ScheduleDef{ID: "automation:a1:t1", Cron: "* * * * *", TenantID: &tenant, Handler: h, ActiveFrom: now.Add(-20 * 24 * time.Hour)}
	s := goScheduler(t, env, events.StaticSchedules(def), &now)

	n, err := s.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, n, "the current slot runs although 28,800 slots are missed")
	require.Equal(t, events.StatusDone, statusOf(t, env, def.ID, time.Date(2026, 6, 21, 10, 30, 0, 0, time.UTC)))
	first := countStatus(t, env, def.ID, events.StatusSkipped)
	require.Equal(t, 5000, first, "a batch of the oldest missed slots")
	_, err = s.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 10000, countStatus(t, env, def.ID, events.StatusSkipped), "the next pass continues where the accounting stopped")
}

func TestHugeLiveBacklogKeepsTheNewestSlots(t *testing.T) {
	env := hosttest.New(t)
	h := &goSlots{}
	tenant := env.Tenant
	def := events.ScheduleDef{ID: "automation:a1:t1", Cron: "* * * * *", TenantID: &tenant, Handler: h, MissedAfter: time.Hour}
	now := time.Date(2026, 6, 1, 10, 30, 20, 0, time.UTC)
	s := goScheduler(t, env, events.StaticSchedules(def), &now)
	events.SetLiveLimit(s, 10)
	n, err := s.RunOnce(context.Background())
	require.NoError(t, err)
	require.True(t, n >= 1 && n <= 20, "a pass runs the newest slots and one backlog chunk, ran %d", n)
	require.Equal(t, events.StatusDone, statusOf(t, env, def.ID, time.Date(2026, 6, 1, 10, 30, 0, 0, time.UTC)), "the current slot is among them")
	for range 6 {
		_, err = s.RunOnce(context.Background())
		require.NoError(t, err)
	}
	require.Equal(t, 60, countStatus(t, env, def.ID, events.StatusDone), "the whole hour of eligible slots ran")
}

func TestReenabledScheduleDoesNotRetrySlotsFromBeforeItsActivation(t *testing.T) {
	env := hosttest.New(t)
	tenant := env.Tenant
	old := events.ScheduleDef{ID: "automation:a1:t1", Cron: "* * * * *", TenantID: &tenant,
		Handler: events.SlotHandlerFunc(func(context.Context, pgx.Tx, events.Slot) error { return errors.New("failed before") })}
	slot := time.Date(2026, 6, 1, 10, 30, 0, 0, time.UTC)
	now := slot.Add(20 * time.Second)
	_, err := goScheduler(t, env, events.StaticSchedules(old), &now).RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, events.StatusFailed, statusOf(t, env, old.ID, slot))

	// Disabled, then enabled again under the same ID with a new handler.
	h := &goSlots{}
	again := events.ScheduleDef{ID: old.ID, Cron: "* * * * *", TenantID: &tenant, Handler: h, ActiveFrom: time.Date(2026, 6, 1, 10, 31, 5, 0, time.UTC)}
	now = time.Date(2026, 6, 1, 10, 31, 30, 0, time.UTC)
	s := goScheduler(t, env, events.StaticSchedules(again), &now)
	for range 5 {
		time.Sleep(30 * time.Millisecond) // the retry backoff elapses
		_, err = s.RunOnce(context.Background())
		require.NoError(t, err)
	}
	require.Equal(t, events.StatusFailed, statusOf(t, env, old.ID, slot), "the slot before ActiveFrom is not retried")
	for _, got := range h.slots() {
		require.True(t, got.At.After(slot), "the new handler ran %s", got.At)
	}
}

func TestBacklogOfEligibleSlotsProgressesAcrossPasses(t *testing.T) {
	env := hosttest.New(t)
	h := &goSlots{}
	tenant := env.Tenant
	now := time.Date(2026, 6, 1, 10, 30, 20, 0, time.UTC)
	def := events.ScheduleDef{ID: "automation:a1:t1", Cron: "* * * * *", TenantID: &tenant, Handler: h, MissedAfter: time.Hour,
		ActiveFrom: time.Date(2026, 6, 1, 10, 25, 0, 0, time.UTC)} // six eligible slots: 10:25 .. 10:30
	s := goScheduler(t, env, events.StaticSchedules(def), &now)
	events.SetLiveLimit(s, 2)

	_, err := s.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, events.StatusDone, statusOf(t, env, def.ID, time.Date(2026, 6, 1, 10, 30, 0, 0, time.UTC)), "the current slot runs at once")
	for range 2 {
		_, err = s.RunOnce(context.Background())
		require.NoError(t, err)
	}
	require.Equal(t, 6, countStatus(t, env, def.ID, events.StatusDone), "older eligible slots run on the following passes, at the same clock")
	require.Len(t, h.slots(), 6)
	n, err := s.RunOnce(context.Background())
	require.NoError(t, err)
	require.Zero(t, n)
}

func TestFailedSlotStaysClaimedUntilItsBackoffIsDurable(t *testing.T) {
	env := hosttest.New(t)
	tenant := env.Tenant
	var mu sync.Mutex
	calls := 0
	var other *events.Scheduler
	var wg sync.WaitGroup
	now := time.Date(2026, 6, 1, 10, 30, 20, 0, time.UTC)
	handler := events.SlotHandlerFunc(func(context.Context, pgx.Tx, events.Slot) error {
		mu.Lock()
		calls++
		first := calls == 1
		mu.Unlock()
		if first {
			// Another pass reaches the slot while this one is still failing.
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := other.RunOnce(context.Background())
				assert.NoError(t, err)
			}()
			// Wait until the other pass is blocked on this slot's claim.
			deadline := time.Now().Add(10 * time.Second)
			for !claimBlocked(env) {
				if time.Now().After(deadline) {
					t.Error("the competing pass never reached the claim") // the handler error alone would only fail the slot
					return errors.New("the competing pass never reached the claim")
				}
				time.Sleep(10 * time.Millisecond)
			}
			return errors.New("boom")
		}
		return nil
	})
	def := events.ScheduleDef{ID: "automation:a1:t1", Cron: "* * * * *", TenantID: &tenant, Handler: handler}
	slow := func(c *events.SchedulerConfig) { c.BackoffBase = time.Minute }
	first := goScheduler(t, env, events.StaticSchedules(def), &now, slow)
	other = goScheduler(t, env, events.StaticSchedules(def), &now, slow)
	_, err := first.RunOnce(context.Background())
	require.NoError(t, err)
	wg.Wait()
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 1, calls, "the slot was not run again before its backoff")
	require.Equal(t, events.StatusFailed, statusOf(t, env, def.ID, time.Date(2026, 6, 1, 10, 30, 0, 0, time.UTC)))
}

func TestChangingTheCronResetsTheBacklogCursor(t *testing.T) {
	env := hosttest.New(t)
	h := &goSlots{}
	tenant := env.Tenant
	def := events.ScheduleDef{ID: "automation:a1:t1", Cron: "*/2 * * * *", TenantID: &tenant, Handler: h, MissedAfter: time.Hour,
		ActiveFrom: time.Date(2026, 6, 1, 10, 20, 0, 0, time.UTC)}
	src := &schedules{defs: []events.ScheduleDef{def}}
	now := time.Date(2026, 6, 1, 10, 30, 20, 0, time.UTC)
	s := goScheduler(t, env, src, &now)
	events.SetLiveLimit(s, 2)
	for range 4 {
		_, err := s.RunOnce(context.Background())
		require.NoError(t, err)
	}
	require.Equal(t, 6, countStatus(t, env, def.ID, events.StatusDone), "every second minute from 10:20 to 10:30")

	def.Cron = "* * * * *"
	src.mu.Lock()
	src.defs = []events.ScheduleDef{def}
	src.mu.Unlock()
	for range 10 {
		_, err := s.RunOnce(context.Background())
		require.NoError(t, err)
	}
	require.Equal(t, 11, countStatus(t, env, def.ID, events.StatusDone), "the odd minutes the new recurrence adds run too")
}

// waitFor polls cond on the test goroutine, so the assertions inside it may
// fail the test at once (testify's Eventually runs it on another goroutine).
func waitFor(t *testing.T, cond func() bool, wait, tick time.Duration, msgAndArgs ...any) {
	t.Helper()
	deadline := time.Now().Add(wait)
	for !cond() {
		if time.Now().After(deadline) {
			require.FailNow(t, "condition never satisfied", msgAndArgs...)
		}
		time.Sleep(tick)
	}
}

// claimBlocked reports whether a backend waits on a lock while claiming a
// schedule slot.
func claimBlocked(env *hosttest.Env) bool {
	var n int
	err := env.DB.QueryRow(context.Background(), `
		SELECT count(*) FROM pg_stat_activity
		WHERE datname = current_database() AND wait_event_type = 'Lock' AND query LIKE '%INSERT INTO extruntime.schedule_runs AS r%'`).Scan(&n)
	return err == nil && n > 0
}

func TestDeferredConstraintViolationCountsAsAFailedAttemptAndIsNotRetriedBeforeTheBackoff(t *testing.T) {
	env := hosttest.New(t)
	ctx := context.Background()
	_, err := env.DB.Exec(ctx, `
		CREATE TABLE slot_parents (id int PRIMARY KEY);
		CREATE TABLE slot_children (id int, parent int REFERENCES slot_parents (id) DEFERRABLE INITIALLY DEFERRED)`)
	require.NoError(t, err)
	tenant := env.Tenant
	var mu sync.Mutex
	calls := 0
	def := events.ScheduleDef{ID: "automation:a1:t1", Cron: "* * * * *", TenantID: &tenant,
		Handler: events.SlotHandlerFunc(func(ctx context.Context, tx pgx.Tx, _ events.Slot) error {
			mu.Lock()
			calls++
			mu.Unlock()
			_, err := tx.Exec(ctx, `INSERT INTO slot_children VALUES (1, 404)`) // checked by the scheduler before commit
			return err
		})}
	slot := time.Date(2026, 6, 1, 10, 30, 0, 0, time.UTC)
	now := slot.Add(20 * time.Second)
	s := goScheduler(t, env, events.StaticSchedules(def), &now, func(c *events.SchedulerConfig) { c.BackoffBase = time.Minute })
	n, err := s.RunOnce(ctx)
	require.NoError(t, err)
	require.Zero(t, n)
	var status string
	var attempts int
	require.NoError(t, env.DB.QueryRow(ctx, `SELECT status, attempts FROM extruntime.schedule_runs WHERE tenant_id = $1 AND schedule = $2 AND slot = $3`, env.Tenant, def.ID, slot).Scan(&status, &attempts))
	require.Equal(t, events.StatusFailed, status)
	require.Equal(t, 1, attempts)
	n, err = s.RunOnce(ctx)
	require.NoError(t, err)
	require.Zero(t, n, "the slot is not retried before its backoff")
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 1, calls, "the handler was not called again, not even to fail")
}

func TestCommitFailureClassification(t *testing.T) {
	require.True(t, events.CommitRejected(&pgconn.PgError{Code: "40001"}), "a serialization failure at commit is a confirmed rollback")
	require.True(t, events.CommitRejected(fmt.Errorf("commit: %w", &pgconn.PgError{Code: "23503"})))
	require.True(t, events.CommitRejected(pgx.ErrTxCommitRollback))
	require.False(t, events.CommitRejected(errors.New("conn closed")), "a connection failure leaves the outcome unknown")
	require.False(t, events.CommitRejected(context.DeadlineExceeded))
}

// When the claiming transaction is gone, the failure is recorded outside it,
// but only over the receipt generation this pass claimed: a pass that recorded
// a result meanwhile keeps it, whatever its status, attempts or deadline.
func TestOutOfTransactionFailureDoesNotOverwriteANewerReceipt(t *testing.T) {
	env := hosttest.New(t)
	ctx := context.Background()
	tenant := env.Tenant
	def := events.ScheduleDef{ID: "automation:a1:t1", Cron: "* * * * *", TenantID: &tenant}
	now := time.Date(2026, 6, 1, 10, 30, 20, 0, time.UTC)
	s := goScheduler(t, env, events.StaticSchedules(def), &now)
	slot := time.Date(2026, 6, 1, 10, 30, 0, 0, time.UTC)
	type receipt struct {
		status   string
		attempts int
		msg      string
		gen      time.Time
	}
	row := func() receipt {
		var r receipt
		require.NoError(t, env.DB.QueryRow(ctx, `SELECT status, attempts, last_error, updated_at FROM extruntime.schedule_runs WHERE tenant_id = $1 AND schedule = $2 AND slot = $3`, env.Tenant, def.ID, slot).Scan(&r.status, &r.attempts, &r.msg, &r.gen))
		return r
	}

	// No receipt existed at the claim (nil generation) and still none: inserted.
	require.NoError(t, events.FailSlotOutside(s, ctx, def, env.Tenant, slot, 1, nil, errors.New("rejected")))
	first := row()
	require.Equal(t, events.StatusFailed, first.status)
	gen := first.gen

	// A receipt existed at the claim, and is unchanged: updated.
	require.NoError(t, events.FailSlotOutside(s, ctx, def, env.Tenant, slot, 2, &gen, errors.New("second")))
	second := row()
	require.Equal(t, events.StatusFailed, second.status)
	require.Equal(t, 2, second.attempts)
	require.Contains(t, second.msg, "second")

	// Another pass recorded a blocked state with zero attempts whose deadline
	// has already passed: the late write of the first pass is dropped.
	_, err := env.DB.Exec(ctx, `UPDATE extruntime.schedule_runs SET status = 'blocked', attempts = 0, last_error = 'newer', next_at = now() - interval '1 minute', updated_at = now() WHERE tenant_id = $1 AND schedule = $2 AND slot = $3`, env.Tenant, def.ID, slot)
	require.NoError(t, err)
	require.NoError(t, events.FailSlotOutside(s, ctx, def, env.Tenant, slot, 0, &gen, errors.New("stale")))
	newer := row()
	require.Equal(t, events.StatusBlocked, newer.status, "the newer receipt is untouched")
	require.Equal(t, 0, newer.attempts)
	require.Equal(t, "newer", newer.msg)

	// No receipt at the claim, but one exists now: never touched.
	require.NoError(t, events.FailSlotOutside(s, ctx, def, env.Tenant, slot, 1, nil, errors.New("stale too")))
	require.Equal(t, "newer", row().msg)
}

// A commit PostgreSQL rejects after the handler succeeded is a failed attempt,
// recorded outside the lost transaction, and the slot waits out its backoff.
func TestRejectedCommitIsRecordedAsAFailedAttempt(t *testing.T) {
	env := hosttest.New(t)
	ctx := context.Background()
	tenant := env.Tenant
	h := &goSlots{}
	def := events.ScheduleDef{ID: "automation:a1:t1", Cron: "* * * * *", TenantID: &tenant, Handler: h}
	slot := time.Date(2026, 6, 1, 10, 30, 0, 0, time.UTC)
	now := slot.Add(20 * time.Second)
	s := goScheduler(t, env, events.StaticSchedules(def), &now, func(c *events.SchedulerConfig) { c.BackoffBase = time.Minute })
	events.SetCommit(s, func(ctx context.Context, tx pgx.Tx) error {
		_ = tx.Rollback(ctx) // what PostgreSQL does with a transaction whose commit it rejects
		return &pgconn.PgError{Code: "23503", Message: "insert or update violates foreign key constraint"}
	})
	n, err := s.RunOnce(ctx)
	require.NoError(t, err)
	require.Zero(t, n)
	var status string
	var attempts int
	require.NoError(t, env.DB.QueryRow(ctx, `SELECT status, attempts FROM extruntime.schedule_runs WHERE tenant_id = $1 AND schedule = $2 AND slot = $3`, env.Tenant, def.ID, slot).Scan(&status, &attempts))
	require.Equal(t, events.StatusFailed, status)
	require.Equal(t, 1, attempts)
	_, err = s.RunOnce(ctx)
	require.NoError(t, err)
	require.Len(t, h.slots(), 1, "the handler ran once: the slot waits out its backoff")
}

// The shared limiter registry does not keep closed pools alive.
func TestSharedLimiterRegistryForgetsCollectedPools(t *testing.T) {
	env := hosttest.New(t)
	before := events.ConnSlotPools()
	func() {
		db, err := pgxpool.NewWithConfig(context.Background(), env.DB.Config())
		require.NoError(t, err)
		require.NoError(t, events.AcquireConnSlot(context.Background(), db))
		require.Equal(t, before+1, events.ConnSlotPools())
		db.Close()
	}()
	waitFor(t, func() bool {
		runtime.GC()
		return events.ConnSlotPools() == before
	}, 10*time.Second, 50*time.Millisecond, "the entry of the collected pool is dropped")
}

func TestSlotHandlerCannotFinalizeTheTransaction(t *testing.T) {
	env := hosttest.New(t)
	tenant := env.Tenant
	var commitErr, rollbackErr error
	def := events.ScheduleDef{ID: "automation:a1:t1", Cron: "* * * * *", TenantID: &tenant,
		Handler: events.SlotHandlerFunc(func(ctx context.Context, tx pgx.Tx, slot events.Slot) error {
			if _, err := tx.Exec(ctx, `INSERT INTO host_items (tenant_id, label) VALUES ($1, 'x')`, slot.TenantID); err != nil {
				return err
			}
			commitErr, rollbackErr = tx.Commit(ctx), tx.Rollback(ctx)
			return errors.New("boom")
		})}
	now := time.Date(2026, 6, 1, 10, 30, 20, 0, time.UTC)
	s := goScheduler(t, env, events.StaticSchedules(def), &now)
	_, err := s.RunOnce(context.Background())
	require.NoError(t, err)
	require.ErrorIs(t, commitErr, events.ErrSlotTxOwned)
	require.ErrorIs(t, rollbackErr, events.ErrSlotTxOwned)
	var rows int
	require.NoError(t, env.DB.QueryRow(context.Background(), `SELECT count(*) FROM host_items`).Scan(&rows))
	require.Zero(t, rows, "the write was not made durable by the handler")
}

func TestDueRetriesAreBoundedPerPassAndLeaveRoomForTheCurrentSlot(t *testing.T) {
	env := hosttest.New(t)
	ctx := context.Background()
	h := &goSlots{}
	tenant := env.Tenant
	def := events.ScheduleDef{ID: "automation:a1:t1", Cron: "* * * * *", TenantID: &tenant, Handler: h}
	now := time.Date(2026, 6, 1, 10, 30, 20, 0, time.UTC)
	for _, m := range []int{20, 21, 22} {
		_, err := env.DB.Exec(ctx, `INSERT INTO extruntime.schedule_runs (tenant_id, schedule, slot, status, attempts, next_at) VALUES ($1, $2, $3, 'failed', 1, now() - interval '1 minute')`,
			env.Tenant, def.ID, time.Date(2026, 6, 1, 10, m, 0, 0, time.UTC))
		require.NoError(t, err)
	}
	s := goScheduler(t, env, events.StaticSchedules(def), &now)
	events.SetRetryLimit(s, 2)
	n, err := s.RunOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, 3, n, "two retries and the current slot")
	require.Equal(t, events.StatusDone, statusOf(t, env, def.ID, now.Truncate(time.Minute)))
	n, err = s.RunOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n, "the remaining retry runs on the next pass")
	require.Equal(t, 3, countStatus(t, env, def.ID, events.StatusDone)-1)
}

func TestSubMicrosecondActivationExcludesTheSlotBeforeIt(t *testing.T) {
	env := hosttest.New(t)
	ctx := context.Background()
	h := &goSlots{}
	tenant := env.Tenant
	activeFrom := time.Date(2026, 6, 1, 10, 30, 0, 500, time.UTC) // 10:30:00.0000005
	def := events.ScheduleDef{ID: "automation:a1:t1", Cron: "* * * * *", TenantID: &tenant, Handler: h, ActiveFrom: activeFrom, MissedAfter: time.Hour}
	// A failed receipt of the 10:30 slot from an earlier activation.
	_, err := env.DB.Exec(ctx, `INSERT INTO extruntime.schedule_runs (tenant_id, schedule, slot, status, attempts, next_at) VALUES ($1, $2, $3, 'failed', 1, now() - interval '1 minute')`,
		env.Tenant, def.ID, time.Date(2026, 6, 1, 10, 30, 0, 0, time.UTC))
	require.NoError(t, err)
	now := time.Date(2026, 6, 1, 10, 31, 0, 0, time.UTC)
	s := goScheduler(t, env, events.StaticSchedules(def), &now)
	n, err := s.RunOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n, "only 10:31; the 10:30 slot predates the activation by half a microsecond")
	require.Equal(t, events.StatusFailed, statusOf(t, env, def.ID, time.Date(2026, 6, 1, 10, 30, 0, 0, time.UTC)))
}

func TestBacklogPassStopsAfterAChunkEvenWhenNoSlotCommits(t *testing.T) {
	env := hosttest.New(t)
	h := &goSlots{fail: func(events.Slot) error { return fmt.Errorf("%w: nobody", events.ErrNoSubject) }}
	tenant := env.Tenant
	now := time.Date(2026, 6, 1, 10, 30, 20, 0, time.UTC)
	def := events.ScheduleDef{ID: "automation:a1:t1", Cron: "* * * * *", TenantID: &tenant, Handler: h, MissedAfter: time.Hour,
		ActiveFrom: time.Date(2026, 6, 1, 10, 25, 0, 0, time.UTC)} // six eligible slots
	s := goScheduler(t, env, events.StaticSchedules(def), &now)
	events.SetLiveLimit(s, 2)
	_, err := s.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 4, countStatus(t, env, def.ID, events.StatusBlocked), "the two newest slots and one backlog chunk of two")
}

func TestDueRetriesRotateAcrossTenantsWhenTheSameRunsFailAgain(t *testing.T) {
	env := hosttest.New(t)
	ctx := context.Background()
	tenants := []uuid.UUID{env.Tenant, env.NewTenant(), env.NewTenant(), env.NewTenant()}
	var mu sync.Mutex
	called := map[uuid.UUID]int{}
	def := events.ScheduleDef{ID: "app.tick", Cron: "0 0 1 1 *", Handler: events.SlotHandlerFunc(func(_ context.Context, _ pgx.Tx, slot events.Slot) error {
		mu.Lock()
		defer mu.Unlock()
		called[slot.TenantID]++
		return &host.InfraError{Err: errors.New("database unavailable")} // not counted: it becomes due again at once
	})}
	for _, id := range tenants {
		_, err := env.DB.Exec(ctx, `INSERT INTO extruntime.schedule_runs (tenant_id, schedule, slot, status, attempts, next_at) VALUES ($1, $2, '2026-01-01T00:00:00Z', 'failed', 1, now() - interval '1 minute')`, id, def.ID)
		require.NoError(t, err)
	}
	now := time.Date(2026, 6, 1, 10, 30, 20, 0, time.UTC)
	s := goScheduler(t, env, events.StaticSchedules(def), &now, func(c *events.SchedulerConfig) {
		c.Tenants = &countingTenants{list: tenants}
	})
	events.SetRetryLimit(s, 2)
	for range 2 {
		_, err := s.RunOnce(ctx)
		require.NoError(t, err)
		time.Sleep(50 * time.Millisecond) // the backoff (10ms) elapses: the same runs are due again
	}
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, called, 4, "every tenant was attempted within two passes: %v", called)
}

func TestDeferredConstraintFailureKeepsTheClaimUntilTheBackoffIsDurable(t *testing.T) {
	env := hosttest.New(t)
	ctx := context.Background()
	_, err := env.DB.Exec(ctx, `
		CREATE TABLE slot_parents (id int PRIMARY KEY);
		CREATE TABLE slot_children (id int, parent int REFERENCES slot_parents (id) DEFERRABLE INITIALLY DEFERRED)`)
	require.NoError(t, err)
	tenant := env.Tenant
	var mu sync.Mutex
	calls := 0
	var other *events.Scheduler
	var wg sync.WaitGroup
	now := time.Date(2026, 6, 1, 10, 30, 20, 0, time.UTC)
	handler := events.SlotHandlerFunc(func(ctx context.Context, tx pgx.Tx, _ events.Slot) error {
		mu.Lock()
		calls++
		first := calls == 1
		mu.Unlock()
		if !first {
			return nil
		}
		if _, err := tx.Exec(ctx, `INSERT INTO slot_children VALUES (1, 404)`); err != nil { // checked at commit
			return err
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := other.RunOnce(context.Background())
			assert.NoError(t, err)
		}()
		deadline := time.Now().Add(10 * time.Second)
		for !claimBlocked(env) {
			if time.Now().After(deadline) {
				t.Error("the competing pass never reached the claim")
				return nil
			}
			time.Sleep(10 * time.Millisecond)
		}
		return nil // the handler succeeds; the database refuses its writes
	})
	def := events.ScheduleDef{ID: "automation:a1:t1", Cron: "* * * * *", TenantID: &tenant, Handler: handler}
	slow := func(c *events.SchedulerConfig) { c.BackoffBase = time.Minute }
	first := goScheduler(t, env, events.StaticSchedules(def), &now, slow)
	other = goScheduler(t, env, events.StaticSchedules(def), &now, slow)
	_, err = first.RunOnce(ctx)
	require.NoError(t, err)
	wg.Wait()
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 1, calls, "the slot was not claimed again before its backoff was durable")
	require.Equal(t, events.StatusFailed, statusOf(t, env, def.ID, time.Date(2026, 6, 1, 10, 30, 0, 0, time.UTC)))
}

// A callback that runs after the commit may call the scheduler again even on a
// pool of the minimum size: the pool share is released before it runs.
func TestAfterCommitCallbackCanCallTheSchedulerOnATwoConnectionPool(t *testing.T) {
	env := hosttest.New(t)
	small := *env.DB.Config()
	small.MaxConns = 2
	db, err := pgxpool.NewWithConfig(context.Background(), &small)
	require.NoError(t, err)
	t.Cleanup(db.Close)
	tenant := env.Tenant
	now := time.Date(2026, 6, 1, 10, 30, 20, 0, time.UTC)
	innerDef := events.ScheduleDef{ID: "automation:a1:t1", Cron: "* * * * *", TenantID: &tenant, Handler: &goSlots{}}
	inner, err := events.NewScheduler(events.SchedulerConfig{DB: db, Schedules: events.StaticSchedules(innerDef), Lookback: time.Minute, Now: func() time.Time { return now }, Logger: env.Logger})
	require.NoError(t, err)
	called := false
	// The runner stands in for a package schedule whose result carries an
	// after-commit callback that runs the scheduler synchronously.
	outer, err := events.NewScheduler(events.SchedulerConfig{
		DB: db, Schedules: events.StaticSchedules(events.ScheduleDef{ID: "pkg.tick", Package: "p", Version: "1", Export: "e", Cron: "* * * * *", TenantID: &tenant}),
		Runner: callbackRunner{after: func(ctx context.Context) {
			called = true
			_, err := inner.RunOnce(ctx)
			assert.NoError(t, err)
		}},
		Subjects: env.Subjects, Lookback: time.Minute, Now: func() time.Time { return now }, Logger: env.Logger,
	})
	require.NoError(t, err)
	_, err = env.DB.Exec(context.Background(), `INSERT INTO extruntime.trigger_owners (trigger_id, tenant_id, user_id) VALUES ('pkg.tick', $1, 42)`, env.Tenant)
	require.NoError(t, err)
	env.Subjects.Grant(42)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	n, err := outer.RunOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.True(t, called)
}

type callbackRunner struct{ after func(context.Context) }

func (r callbackRunner) Run(context.Context, host.RunRequest) (host.Result, error) {
	return host.Result{AfterCommit: r.after}, nil
}
