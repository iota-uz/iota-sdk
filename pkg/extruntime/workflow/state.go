package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	opClaim  = serrors.Op("extruntime.workflow.claim")
	opFence  = serrors.Op("extruntime.workflow.fence")
	opFinish = serrors.Op("extruntime.workflow.finish")
	opLoad   = serrors.Op("extruntime.workflow.load")
)

// Instance is one workflow_instances row.
type Instance struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	Workflow         string
	Package          string
	Version          string
	ArtifactHash     string
	Export           string
	Input            json.RawMessage
	Status           string
	Result           json.RawMessage
	Error            json.RawMessage
	BlockedReason    string
	BlockedDetail    string
	SubjectUserID    int64
	SubjectKind      spi.SubjectKind
	PredecessorID    *uuid.UUID
	ParentInstanceID *uuid.UUID
	StartInvocation  *uuid.UUID
	StartKey         *string
	WakeAt           *time.Time
	LeaseEpoch       int64
	RunCount         int
	Failures         int
	CreatedAt        time.Time
	UpdatedAt        time.Time
	FinishedAt       *time.Time
}

const instanceCols = `id, tenant_id, workflow, package, version, artifact_hash, export, input, status, result, error,
	blocked_reason, blocked_detail, subject_user_id, subject_kind, predecessor_id, parent_instance_id, start_invocation_id,
	start_key, wake_at, lease_epoch, run_count, failures, created_at, updated_at, finished_at`

func scanInstance(row pgx.Row) (Instance, error) {
	var i Instance
	var kind string
	err := row.Scan(&i.ID, &i.TenantID, &i.Workflow, &i.Package, &i.Version, &i.ArtifactHash, &i.Export, &i.Input, &i.Status, &i.Result, &i.Error,
		&i.BlockedReason, &i.BlockedDetail, &i.SubjectUserID, &kind, &i.PredecessorID, &i.ParentInstanceID, &i.StartInvocation,
		&i.StartKey, &i.WakeAt, &i.LeaseEpoch, &i.RunCount, &i.Failures, &i.CreatedAt, &i.UpdatedAt, &i.FinishedAt)
	i.SubjectKind = spi.SubjectKind(kind)
	return i, err
}

// Occurrence is one workflow_occurrences row.
type Occurrence struct {
	Name         string
	N            int
	Kind         string
	Status       string
	InputHash    string
	Result       json.RawMessage
	Error        json.RawMessage
	Attempts     int
	WakeAt       *time.Time
	InvocationID string
	ContractHash string
	Meta         json.RawMessage
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

const occurrenceCols = `name, n, kind, status, input_hash, result, error, attempts, wake_at, COALESCE(invocation_id::text, ''), contract_hash, meta, created_at, updated_at`

func scanOccurrence(row pgx.Row) (Occurrence, error) {
	var o Occurrence
	err := row.Scan(&o.Name, &o.N, &o.Kind, &o.Status, &o.InputHash, &o.Result, &o.Error, &o.Attempts, &o.WakeAt, &o.InvocationID, &o.ContractHash, &o.Meta, &o.CreatedAt, &o.UpdatedAt)
	return o, err
}

func (o Occurrence) terminal() bool { return o.Status == OccCompleted || o.Status == OccFailed }

type occKey struct {
	name string
	n    int
}

// runState is the in-memory state of one active run: the claimed instance, the
// journal and the diagnostics the run must end with. It is created by the
// claim, keyed by the run token, and discarded when the run ends.
type runState struct {
	inst    Instance
	epoch   int64
	subject spi.Subject
	token   string
	cancel  context.CancelFunc

	mu      sync.Mutex
	journal map[occKey]Occurrence
	blocked *blockInfo
	// staleErr is set once the lease is lost or the instance terminated: the
	// run must not record anything and every further host call fails with it.
	staleErr *pool.WireError
	// abortErr is the infrastructure failure that ended the run: every further
	// host call is refused and the run is retried with backoff.
	abortErr error
}

func (rs *runState) get(name string, n int) (Occurrence, bool) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	o, ok := rs.journal[occKey{name, n}]
	return o, ok
}

func (rs *runState) set(o Occurrence) {
	rs.mu.Lock()
	rs.journal[occKey{o.Name, o.N}] = o
	rs.mu.Unlock()
}

func (rs *runState) del(name string, n int) {
	rs.mu.Lock()
	delete(rs.journal, occKey{name, n})
	rs.mu.Unlock()
}

func (rs *runState) block(reason, detail string) *blockInfo {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if rs.blocked == nil {
		rs.blocked = &blockInfo{reason: reason, detail: detail}
	}
	return rs.blocked
}

func (rs *runState) markStale(err *pool.WireError) {
	rs.mu.Lock()
	if rs.staleErr == nil {
		rs.staleErr = err
	}
	rs.mu.Unlock()
}

func (rs *runState) abort(err error) {
	rs.mu.Lock()
	if rs.abortErr == nil {
		rs.abortErr = err
	}
	rs.mu.Unlock()
}

func (rs *runState) aborted() error {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return rs.abortErr
}

func (rs *runState) stale() *pool.WireError {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return rs.staleErr
}

func (rs *runState) blockedInfo() *blockInfo {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return rs.blocked
}

// journalEntry is what the invoke carries to the Node worker: terminal
// occurrences, so replays are answered inside Node without a host call.
type journalEntry struct {
	Name      string          `json:"name"`
	N         int             `json:"n"`
	Kind      string          `json:"kind"`
	Status    string          `json:"status"`
	InputHash string          `json:"inputHash,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     json.RawMessage `json:"error,omitempty"`
}

func (rs *runState) journalPayload() json.RawMessage {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	entries := make([]journalEntry, 0, len(rs.journal))
	for _, o := range rs.journal {
		if !o.terminal() {
			continue
		}
		entries = append(entries, journalEntry{Name: o.Name, N: o.N, Kind: o.Kind, Status: o.Status, InputHash: o.InputHash, Result: o.Result, Error: o.Error})
	}
	return marshal(map[string]any{"journal": entries})
}

type queryer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (e *Engine) loadJournal(ctx context.Context, inst Instance) (map[occKey]Occurrence, error) {
	rows, err := e.cfg.DB.Query(ctx, `SELECT `+occurrenceCols+` FROM extruntime.workflow_occurrences WHERE instance_id = $1 AND tenant_id = $2`, inst.ID, inst.TenantID)
	if err != nil {
		return nil, serrors.FromDB(opLoad, err)
	}
	defer rows.Close()
	out := map[occKey]Occurrence{}
	for rows.Next() {
		o, err := scanOccurrence(rows)
		if err != nil {
			return nil, serrors.FromDB(opLoad, err)
		}
		out[occKey{o.Name, o.N}] = o
	}
	return out, serrors.FromDB(opLoad, rows.Err())
}

// claim takes one ready instance: a new or crashed one (no live lease), or a
// waiting one whose wake_at passed. The epoch is bumped, which fences any
// previous runner of the instance.
func (e *Engine) claim(ctx context.Context) (*Instance, error) {
	row := e.cfg.DB.QueryRow(ctx, `
		UPDATE extruntime.workflow_instances i
		SET status = 'running', lease_owner = $1, lease_until = now() + $2::interval, lease_epoch = i.lease_epoch + 1,
		    run_count = i.run_count + 1, wake_at = NULL, updated_at = now()
		WHERE i.id = (
			SELECT w.id FROM extruntime.workflow_instances w
			WHERE (w.status = 'running' AND (w.lease_until IS NULL OR w.lease_until < now()))
			   OR (w.status = 'waiting' AND w.wake_at IS NOT NULL AND w.wake_at <= now())
			ORDER BY w.updated_at
			FOR UPDATE SKIP LOCKED
			LIMIT 1)
		RETURNING `+instanceCols, e.cfg.Owner, e.cfg.LeaseTTL.String())
	inst, err := scanInstance(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, serrors.FromDB(opClaim, err)
	}
	return &inst, nil
}

// fence is the first statement of every state-writing transaction: it proves
// the run still owns the instance and locks the row, serializing the write
// against takeover, termination, signals and task decisions.
func (e *Engine) fence(ctx context.Context, q queryer, rs *runState) error {
	tag, err := q.Exec(ctx, `
		UPDATE extruntime.workflow_instances SET updated_at = now()
		WHERE id = $1 AND tenant_id = $2 AND lease_epoch = $3 AND status = 'running'`, rs.inst.ID, rs.inst.TenantID, rs.epoch)
	if err != nil {
		return serrors.FromDB(opFence, err)
	}
	if tag.RowsAffected() == 1 {
		return nil
	}
	var status string
	var epoch int64
	err = q.QueryRow(ctx, `SELECT status, lease_epoch FROM extruntime.workflow_instances WHERE id = $1 AND tenant_id = $2`, rs.inst.ID, rs.inst.TenantID).Scan(&status, &epoch)
	switch {
	case err != nil:
		return serrors.FromDB(opFence, err)
	case status == StatusTerminated:
		rs.markStale(errTerminated)
		return errTerminated
	default:
		rs.markStale(errLeaseLost)
		return errLeaseLost
	}
}

// inFence runs fn in a new transaction after the fence and commits it. fn
// returns an optional apply that updates the in-memory journal after commit.
func (e *Engine) inFence(ctx context.Context, rs *runState, fn func(tx pgx.Tx) (apply func(), err error)) error {
	tx, err := e.cfg.DB.Begin(ctx)
	if err != nil {
		return serrors.FromDB(opFence, err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := e.fence(ctx, tx, rs); err != nil {
		return err
	}
	apply, err := fn(tx)
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return serrors.FromDB(opFence, err)
	}
	if apply != nil {
		apply()
	}
	return nil
}

// putOccurrence upserts the occurrence row of the run's instance.
func putOccurrence(ctx context.Context, tx queryer, rs *runState, o Occurrence) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO extruntime.workflow_occurrences
			(instance_id, tenant_id, name, n, kind, status, input_hash, result, error, attempts, wake_at, invocation_id, contract_hash, meta)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::uuid, $13, $14)
		ON CONFLICT (instance_id, name, n) DO UPDATE
		SET kind = EXCLUDED.kind, status = EXCLUDED.status, input_hash = EXCLUDED.input_hash, result = EXCLUDED.result,
		    error = EXCLUDED.error, attempts = EXCLUDED.attempts, wake_at = EXCLUDED.wake_at,
		    invocation_id = EXCLUDED.invocation_id, contract_hash = EXCLUDED.contract_hash, meta = EXCLUDED.meta, updated_at = now()`,
		rs.inst.ID, rs.inst.TenantID, o.Name, o.N, o.Kind, o.Status, o.InputHash, jsonArg(o.Result), jsonArg(o.Error), o.Attempts,
		o.WakeAt, nullString(o.InvocationID), o.ContractHash, jsonArg(o.Meta))
	return serrors.FromDB(opFence, err)
}

func jsonArg(b json.RawMessage) any {
	if len(b) == 0 {
		return nil
	}
	return []byte(b)
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// ---- run lifecycle writes (all fenced) ----

func (e *Engine) fencedUpdate(ctx context.Context, rs *runState, set string, args ...any) error {
	all := append([]any{rs.inst.ID, rs.inst.TenantID, rs.epoch}, args...)
	_, err := e.cfg.DB.Exec(context.WithoutCancel(ctx), `UPDATE extruntime.workflow_instances SET `+set+`, updated_at = now(), lease_owner = NULL, lease_until = NULL
		WHERE id = $1 AND tenant_id = $2 AND lease_epoch = $3 AND status = 'running'`, all...)
	// No affected row means the lease was lost or the instance terminated
	// meanwhile: the new owner decides.
	return serrors.FromDB(opFinish, err)
}

func (e *Engine) finishCompleted(ctx context.Context, rs *runState, result json.RawMessage) error {
	return e.fencedUpdate(ctx, rs, `status = 'completed', result = $4, failures = 0, wake_at = NULL, finished_at = now()`, jsonArg(nonNull(result)))
}

func (e *Engine) finishFailed(ctx context.Context, rs *runState, we *pool.WireError) error {
	return e.fencedUpdate(ctx, rs, `status = 'failed', error = $4, failures = 0, wake_at = NULL, finished_at = now()`, jsonArg(marshal(we)))
}

func (e *Engine) finishBlocked(ctx context.Context, rs *runState, reason, detail string) error {
	e.cfg.Logger.Warn("extruntime: workflow instance blocked", "instance", rs.inst.ID, "reason", reason, "detail", detail)
	return e.fencedUpdate(ctx, rs, `status = 'blocked', blocked_reason = $4, blocked_detail = $5, wake_at = NULL`, reason, detail)
}

func nonNull(b json.RawMessage) json.RawMessage {
	if len(b) == 0 {
		return json.RawMessage(`null`)
	}
	return b
}

// finishSuspended parks the instance until its earliest wake condition. The
// wake time is computed after the row lock is taken, in the same transaction,
// so a signal or decision that raced with the suspension is never lost: it
// either is visible here (wake now) or blocks on the lock and wakes the
// already waiting instance.
func (e *Engine) finishSuspended(ctx context.Context, rs *runState) error {
	ctx = context.WithoutCancel(ctx)
	tx, err := e.cfg.DB.Begin(ctx)
	if err != nil {
		return serrors.FromDB(opFinish, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `
		UPDATE extruntime.workflow_instances SET updated_at = now()
		WHERE id = $1 AND tenant_id = $2 AND lease_epoch = $3 AND status = 'running'`, rs.inst.ID, rs.inst.TenantID, rs.epoch)
	if err != nil {
		return serrors.FromDB(opFinish, err)
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	var wake *time.Time
	err = tx.QueryRow(ctx, `
		SELECT CASE
			WHEN NOT EXISTS (SELECT 1 FROM extruntime.workflow_occurrences o WHERE o.instance_id = $1 AND o.status = 'waiting') THEN now()
			WHEN EXISTS (
				SELECT 1 FROM extruntime.workflow_occurrences o
				WHERE o.instance_id = $1 AND o.status = 'waiting' AND o.kind = 'waitFor'
				  AND EXISTS (SELECT 1 FROM extruntime.workflow_signals s
				              WHERE s.instance_id = $1 AND s.name = o.meta->>'signal' AND s.consumed_name IS NULL)) THEN now()
			WHEN EXISTS (
				SELECT 1 FROM extruntime.workflow_occurrences o
				JOIN extruntime.human_tasks t ON t.instance_id = o.instance_id AND t.name = o.name AND t.n = o.n
				WHERE o.instance_id = $1 AND o.status = 'waiting' AND o.kind = 'task' AND t.status <> 'open') THEN now()
			ELSE (SELECT min(o.wake_at) FROM extruntime.workflow_occurrences o WHERE o.instance_id = $1 AND o.status = 'waiting')
		END`, rs.inst.ID).Scan(&wake)
	if err != nil {
		return serrors.FromDB(opFinish, err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE extruntime.workflow_instances
		SET status = 'waiting', wake_at = $4, lease_owner = NULL, lease_until = NULL, failures = 0, updated_at = now()
		WHERE id = $1 AND tenant_id = $2 AND lease_epoch = $3`, rs.inst.ID, rs.inst.TenantID, rs.epoch, wake); err != nil {
		return serrors.FromDB(opFinish, err)
	}
	return serrors.FromDB(opFinish, tx.Commit(ctx))
}

// finishTransient releases the lease after an infrastructure failure so the
// instance is replayed after a backoff; repeated failures block it.
func (e *Engine) finishTransient(ctx context.Context, rs *runState, cause error) error {
	failures := rs.inst.Failures + 1
	if failures >= e.cfg.MaxRunFailures {
		return e.finishBlocked(ctx, rs, BlockFailing, fmt.Sprintf("%d consecutive runs failed; last: %v", failures, cause))
	}
	delay := e.cfg.RetryBackoff
	for i := 1; i < failures; i++ {
		delay *= 2
	}
	_, err := e.cfg.DB.Exec(context.WithoutCancel(ctx), `
		UPDATE extruntime.workflow_instances
		SET lease_owner = NULL, lease_until = now() + $4::interval, failures = $5, updated_at = now()
		WHERE id = $1 AND tenant_id = $2 AND lease_epoch = $3 AND status = 'running'`,
		rs.inst.ID, rs.inst.TenantID, rs.epoch, delay.String(), failures)
	return serrors.FromDB(opFinish, err)
}
