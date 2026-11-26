// Package backtest runs a version of an automation graph over past and synthetic events without sending
// anything out, and reports what the graph would have done for each of them (#1389).
//
// A dataset belongs to an automation. Its examples are copies of events: from the SDK's event journal
// (the outbox) over a window, from the history of a connection whose connector implements
// HistoryImporter (the email connector's IMAP mailbox), from a module's HistorySource, or synthetic
// events an author writes. An example keeps the state of its subject at the event when the subject
// loader gives it (SnapshotLoader); otherwise the state read at import is kept and marked approximate.
// A dataset survives the pruning of the journal and runs on any version of the graph any number of
// times.
//
// A backtest is the dry-run interpreter of the engine (engine.Authoring.DryRun) in capture mode over each
// example: effects are captured with their params rendered, execute calls other than decisions of models
// are answered from the context, and decisions (a classifier, an LLM) are cached (the decision cache,
// else one call recorded in it) or live. An agent's cached backtest reads the cache only and makes no
// call; live is a person's choice. Backtests run in the background in the worker (Service.Serve), with
// their progress in the backtest row.
//
// Labels are the expected outcome of a node for an example: a synthetic event brings its own (expected),
// a person sets one (manual), and the manual label wins. They belong to the dataset and apply to every
// backtest over it. A report gives precision and recall per classifier and outcome, the mismatches with
// the labels, the effects the graph would have made and the cost of its decisions; Compare puts two
// backtests side by side.
package backtest

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// IDs of the package's objects.
type (
	DatasetID   = uuid.UUID
	ExampleID   = uuid.UUID
	ImportJobID = uuid.UUID
	BacktestID  = uuid.UUID
)

// Actor is who acts: the engine's actor (tenant, user, origin, current permissions).
type Actor = engine.Actor

// Limits of datasets.
const (
	// MaxExamples bounds the examples of a dataset.
	MaxExamples = 5000
	// MaxDatasets bounds the datasets of an automation.
	MaxDatasets = 20
	// MaxActiveBacktests bounds the queued and running backtests of an automation.
	MaxActiveBacktests = 5
	// MaxEventBytes bounds an example's event and its snapshot.
	MaxEventBytes = 256 << 10
	// MaxWindow bounds the window of an import.
	MaxWindow = 366 * 24 * time.Hour
	// MaxMismatches bounds the mismatches a report lists (Report.MismatchTotal counts them all).
	MaxMismatches = 500
	// MaxTraceSteps bounds the trace a result keeps.
	MaxTraceSteps = 200
)

// Decision modes of a backtest.
const (
	DecisionsCached = engine.DecisionsCached
	DecisionsLive   = engine.DecisionsLive
)

// Sources of examples.
const (
	SourceJournal   = "journal"
	SourceHistory   = "history"
	SourceSynthetic = "synthetic"
)

// Label sources: a manual label wins over the expected outcome of a synthetic event.
const (
	LabelManual   = "manual"
	LabelExpected = "expected"
)

// Statuses of import jobs and backtests.
const (
	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusDone      = "done"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

// End kinds of a result: how the graph's run over the example ended.
const (
	EndEnded        = "ended"
	EndWaiting      = "waiting"
	EndNotTriggered = "not_triggered"
	EndError        = "error"
)

// Decision statuses of a result.
const (
	DecisionLive    = "live"    // the provider was called
	DecisionCached  = "cached"  // answered from the decision cache
	DecisionMissing = "missing" // not in the cache and no call was allowed: the node took its default outcome
)

// PermConnectionsManage is the right to manage the tenant's connections: with it any connection's
// history may be imported (DefaultConnectionAccess).
const PermConnectionsManage = "Connections.Manage"

// ErrUnsupported reports a source that cannot import history. The service's other errors are the
// engine's: engine.ErrNotFound, ErrForbidden, ErrInvalidInput, ErrLimitExceeded, ErrHumanRequired.
var ErrUnsupported = errors.New("automation: backtest source is unsupported")

// ErrStaleEstimate refuses a run whose RunInput.Revision is not the dataset's now: examples were added or
// removed since the estimate the person saw.
var ErrStaleEstimate = errors.New("automation: the estimate is stale")

// Service is the API of datasets and backtests. Every method requires Automations.Author.
type Service interface {
	CreateDataset(ctx context.Context, a Actor, in CreateDatasetInput) (DatasetID, error)
	ListDatasets(ctx context.Context, a Actor, automationID uuid.UUID) ([]Dataset, error)
	GetDataset(ctx context.Context, a Actor, ds DatasetID) (Dataset, error)
	DeleteDataset(ctx context.Context, a Actor, ds DatasetID) error
	// Import queues an import of events into the dataset; the worker runs it (ImportStatus).
	Import(ctx context.Context, a Actor, ds DatasetID, src ImportSource) (ImportJobID, error)
	ImportStatus(ctx context.Context, a Actor, id ImportJobID) (ImportJob, error)
	// Sources are the history sources the actor may import from for an automation: the connections
	// whose connector imports history and the modules' sources.
	Sources(ctx context.Context, a Actor, automationID uuid.UUID) ([]SourceOption, error)
	AddSynthetic(ctx context.Context, a Actor, ds DatasetID, ev SyntheticEvent) (ExampleID, error)
	Examples(ctx context.Context, a Actor, ds DatasetID, f ExampleFilter) (Page[Example], error)
	DeleteExample(ctx context.Context, a Actor, ex ExampleID) error
	// Label sets the manual label of a node for an example; outcome "" removes it.
	Label(ctx context.Context, a Actor, ex ExampleID, node string, outcome string) error

	// Estimate is what a backtest would call before it runs: the decisions it may make and their cost.
	Estimate(ctx context.Context, a Actor, in RunInput) (Estimate, error)
	// Run queues a backtest; the worker runs it. Decisions live is a person's: an agent gets
	// engine.ErrHumanRequired.
	Run(ctx context.Context, a Actor, in RunInput) (BacktestID, error)
	Cancel(ctx context.Context, a Actor, id BacktestID) error
	ListBacktests(ctx context.Context, a Actor, automationID uuid.UUID) ([]Backtest, error)
	// Count is the number of backtests of an automation (the editor's «Backtests: N»).
	Count(ctx context.Context, a Actor, automationID uuid.UUID) (int, error)
	Report(ctx context.Context, a Actor, id BacktestID) (Report, error)
	// Results are the examples of a backtest with what the graph did for each.
	Results(ctx context.Context, a Actor, id BacktestID, f ResultFilter) (Page[Result], error)
	Compare(ctx context.Context, a Actor, a1, a2 BacktestID) (Comparison, error)
}

// Page is a page of a list; NextCursor is "" on the last page.
type Page[T any] struct {
	Items      []T
	NextCursor string
	Total      int
}

// CreateDatasetInput is a new dataset of an automation.
type CreateDatasetInput struct {
	AutomationID uuid.UUID
	Name         string
	Description  string
}

// Dataset is a set of examples of an automation.
type Dataset struct {
	ID           DatasetID
	AutomationID uuid.UUID
	Name         string
	Description  string
	Examples     int
	// Labeled counts the examples with at least one label.
	Labeled       int
	CreatedBy     int64
	CreatedOrigin engine.Origin
	CreatedAt     time.Time
	// Imports are the dataset's latest import jobs, newest first.
	Imports []ImportJob
}

// TimeWindow is a half-open interval [From, To) of event times.
type TimeWindow struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// ImportSource is where an import reads events from: Kind journal (the SDK's event journal; Events are
// the event kinds, default the triggers of the automation's latest version), connection (the history
// of ConnectionID, through its connector's HistoryImporter) or module (the HistorySource named Source).
// Filter is passed to the importer as is.
type ImportSource struct {
	Kind         string          `json:"kind"`
	Window       TimeWindow      `json:"window"`
	Events       []string        `json:"events,omitempty"`
	ConnectionID uuid.UUID       `json:"connection_id,omitempty"`
	Source       string          `json:"source,omitempty"`
	Filter       json.RawMessage `json:"filter,omitempty"`
}

// Import source kinds.
const (
	ImportJournal    = "journal"
	ImportConnection = "connection"
	ImportModule     = "module"
)

// ImportJob is an import into a dataset and what it brought.
type ImportJob struct {
	ID        ImportJobID
	DatasetID DatasetID
	Source    ImportSource
	Status    string
	Imported  int
	// Duplicates are events already in the dataset; Overflow are events left out because the dataset
	// reached MaxExamples; Unreadable are events the actor may not read or that are not valid.
	Duplicates int
	Overflow   int
	Unreadable int
	Error      string
	CreatedAt  time.Time
	FinishedAt *time.Time
}

// SourceOption is a history source an author may import from.
type SourceOption struct {
	Kind          string // connection | module
	ConnectionID  uuid.UUID
	Source        string
	Title         string
	ConnectorKind string
}

// SyntheticEvent is an event an author writes: its kind and payload, the subject and its state (nil: the
// subject's record as the author reads it now), and the expected outcomes by node.
type SyntheticEvent struct {
	Kind       string
	Payload    json.RawMessage
	Subject    engine.SubjectRef
	Snapshot   json.RawMessage
	Title      string
	OccurredAt time.Time
	Expected   map[string]string
}

// Example is an event of a dataset.
type Example struct {
	ID        ExampleID
	DatasetID DatasetID
	Source    string
	SourceRef string
	Title     string
	Event     json.RawMessage
	EventKind string
	Subject   engine.SubjectRef
	// Snapshot is the subject's state the example runs with; reads never return it (it was read with the
	// importer's rights at the import). HasSnapshot says whether there is one.
	Snapshot            json.RawMessage
	HasSnapshot         bool
	SnapshotApproximate bool
	// SnapshotReadWith are the permissions of the author (CreatedBy) the snapshot was read with; reads
	// never return them.
	SnapshotReadWith []string
	OccurredAt       time.Time
	// Origin is the source within its kind ("connection:<id>", "module:<name>"); CreatedBy imported it.
	Origin    string
	CreatedBy int64
	// Hidden: the reader may not read the event (the history of a connection or a module's source they
	// have no right to): it keeps its kind only.
	Hidden bool
	// Labels are the example's labels by node; Label.Source says which one is in force.
	Labels map[string]Label
}

// Label is the expected outcome of a node in force for an example, and the expected one it overrides.
type Label struct {
	Outcome  string
	Source   string // manual | expected
	Expected string // the synthetic event's expected outcome when a manual label overrides it
}

// ExampleFilter selects examples of a dataset.
type ExampleFilter struct {
	Cursor string
	Limit  int
	// Labeled keeps examples with (true) or without (false) a label.
	Labeled *bool
}

// RunInput is a backtest of a version over a dataset.
type RunInput struct {
	DatasetID DatasetID
	Version   int    // 0: the latest version
	Decisions string // cached (default) | live
	// Revision, when set, is Estimate.Revision: the run is refused (ErrStaleEstimate) when the dataset's
	// examples or the version run (the latest, for 0) changed since, so it never runs more than the
	// estimate counted. A run with live decisions requires it.
	Revision string
}

// Estimate is what a backtest would do before it runs.
type Estimate struct {
	Examples int
	// Revision names the examples counted and the version whose decision nodes were counted; Run takes it
	// back. It is never empty.
	Revision string
	// DecisionNodes are the graph's nodes whose answers are decisions of models.
	DecisionNodes []string
	// MaxCalls bounds the calls to providers: every decision node on every example, times
	// MaxDecisionsPerItem when the graph has a foreach (PerItem). A backtest enforces the bound per example
	// (an example past it ends with an error); a cached backtest makes only the calls the cache lacks.
	MaxCalls int
	PerItem  bool
	// CostUSD is MaxCalls at the average cost of the nodes' decisions in earlier backtests of the
	// automation; nil when no cost is known.
	CostUSD *float64
	// AgentCallsBlocked: an agent's cached backtest reads the cache only.
	AgentCallsBlocked bool
}

// Backtest is a backtest and its progress.
type Backtest struct {
	ID            BacktestID
	AutomationID  uuid.UUID
	DatasetID     DatasetID
	DatasetName   string
	Version       int
	Decisions     string
	Status        string
	Total         int
	Done          int
	LiveCalls     int
	Error         string
	CreatedBy     int64
	CreatedOrigin engine.Origin
	CreatedAt     time.Time
	StartedAt     *time.Time
	FinishedAt    *time.Time
}

// Decision is a model's decision for an example.
type Decision struct {
	Node         string   `json:"node"`
	Outcome      string   `json:"outcome"`
	Status       string   `json:"status"` // live | cached | missing
	CostUSD      *float64 `json:"cost_usd,omitempty"`
	InputTokens  int64    `json:"input_tokens,omitempty"`
	OutputTokens int64    `json:"output_tokens,omitempty"`
}

// Effect is an effect the graph would have made: the node and its rendered params.
type Effect struct {
	Node   string          `json:"node"`
	Params json.RawMessage `json:"params"`
}

// Result is what the graph did for one example of a backtest.
type Result struct {
	Example   Example
	End       string
	Outcomes  map[string]string
	Decisions []Decision
	Effects   []Effect
	Trace     []engine.TraceStep
	Error     string
	LiveCalls int
	// SnapshotWithheld: the example has a snapshot of its subject the runner may not read now; it ran
	// without the subject's state.
	SnapshotWithheld bool
	// Mismatches are the labeled nodes whose outcome is not the label.
	Mismatches []string
}

// ResultFilter selects results of a backtest.
type ResultFilter struct {
	Cursor string
	Limit  int
	// Node keeps the examples where the node has a label (with Mismatch: where it is not the outcome).
	Node     string
	Mismatch bool
	// End keeps the examples that ended so (ended | waiting | not_triggered | error).
	End string
}

// OutcomeMetrics are the counts and rates of one outcome of a classifier over the labeled examples.
type OutcomeMetrics struct {
	Outcome        string
	TruePositives  int
	FalsePositives int
	FalseNegatives int
	// Support is how many labeled examples expect the outcome.
	Support int
	// Precision and Recall are nil when undefined (nothing predicted, nothing expected).
	Precision *float64
	Recall    *float64
}

// ClassifierMetrics are the metrics of a node with labels or decisions.
type ClassifierMetrics struct {
	Node string
	// Decision: the node is a model's decision.
	Decision bool
	// Labeled counts the examples with a label for the node; Correct those whose outcome is the label;
	// Undecided those whose decision is missing from the cache (an agent's cached backtest), left out
	// of the metrics.
	Labeled   int
	Correct   int
	Undecided int
	Accuracy  *float64
	Outcomes  []OutcomeMetrics
	// Distribution counts the outcomes the node took over all examples.
	Distribution map[string]int
}

// Mismatch is an example whose outcome of a node is not its label.
type Mismatch struct {
	ExampleID   ExampleID
	Title       string
	OccurredAt  time.Time
	Node        string
	Expected    string
	Actual      string
	LabelSource string
}

// EffectSummary counts the effects of a node over the examples.
type EffectSummary struct {
	Node     string
	Examples int
	Count    int
	// Samples are the params of the node's first effects.
	Samples []json.RawMessage
}

// Cost is what the decisions of a backtest cost: Live is what the calls made cost, Cached what the
// decisions taken from the cache cost when they were made. Unknown counts the decisions whose provider
// did not report a cost.
type Cost struct {
	LiveCalls    int
	CachedCalls  int
	Missing      int
	LiveUSD      float64
	CachedUSD    float64
	InputTokens  int64
	OutputTokens int64
	UnknownCost  int
}

// Report is the report of a backtest.
type Report struct {
	Backtest Backtest
	Examples int
	// Triggered counts the examples on which a trigger fired, those that then stopped with an error included.
	Triggered int
	// SnapshotsWithheld counts the examples that ran without their subject's snapshot, which the runner
	// may not read now: their results are not those of the state at the event.
	SnapshotsWithheld int
	Ends              map[string]int
	Classifiers       []ClassifierMetrics
	Mismatches        []Mismatch
	// MismatchTotal counts all the mismatches (Mismatches lists at most MaxMismatches).
	MismatchTotal int
	Effects       []EffectSummary
	Cost          Cost
}

// OutcomeChange is a node whose outcome differs between two backtests of an example.
type OutcomeChange struct {
	Node     string
	A, B     string
	Expected string
}

// ExampleDiff is an example on which two backtests differ.
type ExampleDiff struct {
	ExampleID  ExampleID
	Title      string
	OccurredAt time.Time
	EndA, EndB string
	Changes    []OutcomeChange
	// EffectsA and EffectsB are the nodes of the effects each would have made, when the effects differ;
	// ParamsChanged: the same nodes, with other rendered params (a message's text changed).
	EffectsA, EffectsB []string
	ParamsChanged      bool
}

// Comparison puts two backtests side by side over the examples they share.
type Comparison struct {
	A, B           Report
	CommonExamples int
	Changed        int
	// Diffs lists at most MaxMismatches examples that differ (Changed counts them all).
	Diffs []ExampleDiff
}

// Event is an event a history importer emits. Key is its identity in the source (a message id): an
// event already in the dataset is not imported again. An event without a key is identified by its kind,
// time, subject, BySelf and its whole Payload as emitted (before the fields the importer may not read
// or no schema declares are dropped): two emissions that differ in any of them, a dropped field
// included, are two examples, even when what is stored of them is the same. A source whose events carry
// volatile fields (a delivery attempt, a fetch time) should give each its Key. Snapshot, when set, is the subject's state at the
// event as the source knows it (exact).
type Event struct {
	Kind       string
	Key        string
	Title      string
	Subject    engine.SubjectRef
	Payload    json.RawMessage
	OccurredAt time.Time
	BySelf     bool
	Snapshot   json.RawMessage
}

// ConnectionRef is the connection a history importer reads, opened: with its credentials. A module's
// source gets no Connection and no Credentials but always As: it checks the records it emits against
// As (the person the import runs for, with their rights now).
type ConnectionRef struct {
	Connection  connectors.Connection
	Credentials connectors.Credentials
	// As is the actor the import runs for.
	As spi.Subject
}

// HistoryImporter is implemented by connectors and modules that can return their history: the events
// of a window, in any order, as they would have been published then. emit stops the import when it
// returns an error (the dataset is full, the context is done): the importer returns that error.
type HistoryImporter interface {
	ImportHistory(ctx context.Context, conn ConnectionRef, window TimeWindow, filter json.RawMessage, emit func(Event) error) error
}

// HistorySource is a module's source of history (records of a module as events).
type HistorySource struct {
	Name  string // a stable key, e.g. "crm.clients"
	Title string
	// Permission, when set, is required to import from the source.
	Permission string
	// Events are the event kinds the source emits: it is offered for automations triggered by one.
	Events   []string
	Importer HistoryImporter
}

// SnapshotLoader is an optional interface of the engine's SubjectLoader: the subject's variables as they
// were at a time. ErrNotFound when the record did not exist or is not visible.
type SnapshotLoader interface {
	SnapshotAt(ctx context.Context, as spi.Subject, ref engine.SubjectRef, at time.Time) (json.RawMessage, error)
}
