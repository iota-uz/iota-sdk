package graph

import (
	"encoding/json"
)

// CatalogView is the catalog as a client reads it: everything an editor needs without Go code.
type CatalogView struct {
	Revision     string                     `json:"revision"`
	Events       []EventView                `json:"events"` // by Key
	Nodes        []NodeView                 `json:"nodes"`  // by Type
	Subjects     map[string]json.RawMessage `json:"subjects"`
	MixedSubject string                     `json:"mixed_subject,omitempty"`
}

// EventView is an event of the catalog for a client.
type EventView struct {
	Key           string                `json:"key"`
	Title         string                `json:"title"`
	Description   string                `json:"description,omitempty"`
	Group         string                `json:"group"`
	Subject       string                `json:"subject"`
	Payload       json.RawMessage       `json:"payload"`
	TriggerParams json.RawMessage       `json:"trigger_params,omitempty"`
	Match         map[string]MatchField `json:"match,omitempty"`
	NoTrigger     bool                  `json:"no_trigger,omitempty"`
	// Participants are the recipient keys of the event's notification definition (empty: none).
	Participants []string `json:"participants,omitempty"`
}

// NodeView is a node type of the catalog for a client.
type NodeView struct {
	Type        string          `json:"type"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Group       string          `json:"group,omitempty"`
	Behavior    Behavior        `json:"behavior"`
	Params      json.RawMessage `json:"params"`
	Subjects    []string        `json:"subjects,omitempty"`
	OutcomeRule OutcomeRule     `json:"outcome_rule"`
	Expressions []ExprParam     `json:"expressions,omitempty"`
	Templates   []string        `json:"templates,omitempty"`
	Output      json.RawMessage `json:"output,omitempty"`
	OutputFrom  *ParamRef       `json:"output_from,omitempty"`
	OutputKeys  *ParamRef       `json:"output_keys,omitempty"`
	Dispatch    *Dispatch       `json:"dispatch,omitempty"`
	// Values are CEL params whose value, as JSON, replaces the source in the effect (an action's args).
	Values []string `json:"values,omitempty"`
	// Mixed are params that are CEL or, with {{ }}, a template.
	Mixed    []string `json:"mixed,omitempty"`
	Requires []string `json:"requires,omitempty"`
	// DryRun is what a dry run does with the node: execute, capture or forbid.
	DryRun DryRunMode `json:"dry_run"`
}

// View is the serializable view of a catalog. Revision is the catalog's, or computed when it has none.
func View(c Catalog, b *Behaviors) CatalogView {
	if b == nil {
		b = DefaultBehaviors()
	}
	v := CatalogView{
		Events:       []EventView{},
		Nodes:        []NodeView{},
		Subjects:     map[string]json.RawMessage{},
		MixedSubject: c.MixedSubject,
	}
	for _, key := range sortedKeys(c.Events) {
		e := c.Events[key]
		ev := EventView{
			Key: key, Title: e.Title, Description: e.Description, Group: e.Group, Subject: e.Subject,
			Payload: e.Payload.JSONSchema(), Match: e.Match, NoTrigger: e.NoTrigger, Participants: e.Participants,
		}
		if ev.Title == "" {
			ev.Title = key
		}
		switch {
		case e.TriggerParams != nil:
			ev.TriggerParams = e.TriggerParams.JSONSchema()
		case len(e.Match) > 0:
			if p, err := MatchParams(e.Match); err == nil {
				ev.TriggerParams = p.JSONSchema()
			}
		}
		v.Events = append(v.Events, ev)
	}
	for _, typ := range sortedKeys(c.Nodes) {
		n := c.Nodes[typ]
		nv := NodeView{
			Type: typ, Title: n.Title, Description: n.Description, Group: n.Group, Behavior: n.Behavior,
			Subjects: n.Subjects, Expressions: n.Expressions, Templates: n.Templates, OutputFrom: n.OutputFrom,
			OutputKeys: n.OutputKeys, Dispatch: n.Dispatch, Values: n.Values, Mixed: n.Mixed, OutcomeRule: OutcomeRule{Fixed: []string{}}, DryRun: n.DryRunOf(),
		}
		if nv.Title == "" {
			nv.Title = typ
		}
		if n.Params != nil {
			nv.Params = n.Params.JSONSchema()
		}
		if impl, ok := b.impl(n.Behavior); ok {
			nv.OutcomeRule = impl.OutcomeRule(n)
		}
		if len(n.Output) > 0 && n.OutputKeys == nil && (n.OutputFrom == nil || n.OutputFrom.Field != "") {
			nv.Output = n.Output.JSONSchema()
		}
		v.Nodes = append(v.Nodes, nv)
	}
	for entity, s := range c.Subjects {
		v.Subjects[entity] = s.Context.JSONSchema()
	}
	v.Revision = c.Revision
	if v.Revision == "" {
		v.Revision = contentHash(c, v)
	}
	return v
}
