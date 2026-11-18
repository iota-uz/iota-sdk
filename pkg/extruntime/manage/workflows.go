package manage

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Workflows is the management surface of the durable engine;
// *workflow.Engine implements it.
type Workflows interface {
	List(ctx context.Context, f workflow.ListFilter) ([]workflow.Instance, error)
	Get(ctx context.Context, tenant, id uuid.UUID) (workflow.History, error)
	Signal(ctx context.Context, tenant uuid.UUID, sender int64, instance uuid.UUID, name string, payload json.RawMessage, key string) error
	DecideTask(ctx context.Context, actor spi.Subject, taskID uuid.UUID, decision json.RawMessage) error
	Terminate(ctx context.Context, tenant, id uuid.UUID) error
	StartV2(ctx context.Context, actor spi.Subject, predecessor uuid.UUID, version string) (uuid.UUID, error)
	Resolve(ctx context.Context, tenant, id uuid.UUID, name string, n int, result json.RawMessage) error
	Retry(ctx context.Context, tenant, id uuid.UUID, name string, n int) error
	Resume(ctx context.Context, tenant, id uuid.UUID) error
}

// Audit actions of workflow management.
const (
	ActionWorkflowSignal    = "workflow.signal"
	ActionWorkflowDecide    = "workflow.decide"
	ActionWorkflowTerminate = "workflow.terminate"
	ActionWorkflowStartV2   = "workflow.startv2"
	ActionWorkflowResolve   = "workflow.resolve"
	ActionWorkflowRetry     = "workflow.retry"
	ActionWorkflowResume    = "workflow.resume"
)

// SetWorkflows attaches the engine; workflow management fails explicitly
// without it. The runtime composition calls it once the engine exists (the
// engine needs this service's Implementations).
func (s *Service) SetWorkflows(w Workflows) { s.workflows = w }

func (s *Service) engine() (Workflows, error) {
	if s.workflows == nil {
		return nil, serrors.New(serrors.FailedPrecondition, "the workflow engine is not enabled")
	}
	return s.workflows, nil
}

// engineError maps the engine's sentinel errors to service error codes.
func engineError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, workflow.ErrNotFound):
		return serrors.New(serrors.NotFound, err.Error()).WithCause(err)
	case errors.Is(err, workflow.ErrForbidden):
		return serrors.New(serrors.PermissionDenied, err.Error()).WithCause(err)
	case errors.Is(err, workflow.ErrInvalid):
		return serrors.New(serrors.Invalid, err.Error()).WithCause(err)
	case errors.Is(err, workflow.ErrNotAccepting), errors.Is(err, workflow.ErrTaskClosed):
		return serrors.New(serrors.FailedPrecondition, err.Error()).WithCause(err)
	}
	return err
}

// ListWorkflows lists the tenant's workflow instances, newest first.
func (s *Service) ListWorkflows(ctx context.Context, actor spi.Subject, f workflow.ListFilter) ([]workflow.Instance, error) {
	if err := require(actor, PermRead); err != nil {
		return nil, err
	}
	w, err := s.engine()
	if err != nil {
		return nil, err
	}
	f.TenantID = actor.TenantID
	out, err := w.List(ctx, f)
	return out, engineError(err)
}

// GetWorkflow returns the history of one instance of the actor's tenant.
func (s *Service) GetWorkflow(ctx context.Context, actor spi.Subject, id uuid.UUID) (workflow.History, error) {
	if err := require(actor, PermRead); err != nil {
		return workflow.History{}, err
	}
	w, err := s.engine()
	if err != nil {
		return workflow.History{}, err
	}
	h, err := w.Get(ctx, actor.TenantID, id)
	return h, engineError(err)
}

// workflowAction runs an audited engine action.
func (s *Service) workflowAction(ctx context.Context, actor spi.Subject, perm string, e *auditEntry, fn func(w Workflows) error) error {
	return s.observe(ctx, actor, perm, e, func() error {
		w, err := s.engine()
		if err != nil {
			return err
		}
		return engineError(fn(w))
	})
}

func wfEntry(action string, id uuid.UUID, detail map[string]any) *auditEntry {
	if detail == nil {
		detail = map[string]any{}
	}
	return &auditEntry{Action: action, Target: id.String(), Detail: detail}
}

// SignalWorkflow delivers a named signal to an instance.
func (s *Service) SignalWorkflow(ctx context.Context, actor spi.Subject, id uuid.UUID, name string, payload json.RawMessage, key string) error {
	e := wfEntry(ActionWorkflowSignal, id, map[string]any{"signal": name, "key": key})
	return s.workflowAction(ctx, actor, PermOperate, e, func(w Workflows) error {
		return w.Signal(ctx, actor.TenantID, actor.UserID, id, name, payload, key)
	})
}

// DecideTask records the decision of a human task. The engine lets only the
// assignee (for a task assigned to a permission, a holder of it) or a holder of
// the operate permission decide.
func (s *Service) DecideTask(ctx context.Context, actor spi.Subject, taskID uuid.UUID, decision json.RawMessage) error {
	e := wfEntry(ActionWorkflowDecide, taskID, nil)
	return s.workflowAction(ctx, actor, "", e, func(w Workflows) error {
		return w.DecideTask(ctx, actor, taskID, decision)
	})
}

// TerminateWorkflow stops an instance (done effects stay).
func (s *Service) TerminateWorkflow(ctx context.Context, actor spi.Subject, id uuid.UUID) error {
	e := wfEntry(ActionWorkflowTerminate, id, nil)
	return s.workflowAction(ctx, actor, PermOperate, e, func(w Workflows) error {
		return w.Terminate(ctx, actor.TenantID, id)
	})
}

// StartWorkflowV2 starts a new instance of a finished, terminated or blocked
// one on the given package version ("" = the active one).
func (s *Service) StartWorkflowV2(ctx context.Context, actor spi.Subject, predecessor uuid.UUID, version string) (uuid.UUID, error) {
	e := wfEntry(ActionWorkflowStartV2, predecessor, map[string]any{"version": version})
	var out uuid.UUID
	err := s.workflowAction(ctx, actor, PermOperate, e, func(w Workflows) error {
		id, err := w.StartV2(ctx, actor, predecessor, version)
		out = id
		if err == nil {
			e.Detail["instance"] = id.String()
		}
		return err
	})
	return out, err
}

// ResolveWorkflowOccurrence completes a blocked call or step with a result.
func (s *Service) ResolveWorkflowOccurrence(ctx context.Context, actor spi.Subject, id uuid.UUID, name string, n int, result json.RawMessage) error {
	e := wfEntry(ActionWorkflowResolve, id, map[string]any{"occurrence": name, "n": n})
	return s.workflowAction(ctx, actor, PermOperate, e, func(w Workflows) error {
		return w.Resolve(ctx, actor.TenantID, id, name, n, result)
	})
}

// RetryWorkflowOccurrence forgets an unresolved occurrence so the next run
// executes it again; call ResumeWorkflow afterwards.
func (s *Service) RetryWorkflowOccurrence(ctx context.Context, actor spi.Subject, id uuid.UUID, name string, n int) error {
	e := wfEntry(ActionWorkflowRetry, id, map[string]any{"occurrence": name, "n": n})
	return s.workflowAction(ctx, actor, PermOperate, e, func(w Workflows) error {
		return w.Retry(ctx, actor.TenantID, id, name, n)
	})
}

// ResumeWorkflow makes a blocked instance runnable again.
func (s *Service) ResumeWorkflow(ctx context.Context, actor spi.Subject, id uuid.UUID) error {
	e := wfEntry(ActionWorkflowResume, id, nil)
	return s.workflowAction(ctx, actor, PermOperate, e, func(w Workflows) error {
		return w.Resume(ctx, actor.TenantID, id)
	})
}
