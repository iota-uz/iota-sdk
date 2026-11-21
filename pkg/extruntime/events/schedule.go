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
	// MissedAfter is how late a slot may still run: an older slot that has no
	// run yet is not executed but recorded as skipped. Zero means
	// SchedulerConfig.Lookback. Skipped slots are recorded only for a
	// schedule with an ActiveFrom (see SchedulerConfig.SkipHistory).
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
	Lookback time.Duration
	// SkipHistory bounds how far back skipped slots are recorded: a pass
	// records the slots it skipped within this window (never before the
	// schedule's ActiveFrom), and only for schedules that have an ActiveFrom:
	// without a known start of activity there is no history to account for.
	// Default 24h.
	SkipHistory time.Duration
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
	if cfg.SkipHistory <= 0 {
		cfg.SkipHistory = 24 * time.Hour
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
	return &Scheduler{cfg: cfg}, nil
}

var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

// Slots returns the cron slots of def in (from, to], as UTC instants. The
// expression is evaluated in the schedule's timezone, so "0 9 * * *" in
// Asia/Tashkent is 04:00 UTC.
func Slots(def ScheduleDef, from, to time.Time) ([]time.Time, error) {
	spec, err := cronParser.Parse(def.Cron)
	if err != nil {
		return nil, serrors.Wrap(opSlots, fmt.Errorf("schedule %s: cron %q: %w", def.ID, def.Cron, err))
	}
	loc := time.UTC
	if def.Timezone != "" {
		if loc, err = time.LoadLocation(def.Timezone); err != nil {
			return nil, serrors.Wrap(opSlots, fmt.Errorf("schedule %s: timezone %q: %w", def.ID, def.Timezone, err))
		}
	}
	var out []time.Time
	for t := spec.Next(from.In(loc)); !t.IsZero() && !t.After(to); t = spec.Next(t) {
		out = append(out, t.UTC())
		if len(out) > 10_000 {
			break
		}
	}
	return out, nil
}

type runKey struct {
	tenant uuid.UUID
	slot   time.Time
}

type runState struct {
	status   string
	attempts int
	due      bool // next_at has passed on the database clock
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

// runSchedule runs the due slots of one schedule for tenants. Slots older
// than the schedule's MissedAfter that have no run yet are recorded as
// skipped; slots before ActiveFrom are ignored.
func (s *Scheduler) runSchedule(ctx context.Context, def ScheduleDef, tenants []uuid.UUID, now time.Time) (int, error) {
	if def.ID == "" {
		s.cfg.Logger.Error("extruntime: schedule is invalid", "schedule", "", "error", "empty schedule ID")
		return 0, nil
	}
	missedAfter := def.MissedAfter
	if missedAfter <= 0 {
		missedAfter = s.cfg.Lookback
	}
	cutoff := now.Add(-missedAfter) // slots at or before it are missed
	from := cutoff                  // without ActiveFrom only the slots that can still run
	if !def.ActiveFrom.IsZero() {
		from = now.Add(-max(missedAfter, s.cfg.SkipHistory))
	}
	if from.Before(def.ActiveFrom) {
		from = def.ActiveFrom
	}
	slots, err := Slots(def, from, now)
	if err != nil {
		s.cfg.Logger.Error("extruntime: schedule is invalid", "schedule", def.ID, "error", err)
		return 0, nil
	}
	if len(slots) == 0 {
		return 0, nil
	}
	known, err := s.loadRuns(ctx, def.ID, slots[0])
	if err != nil {
		return 0, err
	}
	ran := 0
	var errs []error
	for _, tenant := range tenants {
		var missed []time.Time
		for _, slot := range slots {
			st, seen := known[runKey{tenant, slot}]
			if !slot.After(cutoff) {
				if !seen {
					missed = append(missed, slot)
				}
				continue
			}
			if seen && (!st.due || (st.status != StatusFailed && st.status != StatusBlocked)) {
				continue
			}
			ok, err := s.runSlot(ctx, def, tenant, slot, st.attempts)
			if err != nil {
				if ctx.Err() != nil {
					return ran, err
				}
				// One tenant's failure (e.g. a tenant that no longer exists)
				// does not stop the others.
				errs = append(errs, fmt.Errorf("tenant %s: %w", tenant, err))
				break
			}
			if ok {
				ran++
			}
		}
		if err := s.recordSkipped(ctx, def, tenant, missed); err != nil {
			if ctx.Err() != nil {
				return ran, err
			}
			errs = append(errs, fmt.Errorf("tenant %s: %w", tenant, err))
		}
	}
	return ran, errors.Join(errs...)
}

// recordSkipped writes the missed slots into the run history. It never
// overwrites an existing run.
func (s *Scheduler) recordSkipped(ctx context.Context, def ScheduleDef, tenant uuid.UUID, slots []time.Time) error {
	if len(slots) == 0 {
		return nil
	}
	_, err := s.cfg.DB.Exec(ctx, `
		INSERT INTO extruntime.schedule_runs (tenant_id, schedule, slot, status, attempts, last_error)
		SELECT $1, $2, slot, 'skipped', 0, $4 FROM unnest($3::timestamptz[]) AS slot
		ON CONFLICT (tenant_id, schedule, slot) DO NOTHING`,
		tenant, def.ID, slots, "slot missed: older than the schedule's missed-slot window")
	if err != nil {
		return serrors.FromDB(opScheduleD, err)
	}
	s.cfg.Logger.Warn("extruntime: schedule slots skipped", "schedule", def.ID, "tenant", tenant, "count", len(slots), "first", slots[0], "last", slots[len(slots)-1])
	return nil
}

func (s *Scheduler) loadRuns(ctx context.Context, schedule string, since time.Time) (map[runKey]runState, error) {
	rows, err := s.cfg.DB.Query(ctx, `SELECT tenant_id, slot, status, attempts, next_at <= now() FROM extruntime.schedule_runs WHERE schedule = $1 AND slot >= $2`, schedule, since)
	if err != nil {
		return nil, serrors.FromDB(opSchedule, err)
	}
	defer rows.Close()
	out := map[runKey]runState{}
	for rows.Next() {
		var k runKey
		var st runState
		if err := rows.Scan(&k.tenant, &k.slot, &st.status, &st.attempts, &st.due); err != nil {
			return nil, serrors.FromDB(opSchedule, err)
		}
		k.slot = k.slot.UTC()
		out[k] = st
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

	tx, err := s.cfg.DB.Begin(ctx)
	if err != nil {
		return false, serrors.FromDB(opScheduleD, err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
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
	var (
		res    host.Result
		runErr error
	)
	if def.Handler != nil {
		runErr = callHandler(ctx, def.Handler, tx, Slot{ScheduleID: def.ID, TenantID: tenant, At: slot.UTC()})
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
		if res.InvocationID != "" {
			if _, err := tx.Exec(ctx, `UPDATE extruntime.schedule_runs SET invocation_id = $4::uuid WHERE tenant_id = $1 AND schedule = $2 AND slot = $3`,
				tenant, def.ID, slot, res.InvocationID); err != nil {
				return false, serrors.FromDB(opScheduleD, err)
			}
		}
		err := tx.Commit(ctx)
		switch {
		case err == nil:
			if res.AfterCommit != nil {
				res.AfterCommit(ctx)
			}
			return true, nil
		case errors.Is(err, pgx.ErrTxCommitRollback) && ctx.Err() == nil:
			// A handler that swallowed a failed statement left the transaction
			// aborted: the slot failed.
			runErr = fmt.Errorf("schedule %s: the slot's transaction was rolled back: %w", def.ID, err)
		default:
			return false, serrors.FromDB(opScheduleD, err)
		}
	}
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	_ = tx.Rollback(context.WithoutCancel(ctx))
	status := StatusFailed
	if errors.Is(runErr, ErrNoSubject) || errors.Is(runErr, ErrRunnerUnavailable) {
		// Nobody to act as, or nothing to run a package schedule with: the slot
		// waits instead of failing, like a blocked delivery.
		return false, s.record(ctx, def, tenant, slot, StatusBlocked, prior, "", runErr.Error())
	}
	if host.IsInfrastructure(runErr) {
		// A failure of the platform is not the handler's: the slot is retried
		// after the backoff without counting as an attempt.
		attempts = max(attempts-1, 0)
	} else if attempts >= s.cfg.MaxAttempts {
		status = StatusDead
	}
	msg := runErr.Error()
	if we, ok := pool.AsWireError(runErr); ok {
		msg = we.Error()
	}
	return false, s.record(ctx, def, tenant, slot, status, attempts, res.InvocationID, msg)
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

// record stores a non-delivered run state (failed, dead or blocked) outside
// the rolled back handler transaction; it never overwrites a committed run.
func (s *Scheduler) record(ctx context.Context, def ScheduleDef, tenant uuid.UUID, slot time.Time, status string, attempts int, invocationID, msg string) error {
	delay := s.cfg.BlockedRecheck
	if status != StatusBlocked {
		delay = s.cfg.BackoffBase
		for i := 1; i < attempts; i++ {
			delay *= 2
		}
	} else {
		attempts = max(attempts, 0)
	}
	var inv any
	if invocationID != "" {
		inv = invocationID
	}
	_, err := s.cfg.DB.Exec(context.WithoutCancel(ctx), `
		INSERT INTO extruntime.schedule_runs AS r (tenant_id, schedule, slot, status, attempts, next_at, last_error, invocation_id)
		VALUES ($1, $2, $3, $4, $5, now() + $6::interval, $7, $8::uuid)
		ON CONFLICT (tenant_id, schedule, slot) DO UPDATE
		SET status = EXCLUDED.status, attempts = EXCLUDED.attempts, next_at = EXCLUDED.next_at, last_error = EXCLUDED.last_error,
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
