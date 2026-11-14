package host

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	opRecordStart  = serrors.Op("extruntime.host.records.start")
	opRecordFinish = serrors.Op("extruntime.host.records.finish")
)

// Outcome is the final state recorded on an invocation.
type Outcome string

const (
	OutcomeOK         Outcome = "ok"
	OutcomeError      Outcome = "error"
	OutcomeWorkerLost Outcome = "worker_lost"
	// OutcomeUnknown is what a record keeps when the host died before it
	// could write the final state.
	OutcomeUnknown Outcome = "unknown"
)

// TriggerKind says what started a root invocation.
type TriggerKind string

const (
	TriggerUI           TriggerKind = "ui"
	TriggerMCP          TriggerKind = "mcp"
	TriggerEndpoint     TriggerKind = "endpoint"
	TriggerSchedule     TriggerKind = "schedule"
	TriggerSubscription TriggerKind = "subscription"
	TriggerWorkflow     TriggerKind = "workflow"
)

type startRecord struct {
	ID, ParentID, RootID  string
	TenantID              string
	UserID                int64
	SubjectKind           string
	Contract              string
	ContractHash          string
	BindingID             string
	Package, Version, Exp string
	TriggerKind           TriggerKind
	TriggerRef            string
}

type finishRecord struct {
	ID                 string
	Outcome            Outcome
	PartiallyCommitted bool
	Result             json.RawMessage
	Error              json.RawMessage
}

type recorder struct{ db *pgxpool.Pool }

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (r recorder) start(ctx context.Context, s startRecord) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO extruntime.invocations
			(id, parent_invocation_id, root_id, tenant_id, subject_user_id, subject_kind, contract, contract_hash,
			 binding_id, binding_package, binding_version, binding_export, trigger_kind, trigger_ref)
		VALUES ($1, $2::uuid, $3, $4, $5, $6, $7, $8, $9::uuid, $10, $11, $12, $13, $14)`,
		s.ID, nullIfEmpty(s.ParentID), s.RootID, s.TenantID, s.UserID, s.SubjectKind, s.Contract, s.ContractHash,
		nullIfEmpty(s.BindingID), s.Package, s.Version, s.Exp, string(s.TriggerKind), s.TriggerRef)
	return serrors.FromDB(opRecordStart, err)
}

// finish runs after the invocation's context may already be done, so it uses
// its own short deadline.
func (r recorder) finish(ctx context.Context, f finishRecord) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	const update = `
		UPDATE extruntime.invocations
		SET finished_at = CURRENT_TIMESTAMP, outcome = $2, partially_committed = $3, result = $4, error = $5
		WHERE id = $1`
	_, err := r.db.Exec(ctx, update, f.ID, string(f.Outcome), f.PartiallyCommitted, jsonOrNil(f.Result), jsonOrNil(f.Error))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUnsupportedEscape {
		// A handler result or error that carries U+0000 (an endpoint echoing a
		// binary body) cannot be stored as jsonb. The outcome must still be
		// recorded, or the invocation would stay "unknown" forever.
		_, err = r.db.Exec(ctx, update, f.ID, string(f.Outcome), f.PartiallyCommitted, unrecordable(f.Result), unrecordable(f.Error))
	}
	return serrors.FromDB(opRecordFinish, err)
}

// pgUnsupportedEscape is SQLSTATE 22P05 (jsonb rejects \u0000).
const pgUnsupportedEscape = "22P05"

func unrecordable(b json.RawMessage) any {
	if len(b) == 0 {
		return nil
	}
	return []byte(`{"unrecordable":"the value contains U+0000 and cannot be stored"}`)
}

// countCommit is called inside the committing transaction so the counter
// commits atomically with the data.
func countCommit(ctx context.Context, tx pgx.Tx, invocationID string) error {
	_, err := tx.Exec(ctx, `UPDATE extruntime.invocations SET committed_tx_count = committed_tx_count + 1 WHERE id = $1`, invocationID)
	return serrors.FromDB(opRecordFinish, err)
}

func jsonOrNil(b json.RawMessage) any {
	if len(b) == 0 {
		return nil
	}
	return []byte(b)
}
