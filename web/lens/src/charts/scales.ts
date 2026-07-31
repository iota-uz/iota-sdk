import type { Encoding, Frame, ValueAxis } from '../contract'

function finiteNumber(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value !== 'string' || value.trim() === '') return undefined
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : undefined
}

/**
 * A logarithmic axis earns its warning only when it materially helps.
 *
 * Fewer than three categories are better read as compact values, and a spread
 * below two orders of magnitude is clearer on an ordinary linear axis. Log
 * axes also cannot represent zero or negative values, so those frames always
 * fall back without claiming a logarithmic treatment.
 */
export function shouldUseLogarithmicScale(frame: Frame, encoding: Encoding, axis?: ValueAxis): boolean {
  if (axis?.scale !== 'logarithmic') return false
  const categoryField = encoding.category ?? encoding.label
  const valueField = encoding.value
  if (!categoryField || !valueField) return false
  const categoryIndex = frame.columns.findIndex((column) => column.name === categoryField)
  const valueIndex = frame.columns.findIndex((column) => column.name === valueField)
  if (categoryIndex < 0 || valueIndex < 0) return false

  const categories = new Set<string>()
  const values: number[] = []
  for (const row of frame.rows) {
    const category = row[categoryIndex]
    if (typeof category === 'string' || typeof category === 'number' || typeof category === 'boolean' || typeof category === 'bigint') {
      categories.add(String(category))
    }
    const value = finiteNumber(row[valueIndex])
    if (value !== undefined) values.push(value)
  }
  if (categories.size < 3 || values.length === 0 || values.some((value) => value <= 0)) return false
  const minimum = Math.min(...values)
  const maximum = Math.max(...values)
  return maximum / minimum >= 100
}
