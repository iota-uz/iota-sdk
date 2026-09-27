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
