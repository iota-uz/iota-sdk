import type { Action, CompareValue, DashboardDocument, Filter, PeriodFilter, PeriodValue } from '../contract'
import { parseISODate } from '../controls/model'
import { resolveSourceValue, variablesFromLocation } from '../explore/actions'

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
export type FilterValues = Record<string, string | Array<string>>

export const cubeFilterParam = '_f'
export const cubeGroupByParam = '_groupby'

export function declaredFilters(document: DashboardDocument): Array<Filter> {
  return (document.filters ?? []).filter((filter) =>
    filter.kind === 'period' && filter.period || filter.kind === 'facet' && filter.facet || filter.kind === 'compare' && filter.compare,
  )
}

export function filterParamNames(document: DashboardDocument): Array<string> {
  // Facet dimensions share the validated cube `_f` representation rather than
  // inventing one URL parameter per declaration. Keep both cube parameters in
  // this canonical set so read, write, and stateful-URL detection agree.
  const names: Array<string> = [cubeFilterParam, cubeGroupByParam]
  for (const filter of declaredFilters(document)) {
    if (filter.kind === 'period' && filter.period) names.push(filter.period.startParam, filter.period.endParam)
    if (filter.kind === 'compare' && filter.compare) names.push(filter.compare.modeParam, filter.compare.startParam, filter.compare.endParam)
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
    if (filter.kind !== 'period') continue
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
  const cubeFilters = url.searchParams.getAll(cubeFilterParam)
  if (cubeFilters.length > 0) values[cubeFilterParam] = cubeFilters
  const groupBy = url.searchParams.get(cubeGroupByParam)
  if (groupBy !== null && groupBy !== '') values[cubeGroupByParam] = groupBy
  for (const filter of declaredFilters(document)) {
    if (filter.kind !== 'compare' || !filter.compare) continue
    const mode = url.searchParams.get(filter.compare.modeParam)
    if (mode !== null) values[filter.compare.modeParam] = mode
    const start = url.searchParams.get(filter.compare.startParam)
    const end = url.searchParams.get(filter.compare.endParam)
    if (start !== null) values[filter.compare.startParam] = start
    if (end !== null) values[filter.compare.endParam] = end
  }
  return values
}

/** Rewrites the declared filter parameters on a URL, leaving all others. */
export function writeFilterValues(url: URL, document: DashboardDocument, values: FilterValues): URL {
  const next = new URL(url)
  for (const name of filterParamNames(document)) {
    next.searchParams.delete(name)
    const value = values[name]
    if (Array.isArray(value)) value.forEach((item) => next.searchParams.append(name, item))
    else if (value !== undefined) next.searchParams.set(name, value)
  }
  return next
}

export function sameFilterValues(left: FilterValues, right: FilterValues): boolean {
  const leftKeys = Object.keys(left)
  if (leftKeys.length !== Object.keys(right).length) return false
  return leftKeys.every((key) => {
    const a = left[key]
    const b = right[key]
    return Array.isArray(a) || Array.isArray(b)
      ? Array.isArray(a) && Array.isArray(b) && a.length === b.length && a.every((value, index) => value === b[index])
      : a === b
  })
}

/**
 * The selection a period control renders: the URL's validated value when one
 * is present, otherwise the server-normalized value the document declared.
 */
export function currentPeriodValue(period: PeriodFilter, values: FilterValues): PeriodValue {
  const start = values[period.startParam]
  const end = values[period.endParam]
  if (Array.isArray(start) || Array.isArray(end)) return period.value
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
  for (const [name, value] of Object.entries(values ?? {})) {
    if (Array.isArray(value)) value.forEach((item) => url.searchParams.append(name, item))
    else url.searchParams.set(name, value)
  }
  // Keep the src's original credential scope: emit a same-origin relative URL.
  return `${url.pathname}${url.search}${url.hash}`
}

export function compareValues(filter: NonNullable<Filter['compare']>, value: CompareValue): FilterValues {
  const values: FilterValues = { [filter.modeParam]: value.mode }
  if (value.mode === 'custom') {
    values[filter.startParam] = value.start ?? ''
    values[filter.endParam] = value.end ?? ''
  }
  return values
}

export function filterActionURL(action: Action, fields: Readonly<Record<string, unknown>>, current: URL): URL | undefined {
  if ((action.kind !== 'cross_filter' && action.kind !== 'cube_drill') || !action.filter) return undefined
  const raw = resolveSourceValue(action.filter.value, { fields, variables: variablesFromLocation(current), location: current })
  if (typeof raw !== 'string' && typeof raw !== 'number' && typeof raw !== 'boolean' && typeof raw !== 'bigint') return undefined
  if (String(raw).trim() === '') return undefined
  const value = String(raw)
  let target: URL
  try {
    target = new URL(action.urlTemplate || current.pathname, current)
  } catch {
    return undefined
  }
  if (target.origin !== current.origin) return undefined
  const explicit = new Set(target.searchParams.keys())
  for (const [name, item] of current.searchParams) {
    if (!explicit.has(name)) target.searchParams.append(name, item)
  }
  const encoded = `${action.filter.dimension}:${value}`
  const filters = target.searchParams.getAll(cubeFilterParam)
  target.searchParams.delete(cubeFilterParam)
  const toggled = filters.includes(encoded) ? filters.filter((item) => item !== encoded) : [...filters, encoded]
  toggled.forEach((item) => target.searchParams.append(cubeFilterParam, item))
  if (action.kind === 'cube_drill' && action.filter.groupBy) target.searchParams.set(cubeGroupByParam, action.filter.groupBy)
  return target
}
