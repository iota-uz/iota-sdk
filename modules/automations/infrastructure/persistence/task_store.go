package persistence

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/automations/tasks"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const opTaskStore = serrors.Op("automations.TaskStore")

// TaskStore reads the automation side of human tasks: the run, wait and step that opened a task and the
// graph version the run is pinned to, and the decisions a user made.
type TaskStore struct{ db *pgxpool.Pool }

var _ tasks.Store = (*TaskStore)(nil)

// NewTaskStore is the store over the automation and extruntime tables.
func NewTaskStore(db *pgxpool.Pool) *TaskStore { return &TaskStore{db: db} }

// Metas implements tasks.Store.
func (s *TaskStore) Metas(ctx context.Context, tenant uuid.UUID, keys []tasks.TaskKey) (map[uuid.UUID]tasks.TaskMeta, error) {
	out := map[uuid.UUID]tasks.TaskMeta{}
	if len(keys) == 0 {
		return out, nil
	}
	ids := make([]uuid.UUID, len(keys))
	instances := make([]uuid.UUID, len(keys))
	waits := make([]string, len(keys))
	for i, k := range keys {
		ids[i], instances[i], waits[i] = k.TaskID, k.InstanceID, k.WaitID
	}
	rows, err := s.db.Query(ctx, `
		SELECT k.task_id, r.id, r.automation_id, w.node_id, s.input,
			(SELECT n->'params' FROM jsonb_array_elements(v.graph->'nodes') n WHERE n->>'id' = w.node_id LIMIT 1)
		FROM unnest($2::uuid[], $3::uuid[], $4::text[]) AS k(task_id, instance_id, wait_id)
		JOIN automation.runs r ON r.tenant_id = $1 AND r.instance_id = k.instance_id
		JOIN automation.waits w ON w.tenant_id = r.tenant_id AND w.run_id = r.id AND w.wait_id = k.wait_id
		LEFT JOIN automation.steps s ON s.tenant_id = w.tenant_id AND s.run_id = w.run_id AND s.token_id = w.token_id
			AND s.seq = (w.spec->>'seq')::int
		LEFT JOIN automation.versions v ON v.tenant_id = r.tenant_id AND v.automation_id = r.automation_id AND v.version = r.version`,
		tenant, ids, instances, waits)
	if err != nil {
		return nil, serrors.FromDB(opTaskStore, err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id uuid.UUID
			m  tasks.TaskMeta
		)
		if err := rows.Scan(&id, &m.RunID, &m.AutomationID, &m.NodeID, &m.Input, &m.Params); err != nil {
			return nil, serrors.FromDB(opTaskStore, err)
		}
		out[id] = m
	}
	return out, serrors.FromDB(opTaskStore, rows.Err())
}

// Decided implements tasks.Store.
func (s *TaskStore) Decided(ctx context.Context, tenant uuid.UUID, userID int64, workflow string, after *tasks.DecidedPosition, limit int) ([]tasks.DecidedPosition, error) {
	var (
		afterAt time.Time
		afterID uuid.UUID
	)
	if after != nil {
		afterAt, afterID = after.At, after.TaskID
	}
	rows, err := s.db.Query(ctx, `
		SELECT d.task_id, d.decided_at FROM extruntime.human_task_decisions d
		JOIN extruntime.human_tasks t ON t.id = d.task_id AND t.tenant_id = d.tenant_id
		JOIN extruntime.workflow_instances i ON i.id = t.instance_id AND i.tenant_id = t.tenant_id
		WHERE d.tenant_id = $1 AND d.user_id = $2 AND i.workflow = $3
		  AND (NOT $4 OR (d.decided_at, d.task_id) < ($5::timestamptz, $6::uuid))
		ORDER BY d.decided_at DESC, d.task_id DESC
		LIMIT $7`, tenant, userID, workflow, after != nil, afterAt, afterID, limit)
	if err != nil {
		return nil, serrors.FromDB(opTaskStore, err)
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (tasks.DecidedPosition, error) {
		var p tasks.DecidedPosition
		err := r.Scan(&p.TaskID, &p.At)
		return p, err
	})
	return list, serrors.FromDB(opTaskStore, err)
}
