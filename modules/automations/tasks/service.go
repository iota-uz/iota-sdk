package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

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

// TaskKey names a human node's task to the Store: the task, its workflow instance and the run's wait
// that opened it.
type TaskKey struct {
	TaskID, InstanceID uuid.UUID
	WaitID             string
}

// TaskMeta is the automation side of a task: its run and automation, the node that opened it, the
// rendered params of the step (Input) and the node's params in the graph version the run is pinned to
// (Params).
type TaskMeta struct {
	RunID, AutomationID uuid.UUID
	NodeID              string
	Input, Params       json.RawMessage
}

// DecidedPosition is a decision of a user: the task and when it was decided.
type DecidedPosition struct {
	TaskID uuid.UUID
	At     time.Time
}

// Store reads the automation side of tasks; the inbox owns paging and visibility.
type Store interface {
	// Metas are the metas of the tasks by task ID; a key of no run's wait is left out.
	Metas(ctx context.Context, tenant uuid.UUID, keys []TaskKey) (map[uuid.UUID]TaskMeta, error)
	// Decided are the user's decisions on tasks of the workflow, newest first ((At, TaskID) descending),
	// strictly after after (nil: from the newest), at most limit.
	Decided(ctx context.Context, tenant uuid.UUID, userID int64, workflow string, after *DecidedPosition, limit int) ([]DecidedPosition, error)
}

// Config wires the inbox.
type Config struct {
	Tasks Substrate
	// Store reads the automation side of a task: its run, the wait and the step that opened it, the graph
	// version the run is pinned to; and the user's decisions.
	Store Store
	// Audience judges role and group assignees by current membership (workflow.TaskAudience).
	Audience workflow.TaskAudience
	// Subjects loads the user's current rights.
	Subjects host.SubjectLoader
	Users    Users
	Forms    FormLookup
	// Subjects of tasks; nil: the subject's entity and key without a link.
	SubjectNames SubjectResolver
	Now          func() time.Time
	// Logger records what the inbox degrades on (a subject it could not name); nil: slog.Default().
	Logger *slog.Logger
}

// Service is the TaskInbox over the human tasks substrate.
type Service struct {
	cfg Config
}

var _ TaskInbox = (*Service)(nil)

// NewService builds the inbox; Tasks, Store and Subjects are required.
func NewService(cfg Config) (*Service, error) {
	if cfg.Tasks == nil || cfg.Store == nil || cfg.Subjects == nil {
		return nil, serrors.New(serrors.Invalid, "automations tasks: Tasks, Store and Subjects are required").WithOp(op)
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
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
	// The open tasks as they are: one past its final deadline the run has not closed yet is still open
	// and overdue (shown, not decidable), in «Mine» and «Overdue» alike, as the counts count it.
	wf.PastDeadline = true
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
	var last workflow.Task
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
				// One more matching task exists: the page ends at the last one it holds, at the place the
				// substrate listed it (its due time as of the traversal's start).
				page.NextCursor = workflow.CursorAfter(last).String()
				return page, nil
			}
			page.Items = append(page.Items, v)
			last = t
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
	var after *DecidedPosition
	if f.Cursor != "" {
		at, id, err := parseDecidedCursor(f.Cursor)
		if err != nil {
			return TaskPage{}, serrors.Wrap(op, fmt.Errorf("%w: cursor", ErrInvalidDecision))
		}
		after = &DecidedPosition{TaskID: id, At: at}
	}
	read := func(after *DecidedPosition, limit int) ([]DecidedPosition, error) {
		list, err := s.cfg.Store.Decided(ctx, a.subject.TenantID, a.subject.UserID, engine.WorkflowID, after, limit)
		if err != nil {
			return nil, serrors.Wrap(op, err)
		}
		return list, nil
	}
	shown := func(list []DecidedPosition) (map[uuid.UUID]TaskView, error) {
		ids := make([]uuid.UUID, len(list))
		for i, p := range list {
			ids[i] = p.TaskID
		}
		tasks, err := s.cfg.Tasks.TasksByID(ctx, a.subject.TenantID, ids)
		if err != nil {
			return nil, serrors.Wrap(op, err)
		}
		views, err := s.views(ctx, a, tasks)
		if err != nil {
			return nil, err
		}
		byID := make(map[uuid.UUID]TaskView, len(views))
		for _, v := range views {
			if matches(v, search) {
				byID[v.ID] = v
			}
		}
		return byID, nil
	}
	return fillDecided(f.Limit, decidedBatch, after, read, shown)
}

// decidedBatch is how many decisions a pass of «Decided by me» reads.
const decidedBatch = 100

// fillDecided reads the user's decisions batch by batch (read, newest first after a position) and
// keeps the ones shown (visible and matching) until the page holds limit; when one more is shown,
// the page's cursor is the position of its last task, whatever batch it came from.
func fillDecided(limit, batch int, after *DecidedPosition, read func(after *DecidedPosition, limit int) ([]DecidedPosition, error),
	shown func([]DecidedPosition) (map[uuid.UUID]TaskView, error)) (TaskPage, error) {
	page := TaskPage{Items: []TaskView{}}
	var last DecidedPosition
	for {
		list, err := read(after, batch)
		if err != nil {
			return TaskPage{}, err
		}
		byID, err := shown(list)
		if err != nil {
			return TaskPage{}, err
		}
		for _, p := range list {
			v, ok := byID[p.TaskID]
			if !ok {
				continue
			}
			if len(page.Items) == limit {
				page.NextCursor = decidedCursor(last.At, last.TaskID)
				return page, nil
			}
			page.Items = append(page.Items, v)
			last = p
		}
		if len(list) < batch {
			return page, nil
		}
		end := list[len(list)-1]
		after = &end
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
	// The open tasks as they are: also one past its final deadline the run has not closed yet (open,
	// overdue, no longer decidable), as the lists show it.
	count := func(overdue *bool) (int, error) {
		f := workflow.TaskFilter{Workflow: engine.WorkflowID, Limit: workflow.MaxTaskInbox, Overdue: overdue, PastDeadline: true}
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
	overdue := true
	late, err := count(&overdue)
	if err != nil {
		return TaskCounts{}, err
	}
	return TaskCounts{Open: open, Overdue: late}, nil
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
		// A repeated submit of the same decision (a double click, a lost answer) is the decision already
		// recorded; any other intent, or one that cannot be told, is not.
		intent, err := s.intentOf(ctx, v, in)
		if err != nil {
			return err
		}
		if intent != "" && intent == *v.MyDecision {
			return nil
		}
		return serrors.Wrap(op, ErrAlreadyDecided)
	}
	if v.State != StateOpen || v.PastDeadline {
		// Decided, expired, cancelled, or past its final deadline: the form's effect never runs then.
		return serrors.Wrap(op, ErrTaskClosed)
	}
	if v.FormUnavailable {
		return serrors.Wrap(op, ErrFormUnavailable)
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
		// What the task would refuse is refused before the form performs its action.
		prepared, err := s.precheck(ctx, form, v, a.user, in)
		if err != nil {
			return serrors.Wrap(op, err)
		}
		// The action is authorized by the task as it is now, read right before it starts: an escalation
		// that set a final deadline, a vote or a delegation may have changed it while the submission was
		// checked.
		if v, err = s.actionable(ctx, a, id); err != nil {
			return err
		}
		fctx := ctx
		if v.Deadline != nil {
			// The action's context ends at the final deadline, as the substrate stops counting votes then.
			var cancel context.CancelFunc
			fctx, cancel = context.WithDeadline(ctx, *v.Deadline)
			defer cancel()
		}
		fd, err := form.Decide(fctx, v, a.user, in.Form)
		if err != nil {
			return serrors.Wrap(op, err)
		}
		outcome, fields = s.recorded(ctx, v, in.Comment, prepared, fd)
	}
	decision, err := decisionOf(v, outcome, in.Comment, fields)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	err = s.cfg.Tasks.DecideTask(ctx, a.subject, id, decision)
	if errors.Is(err, workflow.ErrAlreadyDecided) || errors.Is(err, workflow.ErrTaskClosed) {
		// A concurrent submit of the same user may have won, also with the vote that closed the task:
		// the same outcome is this decision, another one is the user's other decision.
		if again, gerr := s.get(ctx, a, id); gerr == nil && again.MyDecision != nil {
			if *again.MyDecision == outcome {
				return nil
			}
			return serrors.Wrap(op, errors.Join(ErrAlreadyDecided, err))
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

// precheck judges a task form's submission before the form acts: the decision its FormPreparer
// prepares in full (returned), else the outcome its FormIntent asks for, else the comment alone. A form
// without FormPreparer decides only a task whose schema constrains no field (FormUnavailable otherwise),
// so what it returns cannot break the schema.
func (s *Service) precheck(ctx context.Context, form TaskForm, v TaskView, actor user.User, in DecideInput) (*FormDecision, error) {
	if p, ok := form.(FormPreparer); ok {
		fd, err := p.Prepare(ctx, v, actor, in.Form)
		if err != nil {
			return nil, err
		}
		if _, err := decisionOf(v, fd.Outcome, in.Comment, fd.Fields); err != nil {
			return nil, err
		}
		return &fd, nil
	}
	if fi, ok := form.(FormIntent); ok {
		outcome, err := fi.Intent(ctx, v, in.Form)
		if err != nil {
			return nil, errors.Join(ErrInvalidDecision, err)
		}
		_, err = checkOutcome(v, outcome, in.Comment)
		return nil, err
	}
	return nil, checkComment(in.Comment)
}

// actionable is the task read afresh, as long as the user may still decide it now: its final deadline
// is judged by the current clock.
func (s *Service) actionable(ctx context.Context, a actor, id uuid.UUID) (TaskView, error) {
	v, err := s.get(ctx, a, id)
	switch {
	case err != nil:
		return TaskView{}, err
	case v.MyDecision != nil:
		return TaskView{}, serrors.Wrap(op, ErrAlreadyDecided)
	case v.State != StateOpen || v.PastDeadline || (v.Deadline != nil && !s.cfg.Now().Before(*v.Deadline)):
		return TaskView{}, serrors.Wrap(op, ErrTaskClosed)
	case v.FormUnavailable:
		return TaskView{}, serrors.Wrap(op, ErrFormUnavailable)
	case !v.CanDecide:
		return TaskView{}, serrors.Wrap(op, fmt.Errorf("%w: %s", ErrTaskNotFound, id))
	}
	return v, nil
}

// recorded is the outcome and fields a form's action is recorded with. The action has run, so the
// decision must not fail validation now: a prepared decision stands unless the action's own result has
// its outcome and passes too; without one, fields the task refuses are left out (the schema constrains
// none, see precheck) and the form's fault is logged.
func (s *Service) recorded(ctx context.Context, v TaskView, comment string, prepared *FormDecision, fd FormDecision) (string, json.RawMessage) {
	if prepared != nil {
		if fd.Outcome == prepared.Outcome {
			if _, err := decisionOf(v, fd.Outcome, comment, fd.Fields); err == nil {
				return fd.Outcome, fd.Fields
			}
		}
		s.cfg.Logger.ErrorContext(ctx, "automations tasks: a form decided other than it prepared; the prepared decision is recorded",
			"task_id", v.ID, "form", v.FormKind, "prepared", prepared.Outcome, "decided", fd.Outcome)
		return prepared.Outcome, prepared.Fields
	}
	if _, err := decisionOf(v, fd.Outcome, comment, fd.Fields); err != nil {
		if _, err := decisionOf(v, fd.Outcome, comment, nil); err == nil {
			s.cfg.Logger.ErrorContext(ctx, "automations tasks: a form returned fields the task refuses; they are left out",
				"task_id", v.ID, "form", v.FormKind, "error", err)
			return fd.Outcome, nil
		}
	}
	return fd.Outcome, fd.Fields
}

// checkComment bounds a comment's length.
func checkComment(comment string) error {
	if len([]rune(strings.TrimSpace(comment))) > MaxCommentLength {
		return fmt.Errorf("%w: a comment has at most %d characters", ErrInvalidDecision, MaxCommentLength)
	}
	return nil
}

// checkOutcome checks an outcome against the node's buttons and the comment it needs; it returns the
// trimmed comment.
func checkOutcome(v TaskView, outcome, comment string) (string, error) {
	i := slices.IndexFunc(v.Outcomes, func(o OutcomeOption) bool { return o.Value == outcome })
	if i < 0 {
		return "", fmt.Errorf("%w: outcome %q is not a button of the task", ErrInvalidDecision, outcome)
	}
	comment = strings.TrimSpace(comment)
	if v.Outcomes[i].CommentRequired && comment == "" {
		return "", fmt.Errorf("%w: %s needs a comment", ErrInvalidDecision, outcome)
	}
	if err := checkComment(comment); err != nil {
		return "", err
	}
	return comment, nil
}

// decisionOf is the decision a human node's task takes: {outcome, comment, fields}, checked against the
// node's buttons and form.
func decisionOf(v TaskView, outcome, comment string, fields json.RawMessage) (json.RawMessage, error) {
	comment, err := checkOutcome(v, outcome, comment)
	if err != nil {
		return nil, err
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
	if v.State != StateOpen || v.PastDeadline {
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

// waitIDOf is the wait of a human node's task ("task:<wait id>").
func waitIDOf(t workflow.Task) (string, bool) {
	if t.Workflow != engine.WorkflowID || !strings.HasPrefix(t.Name, "task:") {
		return "", false
	}
	return strings.TrimPrefix(t.Name, "task:"), true
}

func (s *Service) metas(ctx context.Context, tenant uuid.UUID, list []workflow.Task) (map[uuid.UUID]TaskMeta, error) {
	var keys []TaskKey
	for _, t := range list {
		if w, ok := waitIDOf(t); ok {
			keys = append(keys, TaskKey{TaskID: t.ID, InstanceID: t.InstanceID, WaitID: w})
		}
	}
	if len(keys) == 0 {
		return map[uuid.UUID]TaskMeta{}, nil
	}
	out, err := s.cfg.Store.Metas(ctx, tenant, keys)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return out, nil
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
func paramsOf(m TaskMeta) (nodeParams, nodeParams) {
	var rendered, pinned nodeParams
	_ = json.Unmarshal(m.Params, &pinned)
	if err := json.Unmarshal(m.Input, &rendered); err != nil || len(rendered.Buttons) == 0 {
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
	var lookupErr error
	if s.cfg.Audience != nil {
		for _, kind := range []string{workflow.TaskAssignRole, workflow.TaskAssignGroup} {
			for _, as := range t.Assignees {
				if as.Kind != kind {
					continue
				}
				ok, err := s.cfg.Audience.IsMember(ctx, a.subject.TenantID, a.subject.UserID, as)
				if err != nil {
					// Another assignee may still cover the user; a failed lookup proves nothing either way.
					lookupErr = errors.Join(lookupErr, err)
					continue
				}
				if ok {
					return kind, nil
				}
			}
		}
	}
	// A permission the user holds covers them whatever a membership lookup said; without one, an
	// unverified membership is an error, not a denial.
	if found == "" && lookupErr != nil {
		return "", serrors.Wrap(op, lookupErr)
	}
	return found, nil
}

// agreeing is the progress of a quorum as the substrate closes a task: the most decisions that agree on
// one outcome (a split vote does not reach it).
func agreeing(list []workflow.TaskDecision) int {
	by := map[string]int{}
	most := 0
	for _, d := range list {
		by[d.Choice]++
		most = max(most, by[d.Choice])
	}
	return most
}

func (s *Service) view(ctx context.Context, a actor, t workflow.Task, m TaskMeta, names map[int64]string, now time.Time) (TaskView, bool, error) {
	me := a.subject.UserID
	v := TaskView{
		ID: t.ID, RunID: m.RunID, AutomationID: m.AutomationID, NodeID: m.NodeID,
		Quorum: Quorum{Required: max(t.Required, 1), Decided: agreeing(t.Decisions)},
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
	// Only the current audience sees a task, its decisions included: a user who left the role after
	// voting sees it no more.
	if v.AssignedVia == "" || delegatedAway {
		return TaskView{}, false, nil
	}
	// An open task past its final deadline stays open (the run closes it) and overdue, but takes no
	// decision any more: the substrate's predicate, judged apart from the state shown.
	v.PastDeadline = t.PastFinalDeadline(now)
	if t.Status == "open" && t.DueAt != nil && t.EscalationLevel >= len(t.Escalation) {
		v.Deadline = t.DueAt
	}
	v.CanDecide = t.Status == "open" && v.MyDecision == nil && !v.PastDeadline
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
		form, found, err := s.form(kind)
		v.FormUnavailable = err != nil || !found
		if !v.FormUnavailable {
			// A form that acts before its fields are known decides only what the schema leaves open.
			_, prepares := form.(FormPreparer)
			v.FormUnavailable = !prepares && ConstrainsFields(v.DecisionSchema)
		}
	} else if !ok {
		v.FormKind, v.FormUnavailable = "invalid", true
	}
	if t.SubjectEntity != "" && t.SubjectEntity != graph.SubjectNone {
		ref := &SubjectRef{Entity: t.SubjectEntity, Key: t.SubjectKey}
		if s.cfg.SubjectNames != nil {
			label, link, err := s.cfg.SubjectNames.Subject(ctx, a.subject, t.SubjectEntity, t.SubjectKey)
			if err != nil {
				// The task stays usable: a neutral label (the entity) and no link.
				s.cfg.Logger.WarnContext(ctx, "automations tasks: subject name lookup failed",
					"task_id", t.ID, "entity", t.SubjectEntity, "error", err)
			} else {
				ref.Label, ref.URL = label, link
			}
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

// intentOf is the outcome a submit asks for without performing anything: the button of the default
// form, or what a task form's data asks for (FormIntent). "" when it cannot be told.
func (s *Service) intentOf(ctx context.Context, v TaskView, in DecideInput) (string, error) {
	// A task form chooses the outcome from its data, as its Decide does: an outcome field sent with it
	// is not the intent.
	if v.FormKind == "" {
		return in.Outcome, nil
	}
	form, ok, err := s.form(v.FormKind)
	if err != nil || !ok {
		return "", nil //nolint:nilerr // an unknown form tells nothing: not a repeat
	}
	fi, ok := form.(FormIntent)
	if !ok {
		return "", nil
	}
	outcome, err := fi.Intent(ctx, v, in.Form)
	if err != nil {
		return "", serrors.Wrap(op, errors.Join(ErrInvalidDecision, err))
	}
	return outcome, nil
}
