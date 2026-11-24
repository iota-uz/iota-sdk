package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// validated is a graph that passed validation against an author's catalog.
type validated struct {
	g    graph.Graph
	raw  json.RawMessage // canonical JSON of g
	view catalog.View
	// connState is the tenant's connections (connectionsState) read before view: an enable compares it
	// under its locks to tell whether view still resolves the graph's connections.
	connState string
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
	connState, err := connectionsState(ctx, r.cfg.DB, a.TenantID)
	if err != nil {
		return validated{}, err
	}
	view, err := r.cfg.Catalog.ForTenant(ctx, a.TenantID, catalog.AsAuthor(a.Subject))
	if err != nil {
		return validated{}, err
	}
	g = r.completeParams(ctx, a.TenantID, view, g)
	opts := r.graphOptions()
	connections := catalog.ConnectionRule(view)
	opts.Rules = func(g graph.Graph) []graph.GraphError {
		out := append(connections(g), r.executorRule(view, g)...)
		out = append(out, r.connectionNodeRule(ctx, a.TenantID, view, g)...)
		out = append(out, scheduleRule(view, g)...)
		out = append(out, collectRule(view, g)...)
		return append(out, runAsRule(view, g)...)
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
	return validated{g: g, raw: canon, view: view, connState: connState}, nil
}

// completeParams lets each node type that implements catalog.CompletedNode pin the values it fixes at
// save time (an AI node's default instruction), and each that implements catalog.ConnectionNode pin what
// depends on its connection (the default model of its provider); the graph it returns is a copy.
func (r *Runtime) completeParams(ctx context.Context, tenant uuid.UUID, view catalog.View, g graph.Graph) graph.Graph {
	out := g
	copied := false
	for i, n := range g.Nodes {
		nt, ok := r.cfg.Catalog.Node(n.Type)
		if !ok {
			continue
		}
		params := n.Params
		if cn, ok := nt.(catalog.CompletedNode); ok {
			params = cn.CompleteParams(params)
		}
		if cn, ok := nt.(catalog.ConnectionNode); ok {
			if ref, _, ok := nodeConnection(view, nt, params); ok {
				params = cn.CompleteParamsFor(ctx, tenant, ref, params)
			}
		}
		if !copied {
			out.Nodes = append([]graph.Node(nil), g.Nodes...)
			copied = true
		}
		out.Nodes[i].Params = params
	}
	return out
}

// nodeConnection is the connection a node of the graph resolves to for the tenant now, as it does at run
// time; ok is false for a node that needs none or finds none.
func nodeConnection(view catalog.View, nt catalog.NodeType, params map[string]any) (catalog.ConnectionRef, catalog.ConnectorKind, bool) {
	kinds := nt.Requires()
	if len(kinds) == 0 {
		return catalog.ConnectionRef{}, "", false
	}
	ref, kind, err := catalog.ResolveConnection(view.Connections, kinds, params[catalog.ConnectionParam])
	return ref, kind, err == nil
}

// connectionNodeRule asks each catalog.ConnectionNode whether its params fit the connection it will run
// with.
func (r *Runtime) connectionNodeRule(ctx context.Context, tenant uuid.UUID, view catalog.View, g graph.Graph) []graph.GraphError {
	var out []graph.GraphError
	for i, n := range g.Nodes {
		nt, ok := r.cfg.Catalog.Node(n.Type)
		if !ok {
			continue
		}
		cn, ok := nt.(catalog.ConnectionNode)
		if !ok {
			continue
		}
		ref, _, ok := nodeConnection(view, nt, n.Params)
		if !ok {
			continue
		}
		for _, issue := range cn.CheckParamsFor(ctx, tenant, ref, n.Params) {
			code := issue.Code
			if code == "" {
				code = graph.CodeParams
			}
			path := fmt.Sprintf("nodes[%d].params", i)
			for _, p := range issue.Path {
				path += fmt.Sprintf(".%v", p)
			}
			out = append(out, graph.GraphError{Path: path, Code: code, Message: issue.Message})
		}
	}
	return out
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

// collectRule refuses collect on a trigger that is not an inbound trigger of a connector.
func collectRule(view catalog.View, g graph.Graph) []graph.GraphError {
	var out []graph.GraphError
	for i, t := range g.Triggers {
		if t.Collect != nil && triggerKind(view, t.Event) != KindInbound {
			out = append(out, graph.GraphError{Path: fmt.Sprintf("triggers[%d].collect", i), Code: graph.CodeParams,
				Message: "collect merges inbound messages of a connector; " + t.Event + " is not one"})
		}
	}
	return out
}

// scheduleRule refuses a schedule trigger whose cron is not five calendar fields or whose time zone is
// unknown (checkSchedule).
func scheduleRule(view catalog.View, g graph.Graph) []graph.GraphError {
	var out []graph.GraphError
	for i, t := range g.Triggers {
		if triggerKind(view, t.Event) != KindSchedule {
			continue
		}
		var p scheduleParams
		p.Cron, _ = t.Params["cron"].(string)
		p.Timezone, _ = t.Params["timezone"].(string)
		if err := checkSchedule(p); err != nil {
			path := fmt.Sprintf("triggers[%d].params.cron", i)
			if p.Timezone != "" && strings.Contains(err.Error(), "time zone") {
				path = fmt.Sprintf("triggers[%d].params.timezone", i)
			}
			out = append(out, graph.GraphError{Path: path, Code: graph.CodeParams, Message: err.Error()})
		}
	}
	return out
}

// runAsRule refuses run_as initiator for a graph with a schedule trigger: a slot has no initiator, and
// the run must not fall back to the owner's rights.
func runAsRule(view catalog.View, g graph.Graph) []graph.GraphError {
	if g.RunAs != graph.RunAsInitiator {
		return nil
	}
	for i, t := range g.Triggers {
		if triggerKind(view, t.Event) == KindSchedule {
			return []graph.GraphError{{Path: fmt.Sprintf("triggers[%d].event", i), Code: graph.CodeInvalid,
				Message: "a schedule has no initiator: a graph with a schedule trigger runs as its owner"}}
		}
	}
	return nil
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
		if _, asks := nt.(Asker); asks {
			if _, names := nt.(AnswerEventer); !names {
				out = append(out, graph.GraphError{Path: fmt.Sprintf("nodes[%d].type", i), Code: CodeUnsupported,
					Message: n.Type + " waits for an answer without naming its answer event (AnswerEventer)"})
			}
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
