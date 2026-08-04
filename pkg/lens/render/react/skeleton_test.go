package react

import (
	"testing"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The server placeholder and the runtime's own are the same picture: the React
// runtime replaces this markup in place on the first paint, so a kind that
// reserves one height here and another there moves the grid at the handoff.
// Neither side decides the shape any more — both label the card with the kind
// the runtime sees and web/lens/src/styles.css answers for both — so what this
// table has to hold is the label itself: the same kind must arrive here under
// the same name the runtime knows it by.
func TestSkeletonCardAttrsLabelTheKindTheRuntimeSees(t *testing.T) {
	t.Parallel()
	cases := map[panel.Kind]templ.Attributes{
		// A stat group is not a wire kind at all: it reaches the runtime as
		// stat panels under a layout group of kind "metrics", and the strip of
		// cells reserves far more than one stat card.
		panel.KindStatGroup: {"class": "lens-skeleton-card", "data-kind": "stat", "data-metrics": "true"},
		panel.KindStat:      {"class": "lens-skeleton-card", "data-kind": "stat"},
		// Collapsed kinds must arrive under the collapsed name, or the shape
		// rule written for the runtime's name would miss the server's card.
		panel.KindSegmentBar:    {"class": "lens-skeleton-card", "data-kind": "coverage"},
		panel.KindTimeSeries:    {"class": "lens-skeleton-card", "data-kind": "line"},
		panel.KindStackedBar:    {"class": "lens-skeleton-card", "data-kind": "bar"},
		panel.KindHorizontalBar: {"class": "lens-skeleton-card", "data-kind": "hbar"},
		panel.KindCascade:       {"class": "lens-skeleton-card", "data-kind": "cascade"},
		panel.KindBar:           {"class": "lens-skeleton-card", "data-kind": "bar"},
		panel.KindPie:           {"class": "lens-skeleton-card", "data-kind": "pie"},
		panel.KindTable:         {"class": "lens-skeleton-card", "data-kind": "table"},
		panel.KindMap:           {"class": "lens-skeleton-card", "data-kind": "map"},
		// Container kinds never reach the runtime and have no name there. Their
		// own is carried through, matches no shape rule, and so keeps the
		// stylesheet's plot default — which is what the server always gave them.
		panel.KindTabs:               {"class": "lens-skeleton-card", "data-kind": "tabs"},
		panel.Kind("unknown-future"): {"class": "lens-skeleton-card", "data-kind": "unknown-future"},
	}
	for kind, want := range cases {
		assert.Equal(t, want, skeletonCardAttrs(kind), "skeletonCardAttrs(%q)", kind)
	}
}

// Reusing the builder's own collapse is the whole point: a second copy of it
// here is exactly the duplication the data attribute removed.
func TestSkeletonWireKindFollowsTheDocumentContract(t *testing.T) {
	t.Parallel()
	for _, kind := range []panel.Kind{
		panel.KindStat, panel.KindTimeSeries, panel.KindBar, panel.KindHorizontalBar,
		panel.KindStackedBar, panel.KindSegmentBar, panel.KindCascade, panel.KindPie,
		panel.KindDonut, panel.KindRadial, panel.KindTable, panel.KindGauge,
		panel.KindHistogram, panel.KindBoxPlot, panel.KindHeatmap, panel.KindMap,
		panel.KindMetricFlow, panel.KindMetricHierarchy, panel.KindMetricRelationship,
	} {
		wire, err := document.PanelKindOf(kind)
		require.NoError(t, err, "kind %q", kind)
		assert.Equal(t, string(wire), skeletonWireKind(kind), "kind %q", kind)
	}
}
