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
	JobType  string
	CronExpr string
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
	if err := r.processQueuedJobs(ctx); err != nil {
		return err
	}
	if err := r.processScheduledJobs(ctx); err != nil {
		return err
	}
	return nil
}

func (r *Runner) processQueuedJobs(ctx context.Context) error {
	jobs, err := r.claimQueuedJobs(ctx, 16)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if err := r.dispatcher.DispatchJob(ctx, job.AppletID, job.TenantID, job.JobID, job.Method, job.Params); err != nil {
			r.logger.WithFields(logrus.Fields{"applet": job.AppletID, "job_id": job.JobID}).WithError(err).Error("applet queued job failed")
			_ = r.setOneOffStatus(ctx, job, "failed", err.Error())
			continue
		}
		if err := r.setOneOffStatus(ctx, job, "completed", ""); err != nil {
			r.logger.WithFields(logrus.Fields{"applet": job.AppletID, "job_id": job.JobID}).WithError(err).Error("failed to mark applet job as completed")
		}
	}
	return nil
}

func (r *Runner) processScheduledJobs(ctx context.Context) error {
	jobs, err := r.claimDueScheduledJobs(ctx, 16)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		nextRunAt, nextErr := NextRun(job.CronExpr, time.Now().UTC())
		if nextErr != nil {
			r.logger.WithFields(logrus.Fields{
				"applet":   job.AppletID,
				"job_id":   job.JobID,
				"cronExpr": job.CronExpr,
			}).WithError(nextErr).Error("invalid cron expression for scheduled applet job")
			_ = r.failScheduledJob(ctx, job, nextErr.Error())
			continue
		}
		if err := r.dispatcher.DispatchJob(ctx, job.AppletID, job.TenantID, job.JobID, job.Method, job.Params); err != nil {
			r.logger.WithFields(logrus.Fields{"applet": job.AppletID, "job_id": job.JobID}).WithError(err).Error("applet scheduled job run failed")
			_ = r.setScheduledStatus(ctx, job, "failed", err.Error(), nextRunAt)
			continue
		}
		if err := r.setScheduledStatus(ctx, job, "completed", "", nextRunAt); err != nil {
			r.logger.WithFields(logrus.Fields{"applet": job.AppletID, "job_id": job.JobID}).WithError(err).Error("failed to mark scheduled applet job run")
		}
	}
	return nil
}

func (r *Runner) claimQueuedJobs(ctx context.Context, limit int) ([]queuedJob, error) {
	rows, err := r.pool.Query(ctx, `
		WITH next_jobs AS (
			SELECT tenant_id, applet_id, job_id, job_type, cron_expr, method_name, params
			FROM applet_engine_jobs
			WHERE status = 'queued' AND job_type = 'one_off'
			ORDER BY created_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE applet_engine_jobs j
		SET status = 'running', updated_at = NOW(), last_status = 'running', last_error = ''
		FROM next_jobs
		WHERE j.tenant_id = next_jobs.tenant_id
		  AND j.applet_id = next_jobs.applet_id
		  AND j.job_id = next_jobs.job_id
		RETURNING next_jobs.tenant_id, next_jobs.applet_id, next_jobs.job_id, next_jobs.job_type, next_jobs.cron_expr, next_jobs.method_name, next_jobs.params
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
		if err := rows.Scan(&job.TenantID, &job.AppletID, &job.JobID, &job.JobType, &job.CronExpr, &job.Method, &raw); err != nil {
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

func (r *Runner) claimDueScheduledJobs(ctx context.Context, limit int) ([]queuedJob, error) {
	rows, err := r.pool.Query(ctx, `
		WITH next_jobs AS (
			SELECT tenant_id, applet_id, job_id, job_type, cron_expr, method_name, params
			FROM applet_engine_jobs
			WHERE status = 'scheduled'
			  AND job_type = 'scheduled'
			  AND next_run_at IS NOT NULL
			  AND next_run_at <= NOW()
			ORDER BY next_run_at, created_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE applet_engine_jobs j
		SET status = 'running', updated_at = NOW(), last_status = 'running', last_error = ''
		FROM next_jobs
		WHERE j.tenant_id = next_jobs.tenant_id
		  AND j.applet_id = next_jobs.applet_id
		  AND j.job_id = next_jobs.job_id
		RETURNING next_jobs.tenant_id, next_jobs.applet_id, next_jobs.job_id, next_jobs.job_type, next_jobs.cron_expr, next_jobs.method_name, next_jobs.params
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("claim due scheduled jobs: %w", err)
	}
	defer rows.Close()

	jobs := make([]queuedJob, 0)
	for rows.Next() {
		var (
			job queuedJob
			raw []byte
		)
		if err := rows.Scan(&job.TenantID, &job.AppletID, &job.JobID, &job.JobType, &job.CronExpr, &job.Method, &raw); err != nil {
			return nil, fmt.Errorf("scan scheduled job: %w", err)
		}
		if err := json.Unmarshal(raw, &job.Params); err != nil {
			return nil, fmt.Errorf("unmarshal scheduled job params: %w", err)
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scheduled jobs rows: %w", err)
	}
	return jobs, nil
}

func (r *Runner) setOneOffStatus(ctx context.Context, job queuedJob, status string, lastError string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE applet_engine_jobs
		SET status = $4, updated_at = NOW(), last_run_at = NOW(), last_status = $4, last_error = $5
		WHERE tenant_id = $1 AND applet_id = $2 AND job_id = $3
	`, job.TenantID, job.AppletID, job.JobID, status, lastError)
	if err != nil {
		return fmt.Errorf("set job status: %w", err)
	}
	return nil
}

func (r *Runner) setScheduledStatus(ctx context.Context, job queuedJob, lastStatus string, lastError string, nextRunAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE applet_engine_jobs
		SET status = 'scheduled',
			next_run_at = $4,
			last_run_at = NOW(),
			last_status = $5,
			last_error = $6,
			updated_at = NOW()
		WHERE tenant_id = $1 AND applet_id = $2 AND job_id = $3
	`, job.TenantID, job.AppletID, job.JobID, nextRunAt, lastStatus, lastError)
	if err != nil {
		return fmt.Errorf("set scheduled job status: %w", err)
	}
	return nil
}

func (r *Runner) failScheduledJob(ctx context.Context, job queuedJob, lastError string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE applet_engine_jobs
		SET status = 'failed',
			next_run_at = NULL,
			last_run_at = NOW(),
			last_status = 'failed',
			last_error = $4,
			updated_at = NOW()
		WHERE tenant_id = $1 AND applet_id = $2 AND job_id = $3
	`, job.TenantID, job.AppletID, job.JobID, lastError)
	if err != nil {
		return fmt.Errorf("set scheduled job failed status: %w", err)
	}
	return nil
}
