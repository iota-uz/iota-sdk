package jobs

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	// DefaultStaleAfter is how long a running job may go without any update
	// before reads report it as interrupted (worker crashed or restarted).
	DefaultStaleAfter = 10 * time.Minute
	// DefaultConcurrency bounds simultaneously executing handlers per process.
	DefaultConcurrency = 2
	// DefaultListLimit is how many of the user's jobs the operations widget
	// asks for by default.
	DefaultListLimit         = 20
	defaultHeartbeatInterval = time.Minute
	defaultProgressInterval  = 500 * time.Millisecond
)

var (
	// ErrUnknownJobKind is returned when enqueueing a kind nobody registered.
	ErrUnknownJobKind = errors.New("unknown job kind")
	// ErrNotFound is returned when a job does not exist in the caller's scope.
	ErrNotFound = errors.New("job not found")
	// ErrNotRetryable is returned when retrying a job that has not failed.
	ErrNotRetryable = errors.New("job is not retryable")
)

// RunnerOptions tunes the Runner. Zero values fall back to defaults.
type RunnerOptions struct {
	Concurrency int
	// StaleAfter bounds how long a running job may look silent before it is
	// reported as interrupted. Pass the same value to the store via
	// WithStaleAfter when using Redis.
	StaleAfter time.Duration
}

func (o RunnerOptions) withDefaults() RunnerOptions {
	if o.Concurrency <= 0 {
		o.Concurrency = DefaultConcurrency
	}
	if o.StaleAfter <= 0 {
		o.StaleAfter = DefaultStaleAfter
	}
	return o
}

// Runner executes registered job kinds in a bounded in-process goroutine
// pool, persisting snapshots to the Store. Enqueue returns immediately; the
// standard components poll the store for progress.
type Runner struct {
	store    Store
	registry *Registry
	opts     RunnerOptions

	slots chan struct{}
	wg    sync.WaitGroup
}

func NewRunner(store Store, registry *Registry, opts RunnerOptions) *Runner {
	o := opts.withDefaults()
	return &Runner{
		store:    store,
		registry: registry,
		opts:     o,
		slots:    make(chan struct{}, o.Concurrency),
	}
}

// Enqueue validates the kind, stores a queued job owned by the calling user,
// and starts executing it in the background. The HTTP request returns
// immediately.
func (r *Runner) Enqueue(ctx context.Context, kind string, params map[string]any) (Job, error) {
	const op serrors.Op = "jobs.Runner.Enqueue"
	if _, ok := r.registry.Get(kind); !ok {
		return Job{}, serrors.E(op, ErrUnknownJobKind)
	}
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return Job{}, serrors.E(op, err)
	}
	user, err := composables.UseUser(ctx)
	if err != nil {
		return Job{}, serrors.E(op, err)
	}
	if params == nil {
		params = map[string]any{}
	}
	now := time.Now()
	j := Job{
		ID:        uuid.New(),
		TenantID:  tenantID,
		UserID:    user.ID(),
		Kind:      kind,
		Params:    params,
		Status:    StatusQueued,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := r.store.Create(ctx, j); err != nil {
		return Job{}, serrors.E(op, err)
	}
	r.start(ctx, j)
	return j, nil
}

// Get returns a job for its owner. Running jobs that stopped heartbeating are
// marked failed so users see an interrupted operation instead of a spinner
// that never ends.
func (r *Runner) Get(ctx context.Context, id uuid.UUID) (Job, error) {
	const op serrors.Op = "jobs.Runner.Get"
	j, err := r.getOwned(ctx, id)
	if err != nil {
		return Job{}, err
	}
	return r.reapIfStale(ctx, j), nil
}

// ListMine returns the calling user's jobs, newest first.
func (r *Runner) ListMine(ctx context.Context, limit int) ([]Job, error) {
	const op serrors.Op = "jobs.Runner.ListMine"
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	user, err := composables.UseUser(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	if limit <= 0 {
		limit = DefaultListLimit
	}
	jobs, err := r.store.ListByUser(ctx, tenantID, user.ID(), limit)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	for i, j := range jobs {
		jobs[i] = r.reapIfStale(ctx, j)
	}
	return jobs, nil
}

// Retry requeues a failed job so it executes again with the same params.
func (r *Runner) Retry(ctx context.Context, id uuid.UUID) (Job, error) {
	const op serrors.Op = "jobs.Runner.Retry"
	j, err := r.getOwned(ctx, id)
	if err != nil {
		return Job{}, err
	}
	if j.Status != StatusFailed {
		return Job{}, serrors.E(op, ErrNotRetryable)
	}
	now := time.Now()
	j.Status = StatusQueued
	j.Progress = 0
	j.Phase = ""
	j.Error = ""
	j.ResultName = ""
	j.ResultURL = ""
	j.FinishedAt = time.Time{}
	j.UpdatedAt = now
	if err := r.store.Save(ctx, j); err != nil {
		return Job{}, serrors.E(op, err)
	}
	r.start(ctx, j)
	return j, nil
}

// Delete dismisses one of the calling user's jobs and its stored result.
func (r *Runner) Delete(ctx context.Context, id uuid.UUID) error {
	const op serrors.Op = "jobs.Runner.Delete"
	if _, err := r.getOwned(ctx, id); err != nil {
		return err
	}
	if err := r.store.Delete(ctx, id); err != nil {
		return serrors.E(op, err)
	}
	return nil
}

// GetResult streams the stored result bytes for the job's owner.
func (r *Runner) GetResult(ctx context.Context, id uuid.UUID) (string, []byte, error) {
	const op serrors.Op = "jobs.Runner.GetResult"
	if _, err := r.getOwned(ctx, id); err != nil {
		return "", nil, err
	}
	name, data, ok, err := r.store.GetResult(ctx, id)
	if err != nil {
		return "", nil, serrors.E(op, err)
	}
	if !ok {
		return "", nil, serrors.E(op, ErrNotFound)
	}
	return name, data, nil
}

// Wait blocks until all in-flight executions finish. Used for graceful
// shutdown and tests.
func (r *Runner) Wait() {
	r.wg.Wait()
}

func (r *Runner) getOwned(ctx context.Context, id uuid.UUID) (Job, error) {
	const op serrors.Op = "jobs.Runner.getOwned"
	j, ok, err := r.store.Get(ctx, id)
	if err != nil {
		return Job{}, serrors.E(op, err)
	}
	if !ok {
		return Job{}, serrors.E(op, ErrNotFound)
	}
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return Job{}, serrors.E(op, err)
	}
	user, err := composables.UseUser(ctx)
	if err != nil {
		return Job{}, serrors.E(op, err)
	}
	if !j.OwnedBy(tenantID, user.ID()) {
		return Job{}, serrors.E(op, ErrNotFound)
	}
	return j, nil
}

// reapIfStale transitions a silent running job to failed. The snapshot is
// returned either way; the failed write is best-effort.
func (r *Runner) reapIfStale(ctx context.Context, j Job) Job {
	if j.Status != StatusRunning || time.Since(j.UpdatedAt) <= r.opts.StaleAfter {
		return j
	}
	j.Status = StatusFailed
	j.Error = "operation interrupted (worker restarted or timed out)"
	j.FinishedAt = time.Now()
	_ = r.store.Save(ctx, j)
	return j
}

// start launches execution of a queued job in the background.
func (r *Runner) start(ctx context.Context, j Job) {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		select {
		case r.slots <- struct{}{}:
		case <-ctx.Done():
			return
		}
		defer func() { <-r.slots }()
		r.execute(context.WithoutCancel(ctx), j)
	}()
}

func (r *Runner) execute(ctx context.Context, j Job) {
	defer func() {
		if rec := recover(); rec != nil {
			r.finalize(ctx, failedJob(j, "internal error: handler panicked"))
		}
	}()
	handler, ok := r.registry.Get(j.Kind)
	if !ok {
		r.finalize(ctx, failedJob(j, "unknown job kind: "+j.Kind))
		return
	}

	now := time.Now()
	j.Status = StatusRunning
	j.StartedAt = now
	j.UpdatedAt = now
	if err := r.store.Save(ctx, j); err != nil {
		return
	}

	reporter := newReporter(r.store, j.ID, defaultProgressInterval)
	hbStop := startHeartbeat(r.store, j.ID, defaultHeartbeatInterval)
	defer hbStop()

	result, err := handler(ctx, j.Params, reporter)
	reporter.flush()
	hbStop()
	if err != nil {
		r.finalize(ctx, failedJob(j, err.Error()))
		return
	}

	done := j
	done.Status = StatusDone
	done.Progress = 100
	done.Error = ""
	done.FinishedAt = time.Now()
	done.UpdatedAt = done.FinishedAt
	done.ResultURL = result.URL
	if len(result.Data) > 0 {
		name := result.FileName
		if name == "" {
			name = j.Kind + "-result"
		}
		if err := r.store.SaveResult(ctx, j.ID, name, result.Data); err != nil {
			r.finalize(ctx, failedJob(j, "job succeeded but storing the result failed"))
			return
		}
		done.ResultName = name
	}
	r.finalize(ctx, done)
}

func (r *Runner) finalize(ctx context.Context, j Job) {
	// Best-effort: on failure the record keeps its previous state and
	// eventually expires or the user retries.
	_ = r.store.Save(ctx, j)
}

func failedJob(j Job, message string) Job {
	j.Status = StatusFailed
	j.Error = message
	j.FinishedAt = time.Now()
	j.UpdatedAt = j.FinishedAt
	return j
}

// startHeartbeat periodically refreshes the job's UpdatedAt so silent-but-
// alive handlers are not reaped by StaleAfter. Returns a stop function.
func startHeartbeat(store Store, id uuid.UUID, interval time.Duration) (stop func()) {
	done := make(chan struct{})
	var once sync.Once
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				_ = store.Touch(context.Background(), id, time.Now())
			}
		}
	}()
	return func() { once.Do(func() { close(done) }) }
}

// reporter adapts handler progress calls into throttled store writes.
type reporter struct {
	store    Store
	id       uuid.UUID
	lastAt   time.Time
	interval time.Duration
	total    int
	done     int
	percent  int
	phase    string
	dirty    bool
}

func newReporter(store Store, id uuid.UUID, interval time.Duration) *reporter {
	return &reporter{store: store, id: id, interval: interval}
}

func (r *reporter) SetTotal(total int) {
	r.total = total
	r.recompute()
}

func (r *reporter) SetDone(done int) {
	r.done = done
	r.recompute()
}

func (r *reporter) Add(n int) {
	r.done += n
	r.recompute()
}

func (r *reporter) SetPercent(percent int) {
	r.percent = clampPercent(percent)
	r.write(false)
}

func (r *reporter) SetPhase(label string) {
	r.phase = label
	r.write(false)
}

func (r *reporter) recompute() {
	if r.total > 0 {
		r.percent = clampPercent(r.done * 100 / r.total)
	} else {
		r.percent = clampPercent(r.done)
	}
	r.write(true)
}

func (r *reporter) write(throttled bool) {
	r.dirty = true
	now := time.Now()
	if throttled && now.Sub(r.lastAt) < r.interval {
		return
	}
	r.lastAt = now
	r.dirty = false
	_ = r.update()
}

// update persists progress by read-modify-write of the stored snapshot.
func (r *reporter) update() error {
	ctx := context.Background()
	j, ok, err := r.store.Get(ctx, r.id)
	if err != nil || !ok {
		return err
	}
	j.Progress = r.percent
	j.Phase = r.phase
	j.UpdatedAt = time.Now()
	return r.store.Save(ctx, j)
}

func (r *reporter) flush() {
	if r.dirty {
		_ = r.update()
		r.dirty = false
	}
}

func clampPercent(percent int) int {
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return percent
}
