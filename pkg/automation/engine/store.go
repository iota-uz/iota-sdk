package engine

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const opStore = serrors.Op("automation.engine.store")

// querier is a pool or a transaction.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// statusSQL is RunStatus of a run row r joined with its instance i.
const statusSQL = `(CASE i.status
	WHEN 'running' THEN 'running' WHEN 'waiting' THEN 'waiting' WHEN 'blocked' THEN 'blocked'
	WHEN 'completed' THEN 'done' WHEN 'failed' THEN 'failed' WHEN 'terminated' THEN 'cancelled'
	ELSE CASE r.start_state WHEN 'queued' THEN 'queued' WHEN 'filter_failed' THEN 'filter_failed'
		WHEN 'blocked' THEN 'blocked' WHEN 'cancelled' THEN 'cancelled' ELSE 'running' END END)`

// supersededSQL is true for a run row r whose blocked instance a Restart replaced: it keeps its instance
// (StartV2 leaves it as it is) but no longer holds its conversation subject, so the replacement starts.
// Should the predecessor be resumed, its steps still take the subject's lock: steps never interleave.
const supersededSQL = `(i.status = 'blocked' AND EXISTS (SELECT 1 FROM automation.runs s WHERE s.tenant_id = r.tenant_id AND s.restarted_from = r.id))`

// liveSQL is true for a run row r (with instance i) that is still active: queued or a live instance.
const liveSQL = `(r.start_state = 'queued' OR i.status IN ('running', 'waiting', 'blocked'))`

// ---------------------------------------------------------------------------------------------
// automations and versions

type automationRow struct {
	Automation
	DeletedAt *time.Time
}

const automationCols = `id, name, latest_version, enabled_version, owner_user_id, layout, created_by, created_origin, created_at, updated_at, deleted_at`

func scanAutomation(row pgx.Row) (automationRow, error) {
	var a automationRow
	var layout []byte
	var origin string
	err := row.Scan(&a.ID, &a.Name, &a.LatestVersion, &a.EnabledVersion, &a.OwnerUserID, &layout, &a.CreatedBy, &origin, &a.CreatedAt, &a.UpdatedAt, &a.DeletedAt)
	if err != nil {
		return a, err
	}
	a.CreatedOrigin = Origin(origin)
	a.Layout = graph.Layout{}
	if len(layout) > 0 {
		if err := json.Unmarshal(layout, &a.Layout); err != nil {
			return a, err
		}
	}
	return a, nil
}

// loadAutomation reads a graph that is not deleted; forUpdate locks its row.
func loadAutomation(ctx context.Context, q querier, tenant, id uuid.UUID, forUpdate bool) (automationRow, error) {
	sql := `SELECT ` + automationCols + ` FROM automation.automations WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL`
	if forUpdate {
		sql += ` FOR UPDATE`
	}
	a, err := scanAutomation(q.QueryRow(ctx, sql, tenant, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return a, ErrNotFound
	}
	return a, serrors.FromDB(opStore, err)
}

type versionRow struct {
	Version
	RunAs string
}

const versionCols = `automation_id, version, graph, catalog_revision, code_digest, run_as, saved_by, saved_origin, note, created_at`

func scanVersion(row pgx.Row) (versionRow, error) {
	var v versionRow
	var raw []byte
	var origin string
	err := row.Scan(&v.AutomationID, &v.Version.Version, &raw, &v.CatalogRevision, &v.CodeDigest, &v.RunAs, &v.SavedBy, &origin, &v.Note, &v.CreatedAt)
	v.Graph = json.RawMessage(raw)
	v.SavedOrigin = Origin(origin)
	return v, err
}

func loadVersion(ctx context.Context, q querier, tenant, id uuid.UUID, version int) (versionRow, error) {
	v, err := scanVersion(q.QueryRow(ctx, `SELECT `+versionCols+` FROM automation.versions WHERE tenant_id = $1 AND automation_id = $2 AND version = $3`, tenant, id, version))
	if errors.Is(err, pgx.ErrNoRows) {
		return v, ErrNotFound
	}
	return v, serrors.FromDB(opStore, err)
}

// ---------------------------------------------------------------------------------------------
// runs

type runRow struct {
	TenantID       uuid.UUID
	ID             uuid.UUID
	AutomationID   uuid.UUID
	Version        int
	InstanceID     *uuid.UUID
	StartKey       *string
	StartState     *string
	StartReason    *string
	TriggerID      string
	TriggerEventID *string
	TriggerKind    string
	Input          json.RawMessage
	Subject        SubjectRef
	RunAsUserID    *int64
	RunAsKind      *string
	InitiatorID    *int64
	InboxSeq       int64
	TxSeq          int64
	StepsCount     int
	Depth          int
	CauseRunID     *uuid.UUID
	RestartedFrom  *uuid.UUID
	Output         json.RawMessage
	Error          json.RawMessage
	Sensitive      []sensitiveRef
	CreatedAt      time.Time
	UpdatedAt      time.Time
	// joined from the instance
	InstanceStatus  *string
	BlockedReason   *string
	BlockedDetail   *string
	InstanceUpdated *time.Time
}

const runCols = `r.tenant_id, r.id, r.automation_id, r.version, r.instance_id, r.start_key, r.start_state, r.start_reason,
	r.trigger_id, r.trigger_event_id, r.trigger_kind, r.input, r.subject_entity, r.subject_key, r.run_as_user_id, r.run_as_kind,
	r.initiator_user_id, r.inbox_seq, r.tx_seq, r.steps_count, r.depth, r.cause_run_id, r.restarted_from, r.output, r.error,
	r.sensitive, r.created_at, r.updated_at, i.status, i.blocked_reason, i.blocked_detail, i.updated_at`

const runFrom = ` FROM automation.runs r LEFT JOIN extruntime.workflow_instances i ON i.id = r.instance_id`

func scanRun(row pgx.Row) (runRow, error) {
	var r runRow
	var input, output, errj, sens []byte
	err := row.Scan(&r.TenantID, &r.ID, &r.AutomationID, &r.Version, &r.InstanceID, &r.StartKey, &r.StartState, &r.StartReason,
		&r.TriggerID, &r.TriggerEventID, &r.TriggerKind, &input, &r.Subject.Entity, &r.Subject.Key, &r.RunAsUserID, &r.RunAsKind,
		&r.InitiatorID, &r.InboxSeq, &r.TxSeq, &r.StepsCount, &r.Depth, &r.CauseRunID, &r.RestartedFrom, &output, &errj,
		&sens, &r.CreatedAt, &r.UpdatedAt, &r.InstanceStatus, &r.BlockedReason, &r.BlockedDetail, &r.InstanceUpdated)
	r.Input, r.Output, r.Error, r.Sensitive = input, output, errj, decodeRefs(sens)
	return r, err
}

func (r runRow) status() RunStatus {
	inst, start := "", ""
	if r.InstanceStatus != nil {
		inst = *r.InstanceStatus
	}
	if r.StartState != nil {
		start = *r.StartState
	}
	return StatusOf(inst, start)
}

func (r runRow) reason() string {
	if r.StartReason != nil && *r.StartReason != "" {
		return *r.StartReason
	}
	if r.BlockedReason != nil && *r.BlockedReason != "" {
		if r.BlockedDetail != nil && *r.BlockedDetail != "" {
			return *r.BlockedReason + ": " + *r.BlockedDetail
		}
		return *r.BlockedReason
	}
	return ""
}

func (r runRow) stepError() *StepError {
	if len(r.Error) == 0 {
		return nil
	}
	var e StepError
	if json.Unmarshal(r.Error, &e) != nil {
		return &StepError{Code: "failed", Message: string(r.Error)}
	}
	return &e
}

func loadRun(ctx context.Context, q querier, tenant, id uuid.UUID) (runRow, error) {
	r, err := scanRun(q.QueryRow(ctx, `SELECT `+runCols+runFrom+` WHERE r.tenant_id = $1 AND r.id = $2`, tenant, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrNotFound
	}
	return r, serrors.FromDB(opStore, err)
}

// ---------------------------------------------------------------------------------------------
// tokens

// nodeWrite is a node output a token set.
type nodeWrite struct {
	Node  string          `json:"node"`
	State graph.NodeState `json:"state"`
}

// flowState is what a forking token waits for: the branches of a parallel or the items of a foreach.
type flowState struct {
	Kind string `json:"kind"` // parallel | foreach
	Node string `json:"node"`
	// parallel
	Join     string          `json:"join,omitempty"`
	State    graph.JoinState `json:"state"`
	Children []uuid.UUID     `json:"children,omitempty"` // the branch tokens, in fork order
	// foreach: the items are in the token's items column, the results on the item tokens (arrival)
	Count       int    `json:"count,omitempty"`
	Target      string `json:"target,omitempty"`
	Next        int    `json:"next,omitempty"`
	Left        int    `json:"left,omitempty"`
	Concurrency int    `json:"concurrency,omitempty"`
}

// arrival is how a branch or an item token ended.
type arrival struct {
	OK    bool   `json:"ok"`
	Value any    `json:"value"`
	Error string `json:"error,omitempty"`
}

type tokenRow struct {
	ID           uuid.UUID
	Parent       *uuid.UUID
	Path         string
	NodeID       string
	From         string
	Status       string
	StepSeq      int
	Branch       *string
	ForeachRef   *string
	ForeachIndex *int
	Item         json.RawMessage
	Writes       []nodeWrite
	Flow         *flowState
	CreatedSeq   int64
}

// Token states.
const (
	tokRunning   = "running"
	tokWaiting   = "waiting"
	tokJoined    = "joined"
	tokDone      = "done"
	tokCancelled = "cancelled"
	tokFailed    = "failed"
)

func tokenTerminal(status string) bool {
	return status == tokJoined || status == tokDone || status == tokCancelled || status == tokFailed
}

const tokenCols = `id, parent_token_id, path, node_id, from_node, status, step_seq, branch, foreach_ref, foreach_index, item, writes, flow, created_seq`

func scanToken(row pgx.Row) (tokenRow, error) {
	var t tokenRow
	var item, writes, flow []byte
	if err := row.Scan(&t.ID, &t.Parent, &t.Path, &t.NodeID, &t.From, &t.Status, &t.StepSeq, &t.Branch, &t.ForeachRef, &t.ForeachIndex, &item, &writes, &flow, &t.CreatedSeq); err != nil {
		return t, err
	}
	t.Item = item
	if len(writes) > 0 {
		if err := expr.UnmarshalJSON(writes, &t.Writes); err != nil {
			return t, err
		}
	}
	if len(flow) > 0 && string(flow) != "null" {
		t.Flow = &flowState{}
		if err := expr.UnmarshalJSON(flow, t.Flow); err != nil {
			return t, err
		}
	}
	return t, nil
}

func loadToken(ctx context.Context, q querier, tenant, run, id uuid.UUID, forUpdate bool) (tokenRow, error) {
	sql := `SELECT ` + tokenCols + ` FROM automation.tokens WHERE tenant_id = $1 AND run_id = $2 AND id = $3`
	if forUpdate {
		sql += ` FOR UPDATE`
	}
	t, err := scanToken(q.QueryRow(ctx, sql, tenant, run, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return t, ErrNotFound
	}
	return t, serrors.FromDB(opStore, err)
}

func queryTokens(ctx context.Context, q querier, sql string, args ...any) ([]tokenRow, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, serrors.FromDB(opStore, err)
	}
	defer rows.Close()
	var out []tokenRow
	for rows.Next() {
		t, err := scanToken(rows)
		if err != nil {
			return nil, serrors.FromDB(opStore, err)
		}
		out = append(out, t)
	}
	return out, serrors.FromDB(opStore, rows.Err())
}

// ancestors are the tokens that started t, nearest first.
func ancestors(ctx context.Context, q querier, tenant, run uuid.UUID, t tokenRow) ([]tokenRow, error) {
	if t.Parent == nil {
		return nil, nil
	}
	return queryTokens(ctx, q, `
		WITH RECURSIVE chain AS (
			SELECT `+tokenCols+`, 1 AS depth FROM automation.tokens WHERE tenant_id = $1 AND run_id = $2 AND id = $3
			UNION ALL
			SELECT t.id, t.parent_token_id, t.path, t.node_id, t.from_node, t.status, t.step_seq, t.branch, t.foreach_ref,
				t.foreach_index, t.item, t.writes, t.flow, t.created_seq, c.depth + 1
			FROM automation.tokens t JOIN chain c ON t.id = c.parent_token_id
			WHERE t.tenant_id = $1 AND t.run_id = $2)
		SELECT `+tokenCols+` FROM chain ORDER BY depth`, tenant, run, *t.Parent)
}

func saveToken(ctx context.Context, q querier, tenant, run uuid.UUID, t tokenRow) error {
	writes, err := json.Marshal(t.Writes)
	if err != nil {
		return err
	}
	var flow any
	if t.Flow != nil {
		b, err := json.Marshal(t.Flow)
		if err != nil {
			return err
		}
		flow = b
	}
	_, err = q.Exec(ctx, `
		UPDATE automation.tokens SET node_id = $4, from_node = $5, status = $6, step_seq = $7, writes = $8, flow = $9, updated_at = now()
		WHERE tenant_id = $1 AND run_id = $2 AND id = $3`, tenant, run, t.ID, t.NodeID, t.From, t.Status, t.StepSeq, writes, flow)
	return serrors.FromDB(opStore, err)
}

func insertToken(ctx context.Context, q querier, tenant, run uuid.UUID, t tokenRow) error {
	writes, err := json.Marshal(t.Writes)
	if err != nil {
		return err
	}
	if t.Writes == nil {
		writes = []byte(`[]`)
	}
	var item any
	if len(t.Item) > 0 {
		item = []byte(t.Item)
	}
	_, err = q.Exec(ctx, `
		INSERT INTO automation.tokens (tenant_id, run_id, id, parent_token_id, path, node_id, from_node, status, branch, foreach_ref, foreach_index, item, writes, created_seq)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13,
			(SELECT coalesce(max(created_seq), 0) + 1 FROM automation.tokens WHERE tenant_id = $1 AND run_id = $2))`,
		tenant, run, t.ID, t.Parent, t.Path, t.NodeID, t.From, t.Status, t.Branch, t.ForeachRef, t.ForeachIndex, item, writes)
	return serrors.FromDB(opStore, err)
}

// set writes a node's output on the token.
func (t *tokenRow) set(node string, st graph.NodeState) {
	for i := range t.Writes {
		if t.Writes[i].Node == node {
			t.Writes[i].State = st
			return
		}
	}
	t.Writes = append(t.Writes, nodeWrite{Node: node, State: st})
}

// ---------------------------------------------------------------------------------------------
// waits

type waitRow struct {
	RunID     uuid.UUID
	TokenID   uuid.UUID
	WaitID    string
	Kind      string
	NodeID    string
	Subject   *SubjectRef
	EventKind *string
	Filter    *string
	TimeoutAt *time.Time
	Status    string
	AfterSeq  int64
	Release   bool
	Spec      waitSpecJSON
}

// waitSpecJSON is what the core keeps about a wait besides its columns.
type waitSpecJSON struct {
	Seq   int        `json:"seq"`   // the token's step the wait belongs to
	Timer bool       `json:"timer"` // a timer occurrence timer:<wait_id> was armed
	Exec  *WaitSpec  `json:"exec,omitempty"`
	Child *uuid.UUID `json:"child,omitempty"`
}

// Wait kinds and states.
const (
	waitEvent = "event"
	waitTimer = "timer"
	waitJoin  = "join"
	waitTask  = "task"
	waitCall  = "call"

	waitOpen      = "open"
	waitFired     = "fired"
	waitTimedOut  = "timed_out"
	waitCancelled = "cancelled"
)

// child is the child instance that answers the wait, if any.
func (w waitRow) child() *uuid.UUID {
	if w.Spec.Child != nil {
		return w.Spec.Child
	}
	if w.Spec.Exec != nil {
		return w.Spec.Exec.Child
	}
	return nil
}

const waitCols = `run_id, token_id, wait_id, kind, node_id, subject_entity, subject_key, event_kind, filter_cel, timeout_at, status, after_seq, release_pending, spec`

func scanWait(row pgx.Row) (waitRow, error) {
	var w waitRow
	var entity, key *string
	var spec []byte
	if err := row.Scan(&w.RunID, &w.TokenID, &w.WaitID, &w.Kind, &w.NodeID, &entity, &key, &w.EventKind, &w.Filter, &w.TimeoutAt, &w.Status, &w.AfterSeq, &w.Release, &spec); err != nil {
		return w, err
	}
	if entity != nil {
		w.Subject = &SubjectRef{Entity: *entity}
		if key != nil {
			w.Subject.Key = *key
		}
	}
	if len(spec) > 0 {
		if err := json.Unmarshal(spec, &w.Spec); err != nil { //nolint:musttag // workflow.TaskSpec keeps Go's field names
			return w, err
		}
	}
	return w, nil
}

func queryWaits(ctx context.Context, q querier, sql string, args ...any) ([]waitRow, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, serrors.FromDB(opStore, err)
	}
	defer rows.Close()
	var out []waitRow
	for rows.Next() {
		w, err := scanWait(rows)
		if err != nil {
			return nil, serrors.FromDB(opStore, err)
		}
		out = append(out, w)
	}
	return out, serrors.FromDB(opStore, rows.Err())
}

func insertWait(ctx context.Context, q querier, tenant uuid.UUID, w waitRow) error {
	spec, err := json.Marshal(w.Spec) //nolint:musttag // workflow.TaskSpec keeps Go's field names
	if err != nil {
		return err
	}
	var entity, key any
	if w.Subject != nil {
		entity, key = w.Subject.Entity, w.Subject.Key
	}
	_, err = q.Exec(ctx, `
		INSERT INTO automation.waits (tenant_id, run_id, token_id, wait_id, kind, node_id, subject_entity, subject_key, event_kind, filter_cel, timeout_at, status, after_seq, spec)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'open', $12, $13)`,
		tenant, w.RunID, w.TokenID, w.WaitID, w.Kind, w.NodeID, entity, key, w.EventKind, w.Filter, w.TimeoutAt, w.AfterSeq, spec)
	return serrors.FromDB(opStore, err)
}

// closeWait moves an open wait to status; release marks what it armed (a timer, a task, a child) for
// cancelling after the commit.
func closeWait(ctx context.Context, q querier, tenant uuid.UUID, w waitRow, status string) error {
	timer := w.Spec.Timer
	other := w.child() != nil || (w.Spec.Exec != nil && w.Spec.Exec.Task != nil)
	var release bool
	switch status {
	case waitCancelled:
		release = timer || other
	case waitFired: // the answer came: only its timeout is left
		release = timer
	case waitTimedOut: // the timer fired: the task or the call is left
		release = other
	}
	_, err := q.Exec(ctx, `
		UPDATE automation.waits SET status = $4, release_pending = release_pending OR $5, updated_at = now()
		WHERE tenant_id = $1 AND run_id = $2 AND wait_id = $3 AND status = 'open'`, tenant, w.RunID, w.WaitID, status, release)
	return serrors.FromDB(opStore, err)
}
