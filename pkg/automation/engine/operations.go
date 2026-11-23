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
	run    runRow
	wait   waitRow
	child  uuid.UUID
	callN  int
	nodeID string
	// pending is an operation that changed the journal and stopped before Resume: "retry" (the
	// occurrence is gone, its new deadline written) or "resolve" (the occurrence is resolved). Repeating
	// the operation resumes the call.
	pending string
}

const (
	pendingRetry   = "retry"
	pendingResolve = "resolve"
)

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
	err = o.r.cfg.DB.QueryRow(ctx, `
		SELECT n FROM extruntime.workflow_occurrences
		WHERE tenant_id = $1 AND instance_id = $2 AND name = 'call' AND status = 'started' ORDER BY n DESC LIMIT 1`, a.TenantID, child).Scan(&n)
	b := blockedCall{run: run, wait: w, child: child, callN: n, nodeID: w.NodeID}
	if errors.Is(err, pgx.ErrNoRows) {
		// No started attempt: an earlier Resolve or Retry that stopped before its Resume, or another reason.
		var resolved, renewed bool
		if err := o.r.cfg.DB.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM extruntime.workflow_occurrences WHERE tenant_id = $1 AND instance_id = $2 AND name = 'call'
				AND status = 'completed' AND meta->>'resolved' = 'true'),
			       EXISTS (SELECT 1 FROM automation.call_deadlines WHERE tenant_id = $1 AND instance_id = $2)`,
			a.TenantID, child).Scan(&resolved, &renewed); err != nil {
			return blockedCall{}, serrors.FromDB(opOperate, err)
		}
		switch {
		case resolved:
			b.pending = pendingResolve
		case renewed:
			b.pending = pendingRetry
		default:
			return blockedCall{}, fmt.Errorf("%w: the call is blocked for another reason; resume it in the workflows", ErrInvalidInput)
		}
		return b, nil
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
	switch b.pending {
	case pendingResolve:
		return o.resume(ctx, a, b) // the outcome is recorded: the call goes on with it
	case pendingRetry:
		return fmt.Errorf("%w: a retry of the call is pending; repeat Retry to run it", ErrInvalidInput)
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
	if err := o.r.cfg.Workflows.Resolve(ctx, a.TenantID, b.child, "call", b.callN, resolvedCall(outcome, output)); err != nil {
		return serrors.Wrap(opOperate, err)
	}
	return o.resume(ctx, a, b)
}

func (o *operations) Retry(ctx context.Context, a Actor, runID uuid.UUID, waitID string) error {
	b, err := o.blocked(ctx, a, runID, waitID)
	if err != nil {
		return err
	}
	switch b.pending {
	case pendingRetry:
		return o.resume(ctx, a, b)
	case pendingResolve:
		return fmt.Errorf("%w: the call is resolved already; repeat Resolve to continue it", ErrInvalidInput)
	}
	// The retried attempt gets the node's timeout again from now: its first deadline may be gone.
	var req CallRequest
	var raw []byte
	if err := o.r.cfg.DB.QueryRow(ctx, `SELECT input FROM extruntime.workflow_instances WHERE tenant_id = $1 AND id = $2`, a.TenantID, b.child).Scan(&raw); err != nil {
		return serrors.FromDB(opOperate, err)
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		return serrors.Wrap(opOperate, err)
	}
	timeout := time.Duration(req.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = o.r.cfg.CallLimits.DefaultTimeout
	}
	if _, err := o.r.cfg.DB.Exec(ctx, `
		INSERT INTO automation.call_deadlines (tenant_id, instance_id, deadline) VALUES ($1, $2, now() + $3::interval)
		ON CONFLICT (tenant_id, instance_id) DO UPDATE SET deadline = EXCLUDED.deadline`, a.TenantID, b.child, pgInterval(timeout.Seconds())); err != nil {
		return serrors.FromDB(opOperate, err)
	}
	if err := o.r.cfg.Workflows.Retry(ctx, a.TenantID, b.child, "call", b.callN); err != nil {
		return serrors.Wrap(opOperate, err)
	}
	return o.resume(ctx, a, b)
}

// resume makes the blocked call runnable: the last step of Resolve and Retry, and what a repeat of an
// interrupted one does.
func (o *operations) resume(ctx context.Context, a Actor, b blockedCall) error {
	return serrors.Wrap(opOperate, o.r.cfg.Workflows.Resume(ctx, a.TenantID, b.child))
}

func (o *operations) interp(ctx context.Context, run runRow) (*interpEntry, error) {
	v, err := loadVersion(ctx, o.r.cfg.DB, run.TenantID, run.AutomationID, run.Version)
	if err != nil {
		return nil, err
	}
	return o.r.interpreter(ctx, run.TenantID, run.AutomationID, run.Version, v.Graph)
}
