// Package tasks is the inbox of a person's automation tasks: the human nodes of automation runs (#1336)
// assigned to the person, to their roles, groups or permissions, or delegated to them, read from the
// human tasks substrate of the workflow engine (#1327). There is no table of its own: a task is a
// workflow.Task, and what the node asked for (instructions, buttons, the form) comes from the run that
// opened it and the graph version the run is pinned to.
//
// TaskInbox is the service the pages call. TaskForm is the extension point of a node whose decision has
// its own procedure (a signature in the browser): a module or connector contributes one with
// ContributeTaskForm, and a human node names it in its form with the annotation `"x-iota-form": "<kind>"`.
package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/a-h/templ"
	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
)

// Scopes of TaskFilter.
const (
	// ScopeMine are the open tasks the user may decide now (the default).
	ScopeMine = "mine"
	// ScopeOverdue are the open tasks of ScopeMine past their due time.
	ScopeOverdue = "overdue"
	// ScopeDecidedByMe are the tasks the user decided, newest decision first.
	ScopeDecidedByMe = "decided_by_me"
)

// Page sizes of TaskFilter.Limit.
const (
	DefaultLimit = 30
	MaxLimit     = 100
)

// How a task reached the user (TaskView.AssignedVia).
const (
	ViaUser       = "user"
	ViaRole       = "role"
	ViaGroup      = "group"
	ViaPermission = "permission"
	ViaDelegated  = "delegated"
)

// States of a task (TaskView.State).
const (
	StateOpen      = "open"
	StateDecided   = "decided"
	StateExpired   = "expired"
	StateCancelled = "cancelled"
)

// Tones of an outcome button (OutcomeOption.Style).
const (
	StylePrimary = "primary"
	StyleDanger  = "danger"
	StyleDefault = "default"
)

// FormAnnotation is the keyword of a human node's form that names the TaskForm deciding it.
const FormAnnotation = "x-iota-form"

// MaxCommentLength bounds a decision's comment (the human node's decision schema).
const MaxCommentLength = 4000

// TaskFilter selects a page of the inbox.
type TaskFilter struct {
	Scope  string // ScopeMine (default) | ScopeOverdue | ScopeDecidedByMe
	Search string // by the task's title and the label of its subject
	Cursor string // NextCursor of the previous page
	Limit  int    // 1..MaxLimit, default DefaultLimit
}

// SubjectRef is the record a task is about. URL is set only when the user may read the record.
type SubjectRef struct {
	Entity, Key, Label, URL string
}

// OutcomeOption is one decision button of a task.
type OutcomeOption struct {
	Value, Label    string
	Style           string // StylePrimary | StyleDanger | StyleDefault
	CommentRequired bool
}

// UserRef names a user.
type UserRef struct {
	ID   uint
	Name string
}

// Quorum is how many decisions a task needs and has.
type Quorum struct {
	Required, Decided int
}

// Decision is one decision of a task.
type Decision struct {
	By            UserRef
	Outcome       string
	Comment       string
	At            time.Time
	DelegatedFrom *UserRef
}

// TaskView is a task as the user sees it.
type TaskView struct {
	ID           uuid.UUID
	RunID        uuid.UUID
	AutomationID uuid.UUID
	NodeID       string
	Title        string          // the node's rendered title
	Instructions string          // the node's rendered text (markdown, shown without HTML)
	Subject      *SubjectRef     // nil for a run without a subject
	Outcomes     []OutcomeOption // the node's buttons
	// DecisionSchema is the JSON Schema of the decision's fields; nil when the node declares none.
	DecisionSchema json.RawMessage
	// FormKind is "" for the default form, else the TaskForm.Kind the node names.
	FormKind string
	// FormUnavailable: the node names a form no TaskForm of this application provides; the task cannot
	// be decided here.
	FormUnavailable bool
	AssignedVia     string // ViaUser | ViaRole | ViaGroup | ViaPermission | ViaDelegated
	DelegatedBy     *UserRef
	AllowDelegation bool
	// CanDecide: the task is open, the user may decide it and has not.
	CanDecide bool
	Quorum    Quorum
	DueAt     *time.Time
	Overdue   bool
	// PastDeadline: the task is still open but past its final deadline; it takes no decision and the
	// run closes it as expired.
	PastDeadline bool
	// Deadline is the final deadline of an open task: after it the task takes no decision. Nil when the
	// task has no due time or an escalation step is still pending.
	Deadline  *time.Time
	Escalated bool
	State     string // StateOpen | StateDecided | StateExpired | StateCancelled
	// Outcome is the outcome a decided task closed with.
	Outcome string
	// ClosedBy and ClosedAt are who closed a decided task and when (the due time of an expired one).
	ClosedBy   *UserRef
	ClosedAt   *time.Time
	MyDecision *string
	Decisions  []Decision
	CreatedAt  time.Time
}

// OutcomeLabel is the label of an outcome of the task (the value itself when it is not a button).
func (t TaskView) OutcomeLabel(value string) string {
	for _, o := range t.Outcomes {
		if o.Value == value {
			return o.Label
		}
	}
	return value
}

// TaskPage is one page of the inbox.
type TaskPage struct {
	Items      []TaskView
	NextCursor string
}

// TaskCounts are the user's open and overdue tasks.
type TaskCounts struct {
	Open, Overdue int
}

// DecideInput is a decision.
type DecideInput struct {
	Outcome string
	Comment string
	Fields  json.RawMessage // by DecisionSchema
	Form    json.RawMessage // the data of the task's own form; only when FormKind != ""
}

// TaskInbox is the inbox of the user of the context. Decide and Delegate take only an interactive
// session of a person (WithOrigin(ctx, engine.OriginHuman)); any other origin gets ErrHumanRequired.
type TaskInbox interface {
	List(ctx context.Context, f TaskFilter) (TaskPage, error)
	Get(ctx context.Context, id uuid.UUID) (TaskView, error)
	Counts(ctx context.Context) (TaskCounts, error)
	Decide(ctx context.Context, id uuid.UUID, in DecideInput) error
	Delegate(ctx context.Context, id uuid.UUID, to uint, comment string) error
}

// TaskForm is the decision form of a kind of task, in place of the default buttons.
type TaskForm interface {
	// Kind is unique among the application's forms; a second form of a kind fails the composition.
	Kind() string
	// Render is the form in place of the default buttons; it may load its own script.
	Render(ctx context.Context, t TaskView) templ.Component
	// Decide checks the form's data and performs the action without which there is no decision (sending
	// a signature, say), and returns the outcome and the fields of the decision. It runs outside any
	// transaction; an error records no decision and the task stays open. An external effect inside it
	// must be safe to repeat: the form deduplicates it.
	//
	// Before Decide runs, the inbox refuses a submission the task would refuse: a comment too long, and,
	// for a form that tells it (FormPreparer, FormIntent), an outcome that is no button or lacks its
	// comment, fields that break the schema. For a task with a final deadline (TaskView.Deadline) Decide
	// starts only before it, and ctx ends at it. The action and the vote cannot be one transaction: an
	// action that completes just as the deadline passes may find the vote refused (ErrTaskClosed) and
	// the task expiring; its effect then stands, and a repeat of it is deduplicated.
	Decide(ctx context.Context, t TaskView, actor user.User, form json.RawMessage) (FormDecision, error)
}

// FormPreparer is an optional interface of a TaskForm: the decision its data asks for, prepared without
// the form's action (no external effect). The inbox validates it as the decision (the outcome among the
// buttons, the comment, the fields against the node's schema) before Decide performs the action; Decide
// must decide as Prepare did.
type FormPreparer interface {
	Prepare(ctx context.Context, t TaskView, actor user.User, form json.RawMessage) (FormDecision, error)
}

// FormIntent is an optional interface of a TaskForm: the outcome its data asks for, read without the
// form's action. A user who already decided submits the form again (a double click, a lost answer):
// the same outcome is that decision, another is ErrAlreadyDecided. A form without it cannot tell, so
// a second submit of it is ErrAlreadyDecided.
type FormIntent interface {
	Intent(ctx context.Context, t TaskView, form json.RawMessage) (outcome string, err error)
}

// FormDecision is what a TaskForm decided.
type FormDecision struct {
	Outcome string
	// Fields are the decision's fields: nodes.<id>.output.decisions[].fields.
	Fields json.RawMessage
}

// FormError is an error of TaskForm.Decide the person reads: Message is localized for them (the
// signature was refused, a certificate expired). Any other error of a form shows a generic phrase.
type FormError struct {
	Message string
	Err     error
}

func (e *FormError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *FormError) Unwrap() error { return e.Err }

// Errors of TaskInbox.
var (
	// ErrTaskNotFound is also the error of a task of another user or tenant.
	ErrTaskNotFound = errors.New("task not found")
	// ErrTaskClosed: the task is decided, expired or cancelled.
	ErrTaskClosed = errors.New("task is closed")
	// ErrAlreadyDecided: the user decided the task with another outcome.
	ErrAlreadyDecided = errors.New("already decided by you")
	// ErrInvalidDecision: an outcome not among the buttons, fields that break the schema, a missing comment.
	ErrInvalidDecision = errors.New("invalid decision")
	// ErrDelegationDenied: the node does not allow delegation, or not to that user (self, inactive, one
	// who already delegated or decided the task).
	ErrDelegationDenied = errors.New("delegation not allowed")
	// ErrFormUnavailable: the task's form is not provided by any TaskForm of the application.
	ErrFormUnavailable = errors.New("task form unavailable")
	// ErrHumanRequired: a decision or a delegation came from an agent or an API token.
	ErrHumanRequired = engine.ErrHumanRequired
)

type originKey struct{}

// WithOrigin marks who acts in ctx: engine.OriginHuman for an interactive session of a person.
func WithOrigin(ctx context.Context, o engine.Origin) context.Context {
	return context.WithValue(ctx, originKey{}, o)
}

// OriginFrom is the origin WithOrigin set; engine.OriginAgent when none was.
func OriginFrom(ctx context.Context) engine.Origin {
	if o, ok := ctx.Value(originKey{}).(engine.Origin); ok {
		return o
	}
	return engine.OriginAgent
}
