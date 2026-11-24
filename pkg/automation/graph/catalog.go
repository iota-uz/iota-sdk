package graph

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
)

// Catalog is what graphs of a tenant may use: events, node types and the context of each subject. A product
// builds it; the package knows only how kinds of nodes behave.
type Catalog struct {
	Events   map[string]EventSpec
	Nodes    map[string]NodeSpec
	Subjects map[string]SubjectSpec // entity → the variables of its expressions
	// MixedSubject is the subject of a graph whose triggers are about different entities; each run is about
	// the subject of its own event.
	MixedSubject string
	// Revision is a hash of the content, kept with each graph version.
	Revision string
}

// Subjects every catalog has besides its entities: a run about a conversation, or about nothing.
const (
	SubjectConversation = "conversation"
	SubjectNone         = "none"
)

// Subject is what a run is about: an entity of the SDK or an application and its key.
type Subject struct {
	Entity string `json:"entity"`
	Key    string `json:"key"`
}

// SubjectSpec declares the variables expressions read about a subject, besides event and nodes.
type SubjectSpec struct {
	Context expr.Fields // each an object or a list
}

// EventSpec is an event a graph may react to.
type EventSpec struct {
	Key           string
	Title         string // localized by the caller
	Description   string // localized by the caller; optional
	Group         string
	Subject       string // the entity the event is about; conversation and none are allowed
	Payload       expr.Fields
	TriggerParams ParamsSpec // nil: params by Match, or none
	Match         map[string]MatchField
	NoTrigger     bool // the event only wakes waits and starts no graph
	// Participants are the recipient keys of the event's notification definition: what a notify node may
	// name as the event's participants. Not part of the catalog format; a registry fills it.
	Participants []string
	MatchHook    string // a product's matching rule from Options.MatchHooks
}

// MatchField is a field a trigger matches by without CEL: its value in the trigger's params (by the field's
// name) is a list of values (strings: the event's value is one of them) or a flag (bool: equal). Left out
// or empty, it does not narrow the trigger.
type MatchField struct {
	Payload string `json:"payload"`        // a dotted path in the payload
	Type    string `json:"type"`           // strings | bool
	Fold    string `json:"fold,omitempty"` // "" | lower
	Title   string `json:"title,omitempty"`
}

// Match field types and folds.
const (
	MatchStrings = "strings"
	MatchBool    = "bool"
	FoldLower    = "lower"
)

// Behavior is how a kind of node runs.
type Behavior string

const (
	BehaviorBranch Behavior = "branch"
	BehaviorSwitch Behavior = "switch"
	BehaviorWait   Behavior = "wait"
	BehaviorEffect Behavior = "effect"
	BehaviorCall   Behavior = "call"
	BehaviorEnd    Behavior = "end"
	// Flow behaviors: several tokens of one run (FlowBehaviors).
	BehaviorParallel Behavior = "parallel"
	BehaviorJoin     Behavior = "join"
	BehaviorForeach  Behavior = "foreach"
	BehaviorCollect  Behavior = "collect"
)

// NodeSpec is a node type of the catalog.
type NodeSpec struct {
	Type        string
	Title       string
	Description string
	Group       string // the palette group (optional)
	Behavior    Behavior
	Params      ParamsSpec
	Subjects    []string // subjects a graph with this node may have; empty: any
	Outcomes    []string // fixed outcomes
	// OutcomesFrom takes outcomes from a param's values (Field: a field of each item of a list of objects).
	OutcomesFrom  *ParamRef
	DoneOutcome   string
	FailedOutcome string
	// Expressions, Templates and Values name params by a dotted path; a "*" segment stands for every key
	// of the object at that point (a map param: values.*, headers.*).
	Expressions []ExprParam // CEL inside params (dotted path) and the type it must have
	Templates   []string    // {{ }} templates inside params (dotted path)
	Values      []string    // CEL inside params whose value, as JSON, replaces the source in the effect
	// MaxOutputBytes, when above the engine's payload limit, is the largest output a node of the type
	// returns (an HTTP response's body, up to 10 MiB); 0: the engine's limit.
	MaxOutputBytes int
	// Mixed are params that are CEL or a template: a string with {{ is a template rendered as Templates
	// are, any other a CEL expression evaluated as Values are (an HTTP body: cel|tpl).
	Mixed  []string
	Answer *AnswerSpec // call: which output field gives the outcome, and a confidence floor
	Wait   *WaitSpec
	Output expr.Fields // what the node puts in nodes.<id>.output
	// OutputFrom: the output is declared by a JSON Schema in this param (e.g. output_schema). With Field
	// set, only that output field takes its type from the schema and the rest is Output (an LLM node's
	// `json` from its `schema`).
	OutputFrom *ParamRef
	// OutputKeys: the output is an object with a dyn field per key of this object param (a set node's
	// values).
	OutputKeys *ParamRef
	// Dispatch makes the type the front of a family of types chosen by a param (Resolve).
	Dispatch *Dispatch
	// StepContext: the effect carries the step's expression context (Effect.Context): a node that runs
	// code reads event, nodes, the subject's variables, tenant and now outside of the step.
	StepContext bool
	// OutputSchemas types fields of Output by JSON Schemas inside params: an output field → the dotted
	// param path of its schema (an MCP tool's pinned output schema). A field whose schema is absent keeps
	// its declared type; one that has it is optional of the schema's type (dyn when the field language
	// cannot express the schema).
	OutputSchemas map[string]string
	// DryRun is what a dry run does with the node: execute it for real (no external effect: a read, an
	// HTTP GET, a decision of a model), capture it (an external effect: send, ask, a write; its rendered
	// params are traced and its outcome comes from the context) or stop on it. Every node type declares
	// it (ValidateCatalog: dry_run_undeclared); the built-in nodes execute.
	DryRun DryRunMode
	// ReadOnlyParam is a dotted bool param that, true, says a node of a capture type makes no external
	// effect (a tool its server declares read-only): a dry run executes it instead of capturing it.
	ReadOnlyParam string
	// Optional is an outcome a call has only when a param is set (a human task's timeout).
	Optional *OptionalOutcome
}

// StructuredFailure reports whether the node's failed outcome carries output.error as an object
// {code, message, retryable}: a call, an effect or a behavior a product registers that does not declare
// an error field of its own. Flow nodes keep their own outputs.
func (n NodeSpec) StructuredFailure() bool {
	if n.FailedOutcome == "" {
		return false
	}
	if _, declared := n.Output.Get("error"); declared {
		return false // the node declares its own error field (a catalog of another product)
	}
	switch n.Behavior { //nolint:exhaustive // the pure built-in behaviors fall through
	case BehaviorCall, BehaviorEffect:
		return true
	case BehaviorWait:
		// A wait that emits its effect fails like an effect when the effect is refused.
		return n.Wait != nil && n.Wait.Effect
	}
	return !isBuiltinBehavior(n.Behavior) && !isFlowBehavior(n.Behavior)
}

// ErrorField is nodes.<id>.output.error of a failed call or effect.
var ErrorField = expr.ObjectOf(expr.Fields{
	{Name: "code", Field: expr.StringField},
	{Name: "message", Field: expr.StringField},
	{Name: "retryable", Field: expr.BoolField},
})

// FailedOutput is the output of a call or an effect that took its failed outcome.
func FailedOutput(code, message string, retryable bool) map[string]any {
	return map[string]any{"error": map[string]any{"code": code, "message": message, "retryable": retryable}}
}

// FailureMessage reads the failure message of a node output: output.error, a string or {message}.
func FailureMessage(output map[string]any) string {
	switch e := output["error"].(type) {
	case string:
		return e
	case map[string]any:
		m, _ := e["message"].(string)
		return m
	}
	return ""
}

// DryRunMode is how a dry run treats a node type.
type DryRunMode string

const (
	DryRunExecute DryRunMode = "execute"
	DryRunCapture DryRunMode = "capture"
	DryRunForbid  DryRunMode = "forbid"
)

func (m DryRunMode) valid() bool {
	return m == DryRunExecute || m == DryRunCapture || m == DryRunForbid
}

// CodeDryRunUndeclared is the code of a node type with an effect that does not say what a dry run does
// with it.
const CodeDryRunUndeclared = "dry_run_undeclared"

// NeedsDryRun reports whether a node type must declare DryRun: its behavior has an effect or is not one
// of the pure built-in ones.
func (n NodeSpec) NeedsDryRun() bool {
	switch n.Behavior { //nolint:exhaustive // the pure built-in behaviors fall through
	case BehaviorCall, BehaviorEffect:
		return true
	case BehaviorWait:
		return n.Wait != nil && n.Wait.Effect
	}
	return !isBuiltinBehavior(n.Behavior) && !isFlowBehavior(n.Behavior)
}

// DryRunOf is the mode of a node type in a dry run: its declaration, execute for a pure node.
func (n NodeSpec) DryRunOf() DryRunMode {
	if n.DryRun != "" {
		return n.DryRun
	}
	if n.NeedsDryRun() {
		return DryRunForbid
	}
	return DryRunExecute
}

// CatalogError is a catalog refused with a code (dry_run_undeclared); other refusals are plain errors.
type CatalogError struct {
	Path    string
	Code    string
	Message string
}

func (e *CatalogError) Error() string { return e.Path + ": " + e.Message }

// ValidateCatalog checks a catalog as Check does and returns its refusal as graph errors with codes.
func ValidateCatalog(c Catalog, b *Behaviors) []GraphError {
	err := c.Check(b)
	if err == nil {
		return nil
	}
	var ce *CatalogError
	if errors.As(err, &ce) {
		return []GraphError{{Path: ce.Path, Code: ce.Code, Message: ce.Message}}
	}
	return []GraphError{{Path: "catalog", Code: CodeInvalid, Message: err.Error()}}
}

// CheckDryRun reports a node type whose DryRun is missing or is not a mode.
func CheckDryRun(n NodeSpec) error {
	switch {
	case n.DryRun != "" && !n.DryRun.valid():
		return &CatalogError{Path: "nodes." + n.Type + ".dry_run", Code: CodeInvalid, Message: fmt.Sprintf("node %s: dry_run is execute, capture or forbid", n.Type)}
	case n.DryRun == "":
		return &CatalogError{Path: "nodes." + n.Type + ".dry_run", Code: CodeDryRunUndeclared,
			Message: fmt.Sprintf("node %s: a node type declares what a dry run does with it (execute, capture or forbid)", n.Type)}
	}
	return nil
}

// ParamRef names a param, and optionally a field of each item of it.
type ParamRef struct {
	Param string `json:"param"`
	Field string `json:"field,omitempty"`
}

// ExprParam is a CEL expression inside params and the type it must have.
type ExprParam struct {
	Param string    `json:"param"`
	Type  expr.Type `json:"type"`
}

// AnswerSpec is how a call's answer gives its outcome: the output field with the chosen value and an
// optional confidence floor (a Field below the MinParam param takes the Below outcome).
type AnswerSpec struct {
	Choice     string           `json:"choice"`
	Confidence *ConfidenceFloor `json:"confidence,omitempty"`
}

// ConfidenceFloor is the confidence rule of an answer.
type ConfidenceFloor struct {
	Field    string `json:"field"`
	MinParam string `json:"min_param"`
	Below    string `json:"below"`
}

// WaitSpec is what a wait sleeps on: an event (none: a delay), a filter over it and a timeout.
type WaitSpec struct {
	Event, Filter  ValueOrParam
	TimeoutParam   string
	Matched        string // outcome when the event arrives
	TimeoutOutcome string
	Effect         bool // the node emits its effect when it starts waiting
}

// ValueOrParam is a fixed value or the value of a param.
type ValueOrParam struct {
	Param string `json:"param,omitempty"`
	Value string `json:"value,omitempty"`
}

// Of reads the value from params.
func (v ValueOrParam) Of(params map[string]any) string {
	if v.Param != "" {
		s, _ := params[v.Param].(string)
		return s
	}
	return v.Value
}

// IsZero reports whether neither a value nor a param is set.
func (v ValueOrParam) IsZero() bool { return v.Param == "" && v.Value == "" }

// reservedVars are the variables every scope has (foreach: inside a foreach's body); a subject's context may not declare them.
var reservedVars = map[string]bool{"event": true, "events": true, "nodes": true, "tenant": true, "now": true, "foreach": true}

// Check reports a catalog that cannot be used: an unknown behavior, a subject that does not exist, a
// context variable that shadows event, nodes, tenant or now.
func (c Catalog) Check(b *Behaviors) error {
	if b == nil {
		b = DefaultBehaviors()
	}
	subjectOK := func(s string) bool {
		_, ok := c.Subjects[s]
		return ok || s == SubjectConversation || s == SubjectNone
	}
	for _, entity := range sortedKeys(c.Subjects) {
		s := c.Subjects[entity]
		if err := s.Context.Validate(); err != nil {
			return fmt.Errorf("subject %s: %w", entity, err)
		}
		for _, p := range s.Context {
			if reservedVars[p.Name] {
				return fmt.Errorf("subject %s: %s is a variable of every scope", entity, p.Name)
			}
			if k := p.Field.Unwrap().Kind; k != expr.Object && k != expr.List {
				return fmt.Errorf("subject %s: %s is an object or a list", entity, p.Name)
			}
		}
	}
	if c.MixedSubject != "" && !subjectOK(c.MixedSubject) {
		return fmt.Errorf("mixed subject %s is not a subject of the catalog", c.MixedSubject)
	}
	for _, key := range sortedKeys(c.Events) {
		e := c.Events[key]
		if e.Key != "" && e.Key != key {
			return fmt.Errorf("event %s: its key is %s", key, e.Key)
		}
		if !subjectOK(e.Subject) {
			return fmt.Errorf("event %s: subject %s is not a subject of the catalog", key, e.Subject)
		}
		if err := e.Payload.Validate(); err != nil {
			return fmt.Errorf("event %s: payload: %w", key, err)
		}
		if _, ok := e.Payload.Get("kind"); ok {
			return fmt.Errorf("event %s: payload: kind is the event's own field", key)
		}
		for _, name := range sortedKeys(e.Match) {
			m := e.Match[name]
			if m.Type != MatchStrings && m.Type != MatchBool {
				return fmt.Errorf("event %s: match %s: type is strings or bool", key, name)
			}
			if m.Fold != "" && m.Fold != FoldLower {
				return fmt.Errorf("event %s: match %s: fold is empty or lower", key, name)
			}
			if m.Payload == "" {
				return fmt.Errorf("event %s: match %s: payload names the field it reads", key, name)
			}
		}
	}
	for _, typ := range sortedKeys(c.Nodes) {
		n := c.Nodes[typ]
		if n.Type != "" && n.Type != typ {
			return fmt.Errorf("node %s: its type is %s", typ, n.Type)
		}
		if _, ok := b.impl(n.Behavior); !ok {
			return fmt.Errorf("node %s: behavior %q is not one of %s", typ, n.Behavior, joinBehaviors(b.Supported()))
		}
		if n.Params == nil {
			return fmt.Errorf("node %s: no params spec", typ)
		}
		for _, s := range n.Subjects {
			if !subjectOK(s) {
				return fmt.Errorf("node %s: subject %s is not a subject of the catalog", typ, s)
			}
		}
		if err := n.Output.Validate(); err != nil {
			return fmt.Errorf("node %s: output: %w", typ, err)
		}
		if n.Wait != nil && isBuiltinBehavior(n.Behavior) && n.Behavior != BehaviorWait {
			return fmt.Errorf("node %s: a %s node does not wait; wait is only for a wait or a behavior that suspends", typ, n.Behavior)
		}
		if err := CheckDryRun(n); err != nil {
			return err
		}
		if n.Behavior == BehaviorCall && len(n.Outcomes) == 0 && n.OutcomesFrom == nil {
			return fmt.Errorf("node %s: a call has outcomes (fixed or from a param)", typ)
		}
		if n.Behavior == BehaviorEffect {
			if err := effectOutcomesError(n); err != "" {
				return fmt.Errorf("node %s: %s", typ, err)
			}
		}
		if n.Behavior == BehaviorWait && n.Wait != nil && len(n.Outcomes) > 0 {
			if err := waitOutcomesError(n); err != "" {
				return fmt.Errorf("node %s: %s", typ, err)
			}
		}
		if n.OutputKeys != nil && (len(n.Output) > 0 || n.OutputFrom != nil) {
			return fmt.Errorf("node %s: output_keys declares the whole output; output and output_from are left out", typ)
		}
		if a := n.Answer; a != nil {
			if a.Choice == "" {
				return fmt.Errorf("node %s: answer names its choice field", typ)
			}
			if c := a.Confidence; c != nil && (c.Field == "" || c.MinParam == "" || c.Below == "") {
				return fmt.Errorf("node %s: answer confidence names field, min_param and below", typ)
			}
		}
		if n.Wait != nil && n.Wait.Event.Value != "" {
			if _, ok := c.Events[n.Wait.Event.Value]; !ok {
				return fmt.Errorf("node %s: waits for %s, not an event of the catalog", typ, n.Wait.Event.Value)
			}
		}
	}
	return nil
}

func joinBehaviors(bs []Behavior) string {
	out := make([]string, len(bs))
	for i, b := range bs {
		out[i] = string(b)
	}
	return strings.Join(out, ", ")
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ---------------------------------------------------------------------------------------------
// Catalogs as data

type catalogJSON struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Changed      string                 `json:"changed"` // why a copied catalog was adapted
	MixedSubject string                 `json:"mixed_subject"`
	Events       map[string]eventJSON   `json:"events"`
	Subjects     map[string]subjectJSON `json:"subjects"`
	Nodes        map[string]nodeJSON    `json:"nodes"`
}

type subjectJSON struct {
	Context expr.Fields `json:"context"`
}

type eventJSON struct {
	Title         string                `json:"title"`
	Description   string                `json:"description"`
	Group         string                `json:"group"`
	Subject       string                `json:"subject"`
	Payload       expr.Fields           `json:"payload"`
	TriggerParams *expr.Fields          `json:"trigger_params"`
	Match         map[string]MatchField `json:"match"`
	NoTrigger     bool                  `json:"no_trigger"`
	MatchHook     string                `json:"match_hook"`
}

type nodeJSON struct {
	Behavior      Behavior        `json:"behavior"`
	Title         string          `json:"title"`
	Description   string          `json:"description"`
	Group         string          `json:"group"`
	Params        *expr.Fields    `json:"params"`
	ParamsSchema  json.RawMessage `json:"params_schema"`
	Subjects      []string        `json:"subjects"`
	Outcomes      []string        `json:"outcomes"`
	OutcomesFrom  *ParamRef       `json:"outcomes_from"`
	DoneOutcome   string          `json:"done_outcome"`
	FailedOutcome string          `json:"failed_outcome"`
	Expressions   []struct {
		Param string  `json:"param"`
		Type  *string `json:"type"`
	} `json:"expressions"`
	Templates   []string    `json:"templates"`
	Values      []string    `json:"values"`
	Answer      *AnswerSpec `json:"answer"`
	Wait        *waitJSON   `json:"wait"`
	Output      expr.Fields `json:"output"`
	OutputFrom  *ParamRef   `json:"output_from"`
	OutputKeys  *ParamRef   `json:"output_keys"`
	StepContext bool        `json:"step_context"`
	DryRun      DryRunMode  `json:"dry_run"`
}

type waitJSON struct {
	Event          ValueOrParam  `json:"event"`
	Filter         *ValueOrParam `json:"filter"`
	Timeout        *ParamRef     `json:"timeout"`
	TimeoutParam   string        `json:"timeout_param"`
	Matched        string        `json:"matched"`
	TimeoutOutcome string        `json:"timeout_outcome"`
	Effect         bool          `json:"effect"`
}

// CatalogFromJSON reads a catalog written as data (the conformance fixtures' format). branch and switch
// take the built-in params; a wait without params is the built-in wait; other nodes declare their params
// in the field language (`params`) or as JSON Schema (`params_schema`). A behavior b does not run is an
// error. Revision is the hash of the catalog's view.
func CatalogFromJSON(raw []byte, b *Behaviors) (Catalog, error) {
	if b == nil {
		b = DefaultBehaviors()
	}
	var cj catalogJSON
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cj); err != nil {
		return Catalog{}, fmt.Errorf("catalog: %w", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return Catalog{}, fmt.Errorf("catalog: unexpected data after the catalog")
	}
	c := Catalog{
		Events:       map[string]EventSpec{},
		Nodes:        map[string]NodeSpec{},
		Subjects:     map[string]SubjectSpec{},
		MixedSubject: cj.MixedSubject,
	}
	for entity, s := range cj.Subjects {
		c.Subjects[entity] = SubjectSpec(s)
	}
	events := sortedKeys(cj.Events)
	for _, key := range events {
		e := cj.Events[key]
		spec := EventSpec{
			Key: key, Title: e.Title, Description: e.Description, Group: e.Group, Subject: e.Subject, Payload: e.Payload,
			Match: e.Match, NoTrigger: e.NoTrigger, MatchHook: e.MatchHook,
		}
		if spec.Title == "" {
			spec.Title = key
		}
		if e.TriggerParams != nil {
			p, err := FieldsParams(*e.TriggerParams)
			if err != nil {
				return Catalog{}, fmt.Errorf("event %s: trigger_params: %w", key, err)
			}
			spec.TriggerParams = p
		}
		c.Events[key] = spec
	}
	for _, typ := range sortedKeys(cj.Nodes) {
		n := cj.Nodes[typ]
		if _, ok := b.impl(n.Behavior); !ok {
			return Catalog{}, fmt.Errorf("node %s: behavior %q is not one of %s", typ, n.Behavior, joinBehaviors(b.Supported()))
		}
		on := n.Subjects // empty or left out: any subject, as NodeSpec.Subjects reads it
		switch {
		case n.Behavior == BehaviorBranch:
			c.Nodes[typ] = withDryRun(withType(branchNode(on), typ), n.DryRun)
			continue
		case n.Behavior == BehaviorSwitch:
			c.Nodes[typ] = withDryRun(withType(switchNode(on), typ), n.DryRun)
			continue
		case n.Behavior == BehaviorWait && n.Params == nil && n.ParamsSchema == nil:
			c.Nodes[typ] = withDryRun(withType(waitNode(on, events), typ), n.DryRun)
			continue
		case isFlowBehavior(n.Behavior):
			// parallel, join, foreach and collect take the built-in params and outputs: the interpreter
			// reads them, so a catalog cannot change them.
			spec := withType(flowNode(n.Behavior, on), typ)
			if n.Title != "" {
				spec.Title = n.Title
			}
			if n.Description != "" {
				spec.Description = n.Description
			}
			c.Nodes[typ] = withDryRun(spec, n.DryRun)
			continue
		}
		spec := NodeSpec{
			Type: typ, Title: n.Title, Description: n.Description, Group: n.Group, Behavior: n.Behavior, Subjects: on,
			Outcomes: n.Outcomes, OutcomesFrom: n.OutcomesFrom, DoneOutcome: n.DoneOutcome, FailedOutcome: n.FailedOutcome,
			Templates: n.Templates, Values: n.Values, Answer: n.Answer, Output: n.Output, OutputFrom: n.OutputFrom,
			OutputKeys: n.OutputKeys, StepContext: n.StepContext, DryRun: n.DryRun,
		}
		if spec.Title == "" {
			spec.Title = typ
		}
		for _, e := range n.Expressions {
			t := expr.TypeAny
			if e.Type != nil {
				t = expr.Type(*e.Type)
			}
			spec.Expressions = append(spec.Expressions, ExprParam{Param: e.Param, Type: t})
		}
		var err error
		switch {
		case n.ParamsSchema != nil:
			spec.Params, err = SchemaParams(n.ParamsSchema)
		case n.Params != nil:
			spec.Params, err = FieldsParams(*n.Params)
		default:
			spec.Params, err = FieldsParams(expr.Fields{})
		}
		if err != nil {
			return Catalog{}, fmt.Errorf("node %s: params: %w", typ, err)
		}
		if w := n.Wait; w != nil {
			ws := &WaitSpec{Event: w.Event, Matched: w.Matched, TimeoutOutcome: w.TimeoutOutcome, Effect: w.Effect, TimeoutParam: w.TimeoutParam}
			if w.Filter != nil {
				ws.Filter = *w.Filter
			}
			if w.Timeout != nil {
				ws.TimeoutParam = w.Timeout.Param
			}
			spec.Wait = ws
		}
		if spec.DryRun == "" {
			// A catalog written as data describes a product's nodes without running them: an effect it
			// does not declare is captured, a pure node executes.
			spec.DryRun = DryRunExecute
			if spec.NeedsDryRun() {
				spec.DryRun = DryRunCapture
			}
		}
		c.Nodes[typ] = spec
	}
	if err := c.Check(b); err != nil {
		return Catalog{}, err
	}
	c.Revision = Revision(c, b)
	return c, nil
}

func withType(n NodeSpec, typ string) NodeSpec {
	n.Type = typ
	return n
}

// withDryRun keeps a catalog's explicit dry run mode of a built-in node (Check validates it); left out,
// the built-in's own.
func withDryRun(n NodeSpec, m DryRunMode) NodeSpec {
	if m != "" {
		n.DryRun = m
	}
	return n
}

// Revision is the hash of the catalog's view: it changes when anything a graph or an editor reads changes.
func Revision(c Catalog, b *Behaviors) string {
	c.Revision = ""
	return View(c, b).Revision
}

// specDetail is what changes how a node or an event runs and the view does not show.
type specDetail struct {
	Outcomes      []string          `json:"outcomes,omitempty"`
	DoneOutcome   string            `json:"done_outcome,omitempty"`
	FailedOutcome string            `json:"failed_outcome,omitempty"`
	Values        []string          `json:"values,omitempty"`
	Answer        *AnswerSpec       `json:"answer,omitempty"`
	Wait          *waitDetail       `json:"wait,omitempty"`
	MatchHook     string            `json:"match_hook,omitempty"`
	Optional      *OptionalOutcome  `json:"optional,omitempty"`
	StepContext   bool              `json:"step_context,omitempty"`
	Dispatch      *Dispatch         `json:"dispatch,omitempty"`
	Mixed         []string          `json:"mixed,omitempty"`
	MaxOutput     int               `json:"max_output,omitempty"`
	ReadOnlyParam string            `json:"read_only_param,omitempty"`
	OutputSchemas map[string]string `json:"output_schemas,omitempty"`
}

type waitDetail struct {
	Event          ValueOrParam `json:"event"`
	Filter         ValueOrParam `json:"filter"`
	TimeoutParam   string       `json:"timeout_param,omitempty"`
	Matched        string       `json:"matched,omitempty"`
	TimeoutOutcome string       `json:"timeout_outcome,omitempty"`
	Effect         bool         `json:"effect,omitempty"`
}

// contentHash hashes the view together with the details of every node and event.
func contentHash(c Catalog, v CatalogView) string {
	details := map[string]specDetail{}
	for typ, n := range c.Nodes {
		d := specDetail{Outcomes: n.Outcomes, DoneOutcome: n.DoneOutcome, FailedOutcome: n.FailedOutcome, Values: n.Values, Mixed: n.Mixed, MaxOutput: n.MaxOutputBytes, Answer: n.Answer, Optional: n.Optional, StepContext: n.StepContext, Dispatch: n.Dispatch, ReadOnlyParam: n.ReadOnlyParam, OutputSchemas: n.OutputSchemas}
		if w := n.Wait; w != nil {
			d.Wait = &waitDetail{Event: w.Event, Filter: w.Filter, TimeoutParam: w.TimeoutParam, Matched: w.Matched, TimeoutOutcome: w.TimeoutOutcome, Effect: w.Effect}
		}
		details["node:"+typ] = d
	}
	for key, e := range c.Events {
		if e.MatchHook != "" {
			details["event:"+key] = specDetail{MatchHook: e.MatchHook}
		}
	}
	v.Revision = ""
	body, err := json.Marshal(struct {
		View    CatalogView           `json:"view"`
		Details map[string]specDetail `json:"details"`
	}{v, details})
	if err != nil {
		panic(fmt.Sprintf("graph: marshal a catalog: %v", err))
	}
	return revisionOf(body)
}

func revisionOf(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:12])
}
