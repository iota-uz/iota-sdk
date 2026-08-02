package react

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
)

// The server placeholder and the runtime's own are the same picture: the React
// runtime replaces this markup in place on the first paint, so a kind that
// reserves one height here and another there moves the grid at the handoff.
// The counterpart table lives in web/lens/src/panels/Skeleton.test.tsx.
func TestSkeletonCardClassMatchesRuntimeShapes(t *testing.T) {
	t.Parallel()
	cases := map[panel.Kind]string{
		panel.KindStat:               "lens-skeleton-card lens-skeleton-card-stat",
		panel.KindStatGroup:          "lens-skeleton-card lens-skeleton-card-metrics",
		panel.KindSegmentBar:         "lens-skeleton-card lens-skeleton-card-compact",
		panel.KindCascade:            "lens-skeleton-card lens-skeleton-card-compact",
		panel.KindBar:                "lens-skeleton-card lens-skeleton-card-plot",
		panel.KindTimeSeries:         "lens-skeleton-card lens-skeleton-card-plot",
		panel.KindPie:                "lens-skeleton-card lens-skeleton-card-plot",
		panel.KindTable:              "lens-skeleton-card lens-skeleton-card-plot",
		panel.KindMap:                "lens-skeleton-card lens-skeleton-card-plot",
		panel.KindMetricHierarchy:    "lens-skeleton-card lens-skeleton-card-plot",
		panel.Kind("unknown-future"): "lens-skeleton-card lens-skeleton-card-plot",
	}
	for kind, want := range cases {
		if got := skeletonCardClass(kind); got != want {
			t.Errorf("skeletonCardClass(%q) = %q, want %q", kind, got, want)
		}
	}
}
