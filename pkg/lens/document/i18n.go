package document

import "sort"

// Runtime chrome translation keys. BuildOptions.I18n is an opaque map, so a
// producer typo silently falls back to the runtime's English default. These
// constants are the canonical, testable spelling of every key the React
// runtime looks up; RuntimeI18nKeys keeps them in sync with the TSX call
// sites.
const (
	I18nCascadeOpenStage  = "cascade.openStage"
	I18nCascadeStages     = "cascade.stages"
	I18nChartDrillHint    = "chart.drillHint"
	I18nChartError        = "chart.error"
	I18nChartLabel        = "chart.label"
	I18nChartLegendLast   = "chart.legendLast"
	I18nChartLegendToggle = "chart.legendToggle"
	I18nCalendarAnnRange  = "calendar.announceRange"
	I18nCalendarAnnStart  = "calendar.announceStart"
	I18nCalendarHintEnd   = "calendar.hintEnd"
	I18nCalendarHintStart = "calendar.hintStart"
	I18nCalendarLabel     = "calendar.label"
	I18nCalendarNextMonth = "calendar.nextMonth"
	I18nCalendarPrevMonth = "calendar.prevMonth"
	// The month/year jump panel behind the calendar heading: the trigger, and
	// the panel's own paging (one year, or one twelve-year block, at a time).
	I18nCalendarChooseMonth = "calendar.chooseMonth"
	I18nCalendarNextPage    = "calendar.nextPage"
	I18nCalendarPrevPage    = "calendar.prevPage"
	I18nDashboardEmpty      = "dashboard.empty"
	I18nDashboardTabs       = "dashboard.tabs"
	I18nDashboardUpdated    = "dashboard.updated"
	I18nDocumentRefetch     = "document.refetchFailed"
	I18nDocumentRetry       = "document.retry"
	I18nDrillReset          = "drill.reset"
	I18nDrawerClose         = "drawer.close"
	I18nDrawerEyebrow       = "drawer.eyebrow"
	I18nDrawerLabel         = "drawer.label"
	I18nExploreBack         = "explore.back"
	I18nExploreBreakdown    = "explore.breakdown"
	I18nExploreChooseView   = "explore.chooseView"
	I18nExploreClose        = "explore.close"
	I18nExploreCopied       = "explore.copied"
	I18nExploreCopyValue    = "explore.copyValue"
	I18nExploreExpand       = "explore.expandSegment"
	I18nExploreNoDetail     = "explore.noDetail"
	I18nExploreOpenBreak    = "explore.openBreakdown"
	I18nExplorePanel        = "explore.panel"
	I18nExplorePath         = "explore.path"
	I18nExplorePathLabel    = "explore.pathLabel"
	I18nExploreSegment      = "explore.segmentEyebrow"
	I18nExploreShareTotal   = "explore.shareOfTotal"
	I18nExploreUnavail      = "explore.unavailable"
	I18nExploreViewAs       = "explore.viewSegmentAs"
	I18nExploreViews        = "explore.views"
	I18nFocusBackToParent   = "focus.backToParent"
	I18nFocusMoreViews      = "focus.moreViews"
	I18nFocusPath           = "focus.path"
	I18nFocusSourceData     = "focus.sourceData"
	I18nFocusSourceLoad     = "focus.sourceLoading"
	I18nFocusSourceRows     = "focus.sourceRows"
	I18nFocusSourceUnav     = "focus.sourceUnavailable"
	I18nFocusToParent       = "focus.toParent"
	I18nFocusViewAs         = "focus.viewAs"
	I18nExportData          = "export.data"
	I18nExportDashboard     = "export.dashboard"
	I18nExportMenu          = "export.menu"
	I18nExportPanel         = "export.panel"
	I18nExportReport        = "export.report"
	I18nExportPending       = "export.pending"
	I18nExportRetry         = "export.retry"
	I18nExportRetryHint     = "export.retryHint"
	I18nFilterBarLabel      = "filter.bar.label"
	I18nFilterAllTime       = "filter.period.allTime"
	I18nFilterApply         = "filter.period.apply"
	I18nFilterCancel        = "filter.period.cancel"
	I18nFilterCompleted     = "filter.period.completed"
	I18nFilterCustom        = "filter.period.custom"
	I18nFilterDateFormat    = "filter.period.dateFormat"
	I18nFilterDayCount      = "filter.period.dayCount"
	I18nFilterFrom          = "filter.period.from"
	I18nFilterOpen          = "filter.period.open"
	I18nFilterQuickSelect   = "filter.period.quickSelect"
	I18nFilterTo            = "filter.period.to"

	// Relative period presets rendered by the runtime's built-in catalog
	// (web/lens/src/controls/model.ts defaultPeriodPresets), which mirrors the
	// legacy HTMX picker's DefaultQuickRanges: current month, 30 days,
	// 12 months, current fiscal year (yearToDate), then last month and last
	// fiscal year (lastYear).
	I18nFilterPresetThisMonth    = "filter.period.preset.thisMonth"
	I18nFilterPresetLast30Days   = "filter.period.preset.last30days"
	I18nFilterPresetLast12Months = "filter.period.preset.last12months"
	I18nFilterPresetYearToDate   = "filter.period.preset.yearToDate"
	I18nFilterPresetLastMonth    = "filter.period.preset.lastMonth"
	I18nFilterPresetLastYear     = "filter.period.preset.lastYear"
	I18nPanelCollapse            = "panel.collapse"
	I18nPanelEmpty               = "panel.empty"
	I18nPanelExpand              = "panel.expand"
	I18nPanelInfo                = "panel.info"
	I18nPanelMissing             = "panel.missing"
	I18nPanelOpenMetric          = "panel.openMetric"
	I18nPanelRetry               = "panel.retry"
	I18nPanelTotal               = "panel.total"
	I18nPanelUnsupported         = "panel.unsupported"
	I18nPanelUpdating            = "panel.updating"
	I18nPrintAppendix            = "print.appendix"
	I18nPrintAppendixNote        = "print.appendixNote"
	I18nPrintBreakdown           = "print.breakdown"
	I18nPrintCategory            = "print.category"
	I18nPrintContents            = "print.contents"
	I18nPrintDefCalculated       = "print.defCalculated"
	I18nPrintDefConfigRequired   = "print.defConfigRequired"
	I18nPrintDefEmptySource      = "print.defEmptySource"
	I18nPrintDefProxy            = "print.defProxy"
	I18nPrintDefReconciliation   = "print.defReconciliation"
	I18nPrintDefUnavailable      = "print.defUnavailable"
	I18nPrintDefVerified         = "print.defVerified"
	I18nPrintFailed              = "print.failed"
	I18nPrintFigure              = "print.figure"
	I18nPrintGenerated           = "print.generated"
	I18nPrintKicker              = "print.kicker"
	I18nPrintLimitations         = "print.limitations"
	I18nPrintLimitationsFlag     = "print.limitationsFlag"
	I18nPrintLimitationsNote     = "print.limitationsNote"
	I18nPrintMethod              = "print.method"
	I18nPrintMissing             = "print.missing"
	I18nPrintNoData              = "print.noData"
	I18nPrintNote                = "print.note"
	I18nPrintNoteProxy           = "print.noteProxy"
	I18nPrintNoteReconciliation  = "print.noteReconciliation"
	I18nPrintNoteUnavailable     = "print.noteUnavailable"
	I18nPrintOther               = "print.other"
	I18nPrintPending             = "print.pending"
	I18nPrintPeriod              = "print.period"
	I18nPrintQuality             = "print.quality"
	I18nPrintQualityCount        = "print.qualityCount"
	I18nPrintQualityTerms        = "print.qualityTerms"
	I18nPrintSections            = "print.sections"
	I18nPrintSeriesTail          = "print.seriesTail"
	I18nPrintShare               = "print.share"
	I18nPrintSnapshot            = "print.snapshot"
	I18nPrintSplitPart           = "print.splitPart"
	I18nPrintStage               = "print.stage"
	I18nPrintSummary             = "print.summary"
	I18nPrintTarget              = "print.target"
	I18nPrintTimeLimit           = "print.timeLimit"
	I18nPrintToneAdverse         = "print.toneAdverse"
	I18nPrintToneFavourable      = "print.toneFavourable"
	I18nPrintToneNeutral         = "print.toneNeutral"
	I18nPrintTotal               = "print.total"
	I18nPrintTruncated           = "print.truncated"
	I18nPrintTruncatedOf         = "print.truncatedOf"
	I18nPrintValue               = "print.value"
	I18nPrintView                = "print.view"
	// Sentences the printed report composes from a frame it has just printed:
	// the leading share, the largest deduction, the change across a series.
	// They carry placeholders so a catalogue keeps its own word order.
	I18nPrintFactPartition    = "print.factPartition"
	I18nPrintFactPartitionAll = "print.factPartitionAll"
	I18nPrintFactPair         = "print.factPair"
	I18nPrintFactProgress     = "print.factProgress"
	I18nPrintFactBridge       = "print.factBridge"
	I18nPrintFactBridgeBare   = "print.factBridgeBare"
	I18nPrintFactBridgeEnds   = "print.factBridgeEnds"
	I18nPrintFactSeries       = "print.factSeries"
	I18nPrintFactSeriesBare   = "print.factSeriesBare"
	I18nPrintFactTable        = "print.factTable"
	I18nPrintFactRows         = "print.factRows"
	I18nRuntimeDismiss        = "runtime.dismissNotice"
	I18nRuntimeLoadError      = "runtime.loadError"
	// Shown before a document exists, so the runtime falls back to its own
	// bundled wording; a host that translates them wins once the document lands.
	I18nRuntimeRetry          = "runtime.retry"
	I18nRuntimeSlowLoad       = "runtime.slowLoad"
	I18nTableActions          = "table.actions"
	I18nTableEmptyPage        = "table.emptyPage"
	I18nTableLoadingPage      = "table.loadingPage"
	I18nTableNext             = "table.next"
	I18nTableOpenRecord       = "table.openRecord"
	I18nTablePage             = "table.page"
	I18nTablePages            = "table.pages"
	I18nTablePrevious         = "table.previous"
	I18nTableRowCount         = "table.rowCount"
	I18nTableSortScope        = "table.sortScope"
	I18nAvailConfig           = "availability.config_required"
	I18nAvailEmptySource      = "availability.empty_source"
	I18nAvailUnavailable      = "availability.unavailable"
	I18nConfCalculated        = "confidence.calculated"
	I18nConfProxy             = "confidence.proxy"
	I18nConfRequiresRecon     = "confidence.requires_reconciliation"
	I18nConfVerified          = "confidence.verified"
	I18nFlowDifference        = "flow.difference"
	I18nFlowEquals            = "flow.equals"
	I18nFlowMinus             = "flow.minus"
	I18nFlowPlus              = "flow.plus"
	I18nFlowStages            = "flow.stages"
	I18nHierAllocated         = "hierarchy.allocated"
	I18nHierDifference        = "hierarchy.difference"
	I18nHierUnallocated       = "hierarchy.unallocated"
	I18nPanelDuplicateKey     = "panel.duplicateKey"
	I18nPanelMissingCol       = "panel.missingColumn"
	I18nRelAssociation        = "relationship.association"
	I18nRelDerivation         = "relationship.derivation"
	I18nRelReconciliation     = "relationship.reconciliation"
	I18nRelTypePrefix         = "relationship.type."
	I18nRelTypeAssociation    = I18nRelTypePrefix + string(MetricRelationshipAssociation)
	I18nRelTypeDerivation     = I18nRelTypePrefix + string(MetricRelationshipDerivation)
	I18nRelTypeReconciliation = I18nRelTypePrefix + string(MetricRelationshipReconciliation)
	I18nSemanticsPrefix       = "explore.semantics."
	I18nSemanticsEvidence     = I18nSemanticsPrefix + string(SemanticsEvidence)
	I18nSemanticsPartn        = I18nSemanticsPrefix + string(SemanticsPartition)
	I18nSemanticsRecon        = I18nSemanticsPrefix + string(SemanticsReconciliation)
	I18nSemanticsSeries       = I18nSemanticsPrefix + string(SemanticsSeries)
)

// RuntimeI18nKeys lists every translation key the runtime resolves, sorted.
// Producers can range over it to assert their catalogue is complete.
func RuntimeI18nKeys() []string {
	keys := []string{
		I18nCalendarAnnRange, I18nCalendarAnnStart, I18nCalendarHintEnd, I18nCalendarHintStart,
		I18nCalendarLabel, I18nCalendarNextMonth, I18nCalendarPrevMonth,
		I18nCalendarChooseMonth, I18nCalendarNextPage, I18nCalendarPrevPage,
		I18nCascadeOpenStage, I18nCascadeStages,
		I18nFilterBarLabel, I18nFilterAllTime, I18nFilterApply, I18nFilterCancel,
		I18nFilterCompleted, I18nFilterCustom,
		I18nFilterDateFormat, I18nFilterDayCount,
		I18nFilterFrom, I18nFilterOpen, I18nFilterQuickSelect, I18nFilterTo,
		I18nFilterPresetThisMonth, I18nFilterPresetLast30Days, I18nFilterPresetLast12Months,
		I18nFilterPresetYearToDate, I18nFilterPresetLastMonth, I18nFilterPresetLastYear,
		I18nChartDrillHint, I18nChartError, I18nChartLabel, I18nChartLegendLast, I18nChartLegendToggle,
		I18nDashboardEmpty, I18nDashboardTabs, I18nDashboardUpdated, I18nDrillReset,
		I18nDocumentRefetch, I18nDocumentRetry,
		I18nDrawerClose, I18nDrawerEyebrow, I18nDrawerLabel,
		I18nExploreBack, I18nExploreBreakdown, I18nExploreChooseView, I18nExploreClose,
		I18nExploreCopied, I18nExploreCopyValue, I18nExploreExpand,
		I18nExploreNoDetail, I18nExploreOpenBreak,
		I18nExplorePanel, I18nExplorePath, I18nExplorePathLabel,
		I18nExploreSegment, I18nExploreShareTotal,
		I18nExploreUnavail, I18nExploreViewAs, I18nExploreViews,
		I18nFocusBackToParent, I18nFocusMoreViews, I18nFocusPath,
		I18nFocusSourceData, I18nFocusSourceLoad, I18nFocusSourceRows, I18nFocusSourceUnav,
		I18nFocusToParent, I18nFocusViewAs,
		I18nExportDashboard, I18nExportData, I18nExportMenu, I18nExportPanel, I18nExportReport,
		I18nExportPending, I18nExportRetry, I18nExportRetryHint,
		I18nPanelCollapse, I18nPanelEmpty, I18nPanelExpand, I18nPanelInfo, I18nPanelMissing, I18nPanelOpenMetric,
		I18nPanelRetry, I18nPanelTotal, I18nPanelUnsupported, I18nPanelUpdating,
		I18nPrintAppendix, I18nPrintAppendixNote, I18nPrintBreakdown, I18nPrintCategory,
		I18nPrintContents, I18nPrintFailed, I18nPrintFigure,
		I18nPrintDefCalculated, I18nPrintDefConfigRequired, I18nPrintDefEmptySource,
		I18nPrintDefProxy, I18nPrintDefReconciliation, I18nPrintDefUnavailable, I18nPrintDefVerified,
		I18nPrintGenerated, I18nPrintKicker, I18nPrintLimitations, I18nPrintLimitationsFlag,
		I18nPrintLimitationsNote, I18nPrintMethod, I18nPrintMissing,
		I18nPrintNoData, I18nPrintNote, I18nPrintNoteProxy, I18nPrintNoteReconciliation,
		I18nPrintNoteUnavailable, I18nPrintOther, I18nPrintPending, I18nPrintPeriod,
		I18nPrintQuality, I18nPrintQualityCount, I18nPrintQualityTerms,
		I18nPrintSections, I18nPrintSeriesTail, I18nPrintShare, I18nPrintSnapshot, I18nPrintSummary,
		I18nPrintTarget, I18nPrintTimeLimit, I18nPrintTotal, I18nPrintTruncated, I18nPrintTruncatedOf,
		I18nPrintSplitPart, I18nPrintStage,
		I18nPrintToneAdverse, I18nPrintToneFavourable, I18nPrintToneNeutral,
		I18nPrintValue, I18nPrintView,
		I18nPrintFactPartition, I18nPrintFactPartitionAll, I18nPrintFactPair, I18nPrintFactProgress,
		I18nPrintFactBridge, I18nPrintFactBridgeBare, I18nPrintFactBridgeEnds,
		I18nPrintFactSeries, I18nPrintFactSeriesBare, I18nPrintFactTable, I18nPrintFactRows,
		I18nRuntimeDismiss, I18nRuntimeLoadError, I18nRuntimeRetry, I18nRuntimeSlowLoad,
		I18nTableActions, I18nTableEmptyPage, I18nTableLoadingPage, I18nTableNext, I18nTableOpenRecord,
		I18nTablePage, I18nTablePages, I18nTablePrevious, I18nTableRowCount, I18nTableSortScope,
		I18nSemanticsEvidence, I18nSemanticsPartn, I18nSemanticsRecon, I18nSemanticsSeries,
		I18nAvailConfig, I18nAvailEmptySource, I18nAvailUnavailable,
		I18nConfCalculated, I18nConfProxy, I18nConfRequiresRecon, I18nConfVerified,
		I18nFlowDifference, I18nFlowEquals, I18nFlowMinus, I18nFlowPlus, I18nFlowStages,
		I18nHierAllocated, I18nHierDifference, I18nHierUnallocated,
		I18nPanelDuplicateKey, I18nPanelMissingCol,
		I18nRelAssociation, I18nRelDerivation, I18nRelReconciliation,
		I18nRelTypeAssociation, I18nRelTypeDerivation, I18nRelTypeReconciliation,
	}
	sort.Strings(keys)
	return keys
}
