package graph

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
)

// Event is an event as a graph reads it: {kind, ...payload}.
type Event map[string]any

// Kind is the event's kind.
func (e Event) Kind() string {
	k, _ := e["kind"].(string)
	return k
}

// Payload is the event without its kind.
func (e Event) Payload() map[string]any {
	out := make(map[string]any, len(e))
	for k, v := range e {
		if k != "kind" {
			out[k] = v
		}
	}
	return out
}

// Context is what a step reads: the trigger's event, node outputs, the subject's variables, the tenant and
// the step's time (CEL has no function of the current time).
type Context struct {
	Event  map[string]any
	Nodes  map[string]NodeState // nodes.<id>.output
	Vars   map[string]any       // the subject's variables
	Now    time.Time
	Tenant map[string]any
	// Foreach is foreach.<id> inside the body of a foreach: the token's item and its index.
	Foreach map[string]ForeachItem
	// Events are the events of a run that a trigger's collect merged, in order (event is the last); nil:
	// the one event.
	Events []map[string]any
	// Subject is `subject`: {entity, key, url}; Run is `run`: {id, url}. Nil: empty strings.
	Subject map[string]any
	Run     map[string]any
}

// ForeachItem is what the body of a foreach reads as foreach.<id>.
type ForeachItem struct {
	Item  any `json:"item"`
	Index int `json:"index"`
}

// NodeState is what a node left: its output, and for a call answered without a choice its outcome.
type NodeState struct {
	Output  map[string]any `json:"output"`
	Outcome string         `json:"outcome,omitempty"`
}

// StepKind is what a step decided.
type StepKind string

const (
	StepOutcome        StepKind = "outcome"
	StepWait           StepKind = "wait"
	StepCall           StepKind = "call"
	StepEnd            StepKind = "end"
	StepMissingContext StepKind = "missing_context"
	StepFailed         StepKind = "failed"
	// StepFork starts a token per branch (parallel): Fork lists them; Wait, when set, is the timeout of
	// the parallel's join, counted from the fork. The forking token waits for the join.
	StepFork StepKind = "fork"
	// StepEach starts the body for each item (foreach): Each lists them; the token of item i is
	// ItemToken(token, node, i) and enters Next(node, "item"). The token waits for the items.
	StepEach StepKind = "each"
	// StepArrive ends the token at a join or a collect: Arrive is what it brings.
	StepArrive StepKind = "arrive"
)

// Failure codes of a step.
const (
	FailExpression     = expr.CodeExpressionFailed
	FailInvalidInput   = expr.CodeInvalidInput
	FailInvalidGraph   = "INVALID_GRAPH"
	FailUnknownOutcome = "UNKNOWN_OUTCOME"
	FailInvalidAnswer  = "INVALID_ANSWER"
	FailInvalidRequest = "INVALID_REQUEST"
	FailValueTooLarge  = "VALUE_TOO_LARGE"
	FailInvalidSubject = "INVALID_SUBJECT"
	// FailLimit: a fork or a foreach would exceed Limits.MaxTokens; the run fails, no token is created.
	FailLimit = "LIMIT"
	// FailJoinIncomplete: a join's branches can no longer give what it waits for and it has no timeout.
	FailJoinIncomplete = "JOIN_INCOMPLETE"
	// FailDryRunForbidden: a dry run reached a node whose type forbids dry runs (DryRunForbid).
	FailDryRunForbidden = "DRY_RUN_FORBIDDEN"
)

// Step is one decision of the interpreter. Nothing in it is applied: the engine applies effects and calls.
type Step struct {
	Kind    StepKind
	Outcome string
	Output  map[string]any
	Effects []Effect       // effect params with templates rendered and values evaluated
	Wait    *WaitIntent    // what the run sleeps on
	End     map[string]any // an end node's params
	Path    string         // MissingContext: the field's path
	Code    string         // Failed: EXPRESSION_FAILED | INVALID_INPUT | ...
	Message string
	Fork    []Fork   // StepFork: the new tokens, in the order of the branches
	Each    *Each    // StepEach: the items
	Arrive  *Arrival // StepArrive: at a join or a collect
	Cancel  []string // tokens to cancel with their waits and tasks (a join any / n_of, a join timeout)
}

// Fork is a token a parallel starts and the node it enters (ignore: the branch ends at once).
type Fork struct {
	Token string `json:"token"`
	Node  string `json:"node"`
}

// Each is what a foreach runs its body for: the items, and how many items run at once in the engine.
// A dry run walks the items in their order whatever the concurrency.
type Each struct {
	Items       []any `json:"items"`
	Concurrency int   `json:"concurrency"`
}

// Arrival is what a token brings to a join (Of: the parallel) or a collect (Of: the foreach): the output
// of the branch's last node, or the collect's value; nil when there is none.
type Arrival struct {
	Of    string `json:"of"`
	Token string `json:"token"`
	Value any    `json:"value"`
}

// Effect is a node's rendered params for the engine to apply. Node is the node type (the catalog key).
type Effect struct {
	Node   string         `json:"node"`
	Params map[string]any `json:"params"`
	// Context is the step's expression context as JSON (event, nodes, subject, run, the subject's
	// variables, tenant, now) for a node type that declares StepContext; nil otherwise.
	Context map[string]any `json:"context,omitempty"`
}

// WaitIntent is what a wait sleeps on: an event kind (empty: only the timeout), a filter, a timeout.
type WaitIntent struct {
	Event   string
	Filter  string
	Timeout time.Duration // 0: none
}

func failed(code, format string, args ...any) Step {
	return Step{Kind: StepFailed, Code: code, Message: fmt.Sprintf(format, args...)}
}

// stepOfError turns an evaluation error into a MissingContext or Failed step.
func stepOfError(err error) Step {
	var mc *expr.MissingContextError
	if errors.As(err, &mc) {
		return Step{Kind: StepMissingContext, Path: mc.Path, Message: mc.Error()}
	}
	var ee *expr.EvalError
	if errors.As(err, &ee) {
		return Step{Kind: StepFailed, Code: ee.Code, Message: ee.Message}
	}
	return Step{Kind: StepFailed, Code: FailExpression, Message: err.Error()}
}

func (s Step) stops() bool { return s.Kind == StepMissingContext || s.Kind == StepFailed }

// Options configure validation and interpretation.
type Options struct {
	Behaviors  *Behaviors
	Limits     Limits
	MatchHooks map[string]func(params map[string]any, event Event) bool
	// Rules are product rules checked after the format's (connection_unavailable, ...).
	Rules func(Graph) []GraphError
}

// Limits bound expressions, values and tokens. Zero is the default.
type Limits struct {
	Expr                  expr.Limits
	MaxValueBytes         int // an event's payload and a node's output; 1 MiB by default
	MaxForeachItems       int // the items one foreach runs; 1000 by default
	MaxForeachConcurrency int // a foreach's concurrency; 20 by default
	MaxParallelBranches   int // a parallel's branches; 20 by default
	MaxTokens             int // the active tokens of one run; 1000 by default
}

func orDefault(v, d int) int {
	if v <= 0 {
		return d
	}
	return v
}

// ForeachItems is MaxForeachItems with its default.
func (l Limits) ForeachItems() int { return orDefault(l.MaxForeachItems, 1000) }

// ForeachConcurrency is MaxForeachConcurrency with its default.
func (l Limits) ForeachConcurrency() int { return orDefault(l.MaxForeachConcurrency, 20) }

// ParallelBranches is MaxParallelBranches with its default.
func (l Limits) ParallelBranches() int { return orDefault(l.MaxParallelBranches, 20) }

// Tokens is MaxTokens with its default.
func (l Limits) Tokens() int { return orDefault(l.MaxTokens, 1000) }

func (o Options) behaviors() *Behaviors {
	if o.Behaviors == nil {
		return DefaultBehaviors()
	}
	return o.Behaviors
}

func (o Options) maxValueBytes() int {
	if o.Limits.MaxValueBytes <= 0 {
		return 1 << 20
	}
	return o.Limits.MaxValueBytes
}

// StepContext is the context of one step for one token of a run. Token "" is the main path; split
// behaviors name theirs ("split.legal", "each[0]", nested "split.legal/each[3]").
type StepContext struct {
	Ctx   Context
	Token string
	// From is the node (or trigger) whose outcome led the token here: a join takes its output as the
	// branch's.
	From string
	// Tokens is how many tokens of the run are active, this one included: a fork or a foreach (by its
	// concurrency) that would exceed Limits.MaxTokens fails before it creates any. 0 counts as 1.
	Tokens int
	in     *Interpreter
	// node is the node (or trigger) the step is for: its expressions read what the triggers that lead to it
	// have, as the validator checked them.
	node string
}

// Params are a node's params as its type parsed them.
func (s *StepContext) Params(node Node) map[string]any {
	if s.in == nil {
		return nil
	}
	p, _ := s.in.view.Params(node.ID)
	return p
}

// View is the graph's view: edges, params, reachability, for behaviors that route between branches.
func (s *StepContext) View() *GraphView {
	if s.in == nil {
		return nil
	}
	return s.in.view
}

// Next is where an outcome of a node leads (see Interpreter.Next).
func (s *StepContext) Next(from, outcome string) (string, bool) {
	if s.in == nil {
		return "", false
	}
	return s.in.Next(from, outcome)
}

// Catalog is the interpreter's catalog.
func (s *StepContext) Catalog() Catalog {
	if s.in == nil {
		return Catalog{}
	}
	return s.in.view.Catalog()
}

// Eval evaluates a CEL source of a node in the graph's scope; a non-nil step is MissingContext or Failed.
func (s *StepContext) Eval(src string, want expr.Type) (any, *Step) {
	return s.EvalWith("*", s.Ctx, src, want)
}

// EvalWith evaluates in the scope of an event kind ("*": the graph's own) over a context; node outputs are
// in scope, as for every node expression.
func (s *StepContext) EvalWith(event string, ctx Context, src string, want expr.Type) (any, *Step) {
	return s.eval(event, true, ctx, src, want)
}

func (s *StepContext) eval(event string, withNodes bool, ctx Context, src string, want expr.Type) (any, *Step) {
	if s.in == nil {
		st := failed(FailInvalidGraph, "a step context without an interpreter")
		return nil, &st
	}
	p, issues := s.in.exprs.program(s.node, event, withNodes, src, want)
	if len(issues) > 0 {
		st := failed(FailExpression, "%s", issues[0].Message)
		return nil, &st
	}
	v, err := p.Eval(varsOf(ctx))
	if err != nil {
		st := stepOfError(err)
		return nil, &st
	}
	return v, nil
}

// Effect renders a node's templates and evaluates its values into the params the engine applies.
func (s *StepContext) Effect(node Node, spec NodeSpec) (Effect, *Step) {
	params := s.Params(node)
	rendered := params
	vars := varsOf(s.Ctx)
	templates, values := templatesAndValues(params, spec)
	for _, t := range templates {
		raw, ok := paramAt(params, t)
		text, isStr := raw.(string)
		if !ok || !isStr {
			continue
		}
		tp, issues := s.in.exprs.template(s.node, "*", text)
		if len(issues) > 0 {
			st := failed(FailExpression, "%s", issues[0].Message)
			return Effect{}, &st
		}
		out, err := tp.Render(vars)
		if err != nil {
			st := stepOfError(err)
			return Effect{}, &st
		}
		rendered = setAt(rendered, t, out)
	}
	for _, path := range values {
		raw, ok := paramAt(params, path)
		src, isStr := raw.(string)
		if !ok || !isStr {
			continue
		}
		v, st := s.Eval(src, expr.TypeAny)
		if st != nil {
			return Effect{}, st
		}
		j, err := jsonOut(v)
		if err != nil {
			s := failed(FailExpression, "%s: %v", path, err)
			return Effect{}, &s
		}
		rendered = setAt(rendered, path, j)
	}
	e := Effect{Node: node.Type, Params: rendered}
	if spec.StepContext {
		c, err := jsonOut(vars)
		if err != nil {
			s := failed(FailExpression, "the step's context: %v", err)
			return Effect{}, &s
		}
		e.Context, _ = c.(map[string]any)
	}
	return e, nil
}

// jsonOut is a CEL value as JSON: timestamps as RFC 3339 strings, durations as seconds.
func jsonOut(v any) (any, error) {
	switch x := v.(type) {
	case time.Time:
		return x.UTC().Format(time.RFC3339Nano), nil
	case time.Duration:
		return x.Seconds(), nil
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			j, err := jsonOut(e)
			if err != nil {
				return nil, err
			}
			out[i] = j
		}
		return out, nil
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			j, err := jsonOut(e)
			if err != nil {
				return nil, err
			}
			out[k] = j
		}
		return out, nil
	}
	return v, nil
}

// infoOf is an info object with every field a string ("" when left out).
func infoOf(m map[string]any, fields ...string) map[string]any {
	out := make(map[string]any, len(fields))
	for _, f := range fields {
		s, _ := m[f].(string)
		out[f] = s
	}
	return out
}

// varsOf is a context as CEL variables.
func varsOf(ctx Context) map[string]any {
	vars := make(map[string]any, len(ctx.Vars)+8)
	// subject and run are the run's own unless the subject's context declares a variable of that name.
	vars["subject"] = infoOf(ctx.Subject, "entity", "key", "url")
	vars["run"] = infoOf(ctx.Run, "id", "url")
	for k, v := range ctx.Vars {
		vars[k] = v
	}
	if ctx.Event != nil {
		vars["event"] = ctx.Event
	}
	switch {
	case len(ctx.Events) > 0:
		list := make([]any, len(ctx.Events))
		for i, e := range ctx.Events {
			list[i] = e
		}
		vars["events"] = list
	case ctx.Event != nil:
		vars["events"] = []any{ctx.Event}
	default:
		vars["events"] = []any{}
	}
	nodes := make(map[string]any, len(ctx.Nodes))
	for id, st := range ctx.Nodes {
		entry := map[string]any{}
		if st.Output != nil {
			entry["output"] = st.Output
		}
		nodes[id] = entry
	}
	vars["nodes"] = nodes
	if ctx.Tenant != nil {
		vars["tenant"] = ctx.Tenant
	}
	if !ctx.Now.IsZero() {
		vars["now"] = ctx.Now
	}
	if len(ctx.Foreach) > 0 {
		each := make(map[string]any, len(ctx.Foreach))
		for id, it := range ctx.Foreach {
			each[id] = map[string]any{"item": it.Item, "index": int64(it.Index)}
		}
		vars["foreach"] = each
	}
	return vars
}

// Interpreter interprets one valid graph against its catalog. It holds no run state.
type Interpreter struct {
	view  *GraphView
	opts  Options
	exprs *exprs
	next  map[string]map[string]string
}

// NewInterpreter prepares a graph; it must be valid by the format's rules.
// Product rules (Options.Rules) are not rechecked: a version saved before a rule started failing still runs.
func NewInterpreter(c Catalog, g Graph, o Options) (*Interpreter, error) {
	// The interpreter keeps the hook set it validated, not the caller's map.
	hooks := make(map[string]func(map[string]any, Event) bool, len(o.MatchHooks))
	for name, h := range o.MatchHooks {
		hooks[name] = h
	}
	o.MatchHooks = hooks
	g = Resolve(c, g)
	format := o
	format.Rules = nil
	if errs := Validate(c, g, format); len(errs) > 0 {
		msgs := make([]string, 0, 3)
		for i, e := range errs {
			if i == 3 {
				msgs = append(msgs, fmt.Sprintf("and %d more", len(errs)-3))
				break
			}
			msgs = append(msgs, e.Error())
		}
		return nil, fmt.Errorf("the graph is not valid: %s", strings.Join(msgs, "; "))
	}
	v := buildView(c, g, o)
	x := newExprs(v, o.Limits.Expr)
	x.run = true
	return &Interpreter{view: v, opts: o, exprs: x, next: v.next}, nil
}

// Graph is the interpreted graph.
func (i *Interpreter) Graph() Graph { return i.view.Graph() }

// View is the graph's view.
func (i *Interpreter) View() *GraphView { return i.view }

// Next is where an outcome leads: a node id or "ignore"; ok=false when the outcome has no edge.
func (i *Interpreter) Next(from, outcome string) (string, bool) {
	to, ok := i.next[from][outcome]
	return to, ok
}

func (i *Interpreter) trigger(id string) (Trigger, bool) {
	for _, t := range i.view.graph.Triggers {
		if t.ID == id {
			return t, true
		}
	}
	return Trigger{}, false
}

func (i *Interpreter) tooLarge(v any, what string) *Step {
	b, err := json.Marshal(v)
	if err != nil {
		s := failed(FailInvalidInput, "%s is not JSON: %v", what, err)
		return &s
	}
	if len(b) > i.opts.maxValueBytes() {
		s := failed(FailValueTooLarge, "%s has %d bytes, at most %d", what, len(b), i.opts.maxValueBytes())
		return &s
	}
	return nil
}

// Mismatch reasons of a trigger.
const (
	MismatchEvent  = "event"
	MismatchHook   = "hook"
	MismatchFilter = "filter"
)

// TriggerMismatch is why a trigger does not fire on an event: "event" (another kind), the name of a match
// field whose value is not among the trigger's, "hook" (the event's match hook said no) or "filter" (its
// CEL is false); "" when it fires. A filter that cannot be read stops with its step. Match fields come
// first, so a filter is read only on an event the trigger's fields take.
func (i *Interpreter) TriggerMismatch(trigger string, ev Event, ctx Context) (string, *Step) {
	t, ok := i.trigger(trigger)
	if !ok {
		s := failed(FailInvalidGraph, "no trigger %s", trigger)
		return "", &s
	}
	if ev.Kind() != t.Event {
		return MismatchEvent, nil
	}
	if s := i.tooLarge(ev, "the event"); s != nil {
		return "", s
	}
	spec := i.view.catalog.Events[t.Event]
	params := t.Params
	if parsed, ok := i.view.triggerParams[t.ID]; ok {
		params = parsed // with the defaults the event's params declare
	}
	for _, name := range sortedKeys(spec.Match) {
		f := spec.Match[name]
		want := params[name]
		value, _ := paramAt(ev.Payload(), f.Payload)
		if f.Type == MatchBool {
			if b, ok := want.(bool); ok && value != b {
				return name, nil
			}
			continue
		}
		wants, ok := want.([]any)
		if !ok || len(wants) == 0 {
			continue
		}
		fold := func(v any) string {
			s := fmt.Sprint(v)
			if f.Fold == FoldLower {
				s = strings.ToLower(s)
			}
			return s
		}
		var values []string
		if _, isList := value.([]any); !isList && value != nil {
			if owned, err := jsonValue(value); err == nil {
				value = owned // a typed Go list ([]string) matches as the list its JSON is
			}
		}
		switch v := value.(type) {
		case nil:
		case []any:
			for _, x := range v {
				values = append(values, fold(x))
			}
		default:
			values = append(values, fold(v))
		}
		hit := false
		for _, w := range wants {
			for _, v := range values {
				if fold(w) == v {
					hit = true
				}
			}
		}
		if !hit {
			return name, nil
		}
	}
	if spec.MatchHook != "" {
		hook := i.opts.MatchHooks[spec.MatchHook]
		if hook == nil || !hook(copyMap(params), ev) {
			return MismatchHook, nil
		}
	}
	if t.Filter != "" {
		sc := &StepContext{Ctx: ctx, in: i, node: t.ID}
		sc.Ctx.Event = ev
		v, st := sc.eval(t.Event, false, sc.Ctx, t.Filter, expr.TypeBool) // a trigger reads no node outputs
		if st != nil {
			return "", st
		}
		if v != true {
			return MismatchFilter, nil
		}
	}
	return "", nil
}

// SubjectDeclares reports whether the context of a subject declares a variable: one named subject or
// run overrides the run's own.
func (i *Interpreter) SubjectDeclares(entity, name string) bool {
	_, ok := i.view.catalog.Subjects[entity].Context.Get(name)
	return ok
}

// Trigger is a trigger of the graph by its id.
func (i *Interpreter) Trigger(id string) (Trigger, bool) { return i.trigger(id) }

// CollectKey evaluates a trigger's collect key over an event: the key as a string. ok is false for a
// trigger without collect; a non-nil step is why the key cannot be read.
func (i *Interpreter) CollectKey(trigger string, ev Event, ctx Context) (string, bool, *Step) {
	t, ok := i.trigger(trigger)
	if !ok || t.Collect == nil || t.Collect.Key == "" {
		return "", false, nil
	}
	sc := &StepContext{Ctx: ctx, in: i, node: t.ID}
	sc.Ctx.Event = ev
	v, st := sc.eval(t.Event, false, sc.Ctx, t.Collect.Key, expr.TypeAny)
	if st != nil {
		return "", true, st
	}
	if s, isString := v.(string); isString {
		return s, true, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		st := failed(FailInvalidInput, "the collect key of %s is not JSON: %v", t.ID, err)
		return "", true, &st
	}
	return string(b), true, nil
}

// TriggerMatches reports whether a trigger fires on an event; a non-nil step is why it cannot tell.
func (i *Interpreter) TriggerMatches(trigger string, ev Event, ctx Context) (bool, *Step) {
	m, st := i.TriggerMismatch(trigger, ev, ctx)
	if st != nil {
		return false, st
	}
	return m == "", nil
}

// Start is the step on entering a node.
func (i *Interpreter) Start(nodeID string, s *StepContext) Step {
	return i.start(nodeID, s, true)
}

// start is Start; checkTokens=false leaves MaxTokens to the caller (a dry run counts by its own schedule).
func (i *Interpreter) start(nodeID string, s *StepContext, checkTokens bool) Step {
	node, ok := i.view.Node(nodeID)
	if !ok {
		return failed(FailInvalidGraph, "no node %s", nodeID)
	}
	spec, ok := i.view.catalog.Nodes[node.Type]
	if !ok {
		return failed(FailInvalidGraph, "%s is not a node type of the catalog", node.Type)
	}
	impl, ok := i.opts.behaviors().impl(spec.Behavior)
	if !ok {
		return failed(FailInvalidGraph, "behavior %s does not run here", spec.Behavior)
	}
	sc := StepContext{}
	if s != nil {
		sc = *s
	}
	sc.in = i
	sc.node = nodeID
	st := impl.Start(&sc, node, copyNodeSpec(spec))
	if checkTokens {
		if limit := i.tokenLimit(sc.Tokens, st); limit != nil {
			return *limit
		}
	}
	if st.Output != nil {
		if big := i.tooLarge(st.Output, "the output of "+nodeID); big != nil {
			return *big // the limit holds for every behavior, registered ones included
		}
	}
	return st
}

// CallOutcome is the outcome of a call from its answer, by the node type's answer rule: the chosen value,
// unless a confidence below the node's floor makes it the floor's outcome. A value the node does not offer
// is refused, so a run cannot follow an edge the graph does not draw.
func (i *Interpreter) CallOutcome(nodeID string, output map[string]any) (string, *Step) {
	node, ok := i.view.Node(nodeID)
	spec := i.view.catalog.Nodes[node.Type]
	params, okp := i.view.Params(nodeID)
	if !ok || !okp || spec.Behavior != BehaviorCall || spec.Answer == nil {
		s := failed(FailInvalidGraph, "node %s is not a call with an answer rule", nodeID)
		return "", &s
	}
	if s := i.tooLarge(output, "the answer"); s != nil {
		return "", s
	}
	choice, _ := output[spec.Answer.Choice].(string)
	choices := spec.Outcomes
	if spec.OutcomesFrom != nil {
		choices = fromValues(spec.OutcomesFrom, params)
	}
	if !contains(choices, choice) {
		s := failed(FailUnknownOutcome, "%s offers no choice %q", nodeID, choice)
		return "", &s
	}
	if floor := spec.Answer.Confidence; floor != nil {
		if minConf, ok := expr.Number(params[floor.MinParam]); ok && params[floor.MinParam] != nil {
			conf, ok := expr.Number(output[floor.Field])
			if !ok {
				s := failed(FailInvalidAnswer, "%s: the answer has no %s", nodeID, floor.Field)
				return "", &s
			}
			if conf < minConf {
				return floor.Below, nil
			}
		}
	}
	return choice, nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// WaitIntentOf is what a wait node sleeps on for its params.
func (i *Interpreter) WaitIntentOf(nodeID string) (WaitIntent, bool) {
	node, ok := i.view.Node(nodeID)
	if !ok {
		return WaitIntent{}, false
	}
	spec := i.view.catalog.Nodes[node.Type]
	params, _ := i.view.Params(nodeID)
	if spec.Wait == nil {
		return WaitIntent{}, false
	}
	return waitIntent(spec, params), true
}

func waitIntent(spec NodeSpec, params map[string]any) WaitIntent {
	w := WaitIntent{Event: spec.Wait.Event.Of(params), Filter: spec.Wait.Filter.Of(params)}
	if spec.Wait.TimeoutParam != "" {
		if ms, ok := DurationMs(params[spec.Wait.TimeoutParam]); ok {
			w.Timeout = time.Duration(ms) * time.Millisecond
		}
	}
	return w
}

// WaitMatches reports whether an event wakes a waiting node: its kind, then its filter with `event` being
// this event.
func (i *Interpreter) WaitMatches(nodeID string, ev Event, ctx Context) (bool, *Step) {
	w, ok := i.WaitIntentOf(nodeID)
	if !ok {
		s := failed(FailInvalidGraph, "node %s does not wait", nodeID)
		return false, &s
	}
	if w.Event == "" || ev.Kind() != w.Event {
		return false, nil
	}
	if s := i.tooLarge(ev, "the event"); s != nil {
		return false, s
	}
	if w.Filter == "" {
		return true, nil
	}
	sc := &StepContext{Ctx: ctx, in: i, node: nodeID}
	sc.Ctx.Event = ev
	v, st := sc.EvalWith(w.Event, sc.Ctx, w.Filter, expr.TypeBool)
	if st != nil {
		return false, st
	}
	return v == true, nil
}

// Wake is the outcome and output of a woken wait: matched with the event's payload, or its timeout
// outcome when ev is nil.
func (i *Interpreter) Wake(nodeID string, ev Event) (string, map[string]any) {
	node, _ := i.view.Node(nodeID)
	spec := i.view.catalog.Nodes[node.Type]
	if spec.Wait == nil {
		return "", nil
	}
	if ev == nil {
		return spec.Wait.TimeoutOutcome, map[string]any{}
	}
	return spec.Wait.Matched, map[string]any{"event": ev.Payload()}
}

// FailedOutcome is the outcome a node takes when it fails, if its type has one.
func (i *Interpreter) FailedOutcome(nodeID string) string {
	node, _ := i.view.Node(nodeID)
	return i.view.catalog.Nodes[node.Type].FailedOutcome
}

// structuredFailure reports whether the node's failed output is output.error {code, message, retryable}.
func (i *Interpreter) structuredFailure(nodeID string) bool {
	node, _ := i.view.Node(nodeID)
	return i.view.catalog.Nodes[node.Type].StructuredFailure()
}

// StructuredFailure is structuredFailure for the engine.
func (i *Interpreter) StructuredFailure(nodeID string) bool { return i.structuredFailure(nodeID) }
