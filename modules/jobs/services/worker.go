package services

import (
	"bytes"
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/jobs/domain/aggregates/job"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/appconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/uploadsconfig"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

const (
	DefaultConcurrency      = 2
	DefaultPollInterval     = time.Second
	DefaultRetention        = 7 * 24 * time.Hour
	DefaultCleanupInterval  = time.Hour
	defaultProgressInterval = 500 * time.Millisecond
	defaultSweepSize        = 500
)

// WorkerOptions tunes the background worker pool. Zero values fall back to
// the package defaults.
type WorkerOptions struct {
	Concurrency     int
	PollInterval    time.Duration
	Retention       time.Duration
	CleanupInterval time.Duration
}

func (o WorkerOptions) withDefaults() WorkerOptions {
	if o.Concurrency <= 0 {
		o.Concurrency = DefaultConcurrency
	}
	if o.PollInterval <= 0 {
		o.PollInterval = DefaultPollInterval
	}
	if o.Retention <= 0 {
		o.Retention = DefaultRetention
	}
	if o.CleanupInterval <= 0 {
		o.CleanupInterval = DefaultCleanupInterval
	}
	return o
}

// UploadStore is the slice of the upload service the worker depends on.
// *coreservices.UploadService satisfies it; tests substitute fakes.
type UploadStore interface {
	Create(ctx context.Context, data *upload.CreateDTO) (upload.Upload, error)
	Delete(ctx context.Context, id uint) (upload.Upload, error)
}

// Worker is the background job executor. It polls the jobs table for queued
// work, executes handlers in a bounded pool, persists throttled progress
// updates, stores result files via the upload service, and expires terminal
// jobs once they pass the retention window.
type Worker struct {
	pool       *pgxpool.Pool
	repo       job.Repository
	registry   *Registry
	uploads    UploadStore
	uploadsCfg *uploadsconfig.Config
	httpCfg    *httpconfig.Config
	appCfg     *appconfig.Config
	logger     *logrus.Logger
	opts       WorkerOptions
}

func NewWorker(
	pool *pgxpool.Pool,
	repo job.Repository,
	registry *Registry,
	uploads UploadStore,
	uploadsCfg *uploadsconfig.Config,
	httpCfg *httpconfig.Config,
	appCfg *appconfig.Config,
	logger *logrus.Logger,
	opts WorkerOptions,
) *Worker {
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	return &Worker{
		pool:       pool,
		repo:       repo,
		registry:   registry,
		uploads:    uploads,
		uploadsCfg: uploadsCfg,
		httpCfg:    httpCfg,
		appCfg:     appCfg,
		logger:     logger,
		opts:       opts.withDefaults(),
	}
}

// Start runs the claim/execute and retention loops until ctx is cancelled.
func (w *Worker) Start(ctx context.Context) error {
	systemCtx := composables.WithPool(ctx, w.pool)

	var wg sync.WaitGroup
	slots := make(chan struct{}, w.opts.Concurrency)

	poll := time.NewTicker(w.opts.PollInterval)
	defer poll.Stop()
	cleanup := time.NewTicker(w.opts.CleanupInterval)
	defer cleanup.Stop()

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return nil
		case <-poll.C:
			for {
				claimed, ok, err := w.repo.ClaimNext(systemCtx)
				if err != nil {
					w.logger.WithError(err).Error("jobs: claim failed")
					break
				}
				if !ok {
					break
				}
				select {
				case slots <- struct{}{}:
				case <-ctx.Done():
					wg.Wait()
					return nil
				}
				wg.Add(1)
				go func(j job.Job) {
					defer wg.Done()
					defer func() { <-slots }()
					defer func() {
						if rec := recover(); rec != nil {
							w.logger.WithField("job_id", j.ID().String()).Errorf("jobs: handler panicked: %v", rec)
							w.finalize(j.MarkFailed("internal error: handler panicked"))
						}
					}()
					w.execute(systemCtx, j)
				}(claimed)
			}
		case <-cleanup.C:
			w.sweep(systemCtx)
		}
	}
}

func (w *Worker) execute(ctx context.Context, j job.Job) {
	handler, ok := w.registry.Get(j.Kind())
	if !ok {
		// Kinds register during component startup, before the HTTP server
		// starts accepting enqueue requests, so this only happens when a
		// deployment is miswired. Fail visibly — the Retry action covers the
		// case where the handler arrives with the next deploy.
		w.finalize(j.MarkFailed("unknown job kind: " + j.Kind()))
		return
	}

	jobCtx := w.jobContext(ctx, j)
	reporter := newPersistingReporter(w.repo, jobCtx, j, defaultProgressInterval)
	result, err := handler(jobCtx, j.Params(), reporter)
	reporter.flush()
	if err != nil {
		w.finalize(j.MarkFailed(err.Error()))
		return
	}

	var uploadID *uint
	if len(result.Data) > 0 {
		name := result.FileName
		if name == "" {
			name = j.Kind() + "-result"
		}
		created, uploadErr := w.uploads.Create(jobCtx, &upload.CreateDTO{
			File:        bytes.NewReader(result.Data),
			Name:        name,
			Size:        len(result.Data),
			UploadsPath: w.uploadsPath(),
			Domain:      w.httpDomain(),
			Scheme:      w.httpScheme(),
		})
		if uploadErr != nil {
			w.logger.WithError(uploadErr).WithField("job_id", j.ID().String()).Error("jobs: storing result upload failed")
			w.finalize(j.MarkFailed("job succeeded but storing the result file failed"))
			return
		}
		id := created.ID()
		uploadID = &id
	}
	w.finalize(j.MarkDone(uploadID))
}

func (w *Worker) finalize(j job.Job) {
	ctx := w.jobContext(context.Background(), j)
	ctx = composables.WithPool(ctx, w.pool)
	if _, err := w.repo.Save(ctx, j); err != nil {
		w.logger.WithError(err).WithField("job_id", j.ID().String()).Error("jobs: persisting terminal status failed")
	}
}

// sweep expires terminal jobs past the retention window and deletes their
// result files.
func (w *Worker) sweep(ctx context.Context) {
	cutoff := time.Now().Add(-w.opts.Retention)
	expired, err := w.repo.ListFinishedBefore(ctx, cutoff, defaultSweepSize)
	if err != nil {
		w.logger.WithError(err).Error("jobs: retention sweep query failed")
		return
	}
	for _, j := range expired {
		if uploadID := j.ResultUploadID(); uploadID != nil {
			jobCtx := w.jobContext(ctx, j)
			if _, err := w.uploads.Delete(jobCtx, *uploadID); err != nil {
				w.logger.WithError(err).WithField("job_id", j.ID().String()).Warn("jobs: deleting expired result upload failed")
			}
		}
		if err := w.repo.Delete(w.jobContext(ctx, j), j.ID()); err != nil {
			w.logger.WithError(err).WithField("job_id", j.ID().String()).Error("jobs: deleting expired job failed")
		}
	}
	if len(expired) > 0 {
		w.logger.WithField("count", len(expired)).Info("jobs: retention sweep removed terminal jobs")
	}
}

// jobContext derives a tenant-scoped context so repositories and the upload
// service work from the worker.
func (w *Worker) jobContext(ctx context.Context, j job.Job) context.Context {
	return composables.WithTenantID(ctx, j.TenantID())
}

func (w *Worker) uploadsPath() string {
	if w.uploadsCfg == nil {
		return ""
	}
	return w.uploadsCfg.Path
}

func (w *Worker) httpDomain() string {
	if w.httpCfg == nil {
		return ""
	}
	return w.httpCfg.Domain
}

func (w *Worker) httpScheme() string {
	if w.appCfg == nil {
		return ""
	}
	return w.appCfg.Scheme()
}

// persistingReporter adapts handler progress calls into throttled repository
// writes. Percent changes are written at most once per interval; SetPercent
// and phase changes always write through.
type persistingReporter struct {
	repo     job.Repository
	ctx      context.Context
	id       uuid.UUID
	lastAt   time.Time
	interval time.Duration
	total    int
	done     int
	percent  int
	phase    string
	dirty    bool
}

func newPersistingReporter(repo job.Repository, ctx context.Context, j job.Job, interval time.Duration) *persistingReporter {
	return &persistingReporter{
		repo:     repo,
		ctx:      ctx,
		id:       j.ID(),
		interval: interval,
	}
}

func (r *persistingReporter) SetTotal(total int) {
	r.total = total
	r.recompute()
}

func (r *persistingReporter) SetDone(done int) {
	r.done = done
	r.recompute()
}

func (r *persistingReporter) Add(n int) {
	r.done += n
	r.recompute()
}

func (r *persistingReporter) SetPercent(percent int) {
	r.percent = clampPercent(percent)
	r.write(false)
}

func (r *persistingReporter) SetPhase(label string) {
	r.phase = label
	r.write(false)
}

func (r *persistingReporter) recompute() {
	if r.total > 0 {
		r.percent = clampPercent(r.done * 100 / r.total)
	} else {
		r.percent = clampPercent(r.done)
	}
	r.write(true)
}

func (r *persistingReporter) write(throttled bool) {
	r.dirty = true
	now := time.Now()
	if throttled && now.Sub(r.lastAt) < r.interval {
		return
	}
	r.lastAt = now
	r.dirty = false
	_ = r.repo.UpdateProgress(r.ctx, r.id, r.percent, r.phase)
}

// flush persists any pending throttled update.
func (r *persistingReporter) flush() {
	if r.dirty {
		_ = r.repo.UpdateProgress(r.ctx, r.id, r.percent, r.phase)
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
