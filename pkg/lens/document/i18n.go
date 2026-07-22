package document

import "sort"

// Runtime chrome translation keys. BuildOptions.I18n is an opaque map, so a
// producer typo silently falls back to the runtime's English default. These
// constants are the canonical, testable spelling of every key the React
// runtime looks up; RuntimeI18nKeys keeps them in sync with the TSX call
// sites.
const (
	I18nCascadeStages     = "cascade.stages"
	I18nChartDrillHint    = "chart.drillHint"
	I18nChartError        = "chart.error"
	I18nChartLabel        = "chart.label"
	I18nChartLegendLast   = "chart.legendLast"
	I18nChartLegendToggle = "chart.legendToggle"
	I18nDashboardEmpty    = "dashboard.empty"
	I18nDashboardTabs     = "dashboard.tabs"
	I18nDashboardUpdated  = "dashboard.updated"
	I18nDrillReset        = "drill.reset"
	I18nDrawerClose       = "drawer.close"
	I18nDrawerEyebrow     = "drawer.eyebrow"
	I18nDrawerLabel       = "drawer.label"
	I18nExploreBack       = "explore.back"
	I18nExploreBreakdown  = "explore.breakdown"
	I18nExploreChooseView = "explore.chooseView"
	I18nExploreClose      = "explore.close"
	I18nExploreExpand     = "explore.expandSegment"
	I18nExploreNoDetail   = "explore.noDetail"
	I18nExploreOpenBreak  = "explore.openBreakdown"
	I18nExplorePanel      = "explore.panel"
	I18nExplorePath       = "explore.path"
	I18nExplorePathLabel  = "explore.pathLabel"
	I18nExploreUnavail    = "explore.unavailable"
	I18nExploreViewAs     = "explore.viewSegmentAs"
	I18nExploreViews      = "explore.views"
	I18nExportDashboard   = "export.dashboard"
	I18nExportPanel       = "export.panel"
	I18nExportPending     = "export.pending"
	I18nExportRetry       = "export.retry"
	I18nExportRetryHint   = "export.retryHint"
	I18nPanelCollapse     = "panel.collapse"
	I18nPanelEmpty        = "panel.empty"
	I18nPanelExpand       = "panel.expand"
	I18nPanelMissing      = "panel.missing"
	I18nPanelOpenMetric   = "panel.openMetric"
	I18nPanelRetry        = "panel.retry"
	I18nPanelTotal        = "panel.total"
	I18nPanelUnsupported  = "panel.unsupported"
	I18nPanelUpdating     = "panel.updating"
	I18nRuntimeDismiss    = "runtime.dismissNotice"
	I18nRuntimeLoadError  = "runtime.loadError"
	I18nTableActions      = "table.actions"
	I18nTableEmptyPage    = "table.emptyPage"
	I18nTableLoadingPage  = "table.loadingPage"
	I18nTableNext         = "table.next"
	I18nTableOpenRecord   = "table.openRecord"
	I18nTablePage         = "table.page"
	I18nTablePages        = "table.pages"
	I18nTablePrevious     = "table.previous"
	I18nTableSortScope    = "table.sortScope"
	I18nSemanticsPrefix   = "explore.semantics."
	I18nSemanticsEvidence = I18nSemanticsPrefix + string(SemanticsEvidence)
	I18nSemanticsPartn    = I18nSemanticsPrefix + string(SemanticsPartition)
	I18nSemanticsRecon    = I18nSemanticsPrefix + string(SemanticsReconciliation)
	I18nSemanticsSeries   = I18nSemanticsPrefix + string(SemanticsSeries)
)

// RuntimeI18nKeys lists every translation key the runtime resolves, sorted.
// Producers can range over it to assert their catalogue is complete.
func RuntimeI18nKeys() []string {
	keys := []string{
		I18nCascadeStages,
		I18nChartDrillHint, I18nChartError, I18nChartLabel, I18nChartLegendLast, I18nChartLegendToggle,
		I18nDashboardEmpty, I18nDashboardTabs, I18nDashboardUpdated, I18nDrillReset,
		I18nDrawerClose, I18nDrawerEyebrow, I18nDrawerLabel,
		I18nExploreBack, I18nExploreBreakdown, I18nExploreChooseView, I18nExploreClose, I18nExploreExpand,
		I18nExploreNoDetail, I18nExploreOpenBreak,
		I18nExplorePanel, I18nExplorePath, I18nExplorePathLabel,
		I18nExploreUnavail, I18nExploreViewAs, I18nExploreViews,
		I18nExportDashboard, I18nExportPanel, I18nExportPending, I18nExportRetry, I18nExportRetryHint,
		I18nPanelCollapse, I18nPanelEmpty, I18nPanelExpand, I18nPanelMissing, I18nPanelOpenMetric,
		I18nPanelRetry, I18nPanelTotal, I18nPanelUnsupported, I18nPanelUpdating,
		I18nRuntimeDismiss, I18nRuntimeLoadError,
		I18nTableActions, I18nTableEmptyPage, I18nTableLoadingPage, I18nTableNext, I18nTableOpenRecord,
		I18nTablePage, I18nTablePages, I18nTablePrevious, I18nTableSortScope,
		I18nSemanticsEvidence, I18nSemanticsPartn, I18nSemanticsRecon, I18nSemanticsSeries,
	}
	sort.Strings(keys)
	return keys
}
