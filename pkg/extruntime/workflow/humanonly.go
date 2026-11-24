package workflow

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// HumanOnlyInstance reports whether an instance of the tenant belongs to a human-only native contract
// (NativeContract.HumanOnly) and the page where a person acts on it. An unknown instance is not.
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

// HumanOnlyTask is HumanOnlyInstance of the instance a human task belongs to.
func (e *Engine) HumanOnlyTask(ctx context.Context, tenant, taskID uuid.UUID) (string, bool, error) {
	var instance uuid.UUID
	err := e.cfg.DB.QueryRow(ctx, `SELECT instance_id FROM extruntime.human_tasks WHERE tenant_id = $1 AND id = $2`, tenant, taskID).Scan(&instance)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, serrors.FromDB(serrors.Op("workflow.Engine.HumanOnlyTask"), err)
	}
	return e.HumanOnlyInstance(ctx, tenant, instance)
}
