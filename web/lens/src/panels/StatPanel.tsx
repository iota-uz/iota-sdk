import type { Panel } from '../contract'
import { useFormat, usePanelFrame } from '../runtime'
import { cell, displayText, panelField } from './data'
import { PanelFrame } from './PanelFrame'

export interface StatPanelProps {
  panel: Panel
}

function numeric(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim()) {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return undefined
}

export function StatPanel({ panel }: StatPanelProps) {
  const frame = usePanelFrame(panel.id)
  const valueField = panelField(panel, 'value')
  const deltaField = panelField(panel, 'final')
  const formatValue = useFormat(valueField ? panel.format[valueField] : undefined)
  const formatDelta = useFormat(deltaField ? panel.format[deltaField] : undefined)
  const label = displayText(cell(frame.data, panelField(panel, 'label')), panel.title)
  const value = cell(frame.data, valueField)
  const delta = deltaField ? cell(frame.data, deltaField) : undefined
  const deltaNumber = numeric(delta)

  return (
    <PanelFrame panel={panel} frame={frame} variant="stat">
      <div className="lens-stat-content">
        <p className="lens-stat-label">{label}</p>
        <div className="lens-stat-value-row">
          <p className="lens-stat-value">{formatValue(value)}</p>
          {delta !== undefined && (
            <span className={`lens-stat-delta${deltaNumber !== undefined && deltaNumber < 0 ? ' lens-stat-delta-negative' : ''}`}>
              {deltaNumber !== undefined && deltaNumber > 0 ? '+' : ''}{formatDelta(delta)}
            </span>
          )}
        </div>
      </div>
    </PanelFrame>
  )
}
