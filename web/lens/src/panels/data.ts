import type { Encoding, Frame, Panel } from '../contract'

export function columnIndex(frame: Frame | undefined, field: string | undefined): number {
  if (!frame || !field?.trim()) return -1
  return frame.columns.findIndex((column) => column.name === field)
}

export function cell(frame: Frame | undefined, field: string | undefined, row = 0): unknown {
  const index = columnIndex(frame, field)
  return index >= 0 ? frame?.rows[row]?.[index] : undefined
}

export function displayText(value: unknown, fallback: string): string {
  if (typeof value === 'string' && value.trim()) return value
  if (typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint') return String(value)
  return fallback
}

export const encodingRoles: ReadonlyArray<keyof Encoding> = [
  'label', 'value', 'id', 'series', 'category', 'cut', 'cutLabel', 'final',
]

export function panelField(panel: Panel, role: keyof Encoding): string | undefined {
  return panel.encoding[role]?.trim() || undefined
}
