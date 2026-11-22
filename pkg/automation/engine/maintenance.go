package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const opMaintain = serrors.Op("automation.engine.maintain")

// finishedSQL is true for a run row r (with instance i) that ended.
const finishedSQL = `(i.status IN ('completed', 'failed', 'terminated') OR (r.instance_id IS NULL AND r.start_state IN ('filter_failed', 'blocked', 'cancelled')))`

// Prune removes the steps of finished runs older than Retention.Steps and finished runs older than
// Retention.Runs. Live runs are never pruned.
func (r *Runtime) Prune(ctx context.Context) (int64, error) {
	tag, err := r.cfg.DB.Exec(ctx, `
		DELETE FROM automation.steps s USING automation.runs r LEFT JOIN extruntime.workflow_instances i ON i.id = r.instance_id
		WHERE s.tenant_id = r.tenant_id AND s.run_id = r.id AND s.started_at < now() - $1::interval AND `+finishedSQL,
		pgInterval(r.cfg.Retention.Steps.Seconds()))
	if err != nil {
		return 0, serrors.FromDB(opMaintain, err)
	}
	n := tag.RowsAffected()
	tag, err = r.cfg.DB.Exec(ctx, `
		DELETE FROM automation.runs x USING automation.runs r LEFT JOIN extruntime.workflow_instances i ON i.id = r.instance_id
		WHERE x.tenant_id = r.tenant_id AND x.id = r.id AND r.created_at < now() - $1::interval AND `+finishedSQL,
		pgInterval(r.cfg.Retention.Runs.Seconds()))
	if err != nil {
		return n, serrors.FromDB(opMaintain, err)
	}
	return n + tag.RowsAffected(), nil
}

func pgInterval(seconds float64) string { return fmt.Sprintf("%f seconds", seconds) }

// cancelOrphans cancels live runs of graphs that were deleted, or disabled after the run started: a
// Disable whose process stopped before it cancelled them all.
func (r *Runtime) cancelOrphans(ctx context.Context) error {
	rows, err := r.cfg.DB.Query(ctx, `
		SELECT r.tenant_id, r.id, r.instance_id`+runFrom+`
		JOIN automation.automations a ON a.tenant_id = r.tenant_id AND a.id = r.automation_id
		WHERE (a.deleted_at IS NOT NULL OR (a.enabled_version IS NULL AND a.disabled_at IS NOT NULL AND r.created_at <= a.disabled_at
			AND r.trigger_kind <> 'manual' AND r.restarted_from IS NULL))
		  AND `+liveSQL+` LIMIT 200`)
	if err != nil {
		return serrors.FromDB(opMaintain, err)
	}
	type live struct {
		tenant, id uuid.UUID
		inst       *uuid.UUID
	}
	var list []live
	for rows.Next() {
		var l live
		if err := rows.Scan(&l.tenant, &l.id, &l.inst); err != nil {
			rows.Close()
			return serrors.FromDB(opMaintain, err)
		}
		list = append(list, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return serrors.FromDB(opMaintain, err)
	}
	for _, l := range list {
		if err := r.cancelRun(ctx, l.tenant, l.id, l.inst); err != nil && !errors.Is(err, ErrRunFinished) {
			return err
		}
	}
	return nil
}

// UsageIndex is connectors.UsageIndex of #1330 over graph versions: the latest and enabled versions of
// graphs that are not deleted, and the versions of live runs, whose nodes or triggers name the connection.
func (r *Runtime) UsageIndex() connectors.UsageIndex { return usageIndex{r: r} }

type usageIndex struct{ r *Runtime }

func (u usageIndex) Uses(ctx context.Context, tenantID, connectionID uuid.UUID) ([]connectors.Use, error) {
	rows, err := u.r.cfg.DB.Query(ctx, `
		SELECT a.id, a.name, v.version, v.graph, coalesce(a.enabled_version = v.version, false)
		FROM automation.versions v JOIN automation.automations a ON a.tenant_id = v.tenant_id AND a.id = v.automation_id
		WHERE v.tenant_id = $1 AND (
			(a.deleted_at IS NULL AND (v.version = a.latest_version OR v.version = a.enabled_version))
			OR EXISTS (SELECT 1`+runFrom+` WHERE r.tenant_id = v.tenant_id AND r.automation_id = v.automation_id AND r.version = v.version AND `+liveSQL+`))
		ORDER BY a.name, a.id, v.version`, tenantID)
	if err != nil {
		return nil, serrors.FromDB(opMaintain, err)
	}
	defer rows.Close()
	want := connectionID.String()
	var out []connectors.Use
	for rows.Next() {
		var use connectors.Use
		var raw []byte
		if err := rows.Scan(&use.AutomationID, &use.Name, &use.Version, &raw, &use.Enabled); err != nil {
			return nil, serrors.FromDB(opMaintain, err)
		}
		var g graph.Graph
		if err := json.Unmarshal(raw, &g); err != nil {
			continue
		}
		seen := map[string]bool{}
		for _, n := range g.Nodes {
			if c, _ := n.Params[catalog.ConnectionParam].(string); c == want {
				seen[n.ID] = true
			}
		}
		for _, t := range g.Triggers {
			if c, _ := t.Params[catalog.ConnectionParam].(string); c == want {
				seen[t.ID] = true
			}
		}
		if len(seen) == 0 {
			continue
		}
		for id := range seen {
			use.NodeIDs = append(use.NodeIDs, id)
		}
		sort.Strings(use.NodeIDs)
		out = append(out, use)
	}
	return out, serrors.FromDB(opMaintain, rows.Err())
}
