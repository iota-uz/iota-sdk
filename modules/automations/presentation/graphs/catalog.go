package graphs

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/intl"
)

// EditorCatalog is EditorCatalog of @iota-uz/sdk/automation-editor (#1331): the catalog an author sees,
// with only the node types and events the author may use with the tenant's connections.
type EditorCatalog struct {
	Revision      string         `json:"revision"`
	Events        []CatalogEvent `json:"events"`
	Nodes         []CatalogNode  `json:"nodes"`
	ContextSchema map[string]any `json:"context_schema"`
	// Subjects are the entities a run may be about (a manual start picks one, or none).
	Subjects []string `json:"subjects"`
}

// CatalogEvent is an event a trigger may start on.
type CatalogEvent struct {
	Key           string          `json:"key"`
	Title         string          `json:"title"`
	Group         string          `json:"group"`
	Subject       string          `json:"subject"`
	Payload       map[string]any  `json:"payload"`
	TriggerParams *map[string]any `json:"trigger_params"`
	Manual        bool            `json:"manual,omitempty"`
	Schedule      bool            `json:"schedule,omitempty"`
	Connected     bool            `json:"connected"`
	// Requires are the connector kinds an inbound event comes from (its trigger picks a connection).
	Requires []string `json:"requires,omitempty"`
}

// ParamRef names a param (and a field of its items).
type ParamRef struct {
	Param string `json:"param"`
	Field string `json:"field,omitempty"`
}

// OptionalOutcome is an outcome that exists while a param is set.
type OptionalOutcome struct {
	Param   string `json:"param"`
	Outcome string `json:"outcome"`
}

// OutcomeRule is how a node's outcomes follow from its params.
type OutcomeRule struct {
	From     *ParamRef        `json:"from,omitempty"`
	Fixed    []string         `json:"fixed"`
	Optional *OptionalOutcome `json:"optional,omitempty"`
}

// ExpressionType is the type a CEL param must have.
type ExpressionType string

const (
	ExprBool   ExpressionType = "bool"
	ExprString ExpressionType = "string"
	ExprMap    ExpressionType = "map"
	ExprList   ExpressionType = "list"
	ExprDyn    ExpressionType = "dyn"
)

// ExpressionParam is a CEL param of a node.
type ExpressionParam struct {
	Param string         `json:"param"`
	Type  ExpressionType `json:"type"`
}

// CatalogNode is a node type a graph may use.
type CatalogNode struct {
	Type        string            `json:"type"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Group       string            `json:"group"`
	Behavior    string            `json:"behavior"`
	Params      map[string]any    `json:"params"`
	Outcomes    []string          `json:"outcomes"`
	OutcomeRule OutcomeRule       `json:"outcome_rule"`
	Expressions []ExpressionParam `json:"expressions"`
	Templates   []string          `json:"templates"`
	Output      *map[string]any   `json:"output"`
	// Connector is the first connector kind the node works with; Requires lists them all.
	Connector string   `json:"connector,omitempty"`
	Requires  []string `json:"requires,omitempty"`
	Connected bool     `json:"connected"`
	// Idempotent is set for a call node: whether repeating the call is safe. A Test node of a call that is
	// not idempotent is confirmed as irreversible.
	Idempotent *bool `json:"idempotent,omitempty"`
	// DryRun is what a dry run does with the node: execute, capture or forbid.
	DryRun string `json:"dry_run,omitempty"`
	// Dispatch is set for a front type (the generic "action"): its param names the type it runs, `prefix`
	// plus the param's value (`action.` + `core.user.block`).
	Dispatch *CatalogDispatch `json:"dispatch,omitempty"`
}

// CatalogDispatch is the rule of a front node type: the param that names the type it stands for.
type CatalogDispatch struct {
	Param  string `json:"param"`
	Prefix string `json:"prefix"`
}

// groupKey is the locale key of a palette group; groupFallback its title without one.
func groupKey(group string) string {
	// go-i18n reserves the plural names (one, other, …) as keys.
	if group == "other" {
		group = "Misc"
	}
	return "Automations.Groups." + group
}

func groupFallback(group string) string {
	if group == "" {
		return group
	}
	return strings.ToUpper(group[:1]) + strings.ReplaceAll(group[1:], "_", " ")
}

func groupTitle(ctx context.Context, group string) string {
	if group == "" {
		group = "other"
	}
	return intl.TWithDefault(ctx, groupKey(group), groupFallback(group))
}

// nodeGroup is the palette group of a node type: logic for the built-in and flow nodes, the connector
// for a connector's node, else the first segment of its type.
func nodeGroup(n graph.NodeView) string {
	switch n.Behavior { //nolint:exhaustive // the other behaviors are grouped by their type
	case graph.BehaviorBranch, graph.BehaviorSwitch, graph.BehaviorEnd, graph.BehaviorParallel, graph.BehaviorJoin, graph.BehaviorForeach, graph.BehaviorCollect:
		return "logic"
	}
	switch n.Type {
	case "wait", "delay":
		return "logic"
	case engine.HumanNodeType:
		return "human"
	}
	if len(n.Requires) > 0 {
		return n.Requires[0]
	}
	// A registered action (action.<module>.…) is grouped by the module it belongs to.
	if rest, ok := strings.CutPrefix(n.Type, "action."); ok {
		if i := strings.Index(rest, "."); i > 0 {
			return rest[:i]
		}
	}
	if i := strings.Index(n.Type, "."); i > 0 {
		return n.Type[:i]
	}
	return "other"
}

// eventGroup is the palette group of an event: its module (core.users → core), so the events of one
// module sit together under the module's localized name.
func eventGroup(group string) string {
	if i := strings.Index(group, "."); i > 0 {
		return group[:i]
	}
	return group
}

func schemaMap(raw json.RawMessage) map[string]any {
	out := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	if out == nil {
		out = map[string]any{}
	}
	return out
}

func optionalSchema(raw json.RawMessage) *map[string]any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	m := schemaMap(raw)
	return &m
}

// editorCatalog converts an author's catalog view (catalog.Registry.ForTenant with AsAuthor and the
// user's locale) into the editor's catalog. Events that only wake waits are left out of the trigger
// list; node types and events the view left out (no permission, no connection) are not in it.
func editorCatalog(ctx context.Context, view catalog.View, b *graph.Behaviors, reg catalog.Registry) EditorCatalog {
	cv := view.JSON(b)
	out := EditorCatalog{Revision: cv.Revision, Events: []CatalogEvent{}, Nodes: []CatalogNode{}}
	for _, e := range cv.Events {
		if e.NoTrigger {
			continue
		}
		ev := CatalogEvent{
			Key: e.Key, Title: e.Title, Group: groupTitle(ctx, eventGroup(e.Group)), Subject: e.Subject,
			Payload: schemaMap(e.Payload), TriggerParams: optionalSchema(e.TriggerParams), Connected: true,
			Manual: e.Key == engine.EventManual, Schedule: e.Key == engine.EventSchedule,
		}
		for _, k := range view.EventRequires[e.Key] {
			ev.Requires = append(ev.Requires, string(k))
		}
		if ev.Manual {
			ev.Title = intl.TWithDefault(ctx, "Automations.Triggers.Manual", ev.Title)
		}
		if ev.Schedule {
			ev.Title = intl.TWithDefault(ctx, "Automations.Triggers.Schedule", ev.Title)
		}
		out.Events = append(out.Events, ev)
	}
	for _, n := range cv.Nodes {
		node := CatalogNode{
			Type: n.Type, Title: n.Title, Description: n.Description, Group: groupTitle(ctx, nodeGroup(n)),
			Behavior: string(n.Behavior), Params: schemaMap(n.Params), Outcomes: append([]string{}, n.OutcomeRule.Fixed...),
			OutcomeRule: OutcomeRule{Fixed: append([]string{}, n.OutcomeRule.Fixed...)},
			Expressions: []ExpressionParam{}, Templates: append([]string{}, n.Templates...), Output: optionalSchema(n.Output),
			Requires: n.Requires, Connected: true, DryRun: string(n.DryRun),
		}
		if reg != nil {
			if nt, ok := reg.Node(n.Type); ok {
				if call, ok := nt.(catalog.CallNode); ok {
					idem := call.Idempotent()
					node.Idempotent = &idem
				}
			}
		}
		if n.OutcomeRule.From != nil {
			node.OutcomeRule.From = &ParamRef{Param: n.OutcomeRule.From.Param, Field: n.OutcomeRule.From.Field}
		}
		if n.OutcomeRule.Optional != nil {
			node.OutcomeRule.Optional = &OptionalOutcome{Param: n.OutcomeRule.Optional.Param, Outcome: n.OutcomeRule.Optional.Outcome}
		}
		for _, x := range n.Expressions {
			node.Expressions = append(node.Expressions, ExpressionParam{Param: x.Param, Type: exprType(string(x.Type))})
		}
		// Value params (an action's CEL args) are CEL to the editor too.
		for _, v := range n.Values {
			node.Expressions = append(node.Expressions, ExpressionParam{Param: v, Type: ExprDyn})
		}
		if n.Dispatch != nil {
			node.Dispatch = &CatalogDispatch{Param: n.Dispatch.Param, Prefix: n.Dispatch.Prefix}
		}
		if len(n.Requires) > 0 {
			node.Connector = n.Requires[0]
		}
		out.Nodes = append(out.Nodes, node)
	}
	out.ContextSchema = contextSchema(cv.Subjects)
	out.Subjects = make([]string, 0, len(cv.Subjects))
	for entity := range cv.Subjects {
		out.Subjects = append(out.Subjects, entity)
	}
	sort.Strings(out.Subjects)
	return out
}

func exprType(t string) ExpressionType {
	switch ExpressionType(t) {
	case ExprBool, ExprString, ExprMap, ExprList:
		return ExpressionType(t)
	case ExprDyn:
	}
	return ExprDyn
}

// contextSchema is the scope an expression reads besides the event and the outputs: the run's subject
// (the schema of the only subject entity of the catalog, else any object) and the run.
func contextSchema(subjects map[string]json.RawMessage) map[string]any {
	subject := map[string]any{"type": "object"}
	if len(subjects) == 1 {
		for _, s := range subjects {
			if m := schemaMap(s); len(m) > 0 {
				subject = m
			}
		}
	}
	return map[string]any{"type": "object", "properties": map[string]any{
		"subject": subject,
		"run": map[string]any{"type": "object", "properties": map[string]any{
			"id": map[string]any{"type": "string"}, "url": map[string]any{"type": "string"},
		}},
	}}
}

// connectionOptions are the tenant's working connections by kind, as a node's connection picker lists
// them.
func connectionOptions(view catalog.View) map[string][]ConnectionOption {
	out := map[string][]ConnectionOption{}
	kinds := make([]string, 0, len(view.Connections))
	for k := range view.Connections {
		kinds = append(kinds, string(k))
	}
	sort.Strings(kinds)
	for _, k := range kinds {
		refs := view.Connections[catalog.ConnectorKind(k)]
		list := make([]ConnectionOption, 0, len(refs))
		for _, r := range refs {
			list = append(list, ConnectionOption{ID: r.ID, Name: r.Name})
		}
		out[k] = list
	}
	return out
}
