package engine

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// maxCallDepth bounds the walk from a call instance up to the run it serves.
const maxCallDepth = 16

// humanPage is workflow.NativeContract.HumanOnly of automation.run and automation.call: a person acts on
// a run (and on its calls and tasks) on the run's page (Links.RunURL), never an agent through the
// workflow tools. "" when the run is not found or no Links are configured.
func (r *Runtime) humanPage(ctx context.Context, inst workflow.Instance) (string, error) {
	var automationID, runID uuid.UUID
	err := r.cfg.DB.QueryRow(ctx, `
		WITH RECURSIVE up (id, parent, depth) AS (
			SELECT id, parent_instance_id, 0 FROM extruntime.workflow_instances WHERE tenant_id = $1 AND id = $2
			UNION ALL
			SELECT i.id, i.parent_instance_id, up.depth + 1 FROM extruntime.workflow_instances i
			JOIN up ON i.id = up.parent AND i.tenant_id = $1 WHERE up.depth < $3
		)
		SELECT r.automation_id, r.id FROM up JOIN automation.runs r ON r.tenant_id = $1 AND r.instance_id = up.id
		ORDER BY up.depth LIMIT 1`, inst.TenantID, inst.ID, maxCallDepth).Scan(&automationID, &runID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", serrors.FromDB(opService, err)
	}
	if r.cfg.Links == nil {
		return "", nil
	}
	return r.cfg.Links.RunURL(inst.TenantID, automationID, runID), nil
}
