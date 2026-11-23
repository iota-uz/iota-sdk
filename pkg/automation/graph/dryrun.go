package graph

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// DryRunInput is one input of a dry run: an event, or a timeout.
type DryRunInput struct {
	Event   Event
	Timeout bool
}

// DryRunOptions adjust a dry run.
type DryRunOptions struct {
	// CallError is the product's check of a call's rendered request, the one its engine makes before
	// calling: a non-empty error sends the node down its failed outcome without reading the given answer.
	CallError func(Effect) string
}

// TraceStep is one step of a dry run's trace. Token is the token that took it ("" the main path).
type TraceStep struct {
	Node         string         `json:"node"`
	Outcome      string         `json:"outcome"`
	ContextPatch map[string]any `json:"context_patch"`
	End          map[string]any `json:"end,omitempty"`
	Token        string         `json:"token,omitempty"`
}

// Trace outcomes besides node outcomes.
const (
	TraceWaiting        = "waiting"
	TraceEnded          = "ended"
	TraceEnd            = "end"
	TraceNotTriggered   = "not_triggered"
	TraceMissingContext = "missing_context"
	TraceFailed         = "failed"
	// TraceArrived: a token arrived at a join or a collect, context_patch {"value": what it brings}.
	TraceArrived = "arrived"
	// TraceCancelled: a token was cancelled (a join any / n_of, a join timeout) on the node it was at.
	TraceCancelled = "cancelled"
)

const (
	maxDryRunSteps = 500    // steps of one token between two waits
	maxDryRunTrace = 10_000 // steps of the whole trace
)

// DryRun runs the graph over the inputs with the given context and applies nothing. The first event a
// trigger takes starts the run; later ones feed its waits. An effect is traced as its rendered params; a
// call's answer is the context's nodes.<id> (output, and outcome for a call whose answer does not name
// it). A field a step needs and the context lacks ends the trace with missing_context and the field's
// path; a run still waiting after the last input ends with waiting. An input no trigger takes leaves the
// trace with not_triggered and, per trigger of the event's kind, the condition that failed.
//
// Tokens: a parallel's branches run in their order, each depth first until it waits, arrives at a join
// or a collect, or ends. A foreach's items run in their order one after another — the next starts once
// the one before ended — whatever its concurrency, so the trace and its MaxTokens count do not depend on
// it. A step of a token other than the main path names it. An event is offered, in the order the tokens
// were created, to the tokens waiting when it arrives and still waiting at their turn: one a join
// cancelled meanwhile is skipped, and a wait started while it is handled does not see it. A timeout input fires the earliest pending timer — a wait's, counted from when it started
// waiting, or a join's, counted from its parallel's fork — the first one created on a tie.
func (i *Interpreter) DryRun(subject Subject, ctx Context, inputs []DryRunInput, o DryRunOptions) []TraceStep {
	run := Context{Event: map[string]any{}, Nodes: map[string]NodeState{}, Vars: ctx.Vars, Now: ctx.Now, Tenant: ctx.Tenant, Foreach: map[string]ForeachItem{}}
	for id, st := range ctx.Nodes {
		// A call's given answer is its answer, not an output: it is visible once the call has run.
		if node, ok := i.view.Node(id); ok && i.view.catalog.Nodes[node.Type].Behavior == BehaviorCall {
			continue
		}
		run.Nodes[id] = st
	}
	if reason := i.subjectMismatch(subject); reason != "" {
		node := "trigger"
		if len(i.view.graph.Triggers) > 0 {
			node = i.view.graph.Triggers[0].ID
		}
		return []TraceStep{{Node: node, Outcome: TraceFailed, ContextPatch: map[string]any{"code": FailInvalidSubject, "message": reason}}}
	}
	d := &dryRun{i: i, o: o, given: ctx.Nodes, trace: make([]TraceStep, 0, len(inputs)+4)}
	started := false

	var missed []map[string]any
	for _, input := range inputs {
		if d.ended {
			break
		}
		if !started {
			if input.Timeout {
				continue
			}
			for _, t := range i.view.graph.Triggers {
				// A run about an entity starts only on a trigger about it (a mixed graph has several).
				if subject.Entity != "" && !i.view.mixedRun(subject.Entity) && i.view.catalog.Events[t.Event].Subject != subject.Entity {
					continue
				}
				evCtx := run
				evCtx.Event = input.Event
				m, st := i.TriggerMismatch(t.ID, input.Event, evCtx)
				if st != nil {
					d.stop(nil, t.ID, *st)
					break
				}
				if m != "" {
					if m != MismatchEvent {
						missed = append(missed, map[string]any{"trigger": t.ID, "failed": m})
					}
					continue
				}
				main := &dryToken{ctx: copyContext(run), from: t.ID}
				main.ctx.Event = map[string]any(input.Event)
				d.tokens = append(d.tokens, main)
				d.emit(nil, TraceStep{Node: t.ID, Outcome: TriggerOutcome})
				started = true
				d.advance(main, t.ID, TriggerOutcome)
				break
			}
			continue
		}
		if input.Timeout {
			d.timeout()
			continue
		}
		d.event(input.Event)
	}
	if len(d.trace) == 0 {
		node := "trigger"
		if len(missed) > 0 {
			node, _ = missed[0]["trigger"].(string)
		} else if len(i.view.graph.Triggers) > 0 {
			node = i.view.graph.Triggers[0].ID
		}
		p := map[string]any{}
		if len(missed) > 0 {
			p["triggers"] = missed
		}
		d.trace = append(d.trace, TraceStep{Node: node, Outcome: TraceNotTriggered, ContextPatch: p})
	}
	return d.trace
}

// dryToken is one token of a dry run.
type dryToken struct {
	name    string
	ctx     Context // its own: a fork or an item copies it, a join merges the arrived branches'
	at      string  // the node it is on
	from    string  // the node whose outcome led it there
	waiting bool
	parked  bool // it forked: it waits for its join or its items
	done    bool
	// writes are the node outputs the token set since it was created (joins of its own branches
	// included), in the order first set: what a join takes from an arrived branch.
	writes map[string]NodeState
	order  []string
	group  *dryGroup // the parallel or foreach it is a branch or an item of; nil: the main path
	key    string    // its branch
	index  int       // its item
}

// set writes a node's output in the token's context and records it among its writes.
func (t *dryToken) set(id string, ns NodeState) {
	t.ctx.Nodes[id] = ns
	if t.writes == nil {
		t.writes = map[string]NodeState{}
	}
	if _, ok := t.writes[id]; !ok {
		t.order = append(t.order, id)
	}
	t.writes[id] = ns
}

// dryGroup is the tokens of one fork or one foreach.
type dryGroup struct {
	node    string
	kind    Behavior
	parent  *dryToken
	tokens  []*dryToken
	join    string // a parallel's join; "": the branches end on their own
	state   JoinState
	results []ItemResult
	left    int
	// A foreach's items run one after another: an item's token is created when the one before ended.
	items   []any
	target  string
	next    int
	current *dryToken
	running bool
	closed  bool
}

// dryTimer is a pending timeout on the dry run's clock: a waiting token's, or a join's.
type dryTimer struct {
	at    int64
	seq   int
	token *dryToken
	group *dryGroup
}

func (t *dryTimer) alive() bool {
	if t.token != nil {
		return !t.token.done && t.token.waiting
	}
	return !t.group.closed && !t.group.parent.done
}

type dryRun struct {
	i      *Interpreter
	o      DryRunOptions
	given  map[string]NodeState
	trace  []TraceStep
	ended  bool
	tokens []*dryToken // in the order they were created
	timers []*dryTimer
	clock  int64 // ms since the run started, moved by timeouts
	seq    int
}

func copyContext(c Context) Context {
	out := c
	out.Nodes = make(map[string]NodeState, len(c.Nodes))
	for k, v := range c.Nodes {
		out.Nodes[k] = v
	}
	out.Foreach = make(map[string]ForeachItem, len(c.Foreach))
	for k, v := range c.Foreach {
		out.Foreach[k] = v
	}
	return out
}

func dryPatch(effects []Effect, output map[string]any) map[string]any {
	p := map[string]any{}
	if len(output) > 0 {
		p["output"] = output
	}
	if len(effects) > 0 {
		p["effects"] = effects
	}
	return p
}

func (d *dryRun) emit(t *dryToken, s TraceStep) {
	if d.ended {
		return // the trace stopped: its stopping step stays the last
	}
	if t != nil {
		s.Token = t.name
	}
	if s.ContextPatch == nil {
		s.ContextPatch = map[string]any{}
	}
	d.trace = append(d.trace, s)
	if len(d.trace) >= maxDryRunTrace && !d.ended {
		d.trace = append(d.trace, TraceStep{Node: s.Node, Outcome: TraceFailed, Token: s.Token, ContextPatch: map[string]any{
			"code": FailLimit, "message": fmt.Sprintf("a dry run stops after %d steps", maxDryRunTrace)}})
		d.ended = true
	}
}

// stop ends the dry run on a step that stops it: missing context, or a failure of the run.
func (d *dryRun) stop(t *dryToken, node string, s Step) {
	if s.Kind == StepMissingContext {
		d.emit(t, TraceStep{Node: node, Outcome: TraceMissingContext, ContextPatch: map[string]any{"path": s.Path}})
	} else {
		d.emit(t, TraceStep{Node: node, Outcome: TraceFailed, ContextPatch: map[string]any{"code": s.Code, "message": s.Message}})
	}
	d.ended = true
}

// fail is a failed step without a failed outcome: it fails the run on the main path, and ends only its
// token elsewhere.
func (d *dryRun) fail(t *dryToken, node string, s Step) {
	if s.Kind == StepMissingContext || t.group == nil {
		d.stop(t, node, s)
		return
	}
	d.emit(t, TraceStep{Node: node, Outcome: TraceFailed, ContextPatch: map[string]any{"code": s.Code, "message": s.Message}})
	d.finish(t, false, nil, s.Message)
}

// active counts the tokens of the trace's schedule that run: not done and not waiting for their join or
// items. A foreach's later items are not tokens yet.
func (d *dryRun) active() int {
	n := 0
	for _, t := range d.tokens {
		if !t.done && !t.parked {
			n++
		}
	}
	return n
}

func (d *dryRun) addTimer(t *dryToken, g *dryGroup, after time.Duration) {
	d.seq++
	d.timers = append(d.timers, &dryTimer{at: d.clock + after.Milliseconds(), seq: d.seq, token: t, group: g})
}

func (d *dryRun) dropTimer(drop *dryTimer) {
	kept := d.timers[:0]
	for _, t := range d.timers {
		if t != drop && t.alive() {
			kept = append(kept, t)
		}
	}
	d.timers = kept
}

// takeFailed sends a failure down the node's failed outcome, if it has one: output {"error"}, and
// "code" when the failure names one the output must keep (a foreach's VALUE_TOO_LARGE).
func (d *dryRun) takeFailed(t *dryToken, node, message string, code ...string) (string, bool) {
	failedOutcome := d.i.FailedOutcome(node)
	if failedOutcome == "" {
		return "", false
	}
	out := map[string]any{"error": message}
	if len(code) > 0 && code[0] != "" {
		out["code"] = code[0]
	}
	// The whole output keeps MaxValueBytes: the message is shortened; when even the fixed fields do not
	// fit, the run fails.
	out["error"] = d.i.fitError(message, out)
	if big := d.i.tooLarge(out, "the failed output of "+node); big != nil {
		d.stop(t, node, *big)
		return "", false
	}
	t.set(node, NodeState{Output: out})
	d.emit(t, TraceStep{Node: node, Outcome: failedOutcome, ContextPatch: map[string]any{"output": out}})
	return failedOutcome, true
}

func (d *dryRun) advance(t *dryToken, from, outcome string) {
	t.from = from
	if current, ok := d.i.Next(from, outcome); ok {
		d.walk(t, current)
		return
	}
	d.noEdge(t)
}

// noEdge ends a branch's or an item's token on an outcome without an edge (a failed outcome inside a
// construct may have none): the branch does not arrive, the item is not ok.
func (d *dryRun) noEdge(t *dryToken) {
	if t.group == nil {
		return // a valid graph connects every outcome on the main path
	}
	reason := "an outcome of " + t.from + " has no edge"
	if msg, ok := t.ctx.Nodes[t.from].Output["error"].(string); ok && msg != "" {
		reason = msg
	}
	d.finish(t, false, nil, reason)
}

// walk runs a token from a node until it waits, forks, arrives or ends.
func (d *dryRun) walk(t *dryToken, current string) {
	i := d.i
	ok := true
	for steps := 0; ok && !d.ended && steps < maxDryRunSteps; steps++ {
		t.at = current
		if current == Ignore {
			d.emit(t, TraceStep{Node: Ignore, Outcome: TraceEnded})
			d.finish(t, false, nil, "ended at "+Ignore)
			return
		}
		node, exists := i.view.Node(current)
		if !exists {
			d.stop(t, current, failed(FailInvalidGraph, "no node %s", current))
			return
		}
		step := i.start(node.ID, &StepContext{Ctx: t.ctx, Token: t.name, From: t.from}, false)
		// MaxTokens by the trace's schedule, whatever the concurrency: a fork adds its branches, a foreach
		// one item at a time. The run fails, whatever outcomes the node has.
		starting := 0
		switch step.Kind { //nolint:exhaustive // only forks and foreachs make tokens
		case StepFork:
			starting = len(step.Fork)
		case StepEach:
			starting = 1
		}
		if starting > 0 {
			if over := i.tokensOver(d.active(), starting); over != nil {
				d.stop(t, node.ID, *over)
				return
			}
		}
		if step.Kind == StepFailed {
			if fo, took := d.takeFailed(t, node.ID, step.Message); took {
				t.from = node.ID
				current, ok = i.Next(node.ID, fo)
				continue
			}
		}
		if step.stops() {
			d.fail(t, node.ID, step)
			return
		}
		switch step.Kind {
		case StepEnd:
			d.emit(t, TraceStep{Node: node.ID, Outcome: TraceEnd, End: step.End})
			reason, _ := step.End["reason"].(string)
			if reason == "" {
				reason = "ended at " + node.ID
			}
			d.finish(t, false, nil, reason)
			return
		case StepCall:
			refused := ""
			if d.o.CallError != nil {
				for _, e := range step.Effects {
					if refused = d.o.CallError(e); refused != "" {
						break
					}
				}
			}
			if refused != "" {
				fo, took := d.takeFailed(t, node.ID, refused)
				if !took {
					d.fail(t, node.ID, failed(FailInvalidRequest, "%s", refused))
					return
				}
				t.from = node.ID
				current, ok = i.Next(node.ID, fo)
				continue
			}
			outcome, output, st := i.callGiven(node.ID, d.given)
			if st != nil {
				d.fail(t, node.ID, *st)
				return
			}
			d.emit(t, TraceStep{Node: node.ID, Outcome: outcome, ContextPatch: dryPatch(step.Effects, output)})
			t.set(node.ID, NodeState{Output: output})
			t.from = node.ID
			current, ok = i.Next(node.ID, outcome)
			continue
		case StepWait:
			out := map[string]any{"waits_for": step.Wait.Event}
			if step.Wait.Event == "" {
				out = map[string]any{"timeout_ms": step.Wait.Timeout.Milliseconds()}
			}
			d.emit(t, TraceStep{Node: node.ID, Outcome: TraceWaiting, ContextPatch: dryPatch(step.Effects, out)})
			t.waiting = true
			if step.Wait.Timeout > 0 {
				d.addTimer(t, nil, step.Wait.Timeout)
			}
			return
		case StepFork:
			d.fork(t, node.ID, step)
			return
		case StepEach:
			d.each(t, node.ID, step)
			return
		case StepArrive:
			d.arrive(t, node.ID, step)
			return
		case StepOutcome, StepMissingContext, StepFailed:
		}
		d.emit(t, TraceStep{Node: node.ID, Outcome: step.Outcome, ContextPatch: dryPatch(step.Effects, step.Output)})
		if step.Output != nil {
			t.set(node.ID, NodeState{Output: step.Output})
		}
		t.from = node.ID
		current, ok = i.Next(node.ID, step.Outcome)
	}
	if !ok && !d.ended {
		d.noEdge(t)
		return
	}
	if ok && !d.ended {
		// A loop that never waits in a dry run: calls answer at once from the context.
		d.stop(t, current, failed(FailLimit, "a token took %d steps without waiting", maxDryRunSteps))
	}
}

func (d *dryRun) fork(t *dryToken, node string, step Step) {
	g := &dryGroup{node: node, kind: BehaviorParallel, parent: t, state: JoinState{Arrived: map[string]Arrival{}, Gone: map[string]bool{}}}
	if joins := newFlowGraph(d.i.view).closers(node, BehaviorJoin); len(joins) > 0 {
		g.join = joins[0]
	}
	t.parked = true
	if step.Wait != nil && step.Wait.Timeout > 0 {
		d.addTimer(nil, g, step.Wait.Timeout)
	}
	for _, f := range step.Fork {
		branch, _ := branchOf(f.Token, node)
		c := &dryToken{name: f.Token, ctx: copyContext(t.ctx), at: f.Node, from: node, group: g, key: branch}
		g.tokens = append(g.tokens, c)
		d.tokens = append(d.tokens, c)
	}
	d.start(g)
}

func (d *dryRun) each(t *dryToken, node string, step Step) {
	target, ok := d.i.Next(node, OutcomeItem)
	if !ok {
		d.stop(t, node, failed(FailInvalidGraph, "%s has no edge from item", node))
		return
	}
	g := &dryGroup{node: node, kind: BehaviorForeach, parent: t, left: len(step.Each.Items), items: step.Each.Items, target: target}
	t.parked = true
	d.runItems(g)
}

// runItems starts a foreach's next items, one after another: the next once the current one ended. The
// trace is the same whatever the concurrency.
func (d *dryRun) runItems(g *dryGroup) {
	if g.running {
		return // the loop below goes on once the current item's walk returns
	}
	g.running = true
	defer func() { g.running = false }()
	for !d.ended && !g.closed && !g.parent.done && g.next < len(g.items) {
		if g.current != nil && !g.current.done {
			return
		}
		k := g.next
		g.next++
		c := &dryToken{name: ItemToken(g.parent.name, g.node, k), ctx: copyContext(g.parent.ctx), at: g.target, from: g.node, group: g, index: k}
		c.ctx.Foreach[g.node] = ForeachItem{Item: g.items[k], Index: k}
		g.tokens = append(g.tokens, c)
		d.tokens = append(d.tokens, c)
		g.current = c
		d.walk(c, c.at)
	}
}

// start walks a fork's new tokens in their order; one a join already cancelled is not walked.
func (d *dryRun) start(g *dryGroup) {
	for _, c := range g.tokens {
		if d.ended {
			return
		}
		if !c.done {
			d.walk(c, c.at)
		}
	}
}

func (d *dryRun) arrive(t *dryToken, node string, step Step) {
	a := step.Arrive
	if t.group == nil || t.group.node != a.Of {
		d.stop(t, node, failed(FailInvalidGraph, "token %q does not belong to %s", t.name, a.Of))
		return
	}
	d.emit(t, TraceStep{Node: node, Outcome: TraceArrived, ContextPatch: map[string]any{"value": a.Value}})
	d.finish(t, true, a.Value, "")
}

// finish ends a token: it arrived with a value, or it ended without arriving (reason).
func (d *dryRun) finish(t *dryToken, arrived bool, value any, reason string) {
	t.done, t.waiting = true, false
	g := t.group
	if g == nil {
		d.ended = true // the main path ended: so did the run
		return
	}
	if g.closed || g.parent.done {
		return
	}
	switch g.kind { //nolint:exhaustive // a group is a parallel or a foreach
	case BehaviorParallel:
		if arrived {
			g.state.Arrived[t.key] = Arrival{Of: g.node, Token: t.name, Value: value}
		} else {
			g.state.Gone[t.key] = true
		}
		d.decideJoin(g)
	case BehaviorForeach:
		r := ItemResult{Index: t.index, OK: arrived, Value: value}
		if !arrived {
			r.Error = d.i.fitError(reason)
		}
		g.results = append(g.results, r)
		g.left--
		if g.left == 0 {
			d.foreachDone(g)
		} else {
			d.runItems(g)
		}
	}
}

func (d *dryRun) decideJoin(g *dryGroup) {
	if g.closed || g.parent.done || d.ended {
		return
	}
	if g.join == "" {
		for _, c := range g.tokens {
			if !c.done {
				return
			}
		}
		g.closed = true
		d.finish(g.parent, false, nil, "the branches of "+g.node+" ended")
		return
	}
	st := d.i.Join(g.join, g.parent.name, g.state)
	switch st.Kind { //nolint:exhaustive // only waiting and failing change the join
	case StepWait:
		return
	case StepFailed:
		// The forking token fails: the run on the main path, only that token inside a branch or an item.
		g.closed = true
		d.cancel(st.Cancel)
		d.fail(g.parent, g.join, st)
		return
	}
	g.closed = true
	d.cancel(st.Cancel)
	p := g.parent
	for _, c := range g.tokens {
		if _, ok := g.state.Arrived[c.key]; ok {
			// Only what the branch wrote since the fork: its copy of the rest may be older than the
			// parent's (a loop that forks again).
			for _, id := range c.order {
				p.set(id, c.writes[id])
			}
		}
	}
	p.set(g.join, NodeState{Output: st.Output})
	d.emit(p, TraceStep{Node: g.join, Outcome: st.Outcome, ContextPatch: dryPatch(nil, st.Output)})
	p.parked = false
	d.dropTimer(nil)
	d.advance(p, g.join, st.Outcome)
}

func (d *dryRun) foreachDone(g *dryGroup) {
	g.closed = true
	st := d.i.ForeachDone(g.node, g.results)
	if st.Kind == StepFailed {
		p := g.parent
		p.parked = false
		if fo, took := d.takeFailed(p, g.node, st.Message, st.Code); took {
			d.advance(p, g.node, fo)
			return
		}
		d.fail(p, g.node, st)
		return
	}
	p := g.parent
	p.set(g.node, NodeState{Output: st.Output})
	d.emit(p, TraceStep{Node: g.node, Outcome: st.Outcome, ContextPatch: dryPatch(nil, st.Output)})
	p.parked = false
	d.advance(p, g.node, st.Outcome)
}

// cancel ends tokens and every token they started, in the order the tokens were created.
func (d *dryRun) cancel(names []string) {
	for _, name := range names {
		for _, t := range d.tokens {
			if t.done || t.name != name && !strings.HasPrefix(t.name, name+"/") {
				continue
			}
			t.done, t.waiting = true, false
			d.emit(t, TraceStep{Node: t.at, Outcome: TraceCancelled})
		}
	}
	d.dropTimer(nil)
}

// timeout fires the earliest pending timer.
func (d *dryRun) timeout() {
	var next *dryTimer
	for _, t := range d.timers {
		if t.alive() && (next == nil || t.at < next.at || t.at == next.at && t.seq < next.seq) {
			next = t
		}
	}
	if next == nil {
		return
	}
	d.dropTimer(next)
	if next.at > d.clock {
		d.clock = next.at
	}
	if g := next.group; g != nil {
		g.state.TimedOut = true
		d.decideJoin(g)
		return
	}
	t := next.token
	w := t.at
	outcome, output := d.i.Wake(w, nil)
	t.set(w, NodeState{Output: output}) // an earlier wake's event is not this timeout's output
	d.emit(t, TraceStep{Node: w, Outcome: outcome})
	t.waiting = false
	d.advance(t, w, outcome)
}

// event wakes every waiting token the event matches, in the order the tokens were created.
func (d *dryRun) event(ev Event) {
	var waiting []*dryToken
	for _, t := range d.tokens {
		if !t.done && t.waiting {
			waiting = append(waiting, t)
		}
	}
	for _, t := range waiting {
		if d.ended {
			return
		}
		if t.done || !t.waiting {
			continue
		}
		w := t.at
		matched, st := d.i.WaitMatches(w, ev, t.ctx)
		if st != nil {
			// A filter that fails ends its token (an item is then not ok); the other waits still read
			// the event. Missing context stops the trace.
			d.fail(t, w, *st)
			continue
		}
		if !matched {
			continue
		}
		outcome, output := d.i.Wake(w, ev)
		t.set(w, NodeState{Output: output})
		d.emit(t, TraceStep{Node: w, Outcome: outcome, ContextPatch: map[string]any{"output": output}})
		t.waiting = false
		d.dropTimer(nil)
		d.advance(t, w, outcome)
	}
}

// subjectMismatch is why a run cannot be about subject: the graph is about another entity, or none of a mixed
// graph's triggers is about it. An empty entity is any subject.
func (i *Interpreter) subjectMismatch(s Subject) string {
	if s.Entity == "" || s.Entity == i.view.subject {
		return ""
	}
	// A graph with triggers about different subjects (a mixed one) has runs about each of them.
	for _, t := range i.view.graph.Triggers {
		if i.view.catalog.Events[t.Event].Subject == s.Entity {
			return ""
		}
	}
	return fmt.Sprintf("the graph is about a %s, not a %s", i.view.subject, s.Entity)
}

// callGiven is the answer a dry run takes for a call from the given nodes.<id>: its output and, by the
// node type's answer rule, its outcome; a node whose answer does not name its outcome (a Code node) reads
// nodes.<id>.outcome, which may be left out when the node has one outcome besides its failed one.
func (i *Interpreter) callGiven(nodeID string, given map[string]NodeState) (string, map[string]any, *Step) {
	st, ok := given[nodeID]
	if !ok || st.Output == nil {
		s := Step{Kind: StepMissingContext, Path: fmt.Sprintf("nodes.%s.output", nodeID), Message: "no answer for this call"}
		return "", nil, &s
	}
	node, _ := i.view.Node(nodeID)
	spec := i.view.catalog.Nodes[node.Type]
	if spec.Answer != nil {
		outcome, s := i.CallOutcome(nodeID, st.Output)
		if s != nil {
			return "", nil, s
		}
		return outcome, st.Output, nil
	}
	if s := i.tooLarge(st.Output, "the answer"); s != nil {
		return "", nil, s
	}
	outcomes := i.view.Outcomes(nodeID)
	if st.Outcome != "" {
		if !contains(outcomes, st.Outcome) {
			s := failed(FailUnknownOutcome, "%s has no outcome %s", nodeID, st.Outcome)
			return "", nil, &s
		}
		return st.Outcome, st.Output, nil
	}
	var taken []string
	for _, o := range outcomes {
		if o != spec.FailedOutcome {
			taken = append(taken, o)
		}
	}
	if len(taken) == 1 {
		return taken[0], st.Output, nil
	}
	s := Step{Kind: StepMissingContext, Path: fmt.Sprintf("nodes.%s.outcome", nodeID), Message: fmt.Sprintf("%s has outcomes %v: name the one taken", nodeID, taken)}
	return "", nil, &s
}

// fitError shortens a failure message so the failed output — {"error": message} and the other fields of
// with, if given — stays within MaxValueBytes; the run still takes the failed outcome.
func (i *Interpreter) fitError(message string, with ...map[string]any) string {
	out := map[string]any{}
	if len(with) > 0 {
		for k, v := range with[0] {
			out[k] = v
		}
	}
	fits := func(m string) bool {
		out["error"] = m
		return i.tooLarge(out, "") == nil
	}
	for !fits(message) && message != "" {
		cut := len(message) - len(message)/4 - 1
		for cut > 0 && !utf8.RuneStart(message[cut]) {
			cut--
		}
		message = message[:cut]
	}
	return message
}
