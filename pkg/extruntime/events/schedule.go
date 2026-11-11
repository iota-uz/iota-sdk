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
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	opSchedule  = serrors.Op("extruntime.events.Scheduler.RunOnce")
	opSlots     = serrors.Op("extruntime.events.Slots")
	opScheduleD = serrors.Op("extruntime.events.Scheduler.run")
)

// ScheduleDef is one active schedule export of an activated package version.
type ScheduleDef struct {
	// ID is the application-wide schedule ID; also the trigger_owners ID.
	ID       string
	Package  string
	Version  string
	Export   string
	Cron     string // 5-field expression
	Timezone string // IANA name; default UTC
	Artifact pool.Artifact
	Secrets  []string
	// ActiveFrom is when the package version carrying the schedule was
	// activated. Slots before it never fire, so a freshly activated schedule
	// does not replay the lookback window. Zero means no lower bound.
	ActiveFrom time.Time
}

// ScheduleSource lists the active schedules; the management slice implements
// it over activations.
type ScheduleSource interface {
	ActiveSchedules(ctx context.Context) ([]ScheduleDef, error)
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
	DB        *pgxpool.Pool
	Runner    Runner
	Schedules ScheduleSource
	Tenants   TenantLister
	Subjects  host.SubjectLoader

	// PollInterval between passes. Default 10s.
	PollInterval time.Duration
	// Lookback is how far behind a missed slot is still run (restarts,
	// downtime). Older slots are skipped. Default 15m.
	Lookback    time.Duration
	MaxAttempts int // default 3
	// BackoffBase doubles per attempt; BlockedRecheck re-evaluates runs
	// blocked on a missing owner or refused subject.
	BackoffBase    time.Duration // default 5s
	BlockedRecheck time.Duration // default 30s
	Logger         *slog.Logger
	Now            func() time.Time
}

// Scheduler runs `schedule` exports on cron as the trigger owner of each
// tenant. A slot runs as one short atomic handler: the schedule_runs receipt
// commits in the same transaction as the handler's effects (including
// tx.start of workflows), so a slot is never run twice.
type Scheduler struct {
	cfg SchedulerConfig

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func NewScheduler(cfg SchedulerConfig) (*Scheduler, error) {
	if cfg.DB == nil || cfg.Runner == nil || cfg.Schedules == nil || cfg.Subjects == nil {
		return nil, serrors.New(serrors.Invalid, "extruntime scheduler: DB, Runner, Schedules and Subjects are required")
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
// every tenant, returning the number of handler runs that committed.
func (s *Scheduler) RunOnce(ctx context.Context) (int, error) {
	defs, err := s.cfg.Schedules.ActiveSchedules(ctx)
	if err != nil {
		return 0, serrors.Wrap(opSchedule, err)
	}
	tenants, err := s.cfg.Tenants.Tenants(ctx)
	if err != nil {
		return 0, serrors.Wrap(opSchedule, err)
	}
	now := s.cfg.Now()
	ran := 0
	for _, def := range defs {
		from := now.Add(-s.cfg.Lookback)
		if from.Before(def.ActiveFrom) {
			from = def.ActiveFrom
		}
		slots, err := Slots(def, from, now)
		if err != nil {
			s.cfg.Logger.Error("extruntime: schedule is invalid", "schedule", def.ID, "error", err)
			continue
		}
		if len(slots) == 0 {
			continue
		}
		known, err := s.loadRuns(ctx, def.ID, slots[0])
		if err != nil {
			return ran, err
		}
		for _, slot := range slots {
			for _, tenant := range tenants {
				if st, ok := known[runKey{tenant, slot}]; ok && (!st.due || (st.status != StatusFailed && st.status != StatusBlocked)) {
					continue
				}
				ok, err := s.runSlot(ctx, def, tenant, slot, known[runKey{tenant, slot}].attempts)
				if err != nil {
					return ran, err
				}
				if ok {
					ran++
				}
			}
		}
	}
	return ran, nil
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
	var ownerID int64
	err := s.cfg.DB.QueryRow(ctx, `SELECT user_id FROM extruntime.trigger_owners WHERE trigger_id = $1 AND tenant_id = $2`, def.ID, tenant).Scan(&ownerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, s.record(ctx, def, tenant, slot, StatusBlocked, prior, "", fmt.Sprintf("schedule %s has no owner-user in tenant %s", def.ID, tenant))
	}
	if err != nil {
		return false, serrors.FromDB(opScheduleD, err)
	}
	subject, err := s.cfg.Subjects.LoadSubject(ctx, tenant, ownerID, spi.SubjectOwner)
	if errors.Is(err, host.ErrSubjectBlocked) {
		return false, s.record(ctx, def, tenant, slot, StatusBlocked, prior, "", err.Error())
	}
	if err != nil {
		return false, serrors.Wrap(opScheduleD, err)
	}
	subject.TenantID, subject.UserID, subject.Kind = tenant, ownerID, spi.SubjectOwner

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
	slotISO := slot.UTC().Format(time.RFC3339)
	// slotISO is an RFC 3339 timestamp: it needs no JSON escaping.
	input := json.RawMessage(`{"slot":"` + slotISO + `"}`)
	res, runErr := s.cfg.Runner.Run(ctx, host.RunRequest{
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
	if runErr == nil {
		if res.InvocationID != "" {
			if _, err := tx.Exec(ctx, `UPDATE extruntime.schedule_runs SET invocation_id = $4::uuid WHERE tenant_id = $1 AND schedule = $2 AND slot = $3`,
				tenant, def.ID, slot, res.InvocationID); err != nil {
				return false, serrors.FromDB(opScheduleD, err)
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return false, serrors.FromDB(opScheduleD, err)
		}
		if res.AfterCommit != nil {
			res.AfterCommit(ctx)
		}
		return true, nil
	}
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	_ = tx.Rollback(context.WithoutCancel(ctx))
	status := StatusFailed
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
