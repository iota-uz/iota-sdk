/* eslint-disable react-refresh/only-export-components */
import type { Frame, NodeKey, Panel } from '../contract'
import { radialNodeKey, type ChartFormatResolver } from '../charts/adapter'
import { fallbackMarkKey } from '../charts/keys'

interface ChartDataEquivalentProps {
  actionable: boolean
  format: ChartFormatResolver
  frame: Frame
  label: string
  onSelect: (key: NodeKey) => void
  panel: Panel
  translate: (key: string, fallback: string, variables?: Record<string, string | number>) => string
}

function textCell(value: unknown): string {
  return typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint'
    ? String(value)
    : ''
}

export function chartRowKey(frame: Frame, panel: Panel, index: number): NodeKey | undefined {
  const row = frame.rows[index]
  if (!row) return undefined
  const at = (field: string | undefined) => {
    const column = frame.columns.findIndex(({ name }) => name === field)
    return column >= 0 ? textCell(row[column]) : ''
  }
  const id = at(panel.encoding.id)
  const category = at(panel.encoding.category) || at(panel.encoding.label)
  const series = at(panel.encoding.series)
  if (panel.radial?.mode === 'partition') return radialNodeKey(series, id || category)
  return id || fallbackMarkKey(category, series)
}

/** A concise, formatted description of exactly one mark in the rendered frame. */
export function chartDatumLabel(frame: Frame, panel: Panel, format: ChartFormatResolver, index: number): string {
  const row = frame.rows[index]
  if (!row) return ''
  const categoryField = [panel.encoding.category, panel.encoding.label]
    .find((field) => Boolean(field) && frame.columns.some(({ name }) => name === field))
  const fields = [
    { field: categoryField, formatted: false },
    { field: panel.encoding.series, formatted: false },
    { field: panel.encoding.value, formatted: true },
  ].filter((entry): entry is { field: string; formatted: boolean } => Boolean(entry.field))
  return fields.flatMap(({ field, formatted }) => {
    const column = frame.columns.findIndex(({ name }) => name === field)
    if (column < 0) return []
    const value = formatted || panel.format[field] ? format(field, row[column]) : textCell(row[column])
    return value ? [value] : []
  }).join(', ')
}

/**
 * Screen-reader data equivalent for the canvas. Actionable marks use the same
 * NodeKey and selection callback as pointer activation; inert marks remain
 * readable without adding stops to the keyboard tab order.
 */
export function ChartDataEquivalent({ actionable, format, frame, label, onSelect, panel, translate }: ChartDataEquivalentProps) {
  return (
    <div aria-label={label} className="lens-chart-keyboard-actions" role="list">
      {frame.rows.map((_, index) => {
        const key = chartRowKey(frame, panel, index)
        const datum = chartDatumLabel(frame, panel, format, index) || String(key ?? index + 1)
        return (
          <div key={`${String(key)}-${index}`} role="listitem">
            {actionable && key !== undefined ? (
              <button onClick={() => onSelect(key)} type="button">
                {translate('chart.openMark', 'Open {name}', { name: datum })}
              </button>
            ) : (
              <span className="lens-sr-only">{datum}</span>
            )}
          </div>
        )
      })}
    </div>
  )
}
