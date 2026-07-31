import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { Frame, NodeKey, Panel } from '../contract'
import { radialNodeKey, type ChartActivation, type ChartAdapter, type ChartAnchor, type ChartFormatResolver, type ChartKind } from '../charts/adapter'
import { fallbackMarkKey } from '../charts/keys'
import { distributeShares, formatShare } from '../charts/shares'
import { shouldUseLogarithmicScale } from '../charts/scales'
import { childForSelection } from '../explore/model'
import { WarningTriangle } from '../icons'
import { levelForPath, useAxisFormat, useDashboard, useDrill, useFormat, usePanelFrame, useTranslate } from '../runtime'
import { hiddenSeriesFromURL, hiddenSeriesToURL } from '../runtime/url'
import { usePanelNavigation } from './actions'
import { ChartHost } from './ChartHost'
import { useLegendVisibility, useMarkSelection } from './context'
import { encodingRoles, rowColorResolver, seriesColorResolver } from './data'
import { PanelFrame } from './PanelFrame'

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
  const field = partition ? (panel.encoding.label ?? panel.encoding.category) : (panel.encoding.category ?? panel.encoding.label)
  const index = frame.columns.findIndex((column) => column.name === field)
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
  return { frame: { ...frame, rows }, collapsed: true }
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
    .filter(({ value }) => value / total < 0.02)
  if (minor.length === 0) return { frame, collapsed: false }
  const minorIndices = new Set(minor.map(({ index }) => index))
  const rows = frame.rows.filter((_, index) => !minorIndices.has(index)).map((row) => [...row])
  const remainder = new Array<unknown>(frame.columns.length).fill(null)
  remainder[labelIndex] = otherLabel
  if (idIndex >= 0) remainder[idIndex] = donutRemainderKey
  remainder[valueIndex] = minor.reduce((sum, item) => sum + item.value, 0)
  rows.push(remainder)
  const colors = frame.colors
    ? [...frame.colors.filter((_, index) => !minorIndices.has(index)), '#94a3b8']
    : undefined
  return { frame: { ...frame, rows, ...(colors ? { colors } : {}) }, collapsed: true }
}

export function ChartPanel({ panel, adapter }: ChartPanelProps) {
  const frame = usePanelFrame(panel.id)
  const translate = useTranslate()
  const { document, navigation } = useDashboard()
  const { drillInto } = useDrill()
  const { format, formatAxis } = useChartFormat(panel)
  const [selectedKey, setSelectedKey] = useState<NodeKey>()
  const [hoveredKey, setHoveredKey] = useState<NodeKey | null>(null)
  const [remainderExpanded, setRemainderExpanded] = useState(false)
  const [resetZoomKey, setResetZoomKey] = useState(0)
  const [showRegression, setShowRegression] = useState(false)
  const [movingAverageWindow, setMovingAverageWindow] = useState<number>()
  const [localHidden, setLocalHidden] = useState<ReadonlySet<string>>(() => (
    typeof window === 'undefined' ? new Set() : hiddenSeriesFromURL(new URL(window.location.href), panel.id)
  ))
  const legendVisibility = useLegendVisibility()
  const hidden = legendVisibility?.hidden ?? localHidden
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
  const markURL = useCallback((key: NodeKey) => {
    if (hasTree || !panelNavigation.action || !frame.data) return undefined
    const index = rowIndexForKey(frame.data, panel, key)
    return panelNavigation.urlForRow(frame.data, index >= 0 ? frame.data.rows[index] : undefined)
  }, [frame.data, hasTree, panelNavigation, panel])
  const kind = panel.kind as ChartKind
  const degenerate = frame.data
    ? distinctCategoryCount(frame.data, panel) <= 1 && distinctSeriesCount(frame.data, panel) <= 1
    : false
  const logarithmic = frame.data ? shouldUseLogarithmicScale(frame.data, panel.encoding, panel.valueAxis) : false

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

  const setHiddenSeries = useCallback((keys: ReadonlySet<string>) => {
    if (legendVisibility) legendVisibility.set(keys)
    else setLocalHidden(new Set(keys))
  }, [legendVisibility])

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

  // Hidden series are removed from the data rather than dimmed, so the plot
  // reclaims their arc. The percentages do not follow: they are measured
  // against the frame's authoritative total, so hiding one slice no longer
  // rewrites the numbers on every slice that stayed.
  const visibleFrame = useMemo(() => {
    if (!frame.data || hidden.size === 0) return frame.data
    const keep = frame.data.rows.map((_, index) => !hidden.has(legendEntryKey(frame.data!, panel, index)))
    const rows = frame.data.rows.filter((_, index) => keep[index])
    // `colors` is positional: entry i pins row i. Dropping rows without
    // dropping their pins slides every colour onto its neighbour's slice.
    const colors = frame.data.colors?.filter((_, index) => keep[index])
    return { ...frame.data, rows, ...(colors ? { colors } : {}) }
  }, [frame.data, hidden, panel])
  const collapsedRemainder = useMemo(() => visibleFrame
    ? (() => {
        const topN = collapseExpandableTopN(visibleFrame, panel)
        return topN.collapsed ? topN : collapseMinorDonutSlices(visibleFrame, panel, translate('chart.other', 'Other'))
      })()
    : { frame: visibleFrame, collapsed: false }, [panel, translate, visibleFrame])
  const renderFrame = remainderExpanded || !collapsedRemainder.collapsed ? visibleFrame : collapsedRemainder.frame
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
  const levelTotal = useMemo(() => {
    if (!active || !frame.data || !panel.encoding.value) return undefined
    // A partition radial holds several decompositions of the same headline at
    // once, so summing its rows counts that headline once per ring.
    if (panel.radial?.mode === 'partition') return undefined
    const valueIndex = frame.data.columns.findIndex((column) => column.name === panel.encoding.value)
    if (valueIndex < 0) return undefined
    return frame.data.rows.reduce((sum, row) => sum + (numericCell(row[valueIndex]) ?? 0), 0)
  }, [active, frame.data, panel.encoding.value, panel.radial?.mode])

  // The whole every share on this panel is measured against. A producer-shipped
  // frame total outranks anything derived here — including `visibleTotal` — so
  // the badge cannot disagree with the percentages printed next to it, and
  // neither moves when a legend entry is toggled off.
  const shareTotal = frame.data?.total ?? visibleTotal ?? levelTotal ?? panel.total
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
  const seriesColor = useMemo(() => {
    const resolve = seriesColorResolver(document.theme, panel, { positional: !active })
    const order = seriesOrder(frame.data, panel)
    if (order.size === 0) return resolve
    return (label: string, index: number) => resolve(label, order.get(label) ?? index)
  }, [active, document.theme, frame.data, panel])
  // The row-indexed half of the same rule. It is handed the *visible* palette
  // because the plot draws the visible rows; the legend builds its own over the
  // full frame, and the two agree row for row either way.
  const rowColor = useMemo(
    () => rowColorResolver(document.theme, panel, { colors: frameColors, positional: !active }),
    [active, document.theme, frameColors, panel],
  )

  const temporal = useMemo(() => panel.temporal ? {
    ...panel.temporal,
    regression: showRegression ? panel.temporal.regression : undefined,
    movingAverages: panel.temporal.movingAverages?.filter(({ window }) => window === movingAverageWindow),
  } : undefined, [movingAverageWindow, panel.temporal, showRegression])
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

  const input = useMemo(() => renderFrame ? ({
    kind,
    frame: renderFrame,
    encoding: panel.encoding,
    format,
    formatAxis,
    categoryFormatDefined: Boolean(panel.encoding.category && panel.format[panel.encoding.category]),
    locale: document.meta?.locale,
    tooltipTotalLabel: translate('panel.total', 'Total'),
    theme: document.theme,
    selectedKey,
    presentation,
    seriesColor,
    rowColor,
    radial: panel.radial,
    temporal,
    valueAxis: panel.valueAxis,
    expandable,
  }) : undefined, [document.meta?.locale, document.theme, expandable, format, formatAxis, kind, panel.encoding, panel.format, panel.radial, panel.valueAxis, presentation, renderFrame, rowColor, selectedKey, seriesColor, temporal, translate])
  const onMarkSelect = useMarkSelection()
  // Explore hosts can open the overlay for any segment that has something to
  // show; a standalone tree panel can only drill where a target exists.
  const interactive = hasTree
    ? (onMarkSelect ? Boolean(level?.children.length ?? panel.drillRoot) : drillable)
    : Boolean(panelNavigation.action)
  const distribution = kind === 'histogram' || kind === 'boxplot' || kind === 'heatmap'
  const compact = degenerate && !interactive && !distribution
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
  const hasLegend = !compact && presentation?.legend === 'below' && Boolean(renderFrame)
  const chartInteractive = interactive || collapsedRemainder.collapsed
  return (
    <PanelFrame allowEmptyContent={!distribution} panel={panel} frame={frame} headerActions={temporalControls} total={shareTotal}>
      <div className={`lens-chart-layout${hasLegend ? ' lens-chart-layout-legend' : ''}`}>
        <div className="lens-chart-area">
          {/* Above the plot, in flow — see PlotTotalBadge. */}
          {presentation?.totalBadge === 'plot' && shareTotal !== undefined && (
            <PlotTotalBadge panel={panel} total={shareTotal} />
          )}
          {logarithmic && (
            <div
              aria-label={translate('chart.logScaleHint', 'Values are shown on a logarithmic scale')}
              className="lens-chart-log-scale"
              role="note"
              title={translate('chart.logScaleHint', 'Values are shown on a logarithmic scale')}
            >
              <WarningTriangle aria-hidden="true" />
              <span>{translate('chart.logScale', 'Logarithmic scale')}</span>
            </div>
          )}
          {remainderExpanded && collapsedRemainder.collapsed && (
            <button className="lens-chart-collapse-other" onClick={() => setRemainderExpanded(false)} type="button">
              {translate('chart.collapseOther', 'Collapse Other')}
            </button>
          )}
          {(kind === 'line' || kind === 'area') && (
            <button className="lens-chart-reset-zoom" onClick={() => setResetZoomKey((value) => value + 1)} type="button">
              {translate('chart.resetZoom', 'Reset zoom')}
            </button>
          )}
          {input && compact ? (
            <CompactChartValue frame={input.frame} panel={panel} />
          ) : input && (
            <ChartHost
              input={input}
              panelId={panel.id}
              adapter={adapter}
              label={translate('chart.label', '{name} chart', { name: panel.title })}
              drillable={chartInteractive}
              onSelect={chartInteractive ? select : undefined}
              onHover={chartInteractive ? setHoveredKey : undefined}
              resetZoomKey={resetZoomKey}
            />
          )}
        </div>
        {hasLegend && legendFrame && (
          <ChartLegend
            frame={legendFrame}
            hidden={hidden}
            onSetHidden={setHiddenSeries}
            onToggle={toggleSeries}
            panel={panel}
            presentation={presentation}
            total={shareTotal}
          />
        )}
      </div>
      {interactive && hoveredKey && (
        <span className="lens-chart-drill-hint" role="status">
          {translate('chart.drillHint', 'Select to explore')}
        </span>
      )}
    </PanelFrame>
  )
}

function CompactChartValue({ frame, panel }: { frame: Frame; panel: Panel }) {
  const translate = useTranslate()
  const labelField = panel.encoding.category ?? panel.encoding.label
  const valueField = panel.encoding.value
  const labelIndex = frame.columns.findIndex((column) => column.name === labelField)
  const valueIndex = frame.columns.findIndex((column) => column.name === valueField)
  const formatValue = useFormat(valueField ? panel.format[valueField] : undefined)
  const total = valueIndex < 0 ? undefined : frame.rows.reduce((sum, row) => sum + (numericCell(row[valueIndex]) ?? 0), 0)
  const rawLabel = labelIndex < 0 ? undefined : frame.rows[0]?.[labelIndex]
  const label = textCell(rawLabel)
  return (
    <div className="lens-chart-compact" role="status">
      {frame.rows.length === 0 ? (
        <span className="lens-chart-compact-empty">{translate('panel.empty', 'No data')}</span>
      ) : (
        <>
          {label && <span className="lens-chart-compact-label" title={label}>{label}</span>}
          <strong className="lens-chart-compact-value">{formatValue(total)}</strong>
        </>
      )}
    </div>
  )
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
function ChartLegend({ panel, frame, hidden, onSetHidden, onToggle, total, presentation }: {
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
  const seriesLegendIndex = legendSeriesIndex(frame, panel)
  const labelField = seriesLegendIndex >= 0 ? panel.encoding.series : (panel.encoding.label ?? panel.encoding.category)
  const valueField = panel.encoding.value
  const formatValue = useFormat(valueField ? panel.format[valueField] : undefined)
  const labelIndex = frame.columns.findIndex((column) => column.name === labelField)
  const valueIndex = frame.columns.findIndex((column) => column.name === valueField)
  if (labelIndex < 0) return null

  // At a drill level the rows are the level's, not the panel's own, so the
  // positional color pins no longer describe them.
  const atLevel = navigation.panelId === panel.id && navigation.path.length > 0
  const color = seriesColorResolver(document.theme, panel, { positional: !atLevel })
  // Part-to-whole entries stand for rows, and a row's colour can come off the
  // frame the level served — which `color` cannot see. Same resolver the plot
  // is built with, over the whole frame rather than the visible part of it.
  const rowColor = rowColorResolver(document.theme, panel, { colors: frame.colors, positional: !atLevel })
  const idIndex = panel.encoding.id ? frame.columns.findIndex((column) => column.name === panel.encoding.id) : -1
  // The plot pins one colour per category across every ring of a partition, so
  // its palette index is the category's, not the row's (see `categoryOrder` in
  // the chart adapter). Anywhere else a row is its own category.
  const categoryOrder = new Map<string, number>()
  if (panel.radial?.mode === 'partition') {
    frame.rows.forEach((_, index) => {
      const key = legendKey(frame, panel, index)
      if (!categoryOrder.has(key)) categoryOrder.set(key, categoryOrder.size)
    })
  }
  const swatch = (label: string, index: number, entryIndex: number): string | undefined => {
    if (seriesLegendIndex >= 0) return color(label, entryIndex)
    const raw = idIndex >= 0 ? frame.rows[index]?.[idIndex] : undefined
    const nodeKey = typeof raw === 'string' && raw.trim() !== '' ? raw : undefined
    return rowColor(label, categoryOrder.get(legendKey(frame, panel, index)) ?? index, nodeKey)
  }
  // A time series repeats its series name for every period, while a partition
  // radial repeats a category across rings. Both need one legend entry per key.
  const entries = seriesLegendIndex >= 0 || panel.radial?.mode === 'partition'
    ? (() => {
        const seen = new Set<string>()
        const indices: number[] = []
        frame.rows.forEach((_, index) => {
          const key = legendEntryKey(frame, panel, index)
          if (seen.has(key)) return
          seen.add(key)
          indices.push(index)
        })
        return indices
      })()
    : frame.rows.map((_, index) => index)
  const allKeys = entries.map((index) => legendEntryKey(frame, panel, index))

  // A partition ring reconciles against its own declared total, so a share is
  // only meaningful within a ring. Rows are grouped by their denominator and
  // reconciled to 100% inside each group.
  const seriesIndex = panel.encoding.series
    ? frame.columns.findIndex((column) => column.name === panel.encoding.series)
    : -1
  const ringTotals = new Map((panel.radial?.rings ?? []).map((ring) => [ring.key, ring.total]))
  const denominatorFor = (index: number): number | undefined => {
    if (panel.radial?.mode !== 'partition' || seriesIndex < 0) return total
    const key = frame.rows[index]?.[seriesIndex]
    return typeof key === 'string' ? ringTotals.get(key) ?? total : total
  }
  const ringOf = (index: number): string => {
    if (panel.radial?.mode !== 'partition' || seriesIndex < 0) return ''
    const key = frame.rows[index]?.[seriesIndex]
    return typeof key === 'string' ? key : ''
  }
  const shares = new Map<number, number | undefined>()
  // Two rings commonly declare the same total — both 100% of their own whole.
  // Grouping by the denominator would then reconcile them as one set of rows
  // and hand each ring a share of half its own decomposition.
  const groups = new Map<string, { denominator: number | undefined; indices: number[] }>()
  for (let index = 0; index < frame.rows.length; index += 1) {
    const denominator = denominatorFor(index)
    const key = panel.radial?.mode === 'partition' ? ringOf(index) : String(denominator)
    const group = groups.get(key) ?? { denominator, indices: [] }
    group.indices.push(index)
    groups.set(key, group)
  }
  for (const { denominator, indices } of groups.values()) {
    const values = indices.map((index) => (valueIndex >= 0 ? numericCell(frame.rows[index]![valueIndex]) : undefined))
    distributeShares(values, denominator).forEach((share, position) => shares.set(indices[position]!, share))
  }
  // A percent legend suppresses nothing: it is the only place the share of a
  // slice labelled with its own category name is written down.
  const showsPercent = presentation?.legendValue === 'percent'
  // A partition category that appears in exactly one ring has exactly one
  // share, so the legend can state it. Only a category repeated across rings
  // is ambiguous — that is what suppressed the suffix here, and suppressing it
  // for everyone left sub-4% arcs, the ones the ring exists to show, with their
  // share written down nowhere at all.
  const partition = panel.radial?.mode === 'partition'
  const rowsPerKey = new Map<string, number>()
  if (partition) {
    for (let index = 0; index < frame.rows.length; index += 1) {
      const key = legendKey(frame, panel, index)
      rowsPerKey.set(key, (rowsPerKey.get(key) ?? 0) + 1)
    }
  }
  const suffixFor = (index: number): 'percent' | 'value' | 'none' => {
    if (seriesLegendIndex >= 0) return 'value'
    if (showsPercent) return 'percent'
    if (!partition) return 'value'
    return rowsPerKey.get(legendKey(frame, panel, index)) === 1 ? 'percent' : 'none'
  }

  const valueFor = (index: number): number | undefined => {
    if (valueIndex < 0) return undefined
    if (seriesLegendIndex < 0) return numericCell(frame.rows[index]?.[valueIndex])
    const key = legendEntryKey(frame, panel, index)
    return frame.rows.reduce((sum, row, rowIndex) => (
      legendEntryKey(frame, panel, rowIndex) === key ? sum + (numericCell(row[valueIndex]) ?? 0) : sum
    ), 0)
  }
  const normalizedSearch = search.trim().toLocaleLowerCase()
  const visibleEntries = normalizedSearch === '' ? entries : entries.filter((index) => {
    const raw = frame.rows[index]?.[labelIndex]
    return textCell(raw).toLocaleLowerCase().includes(normalizedSearch)
  })
  const solo = (key: string) => {
    const alreadySolo = !hidden.has(key) && allKeys.every((candidate) => candidate === key || hidden.has(candidate))
    onSetHidden(alreadySolo ? new Set() : new Set(allKeys.filter((candidate) => candidate !== key)))
  }

  return (
    <div className="lens-chart-legend-shell">
      <div className="lens-chart-legend-tools" role="toolbar" aria-label={translate('chart.legendControls', 'Legend controls')}>
        <button onClick={() => onSetHidden(new Set(allKeys))} type="button">{translate('chart.legendHideAll', 'Hide all')}</button>
        <button onClick={() => onSetHidden(new Set())} type="button">{translate('chart.legendShowAll', 'Show all')}</button>
        <button onClick={() => onSetHidden(new Set(allKeys.filter((key) => !hidden.has(key))))} type="button">{translate('chart.legendInvert', 'Invert')}</button>
      </div>
      {entries.length > 8 && (
        <label className="lens-chart-legend-search">
          <span className="lens-sr-only">{translate('chart.legendSearch', 'Search legend')}</span>
          <input
            onChange={(event) => setSearch(event.currentTarget.value)}
            placeholder={translate('chart.legendSearch', 'Search legend')}
            type="search"
            value={search}
          />
        </label>
      )}
      <ul className="lens-chart-legend">
      {visibleEntries.map((index) => {
        const row = frame.rows[index]!
        const entryIndex = entries.indexOf(index)
        const raw = row[labelIndex]
        const label = typeof raw === 'string' ? raw : raw === null || raw === undefined ? '' : JSON.stringify(raw)
        const key = legendEntryKey(frame, panel, index)
        const isHidden = hidden.has(key)
        return (
          <li className="lens-chart-legend-item" key={`${key}-${index}`}>
            <button
              aria-pressed={!isHidden}
              className={`lens-chart-legend-toggle${isHidden ? ' lens-chart-legend-hidden' : ''}`}
              onClick={() => onToggle(key)}
              onDoubleClick={() => solo(key)}
              title={translate('chart.legendToggle', 'Toggle series; double-click to isolate')}
              type="button"
            >
              <span
                aria-hidden="true"
                className="lens-chart-legend-mark"
                style={{ background: isHidden ? undefined : swatch(label, index, entryIndex) }}
              />
              <span className="lens-chart-legend-label">{label}</span>
              {suffixFor(index) !== 'none' && (
                <>
                  <span aria-hidden="true" className="lens-chart-legend-separator">·</span>
                  <span className="lens-chart-legend-value">
                    {suffixFor(index) === 'percent'
                      ? formatShare(shares.get(index))
                      : formatValue(valueFor(index))}
                  </span>
                </>
              )}
            </button>
          </li>
        )
      })}
      </ul>
    </div>
  )
}

export function PiePanel(props: ChartPanelProps) {
  return <ChartPanel {...props} />
}

export function BarPanel(props: ChartPanelProps) {
  return <ChartPanel {...props} />
}

export function LinePanel(props: ChartPanelProps) {
  return <ChartPanel {...props} />
}

export function DistributionPanel(props: ChartPanelProps) {
  return <ChartPanel {...props} />
}
