package engine

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// attentionSQL is true for a run row r (with instance i) that needs a person: it failed, it is blocked
// (its instance, a call of it, or at the start) or its trigger's filter failed.
const attentionSQL = `(` + statusSQL + ` IN ('failed', 'blocked', 'filter_failed'))`

// MaxOverviews bounds the graphs of one Overviews call.
const MaxOverviews = 100

func (s *service) Overviews(ctx context.Context, a Actor, ids []uuid.UUID) (map[uuid.UUID]Overview, error) {
	if err := s.authorize(a, PermRead); err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]Overview, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	if len(ids) > MaxOverviews {
		ids = ids[:MaxOverviews]
	}
	// One snapshot: the versions, the last runs and the counts describe the same committed state.
	tx, err := s.r.cfg.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, serrors.FromDB(opService, err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	type latest struct {
		ov    Overview
		graph []byte
	}
	got := map[uuid.UUID]*latest{}
	rows, err := tx.Query(ctx, `
		SELECT am.id, v.saved_by, v.saved_origin, v.created_at, v.code_digest, v.graph, coalesce(ev.code_digest, '')
		FROM automation.automations am
		JOIN automation.versions v ON v.tenant_id = am.tenant_id AND v.automation_id = am.id AND v.version = am.latest_version
		LEFT JOIN automation.versions ev ON ev.tenant_id = am.tenant_id AND ev.automation_id = am.id AND ev.version = am.enabled_version
		WHERE am.tenant_id = $1 AND am.id = ANY($2) AND am.deleted_at IS NULL`, a.TenantID, ids)
	if err != nil {
		return nil, serrors.FromDB(opService, err)
	}
	for rows.Next() {
		var l latest
		var origin string
		if err := rows.Scan(&l.ov.AutomationID, &l.ov.LatestSavedBy, &origin, &l.ov.LatestSavedAt, &l.ov.LatestCodeDigest, &l.graph, &l.ov.EnabledCodeDigest); err != nil {
			rows.Close()
			return nil, serrors.FromDB(opService, err)
		}
		l.ov.LatestSavedOrigin = Origin(origin)
		l.ov.Triggers = []TriggerReview{}
		got[l.ov.AutomationID] = &l
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, serrors.FromDB(opService, err)
	}
	if len(got) == 0 {
		return out, nil
	}
	found := make([]uuid.UUID, 0, len(got))
	for id := range got {
		found = append(found, id)
	}

	// The last run of each graph and the count of runs that need attention.
	rows, err = tx.Query(ctx, `
		SELECT DISTINCT ON (r.automation_id) r.automation_id, r.id, r.version, r.trigger_kind, r.trigger_id, `+statusSQL+`, r.created_at,
			greatest(r.updated_at, coalesce(i.updated_at, r.updated_at))`+runFrom+`
		WHERE r.tenant_id = $1 AND r.automation_id = ANY($2)
		ORDER BY r.automation_id, r.created_at DESC, r.id DESC`, a.TenantID, found)
	if err != nil {
		return nil, serrors.FromDB(opService, err)
	}
	for rows.Next() {
		var run RunSummary
		var status string
		if err := rows.Scan(&run.AutomationID, &run.RunID, &run.Version, &run.TriggerKind, &run.TriggerID, &status, &run.CreatedAt, &run.UpdatedAt); err != nil {
			rows.Close()
			return nil, serrors.FromDB(opService, err)
		}
		run.Status = RunStatus(status)
		run.CurrentNodes = []string{}
		if l := got[run.AutomationID]; l != nil {
			l.ov.LastRun = &run
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, serrors.FromDB(opService, err)
	}
	rows, err = tx.Query(ctx, `SELECT r.automation_id, count(*)`+runFrom+`
		WHERE r.tenant_id = $1 AND r.automation_id = ANY($2) AND `+attentionSQL+` GROUP BY r.automation_id`, a.TenantID, found)
	if err != nil {
		return nil, serrors.FromDB(opService, err)
	}
	for rows.Next() {
		var id uuid.UUID
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			rows.Close()
			return nil, serrors.FromDB(opService, err)
		}
		if l := got[id]; l != nil {
			l.ov.Attention = n
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, serrors.FromDB(opService, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, serrors.FromDB(opService, err)
	}

	// The triggers are classified by the tenant's catalog of now (read after the rows are released: it
	// may query the database).
	view, err := s.r.cfg.Catalog.ForTenant(ctx, a.TenantID)
	if err != nil {
		return nil, err
	}
	for id, l := range got {
		if g, errs := graph.Decode(l.graph); len(errs) == 0 {
			l.ov.LatestDescription = g.Description
			l.ov.Triggers = append(l.ov.Triggers, triggerReviews(view, g)...)
		}
		out[id] = l.ov
	}
	return out, nil
}

// LiveCount is what disabling or deleting a graph cancels: its live runs (queued or with a live
// instance) and their open waits.
type LiveCount struct {
	Runs  int
	Waits int
}

func (s *service) CountLive(ctx context.Context, a Actor, id uuid.UUID) (LiveCount, error) {
	// Whoever may disable or delete the graph asks what that cancels, with or without Automations.Read.
	if err := s.authorize(a); err != nil {
		return LiveCount{}, err
	}
	if !a.Subject.Has(PermRead) && !a.Subject.Has(PermEnable) && !a.Subject.Has(PermAuthor) {
		return LiveCount{}, ErrForbidden
	}
	if _, err := loadAutomation(ctx, s.r.cfg.DB, a.TenantID, id, false); err != nil {
		return LiveCount{}, err
	}
	var out LiveCount
	err := s.r.cfg.DB.QueryRow(ctx, `
		SELECT count(*), coalesce(sum((SELECT count(*) FROM automation.waits w WHERE w.tenant_id = r.tenant_id AND w.run_id = r.id AND w.status = 'open')), 0)`+runFrom+`
		WHERE r.tenant_id = $1 AND r.automation_id = $2 AND `+liveSQL, a.TenantID, id).Scan(&out.Runs, &out.Waits)
	if err != nil {
		return LiveCount{}, serrors.FromDB(opService, err)
	}
	return out, nil
}
