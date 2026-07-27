import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { Frame, NodeKey, Panel } from '../contract'
import { radialNodeKey, type ChartAdapter, type ChartAnchor, type ChartFormatResolver, type ChartKind } from '../charts/adapter'
import { distributeShares, formatShare } from '../charts/shares'
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
      // The metric-panel roles (share/confidence/availability) have no chart
      // formatter; a chart never carries them, but the guard keeps the shared
      // encodingRoles list usable here.
      const formatter = formatters[role as keyof typeof formatters]
      if (field && formatter) byField.set(field, formatter)
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

  // Hidden series are removed from the data rather than dimmed, so the plot
  // reclaims their arc. The percentages do not follow: they are measured
  // against the frame's authoritative total, so hiding one slice no longer
  // rewrites the numbers on every slice that stayed.
  const visibleFrame = useMemo(() => {
    if (!frame.data || hidden.size === 0) return frame.data
    const rows = frame.data.rows.filter((_, index) => !hidden.has(legendKey(frame.data!, panel, index)))
    return { ...frame.data, rows }
  }, [frame.data, hidden, panel])

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
    const valueIndex = frame.data.columns.findIndex((column) => column.name === panel.encoding.value)
    if (valueIndex < 0) return undefined
    return frame.data.rows.reduce((sum, row) => sum + (numericCell(row[valueIndex]) ?? 0), 0)
  }, [active, frame.data, panel.encoding.value])

  // The whole every share on this panel is measured against. A producer-shipped
  // frame total outranks anything derived here — including `visibleTotal` — so
  // the badge cannot disagree with the percentages printed next to it, and
  // neither moves when a legend entry is toggled off.
  const shareTotal = frame.data?.total ?? visibleTotal ?? levelTotal ?? panel.total

  const toggleSeries = useCallback((key: string) => {
    setHidden((current) => {
      const next = new Set(current)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }, [])

  // Per-mark drill affordance. Only meaningful once a level is on screen and
  // its children are known; without a level every mark is equally (un)expandable
  // and the chart-wide treatment already says so.
  const expandable = useMemo(() => {
    if (!level) return undefined
    return (key: string) => Boolean(childForSelection(level, key)?.target)
  }, [level])

  const input = useMemo(() => visibleFrame ? ({
    kind,
    frame: visibleFrame,
    encoding: panel.encoding,
    format,
    formatAxis,
    theme: document.theme,
    selectedKey,
    presentation: panel.presentation,
    radial: panel.radial,
    expandable,
  }) : undefined, [document.theme, expandable, format, formatAxis, kind, panel.encoding, panel.presentation, panel.radial, selectedKey, visibleFrame])
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
    <PanelFrame panel={panel} frame={frame} total={shareTotal}>
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
        {panel.presentation?.totalBadge === 'plot' && shareTotal !== undefined && (
          <PlotTotalBadge panel={panel} total={shareTotal} />
        )}
        {hasLegend && frame.data && (
          <ChartLegend frame={frame.data} hidden={hidden} onToggle={toggleSeries} panel={panel} total={shareTotal} />
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
 * buttons: clicking one drops that series from the plot.
 *
 * With `legendValue: 'percent'` the suffix is the share instead, which is the
 * half of the pairing that lets a slice carry its own category label. Shares
 * are measured against `total` — the same authoritative whole the plot uses —
 * so hiding an entry no longer moves the numbers on the entries left behind.
 */
function ChartLegend({ panel, frame, hidden, onToggle, total }: {
  panel: Panel
  frame: Frame
  hidden: ReadonlySet<string>
  onToggle: (key: string) => void
  total?: number
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
  const entries = panel.radial?.mode === 'partition'
    ? frame.rows.reduce<number[]>((indices, _, index) => {
        const key = legendKey(frame, panel, index)
        if (!indices.some((candidate) => legendKey(frame, panel, candidate) === key)) indices.push(index)
        return indices
      }, [])
    : frame.rows.map((_, index) => index)
  const visibleEntryCount = entries.filter((index) => !hidden.has(legendKey(frame, panel, index))).length

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
  const shares = new Map<number, number | undefined>()
  const groups = new Map<number | undefined, number[]>()
  for (let index = 0; index < frame.rows.length; index += 1) {
    const denominator = denominatorFor(index)
    const group = groups.get(denominator) ?? []
    group.push(index)
    groups.set(denominator, group)
  }
  for (const [denominator, indices] of groups) {
    const values = indices.map((index) => (valueIndex >= 0 ? numericCell(frame.rows[index]![valueIndex]) : undefined))
    distributeShares(values, denominator).forEach((share, position) => shares.set(indices[position]!, share))
  }
  // A percent legend suppresses nothing: it is the only place the share of a
  // slice labelled with its own category name is written down.
  const showsPercent = panel.presentation?.legendValue === 'percent'
  const showsSuffix = showsPercent || panel.radial?.mode !== 'partition'

  return (
    <ul className="lens-chart-legend">
      {entries.map((index) => {
        const row = frame.rows[index]!
        const raw = row[labelIndex]
        const label = typeof raw === 'string' ? raw : raw === null || raw === undefined ? '' : JSON.stringify(raw)
        const key = legendKey(frame, panel, index)
        const isHidden = hidden.has(key)
        // Hiding the last visible series would leave an empty plot with no way
        // back except guessing, so the final entry stays locked on.
        const locked = !isHidden && (panel.radial?.mode === 'partition' ? visibleEntryCount : visibleCount) <= 1
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
              {showsSuffix && (
                <>
                  <span aria-hidden="true" className="lens-chart-legend-separator">·</span>
                  <span className="lens-chart-legend-value">
                    {showsPercent
                      ? formatShare(shares.get(index))
                      : (valueIndex >= 0 ? formatValue(row[valueIndex]) : '')}
                  </span>
                </>
              )}
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
