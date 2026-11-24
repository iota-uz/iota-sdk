package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
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
	if f.Version > 0 {
		args = append(args, f.Version)
		where = append(where, fmt.Sprintf("r.version = $%d", len(args)))
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
	if f.Node != "" {
		args = append(args, f.Node)
		where = append(where, fmt.Sprintf("EXISTS (SELECT 1 FROM automation.steps s WHERE s.tenant_id = r.tenant_id AND s.run_id = r.id AND s.node_id = $%d)", len(args)))
	}
	if f.SubjectKeyPrefix != "" {
		args = append(args, likePrefix(f.SubjectKeyPrefix))
		where = append(where, fmt.Sprintf(`r.subject_key ILIKE $%d ESCAPE '\'`, len(args)))
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
	// The page is read and its rows closed before anything is projected: the projector reads the
	// catalog, which may query the database (Catalog.ForTenant), and must not wait for the connection
	// these rows hold. Only the summary and the references are kept of a row, not its payloads.
	type listed struct {
		sum  RunSummary
		refs []sensitiveRef
	}
	var got []listed
	for rows.Next() {
		var r runRow
		var nodes []string
		var input, output, errj, sens, evs []byte
		if err := rows.Scan(&r.TenantID, &r.ID, &r.AutomationID, &r.Version, &r.InstanceID, &r.StartKey, &r.StartState, &r.StartReason,
			&r.TriggerID, &r.TriggerEventID, &r.TriggerKind, &input, &r.Subject.Entity, &r.Subject.Key, &r.RunAsUserID, &r.RunAsKind,
			&r.InitiatorID, &r.InboxSeq, &r.TxSeq, &r.StepsCount, &r.Depth, &r.CauseRunID, &r.RestartedFrom, &output, &errj,
			&sens, &r.CreatedAt, &r.UpdatedAt, &r.InstanceStatus, &r.BlockedReason, &r.BlockedDetail, &r.InstanceUpdated, &evs, &r.ChildBlocked, &nodes); err != nil {
			rows.Close()
			return Page[RunSummary]{}, serrors.FromDB(opService, err)
		}
		r.Input, r.Output, r.Error, r.Sensitive, r.Events = input, output, errj, decodeRefs(sens), evs
		sum := summaryOf(r)
		sum.CurrentNodes = nodes
		got = append(got, listed{sum: sum, refs: decodeRefs(sens)})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Page[RunSummary]{}, serrors.FromDB(opService, err)
	}
	page := Page[RunSummary]{Items: []RunSummary{}}
	proj := s.r.projector(a.TenantID)
	viewer := a.Subject
	viewer.TenantID = a.TenantID
	for _, l := range got {
		sum, err := redactSummary(ctx, proj, &viewer, l.sum, l.refs)
		if err != nil {
			return Page[RunSummary]{}, err
		}
		page.Items = append(page.Items, sum)
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

// redactSummary hides what a run's error and reason may carry of the sensitive fields the run read when
// the viewer may not read them all: a failure message, or the CEL message of a trigger filter that
// failed (filter_error: ...), can quote a protected value.
func redactSummary(ctx context.Context, proj *projector, viewer *spi.Subject, sum RunSummary, refs []sensitiveRef) (RunSummary, error) {
	if len(refs) == 0 || (sum.Error == nil && sum.Reason == "") {
		return sum, nil
	}
	ok, err := proj.readable(ctx, refs, viewer)
	if err != nil || ok {
		return sum, err
	}
	if sum.Error != nil {
		sum.Error = &StepError{Code: sum.Error.Code, Message: Hidden, Path: sum.Error.Path}
	}
	sum.Reason = redactReason(sum.Reason)
	return sum, nil
}

// redactReason keeps the code of a reason (no_owner, filter_error) and hides its message.
func redactReason(reason string) string {
	if code, _, ok := strings.Cut(reason, ": "); ok {
		return code + ": " + Hidden
	}
	return reason
}

// maxDetailSteps bounds the steps GetRun returns (the latest ones).
var maxDetailSteps = 1000

// maxStepsOfVisits bounds the visits of one node StepsOf looks through for one the viewer may read.
const maxStepsOfVisits = 200

func (s *service) GetRun(ctx context.Context, a Actor, runID uuid.UUID, opts ...RunDetailOption) (RunDetail, error) {
	if err := s.authorize(a, PermRead); err != nil {
		return RunDetail{}, err
	}
	var o runDetailOptions
	for _, opt := range opts {
		opt(&o)
	}
	if len(o.nodes) > MaxStepsOfNodes {
		return RunDetail{}, fmt.Errorf("%w: at most %d nodes", ErrInvalidInput, MaxStepsOfNodes)
	}
	if o.nodes == nil {
		o.nodes = []string{}
	}
	// One snapshot: the status, the tokens, the waits and the steps describe the same moment.
	tx, err := s.r.cfg.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return RunDetail{}, serrors.FromDB(opService, err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	run, err := loadRun(ctx, tx, a.TenantID, runID)
	if err != nil {
		return RunDetail{}, err
	}
	if _, err := loadAutomation(ctx, tx, a.TenantID, run.AutomationID, false); err != nil {
		return RunDetail{}, err
	}
	d := RunDetail{RunSummary: summaryOf(run), InstanceID: run.InstanceID, Tokens: []TokenView{}, Steps: []StepView{}, Waits: []WaitView{}}
	toks, err := queryTokens(ctx, tx, `SELECT `+tokenCols+` FROM automation.tokens WHERE tenant_id = $1 AND run_id = $2 ORDER BY created_seq`, a.TenantID, run.ID)
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
	waits, err := queryWaits(ctx, tx, `SELECT `+waitCols+` FROM automation.waits WHERE tenant_id = $1 AND run_id = $2 ORDER BY created_at, wait_id`, a.TenantID, run.ID)
	if err != nil {
		return RunDetail{}, err
	}
	for _, w := range waits {
		wv := WaitView{TokenID: w.TokenID, NodeID: w.NodeID, WaitID: w.WaitID, Kind: w.Kind, TimeoutAt: w.TimeoutAt, Status: w.Status}
		if w.EventKind != nil {
			wv.EventKind = *w.EventKind
		}
		d.Waits = append(d.Waits, wv)
	}
	type stepRow struct {
		sv   StepView
		sens []sensitiveRef
	}
	scan := func(sql string, args ...any) ([]stepRow, error) {
		rows, err := tx.Query(ctx, sql, args...)
		if err != nil {
			return nil, serrors.FromDB(opService, err)
		}
		defer rows.Close()
		var out []stepRow
		for rows.Next() {
			var st stepRow
			sv := &st.sv
			var in, outj, errj, sens, warns, logs []byte
			var ended *time.Time
			if err := rows.Scan(&sv.TokenID, &sv.Seq, &sv.NodeID, &sv.Attempt, &in, &outj, &sv.Outcome, &errj, &sens, &sv.StartedAt, &ended, &warns, &logs); err != nil {
				return nil, serrors.FromDB(opService, err)
			}
			if len(warns) > 0 {
				_ = json.Unmarshal(warns, &sv.Warnings)
			}
			if len(logs) > 0 {
				_ = json.Unmarshal(logs, &sv.Logs)
			}
			sv.Input, sv.Output, sv.EndedAt = in, outj, ended
			if len(errj) > 0 {
				var e StepError
				if json.Unmarshal(errj, &e) == nil {
					sv.Error = &e
				}
			}
			st.sens = decodeRefs(sens)
			out = append(out, st)
		}
		return out, serrors.FromDB(opService, rows.Err())
	}
	const stepCols = `token_id, seq, node_id, attempt, input, output, coalesce(outcome, ''), error, sensitive, started_at, ended_at, warnings, logs`
	steps, err := scan(`SELECT `+stepCols+` FROM (
			SELECT * FROM automation.steps WHERE tenant_id = $1 AND run_id = $2 ORDER BY started_at DESC, seq DESC LIMIT $3) x
		ORDER BY started_at, seq`, a.TenantID, run.ID, maxDetailSteps)
	if err != nil {
		return RunDetail{}, err
	}
	// The visits of each node StepsOf names, latest first wherever they are in the run: the latest one the
	// viewer may read is added below (a later visit the viewer may not read does not hide an earlier one).
	var visits []stepRow
	if len(o.nodes) > 0 {
		if visits, err = scan(`SELECT `+stepCols+` FROM (
				SELECT *, row_number() OVER (PARTITION BY node_id ORDER BY started_at DESC, seq DESC) AS visit
				FROM automation.steps WHERE tenant_id = $1 AND run_id = $2 AND node_id = ANY($3)) x
			WHERE visit <= $4 ORDER BY node_id, started_at DESC, seq DESC`, a.TenantID, run.ID, o.nodes, maxStepsOfVisits); err != nil {
			return RunDetail{}, err
		}
	}
	// The snapshot is complete: its connection goes back to the pool before anything is projected (the
	// projector reads the catalog, which may query the database).
	if err := tx.Rollback(ctx); err != nil {
		return RunDetail{}, serrors.FromDB(opService, err)
	}
	proj := s.r.projector(a.TenantID)
	viewer := a.Subject
	viewer.TenantID = a.TenantID
	if d.Event, err = proj.historicEvent(ctx, run.Input, run.Sensitive, &viewer); err != nil {
		return RunDetail{}, err
	}
	// A step that ran after the run read a sensitive field the viewer may not read can carry it in any
	// form (rendered, transformed by CEL or a node): its data is left out.
	project := func(st stepRow) (StepView, error) {
		sv := st.sv
		ok, err := proj.readable(ctx, st.sens, &viewer)
		if err != nil {
			return StepView{}, err
		}
		if !ok {
			sv.Hidden, sv.Input, sv.Output, sv.Logs = true, nil, nil, nil
			if sv.Error != nil {
				sv.Error = &StepError{Code: sv.Error.Code, Message: Hidden, Path: sv.Error.Path}
			}
		}
		return sv, nil
	}
	type stepKey struct {
		token uuid.UUID
		seq   int
	}
	have := map[stepKey]bool{}
	for _, st := range steps {
		sv, err := project(st)
		if err != nil {
			return RunDetail{}, err
		}
		have[stepKey{sv.TokenID, sv.Seq}] = true
		d.Steps = append(d.Steps, sv)
	}
	// Per named node: its latest visit the viewer may read, else its latest visit (hidden).
	chosen := map[string]StepView{}
	readable := map[string]bool{}
	for _, st := range visits {
		if readable[st.sv.NodeID] {
			continue
		}
		sv, err := project(st)
		if err != nil {
			return RunDetail{}, err
		}
		if _, seen := chosen[sv.NodeID]; !seen || !sv.Hidden {
			chosen[sv.NodeID] = sv
			readable[sv.NodeID] = !sv.Hidden
		}
	}
	added := false
	for _, sv := range chosen {
		if !have[stepKey{sv.TokenID, sv.Seq}] {
			d.Steps = append(d.Steps, sv)
			added = true
		}
	}
	if added {
		sort.SliceStable(d.Steps, func(i, j int) bool {
			if !d.Steps[i].StartedAt.Equal(d.Steps[j].StartedAt) {
				return d.Steps[i].StartedAt.Before(d.Steps[j].StartedAt)
			}
			return d.Steps[i].Seq < d.Steps[j].Seq
		})
	}
	if d.RunSummary, err = redactSummary(ctx, proj, &viewer, d.RunSummary, run.Sensitive); err != nil {
		return RunDetail{}, err
	}
	if d.CurrentNodes == nil {
		d.CurrentNodes = []string{}
	}
	return d, nil
}

// likePrefix is a LIKE pattern matching strings that start with s, its wildcards escaped.
func likePrefix(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s) + "%"
}
