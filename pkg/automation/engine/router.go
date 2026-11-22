package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const opRoute = serrors.Op("automation.engine.route")

// consumer is the engine's one outbox consumer: it fans each event out to the enabled graphs whose
// triggers match and to the open waits that listen for it, in the transaction of its receipt.
type consumer struct{ r *Runtime }

// Consumer is the outbox consumer of the engine (ID ConsumerID); register it with
// wiring.ContributeConsumers.
func (r *Runtime) Consumer() events.Consumer { return &consumer{r: r} }

func (c *consumer) ID() string { return ConsumerID }

// Events are the event keys of the triggers of enabled graphs and of open waits.
func (c *consumer) Events(ctx context.Context) ([]string, error) {
	rows, err := c.r.cfg.DB.Query(ctx, `
		SELECT DISTINCT t.event_kind FROM automation.triggers t
		JOIN automation.automations a ON a.tenant_id = t.tenant_id AND a.id = t.automation_id AND a.enabled_version = t.version AND a.deleted_at IS NULL
		WHERE t.event_kind IS NOT NULL AND t.kind IN ('event', 'inbound')
		UNION
		SELECT DISTINCT event_kind FROM automation.waits WHERE status = 'open' AND event_kind IS NOT NULL`)
	if err != nil {
		return nil, serrors.FromDB(opRoute, err)
	}
	keys, err := pgx.CollectRows(rows, pgx.RowTo[string])
	return keys, serrors.FromDB(opRoute, err)
}

// AfterSeq is the oldest watermark a trigger or an open wait still needs; with none, the current one.
func (c *consumer) AfterSeq(ctx context.Context) (int64, error) {
	var seq *int64
	err := c.r.cfg.DB.QueryRow(ctx, `
		SELECT min(s) FROM (
			SELECT min(t.after_seq) AS s FROM automation.triggers t
			JOIN automation.automations a ON a.tenant_id = t.tenant_id AND a.id = t.automation_id AND a.enabled_version = t.version AND a.deleted_at IS NULL
			WHERE t.kind IN ('event', 'inbound')
			UNION ALL
			SELECT min(after_seq) FROM automation.waits WHERE status = 'open' AND event_kind IS NOT NULL) x`).Scan(&seq)
	if err != nil {
		return 0, serrors.FromDB(opRoute, err)
	}
	if seq != nil {
		return *seq, nil
	}
	return events.Watermark(ctx, c.r.cfg.DB)
}

// fanTrigger is an enabled trigger an event may start.
type fanTrigger struct {
	automationID uuid.UUID
	version      int
	triggerID    string
	kind         string
	runAs        string
	graph        json.RawMessage
}

// fanWait is an open wait an event may wake.
type fanWait struct {
	runID        uuid.UUID
	waitID       string
	nodeID       string
	instance     uuid.UUID
	automationID uuid.UUID
	version      int
	subject      SubjectRef
	runAsUser    int64
	runAsKind    string
	graph        json.RawMessage
}

// cause is the run whose step published an event.
type cause struct {
	RunID uuid.UUID `json:"run_id"`
	Depth int       `json:"depth"`
}

func (c *consumer) Deliver(ctx context.Context, tx pgx.Tx, ev events.Envelope) error {
	r := c.r
	r.metrics.eventLag(ctx, ev.OccurredAt)
	var causeRaw []byte
	if err := tx.QueryRow(ctx, `SELECT cause FROM extruntime.outbox_events WHERE id = $1 AND tenant_id = $2`, ev.ID, ev.TenantID).Scan(&causeRaw); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return &host.InfraError{Err: serrors.FromDB(opRoute, err)}
	}
	var by *cause
	if len(causeRaw) > 0 {
		by = &cause{}
		if err := json.Unmarshal(causeRaw, by); err != nil {
			by = nil
		}
	}
	triggers, err := r.fanTriggers(ctx, tx, ev)
	if err != nil {
		return err
	}
	waits, err := r.fanWaits(ctx, tx, ev)
	if err != nil {
		return err
	}
	if len(triggers) == 0 && len(waits) == 0 {
		return nil
	}
	subject := SubjectRef{Entity: graph.SubjectNone}
	if ev.Subject != nil {
		subject = SubjectRef{Entity: ev.Subject.Entity, Key: ev.Subject.Key}
	}
	// Subject locks first (all of them, in a stable order), instances after: the order steps keep.
	keys := make([]string, 0, len(waits)+1)
	if normSubject(subject).Entity != graph.SubjectNone && len(triggers) > 0 {
		keys = append(keys, subjectLockKey(ev.TenantID, subject, uuid.Nil))
	}
	for _, w := range waits {
		keys = append(keys, subjectLockKey(ev.TenantID, w.subject, w.runID))
	}
	if err := lockSubjects(ctx, tx, keys...); err != nil {
		return &host.InfraError{Err: err}
	}
	proj := r.projector(ev.TenantID)
	for _, t := range triggers {
		if err := r.fanOut(ctx, tx, ev, subject, by, t, proj); err != nil {
			return err
		}
	}
	for _, w := range waits {
		if err := r.wake(ctx, tx, ev, w, proj); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runtime) fanTriggers(ctx context.Context, tx pgx.Tx, ev events.Envelope) ([]fanTrigger, error) {
	rows, err := tx.Query(ctx, `
		SELECT t.automation_id, t.version, t.trigger_id, t.kind, v.run_as, v.graph
		FROM automation.triggers t
		JOIN automation.automations a ON a.tenant_id = t.tenant_id AND a.id = t.automation_id AND a.enabled_version = t.version AND a.deleted_at IS NULL
		JOIN automation.versions v ON v.tenant_id = t.tenant_id AND v.automation_id = t.automation_id AND v.version = t.version
		WHERE t.tenant_id = $1 AND t.event_kind = $2 AND t.after_seq < $3 AND t.kind IN ('event', 'inbound')
		ORDER BY t.automation_id, t.trigger_id`, ev.TenantID, ev.Event, ev.Seq)
	if err != nil {
		return nil, &host.InfraError{Err: serrors.FromDB(opRoute, err)}
	}
	defer rows.Close()
	var out []fanTrigger
	for rows.Next() {
		var t fanTrigger
		var raw []byte
		if err := rows.Scan(&t.automationID, &t.version, &t.triggerID, &t.kind, &t.runAs, &raw); err != nil {
			return nil, &host.InfraError{Err: serrors.FromDB(opRoute, err)}
		}
		t.graph = raw
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, &host.InfraError{Err: serrors.FromDB(opRoute, err)}
	}
	return out, nil
}

func (r *Runtime) fanWaits(ctx context.Context, tx pgx.Tx, ev events.Envelope) ([]fanWait, error) {
	var entity, key any
	if ev.Subject != nil {
		entity, key = ev.Subject.Entity, ev.Subject.Key
	}
	rows, err := tx.Query(ctx, `
		SELECT w.run_id, w.wait_id, w.node_id, r.instance_id, r.automation_id, r.version, r.subject_entity, r.subject_key,
			r.run_as_user_id, r.run_as_kind, v.graph
		FROM automation.waits w
		JOIN automation.runs r ON r.tenant_id = w.tenant_id AND r.id = w.run_id
		JOIN automation.versions v ON v.tenant_id = r.tenant_id AND v.automation_id = r.automation_id AND v.version = r.version
		WHERE w.tenant_id = $1 AND w.status = 'open' AND w.kind = 'event' AND w.event_kind = $2 AND w.after_seq < $3
		  AND (w.subject_entity IS NULL OR (w.subject_entity = $4 AND w.subject_key = $5))
		  AND r.instance_id IS NOT NULL AND r.run_as_user_id IS NOT NULL
		ORDER BY w.created_at, w.run_id, w.wait_id`, ev.TenantID, ev.Event, ev.Seq, entity, key)
	if err != nil {
		return nil, &host.InfraError{Err: serrors.FromDB(opRoute, err)}
	}
	defer rows.Close()
	var out []fanWait
	for rows.Next() {
		var w fanWait
		var raw []byte
		if err := rows.Scan(&w.runID, &w.waitID, &w.nodeID, &w.instance, &w.automationID, &w.version, &w.subject.Entity, &w.subject.Key,
			&w.runAsUser, &w.runAsKind, &raw); err != nil {
			return nil, &host.InfraError{Err: serrors.FromDB(opRoute, err)}
		}
		w.graph = raw
		out = append(out, w)
	}
	if err := rows.Err(); err != nil {
		return nil, &host.InfraError{Err: serrors.FromDB(opRoute, err)}
	}
	return out, nil
}

// fanOut starts a run of one trigger for the event, or records why it did not start.
func (r *Runtime) fanOut(ctx context.Context, tx pgx.Tx, ev events.Envelope, subject SubjectRef, by *cause, t fanTrigger, proj *projector) error {
	req := startRequest{
		tenant: ev.TenantID, automationID: t.automationID, version: t.version, triggerID: t.triggerID, kind: t.kind,
		eventID: ev.ID, key: fmt.Sprintf("%s@%d:%s", t.automationID, t.version, ev.ID), subject: subject,
		initiator: ev.InitiatorUserID,
	}
	if by != nil {
		req.depth, req.cause = by.Depth+1, &by.RunID
	}
	ie, err := r.interpreter(ctx, ev.TenantID, t.automationID, t.version, t.graph)
	if err != nil {
		if host.IsInfrastructure(err) {
			return err
		}
		req.startState, req.reason = startFilterFailed, "invalid_graph: "+err.Error()
		_, err := r.startRun(ctx, tx, req)
		return infra(err)
	}
	req.in = ie.in
	mode := events.RunAsOwner
	if graph.RunAs(t.runAs) == graph.RunAsInitiator {
		mode = events.RunAsInitiator
	}
	runAs, subjErr := events.ResolveSubject(ctx, tx, r.cfg.Users, ev, ownerTrigger(t.automationID), mode)
	if subjErr != nil && !errors.Is(subjErr, events.ErrNoSubject) {
		return subjErr
	}
	var as *spi.Subject
	if subjErr == nil {
		as = &runAs
	}
	payload, err := decodeMap(ev.Payload)
	if err != nil {
		payload = map[string]any{}
	}
	if payload, err = proj.project(ctx, ev.Event, payload, as); err != nil {
		return &host.InfraError{Err: err}
	}
	gev := graph.Event{}
	for k, v := range payload {
		gev[k] = v
	}
	gev["kind"] = ev.Event
	req.event = gev
	vars := map[string]any{}
	if as != nil && r.cfg.Subjects != nil && normSubject(subject).Entity != graph.SubjectNone {
		raw, err := r.cfg.Subjects.Load(ctx, *as, subject)
		switch {
		case errors.Is(err, ErrNotFound):
		case err != nil:
			return &host.InfraError{Err: err}
		default:
			if vars, err = decodeMap(raw); err != nil {
				vars = map[string]any{}
			}
		}
	}
	gctx := graph.Context{Event: gev, Vars: vars, Now: ev.OccurredAt, Tenant: map[string]any{"id": ev.TenantID.String()}}
	mismatch, st := ie.in.TriggerMismatch(t.triggerID, gev, gctx)
	switch {
	case st != nil:
		e := stepErr(*st)
		req.startState, req.reason = startFilterFailed, "filter_error: "+e.Message
		if e.Code == CodeLimitExceeded {
			req.reason = CodeLimitExceeded + ": " + e.Message
		}
	case mismatch != "":
		return nil
	case as == nil:
		req.startState, req.reason = startBlocked, "no_owner"
	case req.depth > r.cfg.Limits.MaxDepth:
		req.startState, req.reason = startBlocked, "depth_exceeded"
	default:
		req.runAs = as
	}
	if _, err := r.startRun(ctx, tx, req); err != nil {
		return infra(err)
	}
	return nil
}

// wake signals an open wait of a running graph with the event, when the wait's filter does not reject it
// already from the event alone (the run checks it again in the token's context).
func (r *Runtime) wake(ctx context.Context, tx pgx.Tx, ev events.Envelope, w fanWait, proj *projector) error {
	// The run-as with the rights it holds now: the payload and the filter see what it may read.
	as, err := r.cfg.Users.LoadSubject(ctx, ev.TenantID, w.runAsUser, spi.SubjectKind(w.runAsKind))
	if errors.Is(err, host.ErrSubjectBlocked) {
		r.cfg.Logger.Info("automation: wait not signalled, the run-as is blocked", "run_id", w.runID.String(), "wait_id", w.waitID)
		return nil
	}
	if err != nil {
		return &host.InfraError{Err: err}
	}
	as.TenantID, as.UserID, as.Kind = ev.TenantID, w.runAsUser, spi.SubjectKind(w.runAsKind)
	payload, err := decodeMap(ev.Payload)
	if err != nil {
		payload = map[string]any{}
	}
	if payload, err = proj.project(ctx, ev.Event, payload, &as); err != nil {
		return &host.InfraError{Err: err}
	}
	if ie, err := r.interpreter(ctx, ev.TenantID, w.automationID, w.version, w.graph); err == nil {
		gev := graph.Event{}
		for k, v := range payload {
			gev[k] = v
		}
		gev["kind"] = ev.Event
		matched, st := ie.in.WaitMatches(w.nodeID, gev, graph.Context{Event: gev, Now: ev.OccurredAt, Tenant: map[string]any{"id": ev.TenantID.String()}})
		if st == nil && !matched {
			return nil
		}
	} else if host.IsInfrastructure(err) {
		return err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	msg, err := json.Marshal(inboxMessage{WaitResult: WaitResult{WaitID: w.waitID}, Event: &inboxEvent{ID: ev.ID, Seq: ev.Seq, Kind: ev.Event, Payload: raw}})
	if err != nil {
		return err
	}
	sp, err := tx.Begin(ctx)
	if err != nil {
		return &host.InfraError{Err: serrors.FromDB(opRoute, err)}
	}
	err = r.cfg.Workflows.SignalInTx(ctx, sp, spi.Scope{Subject: as}, host.SignalIntent{
		Workflow: WorkflowID, Instance: w.instance.String(), Name: InboxSignal, Payload: msg, Key: "event:" + ev.ID + ":" + w.waitID,
		Producer: spi.Producer{EventID: ev.ID},
	})
	if err != nil {
		_ = sp.Rollback(ctx)
		if we, ok := pool.AsWireError(err); ok && (we.Code == workflow.CodeInstanceNotAccepting || we.Code == workflow.CodeInstanceNotFound || we.Code == workflow.CodeSignalDenied) {
			// The run finished meanwhile, or its run-as user is blocked (automation.run declares no
			// permission, so only a refused subject is denied): the instance blocks on its next claim and
			// an operator decides what it waits for after Resume.
			r.cfg.Logger.Info("automation: wait not signalled", "run_id", w.runID.String(), "wait_id", w.waitID, "reason", we.Code)
			return nil
		}
		return err
	}
	return serrors.FromDB(opRoute, sp.Commit(ctx))
}

// ownerTrigger is the trigger_owners ID of a graph: who runs it as owner.
func ownerTrigger(id uuid.UUID) string { return "automation:" + id.String() }
