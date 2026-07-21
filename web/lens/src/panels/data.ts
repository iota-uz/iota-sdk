import type { Encoding, Frame, Panel, Theme } from '../contract'

const fallbackSeries = ['#2563eb', '#059669', '#d97706', '#7c3aed', '#0891b2', '#dc2626']

/**
 * Resolves a datum's color the way the chart adapter does, so a React-rendered
 * legend cannot disagree with the plot: the panel's own `panelId:index` series
 * entry first, then a series entry keyed by label, then the palette order, then
 * the panel accent.
 *
 * `positional: false` drops the `panelId:index` entry. That entry pins a color
 * to the n-th row of the panel's *own* frame, so once the panel is showing a
 * drill level it describes rows that are no longer on screen — the legend was
 * printing the root's second color next to the level's second slice, which the
 * plot (which only ever resolves by label) had drawn in a different color.
 */
export function seriesColorResolver(
  theme: Theme,
  panel: Panel,
  { positional = true }: { positional?: boolean } = {},
): (label: string, index: number) => string | undefined {
  const palette = Object.values(theme.palette).filter((color) => color.trim() !== '')
  const colors = palette.length > 0 ? palette : fallbackSeries
  const resolve = (value: string | undefined) => (value ? theme.palette[value] ?? value : undefined)
  return (label, index) => (positional ? resolve(theme.series[`${panel.id}:${index}`]) : undefined)
    ?? resolve(theme.series[label])
    ?? colors[index % colors.length]
    ?? panel.accent
}

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
