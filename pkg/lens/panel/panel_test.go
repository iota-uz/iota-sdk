package panel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// allKinds lists the panel kinds covered by the predicate tests. Keep it in
// sync with the const block above so the table and partition checks cover every
// known kind.
var allKinds = []Kind{
	KindStat,
	KindTimeSeries,
	KindBar,
	KindHorizontalBar,
	KindStackedBar,
	KindSegmentBar,
	KindCascade,
	KindPie,
	KindDonut,
	KindTable,
	KindGauge,
	KindTabs,
	KindGrid,
	KindSplit,
	KindRepeat,
}

func TestKindPredicates_ClassifiesAllKnownKinds(t *testing.T) {
	cases := []struct {
		kind            Kind
		isContainer     bool
		isChart         bool
		rendersNatively bool
	}{
		{KindStat, false, false, true},
		{KindTimeSeries, false, true, false},
		{KindBar, false, true, false},
		{KindHorizontalBar, false, true, false},
		{KindStackedBar, false, true, false},
		{KindSegmentBar, false, false, true},
		{KindCascade, false, false, true},
		{KindPie, false, true, false},
		{KindDonut, false, true, false},
		{KindTable, false, false, true},
		{KindGauge, false, true, false},
		{KindTabs, true, false, false},
		{KindGrid, true, false, false},
		{KindSplit, true, false, false},
		{KindRepeat, true, false, false},
	}

	require.Len(t, cases, len(allKinds), "predicate table should cover allKinds")
	seen := make(map[Kind]int, len(cases))
	for _, tc := range cases {
		seen[tc.kind]++
	}
	for _, kind := range allKinds {
		require.Equalf(t, 1, seen[kind], "predicate table should include %q exactly once", kind)
	}

	for _, tc := range cases {
		t.Run(string(tc.kind), func(t *testing.T) {
			assert.Equal(t, tc.isContainer, tc.kind.IsContainer())
			assert.Equal(t, tc.isChart, tc.kind.IsChart())
			assert.Equal(t, tc.rendersNatively, tc.kind.RendersNatively())
		})
	}
}

// TestKindPredicatesPartition asserts the structural invariants the predicates
// rely on across the render/runtime code:
//   - every kind is exactly one of: container, chart, or native leaf
//   - containers are never charts or native leaves
//   - "renderable leaf" == IsChart() || RendersNatively() == !IsContainer()
func TestKindPredicatesPartition(t *testing.T) {
	for _, k := range allKinds {
		matches := 0
		if k.IsContainer() {
			matches++
		}
		if k.IsChart() {
			matches++
		}
		if k.RendersNatively() {
			matches++
		}
		assert.Equalf(t, 1, matches, "%q matched %d categories, want exactly 1 (container/chart/native)", k, matches)

		leaf := k.IsChart() || k.RendersNatively()
		assert.NotEqualf(t, leaf, k.IsContainer(), "%q: leaf=%v but IsContainer()=%v; leaf and container must be mutually exclusive and exhaustive", k, leaf, k.IsContainer())
	}
}
