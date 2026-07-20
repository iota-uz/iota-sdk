import { describe, expect, it } from 'vitest'
import type { FieldFormat } from '../contract'
import { formatAxis, formatFieldValue } from './format'

describe('formatFieldValue', () => {
  it.each<{ name: string; value: unknown; field?: FieldFormat; expected: string }>([
    { name: 'default number', value: 1234.5, expected: '1,234.5' },
    { name: 'precision', value: 12.345, field: { kind: 'number', minorUnits: false, precision: 2 }, expected: '12.35' },
    { name: 'major-unit money', value: 123.45, field: { kind: 'money', currency: 'USD', minorUnits: false, precision: 2 }, expected: '$123.45' },
    { name: 'minor-unit money', value: 12345, field: { kind: 'money', currency: 'USD', minorUnits: true, precision: 2 }, expected: '$123.45' },
    { name: 'minor-unit scale ignores display precision', value: 12345, field: { kind: 'money', currency: 'USD', minorUnits: true, precision: 0 }, expected: '$123' },
    { name: 'percent is already percentage points', value: 7.5, field: { kind: 'percent', minorUnits: false, precision: 1 }, expected: '7.5%' },
    { name: 'Go date layout', value: '2026-07-19T09:30:00Z', field: { kind: 'date', minorUnits: false, layout: '02 Jan 2006 15:04' }, expected: '19 Jul 2026 09:30' },
    { name: 'string', value: 'ready', field: { kind: 'string', minorUnits: false }, expected: 'ready' },
    { name: 'null', value: null, field: { kind: 'string', minorUnits: false }, expected: '—' },
  ])('$name', ({ value, field, expected }) => {
    expect(formatFieldValue(value, field, 'en-US')).toBe(expected)
  })

  it.each(['UZS', 'JPY', 'KWD'])('always scales %s minor units by 100', (currency) => {
    const value = 1_234_567
    const expected = new Intl.NumberFormat('en-US', { style: 'currency', currency }).format(value / 100)
    const digits = (formatted: string) => formatted.replace(/\D/g, '')

    expect(digits(formatFieldValue(value, { kind: 'money', currency, minorUnits: true }, 'en-US')))
      .toBe(digits(expected))
  })
})

describe('formatAxis', () => {
  it('renders large money values with compact notation', () => {
    const expected = new Intl.NumberFormat('en-US', {
      style: 'currency', currency: 'USD', notation: 'compact', maximumFractionDigits: 1,
    }).format(1_200_000_000)
    expect(formatAxis(1_200_000_000, { kind: 'money', currency: 'USD', minorUnits: false }, 'en-US')).toBe(expected)
  })

  it('scales minor-unit money before compacting', () => {
    const expected = new Intl.NumberFormat('en-US', {
      style: 'currency', currency: 'UZS', notation: 'compact', maximumFractionDigits: 1,
    }).format(12_000_000)
    expect(formatAxis(1_200_000_000, { kind: 'money', currency: 'UZS', minorUnits: true }, 'en-US')).toBe(expected)
  })

  it('is locale-aware for compact money', () => {
    const value = formatAxis(1_200_000_000, { kind: 'money', currency: 'UZS', minorUnits: false }, 'ru-RU')
    expect(value).toContain('млрд')
  })

  it('compacts plain numbers', () => {
    expect(formatAxis(1_500_000, { kind: 'number', minorUnits: false }, 'en-US')).toBe('1.5M')
  })

  it('delegates non-numeric formats to formatFieldValue', () => {
    const field: FieldFormat = { kind: 'date', minorUnits: false, layout: '2006-01-02' }
    expect(formatAxis('2026-07-20T00:00:00Z', field, 'en-US')).toBe(formatFieldValue('2026-07-20T00:00:00Z', field, 'en-US'))
  })
})

describe('compact formatting', () => {
  it.each([
    { locale: 'ru-RU', expected: '9.36 млрд' },
    { locale: 'uz-UZ', expected: '9.36 mlrd' },
    { locale: 'en-US', expected: '9.36B' },
  ])('takes magnitude words from $locale CLDR data with a pinned decimal separator', ({ locale, expected }) => {
    const field: FieldFormat = { kind: 'number', minorUnits: false, precision: 2, compact: true, decimalSeparator: '.' }
    expect(formatFieldValue(9_364_442_607, field, locale)).toBe(expected)
  })

  it('follows the locale separator when none is pinned', () => {
    const field: FieldFormat = { kind: 'number', minorUnits: false, precision: 2, compact: true }
    expect(formatFieldValue(9_364_442_607, field, 'ru-RU').replace(/\u00A0/g, ' ')).toBe('9,36 млрд')
  })

  it('appends the currency code to compact money instead of a symbol', () => {
    const field: FieldFormat = {
      kind: 'money', currency: 'UZS', minorUnits: false, precision: 2, compact: true, decimalSeparator: '.',
    }
    expect(formatFieldValue(230_310_000_000, field, 'ru-RU')).toBe('230.31 млрд UZS')
  })

  it('renders the pinned currency grapheme instead of the ISO code', () => {
    const field: FieldFormat = {
      kind: 'money', currency: 'UZS', minorUnits: false, precision: 2, symbol: 'so’m', decimalSeparator: '.',
    }
    expect(formatFieldValue(66_856_663_843.68, field, 'ru-RU')).toBe('66 856 663 843.68 so’m')
  })

  it('scales minor units before compacting', () => {
    const field: FieldFormat = {
      kind: 'money', currency: 'UZS', minorUnits: true, precision: 2, compact: true, decimalSeparator: '.',
    }
    expect(formatFieldValue(150_530_000_00, field, 'ru-RU')).toBe('150.53 млн UZS')
  })

  it('pins the separator for percents too', () => {
    const field: FieldFormat = { kind: 'percent', minorUnits: false, precision: 1, decimalSeparator: '.' }
    expect(formatFieldValue(47.14, field, 'ru-RU')).toBe('47.1%')
  })
})

describe('zero precision is a value, not an absence', () => {
  it('renders whole units for a symbol-money field asking for precision 0', () => {
    const field: FieldFormat = {
      kind: 'money', currency: 'UZS', minorUnits: false, precision: 0, symbol: 'so’m', decimalSeparator: '.',
    }
    // Dropping the 0 leaves Intl at its default fraction digits, which is how
    // "51 522 007 533,993 so’m" reached a headline that asked for whole units.
    expect(formatFieldValue(51_522_007_533.993, field, 'ru')).toBe('51 522 007 534 so’m')
  })

  it('keeps whole units for plain numbers and percentages', () => {
    expect(formatFieldValue(12.345, { kind: 'number', minorUnits: false, precision: 0 }, 'en-US')).toBe('12')
    expect(formatFieldValue(47.06, { kind: 'percent', minorUnits: false, precision: 0 }, 'en-US')).toBe('47%')
  })
})
