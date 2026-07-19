import type { Frame, Panel } from '../contract'
import { useFormat, usePanelFrame } from '../runtime'
import { columnIndex, displayText, panelField } from './data'
import { PanelFrame } from './PanelFrame'

/* eslint-disable react-refresh/only-export-components */

const widthFloor = 2

function numeric(value: unknown): number {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim()) {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return 0
}

function boolean(value: unknown): boolean {
  if (typeof value === 'boolean') return value
  if (typeof value === 'number') return value !== 0
  return typeof value === 'string' && value.trim().toLowerCase() === 'true'
}

function signedCut(value: number, format: (value: unknown) => string): string {
  if (value > 0) return `−${format(value)}`
  if (value < 0) return `+${format(-value)}`
  return format(0)
}

export interface CascadeStage {
  label: string
  value: number
  formattedValue: string
  cut: number
  formattedCut: string
  cutLabel: string
  final: boolean
  width: number
}

export function buildCascadeStages(
  panel: Panel,
  frame: Frame,
  formatValue: (value: unknown) => string,
  formatCut: (value: unknown) => string,
): CascadeStage[] {
  const labelField = panelField(panel, 'label') ?? 'label'
  const valueField = panelField(panel, 'value') ?? 'value'
  const cutField = panelField(panel, 'cut') ?? 'cut'
  const cutLabelField = panelField(panel, 'cutLabel') ?? 'cutLabel'
  const finalField = panelField(panel, 'final') ?? 'final'
  const valueIndex = columnIndex(frame, valueField)
  const maximum = Math.max(1, ...frame.rows.map((row) => Math.max(0, numeric(row[valueIndex]))))

  return frame.rows.map((row, index) => {
    const value = numeric(row[valueIndex])
    const cut = numeric(row[columnIndex(frame, cutField)])
    const rawWidth = value > 0 ? Math.min(100, value / maximum * 100) : 0
    return {
      label: displayText(row[columnIndex(frame, labelField)], `Stage ${index + 1}`),
      value,
      formattedValue: formatValue(value),
      cut,
      formattedCut: signedCut(cut, formatCut),
      cutLabel: displayText(row[columnIndex(frame, cutLabelField)], ''),
      final: boolean(row[columnIndex(frame, finalField)]),
      width: rawWidth > 0 ? Math.max(widthFloor, rawWidth) : 0,
    }
  })
}

export interface CascadePanelProps {
  panel: Panel
}

export function CascadePanel({ panel }: CascadePanelProps) {
  const frame = usePanelFrame(panel.id)
  const valueField = panelField(panel, 'value') ?? 'value'
  const cutField = panelField(panel, 'cut') ?? 'cut'
  const formatValue = useFormat(panel.format[valueField])
  const formatCut = useFormat(panel.format[cutField] ?? panel.format[valueField])
  const stages = frame.data ? buildCascadeStages(panel, frame.data, formatValue, formatCut) : []

  return (
    <PanelFrame panel={panel} frame={frame}>
      <div className="lens-cascade" role="list" aria-label={`${panel.title} stages`}>
        {stages.map((stage, index) => (
          <div className="lens-cascade-step" key={`${stage.label}-${index}`} role="listitem">
            {index > 0 && stage.cutLabel && (
              <div className="lens-cascade-connector">
                <span>{stage.cutLabel}</span>
                <strong data-direction={stage.cut > 0 ? 'down' : stage.cut < 0 ? 'up' : 'flat'}>{stage.formattedCut}</strong>
              </div>
            )}
            <div className={`lens-cascade-stage${stage.final ? ' lens-cascade-stage-final' : ''}`} data-final={stage.final || undefined}>
              <div className="lens-cascade-stage-label">
                <span>{stage.label}</span>
                <strong data-negative={stage.value < 0 || undefined}>{stage.formattedValue}</strong>
              </div>
              <div className="lens-cascade-track" aria-hidden="true">
                <span style={{ width: `${stage.width}%` }} />
              </div>
            </div>
          </div>
        ))}
      </div>
    </PanelFrame>
  )
}
