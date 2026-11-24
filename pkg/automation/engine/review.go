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
	// warnings are what the nodes' Prepare noted without refusing the graph.
	warnings []graph.GraphError
}

// validateMode is how validate treats params a node's Prepare fills in.
type validateMode int

const (
	// fillParams: a save, a check or a try of a graph: filled params become the graph's.
	fillParams validateMode = iota
	// storedParams: a stored version being enabled: params that would change are an error.
	storedParams
)

// CodeParamsOutdated is a stored version whose node would get other params when saved now.
const CodeParamsOutdated = "params_outdated"

var nodePathRe = regexp.MustCompile(`^nodes\[(\d+)\]\.type$`)
var triggerPathRe = regexp.MustCompile(`^triggers\[(\d+)\]\.event$`)

// validate decodes and validates a graph against the catalog the actor sees as an author, with the
// connection rule and the engine's executors. Nothing is written.
func (r *Runtime) validate(ctx context.Context, a Actor, raw json.RawMessage, mode ...validateMode) (validated, error) {
	g, errs := graph.Decode(raw)
	if len(errs) > 0 {
		return validated{}, &ValidationError{Errors: errs}
	}
	connState, err := connectionsState(ctx, r.reader(ctx), a.TenantID)
	if err != nil {
		return validated{}, err
	}
	view, err := r.cfg.Catalog.ForTenant(ctx, a.TenantID, catalog.AsAuthor(a.Subject))
	if err != nil {
		return validated{}, err
	}
	opts := r.graphOptions()
	connections := catalog.ConnectionRule(view)
	opts.Rules = func(g graph.Graph) []graph.GraphError {
		out := append(connections(g), r.executorRule(view, g)...)
		out = append(out, scheduleRule(view, g)...)
		out = append(out, collectRule(view, g)...)
		out = append(out, r.nodeRule(ctx, view, g)...)
		out = append(out, r.nodeRules(view, g)...)
		return append(out, runAsRule(view, g)...)
	}
	errs = graph.Validate(view.Catalog, g, opts)
	errs = connectionErrors(view, g, errs)
	if len(errs) > 0 {
		return validated{}, &ValidationError{Errors: errs}
	}
	fill := len(mode) == 0 || mode[0] == fillParams
	g, changed, errs, warnings, err := r.prepare(ctx, a, view, g, fill, opts)
	if err != nil {
		return validated{}, err
	}
	if changed && len(errs) == 0 {
		// The filled params are checked as the author's own would be.
		errs = connectionErrors(view, g, graph.Validate(view.Catalog, g, opts))
	}
	if len(errs) > 0 {
		return validated{}, &ValidationError{Errors: errs, Warnings: warnings}
	}
	canon, err := json.Marshal(g)
	if err != nil {
		return validated{}, err
	}
	return validated{g: g, raw: canon, view: view, connState: connState, warnings: warnings}, nil
}

// prepare runs catalog.PreparingNode.Prepare of every node whose type has it: params it fills in become
// the node's (fill), or an error of a stored version (not fill); its errors and warnings are the graph's,
// on the node's params.
func (r *Runtime) prepare(ctx context.Context, a Actor, view catalog.View, g graph.Graph, fill bool, opts graph.Options) (graph.Graph, bool, []graph.GraphError, []graph.GraphError, error) {
	var (
		errs, warnings []graph.GraphError
		changed        bool
		types          *graph.ExprTypes
	)
	original := g
	nodes := g.Nodes
	for i, n := range nodes {
		nt, ok := r.cfg.Catalog.Node(n.Type)
		if !ok {
			continue
		}
		pn, ok := nt.(catalog.PreparingNode)
		if !ok {
			continue
		}
		in := catalog.PrepareInput{TenantID: a.TenantID, NodeID: n.ID, Params: map[string]any{}, Author: a.Subject, Fill: fill}
		nodeID := n.ID
		in.TypeOf = func(src string) (string, error) {
			if types == nil {
				types = graph.NewExprTypes(view.Catalog, original, opts)
			}
			return types.TypeOf(nodeID, src)
		}
		in.Author.TenantID = a.TenantID
		for k, v := range n.Params {
			if k != catalog.ConnectionParam {
				in.Params[k] = v
			}
		}
		if kinds := view.NodeRequires[n.Type]; len(kinds) > 0 {
			ref, kind, err := catalog.ResolveConnection(view.Connections, kinds, n.Params[catalog.ConnectionParam])
			if err != nil {
				continue // the connection rule reported it
			}
			in.Connection = &catalog.ConnectionInfo{ID: ref.ID, Kind: kind, Name: ref.Name, Platform: strings.HasPrefix(ref.ID, catalog.PlatformPrefix)}
		}
		res, err := pn.Prepare(ctx, in)
		if err != nil {
			return g, false, nil, nil, fmt.Errorf("prepare %s: %w", n.ID, err)
		}
		at := fmt.Sprintf("nodes[%d].params", i)
		issue := func(p catalog.PrepareIssue) graph.GraphError {
			path := at
			if p.Path != "" {
				path += "." + p.Path
			}
			e := graph.GraphError{Path: path, Code: p.Code, Message: p.Message}
			if p.FixRemove != "" {
				e.Fix = &graph.Fix{Remove: at + "." + p.FixRemove, Label: p.FixLabel}
			}
			return e
		}
		for _, e := range res.Errors {
			errs = append(errs, issue(e))
		}
		for _, w := range res.Warnings {
			warnings = append(warnings, issue(w))
		}
		if res.Params == nil || sameJSON(res.Params, in.Params) {
			continue
		}
		if !fill {
			errs = append(errs, graph.GraphError{Path: at, Code: CodeParamsOutdated,
				Message: n.ID + " would be saved with other params now: save the graph again before enabling it"})
			continue
		}
		next := make(map[string]any, len(res.Params)+1)
		for k, v := range res.Params {
			next[k] = v
		}
		if ref, ok := n.Params[catalog.ConnectionParam]; ok {
			next[catalog.ConnectionParam] = ref
		}
		if !changed {
			nodes = append([]graph.Node(nil), g.Nodes...)
			changed = true
		}
		nodes[i].Params = next
	}
	g.Nodes = nodes
	return g, changed, errs, warnings, nil
}

func sameJSON(a, b any) bool {
	x, err1 := json.Marshal(a)
	y, err2 := json.Marshal(b)
	return err1 == nil && err2 == nil && string(x) == string(y)
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
			if i < len(g.Nodes) && view.UnavailableNodes[g.Nodes[i].Type] == catalog.ReasonConnectionUnavailable {
				e.Code, e.Message = CodeConnectionUnavailable, fmt.Sprintf("%s needs a connection the tenant does not have", g.Nodes[i].Type)
			}
		}
		if m := triggerPathRe.FindStringSubmatch(e.Path); m != nil && e.Code == graph.CodeUnknownEvent {
			var i int
			_, _ = fmt.Sscan(m[1], &i)
			if i < len(g.Triggers) && view.UnavailableEvents[g.Triggers[i].Event] == catalog.ReasonConnectionUnavailable {
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

// nodeRule runs the checks node types make of their params (NodeValidator) on the graph's nodes of the
// catalog.
func (r *Runtime) nodeRule(ctx context.Context, view catalog.View, g graph.Graph) []graph.GraphError {
	var out []graph.GraphError
	for i, n := range g.Nodes {
		if _, ok := view.Catalog.Nodes[n.Type]; !ok {
			continue
		}
		nt, ok := r.cfg.Catalog.Node(n.Type)
		if !ok {
			continue
		}
		nv, ok := nt.(NodeValidator)
		if !ok {
			continue
		}
		params := n.Params
		if params == nil {
			params = map[string]any{}
		}
		for _, e := range nv.ValidateParams(ctx, params) {
			e.Path = fmt.Sprintf("nodes[%d].%s", i, strings.TrimPrefix(e.Path, "."))
			if e.Code == "" {
				e.Code = graph.CodeParams
			}
			out = append(out, e)
		}
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
		if _, asks := nt.(Asker); asks {
			if _, names := nt.(AnswerEventer); !names {
				out = append(out, graph.GraphError{Path: fmt.Sprintf("nodes[%d].type", i), Code: CodeUnsupported,
					Message: n.Type + " waits for an answer without naming its answer event (AnswerEventer)"})
			}
		}
	}
	return out
}

// nodeRules are the rules the node types of a graph bring along (GraphChecker).
func (r *Runtime) nodeRules(view catalog.View, g graph.Graph) []graph.GraphError {
	var out []graph.GraphError
	for i, n := range g.Nodes {
		if _, ok := view.Catalog.Nodes[n.Type]; !ok {
			continue
		}
		nt, _ := r.cfg.Catalog.Node(n.Type)
		if gc, ok := nt.(GraphChecker); ok {
			out = append(out, gc.CheckNode(g, i, view)...)
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
		if len(rv.Connections) > 0 {
			rv.Connections = append([]string{}, rv.Connections...)
			sort.Strings(rv.Connections)
		}
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
	rv.Triggers = triggerReviews(view, g)
	return rv
}

// triggerReviews are the triggers of a graph as a person reads them: a schedule with its time zone.
func triggerReviews(view catalog.View, g graph.Graph) []TriggerReview {
	out := make([]TriggerReview, 0, len(g.Triggers))
	for _, t := range triggersOf(view, g) {
		tr := TriggerReview{TriggerID: t.ID, Kind: t.Kind, Event: t.Event}
		if t.Schedule != nil {
			tr.Schedule = t.Schedule.Cron
			if t.Schedule.Timezone != "" {
				tr.Schedule += " (" + t.Schedule.Timezone + ")"
			}
		}
		out = append(out, tr)
	}
	return out
}
