/**
 * Percent shares for the slices of a whole.
 *
 * Two rules, both of which exist because a dashboard reader adds the numbers
 * up:
 *
 * 1. The denominator is the producer's authoritative total when there is one,
 *    not the sum of the rows on screen. Those differ whenever a tail was
 *    collapsed into an "other" bucket, a non-positive row was dropped, or the
 *    viewer hid a series from the legend — and a share that moves when you
 *    hide an unrelated slice is a share nobody can quote.
 * 2. When the rows genuinely partition that total, the printed shares are
 *    corrected to add up to exactly 100.0%. Independent rounding lands on
 *    99.9% or 100.1% often enough that on a financial dashboard it reads as a
 *    reconciliation error rather than as rounding.
 */

import { writeNumberParts } from '../runtime/format'

/** Decimal places every share is printed with. */
export const sharePrecision = 1

const scale = 10 ** sharePrecision
/**
 * How far the rows' own sum may sit from 100% and still be treated as a
 * partition. Wider than float noise, far narrower than a genuinely missing
 * slice.
 */
const partitionTolerance = 0.05

function isFiniteNumber(value: number | undefined): value is number {
  return value !== undefined && Number.isFinite(value)
}

/** A single share, rounded but never reconciled against its siblings. */
export function shareOf(value: number | undefined, total: number | undefined): number | undefined {
  if (!isFiniteNumber(value) || !isFiniteNumber(total) || total === 0) return undefined
  return Math.round((value / total) * 100 * scale) / scale
}

/**
 * Shares for a whole level at once, reconciled to 100% by largest remainder
 * when the values partition `total`.
 *
 * Largest remainder rather than a fudge on the biggest slice: it moves the
 * correction to whichever slices were rounded down hardest, so no single row
 * absorbs the whole error and the ordering of the printed shares still matches
 * the ordering of the underlying values.
 */
export function distributeShares(
  values: readonly (number | undefined)[],
  total: number | undefined,
): (number | undefined)[] {
  if (!isFiniteNumber(total) || total === 0) return values.map(() => undefined)

  const raw = values.map((value) => (isFiniteNumber(value) ? (value / total) * 100 : undefined))
  const present = raw.filter(isFiniteNumber)
  const partitions =
    present.length === raw.length
    && present.every((share) => share >= 0)
    && Math.abs(present.reduce((sum, share) => sum + share, 0) - 100) <= partitionTolerance
  if (!partitions) return raw.map((share) => (isFiniteNumber(share) ? Math.round(share * scale) / scale : undefined))

  const units = raw.map((share) => Math.floor((share ?? 0) * scale))
  const remainders = raw.map((share, index) => ({ index, fraction: (share ?? 0) * scale - units[index]! }))
  let deficit = 100 * scale - units.reduce((sum, unit) => sum + unit, 0)
  // Ties break on the earlier row so the same data always prints the same
  // shares — a share that flips between renders is worse than one rounded down.
  remainders.sort((left, right) => right.fraction - left.fraction || left.index - right.index)
  for (const { index } of remainders) {
    if (deficit <= 0) break
    units[index] = units[index]! + 1
    deficit -= 1
  }
  return units.map((unit) => unit / scale)
}

/**
 * The printed form of a share, e.g. `8.8%` — and `8,8 %` where the reader's
 * language writes it that way. A report that prints «11.2%» on a chart and
 * «11,2 %» in the table beneath it is one report in two typographies.
 *
 * The percent sign is therefore not concatenated here: the sign, and whatever
 * space the reader's language puts in front of it, come out of the same
 * `unit`/`narrow` formatter and the same `writeNumberParts` writer that every
 * other percent field in the runtime goes through (`format.ts`). Concatenating
 * `%` was exactly how a donut label ended up printing «32,2%» beside a stat
 * card's «32,2 %» on one dashboard.
 */
export function formatShare(share: number | undefined, locale?: string, decimalSeparator?: string): string {
  if (!isFiniteNumber(share)) return ''
  // Rounded exactly as the server's %.1f rounds it, then written in the
  // reader's own notation.
  const parts = new Intl.NumberFormat(locale, {
    style: 'unit',
    unit: 'percent',
    unitDisplay: 'narrow',
    minimumFractionDigits: sharePrecision,
    maximumFractionDigits: sharePrecision,
  }).formatToParts(Number(share.toFixed(sharePrecision)))
  return writeNumberParts(parts, decimalSeparator)
}
