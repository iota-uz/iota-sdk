import type { Encoding, Frame, ValueAxis } from '../contract'

function finiteNumber(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value !== 'string' || value.trim() === '') return undefined
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : undefined
}

/**
 * The spread at which a linear axis stops being able to show the small
 * readings: two orders of magnitude between the largest value and the smallest.
 *
 * One number, two rules. It is the threshold at which a producer's request for
 * a logarithmic axis is honoured, and — because the data does not care whether
 * the producer asked — the threshold at which a chart left on a linear axis has
 * to state its values in figures instead of leaving eleven near-zero months as
 * eleven identical hairlines. The two treatments differ; the diagnosis is one.
 */
export const obscuringValueSpread = 100

/** Every finite reading in the frame's value column. */
function frameValues(frame: Frame, encoding: Encoding): number[] | undefined {
  const categoryField = encoding.category ?? encoding.label
  const valueField = encoding.value
  if (!categoryField || !valueField) return undefined
  const categoryIndex = frame.columns.findIndex((column) => column.name === categoryField)
  const valueIndex = frame.columns.findIndex((column) => column.name === valueField)
  if (categoryIndex < 0 || valueIndex < 0) return undefined
  return frame.rows.flatMap((row) => {
    const value = finiteNumber(row[valueIndex])
    return value === undefined ? [] : [value]
  })
}

/**
 * True when the values in this frame cannot all be read off a linear axis.
 *
 * A single month at 25 млрд beside eleven months under 100 млн draws one column
 * and eleven baselines: the eleven are not zero, and the chart says they are.
 * Whether the fix is a logarithmic axis (which the producer must ask for, since
 * it changes what a bar length means) or printed figures (which it never does)
 * is decided elsewhere; this is the shared diagnosis both treatments hang off.
 *
 * Zero and negative readings are excluded from the ratio rather than
 * disqualifying the frame: a zero is legible as a zero on a linear axis, and a
 * chart that crosses the baseline is not the degenerate case this describes.
 */
export function linearScaleObscuresValues(frame: Frame, encoding: Encoding): boolean {
  const values = frameValues(frame, encoding)
  if (!values || values.length < 3) return false
  const magnitudes = values.map((value) => Math.abs(value)).filter((value) => value > 0)
  if (magnitudes.length < 2) return false
  const minimum = Math.min(...magnitudes)
  const maximum = Math.max(...magnitudes)
  return minimum > 0 && maximum / minimum >= obscuringValueSpread
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
  // Keep the unit baseline on a linear axis. Although log(1) exists, ECharts
  // can place that bar on the baseline with no visible height, which turns a
  // real count into an apparent zero.
  if (categories.size < 3 || values.length === 0 || values.some((value) => value <= 1)) return false
  const minimum = Math.min(...values)
  const maximum = Math.max(...values)
  return maximum / minimum >= obscuringValueSpread
}
