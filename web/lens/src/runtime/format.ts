import type { FieldFormat } from '../contract'

function numeric(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return undefined
}

function fallback(value: unknown): string {
  if (value === null || value === undefined) return '—'
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint') return String(value)
  return '—'
}

function precisionOptions(precision: number | undefined): Intl.NumberFormatOptions {
  if (precision === undefined) return {}
  return { minimumFractionDigits: precision, maximumFractionDigits: precision }
}

/**
 * Compact magnitudes come from the locale's own CLDR data (ru → «млрд», uz →
 * «mlrd», en → «B»), never from a hardcoded suffix table. Only the decimal
 * separator can be pinned: `decimalSeparator` exists because the Go renderer
 * prints the mantissa with `%.*f`, i.e. a dot in every locale, so a document
 * that must match it byte for byte asks for ".".
 */
function applyDecimalSeparator(parts: Intl.NumberFormatPart[], separator: string | undefined): string {
  const text = parts.map((part) => (part.type === 'decimal' && separator ? separator : part.value)).join('')
  if (!separator) return text
  // Pinning the separator means "match the Go renderer", which also prints an
  // ASCII space before magnitude words and no space before the percent sign.
  return text.replace(/[\u00A0\u202F]/g, ' ').replace(/\s+%/, '%')
}

/**
 * Below this magnitude compact notation falls back to the exact grouped
 * integer: «12 500 UZS» instead of «12.50 тыс UZS». Mirrors the Go renderer's
 * abbreviationFloor so both runtimes agree.
 */
const COMPACT_FLOOR = 100_000

function formatCompactNumber(value: number, field: FieldFormat, locale: string, currency?: string): string {
  if (Math.abs(value) < COMPACT_FLOOR) {
    const grouped = new Intl.NumberFormat(locale, { maximumFractionDigits: 0 })
    const text = applyDecimalSeparator(grouped.formatToParts(value), field.decimalSeparator)
    return currency ? `${text} ${currency}` : text
  }
  const precision = field.precision ?? 2
  const formatter = new Intl.NumberFormat(locale, {
    notation: 'compact',
    compactDisplay: 'short',
    minimumFractionDigits: precision,
    maximumFractionDigits: precision,
  })
  const compact = applyDecimalSeparator(formatter.formatToParts(value), field.decimalSeparator)
  return currency ? `${compact} ${currency}` : compact
}

function formatMoney(value: number, field: FieldFormat, locale: string): string {
  const currency = field.currency ?? 'USD'
  const scaled = field.minorUnits ? value / 100 : value
  if (field.compact) return formatCompactNumber(scaled, field, locale, currency)
  // A document that pins the currency's grapheme wants the Go renderer's
  // "<amount> <symbol>" shape (UZS → "so’m"), not the locale's own currency
  // display for the ISO code.
  if (field.symbol) {
    const decimal = new Intl.NumberFormat(locale, precisionOptions(field.precision))
    return `${applyDecimalSeparator(decimal.formatToParts(scaled), field.decimalSeparator)} ${field.symbol}`
  }
  const base = new Intl.NumberFormat(locale, { style: 'currency', currency, ...precisionOptions(field.precision) })
  return base.format(scaled)
}

const goDateTokens = [
  'January', 'Monday', 'Z07:00', '-07:00', '-0700', '2006', 'Jan', 'Mon', 'MST', 'PM', 'pm',
  '15', '03', '04', '05', '06', '01', '02', '1', '2', '3',
] as const

function datePart(date: Date, token: (typeof goDateTokens)[number], locale: string): string {
  const number = (value: number, width = 2) => String(value).padStart(width, '0')
  switch (token) {
    case '2006': return number(date.getUTCFullYear(), 4)
    case '06': return number(date.getUTCFullYear() % 100)
    case '01': return number(date.getUTCMonth() + 1)
    case '1': return String(date.getUTCMonth() + 1)
    case '02': return number(date.getUTCDate())
    case '2': return String(date.getUTCDate())
    case '15': return number(date.getUTCHours())
    case '03': return number(date.getUTCHours() % 12 || 12)
    case '3': return String(date.getUTCHours() % 12 || 12)
    case '04': return number(date.getUTCMinutes())
    case '05': return number(date.getUTCSeconds())
    case 'PM': return date.getUTCHours() < 12 ? 'AM' : 'PM'
    case 'pm': return date.getUTCHours() < 12 ? 'am' : 'pm'
    case 'January': return new Intl.DateTimeFormat(locale, { month: 'long', timeZone: 'UTC' }).format(date)
    case 'Jan': return new Intl.DateTimeFormat(locale, { month: 'short', timeZone: 'UTC' }).format(date)
    case 'Monday': return new Intl.DateTimeFormat(locale, { weekday: 'long', timeZone: 'UTC' }).format(date)
    case 'Mon': return new Intl.DateTimeFormat(locale, { weekday: 'short', timeZone: 'UTC' }).format(date)
    case 'MST': return 'UTC'
    case 'Z07:00':
    case '-07:00': return 'Z'
    case '-0700': return '+0000'
  }
}

function formatDate(value: unknown, layout: string | undefined, locale: string): string | undefined {
  const date = value instanceof Date ? value : new Date(typeof value === 'number' || typeof value === 'string' ? value : '')
  if (Number.isNaN(date.getTime())) return undefined
  const template = layout || '2006-01-02'
  let output = ''
  for (let index = 0; index < template.length;) {
    const token = goDateTokens.find((candidate) => template.startsWith(candidate, index))
    if (!token) {
      output += template[index]
      index += 1
      continue
    }
    output += datePart(date, token, locale)
    index += token.length
  }
  return output
}

export function formatAxis(value: unknown, field: FieldFormat | undefined, locale: string): string {
  const number = numeric(value)
  if (number !== undefined && field && (field.kind === 'money' || field.kind === 'number')) {
    // Axis ticks drop the currency suffix that the tooltip (formatFieldValue)
    // still carries: a column of «-90 млрд UZS» repeats the same three
    // letters on every gridline and crowds the plot; the magnitude alone is
    // what an axis needs, the unit stays legible from the tooltip and title.
    const scaled = field.kind === 'money' && field.minorUnits ? number / 100 : number
    return new Intl.NumberFormat(locale, { notation: 'compact', maximumFractionDigits: 1 }).format(scaled)
  }
  return formatFieldValue(value, field, locale)
}

/**
 * The full-precision companion to a compact field: «106.03 млрд UZS» carries
 * «106 034 767 694 UZS» in its tooltip. Returns undefined when the field is
 * not compact (nothing was abbreviated away) or the value is not numeric.
 */
export function formatFieldValueExact(value: unknown, field: FieldFormat | undefined, locale: string): string | undefined {
  if (!field?.compact || (field.kind !== 'money' && field.kind !== 'number')) return undefined
  const number = numeric(value)
  if (number === undefined) return undefined
  const scaled = field.kind === 'money' && field.minorUnits ? number / 100 : number
  if (Math.abs(scaled) < COMPACT_FLOOR) return undefined
  const grouped = new Intl.NumberFormat(locale, { maximumFractionDigits: 0 })
  const text = applyDecimalSeparator(grouped.formatToParts(scaled), field.decimalSeparator)
  return field.kind === 'money' ? `${text} ${field.currency ?? ''}`.trim() : text
}

/**
 * Delta chips clamp beyond ±999.9%: «+13 417.3%» is noise, «>999%» is honest.
 * Returns undefined inside the displayable range. Mirrors the Go renderer's
 * trendPercentText clamp.
 */
export function clampedDeltaPercent(value: number): string | undefined {
  if (value > 999.9) return '>999%'
  if (value < -999.9) return '<−999%'
  return undefined
}

export function formatFieldValue(value: unknown, field: FieldFormat | undefined, locale: string): string {
  if (!field) {
    const number = numeric(value)
    return number === undefined ? fallback(value) : new Intl.NumberFormat(locale).format(number)
  }
  if (field.kind === 'string') return fallback(value)
  if (field.kind === 'date') return formatDate(value, field.layout, locale) ?? fallback(value)
  const number = numeric(value)
  if (number === undefined) return fallback(value)
  if (field.kind === 'money') return formatMoney(number, field, locale)
  if (field.kind === 'percent') {
    const percent = new Intl.NumberFormat(locale, {
      style: 'unit', unit: 'percent', unitDisplay: 'narrow', ...precisionOptions(field.precision),
    })
    return applyDecimalSeparator(percent.formatToParts(number), field.decimalSeparator)
  }
  if (field.compact) return formatCompactNumber(number, field, locale)
  const plain = new Intl.NumberFormat(locale, precisionOptions(field.precision))
  return applyDecimalSeparator(plain.formatToParts(number), field.decimalSeparator)
}
