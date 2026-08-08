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

/** Inclusive day count of a range: a single-day range counts one. */
export function rangeDayCount(start: CalendarDate, end: CalendarDate): number {
  return Math.abs(toEpochDays(end) - toEpochDays(start)) + 1
}

/** The last day a year can currently show: today while the year is still open. */
function yearEnd(year: number, today: CalendarDate): CalendarDate {
  return year === today.year ? today : { year, month: 12, day: 31 }
}

/**
 * A whole calendar year, finished or still running.
 *
 * The running case is required to reach past January, so that on the 31st of
 * January «1 Jan — today» is read as the month it also is: both readings are
 * honest there, and the month is the one whose step size a reader can see.
 */
function isYearRange(start: CalendarDate, end: CalendarDate, today: CalendarDate): boolean {
  if (start.month !== 1 || start.day !== 1 || end.year !== start.year) return false
  if (end.month === 12 && end.day === 31) return true
  return sameDate(end, today) && end.month > 1
}

function isMonthRange(start: CalendarDate, end: CalendarDate): boolean {
  return start.day === 1 && start.year === end.year && start.month === end.month &&
    end.day === daysInMonth(end.year, end.month)
}

/**
 * The period one step earlier or later, on the period's own terms.
 *
 * This is what replaced the row of declared preset chips in the dashboard
 * header. Six chips stated six periods to switch between; one pair of arrows
 * states the relationship those chips actually encoded — the period before this
 * one — in a fraction of the width, and it keeps working on the dashboards that
 * declare no presets at all and on a range a reader drew by hand.
 *
 * The unit is read off the range rather than configured, because a range
 * already states it: a calendar year steps by years, a calendar month by
 * months, and anything else by its own length, so a 17-day window lands on the
 * 17 days before it with neither a gap nor an overlap.
 *
 * An open period — the 1st of January to today — is a year that has not
 * finished. Stepping back off one yields the whole previous year rather than a
 * January-to-August slice of it, which is the comparison the year chips existed
 * to offer; stepping forward into the current year re-opens it at today.
 */
export function shiftPeriodRange(
  start: CalendarDate,
  end: CalendarDate,
  direction: 1 | -1,
  today: CalendarDate,
): { start: CalendarDate; end: CalendarDate } {
  if (isYearRange(start, end, today)) {
    const year = start.year + direction
    return { start: { year, month: 1, day: 1 }, end: yearEnd(year, today) }
  }
  if (isMonthRange(start, end)) {
    const moved = addMonths({ ...start, day: 1 }, direction)
    return {
      start: moved,
      end: { year: moved.year, month: moved.month, day: daysInMonth(moved.year, moved.month) },
    }
  }
  const span = rangeDayCount(start, end) * direction
  return { start: addDays(start, span), end: addDays(end, span) }
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
 *
 * Always six rows, whatever the month needs. A month can fill four rows
 * (February starting on the week's first day) or six, and a grid sized to the
 * month drew a final row holding a single date — 31 January hanging under a
 * full grid reads as a stub rather than as the last week of the month — and
 * resized the popover under the reader every time they stepped a month. Six is
 * the natural maximum (at most six lead days plus 31 gives 37 cells), so the
 * fixed count only ever pads: the first row still holds the 1st, and the
 * surplus rows are trailing days of the next month, which the calendar renders
 * as the same inert padding it already renders at the month's edges.
 */
export const monthGridWeeks = 6

export function monthGrid(year: number, month: number, firstDay: number): Array<Array<MonthCell>> {
  const first: CalendarDate = { year, month, day: 1 }
  const lead = (dayOfWeek(first) - firstDay + 7) % 7
  let cursor = addDays(first, -lead)
  const weeks: Array<Array<MonthCell>> = []
  for (let week = 0; week < monthGridWeeks; week += 1) {
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
  /**
   * Completed past periods (closed intervals that exclude today) render after
   * a divider, mirroring the legacy picker's to-date / past grouping.
   */
  past?: boolean
}

/**
 * The built-in preset catalog rendered when a document declares no server
 * presets. It is the legacy HTMX picker's `DefaultQuickRanges` verbatim:
 * to-date periods first (current month, 30 days, 12 months, current fiscal
 * year), then the completed past periods (last month, last fiscal year).
 * `allTime` is intentionally absent: the control surfaces it through its own
 * rail entry, gated on the filter's `allowEmpty`.
 */
export const defaultPeriodPresets: ReadonlyArray<PeriodPresetDef> = [
  { id: 'thisMonth', labelKey: 'filter.period.preset.thisMonth', fallback: 'Current month' },
  { id: 'last30days', labelKey: 'filter.period.preset.last30days', fallback: '30 days' },
  { id: 'last12months', labelKey: 'filter.period.preset.last12months', fallback: '12 months' },
  { id: 'yearToDate', labelKey: 'filter.period.preset.yearToDate', fallback: 'Current fiscal year' },
  { id: 'lastMonth', labelKey: 'filter.period.preset.lastMonth', fallback: 'Last month', past: true },
  { id: 'lastYear', labelKey: 'filter.period.preset.lastYear', fallback: 'Last fiscal year', past: true },
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

/**
 * Localized "July 2026" heading for a visible month, composed from the month
 * name and the plain year. Composed rather than a single month+year format:
 * the ru locale's combined pattern appends a genitive « г.» suffix that wraps
 * the dual-pane header's centered label. The month name is capitalized —
 * standalone Cyrillic month names come back lowercase.
 */
export function monthLabel(locale: string, year: number, month: number): string {
  const name = dateTimeFormat(locale, { month: 'long' }).format(utcDate({ year, month, day: 1 }))
  return `${name.charAt(0).toUpperCase()}${name.slice(1)} ${year}`
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

/** Localized full date, e.g. for announcements and accessible names. */
export function dayLabel(locale: string, date: CalendarDate): string {
  return dateTimeFormat(locale, { day: 'numeric', month: 'short', year: 'numeric' }).format(utcDate(date))
}

/** Localized short month names, January first, capitalized like the heading. */
export function monthShortLabels(locale: string): Array<string> {
  const format = dateTimeFormat(locale, { month: 'short' })
  return Array.from({ length: 12 }, (_, index) => {
    const name = format.format(utcDate({ year: 2024, month: index + 1, day: 1 }))
    return `${name.charAt(0).toUpperCase()}${name.slice(1)}`
  })
}

/**
 * The twelve-year block a year belongs to, aligned to multiples of twelve so
 * the year panel keeps one stable 4×3 grid however it is reached.
 */
export function yearBlock(year: number): Array<number> {
  const start = Math.floor(year / 12) * 12
  return Array.from({ length: 12 }, (_, index) => start + index)
}

/**
 * The compact numeric day form `dd.mm.yyyy`. The trigger wears it so the applied
 * range is stated in the same vocabulary as the typed From/To fields instead of
 * a second, locale-dependent long form.
 */
export function formatCompactDate(date: CalendarDate): string {
  const day = String(date.day).padStart(2, '0')
  const month = String(date.month).padStart(2, '0')
  const year = String(date.year).padStart(4, '0')
  return `${day}.${month}.${year}`
}

/** The trigger's range label; a single-day range states one date. */
export function compactRangeLabel(start: CalendarDate, end: CalendarDate): string {
  const from = formatCompactDate(start)
  const to = formatCompactDate(end)
  return from === to ? from : `${from} – ${to}`
}

/**
 * The picker's one-line status: the inclusive day count once the draft is a
 * usable range, the next-step prompt while it is not. It lives here rather
 * than in the calendar because the popover footer is what states it.
 */
export function rangeHint(
  draft: RangeDraft,
  translate: (key: string, fallback: string, vars?: Record<string, string | number>) => string,
): string {
  if (draft.start && draft.end && compareDates(draft.start, draft.end) <= 0) {
    // A bare «30 дн.» in a popover footer is a number with no subject: the
    // reader has to infer that the calendar is telling them how long the range
    // they just drew is. The label says it.
    // The abbreviated unit survives every count; a host catalogue can spell it
    // out with its own plural rules.
    return translate('filter.period.duration', 'Duration: {count} d.', {
      count: rangeDayCount(draft.start, draft.end),
    })
  }
  return draft.start
    ? translate('calendar.hintEnd', 'Select an end date')
    : translate('calendar.hintStart', 'Select a start date')
}
