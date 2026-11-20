package graph

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	Group         string
	Subject       string // the entity the event is about; conversation and none are allowed
	Payload       expr.Fields
	TriggerParams ParamsSpec // nil: params by Match, or none
	Match         map[string]MatchField
	NoTrigger     bool   // the event only wakes waits and starts no graph
	MatchHook     string // a product's matching rule from Options.MatchHooks
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
)

// NodeSpec is a node type of the catalog.
type NodeSpec struct {
	Type        string
	Title       string
	Description string
	Behavior    Behavior
	Params      ParamsSpec
	Subjects    []string // subjects a graph with this node may have; empty: any
	Outcomes    []string // fixed outcomes
	// OutcomesFrom takes outcomes from a param's values (Field: a field of each item of a list of objects).
	OutcomesFrom  *ParamRef
	DoneOutcome   string
	FailedOutcome string
	Expressions   []ExprParam // CEL inside params (dotted path) and the type it must have
	Templates     []string    // {{ }} templates inside params (dotted path)
	Values        []string    // CEL inside params whose value, as JSON, replaces the source in the effect
	Answer        *AnswerSpec // call: which output field gives the outcome, and a confidence floor
	Wait          *WaitSpec
	Output        expr.Fields // what the node puts in nodes.<id>.output
	OutputFrom    *ParamRef   // the output is declared by a JSON Schema in this param (e.g. output_schema)
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

// reservedVars are the variables every scope has; a subject's context may not declare them.
var reservedVars = map[string]bool{"event": true, "nodes": true, "tenant": true, "now": true}

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
		for name, m := range e.Match {
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
	Templates  []string    `json:"templates"`
	Values     []string    `json:"values"`
	Answer     *AnswerSpec `json:"answer"`
	Wait       *waitJSON   `json:"wait"`
	Output     expr.Fields `json:"output"`
	OutputFrom *ParamRef   `json:"output_from"`
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
	c := Catalog{
		Events:       map[string]EventSpec{},
		Nodes:        map[string]NodeSpec{},
		Subjects:     map[string]SubjectSpec{},
		MixedSubject: cj.MixedSubject,
	}
	for entity, s := range cj.Subjects {
		c.Subjects[entity] = SubjectSpec(s)
	}
	subjects := sortedKeys(c.Subjects)
	events := sortedKeys(cj.Events)
	for _, key := range events {
		e := cj.Events[key]
		spec := EventSpec{
			Key: key, Title: e.Title, Group: e.Group, Subject: e.Subject, Payload: e.Payload,
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
		on := n.Subjects
		if len(on) == 0 {
			on = subjects
		}
		switch {
		case n.Behavior == BehaviorBranch:
			c.Nodes[typ] = withType(branchNode(on), typ)
			continue
		case n.Behavior == BehaviorSwitch:
			c.Nodes[typ] = withType(switchNode(on), typ)
			continue
		case n.Behavior == BehaviorWait && n.Params == nil && n.ParamsSchema == nil:
			c.Nodes[typ] = withType(waitNode(on, events), typ)
			continue
		}
		spec := NodeSpec{
			Type: typ, Title: n.Title, Description: n.Description, Behavior: n.Behavior, Subjects: on,
			Outcomes: n.Outcomes, OutcomesFrom: n.OutcomesFrom, DoneOutcome: n.DoneOutcome, FailedOutcome: n.FailedOutcome,
			Templates: n.Templates, Values: n.Values, Answer: n.Answer, Output: n.Output, OutputFrom: n.OutputFrom,
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

// Revision is the hash of the catalog's view: it changes when anything a graph or an editor reads changes.
func Revision(c Catalog, b *Behaviors) string {
	c.Revision = ""
	return View(c, b).Revision
}

// specDetail is what changes how a node or an event runs and the view does not show.
type specDetail struct {
	Outcomes      []string    `json:"outcomes,omitempty"`
	DoneOutcome   string      `json:"done_outcome,omitempty"`
	FailedOutcome string      `json:"failed_outcome,omitempty"`
	Values        []string    `json:"values,omitempty"`
	Answer        *AnswerSpec `json:"answer,omitempty"`
	Wait          *waitDetail `json:"wait,omitempty"`
	MatchHook     string      `json:"match_hook,omitempty"`
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
		d := specDetail{Outcomes: n.Outcomes, DoneOutcome: n.DoneOutcome, FailedOutcome: n.FailedOutcome, Values: n.Values, Answer: n.Answer}
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
