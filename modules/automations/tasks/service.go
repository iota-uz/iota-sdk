package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const op = serrors.Op("automations.tasks")

// Substrate is the human tasks substrate of the workflow engine (*workflow.Engine).
type Substrate interface {
	Tasks(ctx context.Context, actor spi.Subject, f workflow.TaskFilter) ([]workflow.Task, error)
	TasksByID(ctx context.Context, tenant uuid.UUID, ids []uuid.UUID) ([]workflow.Task, error)
	DecideTask(ctx context.Context, actor spi.Subject, taskID uuid.UUID, decision json.RawMessage) error
	DelegateTask(ctx context.Context, actor spi.Subject, taskID uuid.UUID, to int64, comment string) error
}

// UserOption is a user a picker offers; Hint tells namesakes apart (an email).
type UserOption struct {
	UserRef
	Hint string
}

// Users are the names of a tenant's users and the search of its active ones.
type Users interface {
	Names(ctx context.Context, tenantID uuid.UUID, ids []int64) (map[int64]string, error)
	Search(ctx context.Context, tenantID uuid.UUID, query string, exclude []int64, limit int) ([]UserOption, error)
}

// SubjectResolver names the record a task is about for a viewer: its label and, only when the viewer
// may read the record, the URL of its page ("" otherwise).
type SubjectResolver interface {
	Subject(ctx context.Context, viewer spi.Subject, entity, key string) (label, url string, err error)
}

// Config wires the inbox.
type Config struct {
	Tasks Substrate
	// DB reads the automation side of a task: its run, the wait and the step that opened it, the graph
	// version the run is pinned to.
	DB *pgxpool.Pool
	// Audience judges role and group assignees by current membership (workflow.TaskAudience).
	Audience workflow.TaskAudience
	// Subjects loads the user's current rights.
	Subjects host.SubjectLoader
	Users    Users
	Forms    FormLookup
	// Subjects of tasks; nil: the subject's entity and key without a link.
	SubjectNames SubjectResolver
	Now          func() time.Time
}

// Service is the TaskInbox over the human tasks substrate.
type Service struct {
	cfg Config
}

var _ TaskInbox = (*Service)(nil)

// NewService builds the inbox; Tasks, DB and Subjects are required.
func NewService(cfg Config) (*Service, error) {
	if cfg.Tasks == nil || cfg.DB == nil || cfg.Subjects == nil {
		return nil, serrors.New(serrors.Invalid, "automations tasks: Tasks, DB and Subjects are required").WithOp(op)
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Service{cfg: cfg}, nil
}

// actor is the user of the context with their rights as they are now.
type actor struct {
	user    user.User
	subject spi.Subject
}

func (s *Service) actor(ctx context.Context) (actor, error) {
	u, err := composables.UseUser(ctx)
	if err != nil || u == nil {
		return actor{}, serrors.Wrap(op, ErrTaskNotFound)
	}
	sub, err := s.cfg.Subjects.LoadSubject(ctx, u.TenantID(), int64(u.ID()), spi.SubjectInitiator)
	if err != nil {
		// A user the tenant does not know (or no longer lets act) sees no task.
		return actor{}, serrors.Wrap(op, errors.Join(ErrTaskNotFound, err))
	}
	return actor{user: u, subject: sub}, nil
}

// ---- reads ----

// List implements TaskInbox.
func (s *Service) List(ctx context.Context, f TaskFilter) (TaskPage, error) {
	a, err := s.actor(ctx)
	if err != nil {
		return TaskPage{}, err
	}
	if f.Limit <= 0 {
		f.Limit = DefaultLimit
	}
	f.Limit = min(f.Limit, MaxLimit)
	search := strings.ToLower(strings.TrimSpace(f.Search))
	switch f.Scope {
	case "", ScopeMine, ScopeOverdue:
		return s.listOpen(ctx, a, f, search)
	case ScopeDecidedByMe:
		return s.listDecided(ctx, a, f, search)
	}
	return TaskPage{}, serrors.Wrap(op, fmt.Errorf("%w: scope %q", ErrInvalidDecision, f.Scope))
}

func matches(v TaskView, search string) bool {
	if search == "" {
		return true
	}
	if strings.Contains(strings.ToLower(v.Title), search) {
		return true
	}
	return v.Subject != nil && strings.Contains(strings.ToLower(v.Subject.Label), search)
}

// listOpen reads the substrate's inbox page by page until the filtered page is full.
func (s *Service) listOpen(ctx context.Context, a actor, f TaskFilter, search string) (TaskPage, error) {
	wf := workflow.TaskFilter{Workflow: engine.WorkflowID, Limit: workflow.MaxTaskInbox}
	if f.Scope == ScopeOverdue {
		overdue := true
		wf.Overdue = &overdue
	}
	if f.Cursor != "" {
		c, err := workflow.ParseTaskCursor(f.Cursor)
		if err != nil {
			return TaskPage{}, serrors.Wrap(op, fmt.Errorf("%w: cursor", ErrInvalidDecision))
		}
		wf.Cursor = &c
	}
	page := TaskPage{Items: []TaskView{}}
	for {
		list, err := s.cfg.Tasks.Tasks(ctx, a.subject, wf)
		if err != nil {
			return TaskPage{}, serrors.Wrap(op, err)
		}
		views, err := s.views(ctx, a, list)
		if err != nil {
			return TaskPage{}, err
		}
		byID := make(map[uuid.UUID]TaskView, len(views))
		for _, v := range views {
			byID[v.ID] = v
		}
		for _, t := range list {
			v, ok := byID[t.ID]
			if !ok || !matches(v, search) {
				continue
			}
			if len(page.Items) == f.Limit {
				// One more matching task exists: the page ends at the last one it holds.
				last := page.Items[len(page.Items)-1]
				page.NextCursor = workflow.TaskCursor{DueAt: last.DueAt, ID: last.ID}.String()
				return page, nil
			}
			page.Items = append(page.Items, v)
		}
		if len(list) < wf.Limit {
			return page, nil
		}
		c := workflow.CursorAfter(list[len(list)-1])
		wf.Cursor = &c
	}
}

// decidedCursor is the position after a decision of the user: "<decided_at>/<task id>".
func decidedCursor(at time.Time, id uuid.UUID) string {
	return at.UTC().Format(time.RFC3339Nano) + "/" + id.String()
}

func parseDecidedCursor(s string) (time.Time, uuid.UUID, error) {
	at, id, ok := strings.Cut(s, "/")
	if !ok {
		return time.Time{}, uuid.Nil, ErrInvalidDecision
	}
	t, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidDecision
	}
	u, err := uuid.Parse(id)
	if err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidDecision
	}
	return t, u, nil
}

// listDecided reads the tasks the user decided, newest decision first.
func (s *Service) listDecided(ctx context.Context, a actor, f TaskFilter, search string) (TaskPage, error) {
	var (
		hasCursor bool
		afterAt   time.Time
		afterID   uuid.UUID
	)
	if f.Cursor != "" {
		at, id, err := parseDecidedCursor(f.Cursor)
		if err != nil {
			return TaskPage{}, serrors.Wrap(op, fmt.Errorf("%w: cursor", ErrInvalidDecision))
		}
		hasCursor, afterAt, afterID = true, at, id
	}
	const batch = 100
	page := TaskPage{Items: []TaskView{}}
	for {
		rows, err := s.cfg.DB.Query(ctx, `
			SELECT d.task_id, d.decided_at FROM extruntime.human_task_decisions d
			JOIN extruntime.human_tasks t ON t.id = d.task_id AND t.tenant_id = d.tenant_id
			JOIN extruntime.workflow_instances i ON i.id = t.instance_id AND i.tenant_id = t.tenant_id
			WHERE d.tenant_id = $1 AND d.user_id = $2 AND i.workflow = $3
			  AND (NOT $4 OR (d.decided_at, d.task_id) < ($5::timestamptz, $6::uuid))
			ORDER BY d.decided_at DESC, d.task_id DESC
			LIMIT $7`, a.subject.TenantID, a.subject.UserID, engine.WorkflowID, hasCursor, afterAt, afterID, batch)
		if err != nil {
			return TaskPage{}, serrors.FromDB(op, err)
		}
		type pos struct {
			id uuid.UUID
			at time.Time
		}
		list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (pos, error) {
			var p pos
			err := r.Scan(&p.id, &p.at)
			return p, err
		})
		if err != nil {
			return TaskPage{}, serrors.FromDB(op, err)
		}
		ids := make([]uuid.UUID, len(list))
		for i, p := range list {
			ids[i] = p.id
		}
		tasks, err := s.cfg.Tasks.TasksByID(ctx, a.subject.TenantID, ids)
		if err != nil {
			return TaskPage{}, serrors.Wrap(op, err)
		}
		views, err := s.views(ctx, a, tasks)
		if err != nil {
			return TaskPage{}, err
		}
		byID := make(map[uuid.UUID]TaskView, len(views))
		for _, v := range views {
			byID[v.ID] = v
		}
		var lastAt time.Time
		for _, p := range list {
			v, ok := byID[p.id]
			if !ok || !matches(v, search) {
				continue
			}
			if len(page.Items) == f.Limit {
				page.NextCursor = decidedCursor(lastAt, page.Items[len(page.Items)-1].ID)
				return page, nil
			}
			page.Items = append(page.Items, v)
			lastAt = p.at
		}
		if len(list) < batch {
			return page, nil
		}
		last := list[len(list)-1]
		hasCursor, afterAt, afterID = true, last.at, last.id
	}
}

// Get implements TaskInbox.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (TaskView, error) {
	a, err := s.actor(ctx)
	if err != nil {
		return TaskView{}, err
	}
	return s.get(ctx, a, id)
}

func (s *Service) get(ctx context.Context, a actor, id uuid.UUID) (TaskView, error) {
	list, err := s.cfg.Tasks.TasksByID(ctx, a.subject.TenantID, []uuid.UUID{id})
	if err != nil {
		return TaskView{}, serrors.Wrap(op, err)
	}
	views, err := s.views(ctx, a, list)
	if err != nil {
		return TaskView{}, err
	}
	if len(views) == 0 {
		return TaskView{}, serrors.Wrap(op, fmt.Errorf("%w: %s", ErrTaskNotFound, id))
	}
	return views[0], nil
}

// countCap bounds the counts: a navigation badge shows "99+" long before it.
const countCap = 1000

// Counts implements TaskInbox.
func (s *Service) Counts(ctx context.Context) (TaskCounts, error) {
	a, err := s.actor(ctx)
	if err != nil {
		return TaskCounts{}, err
	}
	count := func(overdue *bool) (int, error) {
		f := workflow.TaskFilter{Workflow: engine.WorkflowID, Limit: workflow.MaxTaskInbox, Overdue: overdue}
		n := 0
		for n < countCap {
			list, err := s.cfg.Tasks.Tasks(ctx, a.subject, f)
			if err != nil {
				return 0, serrors.Wrap(op, err)
			}
			n += len(list)
			if len(list) < f.Limit {
				break
			}
			c := workflow.CursorAfter(list[len(list)-1])
			f.Cursor = &c
		}
		return min(n, countCap), nil
	}
	open, err := count(nil)
	if err != nil {
		return TaskCounts{}, err
	}
	out := TaskCounts{Open: open}
	if open > 0 {
		overdue := true
		if out.Overdue, err = count(&overdue); err != nil {
			return TaskCounts{}, err
		}
	}
	return out, nil
}

// ---- writes ----

// Decide implements TaskInbox.
func (s *Service) Decide(ctx context.Context, id uuid.UUID, in DecideInput) error {
	if OriginFrom(ctx) != engine.OriginHuman {
		return serrors.Wrap(op, ErrHumanRequired)
	}
	a, err := s.actor(ctx)
	if err != nil {
		return err
	}
	v, err := s.get(ctx, a, id)
	if err != nil {
		return err
	}
	if v.MyDecision != nil {
		// A repeated submit of the same decision (a double click) is the decision already recorded.
		if in.Outcome == "" || in.Outcome == *v.MyDecision {
			return nil
		}
		return serrors.Wrap(op, ErrAlreadyDecided)
	}
	if v.State != StateOpen {
		return serrors.Wrap(op, ErrTaskClosed)
	}
	if !v.CanDecide {
		return serrors.Wrap(op, fmt.Errorf("%w: %s", ErrTaskNotFound, id))
	}
	outcome, fields := in.Outcome, in.Fields
	if v.FormKind != "" {
		form, ok, err := s.form(v.FormKind)
		if err != nil {
			return serrors.Wrap(op, err)
		}
		if !ok {
			return serrors.Wrap(op, ErrFormUnavailable)
		}
		fd, err := form.Decide(ctx, v, a.user, in.Form)
		if err != nil {
			return serrors.Wrap(op, err)
		}
		outcome, fields = fd.Outcome, fd.Fields
	}
	decision, err := decisionOf(v, outcome, in.Comment, fields)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	err = s.cfg.Tasks.DecideTask(ctx, a.subject, id, decision)
	if errors.Is(err, workflow.ErrAlreadyDecided) {
		// A concurrent submit of the same user won: the same outcome is this decision.
		if again, gerr := s.get(ctx, a, id); gerr == nil && again.MyDecision != nil && *again.MyDecision == outcome {
			return nil
		}
	}
	return mapErr(err)
}

func (s *Service) form(kind string) (TaskForm, bool, error) {
	if s.cfg.Forms == nil {
		return nil, false, nil
	}
	return s.cfg.Forms.Form(kind)
}

// decisionOf is the decision a human node's task takes: {outcome, comment, fields}, checked against the
// node's buttons and form.
func decisionOf(v TaskView, outcome, comment string, fields json.RawMessage) (json.RawMessage, error) {
	i := slices.IndexFunc(v.Outcomes, func(o OutcomeOption) bool { return o.Value == outcome })
	if i < 0 {
		return nil, fmt.Errorf("%w: outcome %q is not a button of the task", ErrInvalidDecision, outcome)
	}
	comment = strings.TrimSpace(comment)
	if v.Outcomes[i].CommentRequired && comment == "" {
		return nil, fmt.Errorf("%w: %s needs a comment", ErrInvalidDecision, outcome)
	}
	if len([]rune(comment)) > MaxCommentLength {
		return nil, fmt.Errorf("%w: a comment has at most %d characters", ErrInvalidDecision, MaxCommentLength)
	}
	if f := strings.TrimSpace(string(fields)); f == "" || f == "null" {
		fields = nil
	}
	if len(v.DecisionSchema) > 0 {
		check := fields
		if check == nil {
			check = json.RawMessage(`{}`)
		}
		if err := graph.ValidateJSON(v.DecisionSchema, check); err != nil {
			return nil, fmt.Errorf("%w: fields: %s", ErrInvalidDecision, err.Error())
		}
	} else if fields != nil {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(fields, &obj); err != nil {
			return nil, fmt.Errorf("%w: fields are an object", ErrInvalidDecision)
		}
	}
	d := map[string]any{"outcome": outcome}
	if comment != "" {
		d["comment"] = comment
	}
	if fields != nil {
		d["fields"] = fields
	}
	b, err := json.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidDecision, err)
	}
	return b, nil
}

// Delegate implements TaskInbox.
func (s *Service) Delegate(ctx context.Context, id uuid.UUID, to uint, comment string) error {
	if OriginFrom(ctx) != engine.OriginHuman {
		return serrors.Wrap(op, ErrHumanRequired)
	}
	a, err := s.actor(ctx)
	if err != nil {
		return err
	}
	v, err := s.get(ctx, a, id)
	if err != nil {
		return err
	}
	if v.State != StateOpen {
		return serrors.Wrap(op, ErrTaskClosed)
	}
	if v.MyDecision != nil {
		return serrors.Wrap(op, ErrAlreadyDecided)
	}
	if !v.CanDecide {
		return serrors.Wrap(op, fmt.Errorf("%w: %s", ErrTaskNotFound, id))
	}
	if !v.AllowDelegation || to == 0 || int64(to) == a.subject.UserID {
		return serrors.Wrap(op, ErrDelegationDenied)
	}
	return mapErr(s.cfg.Tasks.DelegateTask(ctx, a.subject, id, int64(to), strings.TrimSpace(comment)))
}

// Delegates are the users the user may pass a task to: active users of the tenant matching query,
// without the user and those who already delegated or decided the task.
func (s *Service) Delegates(ctx context.Context, id uuid.UUID, query string, limit int) ([]UserOption, error) {
	a, err := s.actor(ctx)
	if err != nil {
		return nil, err
	}
	v, err := s.get(ctx, a, id)
	if err != nil {
		return nil, err
	}
	if !v.CanDecide || !v.AllowDelegation || s.cfg.Users == nil {
		return []UserOption{}, nil
	}
	list, err := s.cfg.Tasks.TasksByID(ctx, a.subject.TenantID, []uuid.UUID{id})
	if err != nil || len(list) == 0 {
		return nil, serrors.Wrap(op, ErrTaskNotFound)
	}
	exclude := []int64{a.subject.UserID}
	for _, d := range list[0].Delegations {
		exclude = append(exclude, d.From)
	}
	for _, d := range list[0].Decisions {
		exclude = append(exclude, d.UserID)
	}
	return s.cfg.Users.Search(ctx, a.subject.TenantID, query, exclude, limit)
}

// mapErr turns an error of the substrate into the inbox's.
func mapErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, workflow.ErrNotFound), errors.Is(err, workflow.ErrForbidden):
		return serrors.Wrap(op, errors.Join(ErrTaskNotFound, err))
	case errors.Is(err, workflow.ErrTaskClosed):
		return serrors.Wrap(op, errors.Join(ErrTaskClosed, err))
	case errors.Is(err, workflow.ErrAlreadyDecided):
		return serrors.Wrap(op, errors.Join(ErrAlreadyDecided, err))
	case errors.Is(err, workflow.ErrDelegation):
		return serrors.Wrap(op, errors.Join(ErrDelegationDenied, err))
	case errors.Is(err, workflow.ErrInvalid):
		return serrors.Wrap(op, errors.Join(ErrInvalidDecision, err))
	}
	return serrors.Wrap(op, err)
}

// ---- views ----

// taskMeta is the automation side of a task.
type taskMeta struct {
	runID, automationID uuid.UUID
	nodeID              string
	input, params       json.RawMessage // the rendered params of the step; the node's params in the version
}

// waitIDOf is the wait of a human node's task ("task:<wait id>").
func waitIDOf(t workflow.Task) (string, bool) {
	if t.Workflow != engine.WorkflowID || !strings.HasPrefix(t.Name, "task:") {
		return "", false
	}
	return strings.TrimPrefix(t.Name, "task:"), true
}

func (s *Service) metas(ctx context.Context, tenant uuid.UUID, list []workflow.Task) (map[uuid.UUID]taskMeta, error) {
	var (
		instances []uuid.UUID
		waits     []string
		ids       []uuid.UUID
	)
	for _, t := range list {
		if w, ok := waitIDOf(t); ok {
			instances, waits, ids = append(instances, t.InstanceID), append(waits, w), append(ids, t.ID)
		}
	}
	out := map[uuid.UUID]taskMeta{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.cfg.DB.Query(ctx, `
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
		return nil, serrors.FromDB(op, err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id uuid.UUID
			m  taskMeta
		)
		if err := rows.Scan(&id, &m.runID, &m.automationID, &m.nodeID, &m.input, &m.params); err != nil {
			return nil, serrors.FromDB(op, err)
		}
		out[id] = m
	}
	return out, serrors.FromDB(op, rows.Err())
}

// nodeParams are the params of a human node as the inbox reads them.
type nodeParams struct {
	Title   string `json:"title"`
	Body    string `json:"body"`
	Buttons []struct {
		Outcome        string `json:"outcome"`
		Label          string `json:"label"`
		Style          string `json:"style"`
		RequireComment bool   `json:"require_comment"`
	} `json:"buttons"`
	Form          json.RawMessage `json:"form"`
	AllowDelegate bool            `json:"allow_delegate"`
}

// paramsOf are the rendered params of the step, else (a pruned step) the node's params in the version.
func paramsOf(m taskMeta) (nodeParams, nodeParams) {
	var rendered, pinned nodeParams
	_ = json.Unmarshal(m.params, &pinned)
	if err := json.Unmarshal(m.input, &rendered); err != nil || len(rendered.Buttons) == 0 {
		return pinned, pinned
	}
	return rendered, pinned
}

// views are the tasks the user may see, in order: a task of an assignee that covers the user by
// current membership, delegated to the user, or decided by the user. Others are left out.
func (s *Service) views(ctx context.Context, a actor, list []workflow.Task) ([]TaskView, error) {
	if len(list) == 0 {
		return []TaskView{}, nil
	}
	metas, err := s.metas(ctx, a.subject.TenantID, list)
	if err != nil {
		return nil, err
	}
	names, err := s.names(ctx, a.subject.TenantID, list)
	if err != nil {
		return nil, err
	}
	now := s.cfg.Now()
	out := make([]TaskView, 0, len(list))
	for _, t := range list {
		m, ok := metas[t.ID]
		if !ok {
			continue
		}
		v, visible, err := s.view(ctx, a, t, m, names, now)
		if err != nil {
			return nil, err
		}
		if visible {
			out = append(out, v)
		}
	}
	return out, nil
}

func (s *Service) names(ctx context.Context, tenant uuid.UUID, list []workflow.Task) (map[int64]string, error) {
	var ids []int64
	for _, t := range list {
		for _, d := range t.Delegations {
			ids = append(ids, d.From, d.To)
		}
		for _, d := range t.Decisions {
			ids = append(ids, d.UserID)
		}
		if t.DecidedBy != nil {
			ids = append(ids, *t.DecidedBy)
		}
	}
	if len(ids) == 0 || s.cfg.Users == nil {
		return map[int64]string{}, nil
	}
	slices.Sort(ids)
	names, err := s.cfg.Users.Names(ctx, tenant, slices.Compact(ids))
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return names, nil
}

func userRef(id int64, names map[int64]string) UserRef {
	name := names[id]
	if name == "" {
		name = "#" + strconv.FormatInt(id, 10)
	}
	return UserRef{ID: uint(id), Name: name}
}

// via is how an assignee of the task covers the user now ("" when none does).
func (s *Service) via(ctx context.Context, a actor, t workflow.Task) (string, error) {
	found := ""
	for _, as := range t.Assignees {
		switch as.Kind {
		case workflow.TaskAssignUser:
			if as.UserID != nil && *as.UserID == a.subject.UserID {
				return ViaUser, nil
			}
		case workflow.TaskAssignPermission:
			if as.Ref != "" && a.subject.Has(as.Ref) && found == "" {
				found = ViaPermission
			}
		}
	}
	if s.cfg.Audience != nil {
		for _, kind := range []string{workflow.TaskAssignRole, workflow.TaskAssignGroup} {
			for _, as := range t.Assignees {
				if as.Kind != kind {
					continue
				}
				ok, err := s.cfg.Audience.IsMember(ctx, a.subject.TenantID, a.subject.UserID, as)
				if err != nil {
					return "", serrors.Wrap(op, err)
				}
				if ok {
					return kind, nil
				}
			}
		}
	}
	return found, nil
}

func (s *Service) view(ctx context.Context, a actor, t workflow.Task, m taskMeta, names map[int64]string, now time.Time) (TaskView, bool, error) {
	me := a.subject.UserID
	v := TaskView{
		ID: t.ID, RunID: m.runID, AutomationID: m.automationID, NodeID: m.nodeID,
		Quorum: Quorum{Required: max(t.Required, 1), Decided: len(t.Decisions)},
		DueAt:  t.DueAt, Escalated: t.EscalationLevel > 0, State: t.Status, CreatedAt: t.CreatedAt,
		AllowDelegation: t.AllowDelegation,
	}
	for _, d := range t.Decisions {
		dec := Decision{By: userRef(d.UserID, names), Outcome: d.Choice, At: d.DecidedAt}
		var body struct {
			Comment string `json:"comment"`
		}
		_ = json.Unmarshal(d.Decision, &body)
		dec.Comment = body.Comment
		if d.DelegatedFrom != nil {
			from := userRef(*d.DelegatedFrom, names)
			dec.DelegatedFrom = &from
		}
		v.Decisions = append(v.Decisions, dec)
		if d.UserID == me {
			choice := d.Choice
			v.MyDecision = &choice
		}
	}
	// The delegation passed to the user (the earliest, as the substrate counts it), and whether the user
	// passed the task on: then they lost it.
	delegatedAway := false
	for _, d := range t.Delegations {
		if d.From == me {
			delegatedAway = true
		}
	}
	for _, d := range t.Delegations {
		if d.To == me && v.DelegatedBy == nil {
			by := userRef(d.From, names)
			v.AssignedVia, v.DelegatedBy = ViaDelegated, &by
		}
	}
	if v.AssignedVia == "" && !delegatedAway {
		via, err := s.via(ctx, a, t)
		if err != nil {
			return TaskView{}, false, err
		}
		v.AssignedVia = via
	}
	covered := v.AssignedVia != "" && !delegatedAway
	if !covered && v.MyDecision == nil {
		return TaskView{}, false, nil
	}
	if v.AssignedVia == "" {
		v.AssignedVia = ViaUser
	}
	v.CanDecide = t.Status == "open" && covered && v.MyDecision == nil
	v.Overdue = t.Status == "open" && t.DueAt != nil && t.DueAt.Before(now)
	switch t.Status {
	case "decided":
		v.Outcome = t.Choice
		if t.DecidedBy != nil {
			by := userRef(*t.DecidedBy, names)
			v.ClosedBy = &by
		}
		v.ClosedAt = t.DecidedAt
	case "expired":
		v.ClosedAt = t.DueAt
	}

	rendered, pinned := paramsOf(m)
	v.Title = strings.TrimSpace(labelOf(t.Title))
	if v.Title == "" {
		v.Title = rendered.Title
	}
	v.Instructions = rendered.Body
	for _, b := range rendered.Buttons {
		style := b.Style
		if style != StylePrimary && style != StyleDanger {
			style = StyleDefault
		}
		label := strings.TrimSpace(b.Label)
		if label == "" {
			label = b.Outcome
		}
		v.Outcomes = append(v.Outcomes, OutcomeOption{Value: b.Outcome, Label: label, Style: style, CommentRequired: b.RequireComment})
	}
	if f := strings.TrimSpace(string(rendered.Form)); f != "" && f != "null" {
		v.DecisionSchema = rendered.Form
	}
	// The form's kind is read from the version the run is pinned to.
	if kind, ok := FormKindOf(pinned.Form); ok && kind != "" {
		v.FormKind = kind
		_, found, err := s.form(kind)
		v.FormUnavailable = err != nil || !found
	} else if !ok {
		v.FormKind, v.FormUnavailable = "invalid", true
	}
	if v.FormUnavailable {
		v.CanDecide = false
	}
	if t.SubjectEntity != "" && t.SubjectEntity != graph.SubjectNone {
		ref := &SubjectRef{Entity: t.SubjectEntity, Key: t.SubjectKey}
		if s.cfg.SubjectNames != nil {
			label, link, err := s.cfg.SubjectNames.Subject(ctx, a.subject, t.SubjectEntity, t.SubjectKey)
			if err != nil {
				return TaskView{}, false, serrors.Wrap(op, err)
			}
			ref.Label, ref.URL = label, link
		}
		if ref.Label == "" {
			ref.Label = t.SubjectEntity
		}
		v.Subject = ref
	}
	return v, true, nil
}

// labelOf is a title of one language: en, else any.
func labelOf(l map[string]string) string {
	if s := l["en"]; s != "" {
		return s
	}
	keys := make([]string, 0, len(l))
	for k := range l {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if l[k] != "" {
			return l[k]
		}
	}
	return ""
}

// DisplayName is the name of a user of the user's tenant ("" when unknown).
func (s *Service) DisplayName(ctx context.Context, id uint) string {
	u, err := composables.UseUser(ctx)
	if err != nil || u == nil || s.cfg.Users == nil || id == 0 {
		return ""
	}
	names, err := s.cfg.Users.Names(ctx, u.TenantID(), []int64{int64(id)})
	if err != nil {
		return ""
	}
	return names[int64(id)]
}
