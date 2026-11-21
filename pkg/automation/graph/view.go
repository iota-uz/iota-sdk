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
	Group         string                `json:"group"`
	Subject       string                `json:"subject"`
	Payload       json.RawMessage       `json:"payload"`
	TriggerParams json.RawMessage       `json:"trigger_params,omitempty"`
	Match         map[string]MatchField `json:"match,omitempty"`
	NoTrigger     bool                  `json:"no_trigger,omitempty"`
}

// NodeView is a node type of the catalog for a client.
type NodeView struct {
	Type        string          `json:"type"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Behavior    Behavior        `json:"behavior"`
	Params      json.RawMessage `json:"params"`
	Subjects    []string        `json:"subjects,omitempty"`
	OutcomeRule OutcomeRule     `json:"outcome_rule"`
	Expressions []ExprParam     `json:"expressions,omitempty"`
	Templates   []string        `json:"templates,omitempty"`
	Output      json.RawMessage `json:"output,omitempty"`
	OutputFrom  *ParamRef       `json:"output_from,omitempty"`
	Requires    []string        `json:"requires,omitempty"`
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
			Key: key, Title: e.Title, Group: e.Group, Subject: e.Subject,
			Payload: e.Payload.JSONSchema(), Match: e.Match, NoTrigger: e.NoTrigger,
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
			Type: typ, Title: n.Title, Description: n.Description, Behavior: n.Behavior,
			Subjects: n.Subjects, Expressions: n.Expressions, Templates: n.Templates, OutputFrom: n.OutputFrom,
			OutcomeRule: OutcomeRule{Fixed: []string{}},
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
		if len(n.Output) > 0 && n.OutputFrom == nil {
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
