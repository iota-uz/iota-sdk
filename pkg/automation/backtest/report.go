package backtest

import (
	"bytes"
	"encoding/json"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// scored is what metrics read of one result: its example and what the graph did.
type scored struct {
	example    ExampleID
	title      string
	occurredAt time.Time
	end        string
	triggered  bool
	// snapshotWithheld: the example ran without its subject's snapshot (the runner may not read it).
	snapshotWithheld bool
	outcomes         map[string]string
	decisions        []Decision
	effects          []Effect
	labels           map[string]Label
}

// decisionOf is the status of the first decision of node in a result ("" when it made none).
func (s scored) decisionOf(node string) string {
	for _, d := range s.decisions {
		if d.Node == node {
			return d.Status
		}
	}
	return ""
}

// classifierNodes are the nodes a report measures: the decision nodes, then every labeled node, sorted.
func classifierNodes(decisionNodes map[string]bool, rows []scored) []string {
	set := map[string]bool{}
	for n := range decisionNodes {
		set[n] = true
	}
	for _, r := range rows {
		for n := range r.labels {
			set[n] = true
		}
	}
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func ratio(num, den int) *float64 {
	if den == 0 {
		return nil
	}
	v := float64(num) / float64(den)
	return &v
}

// metricsOf are the metrics of node over rows; mismatches are appended to mm.
func metricsOf(node string, decision bool, rows []scored, mm *[]Mismatch) ClassifierMetrics {
	m := ClassifierMetrics{Node: node, Decision: decision, Distribution: map[string]int{}}
	// One pass: per outcome, how often it was predicted, expected, and both.
	predicted, expected, both := map[string]int{}, map[string]int{}, map[string]int{}
	outcomes := map[string]bool{}
	for _, r := range rows {
		pred := r.outcomes[node]
		undecided := decision && r.decisionOf(node) == DecisionMissing
		if pred != "" && !undecided {
			m.Distribution[pred]++
		}
		l, ok := r.labels[node]
		if !ok {
			continue
		}
		if undecided {
			m.Undecided++
			continue
		}
		m.Labeled++
		if pred == l.Outcome {
			m.Correct++
		} else if mm != nil {
			*mm = append(*mm, Mismatch{ExampleID: r.example, Title: r.title, OccurredAt: r.occurredAt, Node: node,
				Expected: l.Outcome, Actual: pred, LabelSource: l.Source})
		}
		expected[l.Outcome]++
		if pred != "" {
			predicted[pred]++
		}
		if pred == l.Outcome {
			both[pred]++
		}
		outcomes[l.Outcome] = true
		if pred != "" {
			outcomes[pred] = true
		}
	}
	m.Accuracy = ratio(m.Correct, m.Labeled)
	names := make([]string, 0, len(outcomes))
	for o := range outcomes {
		names = append(names, o)
	}
	sort.Strings(names)
	for _, o := range names {
		om := OutcomeMetrics{Outcome: o, TruePositives: both[o], Support: expected[o]}
		om.FalsePositives = predicted[o] - both[o]
		om.FalseNegatives = expected[o] - both[o]
		om.Precision = ratio(om.TruePositives, om.TruePositives+om.FalsePositives)
		om.Recall = ratio(om.TruePositives, om.Support)
		m.Outcomes = append(m.Outcomes, om)
	}
	return m
}

// maxSamples bounds the samples of an effect summary.
const maxSamples = 3

// buildReport computes a backtest's report over its results.
func buildReport(b Backtest, decisionNodes map[string]bool, rows []scored) Report {
	rep := Report{Backtest: b, Examples: len(rows), Ends: map[string]int{}}
	var mm []Mismatch
	for _, node := range classifierNodes(decisionNodes, rows) {
		rep.Classifiers = append(rep.Classifiers, metricsOf(node, decisionNodes[node], rows, &mm))
	}
	sort.SliceStable(mm, func(i, j int) bool {
		if !mm[i].OccurredAt.Equal(mm[j].OccurredAt) {
			return mm[i].OccurredAt.Before(mm[j].OccurredAt)
		}
		return mm[i].Node < mm[j].Node
	})
	rep.MismatchTotal = len(mm)
	if len(mm) > MaxMismatches {
		mm = mm[:MaxMismatches]
	}
	rep.Mismatches = mm
	effects := map[string]*EffectSummary{}
	var order []string
	for _, r := range rows {
		rep.Ends[r.end]++
		if r.triggered {
			rep.Triggered++
		}
		if r.snapshotWithheld {
			rep.SnapshotsWithheld++
		}
		seen := map[string]bool{}
		for _, e := range r.effects {
			s, ok := effects[e.Node]
			if !ok {
				s = &EffectSummary{Node: e.Node}
				effects[e.Node] = s
				order = append(order, e.Node)
			}
			s.Count++
			if !seen[e.Node] {
				s.Examples++
				seen[e.Node] = true
			}
			if len(s.Samples) < maxSamples {
				s.Samples = append(s.Samples, e.Params)
			}
		}
		for _, d := range r.decisions {
			c := &rep.Cost
			switch d.Status {
			case DecisionLive:
				c.LiveCalls++
			case DecisionCached:
				c.CachedCalls++
			case DecisionMissing:
				c.Missing++
				continue
			}
			c.InputTokens += d.InputTokens
			c.OutputTokens += d.OutputTokens
			switch {
			case d.CostUSD == nil:
				c.UnknownCost++
			case d.Status == DecisionLive:
				c.LiveUSD += *d.CostUSD
			default:
				c.CachedUSD += *d.CostUSD
			}
		}
	}
	sort.Strings(order)
	for _, n := range order {
		rep.Effects = append(rep.Effects, *effects[n])
	}
	return rep
}

// analysis is what a result keeps of a dry run's trace.
type analysis struct {
	end string
	// triggered: a trigger fired (the run may still have ended with an error after it).
	triggered bool
	outcomes  map[string]string
	decisions []Decision
	effects   []Effect
	errText   string
	trace     []engine.TraceStep
	// snapshotWithheld: the example ran without its subject's snapshot, which the runner may not read.
	snapshotWithheld bool
}

// analyze reads a dry run's trace: how it ended, the first outcome of each node, the decisions of the
// decision nodes and the captured effects.
func analyze(trace []engine.TraceStep, decisionNodes map[string]bool) analysis {
	a := analysis{end: EndEnded, outcomes: map[string]string{}}
	// waits are the tokens still waiting: a wait that is woken, times out or whose branch goes on is
	// not; one left at the end of the trace makes the run waiting, whatever other branches ended.
	waits := map[string]bool{}
	triggered := false
	for _, s := range trace {
		if s.Kind != engine.TraceWait {
			delete(waits, s.Token)
		}
		switch s.Kind {
		case engine.TraceNotTriggered:
			if !triggered {
				a.end = EndNotTriggered
			}
			continue
		case engine.TraceTriggered:
			triggered = true
			a.triggered = true
			a.end = EndEnded
		case engine.TraceError:
			a.end = EndError
			if a.errText == "" {
				a.errText = s.Message
			}
		case engine.TraceWait:
			waits[s.Token] = true
		}
		if s.Node != "" && s.Outcome != "" && s.Kind != engine.TraceNotTriggered {
			if _, seen := a.outcomes[s.Node]; !seen {
				a.outcomes[s.Node] = s.Outcome
			}
		}
		if decisionNodes[s.Node] && s.Kind == engine.TraceStepKind {
			d := Decision{Node: s.Node, Outcome: s.Outcome}
			switch s.Status {
			case graph.StatusExecuted:
				d.Status = DecisionLive
			case engine.StatusCached:
				d.Status = DecisionCached
			default:
				d.Status = DecisionMissing
			}
			if d.Status != DecisionMissing {
				d.CostUSD, d.InputTokens, d.OutputTokens = usageOf(s.Output)
			}
			a.decisions = append(a.decisions, d)
		}
		if s.Status == graph.StatusCaptured && !decisionNodes[s.Node] {
			for _, e := range s.Effects {
				a.effects = append(a.effects, Effect{Node: s.Node, Params: e})
			}
		}
	}
	if len(waits) > 0 && a.end == EndEnded {
		a.end = EndWaiting
	}
	if len(trace) > MaxTraceSteps {
		trace = trace[:MaxTraceSteps]
	}
	a.trace = trace
	return a
}

// usageOf reads the usage a decision node reports in its output: {"usage": {"cost_usd", "input_tokens",
// "output_tokens"}}.
func usageOf(out json.RawMessage) (*float64, int64, int64) {
	var o struct {
		Usage struct {
			CostUSD      *float64 `json:"cost_usd"`
			InputTokens  int64    `json:"input_tokens"`
			OutputTokens int64    `json:"output_tokens"`
		} `json:"usage"`
	}
	if len(out) == 0 || json.Unmarshal(out, &o) != nil {
		return nil, 0, 0
	}
	return o.Usage.CostUSD, o.Usage.InputTokens, o.Usage.OutputTokens
}

// mismatchesOf are the labeled nodes of a result whose outcome is not the label (undecided decisions
// are not mismatches).
func mismatchesOf(r scored, decisionNodes map[string]bool) []string {
	var out []string
	for node, l := range r.labels {
		if decisionNodes[node] && r.decisionOf(node) == DecisionMissing {
			continue
		}
		if r.outcomes[node] != l.Outcome {
			out = append(out, node)
		}
	}
	sort.Strings(out)
	return out
}

// effectNodes are the nodes of a result's effects, in order.
func effectNodes(effects []Effect) []string {
	out := make([]string, 0, len(effects))
	for _, e := range effects {
		out = append(out, e.Node)
	}
	return out
}

// sameEffects is whether two runs would make the same effects, in order, with the same params (JSON
// compared by value: the order of an object's keys does not count).
func sameEffects(a, b []Effect) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Node != b[i].Node || !sameJSON(a[i].Params, b[i].Params) {
			return false
		}
	}
	return true
}

// sameJSON compares two JSON values by value: an object's key order does not count, numbers compare
// exactly (9007199254740992 and 9007199254740993 differ; 1 and 1.0 do not).
func sameJSON(a, b json.RawMessage) bool {
	x, okA := canonical(a)
	y, okB := canonical(b)
	if !okA || !okB {
		return string(a) == string(b)
	}
	return x == y
}

// canonical is a JSON value as a canonical string: keys sorted, numbers as exact rationals.
func canonical(raw json.RawMessage) (string, bool) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if dec.Decode(&v) != nil {
		return "", false
	}
	var b strings.Builder
	if !writeCanonical(&b, v) {
		return "", false
	}
	return b.String(), true
}

func writeCanonical(b *strings.Builder, v any) bool {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(strconv.Quote(k))
			b.WriteByte(':')
			if !writeCanonical(b, x[k]) {
				return false
			}
		}
		b.WriteByte('}')
	case []any:
		b.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			if !writeCanonical(b, e) {
				return false
			}
		}
		b.WriteByte(']')
	case json.Number:
		r, ok := new(big.Rat).SetString(string(x))
		if !ok {
			return false
		}
		b.WriteString("n" + r.RatString())
	case string:
		b.WriteString(strconv.Quote(x))
	case bool:
		b.WriteString(strconv.FormatBool(x))
	case nil:
		b.WriteString("null")
	default:
		return false
	}
	return true
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// diffOf is how two results of one example differ; ok is false when they do not.
func diffOf(a, b scored) (ExampleDiff, bool) {
	d := ExampleDiff{ExampleID: a.example, Title: a.title, OccurredAt: a.occurredAt, EndA: a.end, EndB: b.end}
	nodes := map[string]bool{}
	for n := range a.outcomes {
		nodes[n] = true
	}
	for n := range b.outcomes {
		nodes[n] = true
	}
	names := make([]string, 0, len(nodes))
	for n := range nodes {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if a.outcomes[n] != b.outcomes[n] {
			c := OutcomeChange{Node: n, A: a.outcomes[n], B: b.outcomes[n]}
			if l, ok := a.labels[n]; ok {
				c.Expected = l.Outcome
			}
			d.Changes = append(d.Changes, c)
		}
	}
	ea, eb := effectNodes(a.effects), effectNodes(b.effects)
	effectsDiffer := !sameEffects(a.effects, b.effects)
	if effectsDiffer {
		d.EffectsA, d.EffectsB = ea, eb
		// The same effects in the same order whose rendered params differ (a changed message text).
		d.ParamsChanged = sameStrings(ea, eb)
	}
	return d, len(d.Changes) > 0 || effectsDiffer || a.end != b.end
}

// traceRecord is a trace step as a result stores it.
type traceRecord struct {
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

func traceRecords(steps []engine.TraceStep) []traceRecord {
	out := make([]traceRecord, 0, len(steps))
	for _, s := range steps {
		out = append(out, traceRecord(s))
	}
	return out
}

func traceSteps(recs []traceRecord) []engine.TraceStep {
	out := make([]engine.TraceStep, 0, len(recs))
	for _, r := range recs {
		out = append(out, engine.TraceStep(r))
	}
	return out
}
