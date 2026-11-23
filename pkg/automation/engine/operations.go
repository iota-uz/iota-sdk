package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

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
	err = o.r.cfg.DB.QueryRow(ctx, `
		SELECT n FROM extruntime.workflow_occurrences
		WHERE tenant_id = $1 AND instance_id = $2 AND name = 'call' AND status = 'started' ORDER BY n DESC LIMIT 1`, a.TenantID, child).Scan(&n)
	if errors.Is(err, pgx.ErrNoRows) {
		return blockedCall{}, fmt.Errorf("%w: the call is blocked for another reason; resume it in the workflows", ErrInvalidInput)
	}
	if err != nil {
		return blockedCall{}, serrors.FromDB(opOperate, err)
	}
	return blockedCall{run: run, wait: w, child: child, callN: n, nodeID: w.NodeID}, nil
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
	if err := o.r.cfg.Workflows.Resolve(ctx, a.TenantID, b.child, "call", b.callN, resolvedCall(outcome, output)); err != nil {
		return serrors.Wrap(opOperate, err)
	}
	return serrors.Wrap(opOperate, o.r.cfg.Workflows.Resume(ctx, a.TenantID, b.child))
}

func (o *operations) Retry(ctx context.Context, a Actor, runID uuid.UUID, waitID string) error {
	b, err := o.blocked(ctx, a, runID, waitID)
	if err != nil {
		return err
	}
	if err := o.r.cfg.Workflows.Retry(ctx, a.TenantID, b.child, "call", b.callN); err != nil {
		return serrors.Wrap(opOperate, err)
	}
	return serrors.Wrap(opOperate, o.r.cfg.Workflows.Resume(ctx, a.TenantID, b.child))
}

func (o *operations) interp(ctx context.Context, run runRow) (*interpEntry, error) {
	v, err := loadVersion(ctx, o.r.cfg.DB, run.TenantID, run.AutomationID, run.Version)
	if err != nil {
		return nil, err
	}
	return o.r.interpreter(ctx, run.TenantID, run.AutomationID, run.Version, v.Graph)
}
