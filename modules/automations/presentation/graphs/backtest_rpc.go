package graphs

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/backtest"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/clienthost/solid"
)

// Codes of the backtest refusals the screen explains.
const (
	ErrBacktestsUnavailable ErrorCode = "backtests_unavailable"
	ErrLimitDatasets        ErrorCode = "limit_datasets"
	ErrLimitExamples        ErrorCode = "limit_examples"
	ErrLimitActive          ErrorCode = "limit_active"
	ErrEmptyDataset         ErrorCode = "empty_dataset"
	ErrUnsupportedSource    ErrorCode = "unsupported_source"
	ErrBacktestFinished     ErrorCode = "backtest_finished"
	ErrEstimateStale        ErrorCode = "estimate_stale"
)

// defaultImportDays is the window of an import that names none.
const defaultImportDays = 30

// backtests is the backtest service, or the refusal the screen shows when the composition has none.
func (c *Controller) backtests() (backtest.Service, error) {
	if c.cfg.Backtests == nil {
		return nil, rpcError(ErrBacktestsUnavailable, "backtests are not available", nil)
	}
	return c.cfg.Backtests, nil
}

// backtestErr maps the backtest service's errors: limits name which one, refusals say why.
func backtestErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, backtest.ErrStaleEstimate):
		return rpcError(ErrEstimateStale, "the dataset or the latest version changed since the estimate", nil)
	case errors.Is(err, backtest.ErrUnsupported):
		return rpcError(ErrUnsupportedSource, "the source cannot import history", nil)
	case errors.Is(err, engine.ErrHumanRequired):
		return rpcError(ErrHumanRequired, "only a person runs a backtest with live decisions", nil)
	case errors.Is(err, engine.ErrRunFinished):
		return rpcError(ErrBacktestFinished, "the backtest has finished", nil)
	case errors.Is(err, engine.ErrLimitExceeded):
		msg := err.Error()
		switch {
		case strings.Contains(msg, "datasets"):
			return rpcError(ErrLimitDatasets, "an automation has at most 20 datasets", map[string]any{"limit": backtest.MaxDatasets})
		case strings.Contains(msg, "at once"):
			return rpcError(ErrLimitActive, "too many backtests run at once", map[string]any{"limit": backtest.MaxActiveBacktests})
		default:
			return rpcError(ErrLimitExamples, "a dataset has at most 5000 examples", map[string]any{"limit": backtest.MaxExamples})
		}
	case errors.Is(err, engine.ErrInvalidInput) && strings.Contains(err.Error(), "no examples"):
		return rpcError(ErrEmptyDataset, "the dataset has no examples", nil)
	}
	return mapErr(err)
}

func optString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func importView(j backtest.ImportJob) ImportView {
	v := ImportView{ID: j.ID.String(), Kind: ImportKind(j.Source.Kind), Status: BacktestStatus(j.Status), Imported: j.Imported,
		Duplicates: j.Duplicates, Overflow: j.Overflow, Unreadable: j.Unreadable, Error: optString(j.Error), CreatedAt: stamp(j.CreatedAt)}
	v.FinishedAt = optStamp(j.FinishedAt)
	return v
}

func backtestView(b backtest.Backtest, names map[int64]string) BacktestView {
	return BacktestView{ID: b.ID.String(), DatasetID: b.DatasetID.String(), DatasetName: b.DatasetName, Version: b.Version,
		Decisions: Decisions(b.Decisions), Status: BacktestStatus(b.Status), Total: b.Total, Done: b.Done, LiveCalls: b.LiveCalls,
		Error: optString(b.Error), CreatedBy: nameOf(names, b.CreatedBy), ByAgent: b.CreatedOrigin == engine.OriginAgent,
		CreatedAt: stamp(b.CreatedAt), FinishedAt: optStamp(b.FinishedAt)}
}

func (c *Controller) backtestList(ctx context.Context, req AutomationRequest) (BacktestsResponse, error) {
	svc, err := c.backtests()
	if err != nil {
		return BacktestsResponse{}, err
	}
	a, err := actor(ctx, permAuthor)
	if err != nil {
		return BacktestsResponse{}, err
	}
	id, err := parseID(req.AutomationID)
	if err != nil {
		return BacktestsResponse{}, err
	}
	sets, err := svc.ListDatasets(ctx, a, id)
	if err != nil {
		return BacktestsResponse{}, backtestErr(err)
	}
	runs, err := svc.ListBacktests(ctx, a, id)
	if err != nil {
		return BacktestsResponse{}, backtestErr(err)
	}
	sources, err := svc.Sources(ctx, a, id)
	if err != nil {
		return BacktestsResponse{}, backtestErr(err)
	}
	decisions, err := c.decisionNodes(ctx, a, id)
	if err != nil {
		return BacktestsResponse{}, mapErr(err)
	}
	ids := make([]int64, 0, len(sets)+len(runs))
	for _, d := range sets {
		ids = append(ids, d.CreatedBy)
	}
	for _, b := range runs {
		ids = append(ids, b.CreatedBy)
	}
	names := c.names(ctx, a.TenantID, ids...)
	out := BacktestsResponse{Datasets: make([]DatasetView, 0, len(sets)), Backtests: make([]BacktestView, 0, len(runs)),
		Sources: make([]SourceView, 0, len(sources)), Live: a.Origin == engine.OriginHuman, DecisionNodes: decisions}
	for _, d := range sets {
		dv := DatasetView{ID: d.ID.String(), Name: d.Name, Description: d.Description, Examples: d.Examples, Labeled: d.Labeled,
			CreatedBy: nameOf(names, d.CreatedBy), CreatedAt: stamp(d.CreatedAt), Imports: make([]ImportView, 0, len(d.Imports))}
		for _, j := range d.Imports {
			dv.Imports = append(dv.Imports, importView(j))
		}
		out.Datasets = append(out.Datasets, dv)
	}
	for _, b := range runs {
		out.Backtests = append(out.Backtests, backtestView(b, names))
	}
	for _, s := range sources {
		sv := SourceView{Kind: ImportKind(s.Kind), Source: s.Source, Title: s.Title, ConnectorKind: s.ConnectorKind}
		if s.ConnectionID != uuid.Nil {
			sv.ConnectionID = s.ConnectionID.String()
		}
		out.Sources = append(out.Sources, sv)
	}
	return out, nil
}

// decisionNodes are the nodes of the automation's latest version whose type answers with a model's
// decision (engine.DecisionNode).
func (c *Controller) decisionNodes(ctx context.Context, a engine.Actor, id uuid.UUID) ([]string, error) {
	am, err := c.cfg.Engine.Service.Get(ctx, a, id)
	if err != nil {
		return nil, err
	}
	v, err := c.cfg.Engine.Service.GetVersion(ctx, a, id, am.LatestVersion)
	if err != nil {
		return nil, err
	}
	var g struct {
		Nodes []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"nodes"`
	}
	out := []string{}
	if json.Unmarshal(v.Graph, &g) != nil {
		return out, nil //nolint:nilerr // a stored graph that does not decode has no decision nodes to name
	}
	for _, n := range g.Nodes {
		if nt, ok := c.cfg.Catalog.Node(n.Type); ok {
			if _, decides := nt.(engine.DecisionNode); decides {
				out = append(out, n.ID)
			}
		}
	}
	return out, nil
}

func (c *Controller) backtestCount(ctx context.Context, req AutomationRequest) (BacktestCountResponse, error) {
	svc, err := c.backtests()
	if err != nil {
		return BacktestCountResponse{}, err
	}
	a, err := actor(ctx, permAuthor)
	if err != nil {
		return BacktestCountResponse{}, err
	}
	id, err := parseID(req.AutomationID)
	if err != nil {
		return BacktestCountResponse{}, err
	}
	n, err := svc.Count(ctx, a, id)
	if err != nil {
		return BacktestCountResponse{}, backtestErr(err)
	}
	return BacktestCountResponse{Count: n}, nil
}

func (c *Controller) datasetCreate(ctx context.Context, req DatasetCreateRequest) (IDResponse, error) {
	svc, err := c.backtests()
	if err != nil {
		return IDResponse{}, err
	}
	a, err := actor(ctx, permAuthor)
	if err != nil {
		return IDResponse{}, err
	}
	id, err := parseID(req.AutomationID)
	if err != nil {
		return IDResponse{}, err
	}
	ds, err := svc.CreateDataset(ctx, a, backtest.CreateDatasetInput{AutomationID: id, Name: req.Name, Description: req.Description})
	if err != nil {
		return IDResponse{}, backtestErr(err)
	}
	return IDResponse{ID: ds.String()}, nil
}

func parseAny(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(s))
	if err != nil {
		return uuid.Nil, solid.NewRPCError(solid.CodeNotFound, "not found", map[string]string{"code": solid.CodeNotFound})
	}
	return id, nil
}

func (c *Controller) datasetDelete(ctx context.Context, req DatasetRequest) (OKResponse, error) {
	svc, err := c.backtests()
	if err != nil {
		return OKResponse{}, err
	}
	a, err := actor(ctx, permAuthor)
	if err != nil {
		return OKResponse{}, err
	}
	ds, err := parseAny(req.DatasetID)
	if err != nil {
		return OKResponse{}, err
	}
	if err := svc.DeleteDataset(ctx, a, ds); err != nil {
		return OKResponse{}, backtestErr(err)
	}
	return OKResponse{OK: true}, nil
}

func (c *Controller) datasetImport(ctx context.Context, req DatasetImportRequest) (IDResponse, error) {
	svc, err := c.backtests()
	if err != nil {
		return IDResponse{}, err
	}
	a, err := actor(ctx, permAuthor)
	if err != nil {
		return IDResponse{}, err
	}
	ds, err := parseAny(req.DatasetID)
	if err != nil {
		return IDResponse{}, err
	}
	src := backtest.ImportSource{Kind: string(req.Kind), Events: req.Events, Source: req.Source, Filter: req.Filter}
	now := time.Now()
	switch {
	case req.From != "":
		from, err := time.Parse(time.RFC3339, req.From)
		if err != nil {
			return IDResponse{}, rpcError(ErrInvalidInput, "from is an RFC 3339 time", nil)
		}
		src.Window.From = from
		if req.To != "" {
			to, err := time.Parse(time.RFC3339, req.To)
			if err != nil {
				return IDResponse{}, rpcError(ErrInvalidInput, "to is an RFC 3339 time", nil)
			}
			src.Window.To = to
		}
	default:
		days := req.Days
		if days <= 0 {
			days = defaultImportDays
		}
		src.Window = backtest.TimeWindow{From: now.Add(-time.Duration(days) * 24 * time.Hour), To: now}
	}
	if req.ConnectionID != "" {
		if src.ConnectionID, err = parseAny(req.ConnectionID); err != nil {
			return IDResponse{}, err
		}
	}
	job, err := svc.Import(ctx, a, ds, src)
	if err != nil {
		return IDResponse{}, backtestErr(err)
	}
	return IDResponse{ID: job.String()}, nil
}

func exampleView(x backtest.Example) ExampleView {
	v := ExampleView{ID: x.ID.String(), Source: ExampleSource(x.Source), Title: x.Title, EventKind: x.EventKind, Event: x.Event,
		Subject: Subject{Entity: x.Subject.Entity, Key: x.Subject.Key}, SnapshotApproximate: x.SnapshotApproximate,
		HasSnapshot: x.HasSnapshot, OccurredAt: stamp(x.OccurredAt), Hidden: x.Hidden, Labels: make(map[string]LabelView, len(x.Labels))}
	if len(v.Event) == 0 {
		v.Event = json.RawMessage("{}")
	}
	for node, l := range x.Labels {
		v.Labels[node] = LabelView{Outcome: l.Outcome, Source: LabelSource(l.Source), Expected: l.Expected}
	}
	return v
}

func (c *Controller) datasetExamples(ctx context.Context, req ExamplesRequest) (ExamplesResponse, error) {
	svc, err := c.backtests()
	if err != nil {
		return ExamplesResponse{}, err
	}
	a, err := actor(ctx, permAuthor)
	if err != nil {
		return ExamplesResponse{}, err
	}
	ds, err := parseAny(req.DatasetID)
	if err != nil {
		return ExamplesResponse{}, err
	}
	page, err := svc.Examples(ctx, a, ds, backtest.ExampleFilter{Cursor: req.Cursor, Limit: req.Limit, Labeled: req.Labeled})
	if err != nil {
		return ExamplesResponse{}, backtestErr(err)
	}
	out := ExamplesResponse{Items: make([]ExampleView, 0, len(page.Items)), NextCursor: optString(page.NextCursor), Total: page.Total}
	for _, x := range page.Items {
		out.Items = append(out.Items, exampleView(x))
	}
	return out, nil
}

func (c *Controller) datasetSynthetic(ctx context.Context, req SyntheticRequest) (IDResponse, error) {
	svc, err := c.backtests()
	if err != nil {
		return IDResponse{}, err
	}
	a, err := actor(ctx, permAuthor)
	if err != nil {
		return IDResponse{}, err
	}
	ds, err := parseAny(req.DatasetID)
	if err != nil {
		return IDResponse{}, err
	}
	ev := backtest.SyntheticEvent{Kind: req.Kind, Payload: req.Payload, Title: req.Title, Expected: map[string]string{}}
	for node, outcome := range req.Expected {
		if strings.TrimSpace(outcome) != "" {
			ev.Expected[node] = outcome
		}
	}
	if req.Subject != nil {
		ev.Subject = engine.SubjectRef{Entity: req.Subject.Entity, Key: req.Subject.Key}
	}
	id, err := svc.AddSynthetic(ctx, a, ds, ev)
	if err != nil {
		return IDResponse{}, backtestErr(err)
	}
	return IDResponse{ID: id.String()}, nil
}

func (c *Controller) exampleDelete(ctx context.Context, req ExampleRequest) (OKResponse, error) {
	svc, err := c.backtests()
	if err != nil {
		return OKResponse{}, err
	}
	a, err := actor(ctx, permAuthor)
	if err != nil {
		return OKResponse{}, err
	}
	id, err := parseAny(req.ExampleID)
	if err != nil {
		return OKResponse{}, err
	}
	if err := svc.DeleteExample(ctx, a, id); err != nil {
		return OKResponse{}, backtestErr(err)
	}
	return OKResponse{OK: true}, nil
}

func (c *Controller) exampleLabel(ctx context.Context, req LabelRequest) (OKResponse, error) {
	svc, err := c.backtests()
	if err != nil {
		return OKResponse{}, err
	}
	a, err := actor(ctx, permAuthor)
	if err != nil {
		return OKResponse{}, err
	}
	id, err := parseAny(req.ExampleID)
	if err != nil {
		return OKResponse{}, err
	}
	if err := svc.Label(ctx, a, id, req.Node, req.Outcome); err != nil {
		return OKResponse{}, backtestErr(err)
	}
	return OKResponse{OK: true}, nil
}

func runInputOf(req BacktestRunRequest) (backtest.RunInput, error) {
	ds, err := parseAny(req.DatasetID)
	if err != nil {
		return backtest.RunInput{}, err
	}
	return backtest.RunInput{DatasetID: ds, Version: req.Version, Decisions: string(req.Decisions), Revision: req.Revision}, nil
}

func (c *Controller) backtestEstimate(ctx context.Context, req BacktestRunRequest) (EstimateResponse, error) {
	svc, err := c.backtests()
	if err != nil {
		return EstimateResponse{}, err
	}
	a, err := actor(ctx, permAuthor)
	if err != nil {
		return EstimateResponse{}, err
	}
	in, err := runInputOf(req)
	if err != nil {
		return EstimateResponse{}, err
	}
	est, err := svc.Estimate(ctx, a, in)
	if err != nil {
		return EstimateResponse{}, backtestErr(err)
	}
	nodes := est.DecisionNodes
	if nodes == nil {
		nodes = []string{}
	}
	return EstimateResponse{Examples: est.Examples, Revision: est.Revision, DecisionNodes: nodes, MaxCalls: est.MaxCalls, PerItem: est.PerItem, CostUSD: est.CostUSD}, nil
}

func (c *Controller) backtestRun(ctx context.Context, req BacktestRunRequest) (IDResponse, error) {
	svc, err := c.backtests()
	if err != nil {
		return IDResponse{}, err
	}
	a, err := actor(ctx, permAuthor)
	if err != nil {
		return IDResponse{}, err
	}
	in, err := runInputOf(req)
	if err != nil {
		return IDResponse{}, err
	}
	id, err := svc.Run(ctx, a, in)
	if err != nil {
		return IDResponse{}, backtestErr(err)
	}
	return IDResponse{ID: id.String()}, nil
}

func (c *Controller) backtestCancel(ctx context.Context, req BacktestRequest) (OKResponse, error) {
	svc, err := c.backtests()
	if err != nil {
		return OKResponse{}, err
	}
	a, err := actor(ctx, permAuthor)
	if err != nil {
		return OKResponse{}, err
	}
	id, err := parseAny(req.BacktestID)
	if err != nil {
		return OKResponse{}, err
	}
	if err := svc.Cancel(ctx, a, id); err != nil {
		return OKResponse{}, backtestErr(err)
	}
	return OKResponse{OK: true}, nil
}

func (c *Controller) reportView(ctx context.Context, a engine.Actor, r backtest.Report) ReportResponse {
	names := c.names(ctx, a.TenantID, r.Backtest.CreatedBy)
	out := ReportResponse{Backtest: backtestView(r.Backtest, names), Examples: r.Examples, Triggered: r.Triggered, Ends: r.Ends, MismatchTotal: r.MismatchTotal,
		Classifiers: make([]ClassifierView, 0, len(r.Classifiers)), Mismatches: make([]MismatchView, 0, len(r.Mismatches)),
		Effects: make([]EffectSummaryView, 0, len(r.Effects)), Cost: CostView(r.Cost)}
	if out.Ends == nil {
		out.Ends = map[string]int{}
	}
	for _, cl := range r.Classifiers {
		cv := ClassifierView{Node: cl.Node, Decision: cl.Decision, Labeled: cl.Labeled, Correct: cl.Correct, Undecided: cl.Undecided,
			Accuracy: cl.Accuracy, Outcomes: make([]OutcomeMetricsView, 0, len(cl.Outcomes)), Distribution: cl.Distribution}
		if cv.Distribution == nil {
			cv.Distribution = map[string]int{}
		}
		for _, o := range cl.Outcomes {
			cv.Outcomes = append(cv.Outcomes, OutcomeMetricsView(o))
		}
		out.Classifiers = append(out.Classifiers, cv)
	}
	for _, m := range r.Mismatches {
		out.Mismatches = append(out.Mismatches, MismatchView{ExampleID: m.ExampleID.String(), Title: m.Title, OccurredAt: stamp(m.OccurredAt),
			Node: m.Node, Expected: m.Expected, Actual: m.Actual, LabelSource: LabelSource(m.LabelSource)})
	}
	for _, e := range r.Effects {
		samples := e.Samples
		if samples == nil {
			samples = []json.RawMessage{}
		}
		out.Effects = append(out.Effects, EffectSummaryView{Node: e.Node, Examples: e.Examples, Count: e.Count, Samples: samples})
	}
	return out
}

func (c *Controller) backtestReport(ctx context.Context, req BacktestRequest) (ReportResponse, error) {
	svc, err := c.backtests()
	if err != nil {
		return ReportResponse{}, err
	}
	a, err := actor(ctx, permAuthor)
	if err != nil {
		return ReportResponse{}, err
	}
	id, err := parseAny(req.BacktestID)
	if err != nil {
		return ReportResponse{}, err
	}
	r, err := svc.Report(ctx, a, id)
	if err != nil {
		return ReportResponse{}, backtestErr(err)
	}
	return c.reportView(ctx, a, r), nil
}

func (c *Controller) backtestResults(ctx context.Context, req ResultsRequest) (ResultsResponse, error) {
	svc, err := c.backtests()
	if err != nil {
		return ResultsResponse{}, err
	}
	a, err := actor(ctx, permAuthor)
	if err != nil {
		return ResultsResponse{}, err
	}
	id, err := parseAny(req.BacktestID)
	if err != nil {
		return ResultsResponse{}, err
	}
	page, err := svc.Results(ctx, a, id, backtest.ResultFilter{Node: req.Node, Mismatch: req.Mismatch, End: string(req.End), Cursor: req.Cursor, Limit: req.Limit})
	if err != nil {
		return ResultsResponse{}, backtestErr(err)
	}
	out := ResultsResponse{Items: make([]ResultView, 0, len(page.Items)), NextCursor: optString(page.NextCursor), Total: page.Total}
	for _, r := range page.Items {
		rv := ResultView{Example: exampleView(r.Example), End: EndKind(r.End), Outcomes: r.Outcomes, Error: r.Error,
			Decisions: make([]DecisionView, 0, len(r.Decisions)), Effects: make([]EffectView, 0, len(r.Effects)),
			Trace: make([]TraceStep, 0, len(r.Trace)), Mismatches: r.Mismatches}
		if rv.Outcomes == nil {
			rv.Outcomes = map[string]string{}
		}
		if rv.Mismatches == nil {
			rv.Mismatches = []string{}
		}
		for _, d := range r.Decisions {
			rv.Decisions = append(rv.Decisions, DecisionView{Node: d.Node, Outcome: d.Outcome, Status: DecisionStatus(d.Status), CostUSD: d.CostUSD})
		}
		for _, e := range r.Effects {
			rv.Effects = append(rv.Effects, EffectView(e))
		}
		for _, s := range r.Trace {
			rv.Trace = append(rv.Trace, TraceStep{Node: s.Node, Token: s.Token, Kind: s.Kind, Outcome: s.Outcome, Path: s.Path, Message: s.Message,
				Status: s.Status, Effects: s.Effects, Output: s.Output})
		}
		out.Items = append(out.Items, rv)
	}
	return out, nil
}

func (c *Controller) backtestCompare(ctx context.Context, req CompareRequest) (CompareResponse, error) {
	svc, err := c.backtests()
	if err != nil {
		return CompareResponse{}, err
	}
	a, err := actor(ctx, permAuthor)
	if err != nil {
		return CompareResponse{}, err
	}
	id1, err := parseAny(req.A)
	if err != nil {
		return CompareResponse{}, err
	}
	id2, err := parseAny(req.B)
	if err != nil {
		return CompareResponse{}, err
	}
	cmp, err := svc.Compare(ctx, a, id1, id2)
	if err != nil {
		return CompareResponse{}, backtestErr(err)
	}
	out := CompareResponse{A: c.reportView(ctx, a, cmp.A), B: c.reportView(ctx, a, cmp.B), CommonExamples: cmp.CommonExamples, Changed: cmp.Changed,
		Diffs: make([]ExampleDiffView, 0, len(cmp.Diffs))}
	for _, d := range cmp.Diffs {
		dv := ExampleDiffView{ExampleID: d.ExampleID.String(), Title: d.Title, OccurredAt: stamp(d.OccurredAt), EndA: EndKind(d.EndA), EndB: EndKind(d.EndB),
			Changes: make([]OutcomeChangeView, 0, len(d.Changes)), EffectsA: d.EffectsA, EffectsB: d.EffectsB, ParamsChanged: d.ParamsChanged}
		if dv.EffectsA == nil {
			dv.EffectsA = []string{}
		}
		if dv.EffectsB == nil {
			dv.EffectsB = []string{}
		}
		for _, ch := range d.Changes {
			dv.Changes = append(dv.Changes, OutcomeChangeView(ch))
		}
		out.Diffs = append(out.Diffs, dv)
	}
	return out, nil
}
