const seriesMarkPrefix = '__lens_series__:'

/**
 * Stable identity for a chart row when the producer has no explicit id field.
 * A multi-series category has one mark per series, so the category alone is
 * ambiguous (and used to make actionable stacked bars emit no selection at
 * all). The prefix keeps this fallback disjoint from ordinary producer ids.
 */
export function fallbackMarkKey(category: string, series: string): string | undefined {
  if (!category && !series) return undefined
  if (!series) return category || undefined
  return `${seriesMarkPrefix}${JSON.stringify([category, series])}`
}

/** Unformatted scalar identity shared by marks and their printed evidence. */
export function markCellText(value: unknown): string {
  if (value === null || value === undefined) return ''
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint') return String(value)
  return ''
}
