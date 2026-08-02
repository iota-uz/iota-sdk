package react

import (
	"fmt"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
)

func skeletonSpanStyle(span int) templ.SafeCSS {
	if span < 1 || span > 12 {
		span = 12
	}
	return templ.SafeCSS(fmt.Sprintf("--lens-panel-span:%d", span))
}

func skeletonCardClass(kind panel.Kind) string {
	switch kind {
	case panel.KindStat:
		return "lens-skeleton-card lens-skeleton-card-stat"
	// A strip of metric cells is much taller than one stat card; reserving a
	// stat card for it left the whole first row short by ~150px.
	case panel.KindStatGroup:
		return "lens-skeleton-card lens-skeleton-card-metrics"
	case panel.KindSegmentBar, panel.KindCascade:
		return "lens-skeleton-card lens-skeleton-card-compact"
	case panel.KindTimeSeries, panel.KindBar, panel.KindHorizontalBar, panel.KindStackedBar,
		panel.KindPie, panel.KindDonut, panel.KindRadial, panel.KindTable, panel.KindGauge,
		panel.KindHistogram, panel.KindBoxPlot, panel.KindHeatmap, panel.KindMap,
		panel.KindTabs, panel.KindGrid, panel.KindSplit, panel.KindRepeat,
		panel.KindMetricFlow, panel.KindMetricHierarchy, panel.KindMetricRelationship:
		return "lens-skeleton-card lens-skeleton-card-plot"
	}
	return "lens-skeleton-card lens-skeleton-card-plot"
}
