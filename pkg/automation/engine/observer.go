package engine

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const opObserve = serrors.Op("automation.engine.observe")

// observer follows the instances of automation.run: when one finishes it closes what the run left open,
// publishes automation.run.finished.v1 and starts the next queued run of its conversation subject, in
// the transaction of the transition, so the event exists also for a terminate from the workflow
// management API.
type observer struct{ r *Runtime }

func (o *observer) InstanceStatusChanged(ctx context.Context, tx pgx.Tx, inst workflow.Instance, from, to string) error {
	var status RunStatus
	switch to {
	case workflow.StatusCompleted:
		status = RunDone
	case workflow.StatusFailed:
		status = RunFailed
	case workflow.StatusTerminated:
		status = RunCancelled
	default:
		return nil
	}
	run, err := scanRun(tx.QueryRow(ctx, `SELECT `+runCols+runFrom+` WHERE r.tenant_id = $1 AND r.instance_id = $2`, inst.TenantID, inst.ID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return serrors.FromDB(opObserve, err)
	}
	// The graph's row before any run row: Disable and Delete lock the graph, then its runs (disableTx).
	if _, err := tx.Exec(ctx, `SELECT 1 FROM automation.automations WHERE tenant_id = $1 AND id = $2 FOR SHARE`, inst.TenantID, run.AutomationID); err != nil {
		return serrors.FromDB(opObserve, err)
	}
	// What the run left open: a terminated run's tokens and waits (the engine cancels its occurrences).
	if _, err := tx.Exec(ctx, `UPDATE automation.tokens SET status = 'cancelled', flow = NULL, updated_at = now()
		WHERE tenant_id = $1 AND run_id = $2 AND status IN ('running', 'waiting')`, inst.TenantID, run.ID); err != nil {
		return serrors.FromDB(opObserve, err)
	}
	if _, err := tx.Exec(ctx, `UPDATE automation.waits SET status = 'cancelled', release_pending = false, updated_at = now()
		WHERE tenant_id = $1 AND run_id = $2 AND status = 'open'`, inst.TenantID, run.ID); err != nil {
		return serrors.FromDB(opObserve, err)
	}
	if _, err := tx.Exec(ctx, `UPDATE automation.steps SET outcome = 'cancelled', ended_at = now()
		WHERE tenant_id = $1 AND run_id = $2 AND ended_at IS NULL`, inst.TenantID, run.ID); err != nil {
		return serrors.FromDB(opObserve, err)
	}
	output := json.RawMessage(`null`)
	withheld := false
	if status == RunDone && len(run.Output) > 0 {
		// Whatever the end node returns may derive from the sensitive fields the run read; the generic
		// event carries no provenance, so the output does not leave the run then.
		if len(run.Sensitive) > 0 {
			withheld = true
		} else {
			output = run.Output
		}
	}
	if status == RunFailed && len(run.Error) == 0 && len(inst.Error) > 0 {
		var we struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(inst.Error, &we)
		b, err := json.Marshal(StepError{Code: we.Code, Message: we.Message})
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE automation.runs SET error = $3 WHERE tenant_id = $1 AND id = $2`, inst.TenantID, run.ID, b); err != nil {
			return serrors.FromDB(opObserve, err)
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE automation.runs SET updated_at = now() WHERE tenant_id = $1 AND id = $2`, inst.TenantID, run.ID); err != nil {
		return serrors.FromDB(opObserve, err)
	}
	payload := RunFinishedPayload{RunID: run.ID.String(), AutomationID: run.AutomationID.String(), Version: run.Version, Status: string(status), Output: output, OutputWithheld: withheld}
	if _, err := events.Publish(ctx, tx, spi.Scope{Subject: spi.Subject{TenantID: inst.TenantID}}, RunFinished,
		&spi.ObjectRef{Entity: SubjectRun, Key: run.ID.String()}, payload); err != nil {
		return serrors.Wrap(opObserve, err)
	}
	if status == RunFailed {
		if err := o.notifyOwner(ctx, tx, run); err != nil {
			return err
		}
	}
	o.r.metrics.runFinished(ctx, status)
	o.r.cfg.Logger.Info("automation run finished", "run_id", run.ID.String(), "automation_id", run.AutomationID.String(), "status", string(status))
	// The next queued run of a conversation starts now when its subject's lock is free; else the
	// maintenance pass starts it (the lock holder may be queueing a run right now).
	s := normSubject(run.Subject)
	if s.Entity == graph.SubjectConversation {
		var locked bool
		if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(hashtextextended($1, 0))`, subjectLockKey(inst.TenantID, s, run.ID)).Scan(&locked); err != nil {
			return serrors.FromDB(opObserve, err)
		}
		if locked {
			if err := o.r.dequeue(ctx, tx, inst.TenantID, s, run.ID); err != nil {
				return err
			}
		}
		// Resumed predecessors waiting for the conversation try to take it now (the oldest wins).
		if err := o.r.wakeAwaiting(ctx, tx, inst.TenantID, s, run.ID); err != nil {
			return err
		}
	}
	return nil
}

// notifyOwner publishes automation.run.failed.v1 for the graph's owner, once per run: owner_notified_at
// is set in the transaction of the transition, so a transition the engine retries (its transaction
// rolled back) publishes again only together with it, and a second failed transition never. The
// notification definition of package ownernotice delivers the event to the owner.
func (o *observer) notifyOwner(ctx context.Context, tx pgx.Tx, run runRow) error {
	var owner *int64
	var name string
	err := tx.QueryRow(ctx, `
		UPDATE automation.runs r SET owner_notified_at = now()
		FROM automation.automations a
		WHERE r.tenant_id = $1 AND r.id = $2 AND r.owner_notified_at IS NULL AND a.tenant_id = r.tenant_id AND a.id = r.automation_id
		RETURNING a.owner_user_id, a.name`, run.TenantID, run.ID).Scan(&owner, &name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return serrors.FromDB(opObserve, err)
	}
	if owner == nil {
		return nil // a graph never enabled by a person has nobody to tell
	}
	code := "failed"
	if failed, err := loadRun(ctx, tx, run.TenantID, run.ID); err != nil {
		return err
	} else if e := failed.stepError(); e != nil && e.Code != "" {
		code = e.Code
	}
	payload := RunFailedPayload{RunID: run.ID.String(), AutomationID: run.AutomationID.String(), AutomationName: name,
		Version: run.Version, OwnerUserID: *owner, Code: code}
	if _, err := events.Publish(ctx, tx, spi.Scope{Subject: spi.Subject{TenantID: run.TenantID}}, RunFailedEvent,
		&spi.ObjectRef{Entity: SubjectRun, Key: run.ID.String()}, payload); err != nil {
		return serrors.Wrap(opObserve, err)
	}
	return nil
}
