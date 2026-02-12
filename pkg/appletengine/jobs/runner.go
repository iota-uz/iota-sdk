package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

type dispatcher interface {
	DispatchJob(ctx context.Context, appletID, tenantID, jobID, method string, params any) error
}

type Runner struct {
	pool       *pgxpool.Pool
	dispatcher dispatcher
	logger     *logrus.Logger
	interval   time.Duration
}

type queuedJob struct {
	TenantID string
	AppletID string
	JobID    string
	Method   string
	Params   any
}

func NewRunner(pool *pgxpool.Pool, dispatcher dispatcher, logger *logrus.Logger, interval time.Duration) (*Runner, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}
	if dispatcher == nil {
		return nil, fmt.Errorf("dispatcher is required")
	}
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	if interval <= 0 {
		interval = 2 * time.Second
	}
	return &Runner{pool: pool, dispatcher: dispatcher, logger: logger, interval: interval}, nil
}

func (r *Runner) Start(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		if err := r.poll(ctx); err != nil {
			r.logger.WithError(err).Error("applet jobs runner poll failed")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *Runner) poll(ctx context.Context) error {
	jobs, err := r.claimQueuedJobs(ctx, 16)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if err := r.dispatcher.DispatchJob(ctx, job.AppletID, job.TenantID, job.JobID, job.Method, job.Params); err != nil {
			r.logger.WithFields(logrus.Fields{"applet": job.AppletID, "job_id": job.JobID}).WithError(err).Error("applet queued job failed")
			_ = r.setJobStatus(ctx, job, "failed")
			continue
		}
		if err := r.setJobStatus(ctx, job, "completed"); err != nil {
			r.logger.WithFields(logrus.Fields{"applet": job.AppletID, "job_id": job.JobID}).WithError(err).Error("failed to mark applet job as completed")
		}
	}
	return nil
}

func (r *Runner) claimQueuedJobs(ctx context.Context, limit int) ([]queuedJob, error) {
	rows, err := r.pool.Query(ctx, `
		WITH next_jobs AS (
			SELECT tenant_id, applet_id, job_id, method_name, params
			FROM applet_engine_jobs
			WHERE status = 'queued'
			ORDER BY created_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE applet_engine_jobs j
		SET status = 'running', updated_at = NOW()
		FROM next_jobs
		WHERE j.tenant_id = next_jobs.tenant_id
		  AND j.applet_id = next_jobs.applet_id
		  AND j.job_id = next_jobs.job_id
		RETURNING next_jobs.tenant_id, next_jobs.applet_id, next_jobs.job_id, next_jobs.method_name, next_jobs.params
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("claim queued jobs: %w", err)
	}
	defer rows.Close()

	jobs := make([]queuedJob, 0)
	for rows.Next() {
		var (
			job queuedJob
			raw []byte
		)
		if err := rows.Scan(&job.TenantID, &job.AppletID, &job.JobID, &job.Method, &raw); err != nil {
			return nil, fmt.Errorf("scan queued job: %w", err)
		}
		if err := json.Unmarshal(raw, &job.Params); err != nil {
			return nil, fmt.Errorf("unmarshal queued job params: %w", err)
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("queued jobs rows: %w", err)
	}
	return jobs, nil
}

func (r *Runner) setJobStatus(ctx context.Context, job queuedJob, status string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE applet_engine_jobs
		SET status = $4, updated_at = NOW()
		WHERE tenant_id = $1 AND applet_id = $2 AND job_id = $3
	`, job.TenantID, job.AppletID, job.JobID, status)
	if err != nil {
		return fmt.Errorf("set job status: %w", err)
	}
	return nil
}
