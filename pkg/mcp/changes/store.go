package changes

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

var (
	// ErrOperationLocked means another apply of the same operation is running.
	ErrOperationLocked = errors.New("operation is locked by a running apply")
	errNotFound        = serrors.New(serrors.NotFound, "not found")
)

// Store keeps plans, approvals, operations and the effect outbox durable;
// results of writes are never served from memory.
type Store interface {
	// InTx runs fn in a transaction that domain services join via ctx.
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
	SavePlan(ctx context.Context, p Plan) error
	Plan(ctx context.Context, tenantID, planID string) (Plan, error)
	SaveApproval(ctx context.Context, a Approval) error
	Approval(ctx context.Context, tenantID, refHash string) (Approval, error)
	PlanApproval(ctx context.Context, tenantID, planID, planHash, subject string, at time.Time) (Approval, error)
	// ClaimOperation records a pending operation for the idempotency key in
	// its own commit, or returns the operation that already holds the key.
	ClaimOperation(ctx context.Context, op Operation) (Operation, error)
	Operation(ctx context.Context, tenantID, subject, operationID string) (Operation, error)
	OperationByKey(ctx context.Context, tenantID, subject, key string) (Operation, error)
	// LockOperation row-locks a pending operation inside InTx; a concurrent
	// holder yields ErrOperationLocked.
	LockOperation(ctx context.Context, operationID string) (Operation, error)
	CompleteOperation(ctx context.Context, operationID string, receipt Receipt, effects []Effect) error
	FailOperation(ctx context.Context, operationID string, receipt *Receipt, effects string, f *mcp.Failure) error
	// PendingEffects claims undelivered effects inside InTx, skipping ones
	// another dispatcher holds.
	PendingEffects(ctx context.Context, limit int) ([]PendingEffect, error)
	FinishEffect(ctx context.Context, effectID string, delivered bool, attempts int, lastError string, final bool) error
	SettleEffects(ctx context.Context, operationID string) error
}

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) db(ctx context.Context) repo.Tx {
	if tx, err := composables.UseTx(ctx); err == nil {
		return tx
	}
	return s.pool
}

func (s *PostgresStore) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(composables.WithTx(ctx, tx)); err != nil {
		if rbErr := tx.Rollback(context.WithoutCancel(ctx)); rbErr != nil {
			return errors.Join(err, rbErr)
		}
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) SavePlan(ctx context.Context, p Plan) error {
	const op serrors.Op = "changes.PostgresStore.SavePlan"
	changes, err := json.Marshal(p.Changes)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	items, err := json.Marshal(p.Items)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	warnings, err := json.Marshal(p.Warnings)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	_, err = s.db(ctx).Exec(ctx, `
		INSERT INTO mcp.plans (id, tenant_id, subject, client_id, hash, reason, changes, items, warnings, approval_required, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		p.ID, p.TenantID, p.Subject, p.ClientID, p.Hash, p.Reason, changes, items, warnings, p.ApprovalRequired, p.ExpiresAt)
	return serrors.FromDB(op, err)
}

func (s *PostgresStore) Plan(ctx context.Context, tenantID, planID string) (Plan, error) {
	const op serrors.Op = "changes.PostgresStore.Plan"
	if _, err := uuid.Parse(planID); err != nil {
		return Plan{}, serrors.Wrap(op, errNotFound)
	}
	var p Plan
	var changes, items, warnings []byte
	var tenant, id uuid.UUID
	err := s.db(ctx).QueryRow(ctx, `
		SELECT id, tenant_id, subject, client_id, hash, reason, changes, items, warnings, approval_required, expires_at
		FROM mcp.plans WHERE id = $1 AND tenant_id = $2`, planID, tenantID,
	).Scan(&id, &tenant, &p.Subject, &p.ClientID, &p.Hash, &p.Reason, &changes, &items, &warnings, &p.ApprovalRequired, &p.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Plan{}, serrors.Wrap(op, errNotFound)
	}
	if err != nil {
		return Plan{}, serrors.FromDB(op, err)
	}
	p.ID, p.TenantID = id.String(), tenant.String()
	if err := json.Unmarshal(changes, &p.Changes); err != nil {
		return Plan{}, serrors.Wrap(op, err)
	}
	if err := json.Unmarshal(items, &p.Items); err != nil {
		return Plan{}, serrors.Wrap(op, err)
	}
	if err := json.Unmarshal(warnings, &p.Warnings); err != nil {
		return Plan{}, serrors.Wrap(op, err)
	}
	return p, nil
}

func (s *PostgresStore) SaveApproval(ctx context.Context, a Approval) error {
	const op serrors.Op = "changes.PostgresStore.SaveApproval"
	_, err := s.db(ctx).Exec(ctx, `
		INSERT INTO mcp.approvals (ref_hash, tenant_id, plan_id, plan_hash, subject, approved_by, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		a.RefHash, a.TenantID, a.PlanID, a.PlanHash, a.Subject, a.ApprovedBy, a.ExpiresAt)
	return serrors.FromDB(op, err)
}

const approvalColumns = `ref_hash, tenant_id::text, plan_id::text, plan_hash, subject, approved_by, expires_at`

func scanApproval(row pgx.Row) (Approval, error) {
	var a Approval
	err := row.Scan(&a.RefHash, &a.TenantID, &a.PlanID, &a.PlanHash, &a.Subject, &a.ApprovedBy, &a.ExpiresAt)
	return a, err
}

func (s *PostgresStore) Approval(ctx context.Context, tenantID, refHash string) (Approval, error) {
	const op serrors.Op = "changes.PostgresStore.Approval"
	a, err := scanApproval(s.db(ctx).QueryRow(ctx,
		`SELECT `+approvalColumns+` FROM mcp.approvals WHERE ref_hash = $1 AND tenant_id = $2`, refHash, tenantID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Approval{}, serrors.Wrap(op, errNotFound)
	}
	return a, serrors.FromDB(op, err)
}

func (s *PostgresStore) PlanApproval(ctx context.Context, tenantID, planID, planHash, subject string, at time.Time) (Approval, error) {
	const op serrors.Op = "changes.PostgresStore.PlanApproval"
	a, err := scanApproval(s.db(ctx).QueryRow(ctx, `
		SELECT `+approvalColumns+` FROM mcp.approvals
		WHERE tenant_id = $1 AND plan_id = $2 AND plan_hash = $3 AND subject = $4 AND expires_at > $5
		ORDER BY created_at DESC LIMIT 1`, tenantID, planID, planHash, subject, at))
	if errors.Is(err, pgx.ErrNoRows) {
		return Approval{}, serrors.Wrap(op, errNotFound)
	}
	return a, serrors.FromDB(op, err)
}

const operationColumns = `id::text, tenant_id::text, subject, idempotency_key, plan_id::text, plan_hash, status, external_effects, receipt, error`

func scanOperation(row pgx.Row) (Operation, error) {
	var o Operation
	var receipt, failure []byte
	if err := row.Scan(&o.ID, &o.TenantID, &o.Subject, &o.IdempotencyKey, &o.PlanID, &o.PlanHash,
		&o.Status, &o.ExternalEffects, &receipt, &failure); err != nil {
		return Operation{}, err
	}
	if receipt != nil {
		o.Receipt = &Receipt{}
		if err := json.Unmarshal(receipt, o.Receipt); err != nil {
			return Operation{}, err
		}
	}
	if failure != nil {
		o.Error = &mcp.Failure{}
		if err := json.Unmarshal(failure, o.Error); err != nil {
			return Operation{}, err
		}
	}
	return o, nil
}

func (s *PostgresStore) ClaimOperation(ctx context.Context, o Operation) (Operation, error) {
	const op serrors.Op = "changes.PostgresStore.ClaimOperation"
	claimed, err := scanOperation(s.db(ctx).QueryRow(ctx, `
		INSERT INTO mcp.operations (id, tenant_id, subject, idempotency_key, plan_id, plan_hash, status)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending')
		ON CONFLICT (tenant_id, subject, idempotency_key) DO NOTHING
		RETURNING `+operationColumns,
		o.ID, o.TenantID, o.Subject, o.IdempotencyKey, o.PlanID, o.PlanHash))
	if errors.Is(err, pgx.ErrNoRows) {
		return s.OperationByKey(ctx, o.TenantID, o.Subject, o.IdempotencyKey)
	}
	return claimed, serrors.FromDB(op, err)
}

func (s *PostgresStore) Operation(ctx context.Context, tenantID, subject, operationID string) (Operation, error) {
	const op serrors.Op = "changes.PostgresStore.Operation"
	if _, err := uuid.Parse(operationID); err != nil {
		return Operation{}, serrors.Wrap(op, errNotFound)
	}
	o, err := scanOperation(s.db(ctx).QueryRow(ctx,
		`SELECT `+operationColumns+` FROM mcp.operations WHERE id = $1 AND tenant_id = $2 AND subject = $3`,
		operationID, tenantID, subject))
	if errors.Is(err, pgx.ErrNoRows) {
		return Operation{}, serrors.Wrap(op, errNotFound)
	}
	return o, serrors.FromDB(op, err)
}

func (s *PostgresStore) OperationByKey(ctx context.Context, tenantID, subject, key string) (Operation, error) {
	const op serrors.Op = "changes.PostgresStore.OperationByKey"
	o, err := scanOperation(s.db(ctx).QueryRow(ctx,
		`SELECT `+operationColumns+` FROM mcp.operations WHERE tenant_id = $1 AND subject = $2 AND idempotency_key = $3`,
		tenantID, subject, key))
	if errors.Is(err, pgx.ErrNoRows) {
		return Operation{}, serrors.Wrap(op, errNotFound)
	}
	return o, serrors.FromDB(op, err)
}

func (s *PostgresStore) LockOperation(ctx context.Context, operationID string) (Operation, error) {
	const op serrors.Op = "changes.PostgresStore.LockOperation"
	o, err := scanOperation(s.db(ctx).QueryRow(ctx,
		`SELECT `+operationColumns+` FROM mcp.operations WHERE id = $1 FOR UPDATE NOWAIT`, operationID))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "55P03" {
		return Operation{}, ErrOperationLocked
	}
	return o, serrors.FromDB(op, err)
}

func (s *PostgresStore) CompleteOperation(ctx context.Context, operationID string, receipt Receipt, effects []Effect) error {
	const op serrors.Op = "changes.PostgresStore.CompleteOperation"
	raw, err := json.Marshal(receipt)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	db := s.db(ctx)
	status, external := OperationSucceeded, EffectsNone
	if len(effects) > 0 {
		status, external = OperationPending, EffectsPending
	}
	if _, err := db.Exec(ctx, `
		UPDATE mcp.operations SET status = $2, external_effects = $3, receipt = $4, updated_at = now()
		WHERE id = $1`, operationID, status, external, raw); err != nil {
		return serrors.FromDB(op, err)
	}
	for _, e := range effects {
		if _, err := db.Exec(ctx, `
			INSERT INTO mcp.operation_effects (id, operation_id, tenant_id, kind, payload)
			SELECT $1, id, tenant_id, $3, $4 FROM mcp.operations WHERE id = $2`,
			uuid.New(), operationID, e.Kind, []byte(e.Payload)); err != nil {
			return serrors.FromDB(op, err)
		}
	}
	return nil
}

func (s *PostgresStore) FailOperation(ctx context.Context, operationID string, receipt *Receipt, effects string, f *mcp.Failure) error {
	const op serrors.Op = "changes.PostgresStore.FailOperation"
	failure, err := json.Marshal(f)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	var rawReceipt []byte
	if receipt != nil {
		if rawReceipt, err = json.Marshal(receipt); err != nil {
			return serrors.Wrap(op, err)
		}
	}
	_, err = s.db(ctx).Exec(ctx, `
		UPDATE mcp.operations SET status = 'failed', external_effects = $2, receipt = $3, error = $4, updated_at = now()
		WHERE id = $1`, operationID, effects, rawReceipt, failure)
	return serrors.FromDB(op, err)
}

func (s *PostgresStore) PendingEffects(ctx context.Context, limit int) ([]PendingEffect, error) {
	const op serrors.Op = "changes.PostgresStore.PendingEffects"
	rows, err := s.db(ctx).Query(ctx, `
		SELECT id::text, operation_id::text, tenant_id::text, attempts, kind, payload
		FROM mcp.operation_effects WHERE status = 'pending'
		ORDER BY created_at LIMIT $1 FOR UPDATE SKIP LOCKED`, limit)
	if err != nil {
		return nil, serrors.FromDB(op, err)
	}
	defer rows.Close()
	var out []PendingEffect
	for rows.Next() {
		var e PendingEffect
		var payload []byte
		if err := rows.Scan(&e.ID, &e.OperationID, &e.TenantID, &e.Attempts, &e.Effect.Kind, &payload); err != nil {
			return nil, serrors.FromDB(op, err)
		}
		e.Effect.Payload = payload
		out = append(out, e)
	}
	return out, serrors.FromDB(op, rows.Err())
}

func (s *PostgresStore) FinishEffect(ctx context.Context, effectID string, delivered bool, attempts int, lastError string, final bool) error {
	const op serrors.Op = "changes.PostgresStore.FinishEffect"
	status := "pending"
	switch {
	case delivered:
		status = "succeeded"
	case final:
		status = "failed"
	}
	_, err := s.db(ctx).Exec(ctx, `
		UPDATE mcp.operation_effects SET status = $2, attempts = $3, last_error = $4, updated_at = now()
		WHERE id = $1`, effectID, status, attempts, lastError)
	return serrors.FromDB(op, err)
}

// SettleEffects derives the operation state from its outbox once no effect
// is pending: all delivered means succeeded, any lost one fails the operation
// while keeping its receipt.
func (s *PostgresStore) SettleEffects(ctx context.Context, operationID string) error {
	const op serrors.Op = "changes.PostgresStore.SettleEffects"
	failure, err := json.Marshal(mcp.Fail(mcp.CodeDependencyUnavailable, mcp.RecoveryNone,
		"the change was committed but an external effect could not be delivered").WithOperation(operationID))
	if err != nil {
		return serrors.Wrap(op, err)
	}
	_, err = s.db(ctx).Exec(ctx, `
		WITH counts AS (
			SELECT count(*) FILTER (WHERE status = 'pending') AS pending,
			       count(*) FILTER (WHERE status = 'failed') AS failed
			FROM mcp.operation_effects WHERE operation_id = $1
		)
		UPDATE mcp.operations o SET
			status = CASE WHEN c.failed > 0 THEN 'failed' ELSE 'succeeded' END,
			external_effects = CASE WHEN c.failed > 0 THEN 'failed' ELSE 'succeeded' END,
			error = CASE WHEN c.failed > 0 THEN $2::jsonb ELSE NULL END,
			updated_at = now()
		FROM counts c
		WHERE o.id = $1 AND c.pending = 0 AND o.external_effects = 'pending'`, operationID, failure)
	return serrors.FromDB(op, err)
}
