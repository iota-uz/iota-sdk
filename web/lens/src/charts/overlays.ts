import type { PanelTemporal } from '../contract'
import type { ChartLabels } from './adapter'

/**
 * The marks a chart draws that are not rows of its frame.
 *
 * Six of them used to share one grey dashed idiom — a reference line, an event
 * annotation, the forecast boundary, the annualized estimate, the regression
 * and the incomplete-period marker — and none of them could reach the legend,
 * because the legend enumerates rows of the data frame and these are not rows.
 * A reader was left with four indistinguishable grey lines and nowhere to look
 * them up.
 *
 * This module is the single list. The chart adapter draws from it and the
 * panel's legend prints from it, so a mark cannot be drawn in one vocabulary
 * and named in another.
 */
export type OverlayKind =
  /** A declared threshold or target the reader compares the data against. */
  | 'reference'
  /** A dated event that explains a discontinuity in the data. */
  | 'annotation'
  /** The region beyond which values are projected rather than observed. */
  | 'forecast'
  /** The period that has not finished, so its column is not comparable. */
  | 'incomplete'
  /** A full-period estimate derived from that partial period. */
  | 'estimate'
  /** A fitted line: derived from the data, not measured. */
  | 'trend'
  /** A smoothed reading of the same series. */
  | 'average'
  /** The same measure over the comparison period. */
  | 'comparison'

/**
 * How an overlay is stroked. This is the vocabulary itself: one idiom per
 * meaning, shared by the plot and by the legend swatch, so the swatch is a
 * miniature of the mark rather than a coloured square standing in for it.
 */
export type OverlayStroke =
  /** An unbroken hairline running edge to edge: a stated threshold. */
  | 'hairline'
  /** A wide dash: derived from the data by a model. */
  | 'dashed'
  /** A fine dot: an estimate for a period that has not happened yet. */
  | 'dotted'
  /** An unbroken line at data weight: the same data, smoothed. */
  | 'solid'
  /** A tinted region rather than a line: something true of a span, not a value. */
  | 'band'
  /** A hatched region: data that is present but incomplete. */
  | 'hatch'

/**
 * The palette role an overlay paints in. Resolved to a colour by whoever
 * paints it — the ECharts theme in the plot, a CSS custom property in the
 * legend — so the two cannot drift apart.
 */
export type OverlayTone =
  | 'warn'
  | 'accent'
  | 'muted'
  | 'faint'
  | 'trend'
  /** Takes the hue of the data series it belongs to; the legend shows it faint. */
  | 'series'

export interface ChartOverlay {
  /** Stable identity, also the key the reader toggles it by. */
  id: string
  kind: OverlayKind
  /** The producer's own name for it where there is one; a generic one otherwise. */
  label: string
  /** The value or the date the mark stands at, printed after the label. */
  detail?: string
  tone: OverlayTone
  stroke: OverlayStroke
  /** False while the reader has switched it off. The entry stays listed. */
  active: boolean
}

/** The overlay identities, spelled once. */
export const overlayId = {
  annotation: (index: number) => `annotation:${index}`,
  average: (window: number) => `average:${window}`,
  comparison: 'comparison',
  estimate: 'estimate',
  forecast: 'forecast',
  incomplete: 'incomplete',
  reference: (index: number) => `reference:${index}`,
  trend: 'trend',
} as const

export interface OverlayContext {
  /** The panel's declared overlays, unfiltered by what is currently shown. */
  temporal?: PanelTemporal
  /** Whether the panel draws a comparison-period ghost behind its series. */
  hasComparison: boolean
  /** Localized names for the overlays the producer did not name itself. */
  labels?: ChartLabels
  /** Formats a reference-line value in the panel's own units. */
  formatValue: (value: number) => string
  /** Formats a category or timestamp the way the axis prints it. */
  formatCategory: (value: string) => string
  /** Overlays the reader has switched off. */
  hidden?: ReadonlySet<string>
}

function periodLabel(temporal: PanelTemporal, labels?: ChartLabels): string {
  const period = temporal.period
  if (!period) return ''
  if (period.label) return period.label
  // The band marks a period that is still filling in; the dotted line is the
  // estimate of where it lands. When both are on the plot they cannot both be
  // called "Estimate", so the band falls back to naming the partial reading.
  return period.state === 'annualized' && !period.annualizedField
    ? (labels?.estimate ?? 'Estimate')
    : (labels?.ytd ?? 'YTD')
}

/**
 * Every overlay a panel declares, in reading order: first the marks that
 * qualify the data itself (an unfinished period, a comparison ghost), then the
 * ones the producer stated (thresholds, events), then the ones a model derived
 * (trend, average, estimate, forecast). A reader scanning the list meets the
 * caveats before the extrapolations.
 */
export function chartOverlays(context: OverlayContext): ChartOverlay[] {
  const { temporal, labels, formatValue, formatCategory } = context
  const hidden = context.hidden ?? new Set<string>()
  const overlays: ChartOverlay[] = []
  const push = (overlay: Omit<ChartOverlay, 'active'>) => {
    overlays.push({ ...overlay, active: !hidden.has(overlay.id) })
  }

  if (temporal?.period) {
    push({
      id: overlayId.incomplete,
      kind: 'incomplete',
      label: periodLabel(temporal, labels),
      detail: formatCategory(temporal.period.category),
      tone: 'faint',
      stroke: 'hatch',
    })
  }
  if (context.hasComparison) {
    push({
      id: overlayId.comparison,
      kind: 'comparison',
      label: labels?.previous ?? 'Previous',
      tone: 'series',
      stroke: 'dashed',
    })
  }
  for (const [index, reference] of (temporal?.referenceLines ?? []).entries()) {
    const value = formatValue(reference.value)
    push({
      id: overlayId.reference(index),
      kind: 'reference',
      label: reference.label || value,
      // The chip on the plot carries the value, so repeating it beside a
      // label that is already the value would print it twice.
      detail: reference.label ? value : undefined,
      tone: 'warn',
      stroke: 'hairline',
    })
  }
  for (const [index, annotation] of (temporal?.annotations ?? []).entries()) {
    push({
      id: overlayId.annotation(index),
      kind: 'annotation',
      label: annotation.label,
      detail: formatCategory(annotation.at),
      tone: 'accent',
      stroke: 'band',
    })
  }
  if (temporal?.regression) {
    push({
      id: overlayId.trend,
      kind: 'trend',
      label: temporal.regression.label || labels?.trend || 'Trend',
      tone: 'trend',
      stroke: 'dashed',
    })
  }
  for (const average of temporal?.movingAverages ?? []) {
    push({
      id: overlayId.average(average.window),
      kind: 'average',
      label: average.label || labels?.movingAverage(average.window) || `SMA ${average.window}`,
      tone: 'series',
      stroke: 'solid',
    })
  }
  if (temporal?.period?.annualizedField) {
    push({
      id: overlayId.estimate,
      kind: 'estimate',
      label: labels?.estimate ?? 'Estimate',
      detail: formatCategory(temporal.period.category),
      tone: 'muted',
      stroke: 'dotted',
    })
  }
  if (temporal?.forecast) {
    push({
      id: overlayId.forecast,
      kind: 'forecast',
      label: temporal.forecast.label || labels?.forecast || 'Forecast',
      detail: formatCategory(temporal.forecast.start),
      tone: 'series',
      stroke: 'band',
    })
  }
  return overlays
}

/**
 * How an overlay prints the category it stands at.
 *
 * The axis already answers this question, and the answer has to be the same in
 * the legend: a forecast that starts at "Jun 2026" on the axis cannot be listed
 * as starting at "2026-06-01T00:00:00Z". Where the producer declared a format
 * for the category field that format wins; a bare timestamp falls back to the
 * same month/year rendering the time axis uses for its own ticks; and anything
 * else is printed as it stands.
 *
 * That last rule matters more than it looks. A category is an identifier, not a
 * quantity: passing the year "2026" through a generic numeric fallback prints
 * "2 026", which is the same defect the axis tooltip already guards against.
 */
export function categoryDisplayFormatter(context: {
  format: (value: unknown) => string
  isTime: boolean
  hasDeclaredFormat: boolean
  locale?: string
}): (value: string) => string {
  return (value: string) => {
    if (context.hasDeclaredFormat) return context.format(value)
    if (!context.isTime) return value
    const parsed = Date.parse(value)
    if (!Number.isFinite(parsed)) return value
    return new Intl.DateTimeFormat(context.locale, { month: 'short', year: 'numeric', timeZone: 'UTC' })
      .format(new Date(parsed))
  }
}

/** The ids of the overlays currently drawn. */
export function activeOverlayIds(overlays: readonly ChartOverlay[]): Set<string> {
  return new Set(overlays.filter((overlay) => overlay.active).map((overlay) => overlay.id))
}

/**
 * The custom property a tone resolves to in the stylesheet. The plot resolves
 * the same tones through the ECharts theme, which reads these very properties
 * off the dashboard root — one definition, two readers.
 */
export function overlayToneProperty(tone: OverlayTone): string {
  switch (tone) {
    case 'warn': return '--lens-warn'
    case 'accent': return '--lens-accent-500'
    case 'muted': return '--lens-text-muted'
    case 'trend': return '--lens-overlay-trend'
    case 'faint':
    case 'series':
      return '--lens-text-faint'
  }
}
