package graph

// Dispatch makes a node type the front of a family of node types: a node of it whose params name a key
// in Param is the node type Prefix+key, with the params left without Param. The action node is one:
// {"type": "action", "params": {"action": "core.user.block", "args": {...}}} runs as the node type
// action.core.user.block with {"args": {...}}, which declares that action's args, outcomes and output.
type Dispatch struct {
	Param  string `json:"param"`
	Prefix string `json:"prefix"`
}

// Resolve is the graph with every node of a dispatching type turned into the type it names. A node whose
// key is not a non-empty string keeps its type, so its params are checked against the front's schema.
// The graph passed in is not changed.
func Resolve(c Catalog, g Graph) Graph {
	var out []Node
	for i, n := range g.Nodes {
		spec, ok := c.Nodes[n.Type]
		if !ok || spec.Dispatch == nil {
			continue
		}
		key, _ := n.Params[spec.Dispatch.Param].(string)
		if key == "" {
			continue
		}
		if out == nil {
			out = append([]Node(nil), g.Nodes...)
		}
		params := make(map[string]any, len(n.Params))
		for k, v := range n.Params {
			if k != spec.Dispatch.Param {
				params[k] = v
			}
		}
		n.Type, n.Params = spec.Dispatch.Prefix+key, params
		out[i] = n
	}
	if out == nil {
		return g
	}
	g.Nodes = out
	return g
}
