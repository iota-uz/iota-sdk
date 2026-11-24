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
// No action tool (signal, decide, terminate, start v2, resolve, retry, resume) ever passes such an
// instance or task on to the service. A caller who passes the action's own check (Extensions.Operate,
// or the right to decide the task) gets HUMAN_REQUIRED with the page where the person acts; anyone
// else gets the refusal that check gives, without the page. Reads stay, reduced to an allowlist of
// fields that carry no business data: the product shows the rest, projected for the viewer, on its
// own pages.
type HumanOnly interface {
	// Contract reports whether a workflow contract is human-only.
	Contract(ctx context.Context, workflow string) (bool, error)
	// Instance reports whether instance id of the tenant is guarded and the URL of its page.
	Instance(ctx context.Context, tenant, id uuid.UUID) (url string, guarded bool, err error)
	// Task reports whether the human task is a task of a guarded instance, whether the actor may decide
	// it now, and then the URL of its page.
	Task(ctx context.Context, actor spi.Subject, taskID uuid.UUID) (url string, guarded, mayDecide bool, err error)
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

// guardInstance refuses an operator action on a guarded instance before the service runs: the
// service never sees it. Without Extensions.Operate the refusal is the service's own, without the page.
func guardInstance(ctx context.Context, g HumanOnly, actor spi.Subject, id uuid.UUID, action string) error {
	if g == nil {
		return nil
	}
	url, guarded, err := g.Instance(ctx, actor.TenantID, id)
	if err != nil {
		return serrors.Wrap(serrors.Op("extruntime.mcptools.guardInstance"), err)
	}
	if !guarded {
		return nil
	}
	if !actor.Has(manage.PermOperate) {
		return serrors.New(serrors.PermissionDenied, "permission "+manage.PermOperate+" is required")
	}
	return HumanRequired("/id", action, url)
}

func guardTask(ctx context.Context, g HumanOnly, actor spi.Subject, id uuid.UUID) error {
	if g == nil {
		return nil
	}
	url, guarded, mayDecide, err := g.Task(ctx, actor, id)
	if err != nil {
		return serrors.Wrap(serrors.Op("extruntime.mcptools.guardTask"), err)
	}
	if !guarded {
		return nil
	}
	// A guarded task never reaches DecideTask: rights that change after this check cannot let it through.
	if !mayDecide {
		return serrors.New(serrors.PermissionDenied, "you may not decide this task")
	}
	return HumanRequired("/task_id", "decide this task", url)
}

// redactInstance keeps of a human-only instance only the fields that carry no business data: ids,
// contract, status and its code, counters and times. Its input (a call's rendered params), result,
// error, blocked detail and start key are left out.
func redactInstance(i workflow.Instance) workflow.Instance {
	return workflow.Instance{
		ID: i.ID, TenantID: i.TenantID, Workflow: i.Workflow, Package: i.Package, Version: i.Version,
		ArtifactHash: i.ArtifactHash, Export: i.Export, Status: i.Status, BlockedReason: i.BlockedReason,
		SubjectUserID: i.SubjectUserID, SubjectKind: i.SubjectKind, PredecessorID: i.PredecessorID,
		ParentID: i.ParentID, StartInvocation: i.StartInvocation, WakeAt: i.WakeAt, LeaseEpoch: i.LeaseEpoch,
		RunCount: i.RunCount, Failures: i.Failures, CreatedAt: i.CreatedAt, UpdatedAt: i.UpdatedAt,
		FinishedAt: i.FinishedAt, Native: i.Native,
	}
}

// redactHistory is the history of a human-only instance reduced to the same allowlist: journal entries
// without results, errors or meta; signals without payloads or keys; tasks without titles, subjects,
// decision forms, decisions, escalation steps or delegation comments. Invocations carry no data.
func redactHistory(h workflow.History) workflow.History {
	out := workflow.History{Instance: redactInstance(h.Instance), Invocations: h.Invocations,
		Occurrences: make([]workflow.Occurrence, 0, len(h.Occurrences)),
		Signals:     make([]workflow.Signal, 0, len(h.Signals)),
		Tasks:       make([]workflow.Task, 0, len(h.Tasks))}
	for _, o := range h.Occurrences {
		out.Occurrences = append(out.Occurrences, workflow.Occurrence{Name: o.Name, N: o.N, Kind: o.Kind, Status: o.Status,
			InputHash: o.InputHash, Attempts: o.Attempts, WakeAt: o.WakeAt, InvocationID: o.InvocationID,
			ContractHash: o.ContractHash, CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt})
	}
	for _, s := range h.Signals {
		out.Signals = append(out.Signals, workflow.Signal{ID: s.ID, Name: s.Name, SenderUserID: s.SenderUserID,
			ConsumedName: s.ConsumedName, ConsumedN: s.ConsumedN, CreatedAt: s.CreatedAt, ConsumedAt: s.ConsumedAt})
	}
	for _, t := range h.Tasks {
		rt := workflow.Task{ID: t.ID, InstanceID: t.InstanceID, Workflow: t.Workflow, Name: t.Name, N: t.N,
			Assignees: t.Assignees, Required: t.Required, Outcomes: t.Outcomes, Veto: t.Veto,
			EscalationLevel: t.EscalationLevel, AllowDelegation: t.AllowDelegation, AssigneeKind: t.AssigneeKind,
			AssigneeUserID: t.AssigneeUserID, AssigneePermission: t.AssigneePermission, Status: t.Status,
			Choice: t.Choice, DecidedBy: t.DecidedBy, DueAt: t.DueAt, CreatedAt: t.CreatedAt, DecidedAt: t.DecidedAt}
		for _, d := range t.Delegations {
			rt.Delegations = append(rt.Delegations, workflow.TaskDelegation{From: d.From, To: d.To, At: d.At})
		}
		for _, d := range t.Decisions {
			rt.Decisions = append(rt.Decisions, workflow.TaskDecision{UserID: d.UserID, Choice: d.Choice,
				DelegatedFrom: d.DelegatedFrom, DecidedAt: d.DecidedAt})
		}
		out.Tasks = append(out.Tasks, rt)
	}
	return out
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

func (m managedHumanOnly) Task(ctx context.Context, actor spi.Subject, taskID uuid.UUID) (string, bool, bool, error) {
	return m.svc.HumanOnlyTask(ctx, actor, taskID)
}
