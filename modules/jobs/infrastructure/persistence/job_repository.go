package persistence

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/jobs/domain/aggregates/job"
	"github.com/iota-uz/iota-sdk/modules/jobs/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

const (
	findJobQuery = `
		SELECT j.id,
			j.tenant_id,
			j.user_id,
			j.kind,
			j.params,
			j.status,
			j.progress,
			j.phase,
			j.attempt,
			j.result_upload_id,
			j.error,
			j.created_at,
			j.updated_at,
			j.started_at,
			j.finished_at
		FROM jobs j`
	insertJobQuery = `
		INSERT INTO jobs (
			tenant_id,
			user_id,
			kind,
			params,
			status,
			progress,
			phase,
			attempt,
			result_upload_id,
			error,
			created_at,
			updated_at,
			started_at,
			finished_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id`
	updateJobQuery = `
		UPDATE jobs
		SET status = $1,
			progress = $2,
			phase = $3,
			attempt = $4,
			result_upload_id = $5,
			error = $6,
			updated_at = $7,
			started_at = $8,
			finished_at = $9
		WHERE id = $10 AND tenant_id = $11`
	deleteJobQuery    = `DELETE FROM jobs WHERE id = $1 AND tenant_id = $2`
	claimNextJobQuery = `
		UPDATE jobs
		SET status = 'running',
			started_at = now(),
			updated_at = now(),
			attempt = attempt + 1
		WHERE id = (
			SELECT id FROM jobs
			WHERE status = 'queued'
			ORDER BY created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING id, tenant_id, user_id, kind, params, status, progress, phase,
			attempt, result_upload_id, error, created_at, updated_at, started_at, finished_at`
	updateJobProgressQuery = `
		UPDATE jobs
		SET progress = $1, phase = $2, updated_at = now()
		WHERE id = $3 AND tenant_id = $4`
)

type JobRepository struct{}

func NewJobRepository() job.Repository {
	return &JobRepository{}
}

func (r *JobRepository) Save(ctx context.Context, j job.Job) (job.Job, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}
	if j.TenantID() == uuid.Nil {
		return nil, errors.New("job tenant id is required")
	}
	if j.TenantID() != tenantID {
		return nil, job.ErrNotFound
	}

	exists, err := r.exists(ctx, j.ID())
	if err != nil {
		return nil, err
	}
	if exists {
		return r.update(ctx, j)
	}
	return r.create(ctx, j)
}

func (r *JobRepository) exists(ctx context.Context, id uuid.UUID) (bool, error) {
	if id == uuid.Nil {
		return false, nil
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return false, err
	}
	var count int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM jobs WHERE id = $1`, id).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *JobRepository) create(ctx context.Context, j job.Job) (job.Job, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	m := JobToModel(j)
	var id uuid.UUID
	err = tx.QueryRow(
		ctx,
		insertJobQuery,
		m.TenantID,
		m.UserID,
		m.Kind,
		m.Params,
		m.Status,
		m.Progress,
		m.Phase,
		m.Attempt,
		m.ResultUploadID,
		m.Error,
		m.CreatedAt,
		m.UpdatedAt,
		m.StartedAt,
		m.FinishedAt,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	j.SetID(id)
	return j, nil
}

func (r *JobRepository) update(ctx context.Context, j job.Job) (job.Job, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	m := JobToModel(j)
	_, err = tx.Exec(
		ctx,
		updateJobQuery,
		m.Status,
		m.Progress,
		m.Phase,
		m.Attempt,
		m.ResultUploadID,
		m.Error,
		m.UpdatedAt,
		m.StartedAt,
		m.FinishedAt,
		m.ID,
		tenantID,
	)
	if err != nil {
		return nil, err
	}
	return j, nil
}

func (r *JobRepository) GetByID(ctx context.Context, id uuid.UUID) (job.Job, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}
	jobs, err := r.queryJobs(ctx, findJobQuery+` WHERE j.id = $1 AND j.tenant_id = $2`, id, tenantID)
	if err != nil {
		return nil, err
	}
	if len(jobs) == 0 {
		return nil, job.ErrNotFound
	}
	return jobs[0], nil
}

func (r *JobRepository) GetByIDForUser(ctx context.Context, id uuid.UUID, userID uint) (job.Job, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}
	jobs, err := r.queryJobs(
		ctx,
		findJobQuery+` WHERE j.id = $1 AND j.tenant_id = $2 AND j.user_id = $3`,
		id, tenantID, userID,
	)
	if err != nil {
		return nil, err
	}
	if len(jobs) == 0 {
		return nil, job.ErrNotFound
	}
	return jobs[0], nil
}

func (r *JobRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return err
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, deleteJobQuery, id, tenantID)
	return err
}

func (r *JobRepository) ListByUser(ctx context.Context, userID uint, limit int) ([]job.Job, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	return r.queryJobs(
		ctx,
		findJobQuery+` WHERE j.tenant_id = $1 AND j.user_id = $2 ORDER BY j.created_at DESC LIMIT $3`,
		tenantID, userID, limit,
	)
}

// ClaimNext is system-level: the background worker claims queued jobs across
// all tenants, so no tenant scoping is applied. Concurrency safety comes from
// FOR UPDATE SKIP LOCKED inside the UPDATE sub-select.
func (r *JobRepository) ClaimNext(ctx context.Context) (job.Job, bool, error) {
	jobs, err := r.queryJobs(ctx, claimNextJobQuery)
	if err != nil {
		return nil, false, err
	}
	if len(jobs) == 0 {
		return nil, false, nil
	}
	return jobs[0], true, nil
}

func (r *JobRepository) UpdateProgress(ctx context.Context, id uuid.UUID, percent int, phase string) error {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return err
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, updateJobProgressQuery, percent, phase, id, tenantID)
	return err
}

// ListFinishedBefore is system-level: the retention sweep scans terminal jobs
// across all tenants.
func (r *JobRepository) ListFinishedBefore(ctx context.Context, cutoff time.Time, limit int) ([]job.Job, error) {
	if limit <= 0 {
		limit = 100
	}
	return r.queryJobs(
		ctx,
		findJobQuery+` WHERE j.status IN ('done','failed') AND j.finished_at < $1 ORDER BY j.finished_at ASC LIMIT $2`,
		cutoff, limit,
	)
}

func (r *JobRepository) queryJobs(ctx context.Context, query string, args ...interface{}) ([]job.Job, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dbRows []*models.Job
	for rows.Next() {
		m := &models.Job{}
		if err := rows.Scan(
			&m.ID,
			&m.TenantID,
			&m.UserID,
			&m.Kind,
			&m.Params,
			&m.Status,
			&m.Progress,
			&m.Phase,
			&m.Attempt,
			&m.ResultUploadID,
			&m.Error,
			&m.CreatedAt,
			&m.UpdatedAt,
			&m.StartedAt,
			&m.FinishedAt,
		); err != nil {
			return nil, err
		}
		dbRows = append(dbRows, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	jobs := make([]job.Job, 0, len(dbRows))
	for _, m := range dbRows {
		jobs = append(jobs, JobToDomain(m))
	}
	return jobs, nil
}
