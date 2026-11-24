package mcpops

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Querier reads the workflow and automation tables (a *pgxpool.Pool).
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Guard tells the extension runtime's workflow tools which instances belong to automations: the
// instances of automation.run and automation.call and their human tasks. It implements
// mcptools.HumanOnly; the URL is the page of the run the instance serves.
type Guard struct {
	db   Querier
	urls URLs
}

// NativeGuard is the guard of automation instances for mcptools.Deps.HumanOnly.
func NativeGuard(db Querier, urls URLs) *Guard { return &Guard{db: db, urls: urls} }

// maxCallDepth bounds the walk from a call instance up to its run.
const maxCallDepth = 16

// Instance reports whether the instance is an automation instance and the page of its run.
func (g *Guard) Instance(ctx context.Context, tenant, id uuid.UUID) (string, bool, error) {
	const op serrors.Op = "mcpops.Guard.Instance"
	var workflow string
	err := g.db.QueryRow(ctx, `SELECT workflow FROM extruntime.workflow_instances WHERE tenant_id = $1 AND id = $2`, tenant, id).Scan(&workflow)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, serrors.Wrap(op, err)
	}
	if workflow != engine.WorkflowID && workflow != engine.CallWorkflowID {
		return "", false, nil
	}
	// The run whose instance is this one or an ancestor of it (a call's parent is its run).
	var automationID, runID uuid.UUID
	err = g.db.QueryRow(ctx, `
		WITH RECURSIVE up (id, parent, depth) AS (
			SELECT id, parent_instance_id, 0 FROM extruntime.workflow_instances WHERE tenant_id = $1 AND id = $2
			UNION ALL
			SELECT i.id, i.parent_instance_id, up.depth + 1 FROM extruntime.workflow_instances i
			JOIN up ON i.id = up.parent AND i.tenant_id = $1 WHERE up.depth < $3
		)
		SELECT r.automation_id, r.id FROM up JOIN automation.runs r ON r.tenant_id = $1 AND r.instance_id = up.id
		ORDER BY up.depth LIMIT 1`, tenant, id, maxCallDepth).Scan(&automationID, &runID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", true, nil
	}
	if err != nil {
		return "", false, serrors.Wrap(op, err)
	}
	return g.urls.Run(automationID, runID), true, nil
}

// Task reports whether the human task belongs to an automation instance and the page of its run.
func (g *Guard) Task(ctx context.Context, tenant, taskID uuid.UUID) (string, bool, error) {
	var instance uuid.UUID
	err := g.db.QueryRow(ctx, `SELECT instance_id FROM extruntime.human_tasks WHERE tenant_id = $1 AND id = $2`, tenant, taskID).Scan(&instance)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, serrors.Wrap(serrors.Op("mcpops.Guard.Task"), err)
	}
	return g.Instance(ctx, tenant, instance)
}
