import { useCallback, useMemo, useState } from 'react'
import type { NodeKey, Panel } from '../contract'
import type { ChartAdapter, ChartFormatResolver, ChartKind } from '../charts/adapter'
import { useDashboard, useDrill, useFormat, usePanelFrame } from '../runtime'
import { ChartHost } from './ChartHost'
import { encodingRoles } from './data'
import { PanelFrame } from './PanelFrame'

export interface ChartPanelProps {
  panel: Panel
  adapter?: ChartAdapter
}

function useChartFormat(panel: Panel): ChartFormatResolver {
  const fallback = useFormat()
  const label = useFormat(panel.encoding.label ? panel.format[panel.encoding.label] : undefined)
  const value = useFormat(panel.encoding.value ? panel.format[panel.encoding.value] : undefined)
  const id = useFormat(panel.encoding.id ? panel.format[panel.encoding.id] : undefined)
  const series = useFormat(panel.encoding.series ? panel.format[panel.encoding.series] : undefined)
  const category = useFormat(panel.encoding.category ? panel.format[panel.encoding.category] : undefined)
  const cut = useFormat(panel.encoding.cut ? panel.format[panel.encoding.cut] : undefined)
  const cutLabel = useFormat(panel.encoding.cutLabel ? panel.format[panel.encoding.cutLabel] : undefined)
  const final = useFormat(panel.encoding.final ? panel.format[panel.encoding.final] : undefined)

  return useMemo(() => {
    const formatters = { label, value, id, series, category, cut, cutLabel, final }
    const byField = new Map<string, (input: unknown) => string>()
    for (const role of encodingRoles) {
      const field = panel.encoding[role]
      if (field) byField.set(field, formatters[role])
    }
    return (field: string, input: unknown) => (byField.get(field) ?? fallback)(input)
  }, [category, cut, cutLabel, fallback, final, id, label, panel.encoding, series, value])
}

export function ChartPanel({ panel, adapter }: ChartPanelProps) {
  const frame = usePanelFrame(panel.id)
  const { document } = useDashboard()
  const { drillInto } = useDrill()
  const format = useChartFormat(panel)
  const [selectedKey, setSelectedKey] = useState<NodeKey>()
  const [hoveredKey, setHoveredKey] = useState<NodeKey | null>(null)
  const drillable = Boolean(panel.drillRoot)
  const kind = panel.kind as ChartKind
  const input = useMemo(() => frame.data ? ({
    kind,
    frame: frame.data,
    encoding: panel.encoding,
    format,
    theme: document.theme,
    selectedKey,
  }) : undefined, [document.theme, format, frame.data, kind, panel.encoding, selectedKey])
  const select = useCallback((key: NodeKey) => {
    if (!drillable) return
    setSelectedKey(key)
    drillInto(key, panel.id)
  }, [drillInto, drillable, panel.id])

  return (
    <PanelFrame panel={panel} frame={frame}>
      {input && (
        <ChartHost
          input={input}
          adapter={adapter}
          label={`${panel.title} ${kind} chart`}
          drillable={drillable}
          onSelect={drillable ? select : undefined}
          onHover={drillable ? setHoveredKey : undefined}
        />
      )}
      {drillable && hoveredKey && <span className="lens-chart-drill-hint" role="status">Select to explore</span>}
    </PanelFrame>
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
