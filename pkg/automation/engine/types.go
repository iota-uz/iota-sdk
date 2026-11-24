package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
)

// WorkflowID is the workflow contract a run is an instance of.
const WorkflowID = "automation.run"

// ConsumerID is the receipt key of the engine's one outbox consumer.
const ConsumerID = "automations"

// SubjectSignal wakes a resumed, superseded run that waits for its conversation subject.
const SubjectSignal = "subject"

// InboxSignal is the signal of automation.run every input of a run arrives on: a matched event, the
// result of a call or a task.
const InboxSignal = "inbox"

// RunStatus is the status of a run as the API shows it. It is derived from the status of the run's
// workflow instance, or from start_state for a run that has no instance.
type RunStatus string

const (
	RunQueued       RunStatus = "queued"        // conversation subject busy, no instance yet
	RunRunning      RunStatus = "running"       // instance running
	RunWaiting      RunStatus = "waiting"       // instance waiting (wait, timer, task, call)
	RunBlocked      RunStatus = "blocked"       // instance blocked, a call of it blocked (unknown outcome), or start_state=blocked (no owner, depth)
	RunDone         RunStatus = "done"          // instance completed
	RunFailed       RunStatus = "failed"        // instance failed
	RunCancelled    RunStatus = "cancelled"     // instance terminated, or a queued run of a disabled graph
	RunFilterFailed RunStatus = "filter_failed" // the trigger's CEL filter failed; no run started
)

// Start states of a run row without an instance.
const (
	startQueued       = "queued"
	startFilterFailed = "filter_failed"
	startBlocked      = "blocked"
	startCancelled    = "cancelled"
)

// StatusOf maps workflow.Status* of the instance (or start_state when there is no instance).
func StatusOf(instanceStatus string, startState string) RunStatus {
	switch instanceStatus {
	case workflow.StatusRunning:
		return RunRunning
	case workflow.StatusWaiting:
		return RunWaiting
	case workflow.StatusBlocked:
		return RunBlocked
	case workflow.StatusCompleted:
		return RunDone
	case workflow.StatusFailed:
		return RunFailed
	case workflow.StatusTerminated:
		return RunCancelled
	}
	switch startState {
	case startQueued:
		return RunQueued
	case startFilterFailed:
		return RunFilterFailed
	case startBlocked:
		return RunBlocked
	case startCancelled:
		return RunCancelled
	}
	// An instance row was created in the same transaction and is about to run.
	return RunRunning
}

// finished reports whether a status is terminal.
func (s RunStatus) finished() bool {
	switch s {
	case RunDone, RunFailed, RunCancelled, RunFilterFailed:
		return true
	case RunQueued, RunRunning, RunWaiting, RunBlocked:
	}
	return false
}

// Origin is who saved or enables a graph: a human through the interactive UI, or an agent.
type Origin string

const (
	OriginHuman Origin = "human"
	OriginAgent Origin = "agent"
)

func (o Origin) valid() bool { return o == OriginHuman || o == OriginAgent }

// Actor is resolved by the caller from the trusted session (UI) or MCP principal; never from input.
type Actor struct {
	TenantID uuid.UUID
	UserID   int64
	Origin   Origin
	Subject  spi.Subject // current permissions
}

// Permissions the service checks on Actor.Subject.
const (
	PermRead    = "Automations.Read"
	PermAuthor  = "Automations.Author"
	PermEnable  = "Automations.Enable"
	PermOperate = "Automations.Operate"
)

// Service is the API of graphs, versions and runs.
type Service interface {
	Create(ctx context.Context, a Actor, in CreateInput) (Automation, Version, error)
	// SaveVersion creates version BaseVersion+1. A stale base → ErrVersionConflict.
	// Validation failure → *ValidationError; nothing is written.
	SaveVersion(ctx context.Context, a Actor, id uuid.UUID, in SaveInput) (Version, error)
	SaveLayout(ctx context.Context, a Actor, id uuid.UUID, l graph.Layout) error // merge, last write wins; no version
	// SaveLayoutAt merges l into the layout when its revision is still baseRevision and returns the new
	// revision; a stale base → *LayoutConflictError (ErrVersionConflict) with the current layout.
	SaveLayoutAt(ctx context.Context, a Actor, id uuid.UUID, baseRevision int, l graph.Layout) (int, error)
	Get(ctx context.Context, a Actor, id uuid.UUID) (Automation, error)
	GetVersion(ctx context.Context, a Actor, id uuid.UUID, version int) (Version, error)
	List(ctx context.Context, a Actor, f ListFilter) (Page[Automation], error)
	ListVersions(ctx context.Context, a Actor, id uuid.UUID) ([]VersionInfo, error)
	// Overviews are what a list of graphs shows of each of ids (graphs of other tenants and deleted ones
	// are left out): the latest version's author and triggers, the last run and the runs that need
	// attention.
	Overviews(ctx context.Context, a Actor, ids []uuid.UUID) (map[uuid.UUID]Overview, error)
	// CountLive counts what Disable and Delete would cancel (Automations.Read, Enable or Author).
	CountLive(ctx context.Context, a Actor, id uuid.UUID) (LiveCount, error)

	// EnableReview returns what a human must see before enabling: code nodes, their secrets and the digest.
	EnableReview(ctx context.Context, a Actor, id uuid.UUID, version int) (EnableReview, error)
	// Enable pins version as enabled_version. a.Origin != human → ErrHumanRequired.
	// in.CodeDigest must equal the version's code_digest when the version has code nodes → ErrReviewRequired.
	Enable(ctx context.Context, a Actor, id uuid.UUID, in EnableInput) (Automation, error)
	Disable(ctx context.Context, a Actor, id uuid.UUID) (Automation, error)
	Delete(ctx context.Context, a Actor, id uuid.UUID) error

	StartManual(ctx context.Context, a Actor, id uuid.UUID, in ManualStart) (RunRef, error)
	Cancel(ctx context.Context, a Actor, runID uuid.UUID) error
	Restart(ctx context.Context, a Actor, runID uuid.UUID, version int) (RunRef, error) // StartV2
	ListRuns(ctx context.Context, a Actor, f RunFilter) (Page[RunSummary], error)
	GetRun(ctx context.Context, a Actor, runID uuid.UUID) (RunDetail, error)
}

// CreateInput is a new graph and its first version.
type CreateInput struct {
	Graph  json.RawMessage // graph.Graph of #1322: name, description and run_as are graph fields
	Layout graph.Layout
	Note   string
}

// SaveInput is a new version of a graph.
type SaveInput struct {
	BaseVersion int
	Graph       json.RawMessage
	Layout      graph.Layout // optional; saved together with the version
	// LayoutBase, when set, is the layout revision Layout was made over: when another write moved the
	// layout since, the stored positions stay and only steps the layout does not place yet take
	// Layout's.
	LayoutBase *int
	Note       string
}

// ListFilter selects graphs of a list.
type ListFilter struct {
	Query   string // a substring of the name, case-insensitive
	Enabled *bool
	// Attention keeps graphs with runs that need attention (failed, blocked or filter_failed).
	Attention bool
	Cursor    string
	Limit     int // ≤ 100, default 50
}

// Automation is a graph: its versions, the one that runs and its layout.
type Automation struct {
	ID             uuid.UUID
	Name           string
	LatestVersion  int
	EnabledVersion *int
	OwnerUserID    *int64
	Layout         graph.Layout
	// LayoutRevision counts the changes of Layout (SaveLayout, SaveLayoutAt, a version saved with one).
	LayoutRevision int
	CreatedBy      int64
	CreatedOrigin  Origin
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Version is one immutable version of a graph.
type Version struct {
	AutomationID    uuid.UUID
	Version         int
	Graph           json.RawMessage
	CatalogRevision string
	CodeDigest      string
	SavedBy         int64
	SavedOrigin     Origin
	Note            string
	CreatedAt       time.Time
}

// VersionInfo is a version in the list of versions.
type VersionInfo struct {
	Version     int
	SavedBy     int64
	SavedOrigin Origin
	Note        string
	Enabled     bool
	CreatedAt   time.Time
}

// EnableReview is what a human sees before enabling a version.
type EnableReview struct {
	Version    int
	CodeNodes  []CodeNodeReview // empty → CodeDigest may be ""
	CodeDigest string           // sha256 over code nodes (id, lang, source, declared secrets, fetch allow) of the version
	Triggers   []TriggerReview  // event keys / schedules the version subscribes to
	RunAs      string           // run_as of the version; shown to the human with the code
	// Problems are why the version does not enable now (validation against the reviewer's catalog: an
	// unavailable connection, a type the reviewer may not use); empty when it does.
	Problems []graph.GraphError
}

// CodeNodeReview is a Code node of a version as the reviewer sees it.
type CodeNodeReview struct {
	NodeID  string   `json:"node_id"`
	Lang    string   `json:"lang"`
	Source  string   `json:"source"`
	Secrets []string `json:"secrets"`
	Fetch   bool     `json:"fetch"`
}

// TriggerReview is a trigger of a version as the reviewer sees it.
type TriggerReview struct{ TriggerID, Kind, Event, Schedule string }

// EnableInput is the version to enable and the digest of the code the human reviewed.
type EnableInput struct {
	Version    int
	CodeDigest string
	// Latest: the version must still be the graph's latest when the enable takes the graph's lock (a
	// person enables what they reviewed; a version saved meanwhile → ErrReviewRequired).
	Latest bool
}

// ManualStart starts a run from a manual trigger.
type ManualStart struct {
	TriggerID string          // manual trigger of the enabled version
	Input     json.RawMessage // validated against the trigger's input schema
	Subject   *SubjectRef
}

// SubjectRef is what a run is about: {Entity, Key}; entity "none" | "conversation" | any SubjectLoader entity.
type SubjectRef = graph.Subject

// RunRef is a started run.
type RunRef struct {
	RunID  uuid.UUID
	Status RunStatus
}

// RunFilter selects runs.
type RunFilter struct {
	AutomationID *uuid.UUID
	Version      int // 0: every version
	Status       []RunStatus
	Subject      *SubjectRef
	// SubjectKeyPrefix keeps the runs whose subject key starts with it, case-insensitively ("": any).
	SubjectKeyPrefix string
	Since, Until     *time.Time
	Cursor           string
	Limit            int // ≤ 100, default 50
}

// Page is one page of a list; Next is the cursor of the next page, "" at the end.
type Page[T any] struct {
	Items []T
	Next  string
}

// RunSummary is a run in a list.
type RunSummary struct {
	RunID        uuid.UUID
	AutomationID uuid.UUID
	Version      int
	Status       RunStatus
	Reason       string
	TriggerKind  string
	TriggerID    string
	Subject      SubjectRef
	CurrentNodes []string
	Error        *StepError
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// RunDetail is a run with its tokens, steps and waits.
type RunDetail struct {
	RunSummary
	Tokens     []TokenView
	Steps      []StepView // input/output projected for the viewer
	Waits      []WaitView
	InstanceID *uuid.UUID
}

// TokenView is a token of a run.
type TokenView struct {
	ID           uuid.UUID
	Parent       *uuid.UUID
	NodeID       string
	Status       string
	Branch       string
	ForeachIndex *int
}

// StepView is a step of a run.
type StepView struct {
	TokenID   uuid.UUID
	Seq       int
	NodeID    string
	Attempt   int
	Input     json.RawMessage
	Output    json.RawMessage
	Outcome   string
	Error     *StepError
	StartedAt time.Time
	EndedAt   *time.Time
	// Warnings are what the step noted without failing (an assignee list cut to its limit).
	Warnings []string
	// Logs are what the node logged (a call: Log), masked by the engine with the connection's secrets.
	Logs []string
	// Hidden: the step ran after the run read a sensitive field the viewer may not read; its input and
	// output are left out and its error message is Hidden.
	Hidden bool
}

// StepError is why a step or a run failed.
type StepError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
	// Retryable: the provider said the request may succeed later (a call's failure).
	Retryable bool `json:"retryable,omitempty"`
}

func (e *StepError) Error() string { return e.Code + ": " + e.Message }

// WaitView is a wait of a run.
type WaitView struct {
	TokenID uuid.UUID
	// NodeID is the node the wait belongs to (a join's timeout wait: the join).
	NodeID    string
	WaitID    string
	Kind      string
	EventKind string
	TimeoutAt *time.Time
	Status    string
}

// Errors of the service.
var (
	ErrNotFound              = errors.New("automation: not_found") // absent or other tenant
	ErrForbidden             = errors.New("automation: forbidden")
	ErrVersionConflict       = errors.New("automation: VERSION_CONFLICT") // *ConflictError{Latest int, By int64}
	ErrHumanRequired         = errors.New("automation: HUMAN_REQUIRED")
	ErrReviewRequired        = errors.New("automation: review_required") // digest mismatch: code changed or not shown
	ErrNotEnabled            = errors.New("automation: not_enabled")     // manual start of a disabled graph
	ErrConnectionUnavailable = errors.New("automation: connection_unavailable")
	ErrLimitExceeded         = errors.New("automation: limit_exceeded")
	ErrRunFinished           = errors.New("automation: run_finished") // cancel of a finished run is a no-op error
	ErrInvalidInput          = errors.New("automation: invalid_input")
)

// ValidationError is VALIDATION_FAILED: the graph does not validate against the author's catalog.
type ValidationError struct{ Errors, Warnings []graph.GraphError }

func (e *ValidationError) Error() string {
	msgs := make([]string, 0, 3)
	for i, ge := range e.Errors {
		if i == 3 {
			msgs = append(msgs, fmt.Sprintf("and %d more", len(e.Errors)-3))
			break
		}
		msgs = append(msgs, ge.Error())
	}
	return "automation: VALIDATION_FAILED: " + strings.Join(msgs, "; ")
}

// Is makes a validation error of connection_unavailable also ErrConnectionUnavailable.
func (e *ValidationError) Is(target error) bool {
	if target != ErrConnectionUnavailable {
		return false
	}
	for _, ge := range e.Errors {
		if ge.Code == CodeConnectionUnavailable {
			return true
		}
	}
	return false
}

// LayoutConflictError is ErrVersionConflict of SaveLayoutAt: the layout's revision and positions now.
type LayoutConflictError struct {
	Revision int
	Layout   graph.Layout
}

func (e *LayoutConflictError) Error() string {
	return fmt.Sprintf("%s: the layout is at revision %d", ErrVersionConflict.Error(), e.Revision)
}

func (e *LayoutConflictError) Is(target error) bool { return target == ErrVersionConflict }

// Overview is a graph as a list shows it.
type Overview struct {
	AutomationID uuid.UUID
	// LatestSavedBy and LatestSavedOrigin saved the latest version, at LatestSavedAt.
	LatestSavedBy     int64
	LatestSavedOrigin Origin
	LatestSavedAt     time.Time
	// LatestDescription is the description of the latest version's graph.
	LatestDescription string
	// LatestCodeDigest and EnabledCodeDigest tell whether the latest version changed the code of the
	// enabled one.
	LatestCodeDigest  string
	EnabledCodeDigest string
	Triggers          []TriggerReview // of the latest version
	LastRun           *RunSummary     // CurrentNodes and Error are left empty
	// Attention counts the kept runs that failed, are blocked or whose filter failed.
	Attention int
}

// ConflictError is ErrVersionConflict with the latest version and who saved it.
type ConflictError struct {
	Latest int
	By     int64
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("%s: the latest version is %d, saved by %d", ErrVersionConflict.Error(), e.Latest, e.By)
}

func (e *ConflictError) Is(target error) bool { return target == ErrVersionConflict }

// Error codes of steps and runs.
const (
	CodeConnectionUnavailable = "connection_unavailable"
	CodeLimitExceeded         = "limit_exceeded"
	CodeMissingContext        = "missing_context"
	CodeUnsupported           = "unsupported"
	CodeEffectFailed          = "effect_failed"
	CodeTimeout               = "timeout"
	CodeCancelled             = "cancelled"
	// CodeTriggersUnreadable: the reviewer may not read an event the version's triggers are on.
	CodeTriggersUnreadable = "triggers_unreadable"
)

// SubjectLoader loads the subject entity for CEL scope `subject`, with the run-as rights.
type SubjectLoader interface {
	Load(ctx context.Context, as spi.Subject, ref SubjectRef) (json.RawMessage, error) // ErrNotFound when not visible
}

// CallExecutor runs a node with behavior=call outside the step lock. Implemented in #1336.
// The core only registers the call as a wait and applies its result.
type CallExecutor interface {
	Begin(ctx context.Context, tx pgx.Tx, run StepContext, node graph.Node, input json.RawMessage) (WaitSpec, error)
}

// TaskBridge turns a human node into a task of #1327 and its decision into an outcome. Implemented in #1336.
type TaskBridge interface {
	Begin(ctx context.Context, tx pgx.Tx, run StepContext, node graph.Node, input json.RawMessage) (WaitSpec, error)
}

// TaskResolver is an optional interface of a TaskBridge: the outcome of a task the core opened for a wait
// (WaitSpec.Task). Without it the decision's "outcome" field is the outcome and the decision the output;
// a task timeout is outcome "timeout".
type TaskResolver interface {
	Resolve(node graph.Node, res workflow.TaskResult) WaitResult
}

// HumanNode is an optional interface of a catalog.NodeType: its nodes are human tasks, run through the
// TaskBridge instead of the CallExecutor.
type HumanNode interface {
	Human() bool
}

// WaitKeyer is an optional interface of a catalog.NodeType with behavior wait: the subject the wait listens on
// when it differs from the run subject (conversation of a connector). Params are rendered.
type WaitKeyer interface {
	WaitSubject(params map[string]any) (SubjectRef, bool)
}

// EffectNode is an optional interface of a catalog.NodeType with behavior effect (or a wait that emits its
// effect): the effect applied inside the step's transaction, atomically with the step. The output is the
// node's output; an error is the node's failure (its failed outcome when it has one).
type EffectNode interface {
	ApplyEffect(ctx context.Context, tx pgx.Tx, run StepContext, node graph.Node, effect graph.Effect) (map[string]any, error)
}

// CodeNode is an optional interface of a catalog.NodeType: its nodes run code a human reviews before a
// version is enabled. Review is what the reviewer sees for a node's params.
type CodeNode interface {
	Review(params map[string]any) CodeNodeReview
}

// StepContext is what a node executor gets about the step it runs in.
type StepContext struct {
	TenantID, RunID, TokenID uuid.UUID
	Seq                      int
	AutomationID             uuid.UUID
	Version                  int
	RunAs                    spi.Subject
	Subject                  SubjectRef
	Depth                    int
	Native                   workflow.NativeRun // primitives of #1321
	// FailedConnected: the node's failed outcome leads to a node (not to ignore): a call whose outcome is
	// unknown takes it; otherwise the call blocks for an operator.
	FailedConnected bool
}

// WaitSpec is a wait a node executor registers.
type WaitSpec struct {
	WaitID  string        `json:"wait_id"` // unique within the run
	Kind    string        `json:"kind"`    // call | task
	Timeout time.Duration `json:"timeout"` // 0 = node default
	// Task, when set, is the human task the core opens for the wait after the step commits (the
	// occurrence "task:<WaitID>"); its decision completes the wait. Without it the result arrives on the
	// inbox as a WaitResult.
	Task *workflow.TaskSpec `json:"task,omitempty"`
	// Child, when set, is the child instance that answers the wait: cancelling the wait terminates its tree.
	Child *uuid.UUID `json:"child,omitempty"`
	// Decision, when set, are the rules a decision of the wait's task must keep beyond its schema; the
	// engine's task observer refuses a decision that breaks them.
	Decision *DecisionRules `json:"decision,omitempty"`
}

// DecisionRules are the rules of a human task's decisions: the outcomes that need a comment and the
// JSON Schema of the extra fields.
type DecisionRules struct {
	CommentRequired []string        `json:"comment_required,omitempty"`
	Form            json.RawMessage `json:"form,omitempty"`
}

// WaitResult is delivered to the run inbox; the core maps it to an outcome of the waiting node.
type WaitResult struct {
	WaitID  string          `json:"wait_id"`
	Outcome string          `json:"outcome"`
	Output  json.RawMessage `json:"output,omitempty"`
	Error   *StepError      `json:"error,omitempty"`
	// Await, set by an ask node's call, opens the wait for its answer instead of applying the outcome.
	Await *AwaitSpec `json:"await,omitempty"`
	// Logs are what a call node logged (Log), already masked: they are kept with the call's step.
	Logs []string `json:"logs,omitempty"`
}

// Limits bound runs.
type Limits struct {
	MaxDepth        int           // automation → event → automation, default 10
	MaxTokens       int           // active tokens per run (parallel branches + foreach items), default 1000
	ForeachItems    int           // default 1000
	ForeachParallel int           // default 20
	WaitMax         time.Duration // default 30d
	PayloadBytes    int           // event payload and node output, default 1 MiB
	CELCost         uint64        // passed to expr of #1322
	StepsPerRun     int           // default 10000; protects against wait-less loops at runtime
	Concurrency     int           // engine slots for automation.run, separate from TS workflows
}

func (l Limits) withDefaults() Limits {
	if l.MaxDepth <= 0 {
		l.MaxDepth = 10
	}
	if l.MaxTokens <= 0 {
		l.MaxTokens = 1000
	}
	if l.ForeachItems <= 0 {
		l.ForeachItems = 1000
	}
	if l.ForeachParallel <= 0 {
		l.ForeachParallel = 20
	}
	if l.WaitMax <= 0 {
		l.WaitMax = 30 * 24 * time.Hour
	}
	if l.PayloadBytes <= 0 {
		l.PayloadBytes = 1 << 20
	}
	if l.StepsPerRun <= 0 {
		l.StepsPerRun = 10000
	}
	if l.Concurrency <= 0 {
		l.Concurrency = 4
	}
	return l
}

// Retention is how long finished runs and their steps are kept.
type Retention struct{ Steps, Runs time.Duration } // defaults 30d / 90d; job prunes finished runs only

func (r Retention) withDefaults() Retention {
	if r.Steps <= 0 {
		r.Steps = 30 * 24 * time.Hour
	}
	if r.Runs <= 0 {
		r.Runs = 90 * 24 * time.Hour
	}
	return r
}
