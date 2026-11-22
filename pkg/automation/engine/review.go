package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// validated is a graph that passed validation against an author's catalog.
type validated struct {
	g    graph.Graph
	raw  json.RawMessage // canonical JSON of g
	view catalog.View
}

var nodePathRe = regexp.MustCompile(`^nodes\[(\d+)\]\.type$`)
var triggerPathRe = regexp.MustCompile(`^triggers\[(\d+)\]\.event$`)

// validate decodes and validates a graph against the catalog the actor sees as an author, with the
// connection rule and the engine's executors. Nothing is written.
func (r *Runtime) validate(ctx context.Context, a Actor, raw json.RawMessage) (validated, error) {
	g, errs := graph.Decode(raw)
	if len(errs) > 0 {
		return validated{}, &ValidationError{Errors: errs}
	}
	view, err := r.cfg.Catalog.ForTenant(ctx, a.TenantID, catalog.AsAuthor(a.Subject))
	if err != nil {
		return validated{}, err
	}
	opts := r.graphOptions()
	connections := catalog.ConnectionRule(view)
	opts.Rules = func(g graph.Graph) []graph.GraphError {
		return append(connections(g), r.executorRule(view, g)...)
	}
	errs = graph.Validate(view.Catalog, g, opts)
	errs = connectionErrors(view, g, errs)
	if len(errs) > 0 {
		return validated{}, &ValidationError{Errors: errs}
	}
	canon, err := json.Marshal(g)
	if err != nil {
		return validated{}, err
	}
	return validated{g: g, raw: canon, view: view}, nil
}

// connectionErrors reports a node type or an event the tenant's catalog lacks for want of a connection
// as connection_unavailable on its path, once.
func connectionErrors(view catalog.View, g graph.Graph, errs []graph.GraphError) []graph.GraphError {
	seen := map[string]bool{}
	out := make([]graph.GraphError, 0, len(errs))
	for _, e := range errs {
		if m := nodePathRe.FindStringSubmatch(e.Path); m != nil && e.Code == graph.CodeUnknownNodeType {
			var i int
			_, _ = fmt.Sscan(m[1], &i)
			if i < len(g.Nodes) && view.Unavailable[g.Nodes[i].Type] == catalog.ReasonConnectionUnavailable {
				e.Code, e.Message = CodeConnectionUnavailable, fmt.Sprintf("%s needs a connection the tenant does not have", g.Nodes[i].Type)
			}
		}
		if m := triggerPathRe.FindStringSubmatch(e.Path); m != nil && e.Code == graph.CodeUnknownEvent {
			var i int
			_, _ = fmt.Sscan(m[1], &i)
			if i < len(g.Triggers) && view.Unavailable[g.Triggers[i].Event] == catalog.ReasonConnectionUnavailable {
				e.Code, e.Message = CodeConnectionUnavailable, fmt.Sprintf("%s needs a connection the tenant does not have", g.Triggers[i].Event)
			}
		}
		k := e.Path + "|" + e.Code
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, e)
	}
	return out
}

// executorRule refuses call and human nodes this process cannot execute.
func (r *Runtime) executorRule(view catalog.View, g graph.Graph) []graph.GraphError {
	var out []graph.GraphError
	for i, n := range g.Nodes {
		spec, ok := view.Catalog.Nodes[n.Type]
		if !ok || spec.Behavior != graph.BehaviorCall {
			continue
		}
		nt, _ := r.cfg.Catalog.Node(n.Type)
		if h, ok := nt.(HumanNode); ok && h.Human() {
			if r.cfg.Tasks == nil {
				out = append(out, graph.GraphError{Path: fmt.Sprintf("nodes[%d].type", i), Code: CodeConnectionUnavailable, Message: "human tasks are not available"})
			}
			continue
		}
		if r.cfg.Calls == nil {
			out = append(out, graph.GraphError{Path: fmt.Sprintf("nodes[%d].type", i), Code: CodeConnectionUnavailable, Message: "external calls are not available"})
		}
	}
	return out
}

// codeNodes are the Code nodes of a graph as the reviewer sees them, ordered by node id.
func (r *Runtime) codeNodes(g graph.Graph) []CodeNodeReview {
	out := make([]CodeNodeReview, 0, len(g.Nodes))
	for _, n := range g.Nodes {
		nt, ok := r.cfg.Catalog.Node(n.Type)
		if !ok {
			continue
		}
		cn, ok := nt.(CodeNode)
		if !ok {
			continue
		}
		rv := cn.Review(n.Params)
		rv.NodeID = n.ID
		rv.Secrets = append([]string{}, rv.Secrets...)
		sort.Strings(rv.Secrets)
		out = append(out, rv)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].NodeID < out[j].NodeID })
	return out
}

// codeDigest is sha256 over the code nodes (id, lang, source, declared secrets, fetch allow); "" without any.
func codeDigest(nodes []CodeNodeReview) string {
	if len(nodes) == 0 {
		return ""
	}
	b, err := json.Marshal(nodes)
	if err != nil {
		panic(fmt.Sprintf("automation engine: marshal code nodes: %v", err))
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// triggerRow is a trigger of a version as automation.triggers keeps it.
type triggerRow struct {
	ID       string
	Kind     string
	Event    string
	Filter   string
	Schedule *scheduleParams
}

// triggersOf are the triggers of a version.
func triggersOf(view catalog.View, g graph.Graph) []triggerRow {
	out := make([]triggerRow, 0, len(g.Triggers))
	for _, t := range g.Triggers {
		tr := triggerRow{ID: t.ID, Kind: triggerKind(view, t.Event), Event: t.Event, Filter: t.Filter}
		if tr.Kind == KindSchedule {
			p := scheduleParams{}
			p.Cron, _ = t.Params["cron"].(string)
			p.Timezone, _ = t.Params["timezone"].(string)
			tr.Schedule = &p
		}
		out = append(out, tr)
	}
	return out
}

func reviewOf(r *Runtime, view catalog.View, v versionRow, g graph.Graph) EnableReview {
	nodes := r.codeNodes(g)
	rv := EnableReview{Version: v.Version.Version, CodeNodes: nodes, CodeDigest: codeDigest(nodes), RunAs: string(g.RunAs)}
	if rv.CodeNodes == nil {
		rv.CodeNodes = []CodeNodeReview{}
	}
	for _, t := range triggersOf(view, g) {
		tr := TriggerReview{TriggerID: t.ID, Kind: t.Kind, Event: t.Event}
		if t.Schedule != nil {
			tr.Schedule = t.Schedule.Cron
			if t.Schedule.Timezone != "" {
				tr.Schedule += " (" + t.Schedule.Timezone + ")"
			}
		}
		rv.Triggers = append(rv.Triggers, tr)
	}
	return rv
}
