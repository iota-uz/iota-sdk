package events_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
)

type schedules struct {
	mu   sync.Mutex
	defs []events.ScheduleDef
}

func (s *schedules) ActiveSchedules(context.Context) ([]events.ScheduleDef, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]events.ScheduleDef{}, s.defs...), nil
}

func (f *fixture) scheduler(src *schedules, now *time.Time, mods ...func(*events.SchedulerConfig)) *events.Scheduler {
	f.t.Helper()
	cfg := events.SchedulerConfig{
		DB: f.env.DB, Runner: f.host, Schedules: src, Subjects: f.env.Subjects,
		Lookback: time.Minute, BackoffBase: 10 * time.Millisecond, BlockedRecheck: 30 * time.Millisecond, MaxAttempts: 3,
		Logger: f.env.Logger, Now: func() time.Time { return *now },
	}
	for _, m := range mods {
		m(&cfg)
	}
	s, err := events.NewScheduler(cfg)
	require.NoError(f.t, err)
	f.t.Cleanup(s.Stop)
	return s
}

func (f *fixture) scheduleDef(id, source string) events.ScheduleDef {
	art := f.env.Artifacts.Add(f.t, "acme.sched."+id, "1.0.0", prelude+source)
	return events.ScheduleDef{ID: "acme.sched." + id, Package: "acme.sched." + id, Version: "1.0.0", Export: "handler", Cron: "* * * * *", Artifact: art}
}

func (f *fixture) allItems() []string {
	rows, err := f.env.DB.Query(context.Background(), `SELECT label FROM host_items ORDER BY id`)
	require.NoError(f.t, err)
	defer rows.Close()
	var out []string
	for rows.Next() {
		var l string
		require.NoError(f.t, rows.Scan(&l))
		out = append(out, l)
	}
	return out
}

func TestSlotsEvaluateTheCronExpressionInTheScheduleTimezone(t *testing.T) {
	def := events.ScheduleDef{ID: "acme.s.daily", Cron: "0 9 * * *", Timezone: "Asia/Tashkent"}
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	slots, err := events.Slots(def, from, from.Add(48*time.Hour))
	require.NoError(t, err)
	require.Equal(t, []time.Time{
		time.Date(2026, 6, 1, 4, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 2, 4, 0, 0, 0, time.UTC),
	}, slots, "09:00 in Tashkent (UTC+5) is 04:00 UTC")
	_, err = events.Slots(events.ScheduleDef{ID: "x", Cron: "not a cron"}, from, from)
	require.Error(t, err)
	_, err = events.Slots(events.ScheduleDef{ID: "x", Cron: "* * * * *", Timezone: "Nowhere/City"}, from, from)
	require.Error(t, err)
}

func TestScheduleRunsAsTheTenantOwnerOncePerSlotAndBlocksTenantsWithoutOwner(t *testing.T) {
	f := newFixture(t)
	other := f.env.NewTenant()
	f.env.Subjects.Grant(42)
	f.env.Subjects.Grant(43)
	def := f.scheduleDef("tick", `export const handler = async (ctx) => {
  await ctx.transaction(async (tx) => { await tx.insert(item, { label: "slot:" + ctx.slot + ":" + tx.user.kind + ":" + tx.user.userId }); });
};`)
	src := &schedules{defs: []events.ScheduleDef{def}}
	now := time.Date(2026, 6, 1, 10, 30, 20, 0, time.UTC)
	_, err := f.env.DB.Exec(context.Background(), `INSERT INTO extruntime.trigger_owners (trigger_id, tenant_id, user_id) VALUES ($1, $2, 42)`, def.ID, f.env.Tenant)
	require.NoError(t, err)

	a, b := f.scheduler(src, &now), f.scheduler(src, &now)
	var wg sync.WaitGroup
	for _, s := range []*events.Scheduler{a, b} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.RunOnce(context.Background())
			require.NoError(t, err)
		}()
	}
	wg.Wait()
	require.Equal(t, []string{"slot:2026-06-01T10:30:00Z:owner:42"}, f.allItems(), "two schedulers ran the slot once, as the owner-user")

	var status, diag string
	require.NoError(t, f.env.DB.QueryRow(context.Background(), `SELECT status, last_error FROM extruntime.schedule_runs WHERE tenant_id = $1 AND schedule = $2`, other, def.ID).Scan(&status, &diag))
	require.Equal(t, events.StatusBlocked, status, "a tenant without an owner gets a blocked run, not a failure")
	require.Contains(t, diag, "no owner-user")

	_, err = f.env.DB.Exec(context.Background(), `INSERT INTO extruntime.trigger_owners (trigger_id, tenant_id, user_id) VALUES ($1, $2, 43)`, def.ID, other)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		_, err := a.RunOnce(context.Background())
		require.NoError(t, err)
		return len(f.allItems()) == 2
	}, 10*time.Second, 40*time.Millisecond)
	require.Contains(t, f.allItems(), "slot:2026-06-01T10:30:00Z:owner:43")

	now = now.Add(time.Minute) // the next slot
	n, err := b.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, n, "one run per tenant for the new slot; earlier slots are not repeated")
	require.Len(t, f.allItems(), 4)
}

func TestFailedScheduleRunRollsBackItsEffectsAndRetriesThenDeadLetters(t *testing.T) {
	f := newFixture(t)
	f.env.Subjects.Grant(42)
	def := f.scheduleDef("boom", `export const handler = async (ctx) => {
  await ctx.transaction(async (tx) => { await tx.insert(item, { label: "partial" }); });
  throw new Error("handler failed");
};`)
	_, err := f.env.DB.Exec(context.Background(), `INSERT INTO extruntime.trigger_owners (trigger_id, tenant_id, user_id) VALUES ($1, $2, 42)`, def.ID, f.env.Tenant)
	require.NoError(t, err)
	src := &schedules{defs: []events.ScheduleDef{def}}
	now := time.Date(2026, 6, 1, 10, 30, 20, 0, time.UTC)
	s := f.scheduler(src, &now)
	require.Eventually(t, func() bool {
		_, err := s.RunOnce(context.Background())
		require.NoError(t, err)
		var status string
		var attempts int
		err = f.env.DB.QueryRow(context.Background(), `SELECT status, attempts FROM extruntime.schedule_runs WHERE tenant_id = $1 AND schedule = $2`, f.env.Tenant, def.ID).Scan(&status, &attempts)
		return err == nil && status == events.StatusDead && attempts == 3
	}, 10*time.Second, 30*time.Millisecond)
	require.Empty(t, f.allItems(), "the handler's transaction and the receipt rolled back together")
}

func TestRolledBackContextTransactionCannotBeFollowedByASuccessfulRun(t *testing.T) {
	f := newFixture(t)
	f.env.Subjects.Grant(42)
	def := f.scheduleDef("swallow", `export const handler = async (ctx) => {
  try { await ctx.transaction(async (tx) => { await tx.insert(item, { label: "x" }); throw new Error("inner"); }); } catch {}
};`)
	_, err := f.env.DB.Exec(context.Background(), `INSERT INTO extruntime.trigger_owners (trigger_id, tenant_id, user_id) VALUES ($1, $2, 42)`, def.ID, f.env.Tenant)
	require.NoError(t, err)
	now := time.Date(2026, 6, 1, 10, 30, 20, 0, time.UTC)
	s := f.scheduler(&schedules{defs: []events.ScheduleDef{def}}, &now)
	n, err := s.RunOnce(context.Background())
	require.NoError(t, err)
	require.Zero(t, n)
	var status, msg string
	require.NoError(t, f.env.DB.QueryRow(context.Background(), `SELECT status, last_error FROM extruntime.schedule_runs WHERE tenant_id = $1`, f.env.Tenant).Scan(&status, &msg))
	require.Equal(t, events.StatusFailed, status)
	require.Contains(t, msg, "rolled back")
	require.Empty(t, f.allItems())
}
