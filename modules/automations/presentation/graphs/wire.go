package graphs

import "encoding/json"

// Wire types of the automations screens' RPC. iota-solid generates the TypeScript contract from them
// (web/Automations.solid.generated.ts); a result union of the issue's contract is one struct whose
// variant fields are optional.

// View is the screen a route renders.
type View string

const (
	ViewList   View = "list"
	ViewNew    View = "new"
	ViewEditor View = "editor"
	// ViewRun is a link that knows only a run's id (/automations/runs/{run}): the screen finds the run's
	// graph and opens its Runs tab there, or says the run is not found.
	ViewRun View = "run"
)

// Props are the initial props of the screen.
type Props struct {
	// BasePath is the path of the list (/automations); a graph is <BasePath>/<id>.
	BasePath string `json:"basePath"`
	// IntegrationsPath is the integrations page a missing connection links to.
	IntegrationsPath string `json:"integrationsPath"`
	View             View   `json:"view"`
	// ID is the graph of the editor.
	ID string `json:"id,omitempty"`
	// Run is the run the editor opens on its Runs tab.
	Run string `json:"run,omitempty"`
	Can Can    `json:"can"`
	// Agent: the session is an agent's; enabling is a human's.
	Agent bool `json:"agent"`
}

// Can is what the caller may do.
type Can struct {
	Author  bool `json:"author"`
	Enable  bool `json:"enable"`
	Operate bool `json:"operate"`
}

// RunStatus is engine.RunStatus.
type RunStatus string

const (
	RunQueued       RunStatus = "queued"
	RunRunning      RunStatus = "running"
	RunWaiting      RunStatus = "waiting"
	RunBlocked      RunStatus = "blocked"
	RunDone         RunStatus = "done"
	RunFailed       RunStatus = "failed"
	RunCancelled    RunStatus = "cancelled"
	RunFilterFailed RunStatus = "filter_failed"
)

// RunState is a run's state as the editor shows it (RunState of #1347): queued is waiting and
// filter_failed is failed with the error code filter_failed.
type RunState string

const (
	StateRunning   RunState = "running"
	StateWaiting   RunState = "waiting"
	StateBlocked   RunState = "blocked"
	StateDone      RunState = "done"
	StateFailed    RunState = "failed"
	StateCancelled RunState = "cancelled"
)

// StatusFilter selects graphs of the list.
type StatusFilter string

const (
	FilterEnabled   StatusFilter = "enabled"
	FilterDisabled  StatusFilter = "disabled"
	FilterAttention StatusFilter = "attention"
)

// TriggerKind is how a trigger starts a graph.
type TriggerKind string

const (
	TriggerEvent    TriggerKind = "event"
	TriggerSchedule TriggerKind = "schedule"
	TriggerManual   TriggerKind = "manual"
	TriggerInbound  TriggerKind = "inbound"
)

// Origin is who saved a version.
type Origin string

const (
	OriginHuman Origin = "human"
	OriginAgent Origin = "agent"
)

// ErrorCode is a refusal the screen explains.
type ErrorCode string

const (
	ErrReviewRequired   ErrorCode = "review_required"
	ErrHumanRequired    ErrorCode = "HUMAN_REQUIRED"
	ErrValidationFailed ErrorCode = "VALIDATION_FAILED"
	ErrForbidden        ErrorCode = "forbidden"
	ErrNotEnabled       ErrorCode = "not_enabled"
	ErrInvalidInput     ErrorCode = "invalid_input"
	ErrVersionConflict  ErrorCode = "VERSION_CONFLICT"
	ErrRunFinished      ErrorCode = "run_finished"
	ErrConnectionGone   ErrorCode = "connection_unavailable"
)

// ListRequest is automations.list.
type ListRequest struct {
	Query  string       `json:"query,omitempty"`
	Status StatusFilter `json:"status,omitempty"`
	Cursor string       `json:"cursor,omitempty"`
	Limit  int          `json:"limit,omitempty"`
}

// TriggerView is a trigger as the list names it: its kind and the title of its event.
type TriggerView struct {
	Kind  TriggerKind `json:"kind"`
	Title string      `json:"title"`
}

// LastRun is the last run of a graph.
type LastRun struct {
	RunID  string    `json:"run_id"`
	Status RunStatus `json:"status"`
	At     string    `json:"at"`
}

// ListItem is a graph of the list.
type ListItem struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Description    *string `json:"description"`
	Enabled        bool    `json:"enabled"`
	EnabledVersion *int    `json:"enabled_version"`
	LatestVersion  int     `json:"latest_version"`
	// PendingVersion: the latest version is saved but not enabled (an agent saved it, or its code changed).
	PendingVersion bool          `json:"pending_version"`
	SavedByAgent   bool          `json:"saved_by_agent"`
	Triggers       []TriggerView `json:"triggers"`
	LastRun        *LastRun      `json:"last_run"`
	// Attention counts the kept runs that failed, are blocked or whose filter failed.
	Attention int    `json:"attention"`
	UpdatedAt string `json:"updated_at"`
	UpdatedBy string `json:"updated_by"`
}

// ListResponse is a page of the list.
type ListResponse struct {
	Items      []ListItem `json:"items"`
	NextCursor *string    `json:"next_cursor"`
	Can        Can        `json:"can"`
}

// GetRequest is automations.get.
type GetRequest struct {
	ID      string `json:"id"`
	Version int    `json:"version,omitempty"`
}

// UserRef is a user by name.
type UserRef struct {
	UserID int64  `json:"user_id"`
	Name   string `json:"name"`
}

// AutomationView is a graph in the editor.
type AutomationView struct {
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	Enabled        bool          `json:"enabled"`
	EnabledVersion *int          `json:"enabled_version"`
	LatestVersion  int           `json:"latest_version"`
	Owner          *UserRef      `json:"owner"`
	Layout         map[string]XY `json:"layout"`
	LayoutRevision int           `json:"layout_revision"`
}

// XY is a position on the canvas.
type XY struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// SavedBy is who saved a version.
type SavedBy struct {
	Name   string `json:"name"`
	Origin Origin `json:"origin"`
}

// VersionEntry is a version in the versions menu.
type VersionEntry struct {
	Version int    `json:"version"`
	Note    string `json:"note,omitempty"`
	Enabled bool   `json:"enabled,omitempty"`
	By      string `json:"by,omitempty"`
	Origin  Origin `json:"origin"`
	At      string `json:"at"`
}

// ReadOnly is why the editor is read-only.
type ReadOnly struct {
	Reason string `json:"reason"`
	// Code is no_author (no Automations.Author) or old_version.
	Code string `json:"code"`
}

// ConnectionOption is a working connection a node may pick.
type ConnectionOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GetResponse is automations.get.
type GetResponse struct {
	Automation AutomationView  `json:"automation"`
	Graph      json.RawMessage `json:"graph"`
	Version    int             `json:"version"`
	SavedBy    SavedBy         `json:"saved_by"`
	Versions   []VersionEntry  `json:"versions"`
	Catalog    EditorCatalog   `json:"catalog"`
	// Connections are the tenant's working connections by connector kind.
	Connections map[string][]ConnectionOption `json:"connections"`
	ReadOnly    *ReadOnly                     `json:"read_only"`
	Can         Can                           `json:"can"`
	// ManualTriggers are the manual triggers of the enabled version (what «Run» starts); empty when
	// the graph is off.
	ManualTriggers []ManualTrigger `json:"manual_triggers"`
}

// ManualTrigger is a manual trigger of the enabled version and the schema of its input.
type ManualTrigger struct {
	TriggerID   string         `json:"trigger_id"`
	Title       string         `json:"title"`
	InputSchema map[string]any `json:"input_schema,omitempty"`
}

// CatalogRequest is automations.catalog: the catalog of a new graph.
type CatalogRequest struct{}

// CatalogResponse is the catalog an author sees and the tenant's connections.
type CatalogResponse struct {
	Catalog     EditorCatalog                 `json:"catalog"`
	Connections map[string][]ConnectionOption `json:"connections"`
	Can         Can                           `json:"can"`
}

// GraphError is a validation error of a graph.
type GraphError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// Conflict is VERSION_CONFLICT of a save: the latest version and who saved it.
type Conflict struct {
	By      string `json:"by"`
	Version int    `json:"version"`
}

// SaveResult is SaveResult of #1331: ok with the saved graph, validation errors, or a conflict.
type SaveResult struct {
	OK      bool            `json:"ok"`
	Graph   json.RawMessage `json:"graph,omitempty"`
	Version int             `json:"version,omitempty"`
	// PendingReview: saved; the enabled version keeps running until a review.
	PendingReview bool         `json:"pending_review,omitempty"`
	Errors        []GraphError `json:"errors,omitempty"`
	Warnings      []GraphError `json:"warnings,omitempty"`
	Conflict      *Conflict    `json:"conflict,omitempty"`
	// ID is the graph a create made.
	ID string `json:"id,omitempty"`
	// LayoutRevision is the layout's revision after the save.
	LayoutRevision int `json:"layout_revision,omitempty"`
	// LayoutChanged: another tab moved steps since the layout revision the save was made over; its
	// Positions were kept.
	LayoutChanged bool          `json:"layout_changed,omitempty"`
	Positions     map[string]XY `json:"positions,omitempty"`
}

// CreateRequest is automations.create.
type CreateRequest struct {
	Graph  json.RawMessage `json:"graph"`
	Layout map[string]XY   `json:"layout"`
}

// SaveRequest is automations.save.
type SaveRequest struct {
	ID     string          `json:"id"`
	Base   int             `json:"base"`
	Graph  json.RawMessage `json:"graph"`
	Layout map[string]XY   `json:"layout"`
	// LayoutRevision is the layout revision the editor's positions were made over.
	LayoutRevision *int   `json:"layout_revision,omitempty"`
	Note           string `json:"note,omitempty"`
}

// SaveLayoutRequest is automations.save_layout.
type SaveLayoutRequest struct {
	ID           string        `json:"id"`
	BaseRevision int           `json:"base_revision"`
	Positions    map[string]XY `json:"positions"`
}

// SaveLayoutResponse is ok with the new revision, or VERSION_CONFLICT with the layout of now.
type SaveLayoutResponse struct {
	OK        bool          `json:"ok"`
	Revision  int           `json:"revision"`
	Error     ErrorCode     `json:"error,omitempty"`
	Positions map[string]XY `json:"positions,omitempty"`
}

// ValidateRequest is automations.validate.
type ValidateRequest struct {
	Graph json.RawMessage `json:"graph"`
}

// ValidateResponse is the result of a validation.
type ValidateResponse struct {
	OK       bool         `json:"ok"`
	Errors   []GraphError `json:"errors"`
	Warnings []GraphError `json:"warnings"`
}

// ReviewRequest is automations.enable_review.
type ReviewRequest struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}

// CodeLang is the language of a Code node.
type CodeLang string

const (
	LangTS CodeLang = "ts"
	LangJS CodeLang = "js"
)

// CodeNodeView is a Code node as the reviewer sees it.
type CodeNodeView struct {
	NodeID  string   `json:"node_id"`
	Title   string   `json:"title"`
	Lang    CodeLang `json:"lang"`
	Source  string   `json:"source"`
	Secrets []string `json:"secrets"`
	Fetch   bool     `json:"fetch"`
}

// ReviewTrigger is a trigger of the reviewed version.
type ReviewTrigger struct {
	TriggerID string      `json:"trigger_id"`
	Kind      TriggerKind `json:"kind"`
	Title     string      `json:"title"`
	Event     *string     `json:"event"`
	Schedule  *string     `json:"schedule"`
}

// RunAs is whose rights a run has.
type RunAs string

const (
	RunAsOwner     RunAs = "owner"
	RunAsInitiator RunAs = "initiator"
)

// ReviewResponse is what a human sees before enabling a version.
type ReviewResponse struct {
	Version    int             `json:"version"`
	CodeNodes  []CodeNodeView  `json:"code_nodes"`
	Triggers   []ReviewTrigger `json:"triggers"`
	RunAs      RunAs           `json:"run_as"`
	CodeDigest string          `json:"code_digest"`
	// Live counts what disabling cancels now (the toggle's confirmation).
	Live LiveView `json:"live"`
	// Problems are why the version does not enable now; empty when it does.
	Problems []GraphError `json:"problems"`
}

// LiveView is what disabling or deleting a graph cancels.
type LiveView struct {
	Runs  int `json:"runs"`
	Waits int `json:"waits"`
}

// IDRequest names a graph.
type IDRequest struct {
	ID string `json:"id"`
}

// EnableRequest is automations.enable.
type EnableRequest struct {
	ID         string `json:"id"`
	Version    int    `json:"version"`
	CodeDigest string `json:"code_digest"`
}

// EnableResponse is ok with the enabled version, or the refusal.
type EnableResponse struct {
	OK             bool         `json:"ok"`
	EnabledVersion int          `json:"enabled_version,omitempty"`
	Error          ErrorCode    `json:"error,omitempty"`
	Errors         []GraphError `json:"errors,omitempty"`
}

// DisableResponse is automations.disable.
type DisableResponse struct {
	OK            bool `json:"ok"`
	CancelledRuns int  `json:"cancelled_runs"`
}

// OKResponse is a plain success.
type OKResponse struct {
	OK bool `json:"ok"`
}

// Subject is what a run is about.
type Subject struct {
	Entity string `json:"entity"`
	Key    string `json:"key"`
	Label  string `json:"label,omitempty"`
}

// RunManualRequest is automations.run_manual.
type RunManualRequest struct {
	ID        string          `json:"id"`
	TriggerID string          `json:"trigger_id"`
	Input     json.RawMessage `json:"input"`
	Subject   *Subject        `json:"subject,omitempty"`
}

// RunManualResponse is the started run, or the refusal.
type RunManualResponse struct {
	RunID  string            `json:"run_id,omitempty"`
	OK     bool              `json:"ok"`
	Error  ErrorCode         `json:"error,omitempty"`
	Fields map[string]string `json:"fields,omitempty"`
}

// DryRunRequest is automations.dry_run.
type DryRunRequest struct {
	Graph   json.RawMessage            `json:"graph"`
	Subject Subject                    `json:"subject"`
	Context map[string]json.RawMessage `json:"context,omitempty"`
	Events  []json.RawMessage          `json:"events"`
	// Decisions is how execute nodes answer: scripted (default, from the context), cached or live.
	Decisions DecisionMode `json:"decisions,omitempty"`
}

// DecisionMode is how a dry run's execute nodes answer (#1336).
type DecisionMode string

// Decision modes of a dry run.
const (
	DecisionsScripted DecisionMode = "scripted"
	DecisionsCached   DecisionMode = "cached"
	DecisionsLive     DecisionMode = "live"
)

// TraceStep is a step of a dry run.
type TraceStep struct {
	Node    string            `json:"node"`
	Token   string            `json:"token,omitempty"`
	Kind    string            `json:"kind"`
	Outcome string            `json:"outcome,omitempty"`
	Path    string            `json:"path,omitempty"`
	Message string            `json:"message,omitempty"`
	Status  string            `json:"status,omitempty"`
	Effects []json.RawMessage `json:"effects,omitempty"`
	Output  json.RawMessage   `json:"output,omitempty"`
}

// DryRunResponse is the trace of a dry run.
type DryRunResponse struct {
	Trace []TraceStep `json:"trace"`
}

// TestNodeRequest is automations.test_node.
type TestNodeRequest struct {
	Graph   json.RawMessage            `json:"graph"`
	NodeID  string                     `json:"node_id"`
	Subject *Subject                   `json:"subject,omitempty"`
	Context map[string]json.RawMessage `json:"context,omitempty"`
}

// TestNodeError is why a tested node failed.
type TestNodeError struct {
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

// TestNodeResponse is TestNodeResult of #1331.
type TestNodeResponse struct {
	Outcome    string          `json:"outcome"`
	Output     json.RawMessage `json:"output,omitempty"`
	Logs       []string        `json:"logs,omitempty"`
	DurationMS int64           `json:"duration_ms,omitempty"`
	Error      *TestNodeError  `json:"error,omitempty"`
}

// SearchSubjectsRequest is automations.search_subjects.
type SearchSubjectsRequest struct {
	Entity string `json:"entity"`
	Query  string `json:"query"`
	Limit  int    `json:"limit,omitempty"`
}

// SubjectOption is a subject an author may pick.
type SubjectOption struct {
	Subject Subject `json:"subject"`
	Label   string  `json:"label"`
	Note    string  `json:"note,omitempty"`
}

// RunsRequest is automations.runs.
type RunsRequest struct {
	AutomationID string     `json:"automation_id"`
	Version      int        `json:"version,omitempty"`
	State        []RunState `json:"state,omitempty"`
	// Subject narrows the runs to subjects whose key starts with it (case-insensitive).
	Subject string `json:"subject,omitempty"`
	Cursor  string `json:"cursor,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

// StepError is why a step or a run failed.
type StepError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Path      string `json:"path,omitempty"`
	Retryable bool   `json:"retryable,omitempty"`
}

// RunSummary is a run in a list (RunSummary of #1347).
type RunSummary struct {
	RunID        string     `json:"run_id"`
	AutomationID string     `json:"automation_id"`
	Version      int        `json:"version"`
	State        RunState   `json:"state"`
	Status       RunStatus  `json:"status"`
	Reason       string     `json:"reason,omitempty"`
	TriggerKind  string     `json:"trigger_kind"`
	TriggerID    string     `json:"trigger_id"`
	Subject      Subject    `json:"subject"`
	CurrentNodes []string   `json:"current_nodes"`
	Error        *StepError `json:"error"`
	CreatedAt    string     `json:"created_at"`
	UpdatedAt    string     `json:"updated_at"`
}

// RunsResponse is a page of runs.
type RunsResponse struct {
	Items      []RunSummary `json:"items"`
	NextCursor *string      `json:"next_cursor"`
}

// RunRequest names a run.
type RunRequest struct {
	RunID string `json:"run_id"`
}

// TokenView is a token of a run.
type TokenView struct {
	ID           string  `json:"id"`
	Parent       *string `json:"parent"`
	NodeID       string  `json:"node_id"`
	Status       string  `json:"status"`
	Branch       string  `json:"branch,omitempty"`
	ForeachIndex *int    `json:"foreach_index"`
}

// StepView is a step of a run; Hidden steps carry no input or output.
type StepView struct {
	TokenID   string          `json:"token_id"`
	Seq       int             `json:"seq"`
	NodeID    string          `json:"node_id"`
	Attempt   int             `json:"attempt"`
	Input     json.RawMessage `json:"input,omitempty"`
	Output    json.RawMessage `json:"output,omitempty"`
	Outcome   string          `json:"outcome,omitempty"`
	Error     *StepError      `json:"error"`
	StartedAt string          `json:"started_at"`
	EndedAt   *string         `json:"ended_at"`
	Warnings  []string        `json:"warnings,omitempty"`
	Hidden    bool            `json:"hidden,omitempty"`
}

// WaitView is a wait of a run.
type WaitView struct {
	TokenID string `json:"token_id"`
	// NodeID is the node the waiting token stands on.
	NodeID    string  `json:"node_id"`
	WaitID    string  `json:"wait_id"`
	Kind      string  `json:"kind"`
	EventKind string  `json:"event_kind,omitempty"`
	TimeoutAt *string `json:"timeout_at"`
	Status    string  `json:"status"`
}

// RunData is a run with its tokens, steps and waits (RunData of #1347).
type RunData struct {
	Run    RunSummary  `json:"run"`
	Tokens []TokenView `json:"tokens"`
	Steps  []StepView  `json:"steps"`
	Waits  []WaitView  `json:"waits"`
}

// RunCancelResponse is ok, or run_finished.
type RunCancelResponse struct {
	OK    bool      `json:"ok"`
	Error ErrorCode `json:"error,omitempty"`
}

// RunResolveRequest is automations.run_resolve.
type RunResolveRequest struct {
	RunID   string          `json:"run_id"`
	WaitID  string          `json:"wait_id"`
	Outcome string          `json:"outcome"`
	Output  json.RawMessage `json:"output,omitempty"`
}

// RunRetryRequest is automations.run_retry.
type RunRetryRequest struct {
	RunID  string `json:"run_id"`
	WaitID string `json:"wait_id"`
}

// RunRestartRequest is automations.run_restart.
type RunRestartRequest struct {
	RunID   string `json:"run_id"`
	Version int    `json:"version"`
}

// RunRef is a started run.
type RunRef struct {
	RunID string `json:"run_id"`
}
