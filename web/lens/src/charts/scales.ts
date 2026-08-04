import type { Encoding, Frame, ValueAxis } from '../contract'

function finiteNumber(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value !== 'string' || value.trim() === '') return undefined
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : undefined
}

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
export function linearScaleObscuresValues(frame: Frame, encoding: Encoding, threshold?: number): boolean {
  if (threshold === undefined) return false
  const values = frameValues(frame, encoding)
  if (!values || values.length < 3) return false
  const magnitudes = values.map((value) => Math.abs(value)).filter((value) => value > 0)
  if (magnitudes.length < 2) return false
  const minimum = Math.min(...magnitudes)
  const maximum = Math.max(...magnitudes)
  return minimum > 0 && maximum / minimum >= threshold
}

/**
 * A logarithmic axis earns its warning only when it materially helps.
 *
 * Fewer than three categories are better read as compact values, and a spread
 * below the producer's configured spread threshold is clearer on an ordinary
 * linear axis. With no threshold, SDK applies no product policy and honours a
 * valid log request. Log axes cannot represent zero or negative values, so
 * those frames always fall back without claiming a logarithmic treatment.
 */
export function shouldUseLogarithmicScale(frame: Frame, encoding: Encoding, axis?: ValueAxis, threshold?: number): boolean {
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
  return threshold === undefined || maximum / minimum >= threshold
}
