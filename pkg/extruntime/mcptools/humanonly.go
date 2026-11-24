package mcptools

import (
	"context"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// IssueHumanRequired is the issue code of a refusal a person must act on instead of the agent. It
// travels in the shared failure contract: approval_required with recovery open_approval, and the URL of
// the page where the person acts in the message.
const IssueHumanRequired = "HUMAN_REQUIRED"

// HumanOnly tells which workflow instances only a person may act on, such as the instances of the
// native contracts of automations. The default (Deps.HumanOnly nil) asks the workflow engine through
// manage.Service, which knows the contracts marked workflow.NativeContract.HumanOnly.
//
// Every action tool (signal, decide, terminate, start v2, resolve, retry, resume) on such an instance
// returns HUMAN_REQUIRED with the page where the person acts, once the caller passed the action's own
// check (Extensions.Operate, or the right to decide the task): who may not act learns nothing. Reads
// stay, without the payloads of such instances (input, result, errors, journal results, signal
// payloads, decisions): the product shows those, projected for the viewer, on its own pages.
type HumanOnly interface {
	// Contract reports whether a workflow contract is human-only.
	Contract(ctx context.Context, workflow string) (bool, error)
	// Instance reports whether instance id of the tenant is guarded and the URL of its page.
	Instance(ctx context.Context, tenant, id uuid.UUID) (url string, guarded bool, err error)
	// Task reports whether the human task is a task of a guarded instance and the URL of its page, for
	// an actor who may decide the task; for anyone else it is not guarded.
	Task(ctx context.Context, actor spi.Subject, taskID uuid.UUID) (url string, guarded bool, err error)
}

// HumanRequired is the refusal of an action only a person may take; path names the input that
// addresses the guarded object.
func HumanRequired(path, action, url string) *mcp.Failure {
	msg := "only a person may " + action
	if url != "" {
		msg += "; open " + url
	}
	return mcp.Fail(mcp.CodeApprovalRequired, mcp.RecoveryOpenApproval, msg,
		mcp.Issue{Path: path, Code: IssueHumanRequired, Message: msg})
}

// guardInstance refuses an operator action on a guarded instance before the service runs. An actor
// without Extensions.Operate is refused by the service as for any instance, so the guard is not asked.
func guardInstance(ctx context.Context, g HumanOnly, actor spi.Subject, id uuid.UUID, action string) error {
	if g == nil || !actor.Has(manage.PermOperate) {
		return nil
	}
	url, guarded, err := g.Instance(ctx, actor.TenantID, id)
	if err != nil {
		return serrors.Wrap(serrors.Op("extruntime.mcptools.guardInstance"), err)
	}
	if guarded {
		return HumanRequired("/id", action, url)
	}
	return nil
}

func guardTask(ctx context.Context, g HumanOnly, actor spi.Subject, id uuid.UUID) error {
	if g == nil {
		return nil
	}
	url, guarded, err := g.Task(ctx, actor, id)
	if err != nil {
		return serrors.Wrap(serrors.Op("extruntime.mcptools.guardTask"), err)
	}
	if guarded {
		return HumanRequired("/task_id", "decide this task", url)
	}
	return nil
}

// redactInstance leaves out what a human-only instance carries: its input (a call's rendered params),
// result, error and blocked detail.
func redactInstance(i workflow.Instance) workflow.Instance {
	i.Input, i.Result, i.Error, i.BlockedDetail = nil, nil, nil, ""
	return i
}

// redactHistory is redactInstance of a history with its journal results, signal payloads and the
// decisions of its tasks.
func redactHistory(h workflow.History) workflow.History {
	h.Instance = redactInstance(h.Instance)
	occ := make([]workflow.Occurrence, len(h.Occurrences))
	for i, o := range h.Occurrences {
		o.Result, o.Error, o.Meta = nil, nil, nil
		occ[i] = o
	}
	h.Occurrences = occ
	sig := make([]workflow.Signal, len(h.Signals))
	for i, s := range h.Signals {
		s.Payload = nil
		sig[i] = s
	}
	h.Signals = sig
	tasks := make([]workflow.Task, len(h.Tasks))
	for i, t := range h.Tasks {
		t.Decision = nil
		ds := make([]workflow.TaskDecision, len(t.Decisions))
		for j, d := range t.Decisions {
			d.Decision = nil
			ds[j] = d
		}
		t.Decisions = ds
		tasks[i] = t
	}
	h.Tasks = tasks
	return h
}

// managedHumanOnly is the default guard: the human-only native contracts of the workflow engine the
// management service runs on.
type managedHumanOnly struct{ svc *manage.Service }

func (m managedHumanOnly) Contract(_ context.Context, workflow string) (bool, error) {
	return m.svc.HumanOnlyWorkflow(workflow)
}

func (m managedHumanOnly) Instance(ctx context.Context, tenant, id uuid.UUID) (string, bool, error) {
	return m.svc.HumanOnlyInstance(ctx, tenant, id)
}

func (m managedHumanOnly) Task(ctx context.Context, actor spi.Subject, taskID uuid.UUID) (string, bool, error) {
	return m.svc.HumanOnlyTask(ctx, actor, taskID)
}
