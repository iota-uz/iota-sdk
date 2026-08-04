import { Fragment, memo, useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import type { Frame, NodeKey, Panel } from '../contract'
import { radialNodeKey, type ChartActivation, type ChartAdapter, type ChartAnchor, type ChartFormatResolver, type ChartInput, type ChartKind } from '../charts/adapter'
import { fallbackMarkKey } from '../charts/keys'
import {
  categoryDisplayFormatter, chartOverlays, overlayId, overlayToneProperty,
  type ChartOverlay,
} from '../charts/overlays'
import { remainderColor } from '../charts/palette'
import { distributeShares, formatShare } from '../charts/shares'
import { shouldUseLogarithmicScale } from '../charts/scales'
import { recordForRow, resolveSourceValue } from '../explore/actions'
import { childForSelection } from '../explore/model'
import { ArrowsLeftRight, Eye, EyeSlash, WarningTriangle } from '../icons'
import { searchableListEntries } from '../listSearch'
import { axisUnit, cubeFilterParam, formatFieldValueAtReference, levelForPath, useAxisFormat, useDashboard, useDrill, useFilters, useFormat, usePanelFrame, useTranslate } from '../runtime'
import { formatFieldValueExact, rawValueText } from '../runtime/format'
import { hiddenSeriesFromURL, hiddenSeriesToURL, temporalStateFromURL, temporalStateToURL } from '../runtime/url'
import { usePanelNavigation } from './actions'
import { ChartDataEquivalent } from './ChartDataEquivalent'
import { ChartHost } from './ChartHost'
import { useMarkSelection } from './context'
import { colorLabels, encodingRoles, rowColorResolver, seriesColorResolver } from './data'
import { PanelFrame } from './PanelFrame'

// Below 2% a donut label cannot fit reliably inside its arc.
const minorDonutShare = 0.02
/**
 * One threshold for the legend column's whole control header.
 *
 * It used to be two: the toolbar rendered always and the search appeared at
 * nine (then six) entries, so the column's composition changed twice for
 * reasons nothing on screen explained. A legend long enough to need typing is
 * the same legend that is long enough to need bulk selection; below it a reader
 * clicks the rows, which are right there.
 */
const legendControlEntries = searchableListEntries
/** Four or fewer entries are quicker to manage individually. */
const legendBulkThreshold = 4

export interface ChartPanelProps {
  panel: Panel
  adapter?: ChartAdapter
}

function useChartFormat(panel: Panel): { format: ChartFormatResolver; formatAxis: ChartFormatResolver } {
  const fallback = useFormat()
  const label = useFormat(panel.encoding.label ? panel.format[panel.encoding.label] : undefined)
  const value = useFormat(panel.encoding.value ? panel.format[panel.encoding.value] : undefined)
  const previous = useFormat(panel.encoding.previous ? panel.format[panel.encoding.previous] : undefined)
  const lower = useFormat(panel.encoding.lower ? panel.format[panel.encoding.lower] : undefined)
  const q1 = useFormat(panel.encoding.q1 ? panel.format[panel.encoding.q1] : undefined)
  const median = useFormat(panel.encoding.median ? panel.format[panel.encoding.median] : undefined)
  const q3 = useFormat(panel.encoding.q3 ? panel.format[panel.encoding.q3] : undefined)
  const upper = useFormat(panel.encoding.upper ? panel.format[panel.encoding.upper] : undefined)
  const id = useFormat(panel.encoding.id ? panel.format[panel.encoding.id] : undefined)
  const series = useFormat(panel.encoding.series ? panel.format[panel.encoding.series] : undefined)
  const category = useFormat(panel.encoding.category ? panel.format[panel.encoding.category] : undefined)
  const cut = useFormat(panel.encoding.cut ? panel.format[panel.encoding.cut] : undefined)
  const cutLabel = useFormat(panel.encoding.cutLabel ? panel.format[panel.encoding.cutLabel] : undefined)
  const final = useFormat(panel.encoding.final ? panel.format[panel.encoding.final] : undefined)
  // Compact axis labels for the value field prevent overlapping full-precision money ticks.
  const valueAxis = useAxisFormat(panel.encoding.value ? panel.format[panel.encoding.value] : undefined)
  const distributionAxis = useAxisFormat(panel.encoding.median ? panel.format[panel.encoding.median] : undefined)

  const format = useMemo<ChartFormatResolver>(() => {
    const formatters = { label, value, previous, lower, q1, median, q3, upper, id, series, category, cut, cutLabel, final }
    const byField = new Map<string, (input: unknown) => string>()
    for (const role of encodingRoles) {
      const field = panel.encoding[role]
      // The metric-panel roles (share/confidence/availability) have no chart
      // formatter; a chart never carries them, but the guard keeps the shared
      // encodingRoles list usable here.
      const formatter = formatters[role as keyof typeof formatters]
      if (field && formatter) byField.set(field, formatter)
    }
    return (field: string, input: unknown) => (byField.get(field) ?? fallback)(input)
  }, [category, cut, cutLabel, fallback, final, id, label, lower, median, panel.encoding, previous, q1, q3, series, upper, value])

  const formatAxis = useMemo<ChartFormatResolver>(() => {
    const valueField = panel.encoding.value
    const medianField = panel.encoding.median
    return (field: string, input: unknown) => valueField && field === valueField
      ? valueAxis(input)
      : medianField && field === medianField ? distributionAxis(input) : format(field, input)
  }, [distributionAxis, format, panel.encoding.median, panel.encoding.value, valueAxis])

  return useMemo(() => ({ format, formatAxis }), [format, formatAxis])
}

/**
 * Identity of one legend entry. The id field is stable across re-queries;
 * without one the label is the only thing a row can be recognised by.
 */
export function legendKey(frame: Frame, panel: Panel, index: number): string {
  const idIndex = panel.encoding.id ? frame.columns.findIndex((column) => column.name === panel.encoding.id) : -1
  // Axis marks are keyed from `category` first (see the ECharts adapter).
  // Some documents retain a legacy `label` role even though that column is
  // absent from the served frame, so resolving the click must use the same
  // category-first rule that created the mark key.
  const labelField = panel.encoding.category ?? panel.encoding.label
  const labelIndex = frame.columns.findIndex((column) => column.name === labelField)
  const raw = idIndex >= 0 ? frame.rows[index]?.[idIndex] : frame.rows[index]?.[labelIndex]
  return typeof raw === 'string' || typeof raw === 'number' || typeof raw === 'bigint' ? String(raw) : String(index)
}

/** Index of the frame row a chart mark key identifies. */
export function rowIndexForKey(frame: Frame, panel: Panel, key: string): number {
  if (panel.radial?.mode === 'partition' && panel.encoding.series) {
    const seriesIndex = frame.columns.findIndex((column) => column.name === panel.encoding.series)
    if (seriesIndex >= 0) {
      const index = frame.rows.findIndex((row, position) => {
        const rawSeries = row[seriesIndex]
        const series = typeof rawSeries === 'string' || typeof rawSeries === 'number' || typeof rawSeries === 'bigint'
          ? String(rawSeries)
          : ''
        return radialNodeKey(series, legendKey(frame, panel, position)) === key
      })
      if (index >= 0) return index
    }
  }
  const index = frame.rows.findIndex((_, position) => legendKey(frame, panel, position) === key)
  if (index >= 0) return index
  const labelField = panel.encoding.category ?? panel.encoding.label
  const labelIndex = frame.columns.findIndex((column) => column.name === labelField)
  const seriesIndex = frame.columns.findIndex((column) => column.name === panel.encoding.series)
  if (labelIndex < 0) return -1
  return frame.rows.findIndex((row) => fallbackMarkKey(
    textCell(row[labelIndex]),
    seriesIndex >= 0 ? textCell(row[seriesIndex]) : '',
  ) === key)
}

function legendSeriesIndex(frame: Frame, panel: Panel): number {
  if (panel.semantics !== 'series' || !panel.encoding.series) return -1
  return frame.columns.findIndex((column) => column.name === panel.encoding.series)
}

/**
 * Ordinal of each series name in the panel's *own* frame, in plot order.
 *
 * A colour pinned to a panel is pinned by position (`panelId:index`), and the
 * chart derives that position from the frame it is drawing. Hiding a legend
 * entry drops its rows, so every series after it slides one position down and
 * repaints itself in the hidden series' colour — while the legend, which is
 * built over the full frame, keeps printing the old one. Resolving against the
 * full frame's order pins a series to its colour for as long as the panel is
 * showing that frame, however few of its series are on screen.
 */
function seriesOrder(frame: Frame | undefined, panel: Panel): Map<string, number> {
  const order = new Map<string, number>()
  if (!frame) return order
  const seriesIndex = frame.columns.findIndex((column) => column.name === panel.encoding.series)
  if (seriesIndex < 0) return order
  for (const row of frame.rows) {
    const raw = row[seriesIndex]
    if (typeof raw !== 'string' && typeof raw !== 'number' && typeof raw !== 'bigint') continue
    const name = String(raw)
    if (!order.has(name)) order.set(name, order.size)
  }
  return order
}

function legendEntryKey(frame: Frame, panel: Panel, index: number): string {
  const seriesIndex = legendSeriesIndex(frame, panel)
  if (seriesIndex < 0) return legendKey(frame, panel, index)
  const raw = frame.rows[index]?.[seriesIndex]
  return typeof raw === 'string' || typeof raw === 'number' || typeof raw === 'bigint' ? String(raw) : String(index)
}

function numericCell(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return undefined
}

function textCell(value: unknown): string {
  return typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint'
    ? String(value)
    : ''
}

function distinctCategoryCount(frame: Frame, panel: Panel): number {
  const partition = panel.kind === 'pie' || panel.kind === 'donut' || panel.kind === 'radial'
  const fields = partition
    ? [panel.encoding.label, panel.encoding.category]
    : [panel.encoding.category, panel.encoding.label]
  const index = fields.reduce((found, field) => (
    found >= 0 || !field ? found : frame.columns.findIndex((column) => column.name === field)
  ), -1)
  if (index < 0) return 0
  return new Set(frame.rows.map((row) => row[index]).filter((value) => value !== null && value !== undefined).map(String)).size
}

function distinctSeriesCount(frame: Frame, panel: Panel): number {
  const index = frame.columns.findIndex((column) => column.name === panel.encoding.series)
  if (index < 0) return 1
  return new Set(frame.rows.map((row) => row[index]).filter((value) => value !== null && value !== undefined).map(String)).size
}

export const donutRemainderKey = '__lens_other__'
export const topNGroupField = '__lens_topn_group'

export function collapseExpandableTopN(frame: Frame, panel: Panel): { frame: Frame; collapsed: boolean } {
  const groupIndex = frame.columns.findIndex((column) => column.name === topNGroupField)
  const labelIndex = frame.columns.findIndex((column) => column.name === (panel.encoding.label ?? panel.encoding.category))
  const idIndex = frame.columns.findIndex((column) => column.name === panel.encoding.id)
  const valueIndex = frame.columns.findIndex((column) => column.name === panel.encoding.value)
  if (groupIndex < 0 || labelIndex < 0 || valueIndex < 0) return { frame, collapsed: false }
  const tail = frame.rows.filter((row) => typeof row[groupIndex] === 'string' && String(row[groupIndex]).trim() !== '')
  if (tail.length === 0) return { frame, collapsed: false }
  const rows = frame.rows.filter((row) => !tail.includes(row)).map((row) => [...row])
  const remainder = new Array<unknown>(frame.columns.length).fill(null)
  remainder[groupIndex] = tail[0]?.[groupIndex]
  remainder[labelIndex] = tail[0]?.[groupIndex]
  if (idIndex >= 0) remainder[idIndex] = donutRemainderKey
  remainder[valueIndex] = tail.reduce((sum, row) => sum + (numericCell(row[valueIndex]) ?? 0), 0)
  rows.push(remainder)
  // A folded tail is not a category either — same rule, same reserved neutral.
  const colors = frame.colors
    ? [...frame.colors.filter((_, index) => !tail.includes(frame.rows[index]!)), remainderColor()]
    : [...rows.slice(0, -1).map(() => ''), remainderColor()]
  return { frame: { ...frame, rows, colors }, collapsed: true }
}

export function collapseMinorDonutSlices(frame: Frame, panel: Panel, otherLabel: string): { frame: Frame; collapsed: boolean } {
  if (panel.kind !== 'donut' || frame.rows.length < 2) return { frame, collapsed: false }
  const labelIndex = frame.columns.findIndex((column) => column.name === (panel.encoding.label ?? panel.encoding.category))
  const idIndex = frame.columns.findIndex((column) => column.name === panel.encoding.id)
  const valueIndex = frame.columns.findIndex((column) => column.name === panel.encoding.value)
  if (labelIndex < 0 || valueIndex < 0) return { frame, collapsed: false }
  const total = frame.total ?? frame.rows.reduce((sum, row) => sum + (numericCell(row[valueIndex]) ?? 0), 0)
  if (!Number.isFinite(total) || total <= 0) return { frame, collapsed: false }
  const minor = frame.rows.map((row, index) => ({ index, value: numericCell(row[valueIndex]) ?? 0 }))
    .filter(({ value }) => value / total < minorDonutShare)
  if (minor.length === 0) return { frame, collapsed: false }
  const minorIndices = new Set(minor.map(({ index }) => index))
  const rows = frame.rows.filter((_, index) => !minorIndices.has(index)).map((row) => [...row])
  const remainder = new Array<unknown>(frame.columns.length).fill(null)
  remainder[labelIndex] = otherLabel
  if (idIndex >= 0) remainder[idIndex] = donutRemainderKey
  remainder[valueIndex] = minor.reduce((sum, item) => sum + item.value, 0)
  rows.push(remainder)
  // The remainder's neutral was pinned only when the frame already shipped a
  // palette. On every other frame the aggregate took the next categorical
  // colour in the walk and was painted as if it were one more real category —
  // the one thing a residual must never look like. A frame with no palette gets
  // one here: the rows keep the colours they would have had, and the remainder
  // gets the reserved neutral.
  const kept = frame.rows.map((_, index) => index).filter((index) => !minorIndices.has(index))
  const colors = frame.colors
    ? [...frame.colors.filter((_, index) => !minorIndices.has(index)), remainderColor()]
    : [...kept.map(() => ''), remainderColor()]
  return { frame: { ...frame, rows, colors }, collapsed: true }
}

export function ChartPanel({ panel, adapter }: ChartPanelProps) {
  const frame = usePanelFrame(panel.id)
  const translate = useTranslate()
  const { document, navigation } = useDashboard()
  const { drillInto } = useDrill()
  const { format, formatAxis } = useChartFormat(panel)
  const [selectedKey, setSelectedKey] = useState<NodeKey>()
  const [remainderExpanded, setRemainderExpanded] = useState(false)
  const [resetZoomKey, setResetZoomKey] = useState(0)
  const initialTemporalState = typeof window === 'undefined'
    ? { regression: false }
    : temporalStateFromURL(new URL(window.location.href), panel.id)
  const [showRegression, setShowRegression] = useState(initialTemporalState.regression)
  const [movingAverageWindow, setMovingAverageWindow] = useState<number | undefined>(initialTemporalState.movingAverage)
  // Overlays the reader switched off from the legend. The trend and the moving
  // average are not held here: they already have state of their own behind the
  // header controls, and one mark must not have two switches that disagree.
  const [dismissedOverlays, setDismissedOverlays] = useState<ReadonlySet<string>>(() => new Set())
  const [localHidden, setLocalHidden] = useState<ReadonlySet<string>>(() => (
    typeof window === 'undefined' ? new Set() : hiddenSeriesFromURL(new URL(window.location.href), panel.id)
  ))
  const hidden = localHidden
  const active = navigation.panelId === panel.id && navigation.path.length > 0
  const level = active
    ? levelForPath(document, navigation.path)
    : (panel.drillRoot ? document.drill.edges[panel.drillRoot] : undefined)
  const drillable = level ? level.children.some(({ target }) => target) : Boolean(panel.drillRoot)
  // One panel, one click behaviour — the legacy rule. A panel with a drill
  // tree explores (the overlay is where its links live); a panel without one
  // navigates straight to its action's target. `hasTree` deliberately keys off
  // the tree's existence, not the current level's children, so reaching a leaf
  // level cannot silently flip a panel from one class to the other.
  const hasTree = Boolean(panel.drillRoot) || Boolean(level)
  const panelNavigation = usePanelNavigation(panel)
  const cancelMarkPrefetch = useRef<() => void>()
  const markURL = useCallback((key: NodeKey) => {
    if (hasTree || !panelNavigation.action || !frame.data) return undefined
    const index = rowIndexForKey(frame.data, panel, key)
    return panelNavigation.urlForRow(frame.data, index >= 0 ? frame.data.rows[index] : undefined)
  }, [frame.data, hasTree, panelNavigation, panel])
  const hoverMark = useCallback((key: NodeKey | null) => {
    cancelMarkPrefetch.current?.()
    cancelMarkPrefetch.current = undefined
    if (key === null || hasTree) return
    cancelMarkPrefetch.current = panelNavigation.prefetch(markURL(key))
  }, [hasTree, markURL, panelNavigation])
  useEffect(() => () => cancelMarkPrefetch.current?.(), [])
  const idleDrawerURLs = useMemo(() => {
    if (hasTree || panelNavigation.action?.kind !== 'open_drawer' || !frame.data) return []
    // Concrete fan-out is safe only for genuinely small result sets. Larger
    // frames wait for hover/focus intent, while the server may still warm their
    // shared dependency closure.
    if (frame.data.rows.length === 0 || frame.data.rows.length > 4) return []
    return [...new Set(frame.data.rows.flatMap((row) => {
      const url = panelNavigation.urlForRow(frame.data, row)
      return url ? [url] : []
    }))]
  }, [frame.data, hasTree, panelNavigation])
  useEffect(() => {
    if (idleDrawerURLs.length === 0) return
    return panelNavigation.prefetchIdle(idleDrawerURLs)
  }, [idleDrawerURLs, panelNavigation])
  // A cross-filter panel is the control the page's filter is chosen from, so
  // the server deliberately leaves it out of its own filter — filtering the
  // age histogram by «65+» would delete the other bars and with them the way
  // back. What was missing is any sign of that: the source panel went on
  // drawing its full distribution while every neighbour dropped to the filtered
  // subset, so the page looked filtered by nothing and the panel looked stale.
  // The mark that is doing the filtering is named here, and the panel says why
  // it is showing all of them (see `.lens-chart-source-note`).
  const filters = useFilters()
  const crossFilter = panelNavigation.action?.kind === 'cross_filter' || panelNavigation.action?.kind === 'cube_drill'
    ? panelNavigation.action.filter
    : undefined
  const activeFilterValues = useMemo(() => {
    if (!crossFilter?.dimension) return undefined
    const raw = filters.values[cubeFilterParam]
    const entries = Array.isArray(raw) ? raw : raw ? [raw] : []
    const prefix = `${crossFilter.dimension}:`
    const values = new Set(entries.filter((entry) => entry.startsWith(prefix)).map((entry) => entry.slice(prefix.length)))
    return values.size > 0 ? values : undefined
  }, [crossFilter?.dimension, filters.values])
  // The mark key of the row the active filter was taken from, resolved through
  // the same source the click resolved (`action.filter.value`) and keyed the
  // same way the chart keys its marks, so the outline lands on the very mark
  // that was clicked rather than on whichever row happens to share its label.
  const filteredKey = useMemo<NodeKey | undefined>(() => {
    if (!activeFilterValues || !crossFilter || !frame.data) return undefined
    const location = new URL(globalThis.location.href)
    const data = frame.data
    for (let index = 0; index < data.rows.length; index += 1) {
      const value = resolveSourceValue(crossFilter.value, {
        fields: recordForRow(data, data.rows[index]!),
        variables: {},
        location,
      })
      const text = textCell(value)
      if (text !== '' && activeFilterValues.has(text)) return legendKey(data, panel, index)
    }
    return undefined
  }, [activeFilterValues, crossFilter, frame.data, panel])
  const kind = panel.kind as ChartKind
  const degenerate = frame.data
    ? distinctCategoryCount(frame.data, panel) <= 1 && distinctSeriesCount(frame.data, panel) <= 1
    : false

  // A new level or perspective is new data; carrying hidden keys across would
  // silently blank out unrelated segments, and carrying the selected key would
  // outline whichever mark of the new level happens to share its id — or, more
  // often, none of them, leaving a selection the user cannot see or clear.
  const viewKey = `${navigation.panelId === panel.id ? navigation.path.join('|') : ''}:${navigation.perspectiveId ?? ''}`
  const previousViewKey = useRef(viewKey)
  useEffect(() => {
    if (previousViewKey.current === viewKey) return
    previousViewKey.current = viewKey
    setSelectedKey(undefined)
  }, [viewKey])

  const setHiddenSeries = useCallback((keys: ReadonlySet<string>) => setLocalHidden(new Set(keys)), [])

  useEffect(() => {
    if (typeof window === 'undefined') return
    const restore = () => setHiddenSeries(hiddenSeriesFromURL(new URL(window.location.href), panel.id))
    window.addEventListener('popstate', restore)
    return () => window.removeEventListener('popstate', restore)
  }, [panel.id, setHiddenSeries])

  useEffect(() => {
    if (typeof window === 'undefined') return
    const current = new URL(window.location.href)
    const next = hiddenSeriesToURL(current, panel.id, hidden)
    if (next.search !== current.search) window.history.replaceState(window.history.state, '', next)
  }, [hidden, panel.id])

  useEffect(() => {
    if (typeof window === 'undefined') return
    const restore = () => {
      const state = temporalStateFromURL(new URL(window.location.href), panel.id)
      setShowRegression(state.regression)
      setMovingAverageWindow(state.movingAverage)
    }
    window.addEventListener('popstate', restore)
    return () => window.removeEventListener('popstate', restore)
  }, [panel.id])

  useEffect(() => {
    if (typeof window === 'undefined') return
    const current = new URL(window.location.href)
    const next = temporalStateToURL(current, panel.id, { regression: showRegression, movingAverage: movingAverageWindow })
    if (next.search !== current.search) window.history.replaceState(window.history.state, '', next)
  }, [movingAverageWindow, panel.id, showRegression])

  // Hidden series are removed from the data rather than dimmed, so the plot
  // and every percentage derived from it share the same visible denominator.
  const visibleFrame = useMemo(() => {
    if (!frame.data || hidden.size === 0) return frame.data
    const keep = frame.data.rows.map((_, index) => !hidden.has(legendEntryKey(frame.data!, panel, index)))
    const rows = frame.data.rows.filter((_, index) => keep[index])
    // `colors` is positional: entry i pins row i. Dropping rows without
    // dropping their pins slides every colour onto its neighbour's slice.
    const colors = frame.data.colors?.filter((_, index) => keep[index])
    const valueIndex = frame.data.columns.findIndex((column) => column.name === panel.encoding.value)
    const total = panel.radial?.mode === 'partition' || valueIndex < 0
      ? frame.data.total
      : rows.reduce((sum, row) => sum + (numericCell(row[valueIndex]) ?? 0), 0)
    return { ...frame.data, rows, total, ...(colors ? { colors } : {}) }
  }, [frame.data, hidden, panel])
  const collapsedRemainder = useMemo(() => visibleFrame
    ? (() => {
      const topN = collapseExpandableTopN(visibleFrame, panel)
      return topN.collapsed ? topN : collapseMinorDonutSlices(visibleFrame, panel, translate('chart.other', 'Other'))
    })()
    : { frame: visibleFrame, collapsed: false }, [panel, translate, visibleFrame])
  const renderFrame = remainderExpanded || !collapsedRemainder.collapsed ? visibleFrame : collapsedRemainder.frame
  // The badge and axis must describe the same rows. Hidden series and a
  // collapsed tail can change whether the rendered values span enough orders
  // of magnitude to justify a logarithmic scale.
  const logarithmic = renderFrame
    ? shouldUseLogarithmicScale(renderFrame, panel.encoding, panel.valueAxis, panel.presentation?.valueSpreadThreshold)
    : false
  // Keep the legend independent of visibility. Hidden entries must remain in
  // the command surface so they can be restored one by one, and their ordinal
  // (therefore their positional colour pin) must not shift when a neighbour is
  // hidden. Remainder grouping still applies so a collapsed tail has one
  // honest "Other" entry rather than exposing rows the plot has folded away.
  const legendFrame = useMemo(() => {
    if (!frame.data || remainderExpanded) return frame.data
    const topN = collapseExpandableTopN(frame.data, panel)
    return topN.collapsed ? topN.frame : collapseMinorDonutSlices(frame.data, panel, translate('chart.other', 'Other')).frame
  }, [frame.data, panel, remainderExpanded, translate])

  const visibleTotal = useMemo(() => {
    if (!frame.data || hidden.size === 0 || !panel.encoding.value || panel.radial?.mode === 'partition') return undefined
    const valueIndex = frame.data.columns.findIndex((column) => column.name === panel.encoding.value)
    if (valueIndex < 0) return undefined
    return (visibleFrame?.rows ?? []).reduce((sum, row) => sum + (numericCell(row[valueIndex]) ?? 0), 0)
  }, [frame.data, hidden.size, panel.encoding.value, panel.radial?.mode, visibleFrame])

  // `panel.total` is the root frame's total, shipped once with the document. At
  // a drill level the panel is showing the level's frame, so the badge has to
  // total that frame instead — the same rows the slice percentages normalize
  // against — or it prints the root's figure over the level's chart.
  const frameRowsTotal = useMemo(() => {
    if (!frame.data || !panel.encoding.value) return undefined
    // A partition radial holds several decompositions of the same headline at
    // once, so summing its rows counts that headline once per ring.
    if (panel.radial?.mode === 'partition') return undefined
    const valueIndex = frame.data.columns.findIndex((column) => column.name === panel.encoding.value)
    if (valueIndex < 0) return undefined
    return frame.data.rows.reduce((sum, row) => sum + (numericCell(row[valueIndex]) ?? 0), 0)
  }, [frame.data, panel.encoding.value, panel.radial?.mode])

  // Once a slice is hidden ECharts normalises the remaining geometry to the
  // visible rows. Use that same denominator for labels and the total badge.
  //
  // With every row hidden there is no denominator and therefore no total —
  // «Итого: 0 UZS» is a claim about the data, and the data did not change. The
  // panel used to print exactly that in its header while the donut in the same
  // card went on printing 45,55 млрд in its hub: one panel, two totals,
  // contradicting each other. Nothing shown, nothing totalled, one owner.
  const nothingVisible = hidden.size > 0 && (visibleFrame?.rows.length ?? 0) === 0
  // The total the panel states at rest. A chart that cannot state one — a series
  // whose rows are ratios, say, where a sum is not a fact — states none, and
  // hiding a series does not conjure one either. The badge used to *appear* on
  // the first legend click, which shifted the export/expand icons under the
  // pointer and gave the recomputed figure no baseline to be compared against.
  const restingTotal = frame.data && frame.data.rows.length > 0
    ? frame.data.total
      ?? (active || panel.kind === 'pie' || panel.kind === 'donut' || panel.kind === 'radial' ? frameRowsTotal : undefined)
      ?? panel.total
    : undefined
  // `null`, not `undefined`: this is the panel's answer, and the header badge
  // must not fall back to the root total behind it. See PanelFrame's `total`.
  const shareTotal: number | null = restingTotal === undefined || nothingVisible
    ? null
    : hidden.size > 0
      ? visibleTotal ?? null
      : restingTotal
  // A served frame may carry the rendering decisions of the panel that produced
  // it. In document mode a drill level is drawn by a placeholder panel frozen
  // before anyone knew which dimension that level would render, so the frame is
  // the only thing that can say "these slices are years, label them as such".
  const presentation = frame.data?.presentation ?? panel.presentation
  // Positional colour pins follow the rows they pin, so they come off the
  // frame the chart is actually drawing.
  const frameColors = renderFrame?.colors

  const toggleSeries = useCallback((key: string) => {
    const next = new Set(hidden)
    if (next.has(key)) next.delete(key)
    else next.add(key)
    setHiddenSeries(next)
  }, [hidden, setHiddenSeries])

  // Per-mark drill affordance. Only meaningful once a level is on screen and
  // its children are known; without a level every mark is equally (un)expandable
  // and the chart-wide treatment already says so.
  const expandable = useMemo(() => {
    if (!level) return undefined
    return (key: string) => Boolean(childForSelection(level, key)?.target)
  }, [level])

  // The plot and the legend resolve colours through the same function, with the
  // same positional rule, so a panel's declared palette reaches both or
  // neither. Resolving them separately is how the legend came to print one
  // colour beside a line drawn in another.
  const paletteLabels = useMemo(() => colorLabels(frame.data, panel), [frame.data, panel])
  const seriesColor = useMemo(() => {
    const resolve = seriesColorResolver(document.theme, panel, { positional: !active, labels: paletteLabels })
    const order = seriesOrder(frame.data, panel)
    if (order.size === 0) return resolve
    return (label: string, index: number) => resolve(label, order.get(label) ?? index)
  }, [active, document.theme, frame.data, paletteLabels, panel])
  // The row-indexed half of the same rule. It is handed the *visible* palette
  // because the plot draws the visible rows; the legend builds its own over the
  // full frame, and the two agree row for row either way.
  const rowColor = useMemo(
    () => rowColorResolver(document.theme, panel, { colors: frameColors, positional: !active, labels: paletteLabels }),
    [active, document.theme, frameColors, paletteLabels, panel],
  )

  // Every overlay the panel declares stays in the input; this set decides which
  // of them are drawn. The chart and the legend then read one list, so an
  // overlay cannot be drawn under a name the legend never prints — or, as
  // before, drawn with no name available anywhere.
  const hiddenOverlays = useMemo(() => {
    const hiddenSet = new Set(dismissedOverlays)
    if (!showRegression) hiddenSet.add(overlayId.trend)
    for (const { window } of panel.temporal?.movingAverages ?? []) {
      if (window !== movingAverageWindow) hiddenSet.add(overlayId.average(window))
    }
    return hiddenSet as ReadonlySet<string>
  }, [dismissedOverlays, movingAverageWindow, panel.temporal?.movingAverages, showRegression])
  const toggleOverlay = useCallback((id: string) => {
    if (id === overlayId.trend) {
      setShowRegression((current) => !current)
      return
    }
    const average = panel.temporal?.movingAverages?.find(({ window }) => overlayId.average(window) === id)
    if (average) {
      setMovingAverageWindow((current) => current === average.window ? undefined : average.window)
      return
    }
    setDismissedOverlays((current) => {
      const next = new Set(current)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }, [panel.temporal?.movingAverages])
  const temporalControls = (kind === 'line' || kind === 'area') && panel.temporal
    && (panel.temporal.regression || panel.temporal.movingAverages?.length) ? (
      <div className="lens-temporal-controls">
        {panel.temporal.regression && (
          <button
            aria-pressed={showRegression}
            className="lens-temporal-toggle"
            onClick={() => setShowRegression((current) => !current)}
            type="button"
          >
            {translate('chart.regression', 'Trend')}
          </button>
        )}
        {Boolean(panel.temporal.movingAverages?.length) && (
          <select
            aria-label={translate('chart.movingAverage', 'Moving average')}
            className="lens-temporal-select"
            onChange={(event) => setMovingAverageWindow(event.target.value ? Number(event.target.value) : undefined)}
            value={movingAverageWindow ?? ''}
          >
            <option value="">{translate('chart.movingAverage', 'Moving average')}</option>
            {panel.temporal.movingAverages?.map(({ window, label }) => (
              <option key={window} value={window}>{label || `SMA ${window}`}</option>
            ))}
          </select>
        )}
      </div>
    ) : undefined

  const chartLabels = useMemo(() => ({
    previous: translate('chart.series.previous', 'Previous'),
    trend: translate('chart.series.trend', 'Trend'),
    movingAverage: (window: number) => translate('chart.series.movingAverage', 'SMA {window}', { window: String(window) }),
    estimate: translate('chart.series.estimate', 'Estimate'),
    ytd: translate('chart.series.ytd', 'YTD'),
    forecast: translate('chart.series.forecast', 'Forecast'),
    forecastLower: (forecast: string) => translate('chart.series.forecastLower', '{forecast} lower', { forecast }),
    forecastConfidence: (forecast: string) => translate('chart.series.forecastConfidence', '{forecast} confidence', { forecast }),
    boxplot: [
      translate('chart.boxplot.min', 'Min'),
      translate('chart.boxplot.q1', 'Q1'),
      translate('chart.boxplot.median', 'Median'),
      translate('chart.boxplot.q3', 'Q3'),
      translate('chart.boxplot.max', 'Max'),
    ] as [string, string, string, string, string],
    noData: translate('panel.empty', 'No data'),
    copyValue: translate('explore.copyValue', 'Copy value'),
    copied: translate('explore.copied', 'Copied'),
  }), [translate])

  const input = useMemo<ChartInput | undefined>(() => renderFrame ? ({
    kind,
    frame: renderFrame,
    encoding: panel.encoding,
    format,
    formatAxis,
    formatTooltip: (field, value, reference) => formatFieldValueAtReference(value, reference, panel.format[field], document.meta.locale),
    // The plot abbreviates because a label has to fit beside a mark; the
    // tooltip is where a reader who stopped to point gets the whole number, and
    // the clipboard gets the machine value behind it.
    formatExact: (field, value) => formatFieldValueExact(value, panel.format[field], document.meta?.locale ?? 'en'),
    rawValue: (field, value) => rawValueText(value, panel.format[field]),
    categoryFormatDefined: Boolean(panel.encoding.category && panel.format[panel.encoding.category]),
    locale: document.meta?.locale,
    shareDecimalSeparator: panel.encoding.value ? panel.format[panel.encoding.value]?.decimalSeparator : undefined,
    tooltipTotalLabel: translate('panel.total', 'Total'),
    labels: chartLabels,
    theme: document.theme,
    // An explicit pick wins; otherwise the mark the page is currently filtered
    // by carries the same outline, so the source panel states its own selection
    // across a reload or a shared URL, not only in the click that made it.
    selectedKey: selectedKey ?? filteredKey,
    presentation,
    seriesColor,
    rowColor,
    radial: panel.radial,
    temporal: panel.temporal,
    hiddenOverlays,
    valueAxis: panel.valueAxis,
    valueUnit: axisUnit(panel.encoding.value ? panel.format[panel.encoding.value] : undefined),
    expandable,
  }) : undefined, [chartLabels, document.meta?.locale, document.theme, expandable, format, formatAxis, filteredKey, hiddenOverlays, kind, panel.encoding, panel.format, panel.radial, panel.temporal, panel.valueAxis, presentation, renderFrame, rowColor, selectedKey, seriesColor, translate])

  // The overlays the legend prints, built from the same list the plot draws
  // from and with the same formatters, so a threshold reads "100%" in both.
  const overlays = useMemo<ChartOverlay[]>(() => {
    if (!panel.temporal && !panel.encoding.previous) return []
    const categoryField = panel.encoding.category ?? panel.encoding.label
    return chartOverlays({
      temporal: panel.temporal,
      hasComparison: Boolean(panel.encoding.previous),
      labels: chartLabels,
      formatValue: (value) => format(panel.encoding.value ?? '', value),
      formatCategory: categoryDisplayFormatter({
        format: (value) => format(categoryField ?? '', value),
        isTime: frame.data?.columns.some((column) => column.name === categoryField && column.type === 'time') === true,
        hasDeclaredFormat: Boolean(categoryField && panel.format[categoryField]),
        locale: document.meta?.locale,
      }),
      hidden: hiddenOverlays,
    })
  }, [chartLabels, document.meta?.locale, format, frame.data?.columns, hiddenOverlays, panel.encoding, panel.format, panel.temporal])
  const onMarkSelect = useMarkSelection()
  // Explore hosts can open the overlay for any segment that has something to
  // show; a standalone tree panel can only drill where a target exists.
  const interactive = hasTree
    ? (onMarkSelect ? Boolean(level?.children.length ?? panel.drillRoot) : drillable)
    : Boolean(panelNavigation.action)
  const distribution = kind === 'histogram' || kind === 'boxplot' || kind === 'heatmap'
  const compact = degenerate && !distribution
  const select = useCallback((key: NodeKey, anchor?: ChartAnchor, activation?: ChartActivation) => {
    if (key === donutRemainderKey && collapsedRemainder.collapsed) {
      setRemainderExpanded((current) => !current)
      return
    }
    if (!hasTree) {
      const href = markURL(key)
      panelNavigation.activate(href, undefined, { newTab: activation?.newTab })
      return
    }
    // With an explore host present the mark opens its overlay; without one the
    // chart keeps drilling directly, so standalone panels are unaffected.
    if (onMarkSelect) {
      setSelectedKey(key)
      onMarkSelect(key, anchor)
      return
    }
    const node = childForSelection(level, key)
    if (level && !node?.target) return
    setSelectedKey(key)
    drillInto(node?.key ?? key, panel.id)
  }, [collapsedRemainder.collapsed, drillInto, hasTree, level, markURL, onMarkSelect, panel.id, panelNavigation])

  // A legend sits to the RIGHT of the plot on a wide panel and drops below it
  // when the panel is too narrow (handled in CSS by a container query). Moving
  // it out of the plot's footer hands the freed width to the chart, which fills
  // the left of the body.
  //
  // A column is for data rows. Rows are the things a legend column exists to
  // command: they carry a share of the total, they can be hidden, isolated,
  // searched and selected in bulk, and there can be twenty-five of them. Marks
  // are none of that — they are a key, four fixed entries that name a stroke —
  // so they accompany the plot as a strip beneath it and never claim a fifth of
  // the plot's width. Naming the marks (79f477ac8) was right; taking a column
  // for them meant «Поступления денежных средств» gave up 240px to print a
  // heading and two struck-through names of marks it was not currently drawing.
  const hasLegend = !compact && presentation?.legend === 'below' && Boolean(renderFrame)
  const hasMarks = !compact && overlays.length > 0 && Boolean(renderFrame)
  const chartInteractive = interactive || collapsedRemainder.collapsed
  const categoryField = panel.encoding.category ?? panel.encoding.label
  const zoomable = (kind === 'line' || kind === 'area')
    && renderFrame?.columns.some((column) => column.name === categoryField && column.type === 'time') === true
  // Which period is unfinished, said once, in words, over the plot — instead of
  // once per series inside it.
  const incompletePeriod = overlays.find((overlay) => overlay.kind === 'incomplete' && overlay.active)
  // The affordance is a glyph; what the click does is the glyph's name. Eight
  // clickable panels on one report were eight grey sentences saying it in
  // prose, so the sentence moved to the accessible name and the tooltip — the
  // same two keys, still in all four locales, still readable at rest and to a
  // screen reader, without spending a line of the card on each panel.
  const drillHint = crossFilter
    ? translate('chart.filterHint', 'Select to filter the page')
    : translate('chart.drillHint', 'Select to explore')
  // The affordance travels with the pointer. As a glyph in the notes row it sat
  // in the corner of the card furthest from wherever the reader was actually
  // pointing — a mark that answers "what happens if I click this" parked where
  // nobody clicks. The tooltip is already open, already under the cursor, and
  // already naming the mark the click would open, so the sentence goes there.
  const hostInput = useMemo(
    () => (input && interactive ? { ...input, actionHint: drillHint } : input),
    [drillHint, input, interactive],
  )
  return (
    <PanelFrame allowEmptyContent={!distribution} panel={panel} frame={frame} headerActions={temporalControls} total={shareTotal}>
      <div className={`lens-chart-layout${hasLegend ? ' lens-chart-layout-legend' : ''}`}>
        <div className="lens-chart-area">
          {/* Above the plot, in flow — see PlotTotalBadge. A donut prints the
              same figure in its hub (the hole exists to carry it), so the chip
              would be the number twice, 100px apart, in two treatments. */}
          {presentation?.totalBadge === 'plot' && kind !== 'donut' && shareTotal !== null && (
            <PlotTotalBadge panel={panel} total={shareTotal} />
          )}
          {/* One row above the plot for everything the plot column carries that
              is not the plot: how to read it on the left, what to do to it on
              the right. The two plot-local buttons were absolutely positioned
              in the same top-right corner — over the gridlines, over the last
              series' final points, and over each other when a collapsed tail
              was expanded on a zoomable chart. A row that takes its own height
              cannot have anything under it, and the note beside the button
              stops competing with the data for the same pixels.

              What the row no longer carries is the drillable glyph: an
              affordance belongs where the act is available, which is under the
              pointer, not in the card's far corner. It rides the tooltip now
              (`ChartInput.actionHint`). */}
          {(logarithmic || incompletePeriod || zoomable || activeFilterValues
            || (remainderExpanded && collapsedRemainder.collapsed)) && (
            <div className="lens-chart-notes">
              {activeFilterValues && (
                <span className="lens-chart-source-note" role="note">
                  {translate('chart.crossFilterSource', 'All categories shown — the filter applies to the other panels')}
                </span>
              )}
              {logarithmic && (
                <span
                  className="lens-chart-log-scale"
                  role="note"
                  title={translate('chart.logScaleHint', 'Values are shown on a logarithmic scale')}
                >
                  <WarningTriangle aria-hidden="true" />
                  <span>{translate('chart.logScale', 'Logarithmic scale')}</span>
                </span>
              )}
              {incompletePeriod && (
                <span className="lens-chart-period-note" role="note">
                  {[incompletePeriod.detail, incompletePeriod.label].filter(Boolean).join(' · ')}
                </span>
              )}
              <span className="lens-chart-notes-actions">
                {remainderExpanded && collapsedRemainder.collapsed && (
                  <button className="lens-chart-collapse-other" onClick={() => setRemainderExpanded(false)} type="button">
                    {translate('chart.collapseOther', 'Collapse Other')}
                  </button>
                )}
                {zoomable && (
                  <button className="lens-chart-reset-zoom" onClick={() => setResetZoomKey((value) => value + 1)} type="button">
                    {translate('chart.resetZoom', 'Reset zoom')}
                  </button>
                )}
              </span>
            </div>
          )}
          {nothingVisible ? (
            <AllSeriesHidden onShowAll={() => setHiddenSeries(new Set())} />
          ) : hostInput && compact ? (
            <CompactChartValue
              actionable={chartInteractive}
              frame={hostInput.frame}
              onSelect={chartInteractive ? select : undefined}
              panel={panel}
            />
          ) : hostInput && (
            <>
              <ChartDataEquivalent
                actionable={chartInteractive}
                format={hostInput.format}
                frame={hostInput.frame}
                label={translate('chart.data', 'Chart data for {name}', { name: panel.title })}
                onHover={hoverMark}
                onSelect={select}
                panel={panel}
                translate={translate}
              />
              <ChartHost
                input={hostInput}
                panelId={panel.id}
                adapter={adapter}
                label={translate('chart.label', '{name} chart', { name: panel.title })}
                drillable={chartInteractive}
                onHover={chartInteractive ? hoverMark : undefined}
                onSelect={chartInteractive ? select : undefined}
                resetZoomKey={resetZoomKey}
              />
            </>
          )}
          {hasMarks && <MarkKey onToggle={toggleOverlay} overlays={overlays} />}
        </div>
        {hasLegend && legendFrame && (
          <ChartLegend
            frame={legendFrame}
            hidden={hidden}
            onSetHidden={setHiddenSeries}
            onToggle={toggleSeries}
            panel={panel}
            presentation={presentation}
            total={shareTotal ?? undefined}
          />
        )}
      </div>
    </PanelFrame>
  )
}

/**
 * What the plot says when the reader has switched everything off.
 *
 * «Скрыть всё» used to leave a featureless grey ring or a blank 280px box with
 * one hairline axis — indistinguishable from a panel that failed — and, on the
 * donuts, a hub still printing 45,55 млрд beside a chip reading «Итого: 0 UZS»:
 * two totals contradicting each other because two components each answered the
 * question. Nothing is shown, so nothing is drawn and nothing is totalled; the
 * panel says which of the two it is and offers the way back.
 */
function AllSeriesHidden({ onShowAll }: { onShowAll: () => void }) {
  const translate = useTranslate()
  return (
    <div className="lens-chart-empty" role="status">
      <span className="lens-chart-empty-note">{translate('chart.allSeriesHidden', 'All series are hidden')}</span>
      <button className="lens-chart-empty-action" onClick={onShowAll} type="button">
        {translate('chart.showAllSeries', 'Show all series')}
      </button>
    </div>
  )
}

function CompactChartValue({ actionable, frame, onSelect, panel }: {
  actionable: boolean
  frame: Frame
  onSelect?: (key: NodeKey) => void
  panel: Panel
}) {
  const translate = useTranslate()
  const labelField = [panel.encoding.category, panel.encoding.label]
    .find((field) => field !== undefined && frame.columns.some((column) => column.name === field))
  const valueField = panel.encoding.value
  const labelIndex = frame.columns.findIndex((column) => column.name === labelField)
  const valueIndex = frame.columns.findIndex((column) => column.name === valueField)
  const formatValue = useFormat(valueField ? panel.format[valueField] : undefined)
  const total = valueIndex < 0 ? undefined : frame.rows.reduce((sum, row) => sum + (numericCell(row[valueIndex]) ?? 0), 0)
  const rawLabel = labelIndex < 0 ? undefined : frame.rows[0]?.[labelIndex]
  const label = textCell(rawLabel)
  const formattedTotal = formatValue(total)
  const keyField = panel.encoding.id ?? labelField
  const keyIndex = frame.columns.findIndex((column) => column.name === keyField)
  const key = textCell(keyIndex < 0 ? undefined : frame.rows[0]?.[keyIndex])
  const content = (
    <>
      {frame.rows.length === 0 ? (
        <span className="lens-chart-compact-empty">{translate('panel.empty', 'No data')}</span>
      ) : (
        <>
          {label && <span className="lens-chart-compact-label" title={label}>{label}</span>}
          <strong className="lens-chart-compact-value">{formattedTotal}</strong>
        </>
      )}
    </>
  )
  const ariaLabel = frame.rows.length === 0
    ? translate('panel.empty', 'No data')
    : [label, formattedTotal].filter(Boolean).join(' / ')
  if (actionable && key && frame.rows.length > 0) {
    return (
      <button
        aria-label={`${ariaLabel}. ${translate('chart.openMark', 'Open {name}', { name: label ?? key })}`}
        className="lens-chart-compact lens-chart-compact-action"
        onClick={() => onSelect?.(key)}
        type="button"
      >
        {content}
      </button>
    )
  }
  return <div aria-label={ariaLabel} className="lens-chart-compact" role="img">{content}</div>
}

/**
 * The plot's own total, as the first row OF the plot column rather than a chip
 * floating over it.
 *
 * It used to be absolutely positioned, and every time something new appeared in
 * whichever corner it had been parked in, the fix was to park it in another
 * corner: away from the drill-back overlay, then out from under the chart
 * toolbar, then out of the plot and into the panel body's top-right — which on a
 * wide panel is the legend column, so it landed on the first legend entry. An
 * overlay has to be told what to avoid, and the list of things to avoid keeps
 * growing. In flow it takes its own height and nothing can be under it.
 */
function PlotTotalBadge({ panel, total }: { panel: Panel; total: number }) {
  const translate = useTranslate()
  const formatTotal = useFormat(panel.encoding.value ? panel.format[panel.encoding.value] : undefined)
  return (
    <span className="lens-plot-total">
      <span className="lens-panel-total-label">{translate('panel.total', 'Total')}:</span>
      {' '}
      {formatTotal(total)}
    </span>
  )
}

/**
 * A legend below the plot lists `label · value` for every slice, so the values
 * stay readable when the plot itself only carries percentages. Entries are
 * buttons: clicking one drops that series from the plot.
 *
 * With `legendValue: 'percent'` the suffix is the share instead, which is the
 * half of the pairing that lets a slice carry its own category label. Shares
 * are measured against `total` — the same authoritative whole the plot uses —
 * so hiding an entry no longer moves the numbers on the entries left behind.
 */
const ChartLegend = memo(function ChartLegend({
  panel, frame, hidden, onSetHidden, onToggle, total, presentation,
}: {
  panel: Panel
  frame: Frame
  hidden: ReadonlySet<string>
  onSetHidden: (keys: ReadonlySet<string>) => void
  onToggle: (key: string) => void
  total?: number
  // The served frame's decisions when it carries any; see ChartPanel.
  presentation?: Panel['presentation']
}) {
  const { document, navigation } = useDashboard()
  const translate = useTranslate()
  const [search, setSearch] = useState('')
  const legendRef = useRef<HTMLUListElement>(null)
  const [legendEdges, setLegendEdges] = useState({ top: false, bottom: false })
  // How many entries the cap is holding below the fold, and whether the reader
  // has lifted it. A fade says "there is more"; only a count says how much more.
  const [beyondFold, setBeyondFold] = useState(0)
  const [expanded, setExpanded] = useState(false)
  const seriesLegendIndex = legendSeriesIndex(frame, panel)
  const labelField = seriesLegendIndex >= 0 ? panel.encoding.series : (panel.encoding.label ?? panel.encoding.category)
  const valueField = panel.encoding.value
  const formatValue = useFormat(valueField ? panel.format[valueField] : undefined)
  const labelIndex = frame.columns.findIndex((column) => column.name === labelField)
  const valueIndex = frame.columns.findIndex((column) => column.name === valueField)
  const formatLabel = useFormat(labelField ? panel.format[labelField] : undefined)
  // At a drill level the rows are the level's, not the panel's own, so the
  // positional color pins no longer describe them.
  const atLevel = navigation.panelId === panel.id && navigation.path.length > 0
  const legendLabels = colorLabels(frame, panel)
  const color = seriesColorResolver(document.theme, panel, { positional: !atLevel, labels: legendLabels })
  // Part-to-whole entries stand for rows, and a row's colour can come off the
  // frame the level served — which `color` cannot see. Same resolver the plot
  // is built with, over the whole frame rather than the visible part of it.
  const rowColor = rowColorResolver(document.theme, panel, {
    colors: frame.colors, positional: !atLevel, labels: legendLabels,
  })
  const idIndex = panel.encoding.id ? frame.columns.findIndex((column) => column.name === panel.encoding.id) : -1
  const seriesIndex = panel.encoding.series
    ? frame.columns.findIndex((column) => column.name === panel.encoding.series)
    : -1
  const partition = panel.radial?.mode === 'partition'
  const model = useMemo(() => {
    const rowKeys = frame.rows.map((row, index) => {
      const raw = idIndex >= 0 ? row[idIndex] : row[labelIndex]
      return typeof raw === 'string' || typeof raw === 'number' || typeof raw === 'bigint' ? String(raw) : String(index)
    })
    const entryKeys = frame.rows.map((row, index) => {
      if (seriesLegendIndex < 0) return rowKeys[index]!
      const raw = row[seriesLegendIndex]
      return typeof raw === 'string' || typeof raw === 'number' || typeof raw === 'bigint' ? String(raw) : String(index)
    })
    let entries = frame.rows.map((_, index) => index)
    if (seriesLegendIndex >= 0 || partition) {
      const seen = new Set<string>()
      entries = []
      for (let index = 0; index < frame.rows.length; index += 1) {
        const key = entryKeys[index]!
        if (seen.has(key)) continue
        seen.add(key)
        entries.push(index)
      }
    }
    const allKeys = entries.map((index) => entryKeys[index]!)
    const entryPositions = new Map(entries.map((index, position) => [index, position]))
    const categoryOrder = new Map<string, number>()
    if (partition) rowKeys.forEach((key) => {
      if (!categoryOrder.has(key)) categoryOrder.set(key, categoryOrder.size)
    })

    const ringTotals = new Map((panel.radial?.rings ?? []).map((ring) => [ring.key, ring.total]))
    const groups = new Map<string, { denominator: number | undefined; indices: number[] }>()
    for (let index = 0; index < frame.rows.length; index += 1) {
      const rawRing = seriesIndex >= 0 ? frame.rows[index]?.[seriesIndex] : undefined
      const ring = partition && typeof rawRing === 'string' ? rawRing : ''
      const denominator = partition ? ringTotals.get(ring) ?? total : total
      const groupKey = partition ? ring : String(denominator)
      const group = groups.get(groupKey) ?? { denominator, indices: [] }
      group.indices.push(index)
      groups.set(groupKey, group)
    }
    const shares = new Map<number, number | undefined>()
    for (const { denominator, indices } of groups.values()) {
      const values = indices.map((index) => valueIndex >= 0 ? numericCell(frame.rows[index]?.[valueIndex]) : undefined)
      distributeShares(values, denominator).forEach((share, position) => shares.set(indices[position]!, share))
    }
    const rowsPerKey = new Map<string, number>()
    if (partition) rowKeys.forEach((key) => rowsPerKey.set(key, (rowsPerKey.get(key) ?? 0) + 1))

    const valueByKey = new Map<string, number>()
    if (valueIndex >= 0) frame.rows.forEach((row, index) => {
      const value = numericCell(row[valueIndex])
      if (value === undefined) return
      const key = entryKeys[index]!
      if (seriesLegendIndex >= 0 && valueField && panel.format[valueField]?.kind !== 'percent') {
        valueByKey.set(key, (valueByKey.get(key) ?? 0) + value)
      } else {
        // Row legends have one value per key; ratio-series legends show the
        // latest reading rather than adding percentages across periods.
        valueByKey.set(key, value)
      }
    })
    const ringByIndex = new Map<number, string>()
    if (partition && seriesIndex >= 0) entries.forEach((index) => {
      const raw = frame.rows[index]?.[seriesIndex]
      if (typeof raw === 'string') ringByIndex.set(index, raw)
    })
    return { allKeys, categoryOrder, entries, entryKeys, entryPositions, ringByIndex, rowKeys, rowsPerKey, shares, valueByKey }
  }, [frame, idIndex, labelIndex, panel.format, panel.radial?.rings, partition, seriesIndex, seriesLegendIndex, total, valueField, valueIndex])
  // The plot pins one colour per category across every ring of a partition, so
  // its palette index is the category's, not the row's (see `categoryOrder` in
  // the chart adapter). Anywhere else a row is its own category.
  const swatch = (label: string, index: number, entryIndex: number): string | undefined => {
    if (seriesLegendIndex >= 0) return color(label, entryIndex)
    const raw = idIndex >= 0 ? frame.rows[index]?.[idIndex] : undefined
    const nodeKey = typeof raw === 'string' && raw.trim() !== '' ? raw : undefined
    return rowColor(label, model.categoryOrder.get(model.rowKeys[index]!) ?? index, nodeKey)
  }
  // A percent legend suppresses nothing: it is the only place the share of a
  // slice labelled with its own category name is written down.
  const showsPercent = presentation?.legendValue === 'percent'
  // A partition category that appears in exactly one ring has exactly one
  // share, so the legend can state it. Only a category repeated across rings
  // is ambiguous — that is what suppressed the suffix here, and suppressing it
  // for everyone left sub-4% arcs, the ones the ring exists to show, with their
  // share written down nowhere at all.
  const suffixFor = (index: number): 'percent' | 'value' | 'none' => {
    if (seriesLegendIndex >= 0) return 'value'
    if (showsPercent) return 'percent'
    if (!partition) return 'value'
    return model.rowsPerKey.get(model.rowKeys[index]!) === 1 ? 'percent' : 'none'
  }
  const normalizedSearch = search.trim().toLocaleLowerCase()
  const visibleEntries = normalizedSearch === '' ? model.entries : model.entries.filter((index) => {
    const raw = frame.rows[index]?.[labelIndex]
    return textCell(raw).toLocaleLowerCase().includes(normalizedSearch)
  })
  useLayoutEffect(() => {
    const list = legendRef.current
    if (!list) return
    const measure = () => {
      const fold = list.scrollTop + list.clientHeight
      // Only a column that is actually taller than its box is holding anything
      // back. Below a narrow plot the list wraps instead of scrolling, and a
      // row's bottom edge sitting past the last line is not an entry the reader
      // cannot see — counting those printed «2 more» over a legend of two.
      const clipped = list.scrollHeight > list.clientHeight + 1
      setLegendEdges({
        top: clipped && list.scrollTop > 1,
        bottom: clipped && fold < list.scrollHeight - 1,
      })
      if (!clipped) {
        setBeyondFold(0)
        return
      }
      const items = list.querySelectorAll<HTMLElement>('.lens-chart-legend-item')
      let beyond = 0
      for (const item of items) if (item.offsetTop + item.offsetHeight > fold + 1) beyond += 1
      setBeyondFold(beyond)
    }
    measure()
    list.addEventListener('scroll', measure, { passive: true })
    const observer = typeof ResizeObserver === 'undefined' ? undefined : new ResizeObserver(measure)
    observer?.observe(list)
    return () => {
      list.removeEventListener('scroll', measure)
      observer?.disconnect()
    }
  }, [expanded, visibleEntries.length])
  if (labelIndex < 0) return null
  const solo = (key: string) => {
    const alreadySolo = !hidden.has(key) && model.allKeys.every((candidate) => candidate === key || hidden.has(candidate))
    onSetHidden(alreadySolo ? new Set() : new Set(model.allKeys.filter((candidate) => candidate !== key)))
  }
  // The whole legend is either shown or hidden or somewhere between, and the
  // control says which: a two-state segmented control, not two loose commands.
  const allHidden = model.allKeys.length > 0 && model.allKeys.every((key) => hidden.has(key))
  const noneHidden = model.allKeys.every((key) => !hidden.has(key))
  const bulk = model.allKeys.length > legendBulkThreshold
  // A legend long enough to be worth searching earns the full control header;
  // a shorter bulk legend gets a single switch.
  const long = model.entries.length >= legendControlEntries

  return (
    <div className="lens-chart-legend-shell" style={expanded ? { '--lens-chart-legend-max': 'none' } as React.CSSProperties : undefined}>
      {bulk && (
        <div className="lens-chart-legend-controls">
          <div
            aria-label={translate('chart.legendControls', 'Legend controls')}
            className="lens-chart-legend-tools"
            data-members={long ? 'many' : 'one'}
            role="group"
          >
            {long ? (
              <>
                <button
                  aria-pressed={allHidden}
                  onClick={() => onSetHidden(new Set(model.allKeys))}
                  type="button"
                >
                  {translate('chart.legendHideAll', 'Hide all')}
                </button>
                <button
                  aria-pressed={noneHidden}
                  onClick={() => onSetHidden(new Set())}
                  type="button"
                >
                  {translate('chart.legendShowAll', 'Show all')}
                </button>
                <button
                  aria-label={translate('chart.legendInvert', 'Invert')}
                  className="lens-chart-legend-invert"
                  onClick={() => onSetHidden(new Set(model.allKeys.filter((key) => !hidden.has(key))))}
                  title={translate('chart.legendInvert', 'Invert')}
                  type="button"
                >
                  <ArrowsLeftRight />
                </button>
              </>
            ) : (
              <button
                onClick={() => onSetHidden(allHidden ? new Set() : new Set(model.allKeys))}
                type="button"
              >
                {allHidden ? translate('chart.legendShowAll', 'Show all') : translate('chart.legendHideAll', 'Hide all')}
              </button>
            )}
          </div>
          {long && (
            <label className="lens-chart-legend-search">
              <span className="lens-sr-only">{translate('chart.legendSearch', 'Search legend')}</span>
              <input
                className="lens-facet-search"
                onChange={(event) => setSearch(event.currentTarget.value)}
                placeholder={translate('chart.legendSearch', 'Search legend')}
                type="search"
                value={search}
              />
            </label>
          )}
        </div>
      )}
      <div
        className="lens-chart-legend-scroll-frame"
        data-overflow-bottom={legendEdges.bottom || undefined}
        data-overflow-top={legendEdges.top || undefined}
      >
        {/* eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex -- overflowing native scroll regions must be keyboard-focusable. */}
        <ul aria-label={translate('chart.legendControls', 'Legend controls')} className="lens-chart-legend" ref={legendRef} role="region" tabIndex={legendEdges.top || legendEdges.bottom ? 0 : undefined}>
          {visibleEntries.map((index, visibleIndex) => {
            const row = frame.rows[index]!
            const entryIndex = model.entryPositions.get(index) ?? index
            const raw = row[labelIndex]
            const label = raw === null || raw === undefined ? '' : formatLabel(raw)
            const key = model.entryKeys[index]!
            const isHidden = hidden.has(key)
            const ringKey = model.ringByIndex.get(index)
            const previousRingKey = visibleIndex > 0 ? model.ringByIndex.get(visibleEntries[visibleIndex - 1]!) : undefined
            const ringLabel = panel.radial?.rings?.find((ring) => ring.key === ringKey)?.label ?? ringKey
            return (
              <Fragment key={`${key}-${index}`}>
                {ringKey && ringKey !== previousRingKey && <li className="lens-chart-legend-heading">{ringLabel}</li>}
                <li className="lens-chart-legend-item" key={`${key}-${index}`}>
                  <button
                    aria-pressed={!isHidden}
                    className={`lens-chart-legend-toggle${isHidden ? ' lens-chart-legend-hidden' : ''}`}
                    onClick={() => onToggle(key)}
                    // The charting idiom every reader arrives with, and the
                    // one the isolate glyph beside it was the only way to
                    // reach: double-clicking a row leaves that row alone on
                    // the plot. The two clicks that precede it cancel out.
                    onDoubleClick={() => solo(key)}
                    title={translate('chart.legendToggleHint', 'Click to toggle · double-click to isolate')}
                    type="button"
                  >
                    <span
                      aria-hidden="true"
                      className="lens-chart-legend-mark"
                      style={{ background: swatch(label, index, entryIndex) }}
                    />
                    <span className="lens-chart-legend-label">{label}</span>
                    {suffixFor(index) !== 'none' && (
                      <span className="lens-chart-legend-value">
                        {suffixFor(index) === 'percent'
                          ? formatShare(model.shares.get(index), document.meta?.locale, valueField ? panel.format[valueField]?.decimalSeparator : undefined)
                          : formatValue(model.valueByKey.get(key))}
                      </span>
                    )}
                    {/* The row's state, drawn as the thing it is. This corner
                        used to hold a `◎` in a button of its own, which
                        isolated the series: an unlabelled shape for an action
                        no reader could guess, and a second tab stop repeating
                        the name of the row it sat in. The glyph now says only
                        what the row already carries in `aria-pressed`, so it
                        is the row's mark rather than a control beside it.
                        Isolating is the idiom every charting library has
                        taught instead — double-click the row. */}
                    <span aria-hidden="true" className="lens-chart-legend-visibility">
                      {isHidden ? <EyeSlash /> : <Eye />}
                    </span>
                  </button>
                </li>
              </Fragment>
            )
          })}
        </ul>
        <span aria-hidden="true" className="lens-chart-legend-edge lens-chart-legend-edge-top" />
        <span aria-hidden="true" className="lens-chart-legend-edge lens-chart-legend-edge-bottom" />
      </div>
      {(beyondFold > 0 || expanded) && (
        <button className="lens-chart-legend-overflow" onClick={() => setExpanded((current) => !current)} type="button">
          {expanded
            ? translate('chart.legendCollapse', 'Show fewer')
            : translate('chart.legendMore', '{count} more', { count: String(beyondFold) })}
        </button>
      )}
    </div>
  )
})

/**
 * The marks that are not rows of the frame: thresholds, events, forecasts, the
 * unfinished period, the fitted line, the comparison ghost.
 *
 * A key, not a legend, and it is laid out as one: a strip that follows the plot
 * across the plot's own width, wrapping when it must. It has no shares, no
 * isolate, no search, no hide-all — an overlay is not part of any sum, so it
 * carries no value cell and the bulk controls in the legend column never
 * reached it anyway. It therefore has no business taking a 240px column: on
 * «Поступления денежных средств» that column held a heading and two names and
 * nothing else, while the plot beside it ran a fifth narrower for the privilege.
 *
 * Clicking one draws or removes that mark, and nothing else on the plot moves:
 * removing a threshold cannot change a percentage the way hiding a series does.
 * The swatch is a miniature of the mark itself — the same stroke, the same ink —
 * so it can be matched to the plot without reading the name.
 */
const MarkKey = memo(function MarkKey({ overlays, onToggle }: {
  overlays: readonly ChartOverlay[]
  onToggle: (id: string) => void
}) {
  const translate = useTranslate()
  const heading = translate('chart.legendOverlays', 'Chart marks')
  return (
    <ul aria-label={heading} className="lens-chart-legend lens-chart-overlay-legend">
      <li className="lens-chart-legend-heading">{heading}</li>
      {overlays.map((overlay) => (
        <li className="lens-chart-legend-item" key={overlay.id}>
          <button
            aria-pressed={overlay.active}
            className={`lens-chart-legend-toggle${overlay.active ? '' : ' lens-chart-legend-hidden'}`}
            onClick={() => onToggle(overlay.id)}
            title={translate('chart.legendToggleOverlay', 'Show or hide this mark')}
            type="button"
          >
            <span
              aria-hidden="true"
              className="lens-chart-overlay-mark"
              data-stroke={overlay.stroke}
              style={{ '--lens-overlay-ink': `var(${overlayToneProperty(overlay.tone)})` } as React.CSSProperties}
            />
            <span className="lens-chart-legend-label">{overlay.label}</span>
            {overlay.detail && (
              <>
                <span aria-hidden="true" className="lens-chart-legend-separator">·</span>
                <span className="lens-chart-legend-value">{overlay.detail}</span>
              </>
            )}
          </button>
        </li>
      ))}
    </ul>
  )
})

// Compatibility aliases for direct consumers; the registry has one lazy
// component and therefore one loading/error boundary for every chart kind.
export const PiePanel = ChartPanel
export const BarPanel = ChartPanel
export const LinePanel = ChartPanel
export const DistributionPanel = ChartPanel
