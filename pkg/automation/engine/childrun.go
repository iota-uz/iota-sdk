package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
)

// Codes of a run a step could not start (StartFromStep).
const (
	// CodeNotEnabled: the graph is not one of the tenant's or is not enabled.
	CodeNotEnabled = "not_enabled"
	// CodeNoEntry: the enabled version has no trigger another automation starts it through: a
	// «Called by an automation» trigger (EventCalled) or a manual one.
	CodeNoEntry = "no_entry"
	// CodeDepthExceeded: the run would pass the cascade limit (Limits.MaxDepth).
	CodeDepthExceeded = "depth_exceeded"
	// CodeNoOwner: the graph runs as its owner and has none.
	CodeNoOwner = "no_owner"
	// CodeSubjectBusy: the run waited for is about a conversation another run holds: it would only start
	// after that run, so the step that waits for it fails instead.
	CodeSubjectBusy = "subject_busy"
)

// StepStart is a run a node starts from its step: the enabled version of another graph of the tenant,
// through its entry: the first «Called by an automation» trigger (EventCalled) by id, else the first manual
// one, with the input as event.input.
type StepStart struct {
	AutomationID uuid.UUID
	Input        map[string]any
	// Subject is what the run is about; nil: the subject of the step's run.
	Subject *SubjectRef
	// Attached makes the run a child of the step's run: cancelling the step's run cancels it (a node that
	// waits for the run). A detached run lives on its own.
	Attached bool
}

// RunStarter starts the runs of other graphs from a step, in the step's transaction (Runtime implements
// it): start_automation and run_automation.
type RunStarter interface {
	StartFromStep(ctx context.Context, tx pgx.Tx, run StepContext, in StepStart) (RunRef, error)
}

var _ RunStarter = (*Runtime)(nil)

// StartFromStep starts a run of the graph in the step's transaction. The run-as of the step's run needs
// Automations.Operate, as a manual start does (forbidden otherwise). The run's depth is the step's run's
// plus one (the cascade limit, Limits.MaxDepth) and its cause is that run; it runs as the graph's run_as
// says, the initiator being the step's run-as. A refusal of the graph's state or of the input is a
// *StepError with a code (not_enabled, depth_exceeded, invalid_input, no_owner, subject_busy).
func (r *Runtime) StartFromStep(ctx context.Context, tx pgx.Tx, run StepContext, in StepStart) (RunRef, error) {
	ref, err := r.startFromStep(ctx, tx, run, in)
	var se *StepError
	if err != nil && !errors.As(err, &se) && !host.IsInfrastructure(err) {
		// A database failure (a lock that deadlocked, a lost connection) is retried with the step, never
		// the node's failure.
		err = &host.InfraError{Err: err}
	}
	return ref, err
}

func (r *Runtime) startFromStep(ctx context.Context, tx pgx.Tx, run StepContext, in StepStart) (RunRef, error) {
	refuse := func(code, format string, args ...any) (RunRef, error) {
		return RunRef{}, &StepError{Code: code, Message: fmt.Sprintf(format, args...)}
	}
	if !run.RunAs.Has(PermOperate) {
		return refuse(CodeForbidden, "the run-as may not start automations (%s)", PermOperate)
	}
	if in.AutomationID == uuid.Nil {
		return refuse(CodeNotEnabled, "no graph to start")
	}
	if run.Depth+1 > r.cfg.Limits.MaxDepth {
		return refuse(CodeDepthExceeded, "starting %s would pass the cascade limit of %d", in.AutomationID, r.cfg.Limits.MaxDepth)
	}
	am, err := loadAutomation(ctx, tx, run.TenantID, in.AutomationID, false)
	if errors.Is(err, ErrNotFound) {
		return refuse(CodeNotEnabled, "graph %s is not a graph of the tenant", in.AutomationID)
	}
	if err != nil {
		return RunRef{}, &host.InfraError{Err: err}
	}
	if am.EnabledVersion == nil {
		return refuse(CodeNotEnabled, "graph %s is not enabled", in.AutomationID)
	}
	v, err := loadVersion(ctx, tx, run.TenantID, in.AutomationID, *am.EnabledVersion)
	if err != nil {
		return RunRef{}, &host.InfraError{Err: err}
	}
	ie, err := r.interpreter(ctx, run.TenantID, in.AutomationID, v.Version.Version, v.Graph)
	if err != nil {
		return RunRef{}, err
	}
	// The entry: the first «Called by an automation» trigger by id, else the first manual one.
	var trig *graph.Trigger
	triggers := append([]graph.Trigger(nil), ie.in.Graph().Triggers...)
	sort.Slice(triggers, func(i, j int) bool { return triggers[i].ID < triggers[j].ID })
	for _, want := range []string{EventCalled, EventManual} {
		for i := range triggers {
			if trig == nil && triggers[i].Event == want {
				trig = &triggers[i]
			}
		}
	}
	if trig == nil {
		return refuse(CodeNoEntry, "graph %s has no «Called by an automation» or manual trigger to start it with", in.AutomationID)
	}
	input := in.Input
	if input == nil {
		input = map[string]any{}
	}
	if schema, ok := trig.Params["input_schema"].(map[string]any); ok && len(schema) > 0 {
		raw, err := json.Marshal(schema)
		if err != nil {
			return RunRef{}, err
		}
		ps, err := graph.SchemaParams(raw)
		if err != nil {
			return refuse(CodeInvalidInput, "the input schema of %s: %v", in.AutomationID, err)
		}
		parsed, issues := ps.Parse(input)
		if len(issues) > 0 {
			return refuse(CodeInvalidInput, "input %v: %s", issues[0].Path, issues[0].Message)
		}
		input = parsed
	}
	subject := normSubject(run.Subject)
	if subject.Entity == graph.SubjectConversation && in.Attached && in.Subject == nil {
		// The step's run holds its conversation and waits for the run: one about the conversation would
		// only start after it. A detached run keeps the conversation and queues behind this run.
		subject = SubjectRef{Entity: graph.SubjectNone}
	}
	if in.Subject != nil {
		subject = normSubject(*in.Subject)
	}
	user := run.RunAs.UserID
	now := time.Now()
	ev := events.Envelope{ID: uuid.NewString(), TenantID: run.TenantID, Event: trig.Event, InitiatorUserID: &user, OccurredAt: now}
	gev := graph.Event{"kind": trig.Event, "input": input, "initiator_user_id": user}
	if trig.Event == EventCalled {
		gev["caller_run_id"], gev["caller_automation_id"] = run.RunID.String(), run.AutomationID.String()
	}
	req := startRequest{
		tenant: run.TenantID, automationID: in.AutomationID, version: v.Version.Version, in: ie.in, triggerID: trig.ID,
		kind: KindManual, key: fmt.Sprintf("step:%s:%s:%d", run.RunID, run.TokenID, run.Seq), event: gev,
		subject: subject, initiator: &user, depth: run.Depth + 1,
	}
	if run.RunID != uuid.Nil {
		cause := run.RunID
		req.cause = &cause
	}
	if in.Attached && run.Native != nil {
		parent := run.Native.Instance().ID
		req.parent = &parent
	}
	// The step holds its own subject's lock; the new run's subject and the graph's row come after it, in
	// the order a manual start takes them.
	if subject.Entity != graph.SubjectNone {
		if err := lockSubjects(ctx, tx, subjectLockKey(run.TenantID, subject, uuid.Nil)); err != nil {
			return RunRef{}, err
		}
	}
	if err := lockGraphShared(ctx, tx, run.TenantID, in.AutomationID, v.Version.Version); err != nil {
		if errors.Is(err, ErrNotEnabled) || errors.Is(err, ErrNotFound) {
			return refuse(CodeNotEnabled, "graph %s is not enabled", in.AutomationID)
		}
		return RunRef{}, err
	}
	mode := events.RunAsOwner
	if graph.RunAs(v.RunAs) == graph.RunAsInitiator {
		mode = events.RunAsInitiator
	}
	runAs, err := events.ResolveSubject(ctx, tx, r.cfg.Users, ev, ownerTrigger(in.AutomationID), mode)
	switch {
	case errors.Is(err, host.ErrSubjectBlocked):
		return refuse(CodeForbidden, "the run-as of %s is blocked", in.AutomationID)
	case errors.Is(err, events.ErrNoSubject):
		return refuse(CodeNoOwner, "graph %s has no owner to run as", in.AutomationID)
	case err != nil:
		return RunRef{}, err
	}
	req.runAs = &runAs
	gctx := graph.Context{Event: gev, Now: now, Tenant: map[string]any{"id": run.TenantID.String()}}
	mismatch, st := ie.in.TriggerMismatch(trig.ID, gev, gctx)
	switch {
	case st != nil:
		return refuse(CodeInvalidInput, "the entry trigger of %s: %s", in.AutomationID, stepErr(*st).Message)
	case mismatch != "":
		return refuse(CodeInvalidInput, "the entry trigger's %s of %s rejects the input", mismatch, in.AutomationID)
	}
	ref, err := r.startRun(ctx, tx, req)
	switch {
	case errors.Is(err, ErrNotEnabled), errors.Is(err, ErrNotFound):
		return refuse(CodeNotEnabled, "graph %s is not enabled", in.AutomationID)
	case err != nil:
		return RunRef{}, err
	case ref.Status == RunQueued && in.Attached:
		return refuse(CodeSubjectBusy, "the conversation of the run is busy: the run would only start after this one")
	}
	return ref, nil
}
