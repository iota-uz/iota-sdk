package explore

import (
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/exportmeta"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
)

type Builder struct{ spec Spec }

func New(id, hostPanelID string, branches ...Branch) *Builder {
	return &Builder{spec: Spec{ID: id, HostPanelID: hostPanelID, Branches: append([]Branch(nil), branches...)}}
}

func (b *Builder) ExpandedSpan(span int) *Builder {
	b.spec.ExpandedSpan = span
	return b
}

func (b *Builder) Branches(branches ...Branch) *Builder {
	b.spec.Branches = append(b.spec.Branches, branches...)
	return b
}

func (b *Builder) Build() (Spec, error) {
	if err := b.spec.Validate(); err != nil {
		return Spec{}, err
	}
	return b.spec, nil
}

func NewBranch(key, label, defaultPerspective string, perspectives ...Perspective) Branch {
	return Branch{Key: key, Label: label, DefaultPerspective: defaultPerspective, Perspectives: append([]Perspective(nil), perspectives...)}
}

func NewPerspective(key, label string, semantics Semantics, rootNode string, nodes ...Node) Perspective {
	return Perspective{Key: key, Label: label, Semantics: semantics, RootNode: rootNode, Nodes: append([]Node(nil), nodes...)}
}

func (p Perspective) WithExport(spec exportmeta.Spec) Perspective {
	p.Export = spec
	return p
}

func PanelNode(key, label string, spec panel.Spec, edges ...Edge) Node {
	return Node{Key: key, Label: label, Panel: &spec, Edges: append([]Edge(nil), edges...)}
}

func LazyNode(key, label, url string, edges ...Edge) Node {
	return Node{Key: key, Label: label, Load: &LoadSpec{URL: url, Method: "GET"}, Edges: append([]Edge(nil), edges...)}
}

func (n Node) WithBalance(expected, actual, tolerance float64) Node {
	n.Check = &BalanceCheck{Expected: expected, Actual: actual, Tolerance: tolerance}
	return n
}

func (n Node) WithDynamicEdges(targets ...string) Node {
	n.DynamicEdges = true
	n.DynamicTargets = append([]string(nil), targets...)
	return n
}

func (n Node) WithDynamicChildren(children DynamicChildren, targets ...string) Node {
	n.DynamicEdges = true
	n.DynamicTargets = append([]string(nil), targets...)
	n.DynamicChildren = &children
	return n
}

func ToNode(pointKey, nodeKey string) Edge {
	return Edge{PointKey: pointKey, ToNode: nodeKey}
}

func ToAction(pointKey string, spec action.Spec) Edge {
	return Edge{PointKey: pointKey, Action: &spec}
}
