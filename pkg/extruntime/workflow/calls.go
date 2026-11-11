package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const opCall = serrors.Op("extruntime.workflow.CallWorkflow")

func internalErr(code, format string, args ...any) *pool.WireError {
	return &pool.WireError{Kind: pool.ErrKindInternal, Code: code, Message: fmt.Sprintf(format, args...)}
}

func blockedWire(b *blockInfo) *pool.WireError {
	return &pool.WireError{Kind: pool.ErrKindInternal, Code: codeBlocked, Message: b.Error()}
}

// nondeterminism blocks the run: the code now does something different at an
// occurrence than what the journal recorded.
func (rs *runState) nondeterminism(name string, n int, want string, have Occurrence) *pool.WireError {
	return blockedWire(rs.block(BlockNondeterminism, fmt.Sprintf(
		"occurrence %s#%d is recorded as a %s (%s) but the workflow code now calls %s there", name, n, have.Kind, have.Status, want)))
}

func (rs *runState) hashMismatch(name string, n int, have Occurrence, got string) *pool.WireError {
	return blockedWire(rs.block(BlockNondeterminism, fmt.Sprintf(
		"occurrence %s#%d was recorded with input %s but the workflow code now passes input %s", name, n, have.InputHash, got)))
}

var durationPattern = regexp.MustCompile(`^(\d+(?:\.\d+)?)(ms|s|m|h|d)$`)

// parseDuration parses the author-facing Duration (`1d`, `30m`, `500ms`).
func parseDuration(s string) (time.Duration, error) {
	m := durationPattern.FindStringSubmatch(s)
	if m == nil {
		return 0, fmt.Errorf("duration %q must be a number followed by ms, s, m, h or d", s)
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, err
	}
	unit := map[string]time.Duration{"ms": time.Millisecond, "s": time.Second, "m": time.Minute, "h": time.Hour, "d": 24 * time.Hour}[m[2]]
	return time.Duration(v * float64(unit)), nil
}

func suspendError(reason string, wakeAt *time.Time) *pool.WireError {
	info := map[string]any{"reason": reason}
	if wakeAt != nil {
		info["wakeAt"] = wakeAt.UTC().Format(time.RFC3339Nano)
	}
	return &pool.WireError{Kind: pool.ErrKindSuspend, Code: "workflow.suspended", Suspend: marshal(info)}
}

// terminalReply returns what a primitive returns for a recorded outcome.
func terminalReply(o Occurrence) (json.RawMessage, error) {
	if o.Status == OccFailed {
		var we pool.WireError
		if err := json.Unmarshal(o.Error, &we); err != nil || we.Kind == "" {
			return nil, internalErr("workflow.corruptJournal", "occurrence %s#%d has an unreadable recorded error", o.Name, o.N)
		}
		return nil, &we
	}
	return nonNull(o.Result), nil
}

// replayWrap is the reply of *.begin calls for a recorded outcome.
func replayWrap(o Occurrence) (json.RawMessage, error) {
	if o.Status == OccFailed {
		return marshal(map[string]any{"replay": true, "ok": false, "error": json.RawMessage(o.Error)}), nil
	}
	return marshal(map[string]any{"replay": true, "ok": true, "value": json.RawMessage(nonNull(o.Result))}), nil
}

// CallWorkflow serves the wf.* host calls of a workflow run. Every call
// carries the occurrence name and its counter n, assigned in Node at call
// time. Completed occurrences are normally answered inside Node from the
// journal the invoke carried; the cases below also serve them so the
// protocol stays correct without that optimisation.
func (e *Engine) CallWorkflow(ctx context.Context, call host.WorkflowCall, method string, args json.RawMessage) (json.RawMessage, error) {
	rs := e.runByToken(call.Token())
	if rs == nil {
		return nil, internalErr("workflow.noRun", "%s: no active workflow run owns this invocation", method)
	}
	if we := rs.stale(); we != nil {
		return nil, we
	}
	switch method {
	case "wf.transaction.begin":
		return e.wfTxBegin(ctx, call, rs, args)
	case "wf.transaction.end":
		return e.wfTxEnd(ctx, call, rs, args)
	case "wf.transaction.call":
		return e.wfTxCall(ctx, call, rs, args)
	case "wf.step.begin":
		return e.wfStepBegin(ctx, call, rs, args)
	case "wf.step.end":
		return e.wfStepEnd(ctx, rs, args)
	case "wf.sleep":
		return e.wfSleep(ctx, rs, args)
	case "wf.waitFor":
		return e.wfWaitFor(ctx, rs, args)
	case "wf.task":
		return e.wfTask(ctx, rs, args)
	case "wf.call":
		return e.wfCall(ctx, call, rs, args)
	case "wf.start":
		return e.wfStart(ctx, rs, args)
	case "wf.now":
		return e.wfNow(ctx, rs, args)
	case "wf.uuid":
		return e.wfUUID(ctx, rs, args)
	}
	return nil, internalErr(pool.CodeHostCall, "unknown workflow host call %q", method)
}

func decodeArgs(args json.RawMessage, into any) error {
	if err := json.Unmarshal(args, into); err != nil {
		return internalErr(pool.CodeHostCall, "malformed workflow host call arguments: %v", err)
	}
	return nil
}

// ---- wf.transaction ----

func (e *Engine) wfTxBegin(ctx context.Context, call host.WorkflowCall, rs *runState, args json.RawMessage) (json.RawMessage, error) {
	var a struct {
		Name string `json:"name"`
		N    int    `json:"n"`
	}
	if err := decodeArgs(args, &a); err != nil {
		return nil, err
	}
	if o, ok := rs.get(a.Name, a.N); ok {
		if o.Kind != KindTransaction || !o.terminal() {
			return nil, rs.nondeterminism(a.Name, a.N, KindTransaction, o)
		}
		return replayWrap(o)
	}
	h, err := call.OpenTx(ctx)
	if err != nil {
		return nil, err
	}
	return marshal(map[string]any{"replay": false, "tx": h}), nil
}

func (e *Engine) wfTxEnd(ctx context.Context, call host.WorkflowCall, rs *runState, args json.RawMessage) (json.RawMessage, error) {
	var a struct {
		Name    string          `json:"name"`
		N       int             `json:"n"`
		Tx      string          `json:"tx"`
		Outcome host.TxOutcome  `json:"outcome"`
		Value   json.RawMessage `json:"value"`
	}
	if err := decodeArgs(args, &a); err != nil {
		return nil, err
	}
	if a.Outcome != host.TxCommit {
		return json.RawMessage(`null`), call.EndTx(ctx, a.Tx, host.TxRollback)
	}
	o := Occurrence{Name: a.Name, N: a.N, Kind: KindTransaction, Status: OccCompleted, Result: nonNull(a.Value), Attempts: 1, InvocationID: call.InvocationID()}
	err := call.WithTx(ctx, a.Tx, func(ctx context.Context, tx pgx.Tx) error {
		if err := e.fence(ctx, tx, rs); err != nil {
			return err
		}
		return putOccurrence(ctx, tx, rs, o)
	})
	if err != nil {
		_ = call.EndTx(ctx, a.Tx, host.TxRollback)
		return nil, err
	}
	if err := call.EndTx(ctx, a.Tx, host.TxCommit); err != nil {
		return nil, err
	}
	rs.set(o)
	return json.RawMessage(`null`), nil
}

// wfTxCall is wf.transaction(name, Operation|AtomicCommand, input): the
// callable joins a Go transaction that also holds the occurrence receipt.
func (e *Engine) wfTxCall(ctx context.Context, call host.WorkflowCall, rs *runState, args json.RawMessage) (json.RawMessage, error) {
	var a struct {
		Name      string          `json:"name"`
		N         int             `json:"n"`
		Callable  string          `json:"callable"`
		Input     json.RawMessage `json:"input"`
		InputHash string          `json:"inputHash"`
	}
	if err := decodeArgs(args, &a); err != nil {
		return nil, err
	}
	if o, ok := rs.get(a.Name, a.N); ok {
		if o.Kind != KindTransaction || !o.terminal() {
			return nil, rs.nondeterminism(a.Name, a.N, KindTransaction, o)
		}
		if o.InputHash != "" && a.InputHash != "" && o.InputHash != a.InputHash {
			return nil, rs.hashMismatch(a.Name, a.N, o, a.InputHash)
		}
		return terminalReply(o)
	}
	h, err := call.OpenTx(ctx)
	if err != nil {
		return nil, err
	}
	out, err := call.CallInTx(ctx, h, a.Callable, a.Input)
	if err != nil {
		_ = call.EndTx(ctx, h, host.TxRollback)
		return nil, err
	}
	o := Occurrence{Name: a.Name, N: a.N, Kind: KindTransaction, Status: OccCompleted, InputHash: a.InputHash, Result: nonNull(out), Attempts: 1, InvocationID: call.InvocationID()}
	err = call.WithTx(ctx, h, func(ctx context.Context, tx pgx.Tx) error {
		if err := e.fence(ctx, tx, rs); err != nil {
			return err
		}
		return putOccurrence(ctx, tx, rs, o)
	})
	if err != nil {
		_ = call.EndTx(ctx, h, host.TxRollback)
		return nil, err
	}
	if err := call.EndTx(ctx, h, host.TxCommit); err != nil {
		return nil, err
	}
	rs.set(o)
	return nonNull(out), nil
}

// ---- wf.step ----

func idempotencyKey(rs *runState, name string, n int) string {
	return fmt.Sprintf("%s:%s:%d", rs.inst.ID, name, n)
}

func (e *Engine) wfStepBegin(ctx context.Context, call host.WorkflowCall, rs *runState, args json.RawMessage) (json.RawMessage, error) {
	var a struct {
		Name string `json:"name"`
		N    int    `json:"n"`
	}
	if err := decodeArgs(args, &a); err != nil {
		return nil, err
	}
	o, ok := rs.get(a.Name, a.N)
	if ok {
		if o.Kind != KindStep || (o.Status != OccStarted && !o.terminal()) {
			return nil, rs.nondeterminism(a.Name, a.N, KindStep, o)
		}
		if o.terminal() {
			return replayWrap(o)
		}
	}
	// First execution, or a step recorded as started whose effect may or may
	// not have run: it runs again with the same idempotency key (at-least-once).
	next := Occurrence{Name: a.Name, N: a.N, Kind: KindStep, Status: OccStarted, Attempts: o.Attempts + 1, InvocationID: call.InvocationID()}
	err := e.inFence(ctx, rs, func(tx pgx.Tx) (func(), error) {
		if err := putOccurrence(ctx, tx, rs, next); err != nil {
			return nil, err
		}
		return func() { rs.set(next) }, nil
	})
	if err != nil {
		return nil, err
	}
	return marshal(map[string]any{"replay": false, "idempotencyKey": idempotencyKey(rs, a.Name, a.N)}), nil
}

func (e *Engine) wfStepEnd(ctx context.Context, rs *runState, args json.RawMessage) (json.RawMessage, error) {
	var a struct {
		Name  string          `json:"name"`
		N     int             `json:"n"`
		OK    bool            `json:"ok"`
		Value json.RawMessage `json:"value"`
		Error json.RawMessage `json:"error"`
	}
	if err := decodeArgs(args, &a); err != nil {
		return nil, err
	}
	o, ok := rs.get(a.Name, a.N)
	if !ok || o.Kind != KindStep || o.Status != OccStarted {
		return nil, internalErr("workflow.stepNotStarted", "step %s#%d was not begun", a.Name, a.N)
	}
	if a.OK {
		o.Status, o.Result = OccCompleted, nonNull(a.Value)
	} else {
		o.Status, o.Error = OccFailed, nonNull(a.Error)
	}
	err := e.inFence(ctx, rs, func(tx pgx.Tx) (func(), error) {
		if err := putOccurrence(ctx, tx, rs, o); err != nil {
			return nil, err
		}
		return func() { rs.set(o) }, nil
	})
	if err != nil {
		return nil, err
	}
	return json.RawMessage(`null`), nil
}

// ---- wf.sleep ----

func (e *Engine) wfSleep(ctx context.Context, rs *runState, args json.RawMessage) (json.RawMessage, error) {
	var a struct {
		Name     string `json:"name"`
		N        int    `json:"n"`
		Duration string `json:"duration"`
	}
	if err := decodeArgs(args, &a); err != nil {
		return nil, err
	}
	d, err := parseDuration(a.Duration)
	if err != nil {
		return nil, &pool.WireError{Kind: pool.ErrKindValidation, Code: "workflow.invalidDuration", Message: err.Error()}
	}
	o, ok := rs.get(a.Name, a.N)
	if ok {
		if o.Kind != KindSleep || (o.Status != OccWaiting && !o.terminal()) {
			return nil, rs.nondeterminism(a.Name, a.N, KindSleep, o)
		}
		if o.terminal() {
			return json.RawMessage(`null`), nil
		}
	}
	var wake *time.Time
	done := false
	err = e.inFence(ctx, rs, func(tx pgx.Tx) (func(), error) {
		var at time.Time
		due := false
		if !ok {
			if err := tx.QueryRow(ctx, `SELECT clock_timestamp() + $1::interval`, d.String()).Scan(&at); err != nil {
				return nil, serrors.FromDB(opCall, err)
			}
			o = Occurrence{Name: a.Name, N: a.N, Kind: KindSleep, Status: OccWaiting, WakeAt: &at, Attempts: 1}
			due = d <= 0
		} else {
			at = *o.WakeAt
			if err := tx.QueryRow(ctx, `SELECT $1::timestamptz <= clock_timestamp()`, at).Scan(&due); err != nil {
				return nil, serrors.FromDB(opCall, err)
			}
		}
		if due {
			o.Status, o.Result, done = OccCompleted, json.RawMessage(`null`), true
		}
		wake = &at
		if err := putOccurrence(ctx, tx, rs, o); err != nil {
			return nil, err
		}
		return func() { rs.set(o) }, nil
	})
	if err != nil {
		return nil, err
	}
	if done {
		return json.RawMessage(`null`), nil
	}
	return nil, suspendError("sleep:"+a.Name, wake)
}

// ---- wf.waitFor ----

func (e *Engine) wfWaitFor(ctx context.Context, rs *runState, args json.RawMessage) (json.RawMessage, error) {
	var a struct {
		Name    string `json:"name"`
		N       int    `json:"n"`
		Signal  string `json:"signal"`
		Timeout string `json:"timeout"`
	}
	if err := decodeArgs(args, &a); err != nil {
		return nil, err
	}
	var timeout time.Duration
	if a.Timeout != "" {
		var err error
		if timeout, err = parseDuration(a.Timeout); err != nil {
			return nil, &pool.WireError{Kind: pool.ErrKindValidation, Code: "workflow.invalidDuration", Message: err.Error()}
		}
	}
	o, ok := rs.get(a.Name, a.N)
	if ok {
		if o.Kind != KindWaitFor || (o.Status != OccWaiting && !o.terminal()) {
			return nil, rs.nondeterminism(a.Name, a.N, KindWaitFor, o)
		}
		if o.terminal() {
			return terminalReply(o)
		}
	}
	var result json.RawMessage
	var wake *time.Time
	err := e.inFence(ctx, rs, func(tx pgx.Tx) (func(), error) {
		// Signal consumption and occurrence completion commit in one tx.
		var sid uuid.UUID
		var payload []byte
		err := tx.QueryRow(ctx, `
			SELECT id, payload FROM extruntime.workflow_signals
			WHERE instance_id = $1 AND tenant_id = $2 AND name = $3 AND consumed_name IS NULL
			ORDER BY created_at, id FOR UPDATE SKIP LOCKED LIMIT 1`, rs.inst.ID, rs.inst.TenantID, a.Signal).Scan(&sid, &payload)
		switch {
		case err == nil:
			if _, err := tx.Exec(ctx, `UPDATE extruntime.workflow_signals SET consumed_name = $2, consumed_n = $3, consumed_at = now() WHERE id = $1`, sid, a.Name, a.N); err != nil {
				return nil, serrors.FromDB(opCall, err)
			}
			result = marshal(map[string]any{"outcome": "received", "payload": json.RawMessage(payload)})
			if !ok {
				o = Occurrence{Name: a.Name, N: a.N, Kind: KindWaitFor, Attempts: 1, Meta: marshal(map[string]string{"signal": a.Signal})}
			}
			o.Status, o.Result = OccCompleted, result
		case errors.Is(err, pgx.ErrNoRows):
			if !ok {
				o = Occurrence{Name: a.Name, N: a.N, Kind: KindWaitFor, Status: OccWaiting, Attempts: 1, Meta: marshal(map[string]string{"signal": a.Signal})}
				if a.Timeout != "" {
					var at time.Time
					if err := tx.QueryRow(ctx, `SELECT clock_timestamp() + $1::interval`, timeout.String()).Scan(&at); err != nil {
						return nil, serrors.FromDB(opCall, err)
					}
					o.WakeAt = &at
				}
			} else if o.WakeAt != nil {
				var due bool
				if err := tx.QueryRow(ctx, `SELECT $1::timestamptz <= clock_timestamp()`, *o.WakeAt).Scan(&due); err != nil {
					return nil, serrors.FromDB(opCall, err)
				}
				if due {
					result = marshal(map[string]string{"outcome": "timeout"})
					o.Status, o.Result = OccCompleted, result
				}
			}
			wake = o.WakeAt
		default:
			return nil, serrors.FromDB(opCall, err)
		}
		if err := putOccurrence(ctx, tx, rs, o); err != nil {
			return nil, err
		}
		return func() { rs.set(o) }, nil
	})
	if err != nil {
		return nil, err
	}
	if result != nil {
		return result, nil
	}
	return nil, suspendError("waitFor:"+a.Name, wake)
}

// ---- wf.task ----

func (e *Engine) wfTask(ctx context.Context, rs *runState, args json.RawMessage) (json.RawMessage, error) {
	var a struct {
		Name     string          `json:"name"`
		N        int             `json:"n"`
		Subject  spi.ObjectRef   `json:"subject"`
		Assignee spi.ObjectRef   `json:"assignee"`
		Decision json.RawMessage `json:"decision"`
		Timeout  string          `json:"timeout"`
	}
	if err := decodeArgs(args, &a); err != nil {
		return nil, err
	}
	assignee, err := strconv.ParseInt(a.Assignee.Key, 10, 64)
	if err != nil {
		return nil, &pool.WireError{Kind: pool.ErrKindValidation, Code: "workflow.invalidAssignee", Message: fmt.Sprintf("task assignee %q is not a user id", a.Assignee.Key)}
	}
	var timeout time.Duration
	if a.Timeout != "" {
		if timeout, err = parseDuration(a.Timeout); err != nil {
			return nil, &pool.WireError{Kind: pool.ErrKindValidation, Code: "workflow.invalidDuration", Message: err.Error()}
		}
	}
	o, ok := rs.get(a.Name, a.N)
	if ok {
		if o.Kind != KindTask || (o.Status != OccWaiting && !o.terminal()) {
			return nil, rs.nondeterminism(a.Name, a.N, KindTask, o)
		}
		if o.terminal() {
			return terminalReply(o)
		}
	}
	var result json.RawMessage
	var wake *time.Time
	err = e.inFence(ctx, rs, func(tx pgx.Tx) (func(), error) {
		var (
			taskID    uuid.UUID
			status    string
			decision  []byte
			decidedBy *int64
			due       *time.Time
		)
		err := tx.QueryRow(ctx, `
			SELECT id, status, decision, decided_by, due_at FROM extruntime.human_tasks
			WHERE instance_id = $1 AND tenant_id = $2 AND name = $3 AND n = $4 FOR UPDATE`,
			rs.inst.ID, rs.inst.TenantID, a.Name, a.N).Scan(&taskID, &status, &decision, &decidedBy, &due)
		if errors.Is(err, pgx.ErrNoRows) {
			var at *time.Time
			if a.Timeout != "" {
				var t time.Time
				if err := tx.QueryRow(ctx, `SELECT clock_timestamp() + $1::interval`, timeout.String()).Scan(&t); err != nil {
					return nil, serrors.FromDB(opCall, err)
				}
				at = &t
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO extruntime.human_tasks (id, tenant_id, instance_id, name, n, subject_entity, subject_key, assignee_user_id, decision_schema, due_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
				uuid.New(), rs.inst.TenantID, rs.inst.ID, a.Name, a.N, a.Subject.Entity, a.Subject.Key, assignee, jsonArg(a.Decision), at); err != nil {
				return nil, serrors.FromDB(opCall, err)
			}
			o = Occurrence{Name: a.Name, N: a.N, Kind: KindTask, Status: OccWaiting, Attempts: 1, WakeAt: at}
			wake = at
		} else if err != nil {
			return nil, serrors.FromDB(opCall, err)
		} else {
			if status == "open" && due != nil {
				var past bool
				if err := tx.QueryRow(ctx, `SELECT $1::timestamptz <= clock_timestamp()`, *due).Scan(&past); err != nil {
					return nil, serrors.FromDB(opCall, err)
				}
				if past {
					// Decision vs timeout is a compare-and-set on the task status.
					tag, err := tx.Exec(ctx, `UPDATE extruntime.human_tasks SET status = 'expired' WHERE id = $1 AND status = 'open'`, taskID)
					if err != nil {
						return nil, serrors.FromDB(opCall, err)
					}
					if tag.RowsAffected() == 1 {
						status = "expired"
					}
				}
			}
			switch status {
			case "decided":
				by := "0"
				if decidedBy != nil {
					by = strconv.FormatInt(*decidedBy, 10)
				}
				result = marshal(map[string]any{"outcome": "decided", "decision": json.RawMessage(decision), "by": by})
			case "expired":
				result = marshal(map[string]string{"outcome": "timeout"})
			case "open":
				wake = due
			default:
				return nil, rs.nondeterminism(a.Name, a.N, KindTask, Occurrence{Name: a.Name, N: a.N, Kind: KindTask, Status: "task " + status})
			}
			if !ok {
				o = Occurrence{Name: a.Name, N: a.N, Kind: KindTask, Attempts: 1, WakeAt: due}
			}
			if result != nil {
				o.Status, o.Result = OccCompleted, result
			} else {
				o.Status = OccWaiting
			}
		}
		if err := putOccurrence(ctx, tx, rs, o); err != nil {
			return nil, err
		}
		return func() { rs.set(o) }, nil
	})
	if err != nil {
		return nil, err
	}
	if result != nil {
		return result, nil
	}
	return nil, suspendError("task:"+a.Name, wake)
}

// ---- wf.now / wf.uuid ----

func (e *Engine) wfNow(ctx context.Context, rs *runState, args json.RawMessage) (json.RawMessage, error) {
	return e.journaled(ctx, rs, args, KindNow, func(ctx context.Context, tx pgx.Tx) (json.RawMessage, error) {
		var now time.Time
		if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
			return nil, serrors.FromDB(opCall, err)
		}
		return marshal(now.UTC().Format(time.RFC3339Nano)), nil
	})
}

func (e *Engine) wfUUID(ctx context.Context, rs *runState, args json.RawMessage) (json.RawMessage, error) {
	return e.journaled(ctx, rs, args, KindUUID, func(context.Context, pgx.Tx) (json.RawMessage, error) {
		return marshal(uuid.NewString()), nil
	})
}

// journaled records a value produced once per occurrence.
func (e *Engine) journaled(ctx context.Context, rs *runState, args json.RawMessage, kind string, produce func(context.Context, pgx.Tx) (json.RawMessage, error)) (json.RawMessage, error) {
	var a struct {
		Name string `json:"name"`
		N    int    `json:"n"`
	}
	if err := decodeArgs(args, &a); err != nil {
		return nil, err
	}
	if o, ok := rs.get(a.Name, a.N); ok {
		if o.Kind != kind || !o.terminal() {
			return nil, rs.nondeterminism(a.Name, a.N, kind, o)
		}
		return terminalReply(o)
	}
	var out json.RawMessage
	err := e.inFence(ctx, rs, func(tx pgx.Tx) (func(), error) {
		v, err := produce(ctx, tx)
		if err != nil {
			return nil, err
		}
		out = v
		o := Occurrence{Name: a.Name, N: a.N, Kind: kind, Status: OccCompleted, Result: v, Attempts: 1}
		if err := putOccurrence(ctx, tx, rs, o); err != nil {
			return nil, err
		}
		return func() { rs.set(o) }, nil
	})
	return out, err
}

// ---- wf.start ----

func (e *Engine) wfStart(ctx context.Context, rs *runState, args json.RawMessage) (json.RawMessage, error) {
	var a struct {
		Name      string          `json:"name"`
		N         int             `json:"n"`
		Workflow  string          `json:"workflow"`
		Input     json.RawMessage `json:"input"`
		InputHash string          `json:"inputHash"`
	}
	if err := decodeArgs(args, &a); err != nil {
		return nil, err
	}
	if o, ok := rs.get(a.Name, a.N); ok {
		if o.Kind != KindStart || !o.terminal() {
			return nil, rs.nondeterminism(a.Name, a.N, KindStart, o)
		}
		if o.InputHash != "" && a.InputHash != "" && o.InputHash != a.InputHash {
			return nil, rs.hashMismatch(a.Name, a.N, o, a.InputHash)
		}
		return terminalReply(o)
	}
	impl, err := e.prepareStart(ctx, a.Workflow, a.Input)
	if err != nil {
		var we *pool.WireError
		if errors.As(err, &we) && we.Kind == pool.ErrKindInternal && (we.Code == "workflow.noImplementation" || we.Code == "workflow.unknownWorkflow") {
			return nil, blockedWire(rs.block(BlockUnavailable, we.Message))
		}
		return nil, err
	}
	var out json.RawMessage
	err = e.inFence(ctx, rs, func(tx pgx.Tx) (func(), error) {
		parent := rs.inst.ID
		id, _, err := e.insertInstance(ctx, tx, startSpec{
			workflow: a.Workflow, input: a.Input, impl: impl,
			subject:    spi.Subject{TenantID: rs.inst.TenantID, UserID: rs.inst.SubjectUserID, Kind: rs.inst.SubjectKind},
			key:        idempotencyKey(rs, a.Name, a.N),
			parent:     &parent,
			invocation: "",
		})
		if err != nil {
			return nil, err
		}
		out = marshal(map[string]string{"instance": id.String()})
		o := Occurrence{Name: a.Name, N: a.N, Kind: KindStart, Status: OccCompleted, InputHash: a.InputHash, Result: out, Attempts: 1}
		if err := putOccurrence(ctx, tx, rs, o); err != nil {
			return nil, err
		}
		return func() { rs.set(o) }, nil
	})
	return out, err
}

// ---- wf.call ----

func (e *Engine) wfCall(ctx context.Context, call host.WorkflowCall, rs *runState, args json.RawMessage) (json.RawMessage, error) {
	var a struct {
		Name      string          `json:"name"`
		N         int             `json:"n"`
		Command   string          `json:"command"`
		Input     json.RawMessage `json:"input"`
		InputHash string          `json:"inputHash"`
	}
	if err := decodeArgs(args, &a); err != nil {
		return nil, err
	}
	o, ok := rs.get(a.Name, a.N)
	if ok {
		if o.Kind != KindCall || (o.Status != OccStarted && !o.terminal()) {
			return nil, rs.nondeterminism(a.Name, a.N, KindCall, o)
		}
		if o.InputHash != "" && a.InputHash != "" && o.InputHash != a.InputHash {
			return nil, rs.hashMismatch(a.Name, a.N, o, a.InputHash)
		}
		if o.terminal() {
			return terminalReply(o)
		}
	}
	rc, found, err := e.cfg.Catalog.Resolve(ctx, a.Command)
	if err != nil {
		return nil, err
	}
	if !found || rc.Resource.Kind != manifest.ResCommand {
		return nil, blockedWire(rs.block(BlockUnavailable, fmt.Sprintf("command %s is not available in the catalog", a.Command)))
	}
	// The newest binding is selected for a NEW call; a recorded one never moves.
	if _, err := e.cfg.Bindings.Resolve(ctx, a.Command); err != nil {
		if ctx.Err() != nil {
			return nil, err
		}
		return nil, blockedWire(rs.block(BlockUnavailable, fmt.Sprintf("command %s has no usable active binding: %v", a.Command, err)))
	}
	hash := rc.Resource.ContractHash
	if hash == "" {
		hash = manifest.ContractHashOf(rc.Resource)
	}
	if ok {
		// Recorded as started: the outcome of the earlier dispatch decides.
		out, handled, err := e.resolveStarted(ctx, rs, o, a.Command)
		if handled {
			return out, err
		}
	} else if rc.Resource.Atomic {
		return e.callAtomic(ctx, call, rs, a.Name, a.N, a.Command, a.Input, a.InputHash, hash)
	}
	return e.callOrdinary(ctx, call, rs, a.Name, a.N, a.Command, a.Input, a.InputHash, hash, o, ok)
}

func isInfrastructure(ctx context.Context, err error) bool {
	return ctx.Err() != nil || errors.Is(err, pool.ErrWorkerLost) || errors.Is(err, pool.ErrPoolClosed) || errors.Is(err, pool.ErrQueueFull)
}

func callMeta(res host.CallResult) json.RawMessage {
	return marshal(map[string]any{
		"binding":            map[string]string{"id": res.Binding.ID, "package": res.Binding.Package, "version": res.Binding.Version, "export": res.Binding.Export},
		"committedTxCount":   res.CommittedTxCount,
		"partiallyCommitted": res.PartiallyCommitted,
	})
}

func (e *Engine) record(ctx context.Context, rs *runState, o Occurrence) error {
	return e.inFence(ctx, rs, func(tx pgx.Tx) (func(), error) {
		if err := putOccurrence(ctx, tx, rs, o); err != nil {
			return nil, err
		}
		return func() { rs.set(o) }, nil
	})
}

func (e *Engine) forget(ctx context.Context, rs *runState, name string, n int) error {
	return e.inFence(ctx, rs, func(tx pgx.Tx) (func(), error) {
		if _, err := tx.Exec(ctx, `DELETE FROM extruntime.workflow_occurrences WHERE instance_id = $1 AND tenant_id = $2 AND name = $3 AND n = $4 AND status = 'started'`,
			rs.inst.ID, rs.inst.TenantID, name, n); err != nil {
			return nil, serrors.FromDB(opCall, err)
		}
		return func() { rs.del(name, n) }, nil
	})
}

// resolveStarted decides what a `started` call occurrence means after a crash
// or a lost reply, from the recorded outcome of its invocation. No invocation
// record means the command was never dispatched (handled=false: dispatch it).
// An unfinished or worker-lost invocation is ambiguous: the instance blocks
// and shows how many transactions the command committed.
func (e *Engine) resolveStarted(ctx context.Context, rs *runState, o Occurrence, command string) (json.RawMessage, bool, error) {
	if o.InvocationID == "" {
		return nil, false, nil
	}
	var (
		outcome   string
		committed int
		finished  bool
		partial   bool
		result    []byte
		errJSON   []byte
	)
	err := e.cfg.DB.QueryRow(ctx, `
		SELECT outcome, committed_tx_count, finished_at IS NOT NULL, partially_committed, result, error
		FROM extruntime.invocations WHERE id = $1::uuid AND tenant_id = $2`, o.InvocationID, rs.inst.TenantID).
		Scan(&outcome, &committed, &finished, &partial, &result, &errJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, true, serrors.FromDB(opCall, err)
	}
	meta := marshal(map[string]any{"committedTxCount": committed, "partiallyCommitted": partial, "recovered": true})
	switch {
	case finished && outcome == "ok":
		next := o
		next.Status, next.Result, next.Meta = OccCompleted, nonNull(result), meta
		if err := e.record(ctx, rs, next); err != nil {
			return nil, true, err
		}
		return nonNull(result), true, nil
	case finished && outcome == "error":
		next := o
		next.Status, next.Error, next.Meta = OccFailed, nonNull(errJSON), meta
		if err := e.record(ctx, rs, next); err != nil {
			return nil, true, err
		}
		out, rerr := terminalReply(next)
		return out, true, rerr
	}
	return nil, true, blockedWire(rs.block(BlockAmbiguous, fmt.Sprintf(
		"command %s (occurrence %s#%d, invocation %s) has no recorded outcome (%s) and committed %d transaction(s); resolve, retry or terminate it manually",
		command, o.Name, o.N, o.InvocationID, outcome, committed)))
}

func (e *Engine) callOrdinary(ctx context.Context, call host.WorkflowCall, rs *runState, name string, n int, command string, input json.RawMessage, inputHash, contractHash string, prev Occurrence, hadPrev bool) (json.RawMessage, error) {
	invID := uuid.NewString()
	attempts := 1
	if hadPrev {
		// Started earlier but never dispatched (no invocation record): reuse.
		if prev.InvocationID != "" {
			invID = prev.InvocationID
		}
		attempts = prev.Attempts + 1
	}
	started := Occurrence{Name: name, N: n, Kind: KindCall, Status: OccStarted, InputHash: inputHash, Attempts: attempts, InvocationID: invID, ContractHash: contractHash}
	if err := e.record(ctx, rs, started); err != nil {
		return nil, err
	}
	res, err := call.CallCommand(ctx, command, input, invID)
	if err == nil {
		done := started
		done.Status, done.Result, done.Meta = OccCompleted, nonNull(res.Output), callMeta(res)
		if res.InvocationID != "" {
			done.InvocationID = res.InvocationID
		}
		if err := e.record(ctx, rs, done); err != nil {
			return nil, err
		}
		return nonNull(res.Output), nil
	}
	if isInfrastructure(ctx, err) {
		// Outcome unknown: stays `started`; the next run resolves it from the
		// invocation record (committed_tx_count) instead of re-dispatching.
		return nil, err
	}
	we, isWire := pool.AsWireError(err)
	if res.InvocationID == "" {
		// Rejected before an invocation was recorded: nothing ran.
		if ferr := e.forget(ctx, rs, name, n); ferr != nil {
			return nil, ferr
		}
		switch {
		case isWire && (we.Kind == pool.ErrKindDomain || we.Kind == pool.ErrKindValidation):
			failed := started
			failed.Status, failed.Error, failed.Meta, failed.InvocationID = OccFailed, marshal(we), callMeta(res), ""
			if rerr := e.record(ctx, rs, failed); rerr != nil {
				return nil, rerr
			}
			return nil, err
		case isWire && we.Kind == pool.ErrKindDenied:
			return nil, blockedWire(rs.block(BlockDenied, fmt.Sprintf("command %s: %s", command, we.Error())))
		case errors.Is(err, host.ErrNoBinding):
			return nil, blockedWire(rs.block(BlockUnavailable, fmt.Sprintf("command %s has no active binding", command)))
		}
		return nil, err
	}
	if isWire && we.Kind == pool.ErrKindDenied && !res.PartiallyCommitted && res.CommittedTxCount == 0 {
		if ferr := e.forget(ctx, rs, name, n); ferr != nil {
			return nil, ferr
		}
		return nil, blockedWire(rs.block(BlockDenied, fmt.Sprintf("command %s: %s", command, we.Error())))
	}
	failed := started
	failed.Status, failed.Meta, failed.InvocationID = OccFailed, callMeta(res), res.InvocationID
	if isWire {
		failed.Error = marshal(we)
	} else {
		failed.Error = marshal(&pool.WireError{Kind: pool.ErrKindInternal, Code: pool.CodeHostCall, Message: err.Error()})
	}
	if rerr := e.record(ctx, rs, failed); rerr != nil {
		return nil, rerr
	}
	return nil, err
}

// callAtomic runs an atomic command joined to a Go transaction that also
// holds the occurrence receipt: both commit or neither does.
func (e *Engine) callAtomic(ctx context.Context, call host.WorkflowCall, rs *runState, name string, n int, command string, input json.RawMessage, inputHash, contractHash string) (json.RawMessage, error) {
	h, err := call.OpenTx(ctx)
	if err != nil {
		return nil, err
	}
	res, err := call.CallCommandInTx(ctx, h, command, input)
	if err != nil {
		_ = call.EndTx(ctx, h, host.TxRollback)
		if isInfrastructure(ctx, err) {
			return nil, err
		}
		we, isWire := pool.AsWireError(err)
		switch {
		case isWire && (we.Kind == pool.ErrKindDomain || we.Kind == pool.ErrKindValidation):
			failed := Occurrence{Name: name, N: n, Kind: KindCall, Status: OccFailed, InputHash: inputHash, Error: marshal(we), Attempts: 1, InvocationID: res.InvocationID, ContractHash: contractHash, Meta: callMeta(res)}
			if rerr := e.record(ctx, rs, failed); rerr != nil {
				return nil, rerr
			}
		case isWire && we.Kind == pool.ErrKindDenied:
			return nil, blockedWire(rs.block(BlockDenied, fmt.Sprintf("command %s: %s", command, we.Error())))
		}
		return nil, err
	}
	o := Occurrence{Name: name, N: n, Kind: KindCall, Status: OccCompleted, InputHash: inputHash, Result: nonNull(res.Output), Attempts: 1, InvocationID: res.InvocationID, ContractHash: contractHash, Meta: callMeta(res)}
	err = call.WithTx(ctx, h, func(ctx context.Context, tx pgx.Tx) error {
		if err := e.fence(ctx, tx, rs); err != nil {
			return err
		}
		return putOccurrence(ctx, tx, rs, o)
	})
	if err != nil {
		_ = call.EndTx(ctx, h, host.TxRollback)
		return nil, err
	}
	if err := call.EndTx(ctx, h, host.TxCommit); err != nil {
		return nil, err
	}
	rs.set(o)
	return nonNull(res.Output), nil
}
