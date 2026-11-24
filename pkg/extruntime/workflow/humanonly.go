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

// HumanOnlyTask is HumanOnlyInstance of the instance a human task belongs to, for an actor who may
// decide the task now (its assignee, a holder of its permission, a delegate). For anyone else the task
// is not reported: deciding it is refused by DecideTask as for any task, and the page is not named.
func (e *Engine) HumanOnlyTask(ctx context.Context, actor spi.Subject, taskID uuid.UUID) (string, bool, error) {
	const op serrors.Op = "workflow.Engine.HumanOnlyTask"
	t, err := e.loadTask(ctx, e.cfg.DB, actor.TenantID, taskID, false)
	if errors.Is(err, ErrNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, serrors.Wrap(op, err)
	}
	current, err := e.currentSubject(ctx, actor)
	if err != nil {
		return "", false, nil //nolint:nilerr // an actor without current rights may not decide: DecideTask refuses
	}
	may, _, err := e.rightToDecide(ctx, current, t)
	if err != nil {
		return "", false, serrors.Wrap(op, err)
	}
	if !may {
		return "", false, nil
	}
	return e.HumanOnlyInstance(ctx, actor.TenantID, t.InstanceID)
}
