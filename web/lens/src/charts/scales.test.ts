import { describe, expect, it } from 'vitest'
import type { Frame } from '../contract'
import { linearScaleObscuresValues, shouldUseLogarithmicScale } from './scales'

const encoding = { category: 'category', value: 'value' }
const frame = (rows: Frame['rows']): Frame => ({
  columns: [{ name: 'category', type: 'string' }, { name: 'value', type: 'number' }],
  rows,
})

describe('shouldUseLogarithmicScale', () => {
  it('applies a producer-owned spread threshold', () => {
    expect(shouldUseLogarithmicScale(frame([['a', 2], ['b', 20], ['c', 200]]), encoding, { scale: 'logarithmic' }, 100)).toBe(true)
    expect(shouldUseLogarithmicScale(frame([['a', 1], ['b', 100]]), encoding, { scale: 'logarithmic' }, 100)).toBe(false)
    expect(shouldUseLogarithmicScale(frame([['a', 2], ['b', 9], ['c', 99]]), encoding, { scale: 'logarithmic' }, 100)).toBe(false)
  })

  it('honours a valid log request when no product threshold is declared', () => {
    expect(shouldUseLogarithmicScale(frame([['a', 2], ['b', 3], ['c', 4]]), encoding, { scale: 'logarithmic' })).toBe(true)
  })

  it('falls back to linear for non-positive data without a warning', () => {
    expect(shouldUseLogarithmicScale(frame([['a', 0], ['b', 10], ['c', 1000]]), encoding, { scale: 'logarithmic' })).toBe(false)
    expect(shouldUseLogarithmicScale(frame([['a', -1], ['b', 10], ['c', 1000]]), encoding, { scale: 'logarithmic' })).toBe(false)
  })

  it('keeps a real unit value visible by falling back to linear', () => {
    expect(shouldUseLogarithmicScale(frame([['a', 1], ['b', 20], ['c', 1000]]), encoding, { scale: 'logarithmic' })).toBe(false)
  })
})

describe('linearScaleObscuresValues', () => {
  it('recognizes the spread a linear axis cannot show, whatever the producer asked for', () => {
    // «Заявленный ущерб»: one month at 25 млрд against eleven near zero, on a
    // panel that never declared a logarithmic axis and drew eleven baselines.
    const months = Array.from({ length: 11 }, (_, index) => [`m${index}`, 40_000 + index * 1_000] as [string, number])
    expect(linearScaleObscuresValues(frame([...months, ['peak', 25_000_000_000]]), encoding, 100)).toBe(true)
  })

  it('leaves an ordinary spread alone', () => {
    expect(linearScaleObscuresValues(frame([['a', 40], ['b', 55], ['c', 90]]), encoding, 100)).toBe(false)
  })

  it('ignores zeros rather than letting one disqualify the frame', () => {
    expect(linearScaleObscuresValues(frame([['a', 0], ['b', 10], ['c', 100_000]]), encoding, 100)).toBe(true)
  })

  it('does not infer a product readability policy', () => {
    expect(linearScaleObscuresValues(frame([['a', 1], ['b', 100], ['c', 100_000]]), encoding)).toBe(false)
  })
})
