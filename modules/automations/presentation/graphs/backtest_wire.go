package graphs

import "encoding/json"

// Wire types of the backtests section (#1389): /automations/{id}/backtests.

// BacktestStatus is the status of an import or a backtest.
type BacktestStatus string

const (
	BacktestQueued    BacktestStatus = "queued"
	BacktestRunning   BacktestStatus = "running"
	BacktestDone      BacktestStatus = "done"
	BacktestFailed    BacktestStatus = "failed"
	BacktestCancelled BacktestStatus = "cancelled"
)

// ImportKind is where an import reads events.
type ImportKind string

const (
	ImportJournal    ImportKind = "journal"
	ImportConnection ImportKind = "connection"
	ImportModule     ImportKind = "module"
)

// ExampleSource is where an example came from.
type ExampleSource string

const (
	SourceJournal   ExampleSource = "journal"
	SourceHistory   ExampleSource = "history"
	SourceSynthetic ExampleSource = "synthetic"
)

// LabelSource says which label is in force: a person's (manual) or the synthetic event's (expected).
type LabelSource string

const (
	LabelManual   LabelSource = "manual"
	LabelExpected LabelSource = "expected"
)

// EndKind is how the graph's run over an example ended.
type EndKind string

const (
	EndEnded        EndKind = "ended"
	EndWaiting      EndKind = "waiting"
	EndNotTriggered EndKind = "not_triggered"
	EndError        EndKind = "error"
)

// DecisionStatus is how a decision was made.
type DecisionStatus string

const (
	DecisionLive    DecisionStatus = "live"
	DecisionCached  DecisionStatus = "cached"
	DecisionMissing DecisionStatus = "missing"
)

// AutomationRequest names an automation.
type AutomationRequest struct {
	AutomationID string `json:"automation_id"`
}

// BacktestCountResponse is automations.backtest_count: the editor's «Backtests: N».
type BacktestCountResponse struct {
	Count int `json:"count"`
}

// ImportView is an import job.
type ImportView struct {
	ID         string         `json:"id"`
	Kind       ImportKind     `json:"kind"`
	Status     BacktestStatus `json:"status"`
	Imported   int            `json:"imported"`
	Duplicates int            `json:"duplicates"`
	Overflow   int            `json:"overflow"`
	Unreadable int            `json:"unreadable"`
	// Error is the reason a failed import gives (English, for the log; the screen says it in words).
	Error      *string `json:"error"`
	CreatedAt  string  `json:"created_at"`
	FinishedAt *string `json:"finished_at"`
}

// DatasetView is a dataset.
type DatasetView struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Examples    int          `json:"examples"`
	Labeled     int          `json:"labeled"`
	CreatedBy   string       `json:"created_by"`
	CreatedAt   string       `json:"created_at"`
	Imports     []ImportView `json:"imports"`
}

// BacktestView is a backtest and its progress.
type BacktestView struct {
	ID          string         `json:"id"`
	DatasetID   string         `json:"dataset_id"`
	DatasetName string         `json:"dataset_name"`
	Version     int            `json:"version"`
	Decisions   Decisions      `json:"decisions"`
	Status      BacktestStatus `json:"status"`
	Total       int            `json:"total"`
	Done        int            `json:"done"`
	LiveCalls   int            `json:"live_calls"`
	Error       *string        `json:"error"`
	CreatedBy   string         `json:"created_by"`
	ByAgent     bool           `json:"by_agent"`
	CreatedAt   string         `json:"created_at"`
	FinishedAt  *string        `json:"finished_at"`
}

// SourceView is a history source the author may import from.
type SourceView struct {
	Kind          ImportKind `json:"kind"`
	ConnectionID  string     `json:"connection_id,omitempty"`
	Source        string     `json:"source,omitempty"`
	Title         string     `json:"title"`
	ConnectorKind string     `json:"connector_kind,omitempty"`
}

// BacktestsResponse is automations.backtests: what the section shows.
type BacktestsResponse struct {
	Datasets  []DatasetView  `json:"datasets"`
	Backtests []BacktestView `json:"backtests"`
	Sources   []SourceView   `json:"sources"`
	// Live: the caller may run a backtest with live decisions (a person, not an agent).
	Live bool `json:"live"`
	// DecisionNodes are the nodes of the latest version whose answers are decisions of models.
	DecisionNodes []string `json:"decision_nodes"`
}

// DatasetCreateRequest is automations.dataset_create.
type DatasetCreateRequest struct {
	AutomationID string `json:"automation_id"`
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
}

// IDResponse names what was created.
type IDResponse struct {
	ID string `json:"id"`
}

// DatasetRequest names a dataset.
type DatasetRequest struct {
	DatasetID string `json:"dataset_id"`
}

// DatasetImportRequest is automations.dataset_import: the last Days days (default 30), or From..To
// (RFC 3339).
type DatasetImportRequest struct {
	DatasetID    string          `json:"dataset_id"`
	Kind         ImportKind      `json:"kind"`
	Days         int             `json:"days,omitempty"`
	From         string          `json:"from,omitempty"`
	To           string          `json:"to,omitempty"`
	Events       []string        `json:"events,omitempty"`
	ConnectionID string          `json:"connection_id,omitempty"`
	Source       string          `json:"source,omitempty"`
	Filter       json.RawMessage `json:"filter,omitempty"`
}

// ExamplesRequest is automations.dataset_examples.
type ExamplesRequest struct {
	DatasetID string `json:"dataset_id"`
	Cursor    string `json:"cursor,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	Labeled   *bool  `json:"labeled,omitempty"`
}

// LabelView is the label in force of a node.
type LabelView struct {
	Outcome string      `json:"outcome"`
	Source  LabelSource `json:"source"`
	// Expected is the synthetic event's outcome a manual label overrides.
	Expected string `json:"expected,omitempty"`
}

// ExampleView is an example of a dataset.
type ExampleView struct {
	ID                  string          `json:"id"`
	Source              ExampleSource   `json:"source"`
	Title               string          `json:"title"`
	EventKind           string          `json:"event_kind"`
	Event               json.RawMessage `json:"event"`
	Subject             Subject         `json:"subject"`
	SnapshotApproximate bool            `json:"snapshot_approximate"`
	// Hidden: the reader may not read the event (history of a connection or a module source they have
	// no right to): it keeps its kind only.
	Hidden      bool                 `json:"hidden,omitempty"`
	HasSnapshot bool                 `json:"has_snapshot"`
	OccurredAt  string               `json:"occurred_at"`
	Labels      map[string]LabelView `json:"labels"`
}

// ExamplesResponse is a page of examples.
type ExamplesResponse struct {
	Items      []ExampleView `json:"items"`
	NextCursor *string       `json:"next_cursor"`
	Total      int           `json:"total"`
}

// SyntheticRequest is automations.dataset_synthetic: an event the author writes and its expected
// outcomes by node.
type SyntheticRequest struct {
	DatasetID string            `json:"dataset_id"`
	Kind      string            `json:"kind"`
	Payload   json.RawMessage   `json:"payload"`
	Subject   *Subject          `json:"subject,omitempty"`
	Title     string            `json:"title,omitempty"`
	Expected  map[string]string `json:"expected,omitempty"`
}

// ExampleRequest names an example.
type ExampleRequest struct {
	ExampleID string `json:"example_id"`
}

// LabelRequest is automations.example_label; outcome "" removes the manual label.
type LabelRequest struct {
	ExampleID string `json:"example_id"`
	Node      string `json:"node"`
	Outcome   string `json:"outcome"`
}

// BacktestRunRequest is automations.backtest_run and automations.backtest_estimate.
type BacktestRunRequest struct {
	DatasetID string    `json:"dataset_id"`
	Version   int       `json:"version,omitempty"`
	Decisions Decisions `json:"decisions,omitempty"`
	// Revision is the estimate's: a run is refused with estimate_stale when the examples or the version
	// run (the latest, for 0) changed since. A run with live decisions requires it.
	Revision string `json:"revision,omitempty"`
}

// EstimateResponse is what a backtest would call.
type EstimateResponse struct {
	Examples int `json:"examples"`
	// Revision names the examples counted; the run passes it back.
	Revision      string   `json:"revision"`
	DecisionNodes []string `json:"decision_nodes"`
	MaxCalls      int      `json:"max_calls"`
	// PerItem: the graph has a foreach, so MaxCalls counts up to MaxDecisionsPerItem decisions of a node
	// per example (a backtest enforces it).
	PerItem bool `json:"per_item"`
	// CostUSD is nil when no cost of these decisions is known yet.
	CostUSD *float64 `json:"cost_usd"`
}

// BacktestRequest names a backtest.
type BacktestRequest struct {
	BacktestID string `json:"backtest_id"`
}

// OutcomeMetricsView are the metrics of one outcome.
type OutcomeMetricsView struct {
	Outcome        string   `json:"outcome"`
	TruePositives  int      `json:"tp"`
	FalsePositives int      `json:"fp"`
	FalseNegatives int      `json:"fn"`
	Support        int      `json:"support"`
	Precision      *float64 `json:"precision"`
	Recall         *float64 `json:"recall"`
}

// ClassifierView are the metrics of a classifier.
type ClassifierView struct {
	Node         string               `json:"node"`
	Decision     bool                 `json:"decision"`
	Labeled      int                  `json:"labeled"`
	Correct      int                  `json:"correct"`
	Undecided    int                  `json:"undecided"`
	Accuracy     *float64             `json:"accuracy"`
	Outcomes     []OutcomeMetricsView `json:"outcomes"`
	Distribution map[string]int       `json:"distribution"`
}

// MismatchView is a mismatch with a label.
type MismatchView struct {
	ExampleID   string      `json:"example_id"`
	Title       string      `json:"title"`
	OccurredAt  string      `json:"occurred_at"`
	Node        string      `json:"node"`
	Expected    string      `json:"expected"`
	Actual      string      `json:"actual"`
	LabelSource LabelSource `json:"label_source"`
}

// EffectSummaryView counts the effects of a node.
type EffectSummaryView struct {
	Node     string            `json:"node"`
	Examples int               `json:"examples"`
	Count    int               `json:"count"`
	Samples  []json.RawMessage `json:"samples"`
}

// CostView is what the decisions cost.
type CostView struct {
	LiveCalls    int     `json:"live_calls"`
	CachedCalls  int     `json:"cached_calls"`
	Missing      int     `json:"missing"`
	LiveUSD      float64 `json:"live_usd"`
	CachedUSD    float64 `json:"cached_usd"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	UnknownCost  int     `json:"unknown_cost"`
}

// ReportResponse is automations.backtest_report.
type ReportResponse struct {
	Backtest BacktestView `json:"backtest"`
	Examples int          `json:"examples"`
	// Triggered counts the examples on which a trigger fired (those that then stopped with an error too).
	Triggered     int                 `json:"triggered"`
	Ends          map[string]int      `json:"ends"`
	Classifiers   []ClassifierView    `json:"classifiers"`
	Mismatches    []MismatchView      `json:"mismatches"`
	MismatchTotal int                 `json:"mismatch_total"`
	Effects       []EffectSummaryView `json:"effects"`
	Cost          CostView            `json:"cost"`
}

// ResultsRequest is automations.backtest_results.
type ResultsRequest struct {
	BacktestID string  `json:"backtest_id"`
	Node       string  `json:"node,omitempty"`
	Mismatch   bool    `json:"mismatch,omitempty"`
	End        EndKind `json:"end,omitempty"`
	Cursor     string  `json:"cursor,omitempty"`
	Limit      int     `json:"limit,omitempty"`
}

// DecisionView is a decision of a model for an example.
type DecisionView struct {
	Node    string         `json:"node"`
	Outcome string         `json:"outcome"`
	Status  DecisionStatus `json:"status"`
	CostUSD *float64       `json:"cost_usd"`
}

// EffectView is an effect the graph would have made.
type EffectView struct {
	Node   string          `json:"node"`
	Params json.RawMessage `json:"params"`
}

// ResultView is what the graph did for an example.
type ResultView struct {
	Example    ExampleView       `json:"example"`
	End        EndKind           `json:"end"`
	Outcomes   map[string]string `json:"outcomes"`
	Decisions  []DecisionView    `json:"decisions"`
	Effects    []EffectView      `json:"effects"`
	Trace      []TraceStep       `json:"trace"`
	Error      string            `json:"error,omitempty"`
	Mismatches []string          `json:"mismatches"`
}

// ResultsResponse is a page of results.
type ResultsResponse struct {
	Items      []ResultView `json:"items"`
	NextCursor *string      `json:"next_cursor"`
	Total      int          `json:"total"`
}

// CompareRequest is automations.backtest_compare.
type CompareRequest struct {
	A string `json:"a"`
	B string `json:"b"`
}

// OutcomeChangeView is a node whose outcome differs between two backtests.
type OutcomeChangeView struct {
	Node     string `json:"node"`
	A        string `json:"a"`
	B        string `json:"b"`
	Expected string `json:"expected,omitempty"`
}

// ExampleDiffView is an example on which two backtests differ.
type ExampleDiffView struct {
	ExampleID  string              `json:"example_id"`
	Title      string              `json:"title"`
	OccurredAt string              `json:"occurred_at"`
	EndA       EndKind             `json:"end_a"`
	EndB       EndKind             `json:"end_b"`
	Changes    []OutcomeChangeView `json:"changes"`
	EffectsA   []string            `json:"effects_a"`
	EffectsB   []string            `json:"effects_b"`
	// ParamsChanged: the same effects with other params (a message's text changed).
	ParamsChanged bool `json:"params_changed,omitempty"`
}

// CompareResponse is automations.backtest_compare.
type CompareResponse struct {
	A              ReportResponse    `json:"a"`
	B              ReportResponse    `json:"b"`
	CommonExamples int               `json:"common_examples"`
	Changed        int               `json:"changed"`
	Diffs          []ExampleDiffView `json:"diffs"`
}
