/**
 * Pure calendar model for the Lens period control.
 *
 * Dates are plain `{ year, month, day }` records and the wire format is the
 * server's own `YYYY-MM-DD` string. Nothing here ever constructs a local-time
 * `Date` from a boundary value: `Date.UTC` is used only as timezone-free epoch
 * arithmetic, so the client cannot reintroduce the timezone defect the server
 * normalization (Tashkent-anchored) exists to prevent.
 */

export interface CalendarDate {
  year: number
  /** 1-12 */
  month: number
  day: number
}

const isoDatePattern = /^(\d{4})-(\d{2})-(\d{2})$/

export function parseISODate(raw: string): CalendarDate | undefined {
  const match = isoDatePattern.exec(raw)
  if (!match) return undefined
  const year = Number(match[1])
  const month = Number(match[2])
  const day = Number(match[3])
  if (month < 1 || month > 12) return undefined
  if (day < 1 || day > daysInMonth(year, month)) return undefined
  return { year, month, day }
}

export function formatISODate(date: CalendarDate): string {
  const month = String(date.month).padStart(2, '0')
  const day = String(date.day).padStart(2, '0')
  return `${String(date.year).padStart(4, '0')}-${month}-${day}`
}

export function compareDates(left: CalendarDate, right: CalendarDate): number {
  if (left.year !== right.year) return left.year - right.year
  if (left.month !== right.month) return left.month - right.month
  return left.day - right.day
}

export function sameDate(left: CalendarDate | undefined, right: CalendarDate | undefined): boolean {
  if (!left || !right) return left === right
  return compareDates(left, right) === 0
}

export function daysInMonth(year: number, month: number): number {
  // Day 0 of the next month is the last day of this one; pure UTC arithmetic.
  return new Date(Date.UTC(year, month, 0)).getUTCDate()
}

const dayMs = 86_400_000

function toEpochDays(date: CalendarDate): number {
  return Date.UTC(date.year, date.month - 1, date.day) / dayMs
}

function fromEpochDays(days: number): CalendarDate {
  const value = new Date(days * dayMs)
  return { year: value.getUTCFullYear(), month: value.getUTCMonth() + 1, day: value.getUTCDate() }
}

export function addDays(date: CalendarDate, days: number): CalendarDate {
  return fromEpochDays(toEpochDays(date) + days)
}

export function addMonths(date: CalendarDate, months: number): CalendarDate {
  const index = date.year * 12 + (date.month - 1) + months
  const year = Math.floor(index / 12)
  const month = (index - year * 12) + 1
  return { year, month, day: Math.min(date.day, daysInMonth(year, month)) }
}

/** ISO day of week: 1 = Monday … 7 = Sunday. */
export function dayOfWeek(date: CalendarDate): number {
  const utcDay = new Date(Date.UTC(date.year, date.month - 1, date.day)).getUTCDay()
  return utcDay === 0 ? 7 : utcDay
}

export function clampDate(date: CalendarDate, min?: CalendarDate, max?: CalendarDate): CalendarDate {
  if (min && compareDates(date, min) < 0) return min
  if (max && compareDates(date, max) > 0) return max
  return date
}

export interface MonthCell {
  date: CalendarDate
  inMonth: boolean
}

/**
 * Weeks covering the month, each exactly seven days, including the boundary
 * days of the previous/next months that pad the first and last week.
 */
export function monthGrid(year: number, month: number, firstDay: number): Array<Array<MonthCell>> {
  const first: CalendarDate = { year, month, day: 1 }
  const lead = (dayOfWeek(first) - firstDay + 7) % 7
  let cursor = addDays(first, -lead)
  const total = lead + daysInMonth(year, month)
  const weekCount = Math.ceil(total / 7)
  const weeks: Array<Array<MonthCell>> = []
  for (let week = 0; week < weekCount; week += 1) {
    const cells: Array<MonthCell> = []
    for (let day = 0; day < 7; day += 1) {
      cells.push({ date: cursor, inMonth: cursor.year === year && cursor.month === month })
      cursor = addDays(cursor, 1)
    }
    weeks.push(cells)
  }
  return weeks
}

export type CalendarKey =
  | 'ArrowLeft'
  | 'ArrowRight'
  | 'ArrowUp'
  | 'ArrowDown'
  | 'PageUp'
  | 'PageDown'
  | 'Home'
  | 'End'

/** The grid-navigation target for a key, unclamped. */
export function keyboardTarget(date: CalendarDate, key: CalendarKey, firstDay: number): CalendarDate {
  switch (key) {
    case 'ArrowLeft': return addDays(date, -1)
    case 'ArrowRight': return addDays(date, 1)
    case 'ArrowUp': return addDays(date, -7)
    case 'ArrowDown': return addDays(date, 7)
    case 'PageUp': return addMonths(date, -1)
    case 'PageDown': return addMonths(date, 1)
    case 'Home': return addDays(date, -((dayOfWeek(date) - firstDay + 7) % 7))
    case 'End': return addDays(date, 6 - ((dayOfWeek(date) - firstDay + 7) % 7))
  }
}

export interface RangeDraft {
  start?: CalendarDate
  end?: CalendarDate
}

export interface RangeSelection {
  draft: RangeDraft
  /** Set when the pick completed a range (ordered start <= end). */
  complete?: { start: CalendarDate; end: CalendarDate }
}

/**
 * One click of the range state machine: the first pick anchors the range, the
 * second completes it (picking before the anchor swaps the boundaries), and a
 * pick over a completed range starts a new one.
 */
export function selectDay(draft: RangeDraft, day: CalendarDate): RangeSelection {
  if (!draft.start || draft.end) return { draft: { start: day } }
  const [start, end] = compareDates(day, draft.start) < 0 ? [day, draft.start] : [draft.start, day]
  return { draft: { start, end }, complete: { start, end } }
}

/** The live range a hover previews while the second boundary is pending. */
export function previewRange(draft: RangeDraft, hover: CalendarDate | undefined): { start: CalendarDate; end: CalendarDate } | undefined {
  if (!draft.start || draft.end || !hover) return undefined
  return compareDates(hover, draft.start) < 0
    ? { start: hover, end: draft.start }
    : { start: draft.start, end: hover }
}

export type RangeDayState = 'none' | 'start' | 'end' | 'inRange' | 'preview' | 'previewEdge'

export function rangeDayState(date: CalendarDate, draft: RangeDraft, hover: CalendarDate | undefined): RangeDayState {
  if (draft.start && sameDate(date, draft.start)) return 'start'
  if (draft.end && sameDate(date, draft.end)) return 'end'
  if (draft.start && draft.end &&
    compareDates(date, draft.start) > 0 && compareDates(date, draft.end) < 0) return 'inRange'
  const preview = previewRange(draft, hover)
  if (preview) {
    if (sameDate(date, preview.start) || sameDate(date, preview.end)) return 'previewEdge'
    if (compareDates(date, preview.start) > 0 && compareDates(date, preview.end) < 0) return 'preview'
  }
  return 'none'
}

/**
 * Predefined period presets, resolved client-side relative to a `today`.
 *
 * These mirror the *resolved bounds* of the legacy HTMX/templ pickers so the
 * React control keeps functional parity when a dashboard document declares no
 * server presets of its own. Every bound is an inclusive day range in the wire
 * calendar; `allTime` resolves to no bounds (the present-but-empty form).
 *
 * Semantics are taken from the legacy pickers:
 *  - the EAI analytics date-range popover (`this month`, `last 30 days`,
 *    `last 12 months`, `year to date`/fiscal year, `last month`, `last year`),
 *  - the generic SDK `filters.Default` picker (`today`, `yesterday`,
 *    `this week`, `last week`).
 * Week presets are Monday-first, matching the legacy generic picker's
 * hard-coded Monday anchor rather than the calendar grid's locale first day.
 */
export type PeriodPresetId =
  | 'today'
  | 'yesterday'
  | 'thisWeek'
  | 'lastWeek'
  | 'thisMonth'
  | 'lastMonth'
  | 'last30days'
  | 'last12months'
  | 'thisQuarter'
  | 'yearToDate'
  | 'thisYear'
  | 'lastYear'
  | 'allTime'

export interface PeriodBounds {
  start: CalendarDate
  end: CalendarDate
}

/**
 * Inclusive day bounds for a preset relative to `today`, or `undefined` for
 * `allTime` (unbounded). `last 12 months` uses clamped month arithmetic
 * (`addMonths`), so a month-overflow origin such as the 31st resolves to the
 * last valid day of the target month rather than rolling forward.
 */
export function resolvePreset(id: PeriodPresetId, today: CalendarDate): PeriodBounds | undefined {
  const firstOfMonth: CalendarDate = { year: today.year, month: today.month, day: 1 }
  switch (id) {
    case 'today':
      return { start: today, end: today }
    case 'yesterday': {
      const day = addDays(today, -1)
      return { start: day, end: day }
    }
    case 'thisWeek': {
      const start = addDays(today, -(dayOfWeek(today) - 1))
      return { start, end: addDays(start, 6) }
    }
    case 'lastWeek': {
      const start = addDays(today, -(dayOfWeek(today) - 1) - 7)
      return { start, end: addDays(start, 6) }
    }
    case 'thisMonth':
      return { start: firstOfMonth, end: today }
    case 'lastMonth':
      return { start: addMonths(firstOfMonth, -1), end: addDays(firstOfMonth, -1) }
    case 'last30days':
      return { start: addDays(today, -29), end: today }
    case 'last12months':
      return { start: addMonths(today, -12), end: today }
    case 'thisQuarter': {
      const quarterStartMonth = Math.floor((today.month - 1) / 3) * 3 + 1
      return { start: { year: today.year, month: quarterStartMonth, day: 1 }, end: today }
    }
    case 'yearToDate':
      return { start: { year: today.year, month: 1, day: 1 }, end: today }
    case 'thisYear':
      return { start: { year: today.year, month: 1, day: 1 }, end: { year: today.year, month: 12, day: 31 } }
    case 'lastYear':
      return { start: { year: today.year - 1, month: 1, day: 1 }, end: { year: today.year - 1, month: 12, day: 31 } }
    case 'allTime':
      return undefined
  }
}

export interface PeriodPresetDef {
  id: PeriodPresetId
  labelKey: string
  fallback: string
}

/**
 * The built-in preset catalog rendered when a document declares no server
 * presets. `allTime` is intentionally absent: the control surfaces it through
 * its own footer chip, gated on the filter's `allowEmpty`.
 */
export const defaultPeriodPresets: ReadonlyArray<PeriodPresetDef> = [
  { id: 'today', labelKey: 'filter.period.preset.today', fallback: 'Today' },
  { id: 'yesterday', labelKey: 'filter.period.preset.yesterday', fallback: 'Yesterday' },
  { id: 'thisWeek', labelKey: 'filter.period.preset.thisWeek', fallback: 'This week' },
  { id: 'lastWeek', labelKey: 'filter.period.preset.lastWeek', fallback: 'Last week' },
  { id: 'thisMonth', labelKey: 'filter.period.preset.thisMonth', fallback: 'This month' },
  { id: 'lastMonth', labelKey: 'filter.period.preset.lastMonth', fallback: 'Last month' },
  { id: 'last30days', labelKey: 'filter.period.preset.last30days', fallback: 'Last 30 days' },
  { id: 'last12months', labelKey: 'filter.period.preset.last12months', fallback: 'Last 12 months' },
  { id: 'thisQuarter', labelKey: 'filter.period.preset.thisQuarter', fallback: 'This quarter' },
  { id: 'yearToDate', labelKey: 'filter.period.preset.yearToDate', fallback: 'Year to date' },
  { id: 'thisYear', labelKey: 'filter.period.preset.thisYear', fallback: 'This year' },
  { id: 'lastYear', labelKey: 'filter.period.preset.lastYear', fallback: 'Last year' },
]

/**
 * Locale canonicalization for calendar text. Granite's Cyrillic-Uzbek locale
 * code is `oz`, which no Intl implementation knows; it is an alias of
 * `uz-Cyrl` for every locale-data purpose.
 */
export function canonicalCalendarLocale(locale: string): string {
  const trimmed = locale.trim()
  if (/^oz\b/i.test(trimmed)) return 'uz-Cyrl'
  return trimmed || 'en'
}

interface WeekInfoLocale extends Intl.Locale {
  getWeekInfo?: () => { firstDay: number }
  weekInfo?: { firstDay: number }
}

/**
 * First day of week for a locale, ISO numbering (1 = Monday … 7 = Sunday).
 * Uses the environment's CLDR week data when exposed; the fallback covers the
 * product locales (en, ru, uz, uz-Cyrl), all Monday-first per CLDR except
 * explicit Sunday-first regions.
 */
export function firstDayOfWeek(locale: string): number {
  const canonical = canonicalCalendarLocale(locale)
  try {
    const resolved = new Intl.Locale(canonical) as WeekInfoLocale
    const info = resolved.getWeekInfo?.() ?? resolved.weekInfo
    if (info && info.firstDay >= 1 && info.firstDay <= 7) return info.firstDay
    const region = resolved.maximize().region
    if (region && ['US', 'CA', 'MX', 'BR', 'JP', 'KR', 'IN', 'IL', 'SA', 'PH'].includes(region)) return 7
  } catch {
    // Fall through to Monday.
  }
  return 1
}

function dateTimeFormat(locale: string, options: Intl.DateTimeFormatOptions): Intl.DateTimeFormat {
  return new Intl.DateTimeFormat(canonicalCalendarLocale(locale), { ...options, timeZone: 'UTC' })
}

function utcDate(date: CalendarDate): Date {
  return new Date(Date.UTC(date.year, date.month - 1, date.day))
}

/** Localized "July 2026" heading for a visible month. */
export function monthLabel(locale: string, year: number, month: number): string {
  return dateTimeFormat(locale, { month: 'long', year: 'numeric' }).format(utcDate({ year, month, day: 1 }))
}

/** Localized weekday header labels, rotated so the week starts at firstDay. */
export function weekdayLabels(locale: string, firstDay: number): Array<string> {
  const format = dateTimeFormat(locale, { weekday: 'short' })
  const labels: Array<string> = []
  for (let offset = 0; offset < 7; offset += 1) {
    // 2024-01-01 is a Monday; walk from the locale's first day of week.
    const isoDay = ((firstDay - 1 + offset) % 7) + 1
    labels.push(format.format(utcDate({ year: 2024, month: 1, day: isoDay })))
  }
  return labels
}

/** Localized full date, e.g. for the trigger button and announcements. */
export function dayLabel(locale: string, date: CalendarDate): string {
  return dateTimeFormat(locale, { day: 'numeric', month: 'short', year: 'numeric' }).format(utcDate(date))
}
