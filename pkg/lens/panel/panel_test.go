package panel

import "testing"

// allKinds is the full closed set of panel kinds. Keep it in sync with the
// const block above; the predicate tests below assert exhaustive, mutually
// consistent membership so a newly added kind that is left out of a predicate
// is caught here rather than at a far-away switch site.
var allKinds = []Kind{
	KindStat,
	KindTimeSeries,
	KindBar,
	KindHorizontalBar,
	KindStackedBar,
	KindSegmentBar,
	KindPie,
	KindDonut,
	KindTable,
	KindGauge,
	KindTabs,
	KindGrid,
	KindSplit,
	KindRepeat,
}

func TestKindPredicates(t *testing.T) {
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
		{KindPie, false, true, false},
		{KindDonut, false, true, false},
		{KindTable, false, false, true},
		{KindGauge, false, true, false},
		{KindTabs, true, false, false},
		{KindGrid, true, false, false},
		{KindSplit, true, false, false},
		{KindRepeat, true, false, false},
	}

	if len(cases) != len(allKinds) {
		t.Fatalf("predicate table covers %d kinds, want %d (allKinds)", len(cases), len(allKinds))
	}

	for _, tc := range cases {
		if got := tc.kind.IsContainer(); got != tc.isContainer {
			t.Errorf("%q.IsContainer() = %v, want %v", tc.kind, got, tc.isContainer)
		}
		if got := tc.kind.IsChart(); got != tc.isChart {
			t.Errorf("%q.IsChart() = %v, want %v", tc.kind, got, tc.isChart)
		}
		if got := tc.kind.RendersNatively(); got != tc.rendersNatively {
			t.Errorf("%q.RendersNatively() = %v, want %v", tc.kind, got, tc.rendersNatively)
		}
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
		if matches != 1 {
			t.Errorf("%q matched %d categories, want exactly 1 (container/chart/native)", k, matches)
		}

		leaf := k.IsChart() || k.RendersNatively()
		if leaf == k.IsContainer() {
			t.Errorf("%q: leaf=%v but IsContainer()=%v; leaf and container must be mutually exclusive and exhaustive", k, leaf, k.IsContainer())
		}
	}
}
