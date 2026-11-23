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
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
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
	// subject is the run-as with the rights it holds now, loaded at the start of every transaction.
	subject spi.Subject
	// holdsSubject: this claim checked that the run may step on its conversation (acquireSubject).
	holdsSubject bool
}

// acquireSubject makes a resumed run that a Restart superseded take its conversation back before it
// steps: while another run of the conversation is live (its replacement, a run queued after it, or an
// older waiting predecessor) it waits for SubjectSignal and takes no step. wait reports that the caller
// must wait; other runs hold their conversation from their start (the queue) and never wait here.
func (x *execution) acquireSubject(ctx context.Context) (bool, error) {
	if x.holdsSubject || normSubject(x.run.Subject).Entity != graph.SubjectConversation {
		x.holdsSubject = true
		return false, nil
	}
	var superseded bool
	if err := x.r.cfg.DB.QueryRow(ctx, `SELECT superseded FROM automation.runs WHERE tenant_id = $1 AND id = $2`,
		x.tenant, x.run.ID).Scan(&superseded); err != nil {
		return false, serrors.FromDB(opRun, err)
	}
	if !superseded {
		x.holdsSubject = true
		return false, nil
	}
	// The wait of an earlier attempt is cancelled first: left waiting with an unconsumed signal it would
	// keep waking the instance after the run took the subject.
	var pending *int64
	if err := x.r.cfg.DB.QueryRow(ctx, `SELECT subject_wait_n FROM automation.runs WHERE tenant_id = $1 AND id = $2`, x.tenant, x.run.ID).Scan(&pending); err != nil {
		return false, serrors.FromDB(opRun, err)
	}
	if pending != nil {
		if err := x.nr.Cancel(ctx, "subject", int(*pending)); err != nil {
			return false, err
		}
	}
	wait := false
	next := x.run.TxSeq + 1 // the occurrence the caller waits on: tx_seq after this transaction
	// Under the subject's lock (x.tx takes it first), so two waiters never both take the subject.
	err := x.tx(ctx, "subject", func(ctx context.Context, tx pgx.Tx, _ *stepState) error {
		var held bool
		err := tx.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1`+runFrom+`
				WHERE r.tenant_id = $1 AND r.subject_entity = $2 AND r.subject_key = $3 AND r.id <> $4
				  AND i.status IN ('running', 'waiting', 'blocked') AND NOT `+supersededSQL+`
				  AND (NOT r.awaiting_subject OR (r.created_at, r.id) < ($5, $4)))`,
			x.tenant, x.run.Subject.Entity, x.run.Subject.Key, x.run.ID, x.run.CreatedAt).Scan(&held)
		if err != nil {
			return serrors.FromDB(opRun, err)
		}
		wait = held
		var n any
		if held {
			n = next
		}
		_, err = tx.Exec(ctx, `UPDATE automation.runs SET awaiting_subject = $3, subject_wait_n = $4 WHERE tenant_id = $1 AND id = $2`, x.tenant, x.run.ID, held, n)
		return serrors.FromDB(opRun, err)
	})
	if err != nil {
		return false, err
	}
	x.holdsSubject = !wait
	return wait, nil
}

// refreshSubject reloads the run-as rights for the next step. A user that is blocked now aborts the
// run as a platform failure: the engine claims the instance again, its own subject load is refused and
// the instance blocks (BlockSubject), so no step runs with rights the user lost.
func (x *execution) refreshSubject(ctx context.Context) error {
	if x.run.RunAsUserID == nil || x.run.RunAsKind == nil {
		x.subject = x.nr.Subject()
		return nil
	}
	sub, err := x.r.cfg.Users.LoadSubject(ctx, x.tenant, *x.run.RunAsUserID, spi.SubjectKind(*x.run.RunAsKind))
	if err != nil {
		return &host.InfraError{Err: fmt.Errorf("run-as of run %s: %w", x.run.ID, err)}
	}
	sub.TenantID, sub.UserID, sub.Kind = x.tenant, *x.run.RunAsUserID, spi.SubjectKind(*x.run.RunAsKind)
	x.subject = sub
	return nil
}

// checkRetained refuses to step a run whose run-as may no longer read a sensitive field the run already
// read: its trigger input, token outputs and foreach items may hold it in any form, so nothing is
// evaluated with them. The run is retried as a platform failure and blocks (failing) after the
// workflow engine's retries; it resumes once the right is back.
func (x *execution) checkRetained(ctx context.Context) error {
	if len(x.run.Sensitive) == 0 {
		return nil
	}
	ok, err := x.r.projector(x.tenant).readable(ctx, x.run.Sensitive, &x.subject)
	if err != nil {
		return &host.InfraError{Err: err}
	}
	if !ok {
		return &host.InfraError{Err: fmt.Errorf("run %s: its run-as may no longer read a sensitive field the run read; the run waits for the right", x.run.ID)}
	}
	return nil
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
		// An inbox signal consumed before a crash or a rolled-back apply is applied before anything else:
		// its receipt is recorded, so a timer must not overtake it.
		if o, ok, err := x.nr.Lookup(ctx, "inbox", int(x.run.InboxSeq)); err != nil {
			return nil, infra(err)
		} else if ok && o.Status == workflow.OccCompleted {
			res, err := x.nr.WaitFor(ctx, "inbox", int(x.run.InboxSeq), InboxSignal, 0)
			if err != nil {
				return nil, infra(err)
			}
			if err := x.consume(ctx, res.Payload); err != nil {
				return nil, infra(err)
			}
			continue
		}
		wait, err := x.acquireSubject(ctx)
		if err != nil {
			return nil, infra(err)
		}
		if wait {
			// A new occurrence per attempt (tx_seq moved on), so a wake that finds the subject still
			// held waits again instead of spinning.
			if _, err := x.nr.WaitFor(ctx, "subject", int(x.run.TxSeq), SubjectSignal, 0); err != nil {
				return nil, infra(err)
			}
			continue
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
	if err := x.refreshSubject(ctx); err != nil {
		return err
	}
	switch label {
	case "release", "fail": // cleanup and failing the run evaluate nothing
	default:
		if err := x.checkRetained(ctx); err != nil {
			return err
		}
	}
	_, err := x.nr.Transaction(ctx, "tx", int(n), func(ctx context.Context, tx pgx.Tx) (json.RawMessage, error) {
		if err := lockSubjects(ctx, tx, subjectLockKey(x.tenant, x.run.Subject, x.run.ID)); err != nil {
			return nil, err
		}
		var seq int64
		var sens []byte
		if err := tx.QueryRow(ctx, `SELECT tx_seq, sensitive FROM automation.runs WHERE tenant_id = $1 AND id = $2 FOR UPDATE`, x.tenant, x.run.ID).Scan(&seq, &sens); err != nil {
			return nil, serrors.FromDB(opRun, err)
		}
		if seq != n {
			return nil, fmt.Errorf("run %s: transaction %d expected, the run is at %d", x.run.ID, n, seq)
		}
		cause, _ := json.Marshal(map[string]any{"run_id": x.run.ID.String(), "depth": x.run.Depth})
		if _, err := tx.Exec(ctx, `SELECT set_config('app.automation_cause', $1, true)`, string(cause)); err != nil {
			return nil, serrors.FromDB(opRun, err)
		}
		s := &stepState{sensitive: decodeRefs(sens)}
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
		var sensUpdate any
		if s.sensitiveChanged {
			b, err := json.Marshal(s.sensitive)
			if err != nil {
				return nil, err
			}
			sensUpdate = b
		}
		if _, err := tx.Exec(ctx, `
			UPDATE automation.runs SET tx_seq = tx_seq + 1, inbox_seq = inbox_seq + $3, steps_count = steps_count + $4,
				output = coalesce($5, output), error = coalesce($6, error), sensitive = coalesce($7, sensitive), updated_at = now()
			WHERE tenant_id = $1 AND id = $2`, x.tenant, x.run.ID, s.inbox, s.steps, nullJSON(s.output), nullJSON(s.failure), sensUpdate); err != nil {
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
	// sensitive are the sensitive fields the run read up to now; steps record them as their provenance.
	sensitive        []sensitiveRef
	sensitiveChanged bool
}

func (s *stepState) sensitiveJSON() []byte {
	if len(s.sensitive) == 0 {
		return []byte(`[]`)
	}
	b, err := json.Marshal(s.sensitive)
	if err != nil {
		return []byte(`[]`)
	}
	return b
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
	// The waits endRun closed may have armed child instances: they are terminated now, as the run's
	// own completion policy cancels only occurrences and tasks.
	if err := x.release(ctx); err != nil {
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
		if w.Kind == waitEvent && !w.Spec.CaughtUp {
			// One bounded page per pass; the loop takes its steps between pages. The wait's timer does
			// not fire before its catch-up ended: a recovered event may have come before the deadline.
			if _, err := x.catchUp(ctx, w); err != nil {
				return false, err
			}
			// Signals ready in the inbox are applied before the next page, so a match closes the wait and
			// ends its scan; otherwise another pass takes the next page or, once caught up, arms the timer.
			ready, err := x.inboxReady(ctx)
			if err != nil || ready {
				return false, err
			}
			return true, nil
		}
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
		if w.Kind == waitEvent {
			// An event that happened before the deadline and waits in the inbox wins over the timer: the
			// loop applies the inbox first.
			early, err := x.earlyEventPending(ctx, w)
			if err != nil {
				return false, err
			}
			if early {
				continue
			}
		}
		d := time.Until(*w.TimeoutAt)
		if d < 0 {
			d = 0
		}
		err := x.nr.Sleep(ctx, "timer:"+w.WaitID, 0, d)
		switch {
		case err == nil:
			if h := x.r.testHook; h != nil {
				// Between the timer's receipt and the timeout's transaction (no transaction is open).
				if err := h(ctx, nil, "timer-fired"); err != nil {
					return false, err
				}
			}
			applied, err := x.timeout(ctx, w)
			if err != nil {
				return false, err
			}
			if !applied {
				continue // the inbox holds an event in time: the loop applies it first
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
	// OccurredAt is `now` of the wait's filter, at the router and in the run alike.
	OccurredAt time.Time `json:"occurred_at"`
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
// timeout fires a wait's timer; applied is false when an event that happened by the deadline was routed
// to the wait meanwhile (checked again under the subject's lock, which the router holds while it
// signals): the inbox applies that event first.
func (x *execution) timeout(ctx context.Context, w waitRow) (bool, error) {
	applied := true
	err := x.tx(ctx, "timeout", func(ctx context.Context, tx pgx.Tx, s *stepState) error {
		ws, err := queryWaits(ctx, tx, `SELECT `+waitCols+` FROM automation.waits WHERE tenant_id = $1 AND run_id = $2 AND wait_id = $3 FOR UPDATE`, x.tenant, x.run.ID, w.WaitID)
		if err != nil || len(ws) == 0 || ws[0].Status != waitOpen {
			return err
		}
		w = ws[0]
		if w.Kind == waitEvent && w.TimeoutAt != nil {
			early, err := earlyEventIn(ctx, tx, x.tenant, x.nr.Instance().ID, w)
			if err != nil {
				return err
			}
			if early {
				applied = false
				return nil
			}
		}
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
	return applied && err == nil, err
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
	out := map[string]any{"decision": nonNull(res.Decision)}
	if res.By != nil {
		out["by"] = *res.By
	}
	b, err := json.Marshal(out)
	if err != nil {
		return WaitResult{Error: &StepError{Code: graph.FailInvalidAnswer, Message: "the decision is not JSON"}}
	}
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

// maxCatchUp is the batch size of a wait's catch-up query.
const maxCatchUp = 200

// inboxReady reports an unconsumed inbox signal: a WaitFor on the inbox returns at once.
func (x *execution) inboxReady(ctx context.Context) (bool, error) {
	var ready bool
	err := x.r.cfg.DB.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM extruntime.workflow_signals
		WHERE tenant_id = $1 AND instance_id = $2 AND name = $3 AND consumed_name IS NULL)`,
		x.tenant, x.nr.Instance().ID, InboxSignal).Scan(&ready)
	return ready, serrors.FromDB(opRun, err)
}

// earlyEventPending reports an unconsumed inbox signal for the wait whose event happened by its deadline.
func (x *execution) earlyEventPending(ctx context.Context, w waitRow) (bool, error) {
	return earlyEventIn(ctx, x.r.cfg.DB, x.tenant, x.nr.Instance().ID, w)
}

// earlyEventIn is earlyEventPending in q (a transaction or the pool).
func earlyEventIn(ctx context.Context, q querier, tenant, instance uuid.UUID, w waitRow) (bool, error) {
	var early bool
	err := q.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM extruntime.workflow_signals
			WHERE tenant_id = $1 AND instance_id = $2 AND name = $3 AND consumed_name IS NULL
			  AND payload->>'wait_id' = $4 AND (payload->'event'->>'occurred_at')::timestamptz <= $5)`,
		tenant, instance, InboxSignal, w.WaitID, *w.TimeoutAt).Scan(&early)
	return early, serrors.FromDB(opRun, err)
}

// catchUp signals a new event wait with the matching events the router could not route to it: an event
// with a seq after the wait's watermark that was delivered while the wait's transaction was still open
// (the router reads committed waits only). Its range is fixed when it begins: up to the outbox watermark
// of then (catch_up_until), as every later event finds the committed wait through the router. One call
// signals one bounded page in a fenced transaction that records how far it got (catch_up_seq); done is
// true once the range is covered, when caught_up is set. The signal key is the router's, so an event both
// paths see reaches the wait once.
func (x *execution) catchUp(ctx context.Context, w waitRow) (bool, error) {
	if w.EventKind == nil {
		return true, nil
	}
	var entity, key any
	if w.Subject != nil {
		entity, key = w.Subject.Entity, w.Subject.Key
	}
	until := w.Spec.CatchUpUntil
	if until == 0 {
		mark, err := events.Watermark(ctx, x.r.cfg.DB)
		if err != nil {
			return false, err
		}
		until = mark
	}
	after := w.AfterSeq
	if w.Spec.CatchUpSeq > after {
		after = w.Spec.CatchUpSeq
	}
	type found struct {
		id       uuid.UUID
		seq      int64
		payload  []byte
		occurred time.Time
	}
	rows, err := x.r.cfg.DB.Query(ctx, `
		SELECT id, seq, payload, occurred_at FROM extruntime.outbox_events
		WHERE tenant_id = $1 AND event = $2 AND seq > $3 AND seq <= $4
		  AND ($5::text IS NULL OR (subject_entity = $5 AND subject_key = $6))
		ORDER BY seq LIMIT $7`, x.tenant, *w.EventKind, after, until, entity, key, maxCatchUp)
	if err != nil {
		return false, serrors.FromDB(opRun, err)
	}
	page := make([]found, 0, maxCatchUp)
	for rows.Next() {
		var f found
		if err := rows.Scan(&f.id, &f.seq, &f.payload, &f.occurred); err != nil {
			rows.Close()
			return false, serrors.FromDB(opRun, err)
		}
		page = append(page, f)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return false, serrors.FromDB(opRun, err)
	}
	done := len(page) < maxCatchUp
	if len(page) > 0 {
		after = page[len(page)-1].seq
		done = done || after >= until
	}
	err = x.tx(ctx, "catchup", func(ctx context.Context, tx pgx.Tx, _ *stepState) error {
		for _, f := range page {
			payload, err := decodeMap(f.payload)
			if err != nil {
				continue
			}
			if payload, err = x.r.projector(x.tenant).project(ctx, *w.EventKind, payload, &x.subject); err != nil {
				return &host.InfraError{Err: err}
			}
			raw, err := json.Marshal(payload)
			if err != nil {
				return err
			}
			msg, err := json.Marshal(inboxMessage{WaitResult: WaitResult{WaitID: w.WaitID},
				Event: &inboxEvent{ID: f.id.String(), Seq: f.seq, Kind: *w.EventKind, Payload: raw, OccurredAt: f.occurred}})
			if err != nil {
				return err
			}
			if err := x.r.cfg.Workflows.SignalInTx(ctx, tx, spi.Scope{Subject: x.subject}, host.SignalIntent{
				Workflow: WorkflowID, Instance: x.nr.Instance().ID.String(), Name: InboxSignal, Payload: msg,
				Key: "event:" + f.id.String() + ":" + w.WaitID, Producer: spi.Producer{EventID: f.id.String()},
			}); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, `
			UPDATE automation.waits SET spec = spec || jsonb_build_object('catch_up_seq', $4::bigint, 'catch_up_until', $5::bigint, 'caught_up', $6::boolean)
			WHERE tenant_id = $1 AND run_id = $2 AND wait_id = $3`, x.tenant, x.run.ID, w.WaitID, after, until, done)
		return serrors.FromDB(opRun, err)
	})
	return done, err
}
