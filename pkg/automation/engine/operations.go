package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const opOperate = serrors.Op("automation.engine.operate")

// Operations on a blocked run (Automations.Operate); act on the blocked call instance of the run.
type Operations interface {
	// Resolve records the outcome of an ambiguous call by hand; the run continues by that outcome.
	Resolve(ctx context.Context, a Actor, runID uuid.UUID, waitID string, outcome string, output json.RawMessage) error
	// Retry repeats the blocked call with the same idempotency key (the operator accepts a possible duplicate).
	Retry(ctx context.Context, a Actor, runID uuid.UUID, waitID string) error
}

// Operations is the API of blocked calls.
func (r *Runtime) Operations() Operations { return &operations{r: r} }

type operations struct{ r *Runtime }

// blockedCall is the blocked child of a run's open call wait and the occurrence it is blocked on.
type blockedCall struct {
	run   runRow
	wait  waitRow
	child uuid.UUID
	callN int
	// callAt is the attempt's updated_at as read: Resolve and Retry act only on that attempt (a newer
	// one, after a resume and a new block, is not touched).
	callAt time.Time
	nodeID string
}

func (o *operations) blocked(ctx context.Context, a Actor, runID uuid.UUID, waitID string) (blockedCall, error) {
	if err := o.r.service.authorize(a, PermOperate); err != nil {
		return blockedCall{}, err
	}
	run, err := loadRun(ctx, o.r.cfg.DB, a.TenantID, runID)
	if err != nil {
		return blockedCall{}, err
	}
	ws, err := queryWaits(ctx, o.r.cfg.DB, `SELECT `+waitCols+` FROM automation.waits WHERE tenant_id = $1 AND run_id = $2 AND wait_id = $3`, a.TenantID, runID, waitID)
	if err != nil {
		return blockedCall{}, err
	}
	if len(ws) == 0 || ws[0].Kind != waitCall || ws[0].child() == nil {
		return blockedCall{}, ErrNotFound
	}
	w := ws[0]
	if w.Status != waitOpen {
		return blockedCall{}, fmt.Errorf("%w: the call is no longer waited for", ErrInvalidInput)
	}
	child := *w.child()
	var status string
	err = o.r.cfg.DB.QueryRow(ctx, `SELECT status FROM extruntime.workflow_instances WHERE tenant_id = $1 AND id = $2`, a.TenantID, child).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return blockedCall{}, ErrNotFound
	}
	if err != nil {
		return blockedCall{}, serrors.FromDB(opOperate, err)
	}
	if status != workflow.StatusBlocked {
		return blockedCall{}, fmt.Errorf("%w: the call is %s, not blocked", ErrInvalidInput, status)
	}
	var n int
	var at time.Time
	err = o.r.cfg.DB.QueryRow(ctx, `
		SELECT n, updated_at FROM extruntime.workflow_occurrences
		WHERE tenant_id = $1 AND instance_id = $2 AND name = 'call' AND status = 'started' ORDER BY n DESC LIMIT 1`, a.TenantID, child).Scan(&n, &at)
	b := blockedCall{run: run, wait: w, child: child, callN: n, callAt: at, nodeID: w.NodeID}
	if errors.Is(err, pgx.ErrNoRows) {
		return blockedCall{}, fmt.Errorf("%w: the call is blocked for another reason; resume it in the workflows", ErrInvalidInput)
	}
	if err != nil {
		return blockedCall{}, serrors.FromDB(opOperate, err)
	}
	return b, nil
}

func (o *operations) Resolve(ctx context.Context, a Actor, runID uuid.UUID, waitID string, outcome string, output json.RawMessage) error {
	b, err := o.blocked(ctx, a, runID, waitID)
	if err != nil {
		return err
	}
	if len(output) == 0 {
		output = json.RawMessage(`{}`)
	}
	if out, err := decodeMap(output); err != nil || out == nil {
		return fmt.Errorf("%w: the output is a JSON object", ErrInvalidInput)
	}
	ie, err := o.interp(ctx, b.run)
	if err != nil {
		return err
	}
	if !slices.Contains(ie.in.View().Outcomes(b.nodeID), outcome) {
		return fmt.Errorf("%w: %s has no outcome %q", ErrInvalidInput, b.nodeID, outcome)
	}
	// The outcome is recorded and the call resumed in one transaction, for the attempt read above only.
	if err := o.r.cfg.Workflows.ResolveIf(ctx, a.TenantID, b.child, "call", b.callN, b.callAt, resolvedCall(outcome, output), nil); err != nil {
		if errors.Is(err, workflow.ErrConflict) || errors.Is(err, workflow.ErrNotAccepting) {
			return fmt.Errorf("%w: the call's attempt changed (or it is no longer blocked); look at it again", ErrInvalidInput)
		}
		return serrors.Wrap(opOperate, err)
	}
	return nil
}

func (o *operations) Retry(ctx context.Context, a Actor, runID uuid.UUID, waitID string) error {
	b, err := o.blocked(ctx, a, runID, waitID)
	if err != nil {
		return err
	}
	// The attempt is forgotten, its deadline (and an ask's token) renewed and the call resumed in one
	// transaction, for the attempt read above only: a stale request changes nothing of a newer attempt.
	if h := o.r.opsHook; h != nil {
		h()
	}
	err = o.r.cfg.Workflows.RetryIf(ctx, a.TenantID, b.child, "call", b.callN, b.callAt, func(ctx context.Context, tx pgx.Tx) error {
		return o.renew(ctx, tx, a, b)
	})
	if errors.Is(err, workflow.ErrConflict) || errors.Is(err, workflow.ErrNotAccepting) {
		return fmt.Errorf("%w: the call's attempt changed (or it is no longer blocked); look at it again", ErrInvalidInput)
	}
	if errors.Is(err, errAskExpired) {
		return fmt.Errorf("%w: the question's correlation expired; resolve the call instead", ErrInvalidInput)
	}
	return serrors.Wrap(opOperate, err)
}

// errAskExpired: a Retry of an ask whose correlation token is gone.
var errAskExpired = errors.New("automation: the ask's correlation expired")

// renew gives the retried attempt the node's timeout again from now: its deadline may be gone.
func (o *operations) renew(ctx context.Context, tx pgx.Tx, a Actor, b blockedCall) error {
	var req CallRequest
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT input FROM extruntime.workflow_instances WHERE tenant_id = $1 AND id = $2`, a.TenantID, b.child).Scan(&raw); err != nil {
		return serrors.FromDB(opOperate, err)
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		return serrors.Wrap(opOperate, err)
	}
	timeout := time.Duration(req.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = o.r.cfg.CallLimits.DefaultTimeout
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO automation.call_deadlines (tenant_id, instance_id, deadline) VALUES ($1, $2, now() + $3::interval)
		ON CONFLICT (tenant_id, instance_id) DO UPDATE SET deadline = EXCLUDED.deadline`, a.TenantID, b.child, pgInterval(timeout.Seconds())); err != nil {
		return serrors.FromDB(opOperate, err)
	}
	// An ask's token lives as long as its renewed attempt may still be answered.
	if req.AnswerWaitID != "" {
		tag, err := tx.Exec(ctx, `
			UPDATE automation.correlations SET expires_at = greatest(expires_at, now() + $4::interval)
			WHERE tenant_id = $1 AND run_id = $2 AND wait_id = $3 AND expires_at > now()`, a.TenantID, req.RunID, req.AnswerWaitID,
			pgInterval((timeout + o.r.cfg.Limits.WaitMax).Seconds()))
		if err != nil {
			return serrors.FromDB(opOperate, err)
		}
		if tag.RowsAffected() == 0 {
			// The question's token expired (or was pruned): a retried question could not be answered.
			// The retry is refused before the call resumes; Resolve still settles the call by hand.
			return errAskExpired
		}
	}
	return nil
}

func (o *operations) interp(ctx context.Context, run runRow) (*interpEntry, error) {
	v, err := loadVersion(ctx, o.r.cfg.DB, run.TenantID, run.AutomationID, run.Version)
	if err != nil {
		return nil, err
	}
	return o.r.interpreter(ctx, run.TenantID, run.AutomationID, run.Version, v.Graph)
}
