// Package persistence stores dbctl execution history and artifacts.
package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrArtifactNotFound = errors.New("dbctl artifact not found")

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) EnsureTables(ctx context.Context) error {
	if r == nil || r.pool == nil {
		return nil
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS dbctl_runs (
			id UUID PRIMARY KEY,
			operation TEXT NOT NULL,
			mode TEXT NOT NULL,
			started_at TIMESTAMPTZ NOT NULL,
			finished_at TIMESTAMPTZ,
			actor TEXT NOT NULL,
			status TEXT NOT NULL,
			target_fingerprint TEXT NOT NULL,
			policy_hash TEXT NOT NULL,
			error TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS dbctl_run_steps (
			run_id UUID NOT NULL REFERENCES dbctl_runs(id) ON DELETE CASCADE,
			step_id TEXT NOT NULL,
			status TEXT NOT NULL,
			started_at TIMESTAMPTZ NOT NULL,
			finished_at TIMESTAMPTZ,
			error TEXT,
			PRIMARY KEY (run_id, step_id)
		)`,
		`CREATE TABLE IF NOT EXISTS dbctl_run_artifacts (
			id BIGSERIAL PRIMARY KEY,
			run_id UUID NOT NULL REFERENCES dbctl_runs(id) ON DELETE CASCADE,
			artifact_type TEXT NOT NULL,
			payload_json JSONB NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
	}
	for _, stmt := range statements {
		if _, err := r.pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("ensure dbctl table: %w", err)
		}
	}
	return nil
}

func (r *Repository) InsertRun(ctx context.Context, rec RunRecord) error {
	if r == nil || r.pool == nil {
		return nil
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO dbctl_runs (id, operation, mode, started_at, finished_at, actor, status, target_fingerprint, policy_hash, error)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, rec.ID, rec.Operation, rec.Mode, rec.StartedAt, rec.FinishedAt, rec.Actor, rec.Status, rec.TargetFingerprint, rec.PolicyHash, rec.Error)
	if err != nil {
		return fmt.Errorf("insert dbctl run: %w", err)
	}
	return nil
}

func (r *Repository) UpdateRunStatus(ctx context.Context, runID, status string, runErr *string) error {
	if r == nil || r.pool == nil {
		return nil
	}
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `UPDATE dbctl_runs SET status=$2, finished_at=$3, error=$4 WHERE id=$1`, runID, status, now, runErr)
	if err != nil {
		return fmt.Errorf("update dbctl run status: %w", err)
	}
	return nil
}

func (r *Repository) UpsertStep(ctx context.Context, rec StepRecord) error {
	if r == nil || r.pool == nil {
		return nil
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO dbctl_run_steps (run_id, step_id, status, started_at, finished_at, error)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (run_id, step_id)
		DO UPDATE SET status=EXCLUDED.status, finished_at=EXCLUDED.finished_at, error=EXCLUDED.error
	`, rec.RunID, rec.StepID, rec.Status, rec.StartedAt, rec.FinishedAt, rec.Error)
	if err != nil {
		return fmt.Errorf("upsert dbctl step: %w", err)
	}
	return nil
}

func (r *Repository) InsertArtifact(ctx context.Context, runID, artifactType, payload string) error {
	if r == nil || r.pool == nil {
		return nil
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO dbctl_run_artifacts (run_id, artifact_type, payload_json)
		VALUES ($1,$2,$3::jsonb)
	`, runID, artifactType, payload)
	if err != nil {
		return fmt.Errorf("insert dbctl artifact: %w", err)
	}
	return nil
}

func (r *Repository) ListRuns(ctx context.Context, limit int) ([]RunRecord, error) {
	if r == nil || r.pool == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, operation, mode, started_at, finished_at, actor, status, target_fingerprint, policy_hash, error
		FROM dbctl_runs
		ORDER BY started_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list dbctl runs: %w", err)
	}
	defer rows.Close()
	out := make([]RunRecord, 0)
	for rows.Next() {
		var rec RunRecord
		if err := rows.Scan(&rec.ID, &rec.Operation, &rec.Mode, &rec.StartedAt, &rec.FinishedAt, &rec.Actor, &rec.Status, &rec.TargetFingerprint, &rec.PolicyHash, &rec.Error); err != nil {
			return nil, fmt.Errorf("scan dbctl run: %w", err)
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dbctl runs: %w", err)
	}
	return out, nil
}

func (r *Repository) LatestArtifact(ctx context.Context, runID, artifactType string) (*ArtifactRecord, error) {
	if r == nil || r.pool == nil {
		return nil, ErrArtifactNotFound
	}
	if _, err := uuid.Parse(runID); err != nil {
		return nil, fmt.Errorf("parse run id %q: %w", runID, err)
	}
	var rec ArtifactRecord
	err := r.pool.QueryRow(ctx, `
		SELECT run_id::text, artifact_type, payload_json::text, created_at
		FROM dbctl_run_artifacts
		WHERE run_id = $1::uuid AND artifact_type = $2
		ORDER BY created_at DESC
		LIMIT 1
	`, runID, artifactType).Scan(&rec.RunID, &rec.ArtifactType, &rec.PayloadJSON, &rec.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrArtifactNotFound
		}
		return nil, fmt.Errorf("query latest artifact for run %s: %w", runID, err)
	}
	return &rec, nil
}
