import type { DashboardDocument, Filter, PeriodFilter, PeriodValue } from '../contract'
import { parseISODate } from '../controls/calendar'

/**
 * Filter values are URL state: a map from a declared parameter name to the
 * string the document endpoint receives. An empty string is meaningful — it
 * is the present-but-empty "all time" form — which is why absence and ''
 * are distinct.
 *
 * The runtime can only ever request values this module has validated against
 * the document's own declaration (wire dates, allowEmpty), so the document
 * endpoint — which normalizes and derives the snapshot key server-side —
 * remains the only authority over what a filter combination means.
 */
export type FilterValues = Record<string, string>

export function declaredFilters(document: DashboardDocument): Array<Filter> {
  return (document.filters ?? []).filter((filter) => filter.kind === 'period' && filter.period)
}

export function filterParamNames(document: DashboardDocument): Array<string> {
  const names: Array<string> = []
  for (const filter of declaredFilters(document)) {
    if (!filter.period) continue
    names.push(filter.period.startParam, filter.period.endParam)
  }
  return names
}

function validBoundary(period: PeriodFilter, raw: string): boolean {
  if (raw === '') return Boolean(period.allowEmpty)
  const parsed = parseISODate(raw)
  if (!parsed) return false
  if (period.min && raw < period.min) return false
  if (period.max && raw > period.max) return false
  return true
}

/**
 * The declared filter values present on a URL. A boundary that fails
 * declaration-level validation drops its whole filter back to the document's
 * server-normalized default rather than sending garbage to the endpoint.
 */
export function readFilterValues(document: DashboardDocument, url: URL): FilterValues {
  const values: FilterValues = {}
  for (const filter of declaredFilters(document)) {
    const period = filter.period
    if (!period) continue
    const start = url.searchParams.get(period.startParam)
    const end = url.searchParams.get(period.endParam)
    if (start === null || end === null) continue
    if (!validBoundary(period, start) || !validBoundary(period, end)) continue
    if (start !== '' && end !== '' && end < start) continue
    values[period.startParam] = start
    values[period.endParam] = end
  }
  return values
}

/** Rewrites the declared filter parameters on a URL, leaving all others. */
export function writeFilterValues(url: URL, document: DashboardDocument, values: FilterValues): URL {
  const next = new URL(url)
  for (const name of filterParamNames(document)) {
    next.searchParams.delete(name)
    const value = values[name]
    if (value !== undefined) next.searchParams.set(name, value)
  }
  return next
}

export function sameFilterValues(left: FilterValues, right: FilterValues): boolean {
  const leftKeys = Object.keys(left)
  if (leftKeys.length !== Object.keys(right).length) return false
  return leftKeys.every((key) => right[key] === left[key])
}

/**
 * The selection a period control renders: the URL's validated value when one
 * is present, otherwise the server-normalized value the document declared.
 */
export function currentPeriodValue(period: PeriodFilter, values: FilterValues): PeriodValue {
  const start = values[period.startParam]
  const end = values[period.endParam]
  if (start !== undefined && end !== undefined) return { start, end }
  return period.value
}

export function periodValues(period: PeriodFilter, value: PeriodValue): FilterValues {
  return { [period.startParam]: value.start, [period.endParam]: value.end }
}

/**
 * The document src with filter parameters applied: every parameter the
 * runtime has ever driven is removed, then the current values are set. The
 * host page's own src params survive untouched, so a src that already carries
 * the initial filter (as EAI's does) stays authoritative until the user acts.
 */
export function srcWithFilterParams(src: string, drivenParams: Iterable<string>, values: FilterValues | undefined): string {
  const base = typeof window === 'undefined' ? 'http://localhost/' : window.location.href
  const url = new URL(src, base)
  for (const name of drivenParams) url.searchParams.delete(name)
  for (const [name, value] of Object.entries(values ?? {})) url.searchParams.set(name, value)
  // Keep the src's original credential scope: emit a same-origin relative URL.
  return `${url.pathname}${url.search}${url.hash}`
}
