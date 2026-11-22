package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

func (s *service) ListRuns(ctx context.Context, a Actor, f RunFilter) (Page[RunSummary], error) {
	if err := s.authorize(a, PermRead); err != nil {
		return Page[RunSummary]{}, err
	}
	c, after, err := decodeCursor(f.Cursor)
	if err != nil {
		return Page[RunSummary]{}, err
	}
	limit := pageLimit(f.Limit)
	args := []any{a.TenantID}
	where := []string{"r.tenant_id = $1",
		"EXISTS (SELECT 1 FROM automation.automations am WHERE am.tenant_id = r.tenant_id AND am.id = r.automation_id AND am.deleted_at IS NULL)"}
	if f.AutomationID != nil {
		args = append(args, *f.AutomationID)
		where = append(where, fmt.Sprintf("r.automation_id = $%d", len(args)))
	}
	if len(f.Status) > 0 {
		st := make([]string, len(f.Status))
		for i, x := range f.Status {
			st[i] = string(x)
		}
		args = append(args, st)
		where = append(where, fmt.Sprintf("%s = ANY($%d)", statusSQL, len(args)))
	}
	if f.Subject != nil {
		sub := normSubject(*f.Subject)
		args = append(args, sub.Entity, sub.Key)
		where = append(where, fmt.Sprintf("r.subject_entity = $%d AND r.subject_key = $%d", len(args)-1, len(args)))
	}
	if f.Since != nil {
		args = append(args, *f.Since)
		where = append(where, fmt.Sprintf("r.created_at >= $%d", len(args)))
	}
	if f.Until != nil {
		args = append(args, *f.Until)
		where = append(where, fmt.Sprintf("r.created_at < $%d", len(args)))
	}
	if after {
		args = append(args, c.At, c.ID)
		where = append(where, fmt.Sprintf("(r.created_at, r.id) < ($%d, $%d)", len(args)-1, len(args)))
	}
	args = append(args, limit+1)
	rows, err := s.r.cfg.DB.Query(ctx, `SELECT `+runCols+`,
			(SELECT coalesce(array_agg(t.node_id ORDER BY t.created_seq), '{}') FROM automation.tokens t
			 WHERE t.tenant_id = r.tenant_id AND t.run_id = r.id AND t.status IN ('running', 'waiting') AND t.flow IS NULL)`+runFrom+`
		WHERE `+strings.Join(where, " AND ")+fmt.Sprintf(` ORDER BY r.created_at DESC, r.id DESC LIMIT $%d`, len(args)), args...)
	if err != nil {
		return Page[RunSummary]{}, serrors.FromDB(opService, err)
	}
	defer rows.Close()
	page := Page[RunSummary]{Items: []RunSummary{}}
	for rows.Next() {
		var r runRow
		var nodes []string
		var input, output, errj []byte
		if err := rows.Scan(&r.TenantID, &r.ID, &r.AutomationID, &r.Version, &r.InstanceID, &r.StartKey, &r.StartState, &r.StartReason,
			&r.TriggerID, &r.TriggerEventID, &r.TriggerKind, &input, &r.Subject.Entity, &r.Subject.Key, &r.RunAsUserID, &r.RunAsKind,
			&r.InitiatorID, &r.InboxSeq, &r.TxSeq, &r.StepsCount, &r.Depth, &r.CauseRunID, &r.RestartedFrom, &output, &errj,
			&r.CreatedAt, &r.UpdatedAt, &r.InstanceStatus, &r.BlockedReason, &r.BlockedDetail, &r.InstanceUpdated, &nodes); err != nil {
			return Page[RunSummary]{}, serrors.FromDB(opService, err)
		}
		r.Input, r.Output, r.Error = input, output, errj
		sum := summaryOf(r)
		sum.CurrentNodes = nodes
		page.Items = append(page.Items, sum)
	}
	if err := rows.Err(); err != nil {
		return Page[RunSummary]{}, serrors.FromDB(opService, err)
	}
	if len(page.Items) > limit {
		page.Items = page.Items[:limit]
		last := page.Items[limit-1]
		page.Next = encodeCursor(last.CreatedAt, last.RunID)
	}
	return page, nil
}

func summaryOf(r runRow) RunSummary {
	updated := r.UpdatedAt
	if r.InstanceUpdated != nil && r.InstanceUpdated.After(updated) {
		updated = *r.InstanceUpdated
	}
	return RunSummary{
		RunID: r.ID, AutomationID: r.AutomationID, Version: r.Version, Status: r.status(), Reason: r.reason(),
		TriggerKind: r.TriggerKind, TriggerID: r.TriggerID, Subject: normSubject(r.Subject), CurrentNodes: []string{},
		Error: r.stepError(), CreatedAt: r.CreatedAt, UpdatedAt: updated,
	}
}

// maxDetailSteps bounds the steps GetRun returns (the latest ones).
const maxDetailSteps = 1000

func (s *service) GetRun(ctx context.Context, a Actor, runID uuid.UUID) (RunDetail, error) {
	if err := s.authorize(a, PermRead); err != nil {
		return RunDetail{}, err
	}
	run, err := loadRun(ctx, s.r.cfg.DB, a.TenantID, runID)
	if err != nil {
		return RunDetail{}, err
	}
	if _, err := loadAutomation(ctx, s.r.cfg.DB, a.TenantID, run.AutomationID, false); err != nil {
		return RunDetail{}, err
	}
	d := RunDetail{RunSummary: summaryOf(run), InstanceID: run.InstanceID, Tokens: []TokenView{}, Steps: []StepView{}, Waits: []WaitView{}}
	toks, err := queryTokens(ctx, s.r.cfg.DB, `SELECT `+tokenCols+` FROM automation.tokens WHERE tenant_id = $1 AND run_id = $2 ORDER BY created_seq`, a.TenantID, run.ID)
	if err != nil {
		return RunDetail{}, err
	}
	for _, t := range toks {
		tv := TokenView{ID: t.ID, Parent: t.Parent, NodeID: t.NodeID, Status: t.Status, ForeachIndex: t.ForeachIndex}
		if t.Branch != nil {
			tv.Branch = *t.Branch
		}
		d.Tokens = append(d.Tokens, tv)
		if (t.Status == tokRunning || t.Status == tokWaiting) && t.Flow == nil {
			d.CurrentNodes = append(d.CurrentNodes, t.NodeID)
		}
	}
	waits, err := queryWaits(ctx, s.r.cfg.DB, `SELECT `+waitCols+` FROM automation.waits WHERE tenant_id = $1 AND run_id = $2 ORDER BY created_at, wait_id`, a.TenantID, run.ID)
	if err != nil {
		return RunDetail{}, err
	}
	eventOf := map[string]string{} // token|seq → the event a wait step woke on
	for _, w := range waits {
		wv := WaitView{TokenID: w.TokenID, WaitID: w.WaitID, Kind: w.Kind, TimeoutAt: w.TimeoutAt, Status: w.Status}
		if w.EventKind != nil {
			wv.EventKind = *w.EventKind
			eventOf[fmt.Sprintf("%s|%d", w.TokenID, w.Spec.Seq)] = *w.EventKind
		}
		d.Waits = append(d.Waits, wv)
	}
	rows, err := s.r.cfg.DB.Query(ctx, `
		SELECT token_id, seq, node_id, attempt, input, output, coalesce(outcome, ''), error, started_at, ended_at FROM (
			SELECT * FROM automation.steps WHERE tenant_id = $1 AND run_id = $2 ORDER BY started_at DESC, seq DESC LIMIT $3) x
		ORDER BY started_at, seq`, a.TenantID, run.ID, maxDetailSteps)
	if err != nil {
		return RunDetail{}, serrors.FromDB(opService, err)
	}
	defer rows.Close()
	proj := s.r.projector(a.TenantID)
	viewer := a.Subject
	viewer.TenantID = a.TenantID
	for rows.Next() {
		var sv StepView
		var in, out, errj []byte
		var ended *time.Time
		if err := rows.Scan(&sv.TokenID, &sv.Seq, &sv.NodeID, &sv.Attempt, &in, &out, &sv.Outcome, &errj, &sv.StartedAt, &ended); err != nil {
			return RunDetail{}, serrors.FromDB(opService, err)
		}
		sv.Input, sv.Output, sv.EndedAt = in, out, ended
		if len(errj) > 0 {
			var e StepError
			if json.Unmarshal(errj, &e) == nil {
				sv.Error = &e
			}
		}
		// The event a wait woke on is shown with the viewer's rights.
		if ev, ok := eventOf[fmt.Sprintf("%s|%d", sv.TokenID, sv.Seq)]; ok && len(out) > 0 {
			if m, err := decodeMap(out); err == nil {
				if p, ok := m["event"].(map[string]any); ok {
					if _, err := proj.project(ctx, ev, p, &viewer); err != nil {
						return RunDetail{}, err
					}
					if b, err := json.Marshal(m); err == nil {
						sv.Output = b
					}
				}
			}
		}
		d.Steps = append(d.Steps, sv)
	}
	if err := rows.Err(); err != nil {
		return RunDetail{}, serrors.FromDB(opService, err)
	}
	if d.CurrentNodes == nil {
		d.CurrentNodes = []string{}
	}
	return d, nil
}
