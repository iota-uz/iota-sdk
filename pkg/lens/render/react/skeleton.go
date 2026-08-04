package react

import (
	"fmt"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
)

func skeletonSpanStyle(span int) templ.SafeCSS {
	if span < 1 || span > 12 {
		span = 12
	}
	return templ.SafeCSS(fmt.Sprintf("--lens-panel-span:%d", span))
}

// skeletonCardAttrs labels a placeholder with the panel kind the React runtime
// will see and leaves the shape to CSS.
//
// The runtime replaces this markup in place on its first paint, so the two
// placeholders must reserve the same height or the grid moves at the handoff.
// Rather than have a Go switch and a TypeScript one agree by hand, both sides
// emit the same `data-kind` (plus `data-metrics` for a metric strip) and
// web/lens/src/styles.css decides a kind's shape once, for both.
func skeletonCardAttrs(kind panel.Kind) templ.Attributes {
	attrs := templ.Attributes{
		"class":     "lens-skeleton-card",
		"data-kind": skeletonWireKind(kind),
	}
	if kind == panel.KindStatGroup {
		// A stat group never reaches the runtime as a kind of its own: it
		// arrives as stat panels under a layout group of kind "metrics". The
		// strip of cells is much taller than one stat card, and reserving a
		// stat card for it left the whole first row short by ~150px.
		attrs["data-metrics"] = "true"
	}
	return attrs
}

// skeletonWireKind is the kind string the runtime would label this panel with.
// Container kinds have no wire kind at all; their own name matches no shape rule
// and so falls to the stylesheet's default, which is the plot shape the server
// has always given them.
func skeletonWireKind(kind panel.Kind) string {
	if kind == panel.KindStatGroup {
		return string(document.PanelKindStat)
	}
	if wire, err := document.PanelKindOf(kind); err == nil {
		return string(wire)
	}
	return string(kind)
}
