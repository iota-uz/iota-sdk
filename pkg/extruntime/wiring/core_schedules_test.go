package wiring_test

import (
	"context"
	"embed"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/extruntimeconfig"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/wiring"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

// scheduleSource is a dynamic source of Go schedules: the schedules of a
// tenant's graphs change while the process runs.
type scheduleSource struct {
	mu   sync.Mutex
	defs []events.ScheduleDef
}

func (s *scheduleSource) set(defs ...events.ScheduleDef) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.defs = defs
}

func (s *scheduleSource) ActiveSchedules(context.Context) ([]events.ScheduleDef, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]events.ScheduleDef(nil), s.defs...), nil
}

type slotRecorder struct {
	mu   sync.Mutex
	runs []events.Slot
}

func (r *slotRecorder) RunSlot(_ context.Context, _ pgx.Tx, slot events.Slot) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runs = append(r.runs, slot)
	return nil
}

func (r *slotRecorder) slots() []events.Slot {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]events.Slot(nil), r.runs...)
}

type scheduleComponent struct{ src events.ScheduleSource }

func (c scheduleComponent) Descriptor() composition.Descriptor {
	return composition.Descriptor{Name: "test.schedules"}
}

func (c scheduleComponent) LocaleFS() []*embed.FS { return nil }

func (c scheduleComponent) Build(b *composition.Builder) error {
	wiring.ContributeSchedules(b, func(*composition.Container) (events.ScheduleSource, error) { return c.src, nil })
	return nil
}

// With the runtime disabled (no EXTRUNTIME_ENABLED, no Node, no
// @iota-uz/sdk) the core's scheduler runs the contributed Go schedules, reads
// the source on every pass and runs a tenant's schedule for that tenant only.
func TestCoreRunsContributedGoSchedulesWithoutTheRuntime(t *testing.T) {
	t.Setenv("IOTA_SDK_DIST", t.TempDir())
	src := &scheduleSource{}
	section := extruntimeconfig.Config{AppRoot: t.TempDir()}
	now := time.Date(2026, 6, 1, 10, 30, 20, 0, time.UTC)
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name: "extruntime_wiring_schedules",
		Components: append(modules.Components(),
			wiring.NewComponent(wiring.Options{
				Natives: modules.Natives(), Section: &section,
				Schedules: events.SchedulerConfig{PollInterval: 50 * time.Millisecond, Lookback: time.Minute, Now: func() time.Time { return now }},
			}),
			scheduleComponent{src: src},
		),
		Database:  itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation: itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	_, err := composition.Resolve[*events.Scheduler](scope.Container)
	require.NoError(t, err)

	rec := &slotRecorder{}
	tenant := scope.Tenant.ID
	src.set(events.ScheduleDef{ID: "automation:a1:t1", Cron: "* * * * *", TenantID: &tenant, Handler: rec})
	require.Eventually(t, func() bool { return len(rec.slots()) == 1 }, 10*time.Second, 20*time.Millisecond,
		"the Go schedule runs with the runtime disabled")
	require.Equal(t, events.Slot{ScheduleID: "automation:a1:t1", TenantID: tenant, At: time.Date(2026, 6, 1, 10, 30, 0, 0, time.UTC)}, rec.slots()[0])

	// A schedule that appears while the scheduler runs fires on the next
	// slot; a removed one stops.
	added := &slotRecorder{}
	now = now.Add(time.Minute)
	src.set(
		events.ScheduleDef{ID: "automation:a1:t1", Cron: "* * * * *", TenantID: &tenant, Handler: rec},
		events.ScheduleDef{ID: "automation:a2:t1", Cron: "* * * * *", TenantID: &tenant, Handler: added},
	)
	require.Eventually(t, func() bool { return len(added.slots()) == 1 && len(rec.slots()) == 2 }, 10*time.Second, 20*time.Millisecond)
	src.set(events.ScheduleDef{ID: "automation:a2:t1", Cron: "* * * * *", TenantID: &tenant, Handler: added})
	now = now.Add(time.Minute)
	require.Eventually(t, func() bool { return len(added.slots()) == 2 }, 10*time.Second, 20*time.Millisecond)
	require.Len(t, rec.slots(), 2, "the removed schedule stopped")

	var n int
	require.NoError(t, scope.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM extruntime.schedule_runs WHERE tenant_id <> $1`, tenant).Scan(&n))
	require.Zero(t, n)
}
