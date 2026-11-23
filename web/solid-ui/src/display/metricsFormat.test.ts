import { describe, expect, it } from 'vitest'
import { compactUnits, formatAmount, formatCompact, formatCount, money, order, sign, type MetricValues } from './metricsFormat'

const nb = '\u00A0'
const minus = '\u2212'
const sp = (text: string) => text.replaceAll(' ', nb)

describe('formatAmount', () => {
  it('matches the templ formatter', () => {
    expect(formatAmount(money(1929625692367))).toBe(sp('19 296 256 923,67'))
    expect(formatAmount(money(5))).toBe('0,05')
    expect(formatAmount({ minor: 1000, decimals: 0 })).toBe(sp('1 000'))
    expect(formatAmount(money(-1250))).toBe('12,50')
    expect(formatCount(2751)).toBe(sp('2 751'))
  })

  it('is exact at int64 extremes', () => {
    expect(formatCount(-9223372036854775808n)).toBe(minus + sp('9 223 372 036 854 775 808'))
    expect(formatAmount(money(-9223372036854775808n))).toBe(sp('92 233 720 368 547 758,08'))
    expect(formatAmount(money('-9223372036854775808'))).toBe(sp('92 233 720 368 547 758,08'))
  })

  it('refuses an unsafe number instead of rounding it silently', () => {
    expect(() => formatAmount(money(2 ** 60))).toThrow(RangeError)
  })
})

describe('sign', () => {
  it('follows the kind, never the colour', () => {
    expect(sign('in', 1)).toBe('+')
    expect(sign('out', 1)).toBe(minus)
    expect(sign('net', 1)).toBe('+')
    expect(sign('net', -1)).toBe(minus)
    expect(sign('net', 0)).toBe('')
    expect(sign('plain', 5)).toBe('')
    expect(sign('plain', -5)).toBe(minus)
  })
})

describe('order', () => {
  const inflow: MetricValues = { values: { USD: money(1), UZS: money(1) } }
  const outflow: MetricValues = { values: { RUB: money(1), EUR: money(1), UZS: money(1) } }

  it('puts the base first, then preference, then the rest by code', () => {
    expect(order('UZS', ['USD'], inflow, outflow)).toEqual(['UZS', 'USD', 'EUR', 'RUB'])
    expect(order('', [], inflow, outflow)).toEqual(['EUR', 'RUB', 'USD', 'UZS'])
  })

  it('skips a base currency without values', () => {
    expect(order('KZT', [], inflow)).toEqual(['USD', 'UZS'])
  })
})

describe('formatCompact', () => {
  it('takes magnitude words from the locale', () => {
    expect(compactUnits('ru')).toEqual(['млн', 'млрд', 'трлн'])
    const words = [1e6, 1e9, 1e12].map((value) => new Intl.NumberFormat('en-US', { notation: 'compact' })
      .formatToParts(value).find((part) => part.type === 'compact')?.value)
    expect(compactUnits('en-US')).toEqual(words)
  })

  it('prints a comma mantissa with the magnitude word', () => {
    expect(formatCompact(money(1929625692367n), { locale: 'ru' })).toEqual({ number: '19,30', unit: 'млрд' })
    expect(formatCompact(money(1929625692367n), { locale: 'ru', digits: 1 })).toEqual({ number: '19,3', unit: 'млрд' })
    expect(formatCompact(money(120400000000000n), { locale: 'ru' })).toEqual({ number: '1,20', unit: 'трлн' })
    expect(formatCompact(money(-91674301591n), { locale: 'ru' })).toEqual({ number: '916,74', unit: 'млн' })
  })

  it('keeps the full value below a million', () => {
    expect(formatCompact(money(99999999), { locale: 'ru' })).toEqual({ number: sp('999 999,99'), unit: '' })
    expect(formatCompact({ minor: 12480, decimals: 0 })).toEqual({ number: sp('12 480'), unit: '' })
  })

  it('rolls a rounded mantissa over into the next magnitude', () => {
    expect(formatCompact({ minor: 999995000, decimals: 0 }, { locale: 'ru' })).toEqual({ number: '1,00', unit: 'млрд' })
    expect(formatCompact({ minor: 999994999, decimals: 0 }, { locale: 'ru' })).toEqual({ number: '999,99', unit: 'млн' })
    expect(formatCompact({ minor: 999999, decimals: 0 }, { locale: 'ru' })).toEqual({ number: sp('999 999'), unit: '' })
  })

  it('is exact at int64 extremes', () => {
    expect(formatCompact(money(-9223372036854775808n), { locale: 'ru' })).toEqual({ number: sp('92 233,72'), unit: 'трлн' })
  })
})
