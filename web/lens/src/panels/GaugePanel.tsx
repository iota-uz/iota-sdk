/* eslint-disable react/no-unknown-property -- Solid JSX uses `class`, the React-era rule expects `className`; the lint config migrates with the Solid port. */
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

export function GaugePanel(props: GaugePanelProps) {
  const panel = props.panel
  const frame = usePanelFrame(panel.id)
  const translate = useTranslate()
  const valueField = panel.encoding.value
  const valueIndex = frame.data?.columns.findIndex((column) => column.name === valueField) ?? -1
  const value = () => valueIndex >= 0 ? numericValue(frame.data?.rows[0]?.[valueIndex]) : undefined
  const formatValue = useFormat(valueField ? panel.format[valueField] : undefined)
  const maximum = panel.radial?.max ?? 1
  const clamped = () => Math.max(0, Math.min(maximum, value() ?? 0))
  const percent = () => maximum > 0 ? clamped() / maximum * 100 : 0
  const formattedValue = () => formatValue(value())
  const formattedMaximum = formatValue(maximum)

  return (
    <PanelFrame panel={panel} frame={frame}>
      <div
        aria-label={translate('chart.gaugeValue', '{name} value', { name: panel.title })}
        aria-valuemax={maximum}
        aria-valuemin={0}
        aria-valuenow={clamped()}
        aria-valuetext={formattedValue()}
        class="lens-gauge"
        role="meter"
      >
        <svg aria-hidden="true" class="lens-gauge-arc" viewBox="0 0 120 70">
          <path class="lens-gauge-track" d="M10 60 A50 50 0 0 1 110 60" pathLength="100" />
          <path class="lens-gauge-value-arc" d="M10 60 A50 50 0 0 1 110 60" pathLength="100" style={{ 'stroke-dasharray': `${percent()} 100` }} />
        </svg>
        <div class="lens-gauge-reading">
          <strong>{formattedValue()}</strong>
          <span>{translate('chart.gaugeRange', 'of {maximum}', { maximum: formattedMaximum })}</span>
        </div>
        <div aria-hidden="true" class="lens-gauge-scale"><span>{formatValue(0)}</span><span>{formattedMaximum}</span></div>
      </div>
    </PanelFrame>
  )
}
