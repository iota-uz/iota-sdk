package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const opStart = serrors.Op("automation.engine.start")

// startRequest is a run to start.
type startRequest struct {
	tenant       uuid.UUID
	automationID uuid.UUID
	version      int
	in           *graph.Interpreter
	triggerID    string
	kind         string
	eventID      string
	key          string
	event        graph.Event
	subject      SubjectRef
	runAs        *spi.Subject // nil: no instance (blocked, filter_failed)
	initiator    *int64
	depth        int
	cause        *uuid.UUID
	restarted    *uuid.UUID
	predecessor  *uuid.UUID
	// startState and reason, when set, record a run that does not start.
	startState string
	reason     string
}

// normSubject is the subject a run is about: none when the event has none.
func normSubject(s SubjectRef) SubjectRef {
	if s.Entity == "" {
		s.Entity = graph.SubjectNone
	}
	if s.Entity == graph.SubjectNone {
		s.Key = ""
	}
	return s
}

// subjectLockKey is the advisory lock key of a subject; a run about nothing is its own subject.
func subjectLockKey(tenant uuid.UUID, s SubjectRef, run uuid.UUID) string {
	s = normSubject(s)
	if s.Entity == graph.SubjectNone {
		return "automation|" + tenant.String() + "|run|" + run.String()
	}
	return "automation|" + tenant.String() + "|" + s.Entity + "|" + s.Key
}

// lockSubjects takes the advisory locks of keys in a stable order, so two transactions that lock several
// subjects never wait on each other in a cycle.
func lockSubjects(ctx context.Context, tx pgx.Tx, keys ...string) error {
	sort.Strings(keys)
	prev := ""
	for _, k := range keys {
		if k == prev {
			continue
		}
		prev = k
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, k); err != nil {
			return serrors.FromDB(opStart, err)
		}
	}
	return nil
}

// startRun records a run and, unless it is a run that does not start or a queued conversation run, starts
// its instance. A repeated start key returns the run it created before.
func (r *Runtime) startRun(ctx context.Context, tx pgx.Tx, req startRequest) (RunRef, error) {
	subject := normSubject(req.subject)
	if req.event == nil {
		req.event = graph.Event{}
	}
	input, err := json.Marshal(req.event)
	if err != nil {
		return RunRef{}, serrors.Wrap(opStart, err)
	}
	id := uuid.New()
	var runAsUser *int64
	var runAsKind *string
	if req.runAs != nil {
		u, k := req.runAs.UserID, string(req.runAs.Kind)
		if k == "" {
			k = string(spi.SubjectInitiator)
		}
		runAsUser, runAsKind = &u, &k
	}
	var key, state, reason, eventID any
	if req.key != "" {
		key = req.key
	}
	if req.startState != "" {
		state = req.startState
	}
	if req.reason != "" {
		reason = req.reason
	}
	if req.eventID != "" {
		eventID = req.eventID
	}
	var got uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO automation.runs (tenant_id, id, automation_id, version, start_key, start_state, start_reason, trigger_id,
			trigger_event_id, trigger_kind, input, subject_entity, subject_key, run_as_user_id, run_as_kind, initiator_user_id,
			depth, cause_run_id, restarted_from)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
		ON CONFLICT (tenant_id, start_key) DO NOTHING
		RETURNING id`,
		req.tenant, id, req.automationID, req.version, key, state, reason, req.triggerID, eventID, req.kind, input,
		subject.Entity, subject.Key, runAsUser, runAsKind, req.initiator, req.depth, req.cause, req.restarted).Scan(&got)
	if errors.Is(err, pgx.ErrNoRows) {
		existing, err := r.runByKey(ctx, tx, req.tenant, req.key)
		if err != nil {
			return RunRef{}, err
		}
		return RunRef{RunID: existing.ID, Status: existing.status()}, nil
	}
	if err != nil {
		return RunRef{}, serrors.FromDB(opStart, err)
	}
	if req.startState != "" {
		return RunRef{RunID: id, Status: StatusOf("", req.startState)}, nil
	}
	// The main token enters the node the trigger's fired edge leads to.
	first, ok := req.in.Next(req.triggerID, graph.TriggerOutcome)
	if !ok {
		first = graph.Ignore
	}
	if err := insertToken(ctx, tx, req.tenant, id, tokenRow{ID: uuid.New(), Path: "", NodeID: first, From: req.triggerID, Status: tokRunning}); err != nil {
		return RunRef{}, err
	}
	if subject.Entity == graph.SubjectConversation {
		if err := lockSubjects(ctx, tx, subjectLockKey(req.tenant, subject, id)); err != nil {
			return RunRef{}, err
		}
		busy, err := subjectBusy(ctx, tx, req.tenant, subject, id)
		if err != nil {
			return RunRef{}, err
		}
		if busy {
			if _, err := tx.Exec(ctx, `UPDATE automation.runs SET start_state = 'queued' WHERE tenant_id = $1 AND id = $2`, req.tenant, id); err != nil {
				return RunRef{}, serrors.FromDB(opStart, err)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO automation.subject_queue (tenant_id, subject_entity, subject_key, run_id) VALUES ($1, $2, $3, $4)`,
				req.tenant, subject.Entity, subject.Key, id); err != nil {
				return RunRef{}, serrors.FromDB(opStart, err)
			}
			return RunRef{RunID: id, Status: RunQueued}, nil
		}
	}
	if err := r.startInstance(ctx, tx, req.tenant, id, *req.runAs, req.predecessor); err != nil {
		return RunRef{}, err
	}
	r.metrics.runStarted(ctx, req.kind)
	return RunRef{RunID: id, Status: RunRunning}, nil
}

// startInstance starts the automation.run instance of a recorded run.
func (r *Runtime) startInstance(ctx context.Context, tx pgx.Tx, tenant, run uuid.UUID, runAs spi.Subject, predecessor *uuid.UUID) error {
	runAs.TenantID = tenant
	in := spi.StartIntent{
		Workflow: WorkflowID,
		Input:    json.RawMessage(fmt.Sprintf(`{"run_id":%q}`, run.String())),
		Key:      "automation.run:" + run.String(),
	}
	if predecessor != nil {
		in.Predecessor = predecessor.String()
	}
	inst, err := r.cfg.Workflows.Start(ctx, tx, spi.Scope{Subject: runAs}, in)
	if err != nil {
		return serrors.Wrap(opStart, err)
	}
	if _, err := tx.Exec(ctx, `UPDATE automation.runs SET instance_id = $3::uuid, start_state = NULL, start_reason = NULL, updated_at = now() WHERE tenant_id = $1 AND id = $2`,
		tenant, run, inst); err != nil {
		return serrors.FromDB(opStart, err)
	}
	return nil
}

// subjectBusy reports whether another run of the subject is active.
func subjectBusy(ctx context.Context, q querier, tenant uuid.UUID, s SubjectRef, except uuid.UUID) (bool, error) {
	var busy bool
	err := q.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1`+runFrom+`
			WHERE r.tenant_id = $1 AND r.subject_entity = $2 AND r.subject_key = $3 AND r.id <> $4 AND `+liveSQL+`)`,
		tenant, s.Entity, s.Key, except).Scan(&busy)
	return busy, serrors.FromDB(opStart, err)
}

func (r *Runtime) runByKey(ctx context.Context, q querier, tenant uuid.UUID, key string) (runRow, error) {
	row, err := scanRun(q.QueryRow(ctx, `SELECT `+runCols+runFrom+` WHERE r.tenant_id = $1 AND r.start_key = $2`, tenant, key))
	return row, serrors.FromDB(opStart, err)
}

// dequeue starts the oldest queued run of a conversation subject when the subject has no active run.
// The caller holds the subject's lock. except is a run that is finishing in the caller's transaction.
func (r *Runtime) dequeue(ctx context.Context, tx pgx.Tx, tenant uuid.UUID, s SubjectRef, except uuid.UUID) error {
	var busy bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1`+runFrom+`
			WHERE r.tenant_id = $1 AND r.subject_entity = $2 AND r.subject_key = $3 AND r.id <> $4
			  AND r.instance_id IS NOT NULL AND i.status IN ('running', 'waiting', 'blocked'))`,
		tenant, s.Entity, s.Key, except).Scan(&busy)
	if err != nil || busy {
		return serrors.FromDB(opStart, err)
	}
	var run, automation uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT q.run_id, r.automation_id FROM automation.subject_queue q JOIN automation.runs r ON r.tenant_id = q.tenant_id AND r.id = q.run_id
		WHERE q.tenant_id = $1 AND q.subject_entity = $2 AND q.subject_key = $3 ORDER BY q.seq LIMIT 1`, tenant, s.Entity, s.Key).Scan(&run, &automation)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return serrors.FromDB(opStart, err)
	}
	// The graph's row lock serializes the promotion with Disable and Delete (disableTx): after them the
	// queue row is gone and the run cancelled.
	var live bool
	err = tx.QueryRow(ctx, `SELECT deleted_at IS NULL FROM automation.automations WHERE tenant_id = $1 AND id = $2 FOR SHARE`, tenant, automation).Scan(&live)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil || !live {
		return serrors.FromDB(opStart, err)
	}
	var user *int64
	var kind *string
	err = tx.QueryRow(ctx, `
		DELETE FROM automation.subject_queue q USING automation.runs r
		WHERE q.tenant_id = $1 AND q.run_id = $2 AND r.tenant_id = q.tenant_id AND r.id = q.run_id
		  AND r.start_state = 'queued' AND NOT r.cancel_requested
		RETURNING r.run_as_user_id, r.run_as_kind`, tenant, run).Scan(&user, &kind)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return serrors.FromDB(opStart, err)
	}
	if user == nil || kind == nil {
		_, err := tx.Exec(ctx, `UPDATE automation.runs SET start_state = 'blocked', start_reason = 'no_owner' WHERE tenant_id = $1 AND id = $2`, tenant, run)
		return serrors.FromDB(opStart, err)
	}
	return r.startInstance(ctx, tx, tenant, run, spi.Subject{TenantID: tenant, UserID: *user, Kind: spi.SubjectKind(*kind)}, nil)
}

// startQueued starts the queued runs whose subject is free (the observer's attempt can lose the race
// with a start that queued a run while the subject's run was finishing).
func (r *Runtime) startQueued(ctx context.Context) error {
	rows, err := r.cfg.DB.Query(ctx, `SELECT DISTINCT tenant_id, subject_entity, subject_key FROM automation.subject_queue LIMIT 200`)
	if err != nil {
		return serrors.FromDB(opStart, err)
	}
	type subj struct {
		tenant uuid.UUID
		s      SubjectRef
	}
	var list []subj
	for rows.Next() {
		var x subj
		if err := rows.Scan(&x.tenant, &x.s.Entity, &x.s.Key); err != nil {
			rows.Close()
			return serrors.FromDB(opStart, err)
		}
		list = append(list, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return serrors.FromDB(opStart, err)
	}
	for _, x := range list {
		err := pgx.BeginFunc(ctx, r.cfg.DB, func(tx pgx.Tx) error {
			if err := lockSubjects(ctx, tx, subjectLockKey(x.tenant, x.s, uuid.Nil)); err != nil {
				return err
			}
			return r.dequeue(ctx, tx, x.tenant, x.s, uuid.Nil)
		})
		if err != nil {
			return err
		}
	}
	return nil
}
