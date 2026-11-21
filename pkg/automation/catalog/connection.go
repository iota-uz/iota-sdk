package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// maxConnectionRef bounds the connection param: a UUID or "platform:<kind>".
const maxConnectionRef = 80

// connectionParams adds the reserved connection param to the params of a node or a trigger that needs
// a connector. The inner spec never sees it.
type connectionParams struct {
	inner graph.ParamsSpec
}

func withConnectionParam(inner graph.ParamsSpec) graph.ParamsSpec {
	if inner == nil {
		inner = emptyParams
	}
	return connectionParams{inner: inner}
}

var emptyParams = func() graph.ParamsSpec {
	p, err := graph.FieldsParams(expr.Fields{})
	if err != nil {
		panic(err)
	}
	return p
}()

func (p connectionParams) JSONSchema() json.RawMessage {
	raw := p.inner.JSONSchema()
	var doc map[string]any
	if err := expr.UnmarshalJSON(raw, &doc); err != nil || doc == nil {
		return raw
	}
	props, _ := doc["properties"].(map[string]any)
	if props == nil {
		props = map[string]any{}
	}
	props[ConnectionParam] = map[string]any{
		"type": "string", "minLength": 1, "maxLength": maxConnectionRef,
		"description": "connection ID or platform:<kind>; left out: the one available connection",
	}
	doc["properties"] = props
	b, err := json.Marshal(doc)
	if err != nil {
		return raw
	}
	return b
}

func (p connectionParams) Parse(raw map[string]any) (map[string]any, []graph.Issue) {
	rest := make(map[string]any, len(raw))
	for k, v := range raw {
		rest[k] = v
	}
	ref, has := rest[ConnectionParam]
	delete(rest, ConnectionParam)
	var issues []graph.Issue
	if has && ref != nil {
		s, ok := ref.(string)
		switch {
		case !ok:
			issues = append(issues, graph.Issue{Path: []any{ConnectionParam}, Message: "connection is a string"})
		case s == "" || len(s) > maxConnectionRef || strings.TrimSpace(s) != s:
			issues = append(issues, graph.Issue{Path: []any{ConnectionParam}, Message: "connection is a connection ID or platform:<kind>"})
		}
	}
	out, inner := p.inner.Parse(rest)
	issues = append(issues, inner...)
	if len(issues) > 0 {
		return nil, issues
	}
	if has && ref != nil {
		out[ConnectionParam] = ref
	}
	return out, nil
}

// declaresConnection reports whether a params schema declares the reserved param itself.
func declaresConnection(p graph.ParamsSpec) bool {
	if p == nil {
		return false
	}
	var doc map[string]any
	if err := expr.UnmarshalJSON(p.JSONSchema(), &doc); err != nil {
		return false
	}
	props, _ := doc["properties"].(map[string]any)
	_, ok := props[ConnectionParam]
	return ok
}

// Errors of ResolveConnection.
var (
	ErrConnectionUnavailable = errors.New("automation catalog: connection is unavailable")
	ErrConnectionRequired    = errors.New("automation catalog: several connections fit; name one with the connection param")
)

// ResolveConnection picks the connection a node or a trigger that requires one of kinds runs with, out
// of the tenant's connected refs: the one named by the connection param (a string), or, when it is
// left out, the only connection of those kinds. A tenant connection overrides the platform credential
// of its own kind only, so enabling a platform grant or adding the tenant's own connection of that kind
// does not break a graph that left the param out, and never switches it to another kind silently. A name that is not a working connection of those kinds
// is ErrConnectionUnavailable; no param and several connections is ErrConnectionRequired.
func ResolveConnection(connected map[ConnectorKind][]ConnectionRef, kinds []ConnectorKind, param any) (ConnectionRef, ConnectorKind, error) {
	ref, _ := param.(string)
	if param != nil && ref == "" {
		return ConnectionRef{}, "", ErrConnectionUnavailable
	}
	var (
		found     []ConnectionRef
		foundKind []ConnectorKind
	)
	seen := map[ConnectorKind]bool{}
	for _, k := range kinds {
		if seen[k] {
			continue
		}
		seen[k] = true
		for _, r := range connected[k] {
			if ref != "" {
				if r.ID == ref {
					return r, k, nil
				}
				continue
			}
			found = append(found, r)
			foundKind = append(foundKind, k)
		}
	}
	switch {
	case ref != "" || len(found) == 0:
		return ConnectionRef{}, "", ErrConnectionUnavailable
	case len(found) == 1:
		return found[0], foundKind[0], nil
	}
	tenantOf := map[ConnectorKind]bool{}
	for i, r := range found {
		if !strings.HasPrefix(r.ID, PlatformPrefix) {
			tenantOf[foundKind[i]] = true
		}
	}
	pick := -1
	for i, r := range found {
		if strings.HasPrefix(r.ID, PlatformPrefix) && tenantOf[foundKind[i]] {
			continue // overridden by the tenant's own connection of this kind
		}
		if pick >= 0 {
			return ConnectionRef{}, "", ErrConnectionRequired
		}
		pick = i
	}
	return found[pick], foundKind[pick], nil
}

// ConnectionRule is the product rule of graph.Options.Rules: a node or a trigger whose connector is not
// connected, or whose connection param names a connection that is not available, is
// connection_unavailable; one that leaves the param out while several connections fit is
// connection_required.
func ConnectionRule(v View) func(graph.Graph) []graph.GraphError {
	return func(g graph.Graph) []graph.GraphError {
		var out []graph.GraphError
		check := func(path string, kinds []ConnectorKind, params map[string]any) {
			if len(kinds) == 0 {
				return
			}
			_, _, err := ResolveConnection(v.Connections, kinds, params[ConnectionParam])
			switch {
			case errors.Is(err, ErrConnectionRequired):
				out = append(out, graph.GraphError{Path: path + ".params." + ConnectionParam, Code: CodeConnectionRequired,
					Message: "several connections fit; choose one"})
			case err != nil:
				out = append(out, graph.GraphError{Path: path + ".params." + ConnectionParam, Code: CodeConnectionUnavailable,
					Message: "the connection is not available"})
			}
		}
		for i, t := range g.Triggers {
			if v.Unavailable[t.Event] == ReasonConnectionUnavailable {
				out = append(out, graph.GraphError{Path: fmt.Sprintf("triggers[%d].event", i), Code: CodeConnectionUnavailable,
					Message: fmt.Sprintf("%s needs a connection the tenant does not have", t.Event)})
				continue
			}
			check(fmt.Sprintf("triggers[%d]", i), v.EventRequires[t.Event], t.Params)
		}
		for i, n := range g.Nodes {
			if v.Unavailable[n.Type] == ReasonConnectionUnavailable {
				out = append(out, graph.GraphError{Path: fmt.Sprintf("nodes[%d].type", i), Code: CodeConnectionUnavailable,
					Message: fmt.Sprintf("%s needs a connection the tenant does not have", n.Type)})
				continue
			}
			check(fmt.Sprintf("nodes[%d]", i), v.NodeRequires[n.Type], n.Params)
		}
		return out
	}
}
