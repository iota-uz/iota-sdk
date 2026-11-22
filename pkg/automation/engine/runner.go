package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const opRun = serrors.Op("automation.engine.run")

// runner implements workflow.NativeRunner for automation.run.
type runner struct{ r *Runtime }

// infra marks an error of the platform (the database, the catalog): the engine retries the run instead of
// failing it. Suspension, fencing and typed wire errors pass through.
func infra(err error) error {
	if err == nil || errors.Is(err, workflow.ErrSuspend) || errors.Is(err, workflow.ErrFenced) {
		return err
	}
	if _, ok := pool.AsWireError(err); ok {
		return err
	}
	if host.IsInfrastructure(err) {
		return err
	}
	return &host.InfraError{Err: err}
}

func (rn *runner) Run(ctx context.Context, nr workflow.NativeRun) (json.RawMessage, error) {
	inst := nr.Instance()
	var in struct {
		RunID uuid.UUID `json:"run_id"`
	}
	if err := json.Unmarshal(inst.Input, &in); err != nil || in.RunID == uuid.Nil {
		return nil, &pool.WireError{Kind: pool.ErrKindDomain, Code: "automation.invalidInput", Message: "automation.run needs a run_id"}
	}
	run, err := loadRun(ctx, rn.r.cfg.DB, inst.TenantID, in.RunID)
	if err != nil {
		return nil, infra(err)
	}
	ctx, span := rn.r.tracer.Start(ctx, "automation.run", trace.WithAttributes(
		attribute.String("tenant", inst.TenantID.String()), attribute.String("automation", run.AutomationID.String()),
		attribute.Int("version", run.Version), attribute.String("run", run.ID.String()), attribute.String("trigger", run.TriggerID)))
	defer span.End()
	x := &execution{r: rn.r, nr: nr, tenant: inst.TenantID, run: run,
		log: rn.r.cfg.Logger.With("run_id", run.ID.String(), "automation_id", run.AutomationID.String())}
	out, err := x.loop(ctx)
	if err != nil && !errors.Is(err, workflow.ErrSuspend) {
		span.RecordError(err)
	}
	return out, err
}

// execution is one run of an instance: the run row as of the last committed transaction.
type execution struct {
	r      *Runtime
	nr     workflow.NativeRun
	tenant uuid.UUID
	run    runRow
	ie     *interpEntry
	log    *slog.Logger
}

func (x *execution) interp(ctx context.Context) (*interpEntry, error) {
	if x.ie != nil {
		return x.ie, nil
	}
	v, err := loadVersion(ctx, x.r.cfg.DB, x.tenant, x.run.AutomationID, x.run.Version)
	if err != nil {
		return nil, &host.InfraError{Err: err}
	}
	ie, err := x.r.interpreter(ctx, x.tenant, x.run.AutomationID, x.run.Version, v.Graph)
	if err != nil {
		return nil, err
	}
	x.ie = ie
	return ie, nil
}

// failure is the error a failed run returns to the workflow engine.
func failure(e StepError) error {
	return &pool.WireError{Kind: pool.ErrKindDomain, Code: "automation." + e.Code, Message: e.Message}
}

// loop drives the run until it suspends, completes or fails.
func (x *execution) loop(ctx context.Context) (json.RawMessage, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		run, err := loadRun(ctx, x.r.cfg.DB, x.tenant, x.run.ID)
		if err != nil {
			return nil, infra(err)
		}
		x.run = run
		if err := x.release(ctx); err != nil {
			return nil, infra(err)
		}
		if len(run.Error) > 0 {
			return nil, failure(*run.stepError())
		}
		if len(run.Output) > 0 {
			return run.Output, nil
		}
		if _, err := x.interp(ctx); err != nil {
			if host.IsInfrastructure(err) {
				return nil, err
			}
			// The version cannot run against the tenant's catalog (an event it uses left it): the run fails.
			return nil, x.failRun(ctx, StepError{Code: CodeConnectionUnavailable, Message: err.Error()})
		}
		progressed, err := x.stepOne(ctx)
		if err != nil {
			return nil, infra(err)
		}
		if progressed {
			continue
		}
		progressed, err = x.timers(ctx)
		if err != nil {
			return nil, infra(err)
		}
		if progressed {
			continue
		}
		res, err := x.nr.WaitFor(ctx, "inbox", int(x.run.InboxSeq), InboxSignal, 0)
		if err != nil {
			return nil, infra(err)
		}
		if err := x.consume(ctx, res.Payload); err != nil {
			return nil, infra(err)
		}
	}
}

// tx runs fn in the run's next fenced transaction: the subject's lock first, the cause of the events the
// step publishes, then fn, then the run's counters; the fence and the receipt commit with it.
func (x *execution) tx(ctx context.Context, label string, fn func(ctx context.Context, tx pgx.Tx, s *stepState) error) error {
	n := x.run.TxSeq
	_, err := x.nr.Transaction(ctx, "tx", int(n), func(ctx context.Context, tx pgx.Tx) (json.RawMessage, error) {
		if err := lockSubjects(ctx, tx, subjectLockKey(x.tenant, x.run.Subject, x.run.ID)); err != nil {
			return nil, err
		}
		var seq int64
		if err := tx.QueryRow(ctx, `SELECT tx_seq FROM automation.runs WHERE tenant_id = $1 AND id = $2 FOR UPDATE`, x.tenant, x.run.ID).Scan(&seq); err != nil {
			return nil, serrors.FromDB(opRun, err)
		}
		if seq != n {
			return nil, fmt.Errorf("run %s: transaction %d expected, the run is at %d", x.run.ID, n, seq)
		}
		cause, _ := json.Marshal(map[string]any{"run_id": x.run.ID.String(), "depth": x.run.Depth})
		if _, err := tx.Exec(ctx, `SELECT set_config('app.automation_cause', $1, true)`, string(cause)); err != nil {
			return nil, serrors.FromDB(opRun, err)
		}
		s := &stepState{}
		if err := tx.QueryRow(ctx, `SELECT now()`).Scan(&s.now); err != nil {
			return nil, serrors.FromDB(opRun, err)
		}
		if err := fn(ctx, tx, s); err != nil {
			return nil, err
		}
		if h := x.r.testHook; h != nil {
			if err := h(ctx, tx, label); err != nil {
				return nil, err
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE automation.runs SET tx_seq = tx_seq + 1, inbox_seq = inbox_seq + $3, steps_count = steps_count + $4,
				output = coalesce($5, output), error = coalesce($6, error), updated_at = now()
			WHERE tenant_id = $1 AND id = $2`, x.tenant, x.run.ID, s.inbox, s.steps, nullJSON(s.output), nullJSON(s.failure)); err != nil {
			return nil, serrors.FromDB(opRun, err)
		}
		return nil, nil
	})
	if err != nil {
		return err
	}
	x.run.TxSeq++
	return nil
}

// stepState is what one transaction decided about the run.
type stepState struct {
	now     time.Time
	inbox   int64           // 1 when the transaction consumed the inbox signal
	steps   int             // node steps taken
	output  json.RawMessage // the run completed with this output
	failure json.RawMessage // the run failed with this error
}

func nullJSON(b json.RawMessage) any {
	if len(b) == 0 {
		return nil
	}
	return []byte(b)
}

// failRun fails the run outside a step (its version cannot run any more).
func (x *execution) failRun(ctx context.Context, e StepError) error {
	err := x.tx(ctx, "fail", func(ctx context.Context, tx pgx.Tx, s *stepState) error {
		m := &mover{x: x, tx: tx, s: s}
		return m.endRun(ctx, nil, &e)
	})
	if err != nil {
		return infra(err)
	}
	return failure(e)
}

// stepOne takes one step of the oldest running token.
func (x *execution) stepOne(ctx context.Context) (bool, error) {
	toks, err := queryTokens(ctx, x.r.cfg.DB, `SELECT `+tokenCols+` FROM automation.tokens WHERE tenant_id = $1 AND run_id = $2 AND status = 'running' ORDER BY created_seq LIMIT 1`, x.tenant, x.run.ID)
	if err != nil || len(toks) == 0 {
		return false, err
	}
	id := toks[0].ID
	started := time.Now()
	behavior := ""
	ctx, span := x.r.tracer.Start(ctx, "automation.step", trace.WithAttributes(attribute.String("run", x.run.ID.String()), attribute.String("node", toks[0].NodeID)))
	defer span.End()
	err = x.tx(ctx, "step", func(ctx context.Context, tx pgx.Tx, s *stepState) error {
		tok, err := loadToken(ctx, tx, x.tenant, x.run.ID, id, true)
		if err != nil {
			return err
		}
		if tok.Status != tokRunning {
			return nil
		}
		m := &mover{x: x, tx: tx, s: s}
		if x.run.StepsCount+1 > x.r.cfg.Limits.StepsPerRun {
			return m.endRun(ctx, nil, &StepError{Code: CodeLimitExceeded, Message: fmt.Sprintf("a run takes at most %d steps", x.r.cfg.Limits.StepsPerRun)})
		}
		s.steps = 1
		behavior, err = m.enter(ctx, &tok)
		return err
	})
	if err == nil {
		x.r.metrics.stepDone(ctx, behavior, time.Since(started))
		x.log.Debug("automation step", "node_id", toks[0].NodeID, "behavior", behavior)
	}
	return true, err
}

// timers fires the first due timer of the run's open waits and opens the tasks waits asked for.
func (x *execution) timers(ctx context.Context) (bool, error) {
	waits, err := queryWaits(ctx, x.r.cfg.DB, `SELECT `+waitCols+` FROM automation.waits WHERE tenant_id = $1 AND run_id = $2 AND status = 'open' ORDER BY created_at, wait_id`, x.tenant, x.run.ID)
	if err != nil {
		return false, err
	}
	for _, w := range waits {
		if w.Spec.Exec != nil && w.Spec.Exec.Task != nil {
			res, err := x.nr.Task(ctx, "task:"+w.WaitID, 0, *w.Spec.Exec.Task)
			switch {
			case errors.Is(err, workflow.ErrSuspend):
			case err != nil:
				return false, err
			case res.Outcome == workflow.TaskCancelled:
			default:
				if err := x.taskDone(ctx, w, res); err != nil {
					return false, err
				}
				return true, nil
			}
		}
		if w.TimeoutAt == nil || !w.Spec.Timer {
			continue
		}
		d := time.Until(*w.TimeoutAt)
		if d < 0 {
			d = 0
		}
		err := x.nr.Sleep(ctx, "timer:"+w.WaitID, 0, d)
		switch {
		case err == nil:
			if err := x.timeout(ctx, w); err != nil {
				return false, err
			}
			return true, nil
		case errors.Is(err, workflow.ErrSuspend), errors.Is(err, workflow.ErrCancelled):
		default:
			return false, err
		}
	}
	return false, nil
}

// release cancels what closed waits armed: timer and task occurrences, child instances.
func (x *execution) release(ctx context.Context) error {
	waits, err := queryWaits(ctx, x.r.cfg.DB, `SELECT `+waitCols+` FROM automation.waits WHERE tenant_id = $1 AND run_id = $2 AND release_pending`, x.tenant, x.run.ID)
	if err != nil || len(waits) == 0 {
		return err
	}
	ids := make([]string, 0, len(waits))
	for _, w := range waits {
		if w.Spec.Timer && w.Status != waitTimedOut {
			if err := x.nr.Cancel(ctx, "timer:"+w.WaitID, 0); err != nil {
				return err
			}
		}
		if w.Spec.Exec != nil && w.Spec.Exec.Task != nil && w.Status != waitFired {
			if err := x.nr.Cancel(ctx, "task:"+w.WaitID, 0); err != nil {
				return err
			}
		}
		if c := w.child(); c != nil && w.Status != waitFired {
			if err := x.r.cfg.Workflows.TerminateTree(ctx, x.tenant, *c); err != nil && !errors.Is(err, workflow.ErrNotAccepting) && !errors.Is(err, workflow.ErrNotFound) {
				return err
			}
		}
		ids = append(ids, w.WaitID)
	}
	return x.tx(ctx, "release", func(ctx context.Context, tx pgx.Tx, _ *stepState) error {
		_, err := tx.Exec(ctx, `UPDATE automation.waits SET release_pending = false WHERE tenant_id = $1 AND run_id = $2 AND wait_id = ANY($3)`, x.tenant, x.run.ID, ids)
		return serrors.FromDB(opRun, err)
	})
}

// inboxMessage is one inbox signal: a matched event or a WaitResult.
type inboxMessage struct {
	WaitResult
	Event *inboxEvent `json:"event,omitempty"`
}

// inboxEvent is the event that woke a wait.
type inboxEvent struct {
	ID      string          `json:"id"`
	Seq     int64           `json:"seq"`
	Kind    string          `json:"kind"`
	Payload json.RawMessage `json:"payload"`
}

// consume applies one inbox signal and moves the inbox on.
func (x *execution) consume(ctx context.Context, payload json.RawMessage) error {
	var msg inboxMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		x.log.Warn("automation: unreadable inbox signal", "error", err)
	}
	return x.tx(ctx, "consume", func(ctx context.Context, tx pgx.Tx, s *stepState) error {
		s.inbox = 1
		if msg.WaitID == "" {
			return nil
		}
		ws, err := queryWaits(ctx, tx, `SELECT `+waitCols+` FROM automation.waits WHERE tenant_id = $1 AND run_id = $2 AND wait_id = $3 FOR UPDATE`, x.tenant, x.run.ID, msg.WaitID)
		if err != nil || len(ws) == 0 || ws[0].Status != waitOpen {
			return err // a wait that closed meanwhile (a timeout, a join) ignores late input
		}
		w := ws[0]
		tok, err := loadToken(ctx, tx, x.tenant, x.run.ID, w.TokenID, true)
		if err != nil {
			return err
		}
		if tok.Status != tokWaiting {
			return nil
		}
		m := &mover{x: x, tx: tx, s: s}
		if msg.Event != nil {
			return m.wakeEvent(ctx, &tok, w, *msg.Event)
		}
		return m.result(ctx, &tok, w, msg.WaitResult)
	})
}

// timeout fires a wait's timer.
func (x *execution) timeout(ctx context.Context, w waitRow) error {
	return x.tx(ctx, "timeout", func(ctx context.Context, tx pgx.Tx, s *stepState) error {
		ws, err := queryWaits(ctx, tx, `SELECT `+waitCols+` FROM automation.waits WHERE tenant_id = $1 AND run_id = $2 AND wait_id = $3 FOR UPDATE`, x.tenant, x.run.ID, w.WaitID)
		if err != nil || len(ws) == 0 || ws[0].Status != waitOpen {
			return err
		}
		w = ws[0]
		tok, err := loadToken(ctx, tx, x.tenant, x.run.ID, w.TokenID, true)
		if err != nil {
			return err
		}
		if tok.Status != tokWaiting {
			return closeWait(ctx, tx, x.tenant, w, waitCancelled)
		}
		m := &mover{x: x, tx: tx, s: s}
		return m.timedOut(ctx, &tok, w)
	})
}

// taskDone applies the outcome of a task the core opened.
func (x *execution) taskDone(ctx context.Context, w waitRow, res workflow.TaskResult) error {
	ie, err := x.interp(ctx)
	if err != nil {
		return err
	}
	node, _ := ie.in.View().Node(w.NodeID)
	var wr WaitResult
	if tr, ok := x.r.cfg.Tasks.(TaskResolver); ok && x.r.cfg.Tasks != nil {
		wr = tr.Resolve(node, res)
	} else {
		wr = defaultTaskResult(res)
	}
	wr.WaitID = w.WaitID
	return x.tx(ctx, "task", func(ctx context.Context, tx pgx.Tx, s *stepState) error {
		ws, err := queryWaits(ctx, tx, `SELECT `+waitCols+` FROM automation.waits WHERE tenant_id = $1 AND run_id = $2 AND wait_id = $3 FOR UPDATE`, x.tenant, x.run.ID, w.WaitID)
		if err != nil || len(ws) == 0 || ws[0].Status != waitOpen {
			return err
		}
		tok, err := loadToken(ctx, tx, x.tenant, x.run.ID, w.TokenID, true)
		if err != nil || tok.Status != tokWaiting {
			return err
		}
		m := &mover{x: x, tx: tx, s: s}
		return m.result(ctx, &tok, ws[0], wr)
	})
}

// defaultTaskResult maps a task's result: the decision's "outcome" field and the decision as the output.
func defaultTaskResult(res workflow.TaskResult) WaitResult {
	if res.Outcome == workflow.TaskTimeout {
		return WaitResult{Outcome: CodeTimeout, Output: json.RawMessage(`{}`)}
	}
	var d struct {
		Outcome string `json:"outcome"`
	}
	_ = json.Unmarshal(res.Decision, &d)
	out := map[string]any{"decision": json.RawMessage(nonNull(res.Decision))}
	if res.By != nil {
		out["by"] = *res.By
	}
	b, _ := json.Marshal(out)
	return WaitResult{Outcome: d.Outcome, Output: b}
}

func nonNull(b json.RawMessage) json.RawMessage {
	if len(b) == 0 {
		return json.RawMessage(`null`)
	}
	return b
}

// decodeMap reads a JSON object as the format reads values (numbers as json.Number).
func decodeMap(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := expr.UnmarshalJSON(raw, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}
