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
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
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
	// events are the events a trigger's collect merged into the run (event is the last); nil: one.
	events      []graph.Event
	subject     SubjectRef
	runAs       *spi.Subject // nil: no instance (blocked, filter_failed)
	initiator   *int64
	depth       int
	cause       *uuid.UUID
	restarted   *uuid.UUID
	predecessor *uuid.UUID
	// startState and reason, when set, record a run that does not start.
	startState string
	reason     string
	// restartOf is the version of the run a Restart starts from: the new run may take it or the enabled
	// version. Other starts need the graph enabled on version, with the trigger of that enable.
	restartOf int
	// eventSeq is the outbox seq of the starting event: it must follow the trigger's watermark as it is
	// now (a Disable and Enable of the same version between the selection and the start move it).
	eventSeq int64
}

// eligible checks, under the graph's row lock (the lock disableTx holds while it marks the live runs),
// that the graph may still start the run: not deleted, and enabled on the version (a restart: on it or
// on the run's own). A start that waited for a Disable sees it here and starts nothing.
func eligible(ctx context.Context, tx pgx.Tx, req startRequest) error {
	var enabled *int
	var deleted bool
	err := tx.QueryRow(ctx, `SELECT enabled_version, deleted_at IS NOT NULL FROM automation.automations WHERE tenant_id = $1 AND id = $2 FOR SHARE`,
		req.tenant, req.automationID).Scan(&enabled, &deleted)
	switch {
	case errors.Is(err, pgx.ErrNoRows) || (err == nil && deleted):
		return ErrNotFound
	case err != nil:
		return serrors.FromDB(opStart, err)
	case req.restartOf > 0 && req.version == req.restartOf:
		return nil
	case enabled == nil || *enabled != req.version:
		return fmt.Errorf("%w: version %d of %s is not enabled", ErrNotEnabled, req.version, req.automationID)
	case req.restartOf > 0:
		return nil
	}
	// The trigger as the current enable wrote it (Enable rebuilds the triggers under the same lock).
	var after int64
	err = tx.QueryRow(ctx, `SELECT after_seq FROM automation.triggers WHERE tenant_id = $1 AND automation_id = $2 AND trigger_id = $3 AND version = $4`,
		req.tenant, req.automationID, req.triggerID, req.version).Scan(&after)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return fmt.Errorf("%w: %s has no trigger %s now", ErrNotEnabled, req.automationID, req.triggerID)
	case err != nil:
		return serrors.FromDB(opStart, err)
	case req.eventSeq > 0 && req.eventSeq <= after:
		return fmt.Errorf("%w: the event precedes the trigger's current enable", ErrNotEnabled)
	}
	return nil
}

// notEligible reports a start the graph's state refused (deleted, disabled, another version enabled):
// an event or a schedule slot then starts nothing.
func notEligible(err error) bool { return errors.Is(err, ErrNotEnabled) || errors.Is(err, ErrNotFound) }

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
	if err := eligible(ctx, tx, req); err != nil {
		return RunRef{}, err
	}
	subject := normSubject(req.subject)
	if req.event == nil {
		req.event = graph.Event{}
	}
	input, err := json.Marshal(req.event)
	if err != nil {
		return RunRef{}, serrors.Wrap(opStart, err)
	}
	var evs any
	if len(req.events) > 0 {
		b, err := json.Marshal(req.events)
		if err != nil {
			return RunRef{}, serrors.Wrap(opStart, err)
		}
		evs = b
	}
	// The sensitive fields of every event the run reads: its event and the ones a collect merged.
	sensitive := []byte(`[]`)
	var all []sensitiveRef
	for _, e := range append([]graph.Event{req.event}, req.events...) {
		kind := e.Kind()
		if kind == "" {
			continue
		}
		refs, err := r.projector(req.tenant).sensitiveIn(ctx, kind, map[string]any(e))
		if err != nil {
			return RunRef{}, &host.InfraError{Err: err}
		}
		all, _ = mergeRefs(all, refs)
	}
	if len(all) > 0 {
		if sensitive, err = json.Marshal(all); err != nil {
			return RunRef{}, err
		}
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
			depth, cause_run_id, restarted_from, sensitive, events)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
		ON CONFLICT (tenant_id, start_key) DO NOTHING
		RETURNING id`,
		req.tenant, id, req.automationID, req.version, key, state, reason, req.triggerID, eventID, req.kind, input,
		subject.Entity, subject.Key, runAsUser, runAsKind, req.initiator, req.depth, req.cause, req.restarted, sensitive, evs).Scan(&got)
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
		// A run that does not start (blocked, filter_failed) replaces nothing: its predecessor keeps its
		// conversation.
		return RunRef{RunID: id, Status: StatusOf("", req.startState)}, nil
	}
	if req.restarted != nil {
		// The predecessor keeps the mark while it lives, whatever happens to this run's row: the
		// replacement takes the conversation over or queues for it.
		if _, err := tx.Exec(ctx, `UPDATE automation.runs SET superseded = true WHERE tenant_id = $1 AND id = $2`, req.tenant, *req.restarted); err != nil {
			return RunRef{}, serrors.FromDB(opStart, err)
		}
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
			WHERE r.tenant_id = $1 AND r.subject_entity = $2 AND r.subject_key = $3 AND r.id <> $4 AND `+liveSQL+` AND NOT `+supersededSQL+`)`,
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
			  AND r.instance_id IS NOT NULL AND i.status IN ('running', 'waiting', 'blocked') AND NOT `+supersededSQL+`)`,
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

// queueScanBatch is how many free subjects one startQueued pass looks at.
const queueScanBatch = 200

// startQueued starts the queued runs whose subject is free (the observer's attempt can lose the race
// with a start that queued a run while the subject's run was finishing).
func (r *Runtime) startQueued(ctx context.Context) error {
	// Only subjects without a live holder (dequeue's busy check) count toward the batch: busy
	// conversations keep their queued runs, and a batch of them would hide a free subject on every pass.
	// The oldest queued run first. dequeue checks again under the subject's lock.
	rows, err := r.cfg.DB.Query(ctx, `
		SELECT q.tenant_id, q.subject_entity, q.subject_key FROM automation.subject_queue q
		WHERE NOT EXISTS (SELECT 1`+runFrom+`
			WHERE r.tenant_id = q.tenant_id AND r.subject_entity = q.subject_entity AND r.subject_key = q.subject_key
			  AND r.instance_id IS NOT NULL AND i.status IN ('running', 'waiting', 'blocked') AND NOT `+supersededSQL+`)
		GROUP BY q.tenant_id, q.subject_entity, q.subject_key
		ORDER BY min(q.seq) LIMIT $1`, queueScanBatch)
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
	// One subject that cannot start its head run does not hold back the others.
	var errs []error
	for _, x := range list {
		err := pgx.BeginFunc(ctx, r.cfg.DB, func(tx pgx.Tx) error {
			if err := lockSubjects(ctx, tx, subjectLockKey(x.tenant, x.s, uuid.Nil)); err != nil {
				return err
			}
			return r.dequeue(ctx, tx, x.tenant, x.s, uuid.Nil)
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("subject %s/%s of %s: %w", x.s.Entity, x.s.Key, x.tenant, err))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// wakeAwaiting signals the live runs of a conversation that wait for it (acquireSubject), except one
// that is finishing in the caller's transaction. A run whose run-as is refused blocks on its own.
func (r *Runtime) wakeAwaiting(ctx context.Context, tx pgx.Tx, tenant uuid.UUID, s SubjectRef, except uuid.UUID) error {
	rows, err := tx.Query(ctx, `SELECT r.instance_id, r.run_as_user_id, r.run_as_kind, r.tx_seq`+runFrom+`
		WHERE r.tenant_id = $1 AND r.subject_entity = $2 AND r.subject_key = $3 AND r.id <> $4 AND r.awaiting_subject
		  AND r.run_as_user_id IS NOT NULL AND i.status IN ('running', 'waiting')`, tenant, s.Entity, s.Key, except)
	if err != nil {
		return serrors.FromDB(opStart, err)
	}
	type waiter struct {
		inst uuid.UUID
		user int64
		kind string
		seq  int64
	}
	var list []waiter
	for rows.Next() {
		var w waiter
		if err := rows.Scan(&w.inst, &w.user, &w.kind, &w.seq); err != nil {
			rows.Close()
			return serrors.FromDB(opStart, err)
		}
		list = append(list, w)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return serrors.FromDB(opStart, err)
	}
	for _, w := range list {
		as, err := r.cfg.Users.LoadSubject(ctx, tenant, w.user, spi.SubjectKind(w.kind))
		if errors.Is(err, host.ErrSubjectBlocked) {
			continue
		}
		if err != nil {
			return &host.InfraError{Err: err}
		}
		as.TenantID, as.UserID, as.Kind = tenant, w.user, spi.SubjectKind(w.kind)
		sp, err := tx.Begin(ctx)
		if err != nil {
			return serrors.FromDB(opStart, err)
		}
		err = r.cfg.Workflows.SignalInTx(ctx, sp, spi.Scope{Subject: as}, host.SignalIntent{
			Workflow: WorkflowID, Instance: w.inst.String(), Name: SubjectSignal, Payload: json.RawMessage(`{}`),
			Key: fmt.Sprintf("subject:%s:%d", except, w.seq),
		})
		if err != nil {
			_ = sp.Rollback(ctx)
			if _, ok := pool.AsWireError(err); ok {
				continue // finished or refused meanwhile
			}
			return err
		}
		if err := sp.Commit(ctx); err != nil {
			return serrors.FromDB(opStart, err)
		}
	}
	return nil
}

// wakeFreeAwaiting signals waiting runs whose conversation is free: the observer's signal can be lost
// to a run that started waiting after the holder finished.
func (r *Runtime) wakeFreeAwaiting(ctx context.Context) error {
	rows, err := r.cfg.DB.Query(ctx, `SELECT r.tenant_id, r.instance_id, r.tx_seq`+runFrom+`
		WHERE r.awaiting_subject AND i.status = 'waiting' AND NOT EXISTS (SELECT 1 FROM automation.runs o
			JOIN extruntime.workflow_instances oi ON oi.id = o.instance_id
			WHERE o.tenant_id = r.tenant_id AND o.subject_entity = r.subject_entity AND o.subject_key = r.subject_key AND o.id <> r.id
			  AND oi.status IN ('running', 'waiting', 'blocked') AND NOT o.awaiting_subject
			  AND NOT (oi.status = 'blocked' AND o.superseded))
		LIMIT 200`)
	if err != nil {
		return serrors.FromDB(opStart, err)
	}
	type waiter struct {
		tenant, inst uuid.UUID
		seq          int64
	}
	var list []waiter
	for rows.Next() {
		var w waiter
		if err := rows.Scan(&w.tenant, &w.inst, &w.seq); err != nil {
			rows.Close()
			return serrors.FromDB(opStart, err)
		}
		list = append(list, w)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return serrors.FromDB(opStart, err)
	}
	// One instance that refuses the signal does not hold back the recovery of the others.
	var errs []error
	for _, w := range list {
		err := r.cfg.Workflows.Signal(ctx, w.tenant, 0, w.inst, SubjectSignal, json.RawMessage(`{}`), fmt.Sprintf("subject-free:%d", w.seq))
		if err != nil && !errors.Is(err, workflow.ErrNotAccepting) && !errors.Is(err, workflow.ErrNotFound) {
			errs = append(errs, fmt.Errorf("instance %s: %w", w.inst, err))
		}
	}
	return errors.Join(errs...)
}
