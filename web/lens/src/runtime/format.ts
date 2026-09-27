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
 * The typographic minus (U+2212). Every numeral this runtime writes carries it,
 * so a negative figure reads the same whether Intl produced the sign (en-US
 * emits an ASCII hyphen, ru-RU already emits U+2212), a panel prepended it
 * (`metricViews`, `CascadePanel`), or the sheet drew it into a formula. A
 * hyphen is also narrower than the `+` it has to line up with under
 * `tabular-nums`, which shows up as a wobbling sign column in a table of deltas.
 */
export const minusSign = '−'

/**
 * A number and its unit are one reading, so the space between them is a
 * no-break space (U+00A0) and never an ASCII one.
 *
 * Intl already glues a compact magnitude to its mantissa this way («201,16 млрд»
 * arrives with U+00A0 inside it), but every currency and symbol this runtime
 * appended did so with a plain space — which is why a narrow strip cell printed
 * «201,16 млрд» on one line and «UZS» on the next, an amount split from its
 * unit. Half the glue was there; this is the other half.
 */
const noBreakSpace = '\u00A0'

/** Joins a formatted numeral to its unit so the pair cannot break across lines. */
export function joinUnit(text: string, unit: string | undefined): string {
  const suffix = unit?.trim()
  return suffix ? `${text}${noBreakSpace}${suffix}` : text
}

/**
 * The single writer for a formatted numeral: it joins Intl's parts, pins the
 * decimal separator when the document asked for one, and normalizes the sign.
 *
 * Compact magnitudes come from the locale's own CLDR data (ru → «млрд», uz →
 * «mlrd», en → «B»), never from a hardcoded suffix table. Only the decimal
 * separator can be pinned: `decimalSeparator` exists because the server
 * formatter prints the mantissa with `%.*f`, so a document
 * that must match it byte for byte asks for ".".
 *
 * The sign is deliberately outside that byte-for-byte deal: a separator is
 * notation the producer chose, a minus is typography this runtime owns, and two
 * glyphs for one operator on one dashboard is the defect being fixed.
 */
export function writeNumberParts(parts: Intl.NumberFormatPart[], separator: string | undefined): string {
  const text = parts
    .map((part) => (part.type === 'minusSign'
      ? minusSign
      : part.type === 'decimal' && separator ? separator : part.value))
    .join('')
  if (!separator) return text
  // Pinning the separator means "match the server formatter", which also prints an
  // ASCII space before magnitude words and no space before the percent sign.
  return text.replace(/[\u00A0\u202F]/g, ' ').replace(/\s+%/, '%')
}

/**
 * Below this magnitude compact notation falls back to the exact grouped
 * integer: «12 500 UZS» instead of «12.50 тыс UZS». Mirrors the server's
 * abbreviation floor so screen and export output agree.
 */
const COMPACT_FLOOR = 100_000

function formatCompactNumber(value: number, field: FieldFormat, locale: string, currency?: string): string {
  if (Math.abs(value) < COMPACT_FLOOR) {
    const grouped = new Intl.NumberFormat(locale, { maximumFractionDigits: 0 })
    const text = writeNumberParts(grouped.formatToParts(value), field.decimalSeparator)
    return joinUnit(text, currency)
  }
  const precision = field.precision ?? 2
  const formatter = new Intl.NumberFormat(locale, {
    notation: 'compact',
    compactDisplay: 'short',
    minimumFractionDigits: precision,
    maximumFractionDigits: precision,
  })
  const compact = writeNumberParts(formatter.formatToParts(value), field.decimalSeparator)
  return joinUnit(compact, currency)
}

function formatMoney(value: number, field: FieldFormat, locale: string): string {
  const currency = field.currency ?? 'USD'
  const scaled = field.minorUnits ? value / 100 : value
  if (field.compact) return formatCompactNumber(scaled, field, locale, currency)
  // A document that pins the currency's grapheme wants the server formatter's
  // "<amount> <symbol>" shape (UZS → "so’m"), not the locale's own currency
  // display for the ISO code.
  if (field.symbol) {
    const decimal = new Intl.NumberFormat(locale, precisionOptions(field.precision))
    return joinUnit(writeNumberParts(decimal.formatToParts(scaled), field.decimalSeparator), field.symbol)
  }
  const base = new Intl.NumberFormat(locale, { style: 'currency', currency, ...precisionOptions(field.precision) })
  return writeNumberParts(base.formatToParts(scaled), field.decimalSeparator)
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
    const compact = new Intl.NumberFormat(locale, { notation: 'compact', maximumFractionDigits: 1 })
    return writeNumberParts(compact.formatToParts(scaled), field.decimalSeparator)
  }
  return formatFieldValue(value, field, locale)
}

/**
 * The unit `formatAxis` took off the ticks, so an axis can state it once.
 *
 * Dropping the currency from every gridline was right — eight repetitions of
 * «млрд UZS» is a column of noise — but the other half of that decision was
 * never built: the ticks lost the unit and nothing gained it, so «35 млн» sat
 * on a revenue axis with the currency findable only by hovering a mark. The
 * axis name is where a unit belongs; this is what goes in it.
 *
 * A percentage is excluded: the glyph is already on every tick, is one
 * character wide, and reads as part of the number rather than as a unit.
 */
export function axisUnit(field: FieldFormat | undefined): string {
  if (!field || field.kind !== 'money') return ''
  return field.symbol ?? field.currency ?? ''
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
  const text = writeNumberParts(grouped.formatToParts(scaled), field.decimalSeparator)
  return field.kind === 'money' ? joinUnit(text, field.currency) : text
}

/**
 * The clipboard companion to every formatted figure: the same quantity with
 * nothing done to it a spreadsheet would have to undo — no thousands
 * separators, no unit, no compact abbreviation, a dot decimal.
 *
 * It is scaled the way the display is. A money field in minor units draws
 * «41,20 млрд UZS» from 4 120 000 000 000 tiyin, and a reader who copies that
 * figure means the som they can see, not the tiyin behind it. Undefined when
 * there is no number to copy.
 */
export function rawValueText(value: unknown, field?: FieldFormat): string | undefined {
  const number = numeric(value)
  if (number === undefined) return undefined
  const scaled = field?.kind === 'money' && field.minorUnits ? number / 100 : number
  // Rounded to the significance the producer declared for the field. A bridge
  // step is a difference of running totals, so it arrives carrying the binary
  // noise of that subtraction — «−5626024042.350006» for a figure the panel
  // draws, correctly, as −5 626 024 042. Pasting the noise into a spreadsheet
  // is pasting an artefact of IEEE-754, not a number anyone reported.
  const rounded = field?.precision === undefined ? scaled : Number(scaled.toFixed(field.precision))
  return String(rounded)
}

/**
 * Delta chips clamp beyond ±999.9%: «+13 417.3%» is noise, «>999%» is honest.
 * Returns undefined inside the displayable range. Mirrors the server's
 * trendPercentText clamp.
 */
export function clampedDeltaPercent(value: number, locale = 'en'): string | undefined {
  const formatLimit = (limit: number) => writeNumberParts(new Intl.NumberFormat(locale, {
    style: 'unit', unit: 'percent', unitDisplay: 'narrow', maximumFractionDigits: 0, signDisplay: 'always',
  }).formatToParts(limit), undefined)
  if (value > 999.9) return `>${formatLimit(999)}`
  if (value < -999.9) return `<${formatLimit(-999)}`
  return undefined
}

export function formatFieldValue(value: unknown, field: FieldFormat | undefined, locale: string): string {
  if (!field) {
    const number = numeric(value)
    return number === undefined
      ? fallback(value)
      : writeNumberParts(new Intl.NumberFormat(locale).formatToParts(number), undefined)
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
    return writeNumberParts(percent.formatToParts(number), field.decimalSeparator)
  }
  if (field.compact) return formatCompactNumber(number, field, locale)
  const plain = new Intl.NumberFormat(locale, precisionOptions(field.precision))
  return writeNumberParts(plain.formatToParts(number), field.decimalSeparator)
}

/**
 * The one magnitude a group of compact values should share.
 *
 * The largest value sets the notation — that is the number the group is read
 * against. But a magnitude that rounds a real value down to «0.00» prints a
 * lie next to the real zeros, so when the smallest non-zero member cannot
 * survive the largest member's magnitude, the group steps down to the
 * smaller one. One notation either way; nothing that exists reads as nothing.
 *
 * Returns undefined when there is nothing to share a magnitude across — an
 * empty group, or one that is entirely zero.
 */
export function compactReference(values: Iterable<number>, field: FieldFormat | undefined): number | undefined {
  if (!field?.compact || (field.kind !== 'money' && field.kind !== 'number')) return undefined
  let largest = 0
  let smallest = Infinity
  for (const value of values) {
    const magnitude = Math.abs(value)
    if (magnitude === 0) continue
    if (magnitude > largest) largest = magnitude
    if (magnitude < smallest) smallest = magnitude
  }
  if (largest === 0) return undefined
  const scale = field.kind === 'money' && field.minorUnits ? 100 : 1
  if (largest / scale < COMPACT_FLOOR) return undefined
  const exponent = Math.min(15, Math.floor(Math.log10(largest / scale) / 3) * 3)
  const smallestAtLargest = (smallest / scale) / 10 ** exponent
  if (smallestAtLargest >= 0.5 * 10 ** -(field.precision ?? 2)) return largest
  // Stepping down stops at the abbreviation floor: below it there is no
  // magnitude word to share, and the group would fall back to per-value
  // formatting — the very thing a shared magnitude exists to prevent.
  return Math.max(smallest, COMPACT_FLOOR * scale)
}

/** Format a compact tooltip group at one shared magnitude. */
export function formatFieldValueAtReference(value: unknown, reference: number, field: FieldFormat | undefined, locale: string): string {
  const number = numeric(value)
  if (number === undefined || !field?.compact || (field.kind !== 'money' && field.kind !== 'number')) {
    return formatFieldValue(value, field, locale)
  }
  const scaledNumber = field.kind === 'money' && field.minorUnits ? number / 100 : number
  const scaledReference = field.kind === 'money' && field.minorUnits ? reference / 100 : reference
  if (Math.abs(scaledReference) < COMPACT_FLOOR) return formatFieldValue(value, field, locale)
  const exponent = Math.min(15, Math.floor(Math.log10(Math.abs(scaledReference)) / 3) * 3)
  const divisor = 10 ** exponent
  // Whether a magnitude is glued to its mantissa is the locale's call, not
  // ours: en writes «9.36B», ru writes «9,36 млрд». Reading the glue off the
  // same parts array that produced the magnitude keeps this formatter writing
  // what the server writes, instead of inserting a space en never has.
  const compactParts = new Intl.NumberFormat(locale, { notation: 'compact', compactDisplay: 'short', maximumFractionDigits: 0 })
    .formatToParts(divisor)
  const compactIndex = compactParts.findIndex((part) => part.type === 'compact')
  const compactPart = compactIndex < 0 ? '' : compactParts[compactIndex]!.value
  // Whether there is a space at all is the locale's call; that it cannot break
  // is this runtime's, the same deal joinUnit strikes for a currency suffix.
  const compactGlue = compactIndex > 0 && compactParts[compactIndex - 1]!.type === 'literal'
    ? compactParts[compactIndex - 1]!.value.replace(/\s+/g, noBreakSpace)
    : ''
  const mantissa = new Intl.NumberFormat(locale, precisionOptions(field.precision))
  const numeral = writeNumberParts(mantissa.formatToParts(scaledNumber / divisor), field.decimalSeparator)
  const text = compactPart ? `${numeral}${compactGlue}${compactPart}` : numeral
  return field.kind === 'money' ? joinUnit(text, field.currency ?? 'USD') : text
}
