package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresJobsStore struct {
	pool *pgxpool.Pool
}

func NewPostgresJobsStore(pool *pgxpool.Pool) (*PostgresJobsStore, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}
	return &PostgresJobsStore{pool: pool}, nil
}

func (s *PostgresJobsStore) Enqueue(ctx context.Context, method string, params any) (map[string]any, error) {
	return s.insert(ctx, "one_off", "", method, params, "queued")
}

func (s *PostgresJobsStore) Schedule(ctx context.Context, cronExpr string, method string, params any) (map[string]any, error) {
	return s.insert(ctx, "scheduled", cronExpr, method, params, "scheduled")
}

func (s *PostgresJobsStore) List(ctx context.Context) ([]map[string]any, error) {
	tenantID, appletID := tenantAndAppletFromContext(ctx)
	rows, err := s.pool.Query(ctx, `
		SELECT job_id, job_type, cron_expr, method_name, params, status, created_at, updated_at
		FROM applet_engine_jobs
		WHERE tenant_id = $1 AND applet_id = $2
		ORDER BY created_at DESC
	`, tenantID, appletID)
	if err != nil {
		return nil, fmt.Errorf("postgres jobs.list: %w", err)
	}
	defer rows.Close()

	out := make([]map[string]any, 0)
	for rows.Next() {
		job, err := scanJobRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres jobs.list rows: %w", err)
	}
	return out, nil
}

func (s *PostgresJobsStore) Cancel(ctx context.Context, jobID string) (bool, error) {
	tenantID, appletID := tenantAndAppletFromContext(ctx)
	commandTag, err := s.pool.Exec(ctx, `
		UPDATE applet_engine_jobs
		SET status = 'canceled', updated_at = NOW()
		WHERE tenant_id = $1 AND applet_id = $2 AND job_id = $3 AND status <> 'canceled'
	`, tenantID, appletID, jobID)
	if err != nil {
		return false, fmt.Errorf("postgres jobs.cancel: %w", err)
	}
	return commandTag.RowsAffected() > 0, nil
}

func (s *PostgresJobsStore) insert(ctx context.Context, jobType, cronExpr, method string, params any, status string) (map[string]any, error) {
	tenantID, appletID := tenantAndAppletFromContext(ctx)
	jobID := uuid.NewString()
	encoded, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal job params: %w", err)
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO applet_engine_jobs(
			tenant_id,
			applet_id,
			job_id,
			job_type,
			cron_expr,
			method_name,
			params,
			status
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8)
		RETURNING job_id, job_type, cron_expr, method_name, params, status, created_at, updated_at
	`, tenantID, appletID, jobID, jobType, cronExpr, method, string(encoded), status)
	return scanJobRow(row)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanJobRow(row rowScanner) (map[string]any, error) {
	var (
		jobID     string
		jobType   string
		cronExpr  string
		method    string
		rawParams []byte
		status    string
		createdAt time.Time
		updatedAt time.Time
	)
	if err := row.Scan(&jobID, &jobType, &cronExpr, &method, &rawParams, &status, &createdAt, &updatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan job row: %w", err)
	}
	params, err := decodeJSONValue(rawParams)
	if err != nil {
		return nil, err
	}
	result := map[string]any{
		"id":        jobID,
		"type":      jobType,
		"cron":      cronExpr,
		"method":    method,
		"params":    params,
		"status":    status,
		"createdAt": createdAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
		"updatedAt": updatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
	}
	return result, nil
}
