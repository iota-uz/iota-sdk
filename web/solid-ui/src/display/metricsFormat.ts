/**
 * Pure formatting and ordering rules of the KPI band, the twin of
 * components/base/metrics/metrics.go. Amounts are minor units held as BigInt,
 * so int64 extremes format exactly.
 */

export type MetricKind = 'in' | 'out' | 'net' | 'count' | 'plain'

/** Minor units as an integer: a bigint, a safe-integer number, or a decimal string (int64 over JSON). */
export type MinorUnits = bigint | number | string

/** A value in minor units: `{ minor: 12345, decimals: 2 }` is 123,45. */
export interface Amount {
  minor: MinorUnits
  decimals: number
}

/** An amount in hundredths, the common case. */
export function money(minor: MinorUnits): Amount {
  return { minor, decimals: 2 }
}

/** A metric that has values keyed by currency; a `count` metric keys its single value by "". */
export interface MetricValues {
  values: Readonly<Record<string, Amount | undefined>>
}

/** How many currency rows a card shows before the rest fold behind "N more currencies". */
export const VISIBLE_CURRENCIES = 3

const NBSP = '\u00A0'
export const MINUS = '\u2212'

export function toMinor(value: MinorUnits): bigint {
  if (typeof value === 'bigint') return value
  if (typeof value === 'number') {
    if (!Number.isSafeInteger(value)) throw new RangeError(`minor units must be a safe integer, got ${value}`)
    return BigInt(value)
  }
  return BigInt(value.trim())
}

function magnitude(value: MinorUnits): bigint {
  const n = toMinor(value)
  return n < 0n ? -n : n
}

function group(digits: string): string {
  let out = ''
  for (let i = 0; i < digits.length; i += 1) {
    if (i > 0 && (digits.length - i) % 3 === 0) out += NBSP
    out += digits[i]
  }
  return out
}

/** The full value: digits grouped by no-break spaces, comma decimals, no sign (see `sign`). */
export function formatAmount(amount: Amount): string {
  let digits = magnitude(amount.minor).toString()
  const decimals = Math.max(0, Math.trunc(amount.decimals))
  if (decimals === 0) return group(digits)
  while (digits.length <= decimals) digits = `0${digits}`
  const cut = digits.length - decimals
  return `${group(digits.slice(0, cut))},${digits.slice(cut)}`
}

/** An integer count grouped by no-break spaces, with a true minus when negative. */
export function formatCount(n: MinorUnits): string {
  const digits = group(magnitude(n).toString())
  return toMinor(n) < 0n ? MINUS + digits : digits
}

/**
 * The prefix a value of this kind carries: "+" for income, a minus for outflow, by
 * sign for a balance, and only a true minus otherwise. Never a colour.
 */
export function sign(kind: MetricKind, minor: MinorUnits): string {
  const n = toMinor(minor)
  if (n === 0n) return ''
  if (kind === 'in') return '+'
  if (kind === 'out') return MINUS
  if (kind === 'net' && n > 0n) return '+'
  if (n < 0n) return MINUS
  return ''
}

/** Only a negative balance is highlighted. */
export function isNegative(kind: MetricKind, amount: Amount): boolean {
  return kind === 'net' && toMinor(amount.minor) < 0n
}

export function valueText(kind: MetricKind, amount: Amount): string {
  return sign(kind, amount.minor) + formatAmount(amount)
}

/**
 * One currency order for a band: base first, then the others in the given
 * preference order, then any remaining codes in code-point order.
 */
export function order(base: string, preferred: readonly string[], ...items: readonly MetricValues[]): string[] {
  const present = new Set<string>()
  for (const item of items) {
    for (const [code, value] of Object.entries(item.values)) {
      if (code !== '' && value !== undefined) present.add(code)
    }
  }
  const out: string[] = []
  const add = (code: string) => {
    if (code !== '' && present.has(code) && !out.includes(code)) out.push(code)
  }
  add(base)
  preferred.forEach(add)
  const rest = [...present].filter((code) => !out.includes(code)).sort()
  return [...out, ...rest]
}

export interface CompactParts {
  /** The mantissa or the full value, without sign: "19,30" or "12 500,00". */
  number: string
  /** The locale's magnitude word ("млрд", "B"); empty below a million. */
  unit: string
}

export interface CompactOptions {
  /** Locale for the magnitude words only (CLDR, as Intl prints them). Defaults to "en-US". */
  locale?: string
  /** Mantissa fraction digits. Defaults to 2. */
  digits?: number
}

const COMPACT_EXPONENTS = [6, 9, 12] as const

const unitCache = new Map<string, readonly string[]>()

/** The locale's short magnitude words for 10^6, 10^9 and 10^12 (ru → млн, млрд, трлн). */
export function compactUnits(locale = 'en-US'): readonly string[] {
  const cached = unitCache.get(locale)
  if (cached) return cached
  const formatter = new Intl.NumberFormat(locale, { notation: 'compact', compactDisplay: 'short' })
  // formatCompact divides by 10^6, 10^9 and 10^12, so a word is usable only if
  // the locale prints exactly "1 <word>" for that power. Locales that group by
  // other magnitudes (ja 万 = 10^4, hi lakh/crore) fall back to the English words.
  const words = COMPACT_EXPONENTS.map((exponent) => {
    const parts = formatter.formatToParts(10 ** exponent)
    const integer = parts.filter((part) => part.type === 'integer').map((part) => part.value).join('')
    const word = parts.filter((part) => part.type === 'compact').map((part) => part.value).join('').trim()
    return integer === '1' && word ? word : undefined
  })
  const units = words.every(Boolean) ? (words as string[]) : (locale === 'en-US' ? words.map((w) => w ?? '') : compactUnits('en-US'))
  unitCache.set(locale, units)
  return units
}

function roundDiv(numerator: bigint, denominator: bigint): bigint {
  return (numerator * 2n + denominator) / (denominator * 2n)
}

/**
 * The compact value of a dashboard tile: below a million it is the full value
 * (`formatAmount`), above it a mantissa with a comma and the locale's magnitude
 * word. Rounding is exact half-up on the minor units and rolls over into the
 * next magnitude (999 999 999 → "1,00 млрд", not "1000,00 млн"). No sign.
 */
export function formatCompact(amount: Amount, options: CompactOptions = {}): CompactParts {
  const digits = Math.max(0, Math.trunc(options.digits ?? 2))
  const decimals = Math.max(0, Math.trunc(amount.decimals))
  const abs = magnitude(amount.minor)
  const units = compactUnits(options.locale)
  const fraction = 10n ** BigInt(digits)
  for (let index = COMPACT_EXPONENTS.length - 1; index >= 0; index -= 1) {
    const exponent = COMPACT_EXPONENTS[index]!
    const scale = 10n ** BigInt(decimals + exponent)
    if (abs < scale) continue
    const scaled = roundDiv(abs * fraction, scale)
    const next = COMPACT_EXPONENTS[index + 1]
    if (next !== undefined && scaled >= fraction * 1000n) {
      const promoted = roundDiv(abs * fraction, 10n ** BigInt(decimals + next))
      return { number: formatAmount({ minor: promoted, decimals: digits }), unit: units[index + 1] ?? '' }
    }
    return { number: formatAmount({ minor: scaled, decimals: digits }), unit: units[index] ?? '' }
  }
  return { number: formatAmount(amount), unit: '' }
}
