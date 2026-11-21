package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/robfig/cron/v3"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/triggerowner"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	opSchedule  = serrors.Op("extruntime.events.Scheduler.RunOnce")
	opSlots     = serrors.Op("extruntime.events.Slots")
	opScheduleD = serrors.Op("extruntime.events.Scheduler.run")
)

// ErrRunnerUnavailable is matched by a Runner error that means there is no
// extension runtime to run a package schedule with (the runtime is not
// enabled). The slot is recorded as blocked with the error as its diagnostic
// and re-evaluated every BlockedRecheck without counting attempts.
var ErrRunnerUnavailable = errors.New("extruntime events: no runner")

// SlotHandler runs one slot of a Go schedule (ScheduleDef.Handler).
type SlotHandler interface {
	// RunSlot runs in tx together with the schedule_runs receipt of (schedule,
	// tenant, slot): the receipt and the handler's effects commit atomically,
	// so a slot is never run twice. An error rolls both back and the slot is
	// retried with backoff up to SchedulerConfig.MaxAttempts; an error that
	// wraps ErrNoSubject blocks the slot instead (it is re-evaluated every
	// BlockedRecheck without counting attempts). The handler chooses who it
	// acts as, e.g. through the trigger owner of def.ID in slot.TenantID.
	//
	// The scheduler owns the end of tx: Commit and Rollback fail with
	// ErrSlotTxOwned, so a handler cannot make its writes durable before it
	// returns. A nested tx.Begin (a savepoint) is allowed.
	RunSlot(ctx context.Context, tx pgx.Tx, slot Slot) error
}

// SlotHandlerFunc adapts a function to SlotHandler.
type SlotHandlerFunc func(ctx context.Context, tx pgx.Tx, slot Slot) error

func (f SlotHandlerFunc) RunSlot(ctx context.Context, tx pgx.Tx, slot Slot) error {
	return f(ctx, tx, slot)
}

// Slot is one planned run of a schedule for a tenant.
type Slot struct {
	ScheduleID string
	TenantID   uuid.UUID
	// At is the planned time of the slot in UTC.
	At time.Time
}

// ScheduleDef is one active schedule: an export of an activated package
// version, or a Go schedule with a Handler.
type ScheduleDef struct {
	// ID is the application-wide schedule ID: the schedule_runs key and, for a
	// package schedule, the trigger_owners ID. A consumer that owns a dynamic
	// schedule keeps it stable for the pair it belongs to (for example
	// "automation:<automation_id>:<trigger_id>"), so a new version of the
	// definition does not repeat slots that already ran.
	ID string
	// Package, Version, Export, Artifact and Secrets describe a package
	// schedule; they are not used when Handler is set.
	Package  string
	Version  string
	Export   string
	Artifact pool.Artifact
	Secrets  []string
	Cron     string // 5-field expression
	Timezone string // IANA name; default UTC
	// ActiveFrom is when the schedule became active (for a package schedule,
	// when its version was activated). Slots before it never fire, so a
	// freshly enabled schedule does not catch up on the past. Zero means no
	// lower bound.
	ActiveFrom time.Time
	// TenantID restricts the schedule to one tenant; nil runs it for every
	// tenant (the package schedules, which are application-wide). A schedule
	// with a TenantID never consults the TenantLister.
	TenantID *uuid.UUID
	// MissedAfter is how late a slot may still run (a slot exactly this old
	// still does): a strictly older slot that has no run is not executed but
	// recorded as skipped. A slot that already ran and failed keeps being
	// retried up to MaxAttempts whatever its age; a blocked one expires into
	// skipped once it is older than MissedAfter. Zero means
	// SchedulerConfig.Lookback.
	//
	// The scheduler remembers, per schedule and tenant, how far the past is
	// accounted for (extruntime.schedule_accounting), so every slot missed
	// since ActiveFrom (or, without ActiveFrom, since the schedule was first
	// seen) is recorded, however long the downtime, a batch per pass.
	MissedAfter time.Duration
	// Handler runs the slot in Go. When set the scheduler looks up no
	// trigger_owners and resolves no subject: the handler picks its run-as.
	Handler SlotHandler
}

// ScheduleSource lists the active schedules; it is read on every pass, so a
// schedule added or removed between passes takes effect without a restart.
// The management slice implements it over activations. A source may return
// schedules together with an error (a composed source one of whose parts
// failed): the scheduler runs the schedules it got and reports the error.
type ScheduleSource interface {
	ActiveSchedules(ctx context.Context) ([]ScheduleDef, error)
}

// ScheduleSourceFunc adapts a function to ScheduleSource.
type ScheduleSourceFunc func(ctx context.Context) ([]ScheduleDef, error)

func (f ScheduleSourceFunc) ActiveSchedules(ctx context.Context) ([]ScheduleDef, error) {
	return f(ctx)
}

// StaticSchedules is a ScheduleSource of a fixed list.
func StaticSchedules(defs ...ScheduleDef) ScheduleSource {
	return ScheduleSourceFunc(func(context.Context) ([]ScheduleDef, error) {
		return append([]ScheduleDef(nil), defs...), nil
	})
}

// ComposeSchedules concatenates sources in order; nil sources are skipped. A
// failing source does not hide the others: the schedules of every source are
// returned with the errors of the failed ones joined. A schedule ID that two
// sources both list is an error and only its first definition is kept.
func ComposeSchedules(srcs ...ScheduleSource) ScheduleSource {
	var list []ScheduleSource
	for _, s := range srcs {
		if s != nil {
			list = append(list, s)
		}
	}
	return ScheduleSourceFunc(func(ctx context.Context) ([]ScheduleDef, error) {
		var (
			out  []ScheduleDef
			errs []error
			seen = map[string]struct{}{}
		)
		for _, s := range list {
			defs, err := s.ActiveSchedules(ctx)
			if err != nil {
				errs = append(errs, err)
			}
			for _, d := range defs {
				if _, dup := seen[d.ID]; dup {
					errs = append(errs, fmt.Errorf("extruntime schedules: duplicate schedule ID %q", d.ID))
					continue
				}
				seen[d.ID] = struct{}{}
				out = append(out, d)
			}
		}
		return out, errors.Join(errs...)
	})
}

// TenantLister lists the tenants schedules run for. Activation is
// application-wide; each tenant runs the schedule as its own trigger owner.
type TenantLister interface {
	Tenants(ctx context.Context) ([]uuid.UUID, error)
}

// PgTenants lists every tenant of the application.
type PgTenants struct{ DB *pgxpool.Pool }

func (t PgTenants) Tenants(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := t.DB.Query(ctx, `SELECT id FROM tenants ORDER BY id`)
	if err != nil {
		return nil, serrors.FromDB(opSchedule, err)
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, serrors.FromDB(opSchedule, err)
		}
		out = append(out, id)
	}
	return out, serrors.FromDB(opSchedule, rows.Err())
}

// SchedulerConfig configures the scheduler. Zero values select the defaults.
type SchedulerConfig struct {
	DB *pgxpool.Pool
	// Runner executes package schedules; it is optional. Without it (or when
	// it returns ErrRunnerUnavailable) only the schedules with a Handler run,
	// and a package schedule's slots are recorded as blocked with a
	// diagnostic.
	Runner    Runner
	Schedules ScheduleSource
	Tenants   TenantLister
	// Subjects resolves the owner of a package schedule; required with Runner.
	Subjects host.SubjectLoader

	// PollInterval between passes. Default 10s.
	PollInterval time.Duration
	// Lookback is how far behind a missed slot is still run (restarts,
	// downtime) unless the schedule sets MissedAfter. Older slots are
	// recorded as skipped. Default 15m.
	Lookback    time.Duration
	MaxAttempts int // default 3
	// BackoffBase doubles per attempt; BlockedRecheck re-evaluates runs
	// blocked on a missing owner or refused subject.
	BackoffBase    time.Duration // default 5s
	BlockedRecheck time.Duration // default 30s
	Logger         *slog.Logger
	Now            func() time.Time
}

// Scheduler runs schedules on cron: `schedule` exports of packages, as the
// trigger owner of each tenant, and Go schedules through their SlotHandler. A
// slot runs as one short atomic handler: the schedule_runs receipt commits in
// the same transaction as the handler's effects (including tx.start of
// workflows), so a slot is never run twice. The schedules are read from the
// source on every pass.
type Scheduler struct {
	cfg SchedulerConfig
	// liveLimit bounds the live slots one pass enumerates.
	liveLimit int
	// retryLimit bounds the due retries one pass handles per schedule.
	retryLimit int
	// backlog remembers, per schedule and tenant, how far the eligible slots
	// older than the newest live ones are all claimed (see runBacklog).
	cmu     sync.Mutex
	backlog map[backlogKey]time.Time

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func NewScheduler(cfg SchedulerConfig) (*Scheduler, error) {
	if cfg.DB == nil || cfg.Schedules == nil {
		return nil, serrors.New(serrors.Invalid, "extruntime scheduler: DB and Schedules are required")
	}
	if cfg.Runner != nil && cfg.Subjects == nil {
		return nil, serrors.New(serrors.Invalid, "extruntime scheduler: Subjects are required with a Runner")
	}
	if cfg.Tenants == nil {
		cfg.Tenants = PgTenants{DB: cfg.DB}
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 10 * time.Second
	}
	if cfg.Lookback <= 0 {
		cfg.Lookback = 15 * time.Minute
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 3
	}
	if cfg.BackoffBase <= 0 {
		cfg.BackoffBase = 5 * time.Second
	}
	if cfg.BlockedRecheck <= 0 {
		cfg.BlockedRecheck = 30 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Scheduler{cfg: cfg, liveLimit: defaultLiveLimit, retryLimit: defaultRetryLimit, backlog: map[backlogKey]time.Time{}}, nil
}

var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

// parseSchedule parses the cron expression and the time zone of def.
func parseSchedule(def ScheduleDef) (cron.Schedule, *time.Location, error) {
	spec, err := cronParser.Parse(def.Cron)
	if err != nil {
		return nil, nil, serrors.Wrap(opSlots, fmt.Errorf("schedule %s: cron %q: %w", def.ID, def.Cron, err))
	}
	loc := time.UTC
	if def.Timezone != "" {
		if loc, err = time.LoadLocation(def.Timezone); err != nil {
			return nil, nil, serrors.Wrap(opSlots, fmt.Errorf("schedule %s: timezone %q: %w", def.ID, def.Timezone, err))
		}
	}
	return spec, loc, nil
}

// Slots returns the cron slots of def in (from, to], as UTC instants. The
// expression is evaluated in the schedule's timezone, so "0 9 * * *" in
// Asia/Tashkent is 04:00 UTC. At most 10,001 slots are returned, the oldest.
func Slots(def ScheduleDef, from, to time.Time) ([]time.Time, error) {
	spec, loc, err := parseSchedule(def)
	if err != nil {
		return nil, err
	}
	out, _ := slotsAfter(spec, loc, from, to, 10_001)
	return out, nil
}

// slotsAfter lists the slots in (from, to], oldest first, at most limit of
// them; more reports that the range holds more.
func slotsAfter(spec cron.Schedule, loc *time.Location, from, to time.Time, limit int) ([]time.Time, bool) {
	var out []time.Time
	for t := spec.Next(from.In(loc)); !t.IsZero() && !t.After(to); t = spec.Next(t) {
		if len(out) == limit {
			return out, true
		}
		out = append(out, t.UTC())
	}
	return out, false
}

const (
	// tick is the smallest step of a time.Time: it turns the inclusive lower
	// bounds of the contract into the exclusive ones slotsAfter takes, exactly.
	tick = time.Nanosecond
	// defaultRetryLimit bounds the due retries one pass handles per schedule,
	// tenants taking turns; the rest stay due for the next passes.
	defaultRetryLimit = 1000
	// historyBatch bounds the skipped slots one pass records per schedule and
	// tenant, so a long downtime is accounted for over several passes without
	// delaying the live slots.
	historyBatch = 5000
	// defaultLiveLimit bounds the live slots one pass enumerates.
	defaultLiveLimit = 100_000
)

// backlogKey identifies a backlog cursor by the schedule's recurrence too: a
// cursor is only valid for the slots of the cron and zone that produced it.
type backlogKey struct {
	schedule, cron, timezone string
	tenant                   uuid.UUID
}

type runKey struct {
	tenant uuid.UUID
	slot   time.Time
}

// RunOnce evaluates every active schedule once and runs the due slots of
// every tenant the schedule applies to, returning the number of handler runs
// that committed. The schedules are read from the source here, so a schedule
// that appeared or disappeared since the previous pass is taken into account.
// A failing schedule does not stop the others: the errors are joined.
func (s *Scheduler) RunOnce(ctx context.Context) (int, error) {
	defs, srcErr := s.cfg.Schedules.ActiveSchedules(ctx)
	if srcErr != nil {
		srcErr = serrors.Wrap(opSchedule, srcErr)
		if len(defs) == 0 {
			return 0, srcErr
		}
	}
	var (
		now     = s.cfg.Now()
		ran     = 0
		errs    = []error{srcErr}
		tenants []uuid.UUID
		listed  bool
	)
	allTenants := func() ([]uuid.UUID, error) {
		if !listed {
			list, err := s.cfg.Tenants.Tenants(ctx)
			if err != nil {
				return nil, serrors.Wrap(opSchedule, err)
			}
			tenants, listed = list, true
		}
		return tenants, nil
	}
	for _, def := range defs {
		if err := ctx.Err(); err != nil {
			return ran, errors.Join(append(errs, err)...)
		}
		var scoped []uuid.UUID
		switch {
		case def.TenantID != nil && *def.TenantID == uuid.Nil:
			s.cfg.Logger.Error("extruntime: schedule is invalid", "schedule", def.ID, "error", "tenant ID is the nil UUID")
			continue
		case def.TenantID != nil:
			scoped = []uuid.UUID{*def.TenantID}
		default:
			list, err := allTenants()
			if err != nil {
				errs = append(errs, err)
				continue
			}
			scoped = list
		}
		n, err := s.runSchedule(ctx, def, scoped, now)
		ran += n
		if err != nil {
			if ctx.Err() != nil {
				return ran, errors.Join(append(errs, err)...)
			}
			errs = append(errs, fmt.Errorf("schedule %s: %w", def.ID, err))
		}
	}
	return ran, errors.Join(errs...)
}

// retryable is a failed or blocked run that is due for another attempt.
type retryable struct {
	tenant   uuid.UUID
	slot     time.Time
	status   string
	attempts int
}

// runSchedule runs one schedule for tenants. It handles three disjoint sets,
// so none can starve another:
//
//   - due retries of runs that already have a receipt (failed ones whatever
//     their age, up to MaxAttempts; blocked ones until they are older than
//     the missed window, then they expire into skipped);
//   - the live slots, those from the missed window that have no receipt;
//   - the past: slots older than the window and not yet accounted for are
//     recorded as skipped, a batch per pass, from the accounted watermark.
//
// A slot before ActiveFrom is in none of them.
func (s *Scheduler) runSchedule(ctx context.Context, def ScheduleDef, tenants []uuid.UUID, now time.Time) (int, error) {
	if def.ID == "" {
		s.cfg.Logger.Error("extruntime: schedule is invalid", "schedule", "", "error", "empty schedule ID")
		return 0, nil
	}
	spec, loc, err := parseSchedule(def)
	if err != nil {
		s.cfg.Logger.Error("extruntime: schedule is invalid", "schedule", def.ID, "error", err)
		return 0, nil
	}
	missedAfter := def.MissedAfter
	if missedAfter <= 0 {
		missedAfter = s.cfg.Lookback
	}
	cutoff := now.Add(-missedAfter) // a slot at or after it may still run
	liveFrom := cutoff
	if liveFrom.Before(def.ActiveFrom) {
		liveFrom = def.ActiveFrom
	}
	live, headFrom := s.liveSlots(spec, loc, liveFrom, now)
	var known map[runKey]struct{}
	if len(live) > 0 {
		if known, err = s.loadKnown(ctx, def.ID, live[0], now, tenants); err != nil {
			return 0, err
		}
	}
	retries, err := s.loadRetries(ctx, def.ID, tenants, def.ActiveFrom)
	if err != nil {
		return 0, err
	}
	accounted, err := s.loadAccounted(ctx, def.ID, tenants)
	if err != nil {
		return 0, err
	}

	var (
		ran  int
		errs []error
		bad  = map[uuid.UUID]bool{} // tenants whose run failed on the database
	)
	fail := func(tenant uuid.UUID, err error) {
		errs = append(errs, fmt.Errorf("tenant %s: %w", tenant, err))
		bad[tenant] = true
	}
	for _, r := range retries {
		if bad[r.tenant] {
			continue
		}
		if r.status == StatusBlocked && r.slot.Before(cutoff) {
			if err := s.expireBlocked(ctx, def, r.tenant, r.slot); err != nil {
				if ctx.Err() != nil {
					return ran, err
				}
				fail(r.tenant, err)
			}
			continue
		}
		ok, err := s.runSlot(ctx, def, r.tenant, r.slot, r.attempts)
		if err != nil {
			if ctx.Err() != nil {
				return ran, err
			}
			fail(r.tenant, err)
			continue
		}
		if ok {
			ran++
		}
	}
	for _, tenant := range tenants {
		for _, slot := range live {
			if bad[tenant] {
				break
			}
			if _, seen := known[runKey{tenant, slot}]; seen {
				continue
			}
			ok, err := s.runSlot(ctx, def, tenant, slot, 0)
			if err != nil {
				if ctx.Err() != nil {
					return ran, err
				}
				fail(tenant, err)
				break
			}
			if ok {
				ran++
			}
		}
		if bad[tenant] {
			continue
		}
		if headFrom.After(liveFrom) {
			n, err := s.runBacklog(ctx, def, spec, loc, tenant, liveFrom, headFrom)
			ran += n
			if err != nil {
				if ctx.Err() != nil {
					return ran, err
				}
				fail(tenant, err)
				continue
			}
		}
		if err := s.accountPast(ctx, def, spec, loc, tenant, accounted, cutoff); err != nil {
			if ctx.Err() != nil {
				return ran, err
			}
			fail(tenant, err)
		}
	}
	return ran, errors.Join(errs...)
}

// liveSlots lists the newest live slots, those in [from, now], at most the
// live limit of them. When the range holds more, headFrom is where the listed
// slots start and the older eligible slots in [from, headFrom) are left to
// runBacklog; otherwise headFrom is from. The current slot is never starved by
// a backlog.
func (s *Scheduler) liveSlots(spec cron.Schedule, loc *time.Location, from, now time.Time) ([]time.Time, time.Time) {
	headFrom := from
	for {
		slots, more := slotsAfter(spec, loc, headFrom.Add(-tick), now, s.liveLimit)
		if !more {
			return slots, headFrom
		}
		headFrom = now.Add(-now.Sub(headFrom) / 2)
	}
}

// runBacklog runs the eligible slots in [from, until) that no run has claimed
// yet, one chunk of at most the live limit per pass, oldest first. A cursor
// per schedule and tenant remembers that everything before it is claimed, so
// the passes make progress through a backlog instead of revisiting it.
func (s *Scheduler) runBacklog(ctx context.Context, def ScheduleDef, spec cron.Schedule, loc *time.Location, tenant uuid.UUID, from, until time.Time) (int, error) {
	key := backlogKey{schedule: def.ID, cron: def.Cron, timezone: def.Timezone, tenant: tenant}
	s.cmu.Lock()
	cursor, ok := s.backlog[key]
	s.cmu.Unlock()
	if !ok || cursor.Before(from) {
		cursor = from
	}
	ran := 0
	for cursor.Before(until) {
		chunk, more := slotsAfter(spec, loc, cursor.Add(-tick), until.Add(-tick), s.liveLimit)
		if len(chunk) == 0 {
			break
		}
		known, err := s.loadKnown(ctx, def.ID, chunk[0], chunk[len(chunk)-1], []uuid.UUID{tenant})
		if err != nil {
			return ran, err
		}
		attempted := 0
		for _, slot := range chunk {
			if _, seen := known[runKey{tenant, slot}]; seen {
				continue
			}
			attempted++
			ok, err := s.runSlot(ctx, def, tenant, slot, 0)
			if err != nil {
				return ran, err
			}
			if ok {
				ran++
			}
		}
		cursor = chunk[len(chunk)-1].Add(tick)
		s.cmu.Lock()
		s.backlog[key] = cursor
		s.cmu.Unlock()
		if attempted > 0 || !more {
			break // one chunk of work per pass, whether or not its runs committed
		}
	}
	return ran, nil
}

// accountPast records the slots of tenant that are older than the missed
// window, have no run and lie after the accounted watermark, as skipped, and
// moves the watermark. The watermark starts at ActiveFrom; without ActiveFrom
// there is no known start, so it starts where the schedule is first seen.
func (s *Scheduler) accountPast(ctx context.Context, def ScheduleDef, spec cron.Schedule, loc *time.Location, tenant uuid.UUID, accounted map[uuid.UUID]time.Time, cutoff time.Time) error {
	through, have := accounted[tenant]
	initial := false
	if !have {
		if def.ActiveFrom.IsZero() {
			through, initial = cutoff.Add(-tick), true
		} else {
			through = def.ActiveFrom.Add(-tick)
		}
	}
	if from := def.ActiveFrom.Add(-tick); !def.ActiveFrom.IsZero() && from.After(through) {
		through = from // enabled again later than it was accounted
	}
	slots, _ := slotsAfter(spec, loc, through, cutoff.Add(-tick), historyBatch)
	if len(slots) == 0 {
		if !initial {
			return nil
		}
	} else {
		through = slots[len(slots)-1]
	}
	tx, err := s.cfg.DB.Begin(ctx)
	if err != nil {
		return serrors.FromDB(opScheduleD, err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	var (
		inserted                  int
		firstSkipped, lastSkipped *time.Time
	)
	if len(slots) > 0 {
		if err := tx.QueryRow(ctx, `
			WITH ins AS (
				INSERT INTO extruntime.schedule_runs (tenant_id, schedule, slot, status, attempts, last_error)
				SELECT $1, $2, slot, 'skipped', 0, $4 FROM unnest($3::timestamptz[]) AS slot
				ON CONFLICT (tenant_id, schedule, slot) DO NOTHING
				RETURNING slot)
			SELECT count(*), min(slot), max(slot) FROM ins`,
			tenant, def.ID, slots, "slot missed: older than the schedule's missed-slot window").Scan(&inserted, &firstSkipped, &lastSkipped); err != nil {
			return serrors.FromDB(opScheduleD, err)
		}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO extruntime.schedule_accounting (tenant_id, schedule, accounted_through)
		VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id, schedule) DO UPDATE
		SET accounted_through = GREATEST(schedule_accounting.accounted_through, EXCLUDED.accounted_through)`,
		tenant, def.ID, through); err != nil {
		return serrors.FromDB(opScheduleD, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return serrors.FromDB(opScheduleD, err)
	}
	if inserted > 0 { // slots that already ran keep their receipt and are not reported
		s.cfg.Logger.Warn("extruntime: schedule slots skipped", "schedule", def.ID, "tenant", tenant, "count", inserted, "first", firstSkipped, "last", lastSkipped)
	}
	return nil
}

// expireBlocked turns a blocked run that outlived the missed window into
// skipped, keeping its diagnostic.
func (s *Scheduler) expireBlocked(ctx context.Context, def ScheduleDef, tenant uuid.UUID, slot time.Time) error {
	_, err := s.cfg.DB.Exec(ctx, `
		UPDATE extruntime.schedule_runs
		SET status = 'skipped', last_error = last_error || ' (expired: older than the schedule''s missed-slot window)', updated_at = now()
		WHERE tenant_id = $1 AND schedule = $2 AND slot = $3 AND status = 'blocked'`, tenant, def.ID, slot)
	if err != nil {
		return serrors.FromDB(opScheduleD, err)
	}
	return nil
}

// loadKnown lists the slots of tenants in [since, until] that have any run.
func (s *Scheduler) loadKnown(ctx context.Context, schedule string, since, until time.Time, tenants []uuid.UUID) (map[runKey]struct{}, error) {
	rows, err := s.cfg.DB.Query(ctx, `
		SELECT tenant_id, slot FROM extruntime.schedule_runs
		WHERE tenant_id = ANY($3) AND schedule = $1 AND slot >= $2 AND slot <= $4`, schedule, since, tenants, until)
	if err != nil {
		return nil, serrors.FromDB(opSchedule, err)
	}
	defer rows.Close()
	out := map[runKey]struct{}{}
	for rows.Next() {
		var k runKey
		if err := rows.Scan(&k.tenant, &k.slot); err != nil {
			return nil, serrors.FromDB(opSchedule, err)
		}
		k.slot = k.slot.UTC()
		out[k] = struct{}{}
	}
	return out, serrors.FromDB(opSchedule, rows.Err())
}

// loadRetries lists the failed and blocked runs of tenants that are due,
// whatever the age of their slot but not from before activeFrom (a schedule
// enabled again under the same ID does not retry the past of its earlier
// activation). At most the retry limit are returned, the tenants taking turns,
// oldest slots first; the rest stay due for later passes.
func (s *Scheduler) loadRetries(ctx context.Context, schedule string, tenants []uuid.UUID, activeFrom time.Time) ([]retryable, error) {
	// The column has microseconds: round the bound up so a slot before an
	// activation with sub-microsecond digits is not admitted.
	if !activeFrom.IsZero() {
		if t := activeFrom.Truncate(time.Microsecond); t.Before(activeFrom) {
			activeFrom = t.Add(time.Microsecond)
		}
	}
	rows, err := s.cfg.DB.Query(ctx, `
		SELECT tenant_id, slot, status, attempts FROM (
			SELECT tenant_id, slot, status, attempts,
			       row_number() OVER (PARTITION BY tenant_id ORDER BY slot) AS turn
			FROM extruntime.schedule_runs
			WHERE tenant_id = ANY($2) AND schedule = $1 AND status IN ('failed', 'blocked') AND next_at <= now()
			  AND ($3::timestamptz IS NULL OR slot >= $3)) due
		ORDER BY turn, slot, tenant_id
		LIMIT $4`, schedule, tenants, nullTime(activeFrom), s.retryLimit)
	if err != nil {
		return nil, serrors.FromDB(opSchedule, err)
	}
	defer rows.Close()
	var out []retryable
	for rows.Next() {
		var r retryable
		if err := rows.Scan(&r.tenant, &r.slot, &r.status, &r.attempts); err != nil {
			return nil, serrors.FromDB(opSchedule, err)
		}
		r.slot = r.slot.UTC()
		out = append(out, r)
	}
	return out, serrors.FromDB(opSchedule, rows.Err())
}

// nullTime is NULL for a zero time.
func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

// loadAccounted lists how far the past of the schedule is accounted for, per
// tenant of tenants.
func (s *Scheduler) loadAccounted(ctx context.Context, schedule string, tenants []uuid.UUID) (map[uuid.UUID]time.Time, error) {
	rows, err := s.cfg.DB.Query(ctx, `SELECT tenant_id, accounted_through FROM extruntime.schedule_accounting WHERE tenant_id = ANY($2) AND schedule = $1`, schedule, tenants)
	if err != nil {
		return nil, serrors.FromDB(opSchedule, err)
	}
	defer rows.Close()
	out := map[uuid.UUID]time.Time{}
	for rows.Next() {
		var id uuid.UUID
		var t time.Time
		if err := rows.Scan(&id, &t); err != nil {
			return nil, serrors.FromDB(opSchedule, err)
		}
		out[id] = t.UTC()
	}
	return out, serrors.FromDB(opSchedule, rows.Err())
}

func (s *Scheduler) runSlot(ctx context.Context, def ScheduleDef, tenant uuid.UUID, slot time.Time, prior int) (bool, error) {
	var (
		subject spi.Subject
		ownerID int64
	)
	if def.Handler == nil {
		if s.cfg.Runner == nil {
			return false, s.record(ctx, def, tenant, slot, StatusBlocked, prior, "",
				fmt.Sprintf("schedule %s is a package schedule and the extension runtime is not enabled", def.ID))
		}
		var found bool
		var err error
		ownerID, found, err = triggerowner.Lookup(ctx, s.cfg.DB, def.ID, tenant)
		if err != nil {
			return false, serrors.Wrap(opScheduleD, err)
		}
		if !found {
			return false, s.record(ctx, def, tenant, slot, StatusBlocked, prior, "", fmt.Sprintf("schedule %s has no owner-user in tenant %s", def.ID, tenant))
		}
		subject, err = s.cfg.Subjects.LoadSubject(ctx, tenant, ownerID, spi.SubjectOwner)
		if errors.Is(err, host.ErrSubjectBlocked) {
			return false, s.record(ctx, def, tenant, slot, StatusBlocked, prior, "", err.Error())
		}
		if err != nil {
			return false, serrors.Wrap(opScheduleD, err)
		}
		subject.TenantID, subject.UserID, subject.Kind = tenant, ownerID, spi.SubjectOwner
	}

	release, err := acquireConnSlot(ctx, s.cfg.DB)
	if err != nil {
		return false, err
	}
	defer release()
	tx, err := s.cfg.DB.Begin(ctx)
	if err != nil {
		return false, serrors.FromDB(opScheduleD, err)
	}
	txOpen := true
	defer func() {
		if txOpen {
			_ = tx.Rollback(context.WithoutCancel(ctx))
		}
	}()
	var attempts int
	err = tx.QueryRow(ctx, `
		INSERT INTO extruntime.schedule_runs AS r (tenant_id, schedule, slot, status, attempts)
		VALUES ($1, $2, $3, 'done', 1)
		ON CONFLICT (tenant_id, schedule, slot) DO UPDATE
		SET status = 'done', attempts = r.attempts + 1, last_error = '', updated_at = now()
		WHERE r.status IN ('failed', 'blocked') AND r.next_at <= now()
		RETURNING attempts`, tenant, def.ID, slot).Scan(&attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil // another node ran it, or it is not due for retry yet
	}
	if err != nil {
		return false, serrors.FromDB(opScheduleD, err)
	}
	// The handler runs under a savepoint. A failure rolls back to it and
	// records the failed state in this same transaction, so the claim on the
	// slot is held until the retry time is durable: a concurrent pass cannot
	// run the slot again before the backoff.
	if _, err := tx.Exec(ctx, `SAVEPOINT schedule_slot`); err != nil {
		return false, serrors.FromDB(opScheduleD, err)
	}
	var (
		res    host.Result
		runErr error
	)
	if def.Handler != nil {
		runErr = callHandler(ctx, def.Handler, slotTx{Tx: tx}, Slot{ScheduleID: def.ID, TenantID: tenant, At: slot.UTC()})
	} else {
		slotISO := slot.UTC().Format(time.RFC3339)
		// slotISO is an RFC 3339 timestamp: it needs no JSON escaping.
		input := json.RawMessage(`{"slot":"` + slotISO + `"}`)
		res, runErr = s.cfg.Runner.Run(ctx, host.RunRequest{
			Export: host.Export{
				Kind:     pool.KindSchedule,
				Resource: def.ID,
				Binding: host.Binding{
					Contract: def.ID, Package: def.Package, Version: def.Version, Export: def.Export,
					Artifact: def.Artifact, Secrets: def.Secrets,
				},
			},
			Input:        input,
			Subject:      subject,
			Trigger:      host.Trigger{Kind: host.TriggerSchedule, Ref: def.ID},
			Tx:           tx,
			StartKeyBase: "schedule:" + def.ID + ":" + slotISO,
		})
	}
	if runErr == nil {
		if _, err := tx.Exec(ctx, `RELEASE SAVEPOINT schedule_slot`); err != nil && ctx.Err() == nil {
			// A handler that swallowed a failed statement left the transaction
			// aborted: the slot failed.
			runErr = fmt.Errorf("schedule %s: the slot's transaction was rolled back: %w", def.ID, err)
		}
	}
	if runErr == nil {
		if res.InvocationID != "" {
			if _, err := tx.Exec(ctx, `UPDATE extruntime.schedule_runs SET invocation_id = $4::uuid WHERE tenant_id = $1 AND schedule = $2 AND slot = $3`,
				tenant, def.ID, slot, res.InvocationID); err != nil {
				return false, serrors.FromDB(opScheduleD, err)
			}
		}
		err := tx.Commit(ctx)
		if err == nil {
			txOpen = false
			if res.AfterCommit != nil {
				res.AfterCommit(ctx)
			}
			return true, nil
		}
		if ctx.Err() != nil || !commitRejected(err) {
			return false, serrors.FromDB(opScheduleD, err) // the outcome may be unknown: do not count an attempt
		}
		// PostgreSQL rejected the commit (a deferred constraint, say): the
		// handler's writes and the claim are gone, and the slot failed.
		txOpen = false
		runErr = fmt.Errorf("schedule %s: the slot's transaction was rejected at commit: %w", def.ID, err)
		return false, s.failSlot(ctx, nil, def, tenant, slot, attempts, res.InvocationID, runErr)
	}
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	if err := s.failSlot(ctx, tx, def, tenant, slot, attempts, res.InvocationID, runErr); err != nil {
		return false, err
	}
	txOpen = false
	return false, nil
}

// commitRejected reports whether err is PostgreSQL refusing a commit, which
// leaves the transaction rolled back, as opposed to a failure of the
// connection, after which the outcome is unknown.
func commitRejected(err error) bool {
	var pg *pgconn.PgError
	return errors.Is(err, pgx.ErrTxCommitRollback) || errors.As(err, &pg)
}

// slotTx is the transaction a Go slot handler receives. The scheduler owns
// its end: the handler works inside it, and its Commit and Rollback fail, so a
// handler can neither make its writes durable before returning nor keep the
// receipt from rolling back with them.
type slotTx struct{ pgx.Tx }

// ErrSlotTxOwned is returned by Commit and Rollback of the transaction a
// SlotHandler receives.
var ErrSlotTxOwned = errors.New("extruntime events: the slot transaction is committed or rolled back by the scheduler")

func (slotTx) Commit(context.Context) error   { return ErrSlotTxOwned }
func (slotTx) Rollback(context.Context) error { return ErrSlotTxOwned }

// failSlot classifies a failed run and stores the state. With tx (the slot's
// open transaction) it rolls back to the savepoint, keeping the claim, and
// commits the failed state with it; without one, or when that fails, it
// records the state outside.
func (s *Scheduler) failSlot(ctx context.Context, tx pgx.Tx, def ScheduleDef, tenant uuid.UUID, slot time.Time, attempts int, invocationID string, runErr error) error {
	status, failedAttempts := StatusFailed, attempts
	msg := runErr.Error()
	switch {
	case errors.Is(runErr, ErrNoSubject) || errors.Is(runErr, ErrRunnerUnavailable):
		// Nobody to act as, or nothing to run a package schedule with: the slot
		// waits instead of failing, like a blocked delivery. The attempts stay
		// as the claim found them (it counted this one).
		status, failedAttempts = StatusBlocked, max(attempts-1, 0)
	case host.IsInfrastructure(runErr):
		// A failure of the platform is not the handler's: the slot is retried
		// after the backoff without counting as an attempt.
		failedAttempts = max(attempts-1, 0)
	case attempts >= s.cfg.MaxAttempts:
		status = StatusDead
	}
	if status != StatusBlocked {
		if we, ok := pool.AsWireError(runErr); ok {
			msg = we.Error()
		}
	}
	if tx != nil {
		// Roll the handler back to the savepoint and keep the claim: the failed
		// state commits with it.
		wc := context.WithoutCancel(ctx)
		if _, err := tx.Exec(wc, `ROLLBACK TO SAVEPOINT schedule_slot`); err == nil {
			var inv any
			if invocationID != "" {
				inv = invocationID
			}
			_, err = tx.Exec(wc, `
				UPDATE extruntime.schedule_runs
				SET status = $4, attempts = $5, next_at = clock_timestamp() + $6::interval, last_error = $7,
				    invocation_id = COALESCE($8::uuid, invocation_id), updated_at = now()
				WHERE tenant_id = $1 AND schedule = $2 AND slot = $3`,
				tenant, def.ID, slot, status, failedAttempts, s.backoff(status, failedAttempts).String(), msg, inv)
			if err == nil {
				if err = tx.Commit(wc); err == nil {
					s.cfg.Logger.Warn("extruntime: schedule slot did not run", "schedule", def.ID, "tenant", tenant, "slot", slot, "status", status, "reason", msg)
					return nil
				}
			}
		}
		// The transaction cannot carry the state (a broken connection): record
		// it outside, as a last resort.
		_ = tx.Rollback(wc)
	}
	return s.record(ctx, def, tenant, slot, status, failedAttempts, invocationID, msg)
}

// backoff is the delay before a failed or blocked slot is due again.
func (s *Scheduler) backoff(status string, attempts int) time.Duration {
	if status == StatusBlocked {
		return s.cfg.BlockedRecheck
	}
	delay := s.cfg.BackoffBase
	for i := 1; i < attempts; i++ {
		delay *= 2
	}
	return delay
}

// callHandler runs a Go slot handler, turning a panic into an error so a
// faulty handler fails its own slot, not the process.
func callHandler(ctx context.Context, h SlotHandler, tx pgx.Tx, slot Slot) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("schedule %s: handler panicked: %v", slot.ScheduleID, r)
		}
	}()
	return h.RunSlot(ctx, tx, slot)
}

// record stores a non-delivered run state (failed, dead, blocked) outside any
// slot transaction: a slot that was blocked before it was claimed, or the last
// resort when the claiming transaction itself failed. It never overwrites a
// committed run.
func (s *Scheduler) record(ctx context.Context, def ScheduleDef, tenant uuid.UUID, slot time.Time, status string, attempts int, invocationID, msg string) error {
	delay := s.backoff(status, attempts)
	attempts = max(attempts, 0)
	var inv any
	if invocationID != "" {
		inv = invocationID
	}
	_, err := s.cfg.DB.Exec(context.WithoutCancel(ctx), `
		INSERT INTO extruntime.schedule_runs AS r (tenant_id, schedule, slot, status, attempts, next_at, last_error, invocation_id)
		VALUES ($1, $2, $3, $4, $5, now() + $6::interval, $7, $8::uuid)
		ON CONFLICT (tenant_id, schedule, slot) DO UPDATE
		SET status = EXCLUDED.status,
		    attempts = CASE WHEN EXCLUDED.status = 'blocked' THEN r.attempts ELSE EXCLUDED.attempts END, next_at = EXCLUDED.next_at, last_error = EXCLUDED.last_error,
		    invocation_id = COALESCE(EXCLUDED.invocation_id, r.invocation_id), updated_at = now()
		WHERE r.status IN ('failed', 'blocked')`,
		tenant, def.ID, slot, status, attempts, delay.String(), msg, inv)
	if err != nil {
		return serrors.FromDB(opScheduleD, err)
	}
	s.cfg.Logger.Warn("extruntime: schedule slot did not run", "schedule", def.ID, "tenant", tenant, "slot", slot, "status", status, "reason", msg)
	return nil
}

// Start runs passes every PollInterval until Stop or ctx ends.
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		return
	}
	ctx, s.cancel = context.WithCancel(ctx)
	s.done = make(chan struct{})
	go func(done chan struct{}) {
		defer close(done)
		t := time.NewTicker(s.cfg.PollInterval)
		defer t.Stop()
		for {
			if _, err := s.RunOnce(ctx); err != nil && ctx.Err() == nil {
				s.cfg.Logger.Error("extruntime: schedule pass failed", "error", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}(s.done)
}

// Stop ends Start's loop and waits for the in-flight pass.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	cancel, done := s.cancel, s.done
	s.cancel, s.done = nil, nil
	s.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	<-done
}
