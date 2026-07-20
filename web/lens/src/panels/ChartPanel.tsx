import { useCallback, useMemo, useState } from 'react'
import type { Frame, NodeKey, Panel } from '../contract'
import type { ChartAdapter, ChartFormatResolver, ChartKind } from '../charts/adapter'
import { childForSelection } from '../explore/model'
import { levelForPath, useAxisFormat, useDashboard, useDrill, useFormat, usePanelFrame, useTranslate } from '../runtime'
import { ChartHost } from './ChartHost'
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

export function ChartPanel({ panel, adapter }: ChartPanelProps) {
  const frame = usePanelFrame(panel.id)
  const { document, navigation } = useDashboard()
  const { drillInto } = useDrill()
  const { format, formatAxis } = useChartFormat(panel)
  const [selectedKey, setSelectedKey] = useState<NodeKey>()
  const [hoveredKey, setHoveredKey] = useState<NodeKey | null>(null)
  const active = navigation.panelId === panel.id && navigation.path.length > 0
  const level = active
    ? levelForPath(document, navigation.path)
    : (panel.drillRoot ? document.drill.edges[panel.drillRoot] : undefined)
  const drillable = level ? level.children.some(({ target }) => target) : Boolean(panel.drillRoot)
  const kind = panel.kind as ChartKind
  const input = useMemo(() => frame.data ? ({
    kind,
    frame: frame.data,
    encoding: panel.encoding,
    format,
    formatAxis,
    theme: document.theme,
    selectedKey,
    presentation: panel.presentation,
  }) : undefined, [document.theme, format, formatAxis, frame.data, kind, panel.encoding, panel.presentation, selectedKey])
  const select = useCallback((key: NodeKey) => {
    if (!drillable) return
    const node = childForSelection(level, key)
    if (level && !node?.target) return
    setSelectedKey(key)
    drillInto(node?.key ?? key, panel.id)
  }, [drillInto, drillable, level, panel.id])

  return (
    <PanelFrame panel={panel} frame={frame}>
      <div className="lens-chart-area">
        {input && (
          <ChartHost
            input={input}
            panelId={panel.id}
            adapter={adapter}
            label={`${panel.title} ${kind} chart`}
            drillable={drillable}
            onSelect={drillable ? select : undefined}
            onHover={drillable ? setHoveredKey : undefined}
          />
        )}
        {panel.presentation?.totalBadge === 'plot' && panel.total !== undefined && (
          <PlotTotalBadge panel={panel} />
        )}
      </div>
      {panel.presentation?.legend === 'below' && frame.data && <ChartLegend panel={panel} frame={frame.data} />}
      {drillable && hoveredKey && <span className="lens-chart-drill-hint" role="status">Select to explore</span>}
    </PanelFrame>
  )
}

function PlotTotalBadge({ panel }: { panel: Panel }) {
  const translate = useTranslate()
  const formatTotal = useFormat(panel.encoding.value ? panel.format[panel.encoding.value] : undefined)
  return (
    <span className="lens-plot-total" title={translate('panel.total', 'Total')}>
      {formatTotal(panel.total)}
    </span>
  )
}

/**
 * A legend below the plot lists `label · value` for every slice, so the values
 * stay readable when the plot itself only carries percentages.
 */
function ChartLegend({ panel, frame }: { panel: Panel; frame: Frame }) {
  const { document } = useDashboard()
  const labelField = panel.encoding.label ?? panel.encoding.category
  const valueField = panel.encoding.value
  const formatValue = useFormat(valueField ? panel.format[valueField] : undefined)
  const labelIndex = frame.columns.findIndex((column) => column.name === labelField)
  const valueIndex = frame.columns.findIndex((column) => column.name === valueField)
  if (labelIndex < 0) return null

  const color = seriesColorResolver(document.theme, panel)
  return (
    <ul className="lens-chart-legend">
      {frame.rows.map((row, index) => {
        const raw = row[labelIndex]
        const label = typeof raw === 'string' ? raw : raw === null || raw === undefined ? '' : JSON.stringify(raw)
        return (
          <li className="lens-chart-legend-item" key={`${label}-${index}`}>
            <span aria-hidden="true" className="lens-chart-legend-mark" style={{ background: color(label, index) }} />
            <span className="lens-chart-legend-label">{label}</span>
            <span aria-hidden="true" className="lens-chart-legend-separator">·</span>
            <span className="lens-chart-legend-value">{valueIndex >= 0 ? formatValue(row[valueIndex]) : ''}</span>
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
