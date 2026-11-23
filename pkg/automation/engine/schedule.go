package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/robfig/cron/v3"

	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const opSchedule = serrors.Op("automation.engine.schedule")

// scheduleParams are the params of a schedule trigger.
type scheduleParams struct {
	Cron     string `json:"cron"`
	Timezone string `json:"timezone,omitempty"`
}

// Schedules is the source of the tenant schedules of enabled graphs: one ScheduleDef per schedule
// trigger of an enabled version, ID automation:<id>:<trigger_id>; register it with
// wiring.ContributeSchedules.
func (r *Runtime) Schedules() events.ScheduleSource {
	return events.ScheduleSourceFunc(r.activeSchedules)
}

func (r *Runtime) activeSchedules(ctx context.Context) ([]events.ScheduleDef, error) {
	rows, err := r.cfg.DB.Query(ctx, `
		SELECT t.tenant_id, t.automation_id, t.trigger_id, t.schedule, t.created_at, t.version
		FROM automation.triggers t
		JOIN automation.automations a ON a.tenant_id = t.tenant_id AND a.id = t.automation_id AND a.enabled_version = t.version AND a.deleted_at IS NULL
		WHERE t.kind = 'schedule'
		ORDER BY t.tenant_id, t.automation_id, t.trigger_id`)
	if err != nil {
		return nil, serrors.FromDB(opSchedule, err)
	}
	defer rows.Close()
	var out []events.ScheduleDef
	for rows.Next() {
		var tenant, id uuid.UUID
		var trigger string
		var raw []byte
		var since time.Time
		var version int
		if err := rows.Scan(&tenant, &id, &trigger, &raw, &since, &version); err != nil {
			return nil, serrors.FromDB(opSchedule, err)
		}
		var p scheduleParams
		if err := json.Unmarshal(raw, &p); err != nil || checkSchedule(p) != nil {
			continue // an interval or a bad cron of a version saved before the rule: never planned
		}
		t := tenant
		gen := scheduleGen{version: version, since: since}
		out = append(out, events.ScheduleDef{
			ID: scheduleID(id, trigger), Cron: p.Cron, Timezone: p.Timezone, ActiveFrom: since, TenantID: &t,
			Handler: events.SlotHandlerFunc(func(ctx context.Context, tx pgx.Tx, slot events.Slot) error { return r.runSlot(ctx, tx, slot, gen) }),
		})
	}
	return out, serrors.FromDB(opSchedule, rows.Err())
}

// scheduleGen is the enable a listed schedule definition comes from: its version and when its trigger
// was written.
type scheduleGen struct {
	version int
	since   time.Time
}

// calendarParser reads the schedules a graph may have: five calendar fields, no descriptors. An
// interval (@every) has no calendar: the scheduler re-anchors it on its moving lookback window, so its
// slots would shift between passes; it is refused on save (scheduleRule).
var calendarParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// checkSchedule reports why schedule params cannot run: a cron that is not five calendar fields, or an
// unknown time zone.
func checkSchedule(p scheduleParams) error {
	if _, err := calendarParser.Parse(p.Cron); err != nil {
		return fmt.Errorf("a schedule is five cron fields (minute hour day month weekday): %w", err)
	}
	if p.Timezone != "" {
		if _, err := time.LoadLocation(p.Timezone); err != nil {
			return fmt.Errorf("unknown time zone %q", p.Timezone)
		}
	}
	return nil
}

// isTick reports whether at is a planned time of the calendar cron in the time zone. Anything else (an
// interval of a version saved before the rule) has no tick a slot could be checked against.
func isTick(p scheduleParams, at time.Time) bool {
	spec, err := calendarParser.Parse(p.Cron)
	if err != nil {
		return false
	}
	loc := time.UTC
	if p.Timezone != "" {
		if loc, err = time.LoadLocation(p.Timezone); err != nil {
			return false
		}
	}
	return spec.Next(at.In(loc).Add(-time.Second)).Equal(at)
}

func scheduleID(automation uuid.UUID, trigger string) string {
	return "automation:" + automation.String() + ":" + trigger
}

// runSlot starts the run of a schedule slot with start key schedule:<def>:<tick>.
func (r *Runtime) runSlot(ctx context.Context, tx pgx.Tx, slot events.Slot, gen scheduleGen) error {
	rest, ok := strings.CutPrefix(slot.ScheduleID, "automation:")
	if !ok {
		return fmt.Errorf("automation engine: schedule %s is not a graph's", slot.ScheduleID)
	}
	idPart, trigger, ok := strings.Cut(rest, ":")
	id, err := uuid.Parse(idPart)
	if !ok || err != nil {
		return fmt.Errorf("automation engine: schedule %s is not a graph's", slot.ScheduleID)
	}
	// Under the graph's row lock (the lock Enable and Disable hold): the trigger of the current enable.
	var enabled *int
	err = tx.QueryRow(ctx, `SELECT enabled_version FROM automation.automations WHERE tenant_id = $1 AND id = $2 AND deleted_at IS NULL FOR SHARE`,
		slot.TenantID, id).Scan(&enabled)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && enabled == nil) {
		return nil // disabled or deleted since the scheduler listed it
	}
	if err != nil {
		return &host.InfraError{Err: serrors.FromDB(opSchedule, err)}
	}
	var raw []byte
	var since time.Time
	var tversion int
	err = tx.QueryRow(ctx, `SELECT schedule, created_at, version FROM automation.triggers WHERE tenant_id = $1 AND automation_id = $2 AND trigger_id = $3 AND kind = 'schedule'`,
		slot.TenantID, id, trigger).Scan(&raw, &since, &tversion)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // the enabled version has no such schedule any more
	}
	if err != nil {
		return &host.InfraError{Err: serrors.FromDB(opSchedule, err)}
	}
	if tversion != *enabled {
		return nil
	}
	if tversion != gen.version || !since.Equal(gen.since) {
		// The definition this handler was listed with is not the current one. The slot is judged by the
		// current trigger below (its activation and its cron), not handed back: a retry would spend the
		// scheduler's attempts on a slot the current enable may run.
		r.cfg.Logger.Debug("automation: schedule slot of an earlier listing, judged by the current trigger",
			"schedule", slot.ScheduleID, "listed_version", gen.version, "version", tversion)
	}
	var p scheduleParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("automation engine: schedule %s: %w", slot.ScheduleID, err)
	}
	if slot.At.Before(since) || !isTick(p, slot.At) {
		return nil // not a slot of this enable: before it, or not on its cron
	}
	v, err := loadVersion(ctx, tx, slot.TenantID, id, tversion)
	if err != nil {
		return &host.InfraError{Err: err}
	}
	at := slot.At.UTC().Format(time.RFC3339)
	req := startRequest{
		tenant: slot.TenantID, automationID: id, version: v.Version.Version, triggerID: trigger, kind: KindSchedule,
		key: fmt.Sprintf("schedule:%s:%d", slot.ScheduleID, slot.At.Unix()), subject: SubjectRef{Entity: graph.SubjectNone},
		event: graph.Event{"kind": EventSchedule, "scheduled_at": at},
	}
	ie, err := r.interpreter(ctx, slot.TenantID, id, v.Version.Version, v.Graph)
	if err != nil {
		if host.IsInfrastructure(err) {
			return err
		}
		req.startState, req.reason = startFilterFailed, "invalid_graph: "+err.Error()
		if _, err := r.startRun(ctx, tx, req); err != nil && !notEligible(err) {
			return err
		}
		return nil
	}
	req.in = ie.in
	ev := events.Envelope{ID: req.key, TenantID: slot.TenantID, Event: EventSchedule, OccurredAt: slot.At}
	var runAs spi.Subject
	if graph.RunAs(v.RunAs) == graph.RunAsInitiator {
		// A slot has no initiator: a version that runs as the initiator does not run as the owner
		// instead (saving it is refused; this is a version saved before that rule).
		err = fmt.Errorf("%w: a schedule has no initiator", events.ErrNoSubject)
		req.reason = "no_initiator"
	} else {
		runAs, err = events.ResolveSubject(ctx, tx, r.cfg.Users, ev, ownerTrigger(id), events.RunAsOwner)
	}
	switch {
	case errors.Is(err, events.ErrNoSubject) && req.reason == "no_initiator":
		req.startState = startBlocked
	case errors.Is(err, events.ErrNoSubject):
		req.startState, req.reason = startBlocked, "no_owner"
	case err != nil:
		return err
	default:
		req.runAs = &runAs
	}
	if req.runAs != nil {
		gctx := graph.Context{Event: req.event, Now: slot.At, Tenant: map[string]any{"id": slot.TenantID.String()}}
		mismatch, st := ie.in.TriggerMismatch(trigger, req.event, gctx)
		switch {
		case st != nil:
			req.runAs, req.startState, req.reason = nil, startFilterFailed, "filter_error: "+stepErr(*st).Message
		case mismatch != "":
			return nil
		}
	}
	if _, err = r.startRun(ctx, tx, req); err != nil && !notEligible(err) {
		return err
	}
	return nil
}
