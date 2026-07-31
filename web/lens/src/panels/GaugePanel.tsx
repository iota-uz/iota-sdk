import type { Panel } from '../contract'
import { useFormat, usePanelFrame, useTranslate } from '../runtime'
import { PanelFrame } from './PanelFrame'

export interface GaugePanelProps {
  panel: Panel
}

function numericValue(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value !== 'string' || value.trim() === '') return undefined
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : undefined
}

export function GaugePanel({ panel }: GaugePanelProps) {
  const frame = usePanelFrame(panel.id)
  const translate = useTranslate()
  const valueField = panel.encoding.value
  const valueIndex = frame.data?.columns.findIndex((column) => column.name === valueField) ?? -1
  const value = valueIndex >= 0 ? numericValue(frame.data?.rows[0]?.[valueIndex]) : undefined
  const formatValue = useFormat(valueField ? panel.format[valueField] : undefined)
  const percent = Math.max(0, Math.min(100, value ?? 0))

  return (
    <PanelFrame panel={panel} frame={frame}>
      <div className="lens-gauge" role="meter" aria-label={`${panel.title} value`} aria-valuemin={0} aria-valuemax={100} aria-valuenow={value}>
        <svg aria-hidden="true" className="lens-gauge-arc" viewBox="0 0 120 70">
          <path className="lens-gauge-track" d="M10 60 A50 50 0 0 1 110 60" pathLength="100" />
          <path className="lens-gauge-value-arc" d="M10 60 A50 50 0 0 1 110 60" pathLength="100" style={{ strokeDasharray: `${percent} 100` }} />
        </svg>
        <div className="lens-gauge-reading">
          <strong>{formatValue(value)}</strong>
          <span>{translate('chart.gaugeRange', 'of 100')}</span>
        </div>
        <div aria-hidden="true" className="lens-gauge-scale"><span>0</span><span>100</span></div>
      </div>
    </PanelFrame>
  )
}
