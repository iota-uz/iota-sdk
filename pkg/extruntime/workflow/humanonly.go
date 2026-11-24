package workflow

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// HumanOnlyWorkflow reports whether a workflow contract is a human-only native one
// (NativeContract.HumanOnly): management tools show its instances without their payloads.
func (e *Engine) HumanOnlyWorkflow(workflow string) (bool, error) {
	natives, err := e.allNatives()
	if err != nil {
		return false, serrors.Wrap("workflow.Engine.HumanOnlyWorkflow", err)
	}
	c, ok := natives[workflow]
	return ok && c.HumanOnly != nil, nil
}

// HumanOnlyInstance reports whether an instance of the tenant belongs to a human-only native contract
// and the page where a person acts on it. An unknown instance is not. The caller checks the right to
// act on instances first: the page is only named to whom may act.
func (e *Engine) HumanOnlyInstance(ctx context.Context, tenant, id uuid.UUID) (string, bool, error) {
	const op serrors.Op = "workflow.Engine.HumanOnlyInstance"
	inst, err := e.instance(ctx, e.cfg.DB, tenant, id)
	if errors.Is(err, ErrNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, serrors.Wrap(op, err)
	}
	natives, err := e.allNatives()
	if err != nil {
		return "", false, serrors.Wrap(op, err)
	}
	c, ok := natives[inst.Workflow]
	if !ok || c.HumanOnly == nil {
		return "", false, nil
	}
	url, err := c.HumanOnly(ctx, inst)
	if err != nil {
		return "", true, serrors.Wrap(op, err)
	}
	return url, true, nil
}

// HumanOnlyTask reports whether a human task belongs to a human-only instance, whatever the actor's
// rights: a management tool never passes such a task on to DecideTask, so no later change of rights
// can let it through. mayDecide tells whether the actor may decide the task now (assignee, holder of
// its permission, delegate); url, the page where a person decides it, is named only then. An actor
// whose rights cannot be read may not decide.
func (e *Engine) HumanOnlyTask(ctx context.Context, actor spi.Subject, taskID uuid.UUID) (url string, guarded, mayDecide bool, err error) {
	const op serrors.Op = "workflow.Engine.HumanOnlyTask"
	t, err := e.loadTask(ctx, e.cfg.DB, actor.TenantID, taskID, false)
	if errors.Is(err, ErrNotFound) {
		return "", false, false, nil
	}
	if err != nil {
		return "", false, false, serrors.Wrap(op, err)
	}
	page, guarded, err := e.HumanOnlyInstance(ctx, actor.TenantID, t.InstanceID)
	if err != nil || !guarded {
		return "", guarded, false, err
	}
	current, err := e.currentSubject(ctx, actor)
	if err != nil {
		return "", true, false, nil //nolint:nilerr // rights that cannot be read decide nothing
	}
	may, _, err := e.rightToDecide(ctx, current, t)
	if err != nil {
		return "", true, false, serrors.Wrap(op, err)
	}
	if !may {
		return "", true, false, nil
	}
	return page, true, true, nil
}
