package document

import "sort"

// Runtime chrome translation keys. BuildOptions.I18n is an opaque map, so a
// producer typo silently falls back to the runtime's English default. These
// constants are the canonical, testable spelling of every key the React
// runtime looks up; RuntimeI18nKeys keeps them in sync with the TSX call
// sites.
const (
	I18nCascadeOpenStage              = "cascade.openStage"
	I18nCascadeStages                 = "cascade.stages"
	I18nChartBoxplotMin               = "chart.boxplot.min"
	I18nChartBoxplotQ1                = "chart.boxplot.q1"
	I18nChartBoxplotMedian            = "chart.boxplot.median"
	I18nChartBoxplotQ3                = "chart.boxplot.q3"
	I18nChartBoxplotMax               = "chart.boxplot.max"
	I18nChartCollapseOther            = "chart.collapseOther"
	I18nChartDrillHint                = "chart.drillHint"
	I18nChartError                    = "chart.error"
	I18nChartGaugeRange               = "chart.gaugeRange"
	I18nChartGaugeValue               = "chart.gaugeValue"
	I18nChartKeyboardActs             = "chart.keyboardActions"
	I18nChartLabel                    = "chart.label"
	I18nChartLegendControl            = "chart.legendControls"
	I18nChartLegendHideAll            = "chart.legendHideAll"
	I18nChartLegendInvert             = "chart.legendInvert"
	I18nChartLegendIsolate            = "chart.legendIsolate"
	I18nChartLegendIsolateName        = "chart.legendIsolateName"
	I18nChartLegendSearch             = "chart.legendSearch"
	I18nChartLegendShowAll            = "chart.legendShowAll"
	I18nChartLegendToggle             = "chart.legendToggle"
	I18nChartLogScale                 = "chart.logScale"
	I18nChartLogScaleHint             = "chart.logScaleHint"
	I18nChartMovingAverage            = "chart.movingAverage"
	I18nChartOther                    = "chart.other"
	I18nChartOpenMark                 = "chart.openMark"
	I18nChartRegression               = "chart.regression"
	I18nChartResetZoom                = "chart.resetZoom"
	I18nChartSeriesPrevious           = "chart.series.previous"
	I18nChartSeriesTrend              = "chart.series.trend"
	I18nChartSeriesMovingAverage      = "chart.series.movingAverage"
	I18nChartSeriesEstimate           = "chart.series.estimate"
	I18nChartSeriesYTD                = "chart.series.ytd"
	I18nChartSeriesForecast           = "chart.series.forecast"
	I18nChartSeriesForecastLower      = "chart.series.forecastLower"
	I18nChartSeriesForecastConfidence = "chart.series.forecastConfidence"
	I18nCalendarAnnRange              = "calendar.announceRange"
	I18nCalendarAnnStart              = "calendar.announceStart"
	I18nCalendarHintEnd               = "calendar.hintEnd"
	I18nCalendarHintStart             = "calendar.hintStart"
	I18nCalendarLabel                 = "calendar.label"
	I18nCalendarNextMonth             = "calendar.nextMonth"
	I18nCalendarPrevMonth             = "calendar.prevMonth"
	// The month/year jump panel behind the calendar heading: the trigger, and
	// the panel's own paging (one year, or one twelve-year block, at a time).
	I18nCalendarChooseMonth   = "calendar.chooseMonth"
	I18nCalendarNextPage      = "calendar.nextPage"
	I18nCalendarPrevPage      = "calendar.prevPage"
	I18nDashboardEmpty        = "dashboard.empty"
	I18nDashboardTabs         = "dashboard.tabs"
	I18nDashboardUpdated      = "dashboard.updated"
	I18nDashboardRecompute    = "dashboard.recompute"
	I18nDashboardRecomputing  = "dashboard.recomputing"
	I18nDocumentRefetch       = "document.refetchFailed"
	I18nDocumentRetry         = "document.retry"
	I18nDrillReset            = "drill.reset"
	I18nDrawerClose           = "drawer.close"
	I18nDrawerEyebrow         = "drawer.eyebrow"
	I18nDrawerLabel           = "drawer.label"
	I18nExploreBack           = "explore.back"
	I18nExploreBreakdown      = "explore.breakdown"
	I18nExploreChooseView     = "explore.chooseView"
	I18nExploreClose          = "explore.close"
	I18nExploreCopied         = "explore.copied"
	I18nExploreCopyValue      = "explore.copyValue"
	I18nExploreExpand         = "explore.expandSegment"
	I18nExploreNoDetail       = "explore.noDetail"
	I18nExploreOpenBreak      = "explore.openBreakdown"
	I18nExplorePanel          = "explore.panel"
	I18nExplorePath           = "explore.path"
	I18nExplorePathLabel      = "explore.pathLabel"
	I18nExploreSegment        = "explore.segmentEyebrow"
	I18nExploreShareTotal     = "explore.shareOfTotal"
	I18nExploreUnavail        = "explore.unavailable"
	I18nExploreViewAs         = "explore.viewSegmentAs"
	I18nExploreViews          = "explore.views"
	I18nFocusBackToParent     = "focus.backToParent"
	I18nFocusMoreViews        = "focus.moreViews"
	I18nFocusPath             = "focus.path"
	I18nFocusSourceData       = "focus.sourceData"
	I18nFocusSourceLoad       = "focus.sourceLoading"
	I18nFocusSourceRows       = "focus.sourceRows"
	I18nFocusSourceUnav       = "focus.sourceUnavailable"
	I18nFocusToParent         = "focus.toParent"
	I18nFocusViewAs           = "focus.viewAs"
	I18nExportData            = "export.data"
	I18nExportDashboard       = "export.dashboard"
	I18nExportMenu            = "export.menu"
	I18nExportPNG             = "export.png"
	I18nExportPanel           = "export.panel"
	I18nExportReport          = "export.report"
	I18nExportPending         = "export.pending"
	I18nExportRetry           = "export.retry"
	I18nExportRetryHint       = "export.retryHint"
	I18nExportSVG             = "export.svg"
	I18nExportImageError      = "export.imageError"
	I18nFilterBarLabel        = "filter.bar.label"
	I18nFilterFacetApply      = "filter.facet.apply"
	I18nFilterFacetClear      = "filter.facet.clear"
	I18nFilterFacetClearAll   = "filter.facet.clearAll"
	I18nFilterFacetEmpty      = "filter.facet.empty"
	I18nFilterFacetError      = "filter.facet.error"
	I18nFilterFacetLoading    = "filter.facet.loading"
	I18nFilterFacetRemove     = "filter.facet.remove"
	I18nFilterFacetSearch     = "filter.facet.search"
	I18nFilterCompareOff      = "filter.compare.off"
	I18nFilterComparePrevious = "filter.compare.previous"
	I18nFilterCompareYearAgo  = "filter.compare.yearAgo"
	I18nFilterCompareCustom   = "filter.compare.custom"
	I18nFilterCompareStart    = "filter.compare.start"
	I18nFilterCompareEnd      = "filter.compare.end"
	I18nFilterAllTime         = "filter.period.allTime"
	I18nFilterApply           = "filter.period.apply"
	I18nFilterCancel          = "filter.period.cancel"
	I18nFilterCompleted       = "filter.period.completed"
	I18nFilterCustom          = "filter.period.custom"
	I18nFilterDateFormat      = "filter.period.dateFormat"
	I18nFilterDayCount        = "filter.period.dayCount"
	I18nFilterFrom            = "filter.period.from"
	I18nFilterOpen            = "filter.period.open"
	I18nFilterQuickSelect     = "filter.period.quickSelect"
	I18nFilterTo              = "filter.period.to"

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
	I18nPanelError               = "panel.error"
	I18nPanelEmpty               = "panel.empty"
	I18nPanelExpand              = "panel.expand"
	I18nPanelInfo                = "panel.info"
	I18nPanelMissing             = "panel.missing"
	I18nPanelOpenMetric          = "panel.openMetric"
	I18nPanelRetry               = "panel.retry"
	I18nPanelTotal               = "panel.total"
	I18nPanelUnsupported         = "panel.unsupported"
	I18nPanelUpdating            = "panel.updating"
	I18nPanelCalculation         = "panel.calculation"
	I18nPanelCacheHit            = "panel.cacheHit"
	I18nPanelCacheMiss           = "panel.cacheMiss"
	I18nPanelTrendNew            = "panel.trend.new"
	I18nPanelTrendNotAvailable   = "panel.trend.notAvailable"
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
	I18nRuntimeRetry             = "runtime.retry"
	I18nRuntimeSlowLoad          = "runtime.slowLoad"
	I18nTableActions             = "table.actions"
	I18nTableEmptyPage           = "table.emptyPage"
	I18nTableLoadingPage         = "table.loadingPage"
	I18nTableNext                = "table.next"
	I18nTableOpenRecord          = "table.openRecord"
	I18nTablePage                = "table.page"
	I18nTablePages               = "table.pages"
	I18nTablePrevious            = "table.previous"
	I18nTableRowCount            = "table.rowCount"
	I18nTableSearch              = "table.search"
	I18nTableSearchPlaceholder   = "table.searchPlaceholder"
	I18nTableSmallSample         = "table.smallSample"
	I18nTableSmallSampleShort    = "table.smallSampleShort"
	I18nTableSortScope           = "table.sortScope"
	I18nTableTotal               = "table.total"
	I18nTableBooleanYes          = "table.boolean.yes"
	I18nTableBooleanNo           = "table.boolean.no"
	I18nShareCopied              = "share.copied"
	I18nShareCopy                = "share.copy"
	I18nShareError               = "share.error"
	I18nViewsEmpty               = "views.empty"
	I18nViewsTeam                = "views.team"
	I18nViewsPersonal            = "views.personal"
	I18nViewsDelete              = "views.delete"
	I18nViewsSaveCurrent         = "views.saveCurrent"
	I18nViewsName                = "views.name"
	I18nViewsScope               = "views.scope"
	I18nViewsDefaultRole         = "views.defaultRole"
	I18nViewsNoDefaultRole       = "views.noDefaultRole"
	I18nViewsSave                = "views.save"
	I18nViewsSchedule            = "views.schedule"
	I18nViewsRecipients          = "views.recipients"
	I18nViewsRecipientsHint      = "views.recipientsHint"
	I18nViewsCron                = "views.cron"
	I18nViewsScheduleSave        = "views.scheduleSave"
	I18nViewsLoading             = "views.loading"
	I18nViewsConfirmDelete       = "views.confirmDelete"
	I18nViewsConfirmDeleteSched  = "views.confirmDeleteSchedule"
	I18nViewsNextRun             = "views.nextRun"
	I18nViewsLoadError           = "views.loadError"
	I18nViewsSaveError           = "views.saveError"
	I18nViewsDeleteError         = "views.deleteError"
	I18nViewsScheduleSaveError   = "views.scheduleSaveError"
	I18nViewsScheduleDeleteError = "views.scheduleDeleteError"
	I18nViewsTriggerTitle        = "views.triggerTitle"
	I18nViewsDialogTitle         = "views.dialogTitle"
	I18nViewsSavedList           = "views.savedList"
	I18nViewsSavedView           = "views.savedView"
	I18nAvailConfig              = "availability.config_required"
	I18nAvailEmptySource         = "availability.empty_source"
	I18nAvailUnavailable         = "availability.unavailable"
	I18nConfCalculated           = "confidence.calculated"
	I18nConfProxy                = "confidence.proxy"
	I18nConfRequiresRecon        = "confidence.requires_reconciliation"
	I18nConfVerified             = "confidence.verified"
	I18nFlowDifference           = "flow.difference"
	I18nFlowEquals               = "flow.equals"
	I18nFlowMinus                = "flow.minus"
	I18nFlowPlus                 = "flow.plus"
	I18nFlowStages               = "flow.stages"
	I18nHierAllocated            = "hierarchy.allocated"
	I18nHierDifference           = "hierarchy.difference"
	I18nHierUnallocated          = "hierarchy.unallocated"
	I18nPanelDuplicateKey        = "panel.duplicateKey"
	I18nPanelTrendComparison     = "panel.trend.comparison"
	I18nPanelMissingCol          = "panel.missingColumn"
	I18nRelAssociation           = "relationship.association"
	I18nRelDerivation            = "relationship.derivation"
	I18nRelReconciliation        = "relationship.reconciliation"
	I18nRelTypePrefix            = "relationship.type."
	I18nRelTypeAssociation       = I18nRelTypePrefix + string(MetricRelationshipAssociation)
	I18nRelTypeDerivation        = I18nRelTypePrefix + string(MetricRelationshipDerivation)
	I18nRelTypeReconciliation    = I18nRelTypePrefix + string(MetricRelationshipReconciliation)
	I18nSemanticsPrefix          = "explore.semantics."
	I18nSemanticsEvidence        = I18nSemanticsPrefix + string(SemanticsEvidence)
	I18nSemanticsPartn           = I18nSemanticsPrefix + string(SemanticsPartition)
	I18nSemanticsRecon           = I18nSemanticsPrefix + string(SemanticsReconciliation)
	I18nSemanticsSeries          = I18nSemanticsPrefix + string(SemanticsSeries)
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
		I18nFilterFacetApply, I18nFilterFacetClear, I18nFilterFacetClearAll, I18nFilterFacetEmpty, I18nFilterFacetError,
		I18nFilterFacetLoading, I18nFilterFacetRemove, I18nFilterFacetSearch,
		I18nFilterCompareOff, I18nFilterComparePrevious, I18nFilterCompareYearAgo,
		I18nFilterCompareCustom, I18nFilterCompareStart, I18nFilterCompareEnd,
		I18nFilterCompleted, I18nFilterCustom,
		I18nFilterDateFormat, I18nFilterDayCount,
		I18nFilterFrom, I18nFilterOpen, I18nFilterQuickSelect, I18nFilterTo,
		I18nFilterPresetThisMonth, I18nFilterPresetLast30Days, I18nFilterPresetLast12Months,
		I18nFilterPresetYearToDate, I18nFilterPresetLastMonth, I18nFilterPresetLastYear,
		I18nChartBoxplotMin, I18nChartBoxplotQ1, I18nChartBoxplotMedian, I18nChartBoxplotQ3, I18nChartBoxplotMax,
		I18nChartCollapseOther, I18nChartDrillHint, I18nChartError, I18nChartGaugeRange, I18nChartGaugeValue, I18nChartKeyboardActs, I18nChartLabel,
		I18nChartLegendControl, I18nChartLegendHideAll, I18nChartLegendInvert, I18nChartLegendIsolate, I18nChartLegendIsolateName, I18nChartLegendSearch,
		I18nChartLegendShowAll, I18nChartLegendToggle, I18nChartLogScale, I18nChartLogScaleHint, I18nChartMovingAverage,
		I18nChartOpenMark, I18nChartOther, I18nChartRegression, I18nChartResetZoom,
		I18nChartSeriesPrevious, I18nChartSeriesTrend, I18nChartSeriesMovingAverage, I18nChartSeriesEstimate,
		I18nChartSeriesYTD, I18nChartSeriesForecast, I18nChartSeriesForecastLower, I18nChartSeriesForecastConfidence,
		I18nDashboardEmpty, I18nDashboardTabs, I18nDashboardUpdated, I18nDashboardRecompute, I18nDashboardRecomputing, I18nDrillReset,
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
		I18nExportPending, I18nExportRetry, I18nExportRetryHint, I18nExportPNG, I18nExportSVG, I18nExportImageError,
		I18nPanelCollapse, I18nPanelEmpty, I18nPanelError, I18nPanelExpand, I18nPanelInfo, I18nPanelMissing, I18nPanelOpenMetric,
		I18nPanelRetry, I18nPanelTotal, I18nPanelUnsupported, I18nPanelUpdating,
		I18nPanelCalculation, I18nPanelCacheHit, I18nPanelCacheMiss, I18nPanelTrendNew, I18nPanelTrendNotAvailable,
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
		I18nTablePage, I18nTablePages, I18nTablePrevious, I18nTableRowCount, I18nTableSearch, I18nTableSearchPlaceholder,
		I18nTableSmallSample, I18nTableSmallSampleShort, I18nTableSortScope, I18nTableTotal, I18nTableBooleanYes, I18nTableBooleanNo,
		I18nShareCopied, I18nShareCopy, I18nShareError,
		I18nViewsEmpty, I18nViewsTeam, I18nViewsPersonal,
		I18nViewsDelete, I18nViewsSaveCurrent, I18nViewsName, I18nViewsScope, I18nViewsDefaultRole, I18nViewsNoDefaultRole,
		I18nViewsSave, I18nViewsSchedule, I18nViewsRecipients, I18nViewsRecipientsHint,
		I18nViewsCron, I18nViewsScheduleSave, I18nViewsLoading,
		I18nViewsConfirmDelete, I18nViewsConfirmDeleteSched, I18nViewsNextRun,
		I18nViewsLoadError, I18nViewsSaveError, I18nViewsDeleteError,
		I18nViewsScheduleSaveError, I18nViewsScheduleDeleteError,
		I18nViewsTriggerTitle, I18nViewsDialogTitle, I18nViewsSavedList, I18nViewsSavedView,
		I18nSemanticsEvidence, I18nSemanticsPartn, I18nSemanticsRecon, I18nSemanticsSeries,
		I18nAvailConfig, I18nAvailEmptySource, I18nAvailUnavailable,
		I18nConfCalculated, I18nConfProxy, I18nConfRequiresRecon, I18nConfVerified,
		I18nFlowDifference, I18nFlowEquals, I18nFlowMinus, I18nFlowPlus, I18nFlowStages,
		I18nHierAllocated, I18nHierDifference, I18nHierUnallocated,
		I18nPanelDuplicateKey, I18nPanelMissingCol, I18nPanelTrendComparison,
		I18nRelAssociation, I18nRelDerivation, I18nRelReconciliation,
		I18nRelTypeAssociation, I18nRelTypeDerivation, I18nRelTypeReconciliation,
	}
	sort.Strings(keys)
	return keys
}
