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
		SELECT t.tenant_id, t.automation_id, t.trigger_id, t.schedule, t.created_at
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
		if err := rows.Scan(&tenant, &id, &trigger, &raw, &since); err != nil {
			return nil, serrors.FromDB(opSchedule, err)
		}
		var p scheduleParams
		if err := json.Unmarshal(raw, &p); err != nil || p.Cron == "" {
			continue
		}
		t := tenant
		out = append(out, events.ScheduleDef{
			ID: scheduleID(id, trigger), Cron: p.Cron, Timezone: p.Timezone, ActiveFrom: since, TenantID: &t,
			Handler: events.SlotHandlerFunc(r.runSlot),
		})
	}
	return out, serrors.FromDB(opSchedule, rows.Err())
}

func scheduleID(automation uuid.UUID, trigger string) string {
	return "automation:" + automation.String() + ":" + trigger
}

// runSlot starts the run of a schedule slot with start key schedule:<def>:<tick>.
func (r *Runtime) runSlot(ctx context.Context, tx pgx.Tx, slot events.Slot) error {
	rest, ok := strings.CutPrefix(slot.ScheduleID, "automation:")
	if !ok {
		return fmt.Errorf("automation engine: schedule %s is not a graph's", slot.ScheduleID)
	}
	idPart, trigger, ok := strings.Cut(rest, ":")
	id, err := uuid.Parse(idPart)
	if !ok || err != nil {
		return fmt.Errorf("automation engine: schedule %s is not a graph's", slot.ScheduleID)
	}
	a, err := loadAutomation(ctx, tx, slot.TenantID, id, false)
	if errors.Is(err, ErrNotFound) || (err == nil && a.EnabledVersion == nil) {
		return nil // disabled or deleted since the scheduler listed it
	}
	if err != nil {
		return &host.InfraError{Err: err}
	}
	v, err := loadVersion(ctx, tx, slot.TenantID, id, *a.EnabledVersion)
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
