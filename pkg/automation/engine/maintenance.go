package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const opMaintain = serrors.Op("automation.engine.maintain")

// finishedSQL is true for a run row r (with instance i) that ended.
const finishedSQL = `(i.status IN ('completed', 'failed', 'terminated') OR (r.instance_id IS NULL AND r.start_state IN ('filter_failed', 'blocked', 'cancelled')))`

// Retention batch sizes: each batch is its own statement (and transaction), so a backlog after a long
// pause neither holds locks long nor writes one huge WAL record.
const (
	pruneStepsBatch = 1000
	pruneRunsBatch  = 100
)

// finishedAtSQL is when a finished run r (with instance i) ended: its instance's finish, or the last
// change of a run that never had an instance.
const finishedAtSQL = `coalesce(i.finished_at, r.updated_at)`

// Prune removes, in bounded batches, the steps of runs that finished more than Retention.Steps ago and
// the runs that finished more than Retention.Runs ago. Live runs are never pruned.
func (r *Runtime) Prune(ctx context.Context) (int64, error) {
	var total int64
	// Steps go at the earlier of the two cutoffs: a run is deleted only after its steps are, so its
	// deletion never cascades to a large batch of steps (Retention.Steps longer than Retention.Runs).
	stepsAfter := r.cfg.Retention.Steps
	if r.cfg.Retention.Runs < stepsAfter {
		stepsAfter = r.cfg.Retention.Runs
	}
	// One clock for both loops: each statement's now() is its own, so a run that ended between the
	// two would be too new for the steps and old enough for the runs.
	var now time.Time
	if err := r.cfg.DB.QueryRow(ctx, `SELECT now()`).Scan(&now); err != nil {
		return 0, serrors.FromDB(opMaintain, err)
	}
	for {
		tag, err := r.cfg.DB.Exec(ctx, `
			DELETE FROM automation.steps WHERE (tenant_id, run_id, token_id, seq) IN (
				SELECT s.tenant_id, s.run_id, s.token_id, s.seq FROM automation.steps s
				JOIN automation.runs r ON r.tenant_id = s.tenant_id AND r.id = s.run_id
				LEFT JOIN extruntime.workflow_instances i ON i.id = r.instance_id
				WHERE `+finishedSQL+` AND `+finishedAtSQL+` < $3::timestamptz - $1::interval
				ORDER BY s.tenant_id, s.run_id, s.token_id, s.seq LIMIT $2)`,
			pgInterval(stepsAfter.Seconds()), pruneStepsBatch, now)
		if err != nil {
			return total, serrors.FromDB(opMaintain, err)
		}
		total += tag.RowsAffected()
		if tag.RowsAffected() < pruneStepsBatch || ctx.Err() != nil {
			break
		}
	}
	if h := r.testHook; h != nil {
		if err := h(ctx, nil, "prune-runs"); err != nil {
			return total, err
		}
	}
	for {
		tag, err := r.cfg.DB.Exec(ctx, `
			DELETE FROM automation.runs WHERE (tenant_id, id) IN (
				SELECT r.tenant_id, r.id FROM automation.runs r
				LEFT JOIN extruntime.workflow_instances i ON i.id = r.instance_id
				WHERE `+finishedSQL+` AND `+finishedAtSQL+` < $3::timestamptz - $1::interval
				  -- A run whose steps are still kept (they were not old enough, or the step loop
				  -- stopped early) is not deleted: its delete would cascade to them unbounded.
				  AND NOT EXISTS (SELECT 1 FROM automation.steps s WHERE s.tenant_id = r.tenant_id AND s.run_id = r.id)
				ORDER BY r.tenant_id, r.id LIMIT $2)`,
			pgInterval(r.cfg.Retention.Runs.Seconds()), pruneRunsBatch, now)
		if err != nil {
			return total, serrors.FromDB(opMaintain, err)
		}
		total += tag.RowsAffected()
		if tag.RowsAffected() < pruneRunsBatch || ctx.Err() != nil {
			break
		}
	}
	// Inbound dedup keys live as long as steps; expired correlations and old cached decisions go.
	for _, q := range []struct {
		sql string
		arg any
	}{
		{`DELETE FROM automation.inbound_actions WHERE ctid IN (SELECT ctid FROM automation.inbound_actions WHERE updated_at < now() - $1::interval LIMIT 1000)`, pgInterval(r.cfg.Retention.Steps.Seconds())},
		{`DELETE FROM automation.inbound_dedup WHERE ctid IN (SELECT ctid FROM automation.inbound_dedup WHERE seen_at < now() - $1::interval LIMIT 1000)`, pgInterval(r.cfg.Retention.Steps.Seconds())},
		{`DELETE FROM automation.correlations WHERE ctid IN (SELECT ctid FROM automation.correlations WHERE expires_at < now() - $1::interval LIMIT 1000)`, pgInterval(0)},
		{`DELETE FROM automation.decision_cache WHERE ctid IN (SELECT ctid FROM automation.decision_cache WHERE created_at < now() - $1::interval LIMIT 1000)`, pgInterval(DecisionTTL.Seconds())},
	} {
		tag, err := r.cfg.DB.Exec(ctx, q.sql, q.arg)
		if err != nil {
			return total, serrors.FromDB(opMaintain, err)
		}
		total += tag.RowsAffected()
	}
	return total, ctx.Err()
}

// releasePending finishes what closed waits still hold (release_pending) when their run is not driven
// again: a child instance is terminated here; a live run is woken so its runner cancels its own timer
// and task occurrences and clears the mark; a blocked run keeps the mark for its runner after Resume; a
// finished run (or one without an instance) has no occurrence left (CancelPending), so its mark is cleared.
func (r *Runtime) releasePending(ctx context.Context) error {
	ws, err := queryWaitsWithRun(ctx, r.cfg.DB)
	if err != nil {
		return err
	}
	var errs []error
	for _, w := range ws {
		if c := w.child(); c != nil && w.Status != waitFired {
			if err := r.cfg.Workflows.TerminateTree(ctx, w.tenant, *c); err != nil && !errors.Is(err, workflow.ErrNotAccepting) && !errors.Is(err, workflow.ErrNotFound) {
				errs = append(errs, err)
				continue
			}
		}
		if w.live && w.inst != nil {
			// The run's next pass releases the rest and clears the mark.
			err := r.cfg.Workflows.Signal(ctx, w.tenant, 0, *w.inst, InboxSignal, json.RawMessage(`{}`), fmt.Sprintf("release:%s:%d", w.RunID, w.txSeq))
			if err != nil && !errors.Is(err, workflow.ErrNotAccepting) && !errors.Is(err, workflow.ErrNotFound) {
				errs = append(errs, err)
			}
			continue
		}
		if !w.finished {
			continue // blocked: only its own runner may cancel its occurrences
		}
		if _, err := r.cfg.DB.Exec(ctx, `UPDATE automation.waits SET release_pending = false WHERE tenant_id = $1 AND run_id = $2 AND wait_id = $3`,
			w.tenant, w.RunID, w.WaitID); err != nil {
			errs = append(errs, serrors.FromDB(opMaintain, err))
		}
	}
	return errors.Join(errs...)
}

type pendingWait struct {
	waitRow
	tenant   uuid.UUID
	inst     *uuid.UUID
	live     bool
	finished bool // the instance ended, or the run never had one
	txSeq    int64
}

func queryWaitsWithRun(ctx context.Context, q querier) ([]pendingWait, error) {
	rows, err := q.Query(ctx, `
		SELECT w.run_id, w.token_id, w.wait_id, w.kind, w.node_id, w.subject_entity, w.subject_key, w.event_kind, w.filter_cel,
			w.timeout_at, w.status, w.after_seq, w.release_pending, w.spec, w.tenant_id, r.instance_id,
			coalesce(i.status IN ('running', 'waiting'), false), coalesce(i.status IN ('completed', 'failed', 'terminated'), true), r.tx_seq
		FROM automation.waits w
		JOIN automation.runs r ON r.tenant_id = w.tenant_id AND r.id = w.run_id
		LEFT JOIN extruntime.workflow_instances i ON i.id = r.instance_id
		WHERE w.release_pending AND w.updated_at < now() - interval '1 minute'
		ORDER BY w.updated_at LIMIT 200`)
	if err != nil {
		return nil, serrors.FromDB(opMaintain, err)
	}
	defer rows.Close()
	var out []pendingWait
	for rows.Next() {
		var p pendingWait
		var entity, key *string
		var spec []byte
		if err := rows.Scan(&p.RunID, &p.TokenID, &p.WaitID, &p.Kind, &p.NodeID, &entity, &key, &p.EventKind, &p.Filter, &p.TimeoutAt,
			&p.Status, &p.AfterSeq, &p.Release, &spec, &p.tenant, &p.inst, &p.live, &p.finished, &p.txSeq); err != nil {
			return nil, serrors.FromDB(opMaintain, err)
		}
		if len(spec) > 0 {
			if err := json.Unmarshal(spec, &p.Spec); err != nil { //nolint:musttag // workflow.TaskSpec keeps Go's field names
				return nil, err
			}
		}
		out = append(out, p)
	}
	return out, serrors.FromDB(opMaintain, rows.Err())
}

func pgInterval(seconds float64) string { return fmt.Sprintf("%f seconds", seconds) }

// cancelOrphans finishes the cancellations a Disable or Delete recorded (cancel_requested) when its
// process stopped before it terminated every run.
func (r *Runtime) cancelOrphans(ctx context.Context) error {
	rows, err := r.cfg.DB.Query(ctx, `
		SELECT r.tenant_id, r.id, r.instance_id`+runFrom+`
		WHERE r.cancel_requested AND `+liveSQL+` LIMIT 200`)
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

var _ connectors.TxUsageIndex = usageIndex{}

func (u usageIndex) Uses(ctx context.Context, tenantID, connectionID uuid.UUID) ([]connectors.Use, error) {
	view, err := u.r.cfg.Catalog.ForTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return usesIn(ctx, u.r.cfg.DB, view, tenantID, connectionID)
}

// PrepareUses reads the tenant's catalog now (it may need a pool connection of its own); the check it
// returns reads the uses with the caller's transaction only. The tenant's connections are read before
// the catalog and again by the check, under the caller's row lock: when any of them changed in between
// (a status, new credentials, one added or removed), the view may resolve a graph to another connection
// than the catalog does now, and the check returns connectors.ErrUsageChanged for the caller to
// prepare again. Unchanged rows mean the view was read over the state the lock now protects.
func (u usageIndex) PrepareUses(ctx context.Context, tenantID uuid.UUID) (connectors.UsesCheck, error) {
	before, err := connectionsState(ctx, u.r.cfg.DB, tenantID)
	if err != nil {
		return nil, err
	}
	view, err := u.r.cfg.Catalog.ForTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, tx pgx.Tx, connectionID uuid.UUID) ([]connectors.Use, error) {
		now, err := connectionsState(ctx, tx, tenantID)
		if err != nil {
			return nil, err
		}
		if now != before {
			return nil, connectors.ErrUsageChanged
		}
		return usesIn(ctx, tx, view, tenantID, connectionID)
	}, nil
}

// connectionsState is what the catalog's working connections of a tenant are derived from: the
// identity, status and version of every connection, and every grant of an opt_in platform credential
// (its kind, whether it is enabled and when it was last set).
func connectionsState(ctx context.Context, q querier, tenantID uuid.UUID) (string, error) {
	var state string
	err := q.QueryRow(ctx, `SELECT
		coalesce((SELECT string_agg(id::text || ':' || status || ':' || version, ',' ORDER BY id) FROM automation.connections WHERE tenant_id = $1), '')
		|| '|' ||
		coalesce((SELECT string_agg(kind || ':' || enabled || ':' || enabled_at, ',' ORDER BY kind) FROM automation.platform_grants WHERE tenant_id = $1), '')`,
		tenantID).Scan(&state)
	return state, serrors.FromDB(opMaintain, err)
}

// usesIn lists the uses of a connection in q, resolving connections a node or trigger leaves out with
// view.
func usesIn(ctx context.Context, q querier, view catalog.View, tenantID, connectionID uuid.UUID) ([]connectors.Use, error) {
	rows, err := q.Query(ctx, `
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
		seen := mayUse(view, g, want)
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

// mayUse are the nodes and triggers of g that use connection or may come to: one that names it, and
// one that names no connection while connection is among the working connections of its kinds. The
// latter resolves to it as soon as the others of its kinds leave (a reauthorization, a disconnect),
// which a delete holding only this connection's lock cannot prevent, so it blocks the delete even while
// the resolution is ambiguous or picks another connection now.
func mayUse(view catalog.View, g graph.Graph, connection string) map[string]bool {
	out := map[string]bool{}
	use := func(id string, kinds []catalog.ConnectorKind, param any) {
		if c, _ := param.(string); c != "" {
			if c == connection {
				out[id] = true
			}
			return
		}
		if param != nil {
			return
		}
		for _, k := range kinds {
			for _, ref := range view.Connections[k] {
				if ref.ID == connection {
					out[id] = true
				}
			}
		}
	}
	for _, n := range g.Nodes {
		use(n.ID, view.NodeRequires[n.Type], n.Params[catalog.ConnectionParam])
	}
	for _, t := range g.Triggers {
		use(t.ID, view.EventRequires[t.Event], t.Params[catalog.ConnectionParam])
	}
	return out
}

// connectionUses maps each node and trigger of g that uses a connection to its ID. One that names no
// connection uses the one the catalog resolves for its kinds, as validation and execution do.
func connectionUses(view catalog.View, g graph.Graph) map[string]string {
	out := map[string]string{}
	use := func(id string, kinds []catalog.ConnectorKind, param any) {
		if c, _ := param.(string); c != "" {
			out[id] = c
			return
		}
		if param != nil || len(kinds) == 0 {
			return
		}
		if ref, _, err := catalog.ResolveConnection(view.Connections, kinds, nil); err == nil {
			out[id] = ref.ID
		}
	}
	for _, n := range g.Nodes {
		use(n.ID, view.NodeRequires[n.Type], n.Params[catalog.ConnectionParam])
	}
	for _, t := range g.Triggers {
		use(t.ID, view.EventRequires[t.Event], t.Params[catalog.ConnectionParam])
	}
	return out
}
