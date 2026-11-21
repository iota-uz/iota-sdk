// Cron for schedule triggers (#1326 ScheduleDef): five fields, an IANA time zone and `missed_after`.
// Pure and dependency-free: the next runs are computed in the zone through Intl.DateTimeFormat.
import { pick } from './locale'
import type { EditorLocale } from './types'

export type CronFieldName = 'minute' | 'hour' | 'day' | 'month' | 'weekday'

export type CronFields = {
  minute: number[]
  hour: number[]
  day: number[]
  month: number[]
  /** 0 = Sunday … 6 = Saturday (7 is folded into 0). */
  weekday: number[]
  /** Vixie rule: when both day and weekday are restricted, either one matching is enough. */
  dayStar: boolean
  weekdayStar: boolean
}

export type CronError =
  | { code: 'fields'; count: number }
  | { code: 'value'; field: CronFieldName; value: string }
  | { code: 'range'; field: CronFieldName; value: string; min: number; max: number }

export type CronParse = { ok: true; fields: CronFields } | { ok: false; error: CronError }

const FIELD_ORDER: CronFieldName[] = ['minute', 'hour', 'day', 'month', 'weekday']
const BOUNDS: Record<CronFieldName, [number, number]> = {
  minute: [0, 59],
  hour: [0, 23],
  day: [1, 31],
  month: [1, 12],
  weekday: [0, 7],
}
const MONTH_NAMES = ['JAN', 'FEB', 'MAR', 'APR', 'MAY', 'JUN', 'JUL', 'AUG', 'SEP', 'OCT', 'NOV', 'DEC']
const DAY_NAMES = ['SUN', 'MON', 'TUE', 'WED', 'THU', 'FRI', 'SAT']

function atom(field: CronFieldName, text: string): number | CronError {
  const upper = text.toUpperCase()
  if (field === 'month') {
    const index = MONTH_NAMES.indexOf(upper)
    if (index >= 0) return index + 1
  }
  if (field === 'weekday') {
    const index = DAY_NAMES.indexOf(upper)
    if (index >= 0) return index
  }
  if (!/^\d+$/.test(text)) return { code: 'value', field, value: text }
  const value = Number(text)
  const [min, max] = BOUNDS[field]
  if (value < min || value > max) return { code: 'range', field, value: text, min, max: field === 'weekday' ? 7 : max }
  return value
}

function parseField(field: CronFieldName, text: string): number[] | CronError {
  const [min, max] = BOUNDS[field]
  const values = new Set<number>()
  for (const part of text.split(',')) {
    if (part === '') return { code: 'value', field, value: text }
    const [rangeText, stepText, extra] = part.split('/')
    if (extra !== undefined || rangeText === undefined || rangeText === '') return { code: 'value', field, value: part }
    let step = 1
    if (stepText !== undefined) {
      if (!/^\d+$/.test(stepText) || Number(stepText) === 0) return { code: 'value', field, value: part }
      step = Number(stepText)
    }
    let low: number
    let high: number
    if (rangeText === '*') {
      low = min
      high = field === 'weekday' ? 6 : max
    } else {
      const bounds = rangeText.split('-')
      if (bounds.length > 2) return { code: 'value', field, value: part }
      const a = atom(field, bounds[0]!)
      if (typeof a !== 'number') return a
      low = a
      if (bounds.length === 2) {
        const b = atom(field, bounds[1]!)
        if (typeof b !== 'number') return b
        high = b
        if (high < low) return { code: 'value', field, value: part }
      } else {
        high = stepText !== undefined ? (field === 'weekday' ? 6 : max) : low
      }
    }
    for (let v = low; v <= high; v += step) values.add(field === 'weekday' && v === 7 ? 0 : v)
  }
  return [...values].sort((a, b) => a - b)
}

export function parseCron(expression: string): CronParse {
  const parts = expression.trim().split(/\s+/).filter(Boolean)
  if (parts.length !== 5) return { ok: false, error: { code: 'fields', count: parts.length } }
  const out: Partial<Record<CronFieldName, number[]>> = {}
  for (let i = 0; i < 5; i += 1) {
    const field = FIELD_ORDER[i]!
    const parsed = parseField(field, parts[i]!)
    if (!Array.isArray(parsed)) return { ok: false, error: parsed }
    out[field] = parsed
  }
  return {
    ok: true,
    fields: {
      minute: out.minute!, hour: out.hour!, day: out.day!, month: out.month!, weekday: out.weekday!,
      dayStar: parts[2]!.startsWith('*'),
      weekdayStar: parts[4]!.startsWith('*'),
    },
  }
}

export function isValidCron(expression: string): boolean {
  return parseCron(expression).ok
}

// ---- Time zones ------------------------------------------------------------------------------

const FALLBACK_ZONES = [
  'UTC', 'Asia/Tashkent', 'Asia/Samarkand', 'Asia/Almaty', 'Asia/Dubai', 'Asia/Tokyo', 'Asia/Shanghai', 'Asia/Kolkata',
  'Europe/Moscow', 'Europe/Istanbul', 'Europe/Berlin', 'Europe/London', 'Europe/Paris', 'America/New_York',
  'America/Chicago', 'America/Denver', 'America/Los_Angeles', 'America/Sao_Paulo', 'Australia/Sydney', 'Africa/Cairo',
]

export function timeZones(): string[] {
  const intl = Intl as typeof Intl & { supportedValuesOf?: (key: string) => string[] }
  let zones: string[] = []
  try {
    zones = intl.supportedValuesOf?.('timeZone') ?? []
  } catch {
    zones = []
  }
  if (zones.length === 0) zones = FALLBACK_ZONES
  return ['UTC', ...zones.filter((zone) => zone !== 'UTC')]
}

export function isValidTimeZone(zone: string): boolean {
  try {
    new Intl.DateTimeFormat('en-US', { timeZone: zone })
    return true
  } catch {
    return false
  }
}

const formatters = new Map<string, Intl.DateTimeFormat>()

function zoneFormatter(zone: string): Intl.DateTimeFormat {
  let formatter = formatters.get(zone)
  if (!formatter) {
    formatter = new Intl.DateTimeFormat('en-US', {
      timeZone: zone, hourCycle: 'h23', year: 'numeric', month: 'numeric', day: 'numeric', hour: 'numeric', minute: 'numeric',
    })
    formatters.set(zone, formatter)
  }
  return formatter
}

type WallTime = { year: number; month: number; day: number; hour: number; minute: number }

export function wallTime(instant: number, zone: string): WallTime {
  const parts: Record<string, number> = {}
  for (const part of zoneFormatter(zone).formatToParts(new Date(instant))) {
    if (part.type !== 'literal') parts[part.type] = Number(part.value)
  }
  return { year: parts.year!, month: parts.month!, day: parts.day!, hour: parts.hour! % 24, minute: parts.minute! }
}

/** The UTC instant of a wall-clock time in the zone, or null when DST skips that time. */
export function zonedToUtc(wall: WallTime, zone: string): number | null {
  const guess = Date.UTC(wall.year, wall.month - 1, wall.day, wall.hour, wall.minute)
  const offsetAt = (instant: number) => {
    const w = wallTime(instant, zone)
    return Date.UTC(w.year, w.month - 1, w.day, w.hour, w.minute) - instant
  }
  // Earlier offset first so an ambiguous (fall-back) time resolves to its first occurrence.
  const candidates = [...new Set([offsetAt(guess - 86_400_000), offsetAt(guess), offsetAt(guess + 86_400_000)])]
    .map((offset) => guess - offset)
    .sort((a, b) => a - b)
  for (const instant of candidates) {
    const w = wallTime(instant, zone)
    if (w.year === wall.year && w.month === wall.month && w.day === wall.day && w.hour === wall.hour && w.minute === wall.minute) return instant
  }
  return null
}

/** The next `count` runs after `from` (exclusive), in the zone. Times skipped by DST do not run. */
export function nextRuns(expression: string, timeZone: string, from: Date, count = 5): Date[] {
  const parsed = parseCron(expression)
  if (!parsed.ok || count <= 0) return []
  const zone = isValidTimeZone(timeZone) ? timeZone : 'UTC'
  const f = parsed.fields
  const months = new Set(f.month)
  const days = new Set(f.day)
  const weekdays = new Set(f.weekday)
  const restrictedBoth = !f.dayStar && !f.weekdayStar
  const start = from.getTime()
  const startWall = wallTime(start, zone)
  const runs: Date[] = []
  // Walk local calendar days; at most ~8 years so impossible dates (30 FEB) terminate.
  for (let offset = 0; offset < 366 * 8 && runs.length < count; offset += 1) {
    const date = new Date(Date.UTC(startWall.year, startWall.month - 1, startWall.day + offset))
    const month = date.getUTCMonth() + 1
    if (!months.has(month)) {
      // Jump to the first day of the next month.
      const next = new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth() + 1, 1))
      offset += Math.round((next.getTime() - date.getTime()) / 86_400_000) - 1
      continue
    }
    const day = date.getUTCDate()
    const weekday = date.getUTCDay()
    const dayMatch = days.has(day)
    const weekdayMatch = weekdays.has(weekday)
    const matches = restrictedBoth ? dayMatch || weekdayMatch : dayMatch && weekdayMatch
    if (!matches) continue
    for (const hour of f.hour) {
      for (const minute of f.minute) {
        const instant = zonedToUtc({ year: date.getUTCFullYear(), month, day, hour, minute }, zone)
        if (instant === null || instant <= start) continue
        runs.push(new Date(instant))
        if (runs.length >= count) return runs
      }
    }
  }
  return runs
}

// ---- Presets ---------------------------------------------------------------------------------

export type PresetKind = 'daily' | 'weekdays' | 'weekly' | 'monthly' | 'custom'
export type Preset = { kind: PresetKind; hour: number; minute: number; weekday?: number; day?: number }

export const DEFAULT_CRON = '0 9 * * *'

export const presets = {
  daily: (p: { hour: number; minute: number }) => `${p.minute} ${p.hour} * * *`,
  weekdays: (p: { hour: number; minute: number }) => `${p.minute} ${p.hour} * * 1-5`,
  weekly: (p: { hour: number; minute: number; weekday?: number }) => `${p.minute} ${p.hour} * * ${p.weekday ?? 1}`,
  monthly: (p: { hour: number; minute: number; day?: number }) => `${p.minute} ${p.hour} ${p.day ?? 1} * *`,
}

export function presetCron(preset: Preset): string | null {
  switch (preset.kind) {
    case 'daily': return presets.daily(preset)
    case 'weekdays': return presets.weekdays(preset)
    case 'weekly': return presets.weekly(preset)
    case 'monthly': return presets.monthly(preset)
    default: return null
  }
}

const single = (text: string, min: number, max: number): number | null => {
  if (!/^\d+$/.test(text)) return null
  const value = Number(text)
  return value >= min && value <= max ? value : null
}

export function detectPreset(expression: string): Preset {
  const parts = expression.trim().split(/\s+/)
  const fallback: Preset = { kind: 'custom', hour: 9, minute: 0 }
  if (parts.length !== 5 || !parseCron(expression).ok) return fallback
  const [m, h, dom, mon, dow] = parts as [string, string, string, string, string]
  const minute = single(m, 0, 59)
  const hour = single(h, 0, 23)
  if (minute === null || hour === null) return { ...fallback, hour: hour ?? 9, minute: minute ?? 0 }
  if (mon !== '*') return { kind: 'custom', hour, minute }
  if (dom === '*' && dow === '*') return { kind: 'daily', hour, minute }
  if (dom === '*' && (dow === '1-5' || dow.toUpperCase() === 'MON-FRI')) return { kind: 'weekdays', hour, minute }
  if (dom === '*') {
    const named = DAY_NAMES.indexOf(dow.toUpperCase())
    const weekday = named >= 0 ? named : single(dow, 0, 7)
    if (weekday !== null) return { kind: 'weekly', hour, minute, weekday: weekday % 7 }
  }
  if (dow === '*') {
    const day = single(dom, 1, 31)
    if (day !== null) return { kind: 'monthly', hour, minute, day }
  }
  return { kind: 'custom', hour, minute }
}

// ---- Human names (C3) ------------------------------------------------------------------------

const pad = (n: number) => String(n).padStart(2, '0')
const clock = (p: Preset) => `${pad(p.hour)}:${pad(p.minute)}`

type DescribeStrings = {
  daily: (t: string) => string
  weekdays: (t: string) => string
  weekly: (weekday: number, t: string) => string
  monthly: (day: number, t: string) => string
  custom: (cron: string) => string
  invalid: string
}

const RU_WEEKLY = ['Каждое воскресенье', 'Каждый понедельник', 'Каждый вторник', 'Каждую среду', 'Каждый четверг', 'Каждую пятницу', 'Каждую субботу']
const EN_DAYS = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']
const UZ_DAYS = ['yakshanba', 'dushanba', 'seshanba', 'chorshanba', 'payshanba', 'juma', 'shanba']
const UZ_CYRL_DAYS = ['якшанба', 'душанба', 'сешанба', 'чоршанба', 'пайшанба', 'жума', 'шанба']

const DESCRIBE: Record<EditorLocale, DescribeStrings> = {
  en: {
    daily: (t) => `Every day at ${t}`,
    weekdays: (t) => `Every weekday at ${t}`,
    weekly: (d, t) => `Every ${EN_DAYS[d]} at ${t}`,
    monthly: (day, t) => `Monthly on day ${day} at ${t}`,
    custom: (c) => `On schedule ${c}`,
    invalid: 'Schedule',
  },
  ru: {
    daily: (t) => `Каждый день в ${t}`,
    weekdays: (t) => `По будням в ${t}`,
    weekly: (d, t) => `${RU_WEEKLY[d]} в ${t}`,
    monthly: (day, t) => `Каждый месяц, ${day}-го числа в ${t}`,
    custom: (c) => `По расписанию ${c}`,
    invalid: 'Расписание',
  },
  uz: {
    daily: (t) => `Har kuni ${t} da`,
    weekdays: (t) => `Ish kunlari ${t} da`,
    weekly: (d, t) => `Har ${UZ_DAYS[d]} ${t} da`,
    monthly: (day, t) => `Har oyning ${day}-kuni ${t} da`,
    custom: (c) => `Jadval boʻyicha ${c}`,
    invalid: 'Jadval',
  },
  'uz-Cyrl': {
    daily: (t) => `Ҳар куни ${t} да`,
    weekdays: (t) => `Иш кунлари ${t} да`,
    weekly: (d, t) => `Ҳар ${UZ_CYRL_DAYS[d]} ${t} да`,
    monthly: (day, t) => `Ҳар ойнинг ${day}-куни ${t} да`,
    custom: (c) => `Жадвал бўйича ${c}`,
    invalid: 'Жадвал',
  },
}

/** A schedule named by meaning: "Every day at 20:00", "Каждый день в 20:00". */
export function describeSchedule(expression: string, locale: EditorLocale | undefined): string {
  const s = pick(DESCRIBE, locale)
  if (!isValidCron(expression)) return s.invalid
  const preset = detectPreset(expression)
  switch (preset.kind) {
    case 'daily': return s.daily(clock(preset))
    case 'weekdays': return s.weekdays(clock(preset))
    case 'weekly': return s.weekly(preset.weekday ?? 1, clock(preset))
    case 'monthly': return s.monthly(preset.day ?? 1, clock(preset))
    default: return s.custom(expression.trim().split(/\s+/).join(' '))
  }
}

// ---- Durations (missed_after) ----------------------------------------------------------------

/** The longest duration the graph format accepts: 30 days (pkg/automation/graph durationSchema). */
export const MAX_DURATION_MS = 30 * 86_400_000
const DURATION_PATTERN = /^([1-9][0-9]{0,5})(s|m|h|d)$/
const UNIT_MS: Record<string, number> = { s: 1000, m: 60_000, h: 3_600_000, d: 86_400_000 }

/** Milliseconds of `30s`, `20m`, `2h`, `7d`; null for anything else or over 30 days. */
export function durationMs(text: string): number | null {
  const match = DURATION_PATTERN.exec(text)
  if (!match) return null
  const ms = Number(match[1]) * UNIT_MS[match[2]!]!
  return ms <= MAX_DURATION_MS ? ms : null
}

export function isDuration(text: string): boolean {
  return durationMs(text) !== null
}

/** The largest whole unit for a number of milliseconds: 7200000 → `2h`. */
export function formatDuration(ms: number): string {
  for (const unit of ['d', 'h', 'm', 's'] as const) {
    const size = UNIT_MS[unit]!
    if (ms >= size && ms % size === 0) return `${ms / size}${unit}`
  }
  return `${Math.max(1, Math.round(ms / 1000))}s`
}
