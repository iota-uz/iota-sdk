package mcpops

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/backtest"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
)

// Names of the backtest tools (#1389).
const (
	ToolBacktests        = "automation_backtests"
	ToolBacktestDataset  = "automation_backtest_dataset_create"
	ToolBacktestExamples = "automation_backtest_examples_add"
	ToolBacktestLabel    = "automation_backtest_label"
	ToolBacktestRun      = "automation_backtest_run"
	ToolBacktestReport   = "automation_backtest_report"
)

// MaxBacktestEvents bounds the synthetic events of one automation_backtest_examples_add.
const MaxBacktestEvents = 100

// BacktestURLs is an optional interface of URLs: the page of an automation's backtests.
type BacktestURLs interface {
	Backtests(id uuid.UUID) string
}

// WithBacktests adds the backtest tools to Tools: an agent lists, creates and fills datasets, labels
// examples, runs a backtest with cached decisions (the decision cache only: no call to a provider) and
// reads its report. Live decisions are a person's: the tool does not offer them.
func WithBacktests(svc backtest.Service) Option { return func(o *options) { o.backtests = svc } }

func (t *tools) backtestTools(add func(mcp.Tool, error)) {
	if t.o.backtests == nil {
		return
	}
	add(newTool(t.o, mcp.Spec{
		Name:        ToolBacktests,
		Description: "List the backtest datasets of an automation and its backtests, newest first, with their progress. Requires " + engine.PermAuthor + ".",
		Input:       objectSchema(map[string]*jsonschema.Schema{"id": {Type: "string", Format: "uuid"}}, "id"),
		Annotations: mcp.Annotations{Title: "List automation backtests", ReadOnly: true, Idempotent: true}, Policy: AuthorPolicy,
	}, t.backtestList))
	add(newTool(t.o, mcp.Spec{
		Name: ToolBacktestDataset,
		Description: "Create a dataset of events for backtesting an automation (at most 20 per automation, 5000 events each). " +
			"journal_days, when set, imports the events of the SDK's journal of the last N days that start the automation's " +
			"triggers (in the background: read automation_backtests for the import's result). Requires " + engine.PermAuthor + ".",
		Input: objectSchema(map[string]*jsonschema.Schema{
			"id":           {Type: "string", Format: "uuid"},
			"name":         {Type: "string", MinLength: jsonschema.Ptr(1), MaxLength: jsonschema.Ptr(200)},
			"description":  {Type: "string", MaxLength: jsonschema.Ptr(2000)},
			"journal_days": {Type: "integer", Minimum: jsonschema.Ptr(1.0), Maximum: jsonschema.Ptr(366.0)},
		}, "id", "name"),
		Annotations: mcp.Annotations{Title: "Create backtest dataset"}, Policy: AuthorPolicy,
	}, t.backtestDatasetCreate))
	add(newTool(t.o, mcp.Spec{
		Name: ToolBacktestExamples,
		Description: "Add synthetic events to a dataset: each {kind, payload, subject?, title?, expected?}; expected maps a node id " +
			"to the outcome it should take, counted in precision and recall (a person's label overrides it). Requires " + engine.PermAuthor + ".",
		Input: objectSchema(map[string]*jsonschema.Schema{
			"dataset_id": {Type: "string", Format: "uuid"},
			"events": {Type: "array", MinItems: jsonschema.Ptr(1), MaxItems: jsonschema.Ptr(MaxBacktestEvents), Items: &jsonschema.Schema{
				Type: "object", Required: []string{"kind"},
				Properties: map[string]*jsonschema.Schema{
					"kind":     {Type: "string"},
					"payload":  {Type: "object"},
					"subject":  {Type: "object", Properties: map[string]*jsonschema.Schema{"entity": {Type: "string"}, "key": {Type: "string"}}},
					"title":    {Type: "string", MaxLength: jsonschema.Ptr(200)},
					"expected": {Type: "object", AdditionalProperties: &jsonschema.Schema{Type: "string"}},
				},
			}},
		}, "dataset_id", "events"),
		Annotations: mcp.Annotations{Title: "Add backtest examples"}, Policy: AuthorPolicy,
	}, t.backtestExamplesAdd))
	add(newTool(t.o, mcp.Spec{
		Name:        ToolBacktestLabel,
		Description: "Set the expected outcome of a node for an example of a dataset (outcome empty: remove the label). Requires " + engine.PermAuthor + ".",
		Input: objectSchema(map[string]*jsonschema.Schema{
			"example_id": {Type: "string", Format: "uuid"},
			"node":       {Type: "string", MinLength: jsonschema.Ptr(1), MaxLength: jsonschema.Ptr(100)},
			"outcome":    {Type: "string", MaxLength: jsonschema.Ptr(100)},
		}, "example_id", "node", "outcome"),
		Annotations: mcp.Annotations{Title: "Label backtest example", Idempotent: true}, Policy: AuthorPolicy,
	}, t.backtestLabel))
	add(newTool(t.o, mcp.Spec{
		Name: ToolBacktestRun,
		Description: "Run a version of the automation (the latest by default) over a dataset in the background, with cached decisions: " +
			"a classifier's or an LLM's decision comes from the decision cache, and one the cache lacks is reported missing (no call " +
			"to a provider is made). Effects are captured, nothing is sent. Live decisions are a person's, on the backtests page. " +
			"Read the result with " + ToolBacktestReport + ". Requires " + engine.PermAuthor + ".",
		Input: objectSchema(map[string]*jsonschema.Schema{
			"dataset_id": {Type: "string", Format: "uuid"},
			"version":    {Type: "integer", Minimum: jsonschema.Ptr(1.0)},
		}, "dataset_id"),
		Annotations: mcp.Annotations{Title: "Run automation backtest"}, Policy: AuthorPolicy,
	}, t.backtestRun))
	add(newTool(t.o, mcp.Spec{
		Name: ToolBacktestReport,
		Description: "Read the report of a backtest: its progress, precision and recall per classifier and outcome, the mismatches " +
			"with the labels, the effects the graph would have made and the cost of its decisions. compare_with, another backtest " +
			"of the automation, adds the examples on which the two differ. Requires " + engine.PermAuthor + ".",
		Input: objectSchema(map[string]*jsonschema.Schema{
			"backtest_id":  {Type: "string", Format: "uuid"},
			"compare_with": {Type: "string", Format: "uuid"},
		}, "backtest_id"),
		Annotations: mcp.Annotations{Title: "Read backtest report", ReadOnly: true, Idempotent: true}, Policy: AuthorPolicy,
	}, t.backtestReport))
}

func (t *tools) backtestsURL(id uuid.UUID) string {
	if u, ok := t.urls.(BacktestURLs); ok {
		return u.Backtests(id)
	}
	return t.urls.Editor(id, 0)
}

type importOut struct {
	ID         string  `json:"id"`
	Kind       string  `json:"kind"`
	Status     string  `json:"status"`
	Imported   int     `json:"imported"`
	Duplicates int     `json:"duplicates"`
	Overflow   int     `json:"overflow"`
	Unreadable int     `json:"unreadable"`
	Error      *string `json:"error"`
}

type datasetOut struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Examples int         `json:"examples"`
	Labeled  int         `json:"labeled"`
	Imports  []importOut `json:"imports"`
}

type backtestOut struct {
	ID        string  `json:"id"`
	DatasetID string  `json:"dataset_id"`
	Version   int     `json:"version"`
	Decisions string  `json:"decisions"`
	Status    string  `json:"status"`
	Total     int     `json:"total"`
	Done      int     `json:"done"`
	LiveCalls int     `json:"live_calls"`
	Error     *string `json:"error"`
	CreatedAt string  `json:"created_at"`
}

type backtestsOut struct {
	Datasets  []datasetOut  `json:"datasets"`
	Backtests []backtestOut `json:"backtests"`
	PageURL   string        `json:"page_url"`
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func importOf(j backtest.ImportJob) importOut {
	return importOut{ID: j.ID.String(), Kind: j.Source.Kind, Status: j.Status, Imported: j.Imported, Duplicates: j.Duplicates,
		Overflow: j.Overflow, Unreadable: j.Unreadable, Error: strPtr(j.Error)}
}

func backtestOf(b backtest.Backtest) backtestOut {
	return backtestOut{ID: b.ID.String(), DatasetID: b.DatasetID.String(), Version: b.Version, Decisions: b.Decisions, Status: b.Status,
		Total: b.Total, Done: b.Done, LiveCalls: b.LiveCalls, Error: strPtr(b.Error), CreatedAt: timeString(b.CreatedAt)}
}

type idIn struct {
	ID string `json:"id"`
}

func (t *tools) backtestList(ctx context.Context, a engine.Actor, in idIn) (backtestsOut, error) {
	id, err := parseID("id", in.ID)
	if err != nil {
		return backtestsOut{}, err
	}
	sets, err := t.o.backtests.ListDatasets(ctx, a, id)
	if err != nil {
		return backtestsOut{}, err
	}
	runs, err := t.o.backtests.ListBacktests(ctx, a, id)
	if err != nil {
		return backtestsOut{}, err
	}
	out := backtestsOut{Datasets: make([]datasetOut, 0, len(sets)), Backtests: make([]backtestOut, 0, len(runs)), PageURL: t.backtestsURL(id)}
	for _, d := range sets {
		do := datasetOut{ID: d.ID.String(), Name: d.Name, Examples: d.Examples, Labeled: d.Labeled, Imports: make([]importOut, 0, len(d.Imports))}
		for _, j := range d.Imports {
			do.Imports = append(do.Imports, importOf(j))
		}
		out.Datasets = append(out.Datasets, do)
	}
	for _, b := range runs {
		out.Backtests = append(out.Backtests, backtestOf(b))
	}
	return out, nil
}

type datasetCreateIn struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	JournalDays int    `json:"journal_days"`
}

type datasetCreateOut struct {
	DatasetID string  `json:"dataset_id"`
	ImportID  *string `json:"import_id"`
	PageURL   string  `json:"page_url"`
}

func (t *tools) backtestDatasetCreate(ctx context.Context, a engine.Actor, in datasetCreateIn) (datasetCreateOut, error) {
	id, err := parseID("id", in.ID)
	if err != nil {
		return datasetCreateOut{}, err
	}
	ds, err := t.o.backtests.CreateDataset(ctx, a, backtest.CreateDatasetInput{AutomationID: id, Name: in.Name, Description: in.Description})
	if err != nil {
		return datasetCreateOut{}, backtestFailure(err)
	}
	out := datasetCreateOut{DatasetID: ds.String(), PageURL: t.backtestsURL(id)}
	if in.JournalDays > 0 {
		from := time.Now().Add(-time.Duration(in.JournalDays) * 24 * time.Hour)
		job, err := t.o.backtests.Import(ctx, a, ds, backtest.ImportSource{Kind: backtest.ImportJournal, Window: backtest.TimeWindow{From: from}})
		if err != nil {
			return datasetCreateOut{}, backtestFailure(err)
		}
		s := job.String()
		out.ImportID = &s
	}
	return out, nil
}

type syntheticIn struct {
	Kind     string            `json:"kind"`
	Payload  json.RawMessage   `json:"payload"`
	Subject  engine.SubjectRef `json:"subject"`
	Title    string            `json:"title"`
	Expected map[string]string `json:"expected"`
}

type examplesAddIn struct {
	DatasetID string        `json:"dataset_id"`
	Events    []syntheticIn `json:"events"`
}

type examplesAddOut struct {
	ExampleIDs []string `json:"example_ids"`
}

func (t *tools) backtestExamplesAdd(ctx context.Context, a engine.Actor, in examplesAddIn) (examplesAddOut, error) {
	ds, err := parseID("dataset_id", in.DatasetID)
	if err != nil {
		return examplesAddOut{}, err
	}
	if len(in.Events) == 0 || len(in.Events) > MaxBacktestEvents {
		return examplesAddOut{}, mcp.Fail(mcp.CodeInvalidInput, mcp.RecoveryFixInput, "events holds 1.."+strconv.Itoa(MaxBacktestEvents)+" events")
	}
	out := examplesAddOut{ExampleIDs: make([]string, 0, len(in.Events))}
	for i, ev := range in.Events {
		id, err := t.o.backtests.AddSynthetic(ctx, a, ds, backtest.SyntheticEvent{Kind: ev.Kind, Payload: ev.Payload, Subject: ev.Subject,
			Title: ev.Title, Expected: ev.Expected})
		if err != nil {
			f := backtestFailure(err)
			var mf *mcp.Failure
			if errors.As(f, &mf) && len(out.ExampleIDs) > 0 {
				mf.Message = "events[" + strconv.Itoa(i) + "]: " + mf.Message + " (the events before it were added: " + strconv.Itoa(len(out.ExampleIDs)) + ")"
			}
			return examplesAddOut{}, f
		}
		out.ExampleIDs = append(out.ExampleIDs, id.String())
	}
	return out, nil
}

type labelIn struct {
	ExampleID string `json:"example_id"`
	Node      string `json:"node"`
	Outcome   string `json:"outcome"`
}

type okOut struct {
	OK bool `json:"ok"`
}

func (t *tools) backtestLabel(ctx context.Context, a engine.Actor, in labelIn) (okOut, error) {
	id, err := parseID("example_id", in.ExampleID)
	if err != nil {
		return okOut{}, err
	}
	if err := t.o.backtests.Label(ctx, a, id, in.Node, in.Outcome); err != nil {
		return okOut{}, backtestFailure(err)
	}
	return okOut{OK: true}, nil
}

type runIn struct {
	DatasetID string `json:"dataset_id"`
	Version   int    `json:"version"`
}

type runOut struct {
	BacktestID string `json:"backtest_id"`
	PageURL    string `json:"page_url"`
}

func (t *tools) backtestRun(ctx context.Context, a engine.Actor, in runIn) (runOut, error) {
	ds, err := parseID("dataset_id", in.DatasetID)
	if err != nil {
		return runOut{}, err
	}
	d, err := t.o.backtests.GetDataset(ctx, a, ds)
	if err != nil {
		return runOut{}, backtestFailure(err)
	}
	id, err := t.o.backtests.Run(ctx, a, backtest.RunInput{DatasetID: ds, Version: in.Version, Decisions: backtest.DecisionsCached})
	if err != nil {
		return runOut{}, backtestFailure(err)
	}
	return runOut{BacktestID: id.String(), PageURL: t.backtestsURL(d.AutomationID)}, nil
}

type reportIn struct {
	BacktestID  string `json:"backtest_id"`
	CompareWith string `json:"compare_with"`
}

type outcomeOut struct {
	Outcome        string   `json:"outcome"`
	TruePositives  int      `json:"true_positives"`
	FalsePositives int      `json:"false_positives"`
	FalseNegatives int      `json:"false_negatives"`
	Support        int      `json:"support"`
	Precision      *float64 `json:"precision"`
	Recall         *float64 `json:"recall"`
}

type classifierOut struct {
	Node         string         `json:"node"`
	Decision     bool           `json:"decision"`
	Labeled      int            `json:"labeled"`
	Correct      int            `json:"correct"`
	Undecided    int            `json:"undecided"`
	Accuracy     *float64       `json:"accuracy"`
	Outcomes     []outcomeOut   `json:"outcomes"`
	Distribution map[string]int `json:"distribution"`
}

type mismatchOut struct {
	ExampleID   string `json:"example_id"`
	Title       string `json:"title"`
	OccurredAt  string `json:"occurred_at"`
	Node        string `json:"node"`
	Expected    string `json:"expected"`
	Actual      string `json:"actual"`
	LabelSource string `json:"label_source"`
}

type effectOut struct {
	Node     string            `json:"node"`
	Examples int               `json:"examples"`
	Count    int               `json:"count"`
	Samples  []json.RawMessage `json:"samples"`
}

type costOut struct {
	LiveCalls    int     `json:"live_calls"`
	CachedCalls  int     `json:"cached_calls"`
	Missing      int     `json:"missing"`
	LiveUSD      float64 `json:"live_usd"`
	CachedUSD    float64 `json:"cached_usd"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	UnknownCost  int     `json:"unknown_cost"`
}

type reportOut struct {
	Backtest      backtestOut     `json:"backtest"`
	Examples      int             `json:"examples"`
	Ends          map[string]int  `json:"ends"`
	Classifiers   []classifierOut `json:"classifiers"`
	Mismatches    []mismatchOut   `json:"mismatches"`
	MismatchTotal int             `json:"mismatch_total"`
	Effects       []effectOut     `json:"effects"`
	Cost          costOut         `json:"cost"`
}

type changeOut struct {
	Node     string `json:"node"`
	A        string `json:"a"`
	B        string `json:"b"`
	Expected string `json:"expected,omitempty"`
}

type diffOut struct {
	ExampleID string      `json:"example_id"`
	Title     string      `json:"title"`
	EndA      string      `json:"end_a"`
	EndB      string      `json:"end_b"`
	Changes   []changeOut `json:"changes"`
	EffectsA  []string    `json:"effects_a"`
	EffectsB  []string    `json:"effects_b"`
}

type comparisonOut struct {
	Other          reportOut `json:"other"`
	CommonExamples int       `json:"common_examples"`
	Changed        int       `json:"changed"`
	Diffs          []diffOut `json:"diffs"`
}

type reportResult struct {
	Report     reportOut      `json:"report"`
	Comparison *comparisonOut `json:"comparison"`
}

func reportOf(r backtest.Report) reportOut {
	out := reportOut{Backtest: backtestOf(r.Backtest), Examples: r.Examples, Ends: r.Ends, MismatchTotal: r.MismatchTotal,
		Classifiers: make([]classifierOut, 0, len(r.Classifiers)), Mismatches: make([]mismatchOut, 0, len(r.Mismatches)),
		Effects: make([]effectOut, 0, len(r.Effects)), Cost: costOut(r.Cost)}
	for _, c := range r.Classifiers {
		co := classifierOut{Node: c.Node, Decision: c.Decision, Labeled: c.Labeled, Correct: c.Correct, Undecided: c.Undecided,
			Accuracy: c.Accuracy, Outcomes: make([]outcomeOut, 0, len(c.Outcomes)), Distribution: c.Distribution}
		for _, o := range c.Outcomes {
			co.Outcomes = append(co.Outcomes, outcomeOut(o))
		}
		out.Classifiers = append(out.Classifiers, co)
	}
	for _, m := range r.Mismatches {
		out.Mismatches = append(out.Mismatches, mismatchOut{ExampleID: m.ExampleID.String(), Title: m.Title, OccurredAt: timeString(m.OccurredAt),
			Node: m.Node, Expected: m.Expected, Actual: m.Actual, LabelSource: m.LabelSource})
	}
	for _, e := range r.Effects {
		out.Effects = append(out.Effects, effectOut{Node: e.Node, Examples: e.Examples, Count: e.Count, Samples: e.Samples})
	}
	return out
}

// maxToolDiffs bounds the differences the report tool returns.
const maxToolDiffs = 100

func (t *tools) backtestReport(ctx context.Context, a engine.Actor, in reportIn) (reportResult, error) {
	id, err := parseID("backtest_id", in.BacktestID)
	if err != nil {
		return reportResult{}, err
	}
	if in.CompareWith == "" {
		r, err := t.o.backtests.Report(ctx, a, id)
		if err != nil {
			return reportResult{}, backtestFailure(err)
		}
		return reportResult{Report: reportOf(r)}, nil
	}
	other, err := parseID("compare_with", in.CompareWith)
	if err != nil {
		return reportResult{}, err
	}
	c, err := t.o.backtests.Compare(ctx, a, id, other)
	if err != nil {
		return reportResult{}, backtestFailure(err)
	}
	co := &comparisonOut{Other: reportOf(c.B), CommonExamples: c.CommonExamples, Changed: c.Changed, Diffs: make([]diffOut, 0, min(len(c.Diffs), maxToolDiffs))}
	for i, d := range c.Diffs {
		if i == maxToolDiffs {
			break
		}
		do := diffOut{ExampleID: d.ExampleID.String(), Title: d.Title, EndA: d.EndA, EndB: d.EndB, Changes: make([]changeOut, 0, len(d.Changes)),
			EffectsA: d.EffectsA, EffectsB: d.EffectsB}
		for _, ch := range d.Changes {
			do.Changes = append(do.Changes, changeOut(ch))
		}
		co.Diffs = append(co.Diffs, do)
	}
	return reportResult{Report: reportOf(c.A), Comparison: co}, nil
}

// backtestFailure maps the backtest's own errors; the engine's are mapped by failure.
func backtestFailure(err error) error {
	if errors.Is(err, backtest.ErrUnsupported) {
		return mcp.Fail(mcp.CodeInvalidInput, mcp.RecoveryFixInput, publicMessage(err, "the source cannot import history"))
	}
	if errors.Is(err, engine.ErrHumanRequired) {
		return HumanRequired("/decisions", "run a backtest with live decisions", "")
	}
	return err
}
