import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { Frame, NodeKey, Panel } from '../contract'
import type { ChartAdapter, ChartAnchor, ChartFormatResolver, ChartKind } from '../charts/adapter'
import { childForSelection } from '../explore/model'
import { levelForPath, useAxisFormat, useDashboard, useDrill, useFormat, usePanelFrame, useTranslate } from '../runtime'
import { usePanelNavigation } from './actions'
import { ChartHost } from './ChartHost'
import { useMarkSelection } from './context'
import { encodingRoles, seriesColorResolver } from './data'
import { PanelFrame } from './PanelFrame'

export interface ChartPanelProps {
  panel: Panel
  adapter?: ChartAdapter
}

function useChartFormat(panel: Panel): { format: ChartFormatResolver; formatAxis: ChartFormatResolver } {
  const fallback = useFormat()
  const label = useFormat(panel.encoding.label ? panel.format[panel.encoding.label] : undefined)
  const value = useFormat(panel.encoding.value ? panel.format[panel.encoding.value] : undefined)
  const id = useFormat(panel.encoding.id ? panel.format[panel.encoding.id] : undefined)
  const series = useFormat(panel.encoding.series ? panel.format[panel.encoding.series] : undefined)
  const category = useFormat(panel.encoding.category ? panel.format[panel.encoding.category] : undefined)
  const cut = useFormat(panel.encoding.cut ? panel.format[panel.encoding.cut] : undefined)
  const cutLabel = useFormat(panel.encoding.cutLabel ? panel.format[panel.encoding.cutLabel] : undefined)
  const final = useFormat(panel.encoding.final ? panel.format[panel.encoding.final] : undefined)
  // Compact axis labels for the value field prevent overlapping full-precision money ticks.
  const valueAxis = useAxisFormat(panel.encoding.value ? panel.format[panel.encoding.value] : undefined)

  const format = useMemo<ChartFormatResolver>(() => {
    const formatters = { label, value, id, series, category, cut, cutLabel, final }
    const byField = new Map<string, (input: unknown) => string>()
    for (const role of encodingRoles) {
      const field = panel.encoding[role]
      if (field) byField.set(field, formatters[role])
    }
    return (field: string, input: unknown) => (byField.get(field) ?? fallback)(input)
  }, [category, cut, cutLabel, fallback, final, id, label, panel.encoding, series, value])

  const formatAxis = useMemo<ChartFormatResolver>(() => {
    const valueField = panel.encoding.value
    return (field: string, input: unknown) => valueField && field === valueField ? valueAxis(input) : format(field, input)
  }, [format, panel.encoding.value, valueAxis])

  return useMemo(() => ({ format, formatAxis }), [format, formatAxis])
}

/**
 * Identity of one legend entry. The id field is stable across re-queries;
 * without one the label is the only thing a row can be recognised by.
 */
export function legendKey(frame: Frame, panel: Panel, index: number): string {
  const idIndex = panel.encoding.id ? frame.columns.findIndex((column) => column.name === panel.encoding.id) : -1
  const labelField = panel.encoding.label ?? panel.encoding.category
  const labelIndex = frame.columns.findIndex((column) => column.name === labelField)
  const raw = idIndex >= 0 ? frame.rows[index]?.[idIndex] : frame.rows[index]?.[labelIndex]
  return typeof raw === 'string' || typeof raw === 'number' || typeof raw === 'bigint' ? String(raw) : String(index)
}

/** Index of the frame row a chart mark key identifies. */
export function rowIndexForKey(frame: Frame, panel: Panel, key: string): number {
  const index = frame.rows.findIndex((_, position) => legendKey(frame, panel, position) === key)
  if (index >= 0) return index
  const labelField = panel.encoding.label ?? panel.encoding.category
  const labelIndex = frame.columns.findIndex((column) => column.name === labelField)
  return labelIndex >= 0 ? frame.rows.findIndex((row) => String(row[labelIndex]) === key) : -1
}

function numericCell(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return undefined
}

export function ChartPanel({ panel, adapter }: ChartPanelProps) {
  const frame = usePanelFrame(panel.id)
  const translate = useTranslate()
  const { document, navigation } = useDashboard()
  const { drillInto } = useDrill()
  const { format, formatAxis } = useChartFormat(panel)
  const [selectedKey, setSelectedKey] = useState<NodeKey>()
  const [hoveredKey, setHoveredKey] = useState<NodeKey | null>(null)
  const [hidden, setHidden] = useState<ReadonlySet<string>>(() => new Set())
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

  // A new level or perspective is new data; carrying hidden keys across would
  // silently blank out unrelated segments, and carrying the selected key would
  // outline whichever mark of the new level happens to share its id — or, more
  // often, none of them, leaving a selection the user cannot see or clear.
  const viewKey = `${navigation.panelId === panel.id ? navigation.path.join('|') : ''}:${navigation.perspectiveId ?? ''}`
  const previousViewKey = useRef(viewKey)
  useEffect(() => {
    if (previousViewKey.current === viewKey) return
    previousViewKey.current = viewKey
    setHidden(new Set())
    setSelectedKey(undefined)
  }, [viewKey])

  // Hidden series are removed from the data, not dimmed: ECharts derives slice
  // percentages from the data it is given, so dimming would leave every label
  // computed against the old total — the recalculation is the whole point.
  const visibleFrame = useMemo(() => {
    if (!frame.data || hidden.size === 0) return frame.data
    const rows = frame.data.rows.filter((_, index) => !hidden.has(legendKey(frame.data!, panel, index)))
    return { ...frame.data, rows }
  }, [frame.data, hidden, panel])

  const visibleTotal = useMemo(() => {
    if (!frame.data || hidden.size === 0 || !panel.encoding.value) return undefined
    const valueIndex = frame.data.columns.findIndex((column) => column.name === panel.encoding.value)
    if (valueIndex < 0) return undefined
    return (visibleFrame?.rows ?? []).reduce((sum, row) => sum + (numericCell(row[valueIndex]) ?? 0), 0)
  }, [frame.data, hidden.size, panel.encoding.value, visibleFrame])

  // `panel.total` is the root frame's total, shipped once with the document. At
  // a drill level the panel is showing the level's frame, so the badge has to
  // total that frame instead — the same rows the slice percentages normalize
  // against — or it prints the root's figure over the level's chart.
  const levelTotal = useMemo(() => {
    if (!active || !frame.data || !panel.encoding.value) return undefined
    const valueIndex = frame.data.columns.findIndex((column) => column.name === panel.encoding.value)
    if (valueIndex < 0) return undefined
    return frame.data.rows.reduce((sum, row) => sum + (numericCell(row[valueIndex]) ?? 0), 0)
  }, [active, frame.data, panel.encoding.value])

  const toggleSeries = useCallback((key: string) => {
    setHidden((current) => {
      const next = new Set(current)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }, [])

  const input = useMemo(() => visibleFrame ? ({
    kind,
    frame: visibleFrame,
    encoding: panel.encoding,
    format,
    formatAxis,
    theme: document.theme,
    selectedKey,
    presentation: panel.presentation,
  }) : undefined, [document.theme, format, formatAxis, kind, panel.encoding, panel.presentation, selectedKey, visibleFrame])
  const onMarkSelect = useMarkSelection()
  // Explore hosts can open the overlay for any segment that has something to
  // show; a standalone tree panel can only drill where a target exists.
  const interactive = hasTree
    ? (onMarkSelect ? Boolean(level?.children.length ?? panel.drillRoot) : drillable)
    : Boolean(panelNavigation.action)
  const select = useCallback((key: NodeKey, anchor?: ChartAnchor) => {
    if (!hasTree) {
      const href = markURL(key)
      panelNavigation.activate(href)
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
  }, [drillInto, hasTree, level, markURL, onMarkSelect, panel.id, panelNavigation])

  // A legend sits to the RIGHT of the plot on a wide panel and drops below it
  // when the panel is too narrow (handled in CSS by a container query). Moving
  // it out of the plot's footer hands the freed width to the chart, which fills
  // the left of the body.
  const hasLegend = panel.presentation?.legend === 'below' && Boolean(frame.data)
  return (
    <PanelFrame panel={panel} frame={frame} total={levelTotal ?? panel.total}>
      <div className={`lens-chart-layout${hasLegend ? ' lens-chart-layout-legend' : ''}`}>
        <div className="lens-chart-area">
          {input && (
            <ChartHost
              input={input}
              panelId={panel.id}
              adapter={adapter}
              label={translate('chart.label', '{name} chart', { name: panel.title })}
              drillable={interactive}
              onSelect={interactive ? select : undefined}
              onHover={interactive ? setHoveredKey : undefined}
            />
          )}
        </div>
        {panel.presentation?.totalBadge === 'plot' && (visibleTotal ?? levelTotal ?? panel.total) !== undefined && (
          <PlotTotalBadge panel={panel} total={(visibleTotal ?? levelTotal ?? panel.total)!} />
        )}
        {hasLegend && frame.data && (
          <ChartLegend frame={frame.data} hidden={hidden} onToggle={toggleSeries} panel={panel} />
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
 * buttons: clicking one drops that series from the plot, which is what makes
 * the remaining percentages re-normalize, exactly like the legacy legend.
 */
function ChartLegend({ panel, frame, hidden, onToggle }: {
  panel: Panel
  frame: Frame
  hidden: ReadonlySet<string>
  onToggle: (key: string) => void
}) {
  const { document, navigation } = useDashboard()
  const translate = useTranslate()
  const labelField = panel.encoding.label ?? panel.encoding.category
  const valueField = panel.encoding.value
  const formatValue = useFormat(valueField ? panel.format[valueField] : undefined)
  const labelIndex = frame.columns.findIndex((column) => column.name === labelField)
  const valueIndex = frame.columns.findIndex((column) => column.name === valueField)
  if (labelIndex < 0) return null

  // At a drill level the rows are the level's, not the panel's own, so the
  // positional color pins no longer describe them.
  const atLevel = navigation.panelId === panel.id && navigation.path.length > 0
  const color = seriesColorResolver(document.theme, panel, { positional: !atLevel })
  const visibleCount = frame.rows.filter((_, index) => !hidden.has(legendKey(frame, panel, index))).length

  return (
    <ul className="lens-chart-legend">
      {frame.rows.map((row, index) => {
        const raw = row[labelIndex]
        const label = typeof raw === 'string' ? raw : raw === null || raw === undefined ? '' : JSON.stringify(raw)
        const key = legendKey(frame, panel, index)
        const isHidden = hidden.has(key)
        // Hiding the last visible series would leave an empty plot with no way
        // back except guessing, so the final entry stays locked on.
        const locked = !isHidden && visibleCount <= 1
        return (
          <li className="lens-chart-legend-item" key={`${key}-${index}`}>
            <button
              aria-pressed={!isHidden}
              className={`lens-chart-legend-toggle${isHidden ? ' lens-chart-legend-hidden' : ''}`}
              disabled={locked}
              onClick={() => onToggle(key)}
              title={locked
                ? translate('chart.legendLast', 'The last visible series cannot be hidden')
                : translate('chart.legendToggle', 'Toggle series')}
              type="button"
            >
              <span
                aria-hidden="true"
                className="lens-chart-legend-mark"
                style={{ background: isHidden ? undefined : color(label, index) }}
              />
              <span className="lens-chart-legend-label">{label}</span>
              <span aria-hidden="true" className="lens-chart-legend-separator">·</span>
              <span className="lens-chart-legend-value">{valueIndex >= 0 ? formatValue(row[valueIndex]) : ''}</span>
            </button>
          </li>
        )
      })}
    </ul>
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
