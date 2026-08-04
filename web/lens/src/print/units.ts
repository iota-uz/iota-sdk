import type { FieldFormat } from '../contract'
import { formatFieldValue } from '../runtime'

/**
 * How one column of numbers is printed once its unit has been said in the
 * header rather than on every cell.
 */
export interface ColumnUnit {
  /** The unit, for the column head: «млрд UZS», «UZS», «%». */
  note?: string
  format(value: unknown): string
}

/**
 * A column is normalised to one magnitude only when its rows are close enough
 * for that to help. A product catalogue runs from 4.21 млрд to 3 116 — nine
 * orders — and printing the tail as 0.00 млрд would destroy it. Four orders is
 * where a shared scale still leaves the smallest row two significant digits.
 */
const NORMALISE_SPREAD = 1e4

/** Magnitudes a column can be stated in, largest first. */
const MAGNITUDES = [1e9, 1e6, 1e3]

function scaled(value: unknown, field: FieldFormat): number | undefined {
  if (value === null || value === undefined || value === '') return undefined
  const number = typeof value === 'number' ? value : Number(value)
  if (!Number.isFinite(number)) return undefined
  return field.minorUnits && field.kind === 'money' ? number / 100 : number
}

/**
 * The locale's own word for a magnitude — «млрд», «mlrd», «B» — taken from
 * CLDR compact notation rather than a suffix table of our own.
 */
function magnitudeWord(magnitude: number, locale: string): string | undefined {
  const parts = new Intl.NumberFormat(locale, { notation: 'compact', compactDisplay: 'short' })
    .formatToParts(magnitude)
  const word = parts.find((part) => part.type === 'compact')?.value.trim()
  return word || undefined
}

function plain(value: number, locale: string, field: FieldFormat, precision: number): string {
  return formatFieldValue(value, {
    ...field,
    kind: 'number',
    minorUnits: false,
    compact: false,
    currency: undefined,
    symbol: undefined,
    precision,
  }, locale)
}

/**
 * Decides how a whole column of one field prints.
 *
 * Two things a table repeats without needing to: the currency, which is the
 * same three letters on every row, and — when the rows are of comparable size —
 * the magnitude, which forces a reader to compare «4.00 млрд» against «312.20
 * млн» digit by digit. Both belong in the column head, said once.
 *
 * Returns a formatter that always produces a cell; when nothing can be hoisted
 * it is `formatFieldValue` unchanged.
 */
export function columnUnit(
  values: Array<unknown>,
  field: FieldFormat | undefined,
  locale: string,
): ColumnUnit {
  const fallback = { format: (value: unknown) => formatFieldValue(value, field, locale) }
  if (!field || (field.kind !== 'money' && field.kind !== 'number')) return fallback
  const numbers = values
    .map((value) => scaled(value, field))
    .filter((value): value is number => value !== undefined && value !== 0)
  if (numbers.length === 0) return fallback
  const currency = field.kind === 'money' ? (field.currency ?? '').trim() : ''
  const magnitudes = numbers.map((value) => Math.abs(value))
  const largest = Math.max(...magnitudes)
  const smallest = Math.min(...magnitudes)
  // A single unit for the column: the largest row's magnitude, provided the
  // smallest row survives it.
  const magnitude = largest / smallest <= NORMALISE_SPREAD
    ? MAGNITUDES.find((candidate) => largest >= candidate)
    : undefined
  const word = magnitude ? magnitudeWord(magnitude, locale) : undefined
  if (magnitude && word) {
    return {
      note: [word, currency].filter(Boolean).join(' '),
      format: (value: unknown) => {
        const number = scaled(value, field)
        if (number === undefined) return formatFieldValue(value, field, locale)
        return plain(number / magnitude, locale, field, 2)
      },
    }
  }
  // Nothing to normalise, but a money column still says its currency once.
  if (!currency) return fallback
  return {
    note: currency,
    format: (value: unknown) => {
      const number = scaled(value, field)
      if (number === undefined) return formatFieldValue(value, field, locale)
      return formatFieldValue(number, {
        ...field,
        kind: 'number',
        minorUnits: false,
        currency: undefined,
        symbol: undefined,
      }, locale)
    },
  }
}
