package workflow

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
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Human tasks (#1327). A task is assigned to users, the holders of a
// permission, the members of a role or of a group; who may decide is judged
// by current rights when the decision comes, never fixed at creation. The
// task closes when Required decisions agree on one outcome, at once on a veto
// outcome, on timeout once its escalation steps are spent, or on cancel. A
// user decides once, and may pass their decision to another user when the
// task allows delegation (and then loses it). The assignees, the
// delegations and the decisions are rows of their own (human_task_assignees,
// human_task_decisions); one inbox (Tasks) serves package workflows and
// native ones.

// Task assignee kinds.
const (
	TaskAssignUser       = "user"
	TaskAssignPermission = "permission"
	TaskAssignRole       = "role"
	TaskAssignGroup      = "group"
)

// Who a human task is assigned to, as the compatibility fields of Task name
// it (the same values as the TaskAssign* kinds).
const (
	TaskAssigneeUser       = TaskAssignUser
	TaskAssigneePermission = TaskAssignPermission
)

// Limits of a TaskSpec.
const (
	MaxTaskAssignees  = 20
	MaxTaskRequired   = 100
	MaxTaskOutcomes   = 20
	MaxTaskEscalation = 5
	MaxTaskInbox      = 200
	// MaxTaskDuration bounds a task timeout and an escalation extension.
	MaxTaskDuration       = 10 * 365 * 24 * time.Hour
	maxDelegationComment  = 2000
	defaultTaskInboxLimit = 50
)

// TaskAssignee is one assignee of a human task.
type TaskAssignee struct {
	// Kind is TaskAssignUser (UserID), TaskAssignPermission (Ref is the
	// permission name), TaskAssignRole (Ref is the role id) or
	// TaskAssignGroup (Ref is the group uuid).
	Kind   string `json:"kind"`
	UserID *int64 `json:"userId,omitempty"`
	Ref    string `json:"ref,omitempty"`
}

func (a TaskAssignee) String() string {
	if a.Kind == TaskAssignUser && a.UserID != nil {
		return "user " + strconv.FormatInt(*a.UserID, 10)
	}
	return a.Kind + " " + a.Ref
}

// TaskSpec describes a human task.
type TaskSpec struct {
	// Assignees are 1..MaxTaskAssignees.
	Assignees []TaskAssignee
	// Required is the quorum, 1..MaxTaskRequired (0 means 1): the task closes
	// when this many decisions have the same outcome.
	Required int
	// Outcomes are the allowed values of the decision's outcome field. Empty:
	// decisions have no outcome, and Required must be 1.
	Outcomes []string
	// Veto are outcomes that close the task with the first decision (for
	// example "reject"); a subset of Outcomes.
	Veto []string
	// Decision is the schema of a decision. With Outcomes it must be an object
	// with an `outcome` enum of exactly the Outcomes; nil then means that
	// object alone.
	Decision *manifest.Type
	// Timeout is the due time of the first step; 0: none.
	Timeout time.Duration
	// Escalation are the steps applied when the due time passes, at most
	// MaxTaskEscalation; without steps (or once they are spent) the due time
	// closes the task with a timeout. Requires a Timeout.
	Escalation []EscalationStep
	// AllowDelegation lets an assignee pass their decision to another user.
	AllowDelegation bool
	Subject         *spi.ObjectRef
	Title           manifest.Label
}

// EscalationStep is applied when a task's due time passes.
type EscalationStep struct {
	// Add are added to the assignees.
	Add []TaskAssignee `json:"add,omitempty"`
	// Extend is the new due time from the escalation; 0: the task times out
	// right after the step.
	Extend time.Duration `json:"extend,omitempty"`
}

// Task outcomes.
const (
	TaskDecided   = "decided"
	TaskTimeout   = "timeout"
	TaskCancelled = "cancelled"
)

// TaskResult is the outcome of a Task.
type TaskResult struct {
	// Outcome is TaskDecided, TaskTimeout or TaskCancelled.
	Outcome string `json:"outcome"`
	// Choice is the winning outcome of the decisions ("" without Outcomes).
	Choice string `json:"choice,omitempty"`
	// Decision is the decision that closed the task.
	Decision json.RawMessage `json:"decision,omitempty"`
	By       *int64          `json:"by,omitempty"`
	// Decisions are every decision, in order: for a timeout the ones
	// collected until then.
	Decisions []TaskDecision `json:"decisions,omitempty"`
}

// TaskDecision is one decision of a task.
type TaskDecision struct {
	UserID   int64           `json:"userId"`
	Choice   string          `json:"choice,omitempty"`
	Decision json.RawMessage `json:"decision"`
	// DelegatedFrom is the user who delegated the decision to UserID.
	DelegatedFrom *int64    `json:"delegatedFrom,omitempty"`
	DecidedAt     time.Time `json:"decidedAt"`
}

// TaskDelegation is one delegation of a task: From passed their decision to
// To and lost it.
type TaskDelegation struct {
	From    int64
	To      int64
	Comment string
	At      time.Time
}

// Task is one human task with its assignees, delegations and decisions.
type Task struct {
	ID            uuid.UUID
	InstanceID    uuid.UUID
	Workflow      string
	Name          string
	N             int
	SubjectEntity string
	SubjectKey    string
	// Assignees are the task's assignees and those its escalation steps
	// added, in order; delegations are in Delegations.
	Assignees       []TaskAssignee
	Delegations     []TaskDelegation
	Required        int
	Outcomes        []string
	Veto            []string
	Escalation      []EscalationStep
	EscalationLevel int
	AllowDelegation bool
	// AssigneeKind, AssigneeUserID and AssigneePermission are the one
	// assignee of a task that has exactly one (compatibility); empty
	// otherwise. AssigneeUserID is set for a user, AssigneePermission for a
	// permission.
	AssigneeKind       string
	AssigneeUserID     int64
	AssigneePermission string
	DecisionSchema     json.RawMessage
	// Status is open, decided, expired (timeout) or cancelled.
	Status    string
	Choice    string
	Decision  json.RawMessage
	DecidedBy *int64
	Decisions []TaskDecision
	// DueAt is the due time of the current step.
	DueAt *time.Time
	// Title is the localized title a native workflow gave the task (empty for
	// package workflows).
	Title     manifest.Label
	CreatedAt time.Time
	DecidedAt *time.Time
}

// TaskAudience judges membership in role and group assignees by current
// rights. The core implements it over its roles and groups
// (pkg/extruntime/workflow/audience); an application with its own user model
// provides its own.
//
// While a task lock is held, the engine calls it with the transaction in the
// context (composables.WithTx); an implementation that reads the database
// must read through that transaction, not take another connection.
type TaskAudience interface {
	// IsMember reports whether the user, of the tenant, is covered by the
	// assignee now. A user, role or group of another tenant never is.
	IsMember(ctx context.Context, tenantID uuid.UUID, userID int64, a TaskAssignee) (bool, error)
	// Users lists current members (for notifications and previews), at most
	// limit.
	Users(ctx context.Context, tenantID uuid.UUID, a TaskAssignee, limit int) ([]int64, error)
}

// Task change kinds.
const (
	TaskChangeCreated   = "created"
	TaskChangeDecision  = "decision"
	TaskChangeDelegated = "delegated"
	TaskChangeEscalated = "escalated"
	TaskChangeClosed    = "closed"
)

// TaskChange is one lifecycle change of a task.
type TaskChange struct {
	Kind    string
	ActorID *int64
	// Assignees are the new assignees of created, delegated and escalated.
	Assignees []TaskAssignee
}

// TaskObserver follows the lifecycle of human tasks, in the transaction of
// the change (the task as it is after it); an error rolls the change back.
// The task model sends nothing itself: assignment events and notifications
// are observers.
type TaskObserver interface {
	TaskChanged(ctx context.Context, tx pgx.Tx, t Task, change TaskChange) error
}

// TaskObserverFunc adapts a function to TaskObserver.
type TaskObserverFunc func(ctx context.Context, tx pgx.Tx, t Task, change TaskChange) error

func (f TaskObserverFunc) TaskChanged(ctx context.Context, tx pgx.Tx, t Task, change TaskChange) error {
	return f(ctx, tx, t, change)
}

// TaskFilter narrows Tasks.
type TaskFilter struct {
	// Status defaults to open.
	Status []string
	// Overdue, when set, keeps tasks past their due time (true) or not (false).
	Overdue  *bool
	Workflow string
	// Limit is at most MaxTaskInbox; 0 means 50.
	Limit int
	// After is the cursor of the issue's contract: the ID of the last task of
	// the previous page. Its sort key is read as it is now, so a page whose
	// last task's due time moved since can skip or repeat tasks; Cursor does
	// not.
	After *uuid.UUID
	// Cursor, when set, is CursorAfter of the last task of the previous page:
	// its sort key as that page returned it. It takes precedence over After.
	Cursor *TaskCursor
}

// ---- validation ----

func invalidAssignee(format string, args ...any) error {
	return &pool.WireError{Kind: pool.ErrKindValidation, Code: "workflow.invalidAssignee", Message: fmt.Sprintf(format, args...)}
}

func invalidTask(format string, args ...any) error {
	return &pool.WireError{Kind: pool.ErrKindValidation, Code: "workflow.invalidTask", Message: fmt.Sprintf(format, args...)}
}

// taskRow is a validated TaskSpec: what a new human_tasks row holds.
type taskRow struct {
	subject         spi.ObjectRef
	assignees       []TaskAssignee
	required        int
	outcomes        []string
	veto            []string
	escalation      []EscalationStep
	allowDelegation bool
	schema          json.RawMessage
	title           json.RawMessage
	timeout         time.Duration
	hasTimeout      bool
}

// normalizeAssignee checks an assignee and returns its canonical form.
func (e *Engine) normalizeAssignee(a TaskAssignee) (TaskAssignee, error) {
	switch a.Kind {
	case TaskAssignUser:
		if a.UserID == nil || *a.UserID <= 0 || a.Ref != "" {
			return a, invalidAssignee("a user assignee needs a positive UserID and no Ref")
		}
		id := *a.UserID
		return TaskAssignee{Kind: a.Kind, UserID: &id}, nil
	case TaskAssignPermission:
		if a.Ref == "" || a.UserID != nil {
			return a, invalidAssignee("a permission assignee needs Ref and no UserID")
		}
		return TaskAssignee{Kind: a.Kind, Ref: a.Ref}, nil
	case TaskAssignRole, TaskAssignGroup:
		if a.UserID != nil {
			return a, invalidAssignee("a %s assignee needs Ref and no UserID", a.Kind)
		}
		var ref string
		if a.Kind == TaskAssignRole {
			id, err := strconv.ParseInt(a.Ref, 10, 64)
			if err != nil || id <= 0 {
				return a, invalidAssignee("role assignee %q is not a role id", a.Ref)
			}
			ref = strconv.FormatInt(id, 10)
		} else {
			id, err := uuid.Parse(a.Ref)
			if err != nil {
				return a, invalidAssignee("group assignee %q is not a group uuid", a.Ref)
			}
			ref = id.String()
		}
		if e.cfg.TaskAudience == nil {
			return a, invalidAssignee("%s assignees need a TaskAudience, and the engine has none", a.Kind)
		}
		return TaskAssignee{Kind: a.Kind, Ref: ref}, nil
	}
	return a, invalidAssignee("assignee kind %q is not supported", a.Kind)
}

func (e *Engine) normalizeAssignees(list []TaskAssignee, least int) ([]TaskAssignee, error) {
	if len(list) < least || len(list) > MaxTaskAssignees {
		return nil, invalidAssignee("a task needs %d to %d assignees, got %d", least, MaxTaskAssignees, len(list))
	}
	out := make([]TaskAssignee, 0, len(list))
	for _, a := range list {
		n, err := e.normalizeAssignee(a)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

// taskRowOf validates a TaskSpec of a native task: a zero Timeout is none.
func (e *Engine) taskRowOf(spec TaskSpec) (taskRow, error) {
	return e.taskRowWith(spec, spec.Timeout > 0)
}

// taskRowWith validates a TaskSpec whose timeout is set (hasTimeout, even
// when zero: the task is due at once) or absent.
func (e *Engine) taskRowWith(spec TaskSpec, hasTimeout bool) (taskRow, error) {
	assignees, err := e.normalizeAssignees(spec.Assignees, 1)
	if err != nil {
		return taskRow{}, err
	}
	row := taskRow{assignees: assignees, required: spec.Required, timeout: spec.Timeout, hasTimeout: hasTimeout, allowDelegation: spec.AllowDelegation}
	if spec.Timeout < 0 || spec.Timeout > MaxTaskDuration {
		return taskRow{}, invalidTask("a task timeout is 0 to %s", MaxTaskDuration)
	}
	if row.required == 0 {
		row.required = 1
	}
	if row.required < 1 || row.required > MaxTaskRequired {
		return taskRow{}, invalidTask("a task quorum is 1 to %d, got %d", MaxTaskRequired, spec.Required)
	}
	if len(spec.Outcomes) > MaxTaskOutcomes {
		return taskRow{}, invalidTask("a task has at most %d outcomes", MaxTaskOutcomes)
	}
	for i, o := range spec.Outcomes {
		if o == "" || slices.Contains(spec.Outcomes[:i], o) {
			return taskRow{}, invalidTask("task outcomes must be distinct and not empty")
		}
	}
	if len(spec.Outcomes) == 0 && row.required != 1 {
		return taskRow{}, invalidTask("a quorum of %d needs outcomes to agree on", row.required)
	}
	for i, v := range spec.Veto {
		if !slices.Contains(spec.Outcomes, v) || slices.Contains(spec.Veto[:i], v) {
			return taskRow{}, invalidTask("veto outcome %q is not one of the task outcomes", v)
		}
	}
	row.outcomes, row.veto = slices.Clone(spec.Outcomes), slices.Clone(spec.Veto)
	if len(spec.Escalation) > MaxTaskEscalation {
		return taskRow{}, invalidTask("a task has at most %d escalation steps", MaxTaskEscalation)
	}
	if len(spec.Escalation) > 0 && !row.hasTimeout {
		return taskRow{}, invalidTask("escalation steps need a task timeout")
	}
	for _, s := range spec.Escalation {
		if s.Extend < 0 || s.Extend > MaxTaskDuration {
			return taskRow{}, invalidTask("an escalation step extends by 0 to %s", MaxTaskDuration)
		}
		add, err := e.normalizeAssignees(s.Add, 0)
		if err != nil {
			return taskRow{}, err
		}
		row.escalation = append(row.escalation, EscalationStep{Add: add, Extend: s.Extend})
	}
	schema := spec.Decision
	if len(row.outcomes) > 0 {
		if schema == nil {
			schema = &manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{
				{Name: "outcome", Type: manifest.Type{Kind: manifest.KindEnum, Values: slices.Clone(row.outcomes)}},
			}}
		} else if err := checkOutcomeSchema(*schema, row.outcomes); err != nil {
			return taskRow{}, err
		}
	}
	if schema != nil {
		row.schema = marshal(schema)
	}
	if spec.Subject != nil {
		row.subject = *spec.Subject
	}
	if len(spec.Title) > 0 {
		row.title = marshal(spec.Title)
	}
	return row, nil
}

// checkOutcomeSchema requires a decision schema with an outcome field whose
// enum is exactly the outcomes.
func checkOutcomeSchema(t manifest.Type, outcomes []string) error {
	if t.Kind != manifest.KindObject {
		return invalidTask("a task with outcomes needs an object decision with an outcome field")
	}
	for _, p := range t.Properties {
		if p.Name != "outcome" {
			continue
		}
		if p.Type.Kind != manifest.KindEnum || p.Type.Optional || p.Type.Nullable || len(p.Type.Values) != len(outcomes) {
			break
		}
		// Same length, distinct values all among the (distinct) outcomes:
		// the two sets are equal, so every outcome (and veto) is reachable.
		for i, v := range p.Type.Values {
			if !slices.Contains(outcomes, v) || slices.Contains(p.Type.Values[:i], v) {
				return invalidTask("the decision's outcome enum must be exactly the task outcomes")
			}
		}
		return nil
	}
	return invalidTask("the decision's outcome field must be a required enum of exactly the task outcomes")
}

// choiceOf reads the outcome of a decision of a task with outcomes.
func choiceOf(outcomes []string, decision json.RawMessage) (string, error) {
	if len(outcomes) == 0 {
		return "", nil
	}
	var d struct {
		Outcome *string `json:"outcome"`
	}
	if err := json.Unmarshal(decision, &d); err != nil || d.Outcome == nil || !slices.Contains(outcomes, *d.Outcome) {
		return "", fmt.Errorf("%w: decision: outcome must be one of %s", ErrInvalid, strings.Join(outcomes, ", "))
	}
	return *d.Outcome, nil
}

// escalationJSON is the stored form of an escalation step.
type escalationJSON struct {
	Add []TaskAssignee `json:"add,omitempty"`
	// ExtendUs is Extend in microseconds (the precision of the due time),
	// rounded up so a positive extension never becomes an immediate expiry.
	ExtendUs int64 `json:"extendUs"`
}

func encodeEscalation(steps []EscalationStep) any {
	if len(steps) == 0 {
		return nil
	}
	out := make([]escalationJSON, len(steps))
	for i, s := range steps {
		us := s.Extend / time.Microsecond
		if s.Extend%time.Microsecond != 0 {
			us++
		}
		out[i] = escalationJSON{Add: s.Add, ExtendUs: int64(us)}
	}
	return []byte(marshal(out))
}

func decodeEscalation(raw []byte) ([]EscalationStep, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var in []escalationJSON
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, err
	}
	out := make([]EscalationStep, len(in))
	for i, s := range in {
		out[i] = EscalationStep{Add: s.Add, Extend: time.Duration(s.ExtendUs) * time.Microsecond}
	}
	return out, nil
}

// ---- reads ----

type taskQueryer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

const taskCols = `t.id, t.instance_id, i.workflow, t.name, t.n, t.subject_entity, t.subject_key, t.decision_schema, t.status, t.choice,
	t.decision, t.decided_by, t.due_at, t.title, t.created_at, t.decided_at, t.required, t.outcomes, t.veto, t.escalation,
	t.escalation_level, t.allow_delegation`

const taskFrom = ` FROM extruntime.human_tasks t JOIN extruntime.workflow_instances i ON i.id = t.instance_id AND i.tenant_id = t.tenant_id `

func scanTask(row pgx.Row) (Task, error) {
	var (
		t   Task
		esc []byte
	)
	err := row.Scan(&t.ID, &t.InstanceID, &t.Workflow, &t.Name, &t.N, &t.SubjectEntity, &t.SubjectKey, &t.DecisionSchema, &t.Status, &t.Choice,
		&t.Decision, &t.DecidedBy, &t.DueAt, &t.Title, &t.CreatedAt, &t.DecidedAt, &t.Required, &t.Outcomes, &t.Veto, &esc,
		&t.EscalationLevel, &t.AllowDelegation)
	if err != nil {
		return t, err
	}
	if t.Escalation, err = decodeEscalation(esc); err != nil {
		return t, fmt.Errorf("task %s has an unreadable escalation: %w", t.ID, err)
	}
	return t, nil
}

// queryTasks reads the tenant's tasks matching where ($1 is the tenant, args
// follow) with their assignees, delegations and decisions.
func (e *Engine) queryTasks(ctx context.Context, q taskQueryer, tenant uuid.UUID, where string, args ...any) ([]Task, error) {
	rows, err := q.Query(ctx, `SELECT `+taskCols+taskFrom+`WHERE t.tenant_id = $1 AND `+where, append([]any{tenant}, args...)...)
	if err != nil {
		return nil, serrors.FromDB(opManage, err)
	}
	tasks, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Task, error) { return scanTask(r) })
	if err != nil {
		return nil, serrors.FromDB(opManage, err)
	}
	return tasks, e.fillTasks(ctx, q, tenant, tasks)
}

// fillTasks loads the assignees, delegations and decisions of tasks.
func (e *Engine) fillTasks(ctx context.Context, q taskQueryer, tenant uuid.UUID, tasks []Task) error {
	if len(tasks) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(tasks))
	at := make(map[uuid.UUID]int, len(tasks))
	for i, t := range tasks {
		ids[i], at[t.ID] = t.ID, i
	}
	rows, err := q.Query(ctx, `
		SELECT task_id, kind, user_id, ref, delegated_from, comment, added_at FROM extruntime.human_task_assignees
		WHERE tenant_id = $1 AND task_id = ANY($2) ORDER BY added_at, id`, tenant, ids)
	if err != nil {
		return serrors.FromDB(opManage, err)
	}
	for rows.Next() {
		var (
			id    uuid.UUID
			a     TaskAssignee
			from  *int64
			note  string
			added time.Time
		)
		if err := rows.Scan(&id, &a.Kind, &a.UserID, &a.Ref, &from, &note, &added); err != nil {
			rows.Close()
			return serrors.FromDB(opManage, err)
		}
		t := &tasks[at[id]]
		if from != nil && a.UserID != nil {
			t.Delegations = append(t.Delegations, TaskDelegation{From: *from, To: *a.UserID, Comment: note, At: added})
			continue
		}
		t.Assignees = append(t.Assignees, a)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return serrors.FromDB(opManage, err)
	}
	rows, err = q.Query(ctx, `
		SELECT task_id, user_id, choice, decision, delegated_from, decided_at FROM extruntime.human_task_decisions
		WHERE tenant_id = $1 AND task_id = ANY($2) ORDER BY decided_at, user_id`, tenant, ids)
	if err != nil {
		return serrors.FromDB(opManage, err)
	}
	for rows.Next() {
		var (
			id uuid.UUID
			d  TaskDecision
		)
		if err := rows.Scan(&id, &d.UserID, &d.Choice, &d.Decision, &d.DelegatedFrom, &d.DecidedAt); err != nil {
			rows.Close()
			return serrors.FromDB(opManage, err)
		}
		d.Decision, d.DecidedAt = nonNull(d.Decision), d.DecidedAt.UTC()
		t := &tasks[at[id]]
		t.Decisions = append(t.Decisions, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return serrors.FromDB(opManage, err)
	}
	for i := range tasks {
		t := &tasks[i]
		if len(t.Assignees) != 1 {
			continue
		}
		switch a := t.Assignees[0]; a.Kind {
		case TaskAssignUser:
			t.AssigneeKind, t.AssigneeUserID = a.Kind, *a.UserID
		case TaskAssignPermission:
			t.AssigneeKind, t.AssigneePermission = a.Kind, a.Ref
		default:
			t.AssigneeKind = a.Kind
		}
	}
	return nil
}

// loadTask reads one task of the tenant; ErrNotFound when there is none.
func (e *Engine) loadTask(ctx context.Context, q taskQueryer, tenant, id uuid.UUID, lock bool) (Task, error) {
	where := `t.id = $2`
	if lock {
		where += ` FOR UPDATE OF t`
	}
	tasks, err := e.queryTasks(ctx, q, tenant, where, id)
	if err != nil {
		return Task{}, err
	}
	if len(tasks) == 0 {
		return Task{}, serrors.Wrap(opManage, fmt.Errorf("%w: task %s", ErrNotFound, id))
	}
	return tasks[0], nil
}

// ---- observers ----

// observeTask tells the observers about a change of a task, in tx.
func (e *Engine) observeTask(ctx context.Context, tx pgx.Tx, tenant, id uuid.UUID, change TaskChange) error {
	if len(e.cfg.TaskObservers) == 0 {
		return nil
	}
	t, err := e.loadTask(ctx, tx, tenant, id, false)
	if err != nil {
		return err
	}
	for _, o := range e.cfg.TaskObservers {
		if err := o.TaskChanged(ctx, tx, t, change); err != nil {
			return fmt.Errorf("extruntime workflow: task observer: %w", err)
		}
	}
	return nil
}

// ---- creation, escalation, timeout (in the run) ----

// taskState is the closed state of a task: decided, expired or cancelled.
type taskState struct {
	status    string
	choice    string
	decision  []byte
	decidedBy *int64
	decidedAt *time.Time
	required  int
	outcomes  []string
	decisions []TaskDecision
}

// createTask inserts a task, its assignees and tells the observers.
func (e *Engine) createTask(ctx context.Context, tx pgx.Tx, rs *runState, name string, n int, spec taskRow) (*time.Time, error) {
	var at *time.Time
	if spec.hasTimeout {
		var t time.Time
		if err := tx.QueryRow(ctx, `SELECT clock_timestamp() + $1::interval`, pgInterval(spec.timeout)).Scan(&t); err != nil {
			return nil, serrors.FromDB(opCall, err)
		}
		at = &t
	}
	// The compatibility columns name the one user or permission assignee.
	var (
		kind       string
		userID     *int64
		permission string
	)
	if len(spec.assignees) == 1 {
		switch a := spec.assignees[0]; a.Kind {
		case TaskAssignUser:
			kind, userID = a.Kind, a.UserID
		case TaskAssignPermission:
			kind, permission = a.Kind, a.Ref
		}
	}
	id := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO extruntime.human_tasks (id, tenant_id, instance_id, name, n, subject_entity, subject_key, assignee_kind, assignee_user_id, assignee_permission,
			decision_schema, due_at, title, required, outcomes, veto, escalation, allow_delegation)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)`,
		id, rs.inst.TenantID, rs.inst.ID, name, n, spec.subject.Entity, spec.subject.Key, kind, userID, permission,
		jsonArg(spec.schema), at, jsonArg(spec.title), spec.required, textArray(spec.outcomes), textArray(spec.veto), encodeEscalation(spec.escalation), spec.allowDelegation); err != nil {
		return nil, serrors.FromDB(opCall, err)
	}
	if err := insertAssignees(ctx, tx, rs.inst.TenantID, id, spec.assignees); err != nil {
		return nil, err
	}
	if err := e.observeTask(ctx, tx, rs.inst.TenantID, id, TaskChange{Kind: TaskChangeCreated, Assignees: spec.assignees}); err != nil {
		return nil, &host.InfraError{Err: err}
	}
	return at, nil
}

func textArray(s []string) any {
	if len(s) == 0 {
		return nil
	}
	return s
}

func insertAssignees(ctx context.Context, tx pgx.Tx, tenant, task uuid.UUID, list []TaskAssignee) error {
	for _, a := range list {
		if _, err := tx.Exec(ctx, `
			INSERT INTO extruntime.human_task_assignees (task_id, tenant_id, kind, user_id, ref, added_at) VALUES ($1, $2, $3, $4, $5, clock_timestamp())`,
			task, tenant, a.Kind, a.UserID, a.Ref); err != nil {
			return serrors.FromDB(opCall, err)
		}
	}
	return nil
}

// dueTask handles an open task whose due time passed, in the run's fenced
// transaction: it applies the next escalation step (assignees added, due
// time extended), or closes the task with a timeout. The applied step is a
// record of its own (human_task_escalations, keyed by task and step, apart
// from the workflow's occurrence keys) committed with the task change: a run
// that crashed before the commit applies it again, one that committed never
// does.
func (e *Engine) dueTask(ctx context.Context, tx pgx.Tx, rs *runState, taskID uuid.UUID, name string, n, level int, raw []byte) (bool, *time.Time, error) {
	steps, err := decodeEscalation(raw)
	if err != nil {
		return false, nil, internalErr("workflow.corruptJournal", "task %s#%d has an unreadable escalation", name, n)
	}
	if level < len(steps) {
		step := steps[level]
		tag, err := tx.Exec(ctx, `
			INSERT INTO extruntime.human_task_escalations (task_id, tenant_id, step, added, applied_at)
			VALUES ($1, $2, $3, $4, clock_timestamp()) ON CONFLICT (task_id, step) DO NOTHING`,
			taskID, rs.inst.TenantID, level, []byte(marshal(step.Add)))
		if err != nil {
			return false, nil, serrors.FromDB(opCall, err)
		}
		if tag.RowsAffected() == 0 {
			// The step and the level advance commit together under the task
			// row's lock: a recorded step with the level behind it is damage.
			return false, nil, internalErr("workflow.corruptJournal", "task %s#%d: escalation step %d is recorded but not applied", name, n, level)
		}
		if err := insertAssignees(ctx, tx, rs.inst.TenantID, taskID, step.Add); err != nil {
			return false, nil, err
		}
		var (
			open bool
			due  *time.Time
		)
		if step.Extend > 0 {
			var next time.Time
			if err := tx.QueryRow(ctx, `
				UPDATE extruntime.human_tasks SET escalation_level = $2, due_at = clock_timestamp() + $3::interval
				WHERE id = $1 RETURNING due_at`, taskID, level+1, pgInterval(step.Extend)).Scan(&next); err != nil {
				return false, nil, serrors.FromDB(opCall, err)
			}
			due, open = &next, true
		} else if _, err := tx.Exec(ctx, `UPDATE extruntime.human_tasks SET escalation_level = $2, status = 'expired' WHERE id = $1 AND status = 'open'`, taskID, level+1); err != nil {
			return false, nil, serrors.FromDB(opCall, err)
		}
		if err := e.observeTask(ctx, tx, rs.inst.TenantID, taskID, TaskChange{Kind: TaskChangeEscalated, Assignees: step.Add}); err != nil {
			return false, nil, &host.InfraError{Err: err}
		}
		if open {
			return true, due, nil
		}
	} else if _, err := tx.Exec(ctx, `UPDATE extruntime.human_tasks SET status = 'expired' WHERE id = $1 AND status = 'open'`, taskID); err != nil {
		// Decision vs timeout is a compare-and-set on the task status; the row
		// is locked, so the open status read before holds.
		return false, nil, serrors.FromDB(opCall, err)
	}
	if err := e.observeTask(ctx, tx, rs.inst.TenantID, taskID, TaskChange{Kind: TaskChangeClosed}); err != nil {
		return false, nil, &host.InfraError{Err: err}
	}
	return false, nil, nil
}

// closedState reads the closed state of a task and its decisions.
func closedState(ctx context.Context, tx pgx.Tx, taskID uuid.UUID) (taskState, error) {
	var st taskState
	if err := tx.QueryRow(ctx, `
		SELECT status, choice, decision, decided_by, decided_at, required, outcomes FROM extruntime.human_tasks WHERE id = $1`,
		taskID).Scan(&st.status, &st.choice, &st.decision, &st.decidedBy, &st.decidedAt, &st.required, &st.outcomes); err != nil {
		return st, serrors.FromDB(opCall, err)
	}
	rows, err := tx.Query(ctx, `
		SELECT user_id, choice, decision, delegated_from, decided_at FROM extruntime.human_task_decisions
		WHERE task_id = $1 ORDER BY decided_at, user_id`, taskID)
	if err != nil {
		return st, serrors.FromDB(opCall, err)
	}
	st.decisions, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (TaskDecision, error) {
		var d TaskDecision
		err := r.Scan(&d.UserID, &d.Choice, &d.Decision, &d.DelegatedFrom, &d.DecidedAt)
		d.Decision, d.DecidedAt = nonNull(d.Decision), d.DecidedAt.UTC()
		return d, err
	})
	return st, serrors.FromDB(opCall, err)
}

// taskRecord creates the task of a new occurrence, or reads the task of a
// waiting one (escalating or expiring it when its due time passed). render
// turns a closed task into the occurrence result; a state it does not render
// (false) is nondeterminism. The result is nil while the task is open.
func (e *Engine) taskRecord(ctx context.Context, rs *runState, name string, n int, spec taskRow, o Occurrence, ok bool, render func(taskState) (json.RawMessage, bool)) (json.RawMessage, *time.Time, error) {
	var result json.RawMessage
	var wake *time.Time
	err := e.inFence(ctx, rs, func(tx pgx.Tx) (func(), error) {
		var (
			taskID uuid.UUID
			status string
			due    *time.Time
			level  int
			esc    []byte
		)
		err := tx.QueryRow(ctx, `
			SELECT id, status, due_at, escalation_level, escalation FROM extruntime.human_tasks
			WHERE instance_id = $1 AND tenant_id = $2 AND name = $3 AND n = $4 FOR UPDATE`,
			rs.inst.ID, rs.inst.TenantID, name, n).Scan(&taskID, &status, &due, &level, &esc)
		if errors.Is(err, pgx.ErrNoRows) {
			at, err := e.createTask(ctx, tx, rs, name, n, spec)
			if err != nil {
				return nil, err
			}
			o = Occurrence{Name: name, N: n, Kind: KindTask, Status: OccWaiting, Attempts: 1, WakeAt: at}
			wake = at
		} else if err != nil {
			return nil, serrors.FromDB(opCall, err)
		} else {
			open := status == "open"
			if open && due != nil {
				var past bool
				if err := tx.QueryRow(ctx, `SELECT $1::timestamptz <= clock_timestamp()`, *due).Scan(&past); err != nil {
					return nil, serrors.FromDB(opCall, err)
				}
				if past {
					if open, due, err = e.dueTask(ctx, tx, rs, taskID, name, n, level, esc); err != nil {
						return nil, err
					}
				}
			}
			if open {
				wake = due
			} else {
				st, err := closedState(ctx, tx, taskID)
				if err != nil {
					return nil, err
				}
				r, rendered := render(st)
				if !rendered {
					return nil, rs.nondeterminism(name, n, KindTask, Occurrence{Name: name, N: n, Kind: KindTask, Status: "task " + st.status})
				}
				result = r
			}
			if !ok {
				o = Occurrence{Name: name, N: n, Kind: KindTask, Attempts: 1}
			}
			o.WakeAt = due
			if result != nil {
				o.Status, o.Result, o.WakeAt = OccCompleted, result, nil
			} else {
				o.Status = OccWaiting
			}
		}
		if err := putOccurrence(ctx, tx, rs, o); err != nil {
			return nil, err
		}
		return func() { rs.set(o) }, nil
	})
	return result, wake, err
}

// renderNativeTask is the TaskResult of a closed task.
func renderNativeTask(t taskState) (json.RawMessage, bool) {
	tr := TaskResult{Decisions: t.decisions}
	switch t.status {
	case "decided":
		tr.Outcome, tr.Choice, tr.Decision, tr.By = TaskDecided, t.choice, nonNull(t.decision), t.decidedBy
	case "expired":
		tr.Outcome = TaskTimeout
	case "cancelled":
		tr.Outcome = TaskCancelled
	default:
		return nil, false
	}
	return marshal(tr), true
}

// renderPackageTask is the wf.task result of a closed task:
// `{outcome:"decided", decision, by}` and `{outcome:"timeout"}` as before
// quorums existed, with `choice` and `decisions` added for a task with
// outcomes or a quorum (and `decisions` for a timeout that collected some).
func renderPackageTask(t taskState) (json.RawMessage, bool) {
	var out map[string]any
	switch t.status {
	case "decided":
		by := "0"
		if t.decidedBy != nil {
			by = strconv.FormatInt(*t.decidedBy, 10)
		}
		out = map[string]any{"outcome": "decided", "decision": nonNull(t.decision), "by": by}
		if t.choice != "" {
			out["choice"] = t.choice
		}
	case "expired":
		out = map[string]any{"outcome": "timeout"}
	default:
		return nil, false
	}
	if quorum := t.required > 1 || len(t.outcomes) > 0; quorum || (t.status == "expired" && len(t.decisions) > 0) {
		list := make([]map[string]any, 0, len(t.decisions))
		for _, d := range t.decisions {
			item := map[string]any{"by": strconv.FormatInt(d.UserID, 10), "decision": d.Decision, "decidedAt": d.DecidedAt.Format(time.RFC3339Nano)}
			if d.Choice != "" {
				item["choice"] = d.Choice
			}
			if d.DelegatedFrom != nil {
				item["delegatedFrom"] = strconv.FormatInt(*d.DelegatedFrom, 10)
			}
			list = append(list, item)
		}
		out["decisions"] = list
	}
	return marshal(out), true
}

// cancelTasks cancels the open tasks of an instance (only the task of the
// occurrence when one is given) and tells the observers.
func (e *Engine) cancelTasks(ctx context.Context, tx pgx.Tx, tenant, instance uuid.UUID, only *occKey) error {
	var (
		rows pgx.Rows
		err  error
	)
	if only != nil {
		rows, err = tx.Query(ctx, `
			UPDATE extruntime.human_tasks SET status = 'cancelled'
			WHERE instance_id = $1 AND tenant_id = $2 AND name = $3 AND n = $4 AND status = 'open' RETURNING id`, instance, tenant, only.name, only.n)
	} else {
		rows, err = tx.Query(ctx, `
			UPDATE extruntime.human_tasks SET status = 'cancelled'
			WHERE instance_id = $1 AND tenant_id = $2 AND status = 'open' RETURNING id`, instance, tenant)
	}
	if err != nil {
		return serrors.FromDB(opFinish, err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return serrors.FromDB(opFinish, err)
	}
	for _, id := range ids {
		if err := e.observeTask(ctx, tx, tenant, id, TaskChange{Kind: TaskChangeClosed}); err != nil {
			return err
		}
	}
	return nil
}

// ---- the right to decide ----

// rightToDecide reports whether the subject, with its current rights, may
// decide the task now, and the user whose delegation it decides for. A user
// who delegated the task lost the right, whatever else covers them.
func (e *Engine) rightToDecide(ctx context.Context, subj spi.Subject, t Task) (bool, *int64, error) {
	for _, d := range t.Delegations {
		if d.From == subj.UserID {
			return false, nil, nil
		}
	}
	for _, d := range t.Delegations {
		if d.To == subj.UserID {
			from := d.From
			return true, &from, nil
		}
	}
	ok, err := e.assigned(ctx, subj, t.Assignees)
	if err != nil || ok {
		return ok, nil, err
	}
	return e.cfg.TaskAdminPermission != "" && subj.Has(e.cfg.TaskAdminPermission), nil, nil
}

// assigned reports whether an assignee covers the subject now: users and
// permissions first, then roles and groups through the TaskAudience.
func (e *Engine) assigned(ctx context.Context, subj spi.Subject, list []TaskAssignee) (bool, error) {
	for _, a := range list {
		switch a.Kind {
		case TaskAssignUser:
			if a.UserID != nil && *a.UserID == subj.UserID {
				return true, nil
			}
		case TaskAssignPermission:
			if a.Ref != "" && subj.Has(a.Ref) {
				return true, nil
			}
		}
	}
	if e.cfg.TaskAudience == nil {
		return false, nil
	}
	for _, a := range list {
		if a.Kind != TaskAssignRole && a.Kind != TaskAssignGroup {
			continue
		}
		ok, err := e.cfg.TaskAudience.IsMember(ctx, subj.TenantID, subj.UserID, a)
		if err != nil {
			return false, serrors.Wrap(opManage, err)
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

func decided(t Task, user int64) bool {
	return slices.ContainsFunc(t.Decisions, func(d TaskDecision) bool { return d.UserID == user })
}

func describeAssignees(list []TaskAssignee) string {
	parts := make([]string, len(list))
	for i, a := range list {
		parts[i] = a.String()
	}
	return strings.Join(parts, ", ")
}

// currentSubject loads the actor's rights as they are now.
func (e *Engine) currentSubject(ctx context.Context, actor spi.Subject) (spi.Subject, error) {
	current, err := e.cfg.Subjects.LoadSubject(ctx, actor.TenantID, actor.UserID, spi.SubjectInitiator)
	if err != nil {
		return spi.Subject{}, serrors.Wrap(opManage, fmt.Errorf("%w: %v", ErrForbidden, err))
	}
	return current, nil
}

// ---- management ----

// DecideTask records the decision of one assignee. The actor must be
// covered by an assignee by their CURRENT rights (a user, a permission they
// hold, a role or group the TaskAudience counts them in), be the user a
// delegation passed the task to, or hold Config.TaskAdminPermission; a user
// who delegated the task may not. Each user decides once
// (ErrAlreadyDecided). The decision must match the task's schema and, for a
// task with outcomes, name one. The task closes when Required decisions
// have the same outcome, or at once on a veto outcome; a decision and the
// task's timeout race on a compare-and-set of the task status, so a late
// decision gets ErrTaskClosed.
func (e *Engine) DecideTask(ctx context.Context, actor spi.Subject, taskID uuid.UUID, decision json.RawMessage) error {
	// The schema, outcomes and quorum never change after creation: the
	// decision is validated against them before any lock is taken.
	t, err := e.loadTask(ctx, e.cfg.DB, actor.TenantID, taskID, false)
	if err != nil {
		return err
	}
	if t.Status != "open" {
		return serrors.Wrap(opManage, fmt.Errorf("%w: task %s is %s", ErrTaskClosed, taskID, t.Status))
	}
	if len(decision) > 0 && !json.Valid(decision) {
		return serrors.Wrap(opManage, fmt.Errorf("%w: decision is not valid JSON", ErrInvalid))
	}
	if len(t.DecisionSchema) > 0 {
		var typ manifest.Type
		if err := json.Unmarshal(t.DecisionSchema, &typ); err == nil {
			if verr := manifest.ValidateValue(typ, decision); verr != nil {
				return serrors.Wrap(opManage, fmt.Errorf("%w: decision: %v", ErrInvalid, verr))
			}
		}
	}
	choice, err := choiceOf(t.Outcomes, decision)
	if err != nil {
		return serrors.Wrap(opManage, err)
	}
	return retryConflicts(ctx, func() error {
		return e.inTx(ctx, func(tx pgx.Tx) error {
			return e.decideTx(ctx, tx, actor, t, decision, choice)
		})
	})
}

// authorizeLocked locks the task row and, only then, reads the task afresh
// (each statement after the lock sees what its previous holder committed)
// and judges the actor's right to decide by their rights as they are now: a
// membership revoked or a delegation committed while this waited for the
// lock counts. It returns the task and the user whose delegation the actor
// decides for (the earliest delegation to them, as a sequential call would).
func (e *Engine) authorizeLocked(ctx context.Context, tx pgx.Tx, actor spi.Subject, taskID uuid.UUID, verb string) (Task, *int64, error) {
	if _, err := lockTask(ctx, tx, actor.TenantID, taskID); err != nil {
		return Task{}, nil, err
	}
	// The checks below read through tx (see txContext): a call that holds the
	// task lock never waits for a second connection of the pool.
	ctx = txContext(ctx, tx)
	t, err := e.loadTask(ctx, tx, actor.TenantID, taskID, false)
	if err != nil {
		return Task{}, nil, err
	}
	if t.Status != "open" {
		return Task{}, nil, serrors.Wrap(opManage, fmt.Errorf("%w: task %s is %s", ErrTaskClosed, taskID, t.Status))
	}
	// A task past its final deadline is the run's to close as a timeout; one
	// with an escalation step pending still takes decisions (the run applies
	// the step, then judges the due time it sets).
	var late bool
	if err := tx.QueryRow(ctx, `SELECT `+pastFinalDeadline+` FROM extruntime.human_tasks t WHERE t.id = $1`, taskID).Scan(&late); err != nil {
		return Task{}, nil, serrors.FromDB(opManage, err)
	}
	if late {
		return Task{}, nil, serrors.Wrap(opManage, fmt.Errorf("%w: task %s is past its deadline", ErrTaskClosed, taskID))
	}
	if decided(t, actor.UserID) {
		return Task{}, nil, serrors.Wrap(opManage, fmt.Errorf("%w: user %d, task %s", ErrAlreadyDecided, actor.UserID, taskID))
	}
	current, err := e.currentSubject(ctx, actor)
	if err != nil {
		return Task{}, nil, err
	}
	ok, from, err := e.rightToDecide(ctx, current, t)
	if err != nil {
		return Task{}, nil, err
	}
	if !ok {
		return Task{}, nil, serrors.Wrap(opManage, fmt.Errorf("%w: user %d may not %s task %s (assigned to %s)", ErrForbidden, actor.UserID, verb, taskID, describeAssignees(t.Assignees)))
	}
	return t, from, nil
}

func (e *Engine) decideTx(ctx context.Context, tx pgx.Tx, actor spi.Subject, t Task, decision json.RawMessage, choice string) error {
	// The instance row is locked first, as every run write does (fence), and
	// the wake is decided under that lock: a run that is suspending commits its
	// wait before this proceeds, so the instance is `waiting` here and gets
	// woken; a run that has not suspended yet sees the decision when it does.
	if _, err := tx.Exec(ctx, `SELECT 1 FROM extruntime.workflow_instances WHERE id = $1 AND tenant_id = $2 FOR UPDATE`, t.InstanceID, actor.TenantID); err != nil {
		return serrors.FromDB(opManage, err)
	}
	_, from, err := e.authorizeLocked(ctx, tx, actor, t.ID, "decide")
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO extruntime.human_task_decisions (task_id, tenant_id, user_id, choice, decision, delegated_from, decided_at)
		VALUES ($1, $2, $3, $4, $5, $6, clock_timestamp()) ON CONFLICT (task_id, user_id) DO NOTHING`,
		t.ID, actor.TenantID, actor.UserID, choice, jsonArg(decision), from)
	if err != nil {
		return serrors.FromDB(opManage, err)
	}
	if tag.RowsAffected() == 0 {
		return serrors.Wrap(opManage, fmt.Errorf("%w: user %d, task %s", ErrAlreadyDecided, actor.UserID, t.ID))
	}
	var agree int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM extruntime.human_task_decisions WHERE task_id = $1 AND choice = $2`, t.ID, choice).Scan(&agree); err != nil {
		return serrors.FromDB(opManage, err)
	}
	actorID := actor.UserID
	if err := e.observeTask(ctx, tx, actor.TenantID, t.ID, TaskChange{Kind: TaskChangeDecision, ActorID: &actorID}); err != nil {
		return err
	}
	if agree < t.Required && !slices.Contains(t.Veto, choice) {
		return nil
	}
	if _, err := tx.Exec(ctx, `
		UPDATE extruntime.human_tasks SET status = 'decided', choice = $3, decision = $4, decided_by = $5, decided_at = now()
		WHERE id = $1 AND tenant_id = $2 AND status = 'open'`, t.ID, actor.TenantID, choice, jsonArg(decision), actor.UserID); err != nil {
		return serrors.FromDB(opManage, err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE extruntime.workflow_instances
		SET wake_at = CASE WHEN status = 'waiting' THEN now() ELSE wake_at END, updated_at = now()
		WHERE id = $1 AND tenant_id = $2`, t.InstanceID, actor.TenantID); err != nil {
		return serrors.FromDB(opManage, err)
	}
	if err := e.observeTask(ctx, tx, actor.TenantID, t.ID, TaskChange{Kind: TaskChangeClosed, ActorID: &actorID}); err != nil {
		return err
	}
	return notify(ctx, tx)
}

// DelegateTask passes the actor's right to decide an open task to one other
// user, who must be an active user of the same tenant: the actor loses it,
// the delegate decides for themselves with DelegatedFrom recorded. The task
// must allow delegation, the actor must have the right to decide (as for
// DecideTask) and not have decided. A chain is allowed; a delegation to a
// user who already delegated this task (back along the chain) or decided it
// is not (ErrDelegation). Rights and the target are judged under the task's
// lock.
func (e *Engine) DelegateTask(ctx context.Context, actor spi.Subject, taskID uuid.UUID, to int64, comment string) error {
	if len(comment) > maxDelegationComment {
		return serrors.Wrap(opManage, fmt.Errorf("%w: a delegation comment has at most %d bytes", ErrInvalid, maxDelegationComment))
	}
	return retryConflicts(ctx, func() error {
		return e.inTx(ctx, func(tx pgx.Tx) error {
			t, _, err := e.authorizeLocked(ctx, tx, actor, taskID, "delegate")
			if err != nil {
				return err
			}
			if !t.AllowDelegation {
				return serrors.Wrap(opManage, fmt.Errorf("%w: task %s does not allow delegation", ErrDelegation, taskID))
			}
			if to == actor.UserID || to <= 0 {
				return serrors.Wrap(opManage, fmt.Errorf("%w: delegate to another user", ErrDelegation))
			}
			for _, d := range t.Delegations {
				if d.From == to {
					return serrors.Wrap(opManage, fmt.Errorf("%w: user %d already delegated task %s", ErrDelegation, to, taskID))
				}
			}
			if decided(t, to) {
				return serrors.Wrap(opManage, fmt.Errorf("%w: user %d already decided task %s", ErrDelegation, to, taskID))
			}
			if _, err := e.cfg.Subjects.LoadSubject(txContext(ctx, tx), actor.TenantID, to, spi.SubjectInitiator); err != nil {
				return serrors.Wrap(opManage, fmt.Errorf("%w: user %d is not an active user of the tenant: %v", ErrDelegation, to, err))
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO extruntime.human_task_assignees (task_id, tenant_id, kind, user_id, delegated_from, comment, added_at)
				VALUES ($1, $2, 'user', $3, $4, $5, clock_timestamp())`, taskID, actor.TenantID, to, actor.UserID, comment); err != nil {
				return serrors.FromDB(opManage, err)
			}
			actorID, target := actor.UserID, to
			return e.observeTask(ctx, tx, actor.TenantID, taskID, TaskChange{
				Kind: TaskChangeDelegated, ActorID: &actorID, Assignees: []TaskAssignee{{Kind: TaskAssignUser, UserID: &target}},
			})
		})
	})
}

// txContext carries tx (composables.WithTx) to the SubjectLoader and the
// TaskAudience called while a task lock is held: an implementation that
// reads the database reads through it (the core's do), so the check costs no
// second connection and sees what the transaction sees.
func txContext(ctx context.Context, tx pgx.Tx) context.Context {
	return composables.WithTx(ctx, tx)
}

// lockTask locks a task row of the tenant and returns its status.
func lockTask(ctx context.Context, tx pgx.Tx, tenant, id uuid.UUID) (string, error) {
	var status string
	err := tx.QueryRow(ctx, `SELECT status FROM extruntime.human_tasks WHERE id = $1 AND tenant_id = $2 FOR UPDATE`, id, tenant).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", serrors.Wrap(opManage, fmt.Errorf("%w: task %s", ErrNotFound, id))
	}
	return status, serrors.FromDB(opManage, err)
}

// TaskCursor is the inbox position after a task: its sort key as the page
// returned it, so a task whose due time changes later moves no other task
// across pages.
type TaskCursor struct {
	DueAt *time.Time
	ID    uuid.UUID
}

// CursorAfter is the cursor after a task of a page.
func CursorAfter(t Task) TaskCursor { return TaskCursor{DueAt: t.DueAt, ID: t.ID} }

// String encodes the cursor as "<RFC 3339 due time>/<id>" ("/<id>" without a
// due time).
func (c TaskCursor) String() string {
	due := ""
	if c.DueAt != nil {
		due = c.DueAt.UTC().Format(time.RFC3339Nano)
	}
	return due + "/" + c.ID.String()
}

// ParseTaskCursor reads a TaskCursor.String.
func ParseTaskCursor(s string) (TaskCursor, error) {
	due, id, ok := strings.Cut(s, "/")
	if !ok {
		return TaskCursor{}, fmt.Errorf("%w: task cursor %q", ErrInvalid, s)
	}
	var c TaskCursor
	var err error
	if c.ID, err = uuid.Parse(id); err != nil {
		return TaskCursor{}, fmt.Errorf("%w: task cursor %q", ErrInvalid, s)
	}
	if due != "" {
		t, err := time.Parse(time.RFC3339Nano, due)
		if err != nil {
			return TaskCursor{}, fmt.Errorf("%w: task cursor %q", ErrInvalid, s)
		}
		c.DueAt = &t
	}
	return c, nil
}

// cursorOf is the cursor after a task of the tenant, from its current sort
// key; ErrNotFound for an unknown task.
func cursorOf(ctx context.Context, tx pgx.Tx, tenant, id uuid.UUID) (TaskCursor, error) {
	c := TaskCursor{ID: id}
	err := tx.QueryRow(ctx, `SELECT due_at FROM extruntime.human_tasks WHERE id = $1 AND tenant_id = $2`, id, tenant).Scan(&c.DueAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return TaskCursor{}, serrors.Wrap(opManage, fmt.Errorf("%w: cursor task %s", ErrNotFound, id))
	}
	return c, serrors.FromDB(opManage, err)
}

// pastFinalDeadline is true of an open task whose due time passed with no
// escalation step left: the run closes it as a timeout, and no decision is
// accepted meanwhile.
const pastFinalDeadline = `(t.due_at IS NOT NULL AND t.due_at <= clock_timestamp()
	AND t.escalation_level >= COALESCE(jsonb_array_length(t.escalation), 0))`

// Tasks is the actor's inbox: the tasks of the actor's tenant they may
// decide now by their current rights (an assignee covers them, or a
// delegation passed the task to them), which they neither decided nor
// delegated, ordered by due time (tasks without one last). An open task past
// its final deadline is not listed. Another status filter lists the tasks
// with those statuses that cover the actor. The task admin permission does
// not fill the inbox: it lets an operator decide a task found elsewhere.
// The candidates and the page are read from one snapshot, so every returned
// task matches the filters as of that read.
func (e *Engine) Tasks(ctx context.Context, actor spi.Subject, f TaskFilter) ([]Task, error) {
	if f.Limit <= 0 {
		f.Limit = defaultTaskInboxLimit
	}
	if f.Limit > MaxTaskInbox {
		f.Limit = MaxTaskInbox
	}
	statuses := f.Status
	if len(statuses) == 0 {
		statuses = []string{"open"}
	}
	tx, err := e.cfg.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, serrors.FromDB(opManage, err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	// The actor's rights come from the same snapshot as the tasks.
	tctx := txContext(ctx, tx)
	current, err := e.currentSubject(tctx, actor)
	if err != nil {
		return nil, err
	}
	out, err := e.inbox(tctx, tx, current, statuses, f)
	if err != nil {
		return nil, err
	}
	return out, serrors.FromDB(opManage, tx.Commit(ctx))
}

func (e *Engine) inbox(ctx context.Context, tx pgx.Tx, current spi.Subject, statuses []string, f TaskFilter) ([]Task, error) {
	perms := current.Permissions
	if perms == nil {
		perms = []string{}
	}
	audience := e.cfg.TaskAudience != nil
	batch := max(f.Limit*2, 50)
	cursor := f.Cursor
	if cursor == nil && f.After != nil {
		c, err := cursorOf(ctx, tx, current.TenantID, *f.After)
		if err != nil {
			return nil, err
		}
		cursor = &c
	}
	var ids []uuid.UUID
	for len(ids) < f.Limit {
		var (
			hasCursor bool
			afterDue  *time.Time
			afterID   uuid.UUID
		)
		if cursor != nil {
			hasCursor, afterDue, afterID = true, cursor.DueAt, cursor.ID
		}
		rows, err := tx.Query(ctx, `
			SELECT t.id, t.due_at,
				EXISTS (SELECT 1 FROM extruntime.human_task_assignees a WHERE a.task_id = t.id
					AND ((a.kind = 'user' AND a.user_id = $2) OR (a.kind = 'permission' AND a.ref = ANY($3))))
			`+taskFrom+`
			WHERE t.tenant_id = $1 AND t.status = ANY($4)
			  AND ($5 = '' OR i.workflow = $5)
			  AND ($6::boolean IS NULL OR (t.due_at IS NOT NULL AND t.due_at < now()) = $6)
			  AND (NOT $7 OR (COALESCE(t.due_at, 'infinity'::timestamptz), t.id) > (COALESCE($8::timestamptz, 'infinity'::timestamptz), $9::uuid))
			  AND (t.status <> 'open' OR NOT `+pastFinalDeadline+`)
			  AND NOT EXISTS (SELECT 1 FROM extruntime.human_task_assignees a WHERE a.task_id = t.id AND a.delegated_from = $2)
			  AND (t.status <> 'open' OR NOT EXISTS (SELECT 1 FROM extruntime.human_task_decisions d WHERE d.task_id = t.id AND d.user_id = $2))
			  AND EXISTS (SELECT 1 FROM extruntime.human_task_assignees a WHERE a.task_id = t.id
					AND ((a.kind = 'user' AND a.user_id = $2) OR (a.kind = 'permission' AND a.ref = ANY($3))
					     OR (a.kind IN ('role', 'group') AND a.delegated_from IS NULL AND $10)))
			ORDER BY COALESCE(t.due_at, 'infinity'::timestamptz), t.id
			LIMIT $11`,
			current.TenantID, current.UserID, perms, statuses, f.Workflow, f.Overdue, hasCursor, afterDue, afterID, audience, batch)
		if err != nil {
			return nil, serrors.FromDB(opManage, err)
		}
		type candidate struct {
			id     uuid.UUID
			due    *time.Time
			direct bool
		}
		cands, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (candidate, error) {
			var c candidate
			err := r.Scan(&c.id, &c.due, &c.direct)
			return c, err
		})
		if err != nil {
			return nil, serrors.FromDB(opManage, err)
		}
		var indirect []uuid.UUID
		for _, c := range cands {
			if !c.direct {
				indirect = append(indirect, c.id)
			}
		}
		covered := map[uuid.UUID]bool{}
		if len(indirect) > 0 {
			if covered, err = e.coveredByAudience(ctx, tx, current, indirect); err != nil {
				return nil, err
			}
		}
		for _, c := range cands {
			if len(ids) == f.Limit {
				break
			}
			if c.direct || covered[c.id] {
				ids = append(ids, c.id)
			}
		}
		if len(cands) < batch {
			break
		}
		last := cands[len(cands)-1]
		cursor = &TaskCursor{DueAt: last.due, ID: last.id}
	}
	if len(ids) == 0 {
		return []Task{}, nil
	}
	return e.queryTasks(ctx, tx, current.TenantID, `t.id = ANY($2) ORDER BY COALESCE(t.due_at, 'infinity'::timestamptz), t.id`, ids)
}

// coveredByAudience reports which tasks a role or group assignee covers the
// subject in, asking the TaskAudience once per distinct assignee.
func (e *Engine) coveredByAudience(ctx context.Context, q taskQueryer, subj spi.Subject, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	rows, err := q.Query(ctx, `
		SELECT task_id, kind, ref FROM extruntime.human_task_assignees
		WHERE tenant_id = $1 AND task_id = ANY($2) AND kind IN ('role', 'group') AND delegated_from IS NULL`, subj.TenantID, ids)
	if err != nil {
		return nil, serrors.FromDB(opManage, err)
	}
	type ref struct {
		task uuid.UUID
		a    TaskAssignee
	}
	refs, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (ref, error) {
		var x ref
		err := r.Scan(&x.task, &x.a.Kind, &x.a.Ref)
		return x, err
	})
	if err != nil {
		return nil, serrors.FromDB(opManage, err)
	}
	member := map[TaskAssignee]bool{}
	out := map[uuid.UUID]bool{}
	for _, r := range refs {
		ok, seen := member[r.a]
		if !seen {
			if ok, err = e.cfg.TaskAudience.IsMember(ctx, subj.TenantID, subj.UserID, r.a); err != nil {
				return nil, serrors.Wrap(opManage, err)
			}
			member[r.a] = ok
		}
		if ok {
			out[r.task] = true
		}
	}
	return out, nil
}

// ---- native and package entry points ----

func (r *nativeRun) Task(ctx context.Context, name string, n int, spec TaskSpec) (TaskResult, error) {
	o, ok, err := r.lookup(ctx, name, n, KindTask)
	if err != nil {
		return TaskResult{}, err
	}
	switch {
	case ok && o.Status == OccCancelled:
		// The occurrence was cancelled with its task: the result carries the
		// decisions recorded before, read from the task on every call.
		decisions, err := r.e.cancelledDecisions(ctx, r.rs, name, n)
		if err != nil {
			return TaskResult{}, r.exit(err)
		}
		return TaskResult{Outcome: TaskCancelled, Decisions: decisions}, nil
	case ok && o.Status == OccCompleted:
		var tr TaskResult
		if err := json.Unmarshal(o.Result, &tr); err != nil {
			return TaskResult{}, internalErr("workflow.corruptJournal", "occurrence %s#%d has an unreadable result", name, n)
		}
		return tr, nil
	case ok && o.Status != OccWaiting:
		return TaskResult{}, r.rs.nondeterminism(name, n, KindTask, o)
	}
	// The spec makes a new task; the task of a waiting occurrence exists and
	// is read as it was created, whatever the spec says now.
	var row taskRow
	if !ok {
		if row, err = r.e.taskRowOf(spec); err != nil {
			return TaskResult{}, err
		}
	}
	result, _, err := r.e.taskRecord(ctx, r.rs, name, n, row, o, ok, renderNativeTask)
	if err != nil {
		return TaskResult{}, r.exit(err)
	}
	if result == nil {
		return TaskResult{}, suspended("task", name, n)
	}
	var tr TaskResult
	if err := json.Unmarshal(result, &tr); err != nil {
		return TaskResult{}, internalErr("workflow.corruptJournal", "occurrence %s#%d has an unreadable result", name, n)
	}
	return tr, nil
}

// cancelledDecisions reads the decisions of the task of a cancelled
// occurrence (none when it never had a task).
func (e *Engine) cancelledDecisions(ctx context.Context, rs *runState, name string, n int) ([]TaskDecision, error) {
	rows, err := e.cfg.DB.Query(ctx, `
		SELECT d.user_id, d.choice, d.decision, d.delegated_from, d.decided_at
		FROM extruntime.human_tasks t JOIN extruntime.human_task_decisions d ON d.task_id = t.id
		WHERE t.instance_id = $1 AND t.tenant_id = $2 AND t.name = $3 AND t.n = $4
		ORDER BY d.decided_at, d.user_id`, rs.inst.ID, rs.inst.TenantID, name, n)
	if err != nil {
		return nil, &host.InfraError{Err: serrors.FromDB(opNative, err)}
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (TaskDecision, error) {
		var d TaskDecision
		err := r.Scan(&d.UserID, &d.Choice, &d.Decision, &d.DelegatedFrom, &d.DecidedAt)
		d.Decision, d.DecidedAt = nonNull(d.Decision), d.DecidedAt.UTC()
		return d, err
	})
	if err != nil {
		return nil, &host.InfraError{Err: serrors.FromDB(opNative, err)}
	}
	return out, nil
}

// taskAssignee is the wire form of a package task's assignee: a user
// (`{entity: "core.user", key}`, the key a user id), the holders of a
// permission (`{permission}`, a permission declared in the catalog), the
// members of a role (`{role}`, a role id) or of a group (`{group}`, a group
// uuid).
type taskAssignee struct {
	Entity     string `json:"entity"`
	Key        string `json:"key"`
	Permission string `json:"permission"`
	Role       string `json:"role"`
	Group      string `json:"group"`
}

func (a taskAssignee) spec() (TaskAssignee, error) {
	named := countSet(a.Key != "" || a.Entity != "", a.Permission != "", a.Role != "", a.Group != "")
	if named != 1 {
		return TaskAssignee{}, invalidAssignee("a task assignee names exactly one of a user, a permission, a role or a group")
	}
	switch {
	case a.Permission != "":
		return TaskAssignee{Kind: TaskAssignPermission, Ref: a.Permission}, nil
	case a.Role != "":
		return TaskAssignee{Kind: TaskAssignRole, Ref: a.Role}, nil
	case a.Group != "":
		return TaskAssignee{Kind: TaskAssignGroup, Ref: a.Group}, nil
	}
	// A user is a core.user reference; a bare key (no entity) is read as a
	// user id, as wf.task always did.
	if a.Entity != "" && a.Entity != "core.user" {
		return TaskAssignee{}, invalidAssignee("task assignee entity %q is not core.user", a.Entity)
	}
	id, err := strconv.ParseInt(a.Key, 10, 64)
	if err != nil {
		return TaskAssignee{}, invalidAssignee("task assignee %q is not a user id", a.Key)
	}
	return TaskAssignee{Kind: TaskAssignUser, UserID: &id}, nil
}

// checkDeclared requires the permissions a new package task names to be
// declared in the catalog as it is now.
func (e *Engine) checkDeclared(ctx context.Context, row taskRow) error {
	lists := [][]TaskAssignee{row.assignees}
	for _, s := range row.escalation {
		lists = append(lists, s.Add)
	}
	for _, list := range lists {
		for _, a := range list {
			if a.Kind != TaskAssignPermission {
				continue
			}
			cat := e.catalog()
			if cat == nil {
				return invalidAssignee("task assignee permission %q cannot be checked without a catalog", a.Ref)
			}
			res, found, err := cat.Resolve(ctx, a.Ref)
			if err != nil {
				return serrors.Wrap(opCall, err)
			}
			if !found || res.Resource.Kind != manifest.ResPermission {
				return invalidAssignee("task assignee permission %q is not declared by any active package", a.Ref)
			}
		}
	}
	return nil
}

func (e *Engine) wfTask(ctx context.Context, rs *runState, args json.RawMessage) (json.RawMessage, error) {
	var a wfTaskArgs
	if err := decodeArgs(args, &a); err != nil {
		return nil, err
	}
	o, ok := rs.get(a.Name, a.N)
	if ok {
		if o.Kind != KindTask || (o.Status != OccWaiting && !o.terminal()) {
			return nil, rs.nondeterminism(a.Name, a.N, KindTask, o)
		}
		if o.terminal() {
			return terminalReply(o)
		}
	}
	// The arguments make a new task; the task of a waiting occurrence exists
	// and is read as it was created.
	var row taskRow
	if !ok {
		var err error
		if row, err = e.packageTaskRow(ctx, a); err != nil {
			return nil, err
		}
	}
	result, wake, err := e.taskRecord(ctx, rs, a.Name, a.N, row, o, ok, renderPackageTask)
	if err != nil {
		return nil, err
	}
	if result != nil {
		return result, nil
	}
	return nil, suspendError("task:"+a.Name, wake)
}

// wfTaskArgs are the arguments of wf.task.
type wfTaskArgs struct {
	Name       string         `json:"name"`
	N          int            `json:"n"`
	Subject    spi.ObjectRef  `json:"subject"`
	Assignee   *taskAssignee  `json:"assignee"`
	Assignees  []taskAssignee `json:"assignees"`
	Required   int            `json:"required"`
	Outcomes   []string       `json:"outcomes"`
	Veto       []string       `json:"veto"`
	Escalation []struct {
		Add    []taskAssignee `json:"add"`
		Extend string         `json:"extend"`
	} `json:"escalation"`
	AllowDelegation bool            `json:"allowDelegation"`
	Decision        json.RawMessage `json:"decision"`
	Timeout         string          `json:"timeout"`
}

// packageTaskRow validates the wf.task arguments of a new task; the
// permissions it names are checked against the catalog as it is now.
func (e *Engine) packageTaskRow(ctx context.Context, a wfTaskArgs) (taskRow, error) {
	spec := TaskSpec{Required: a.Required, Outcomes: a.Outcomes, Veto: a.Veto, AllowDelegation: a.AllowDelegation, Subject: &a.Subject}
	if a.Timeout != "" {
		d, err := parseDuration(a.Timeout)
		if err != nil {
			return taskRow{}, &pool.WireError{Kind: pool.ErrKindValidation, Code: "workflow.invalidDuration", Message: err.Error()}
		}
		spec.Timeout = d
	}
	wire := a.Assignees
	if a.Assignee != nil {
		if len(wire) > 0 {
			return taskRow{}, invalidAssignee("a task names either assignee or assignees")
		}
		wire = []taskAssignee{*a.Assignee}
	}
	conv := func(list []taskAssignee) ([]TaskAssignee, error) {
		out := make([]TaskAssignee, 0, len(list))
		for _, w := range list {
			s, err := w.spec()
			if err != nil {
				return nil, err
			}
			out = append(out, s)
		}
		return out, nil
	}
	var err error
	if spec.Assignees, err = conv(wire); err != nil {
		return taskRow{}, err
	}
	for _, s := range a.Escalation {
		step := EscalationStep{}
		if step.Add, err = conv(s.Add); err != nil {
			return taskRow{}, err
		}
		if s.Extend != "" {
			if step.Extend, err = parseDuration(s.Extend); err != nil {
				return taskRow{}, &pool.WireError{Kind: pool.ErrKindValidation, Code: "workflow.invalidDuration", Message: err.Error()}
			}
		}
		spec.Escalation = append(spec.Escalation, step)
	}
	if len(a.Decision) > 0 && string(a.Decision) != "null" {
		var typ manifest.Type
		if err := json.Unmarshal(a.Decision, &typ); err != nil {
			return taskRow{}, invalidTask("the decision type is not a type: %v", err)
		}
		spec.Decision = &typ
	}
	// A timeout the workflow wrote, even "0s", makes the task due; none
	// leaves it open until decided (as wf.task always did).
	row, err := e.taskRowWith(spec, a.Timeout != "")
	if err != nil {
		return taskRow{}, err
	}
	// The decision type of a package task is stored as the workflow wrote it.
	if len(a.Decision) > 0 && string(a.Decision) != "null" {
		row.schema = a.Decision
	}
	if err := e.checkDeclared(ctx, row); err != nil {
		return taskRow{}, err
	}
	return row, nil
}
