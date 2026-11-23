package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const opCollect = serrors.Op("automation.engine.collect")

// collected is one event of a collect window, as the run will read it.
type collected struct {
	ID         string          `json:"id"`
	Seq        int64           `json:"seq"`
	Event      json.RawMessage `json:"event"` // the graph's event: {kind, ...payload}, projected
	Payload    json.RawMessage `json:"payload"`
	OccurredAt time.Time       `json:"occurred_at"`
	Initiator  *int64          `json:"initiator_user_id,omitempty"`
	Subject    SubjectRef      `json:"subject"`
}

// collect adds an event to its trigger's window instead of starting a run: events with one key inside
// the window start one run when it closes (Maintain), or at once when it reaches its max.
func (r *Runtime) collect(ctx context.Context, tx pgx.Tx, ev events.Envelope, t fanTrigger, trig graph.Trigger, req startRequest, gctx graph.Context) error {
	key, _, st := req.in.CollectKey(t.triggerID, req.event, gctx)
	if st != nil {
		e := stepErr(*st)
		req.startState, req.reason = startFilterFailed, "collect_key_error: "+e.Message
		req.key = diagnosticKey(req.key, t.triggerID)
		req.runAs = nil
		if _, err := r.startRun(ctx, tx, req); err != nil && !notEligible(err) {
			return infra(err)
		}
		return nil
	}
	window, ok := trig.Collect.WindowDuration()
	if !ok {
		window = graph.CollectMinWindow
	}
	gev, err := json.Marshal(req.event)
	if err != nil {
		return err
	}
	item, err := json.Marshal(collected{ID: ev.ID, Seq: ev.Seq, Event: gev, Payload: nonNull(ev.Payload), OccurredAt: ev.OccurredAt,
		Initiator: ev.InitiatorUserID, Subject: req.subject})
	if err != nil {
		return err
	}
	at := ev.OccurredAt
	if at.IsZero() {
		at = time.Now()
	}
	var count int
	err = tx.QueryRow(ctx, `
		INSERT INTO automation.inbound_collect (tenant_id, automation_id, version, trigger_id, collect_key, events, count, first_at, deadline)
		VALUES ($1, $2, $3, $4, $5, jsonb_build_array($6::jsonb), 1, $7::timestamptz, $7::timestamptz + make_interval(secs => $8::float8))
		ON CONFLICT (tenant_id, automation_id, trigger_id, collect_key) DO UPDATE SET
			version = EXCLUDED.version,
			events = automation.inbound_collect.events || EXCLUDED.events,
			count = automation.inbound_collect.count + 1,
			deadline = LEAST(EXCLUDED.first_at + make_interval(secs => $8::float8), automation.inbound_collect.first_at + make_interval(secs => $8::float8 * 2))
		RETURNING count`,
		ev.TenantID, t.automationID, t.version, t.triggerID, key, item, at, window.Seconds()).Scan(&count)
	if err != nil {
		return &host.InfraError{Err: serrors.FromDB(opCollect, err)}
	}
	if count >= trig.Collect.MaxEvents() {
		return r.closeWindow(ctx, tx, ev.TenantID, t.automationID, t.triggerID, key)
	}
	if !events.AfterCommit(ctx, r.pokeServe) {
		r.pokeServe(ctx)
	}
	return nil
}

func (r *Runtime) pokeServe(context.Context) {
	select {
	case r.poke <- struct{}{}:
	default:
	}
}

// flushCollects starts the runs of the windows whose deadline passed.
func (r *Runtime) flushCollects(ctx context.Context) error {
	type window struct {
		tenant, automation uuid.UUID
		trigger, key       string
	}
	rows, err := r.cfg.DB.Query(ctx, `SELECT tenant_id, automation_id, trigger_id, collect_key FROM automation.inbound_collect WHERE deadline <= now() ORDER BY deadline LIMIT 100`)
	if err != nil {
		return serrors.FromDB(opCollect, err)
	}
	var due []window
	for rows.Next() {
		var w window
		if err := rows.Scan(&w.tenant, &w.automation, &w.trigger, &w.key); err != nil {
			rows.Close()
			return serrors.FromDB(opCollect, err)
		}
		due = append(due, w)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return serrors.FromDB(opCollect, err)
	}
	var errs []error
	for _, w := range due {
		err := pgx.BeginFunc(ctx, r.cfg.DB, func(tx pgx.Tx) error {
			return r.closeWindow(ctx, tx, w.tenant, w.automation, w.trigger, w.key)
		})
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// nextCollect is the deadline of the window that closes first; ok is false with none open.
func (r *Runtime) nextCollect(ctx context.Context) (time.Time, bool, error) {
	var at *time.Time
	if err := r.cfg.DB.QueryRow(ctx, `SELECT min(deadline) FROM automation.inbound_collect`).Scan(&at); err != nil {
		return time.Time{}, false, serrors.FromDB(opCollect, err)
	}
	if at == nil {
		return time.Time{}, false, nil
	}
	return *at, true, nil
}

// readCollected reads the events of a window; unreadable ones are none.
func readCollected(raw []byte) []collected {
	var items []collected
	if json.Unmarshal(raw, &items) != nil {
		return nil
	}
	return items
}

// closeWindow removes a window and starts its run: event is its last event, events all of them.
func (r *Runtime) closeWindow(ctx context.Context, tx pgx.Tx, tenant, automation uuid.UUID, trigger, key string) error {
	var version int
	var raw []byte
	err := tx.QueryRow(ctx, `
		DELETE FROM automation.inbound_collect WHERE tenant_id = $1 AND automation_id = $2 AND trigger_id = $3 AND collect_key = $4
		RETURNING version, events`, tenant, automation, trigger, key).Scan(&version, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // another process closed it
	}
	if err != nil {
		return serrors.FromDB(opCollect, err)
	}
	items := readCollected(raw)
	if len(items) == 0 {
		return nil // nothing readable to start
	}
	v, err := loadVersion(ctx, tx, tenant, automation, version)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	ie, err := r.interpreter(ctx, tenant, automation, version, v.Graph)
	if err != nil {
		if host.IsInfrastructure(err) {
			return err
		}
		return nil // the version no longer runs against the catalog: the window starts nothing
	}
	first, last := items[0], items[len(items)-1]
	evs := make([]graph.Event, 0, len(items))
	for _, it := range items {
		m, err := decodeMap(it.Event)
		if err != nil {
			continue
		}
		evs = append(evs, graph.Event(m))
	}
	lastEvent := graph.Event{}
	if len(evs) > 0 {
		lastEvent = evs[len(evs)-1]
	}
	req := startRequest{
		tenant: tenant, automationID: automation, version: version, in: ie.in, triggerID: trigger, kind: KindInbound,
		eventID: last.ID, key: fmt.Sprintf("%s@%d:collect:%s", automation, version, first.ID), subject: last.Subject,
		initiator: last.Initiator, eventSeq: first.Seq, event: lastEvent, events: evs,
	}
	mode := events.RunAsOwner
	if graph.RunAs(v.RunAs) == graph.RunAsInitiator {
		mode = events.RunAsInitiator
	}
	env := events.Envelope{ID: last.ID, Seq: last.Seq, TenantID: tenant, Event: lastEvent.Kind(), Payload: last.Payload,
		InitiatorUserID: last.Initiator, OccurredAt: last.OccurredAt}
	if s := normSubject(last.Subject); s.Entity != graph.SubjectNone {
		env.Subject = &spi.ObjectRef{Entity: s.Entity, Key: s.Key}
	}
	runAs, err := events.ResolveSubject(ctx, tx, r.cfg.Users, env, ownerTrigger(automation), mode)
	switch {
	case errors.Is(err, events.ErrNoSubject):
		req.startState, req.reason = startBlocked, "no_owner"
		req.key = diagnosticKey(req.key, trigger)
	case err != nil:
		return err
	default:
		req.runAs = &runAs
	}
	if _, err := r.startRun(ctx, tx, req); err != nil && !notEligible(err) {
		return infra(err)
	}
	return nil
}
