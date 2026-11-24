package mcptools

import (
	"context"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manage"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// IssueHumanRequired is the issue code of a refusal a person must act on instead of the agent. It
// travels in the shared failure contract: approval_required with recovery open_approval, and the URL of
// the page where the person acts in the message.
const IssueHumanRequired = "HUMAN_REQUIRED"

// HumanOnly tells which workflow instances only a person may act on, such as the instances of the
// native contracts of automations. The default (Deps.HumanOnly nil) asks the workflow engine through
// manage.Service, which knows the contracts marked workflow.NativeContract.HumanOnly. Reads of them stay available;
// every action tool (signal, decide, terminate, start v2, resolve, retry, resume) returns
// HUMAN_REQUIRED with the page where the person acts.
type HumanOnly interface {
	// Instance reports whether instance id of the tenant is guarded and the URL of its page.
	Instance(ctx context.Context, tenant, id uuid.UUID) (url string, guarded bool, err error)
	// Task reports whether the human task is a task of a guarded instance and the URL of its page.
	Task(ctx context.Context, tenant, taskID uuid.UUID) (url string, guarded bool, err error)
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

// guardInstance refuses an action on a guarded instance before the service runs.
func guardInstance(ctx context.Context, g HumanOnly, actor spi.Subject, id uuid.UUID, action string) error {
	if g == nil {
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
	url, guarded, err := g.Task(ctx, actor.TenantID, id)
	if err != nil {
		return serrors.Wrap(serrors.Op("extruntime.mcptools.guardTask"), err)
	}
	if guarded {
		return HumanRequired("/task_id", "decide this task", url)
	}
	return nil
}

// managedHumanOnly is the default guard: the human-only native contracts of the workflow engine the
// management service runs on.
type managedHumanOnly struct{ svc *manage.Service }

func (m managedHumanOnly) Instance(ctx context.Context, tenant, id uuid.UUID) (string, bool, error) {
	return m.svc.HumanOnlyInstance(ctx, tenant, id)
}

func (m managedHumanOnly) Task(ctx context.Context, tenant, taskID uuid.UUID) (string, bool, error) {
	return m.svc.HumanOnlyTask(ctx, tenant, taskID)
}
