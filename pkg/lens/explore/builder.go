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

// WithView declares the chart kind the wire runtime should render this level
// with, instead of the drill host panel's own kind.
func (n Node) WithView(kind panel.Kind) Node {
	n.View = kind
	return n
}

// WithPresentation carries opt-in rendering hints onto this level.
func (n Node) WithPresentation(hints panel.PresentationHints) Node {
	n.Presentation = &hints
	return n
}

// WithStatus renders a data-quality chip next to this level's heading.
func (n Node) WithStatus(label string, tone panel.StatusTone) Node {
	n.Status = &panel.StatusSpec{Label: label, Tone: tone}
	return n
}

// WithSourceData declares this level's audit table: label is the collapsed
// disclosure heading and table must be a table panel spec whose executed
// frame backs the disclosure.
func (n Node) WithSourceData(label string, table panel.Spec) Node {
	n.SourceData = &SourceData{Label: label, Panel: table}
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
