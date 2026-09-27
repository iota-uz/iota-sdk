import { describe, expect, it } from 'vitest'
import { distributeShares, formatShare, shareOf } from './shares'

describe('shareOf', () => {
  it('measures a value against the given total', () => {
    expect(shareOf(25, 200)).toBe(12.5)
  })

  it('has no answer without a usable total', () => {
    expect(shareOf(25, 0)).toBeUndefined()
    expect(shareOf(25, undefined)).toBeUndefined()
    expect(shareOf(undefined, 200)).toBeUndefined()
  })
})

describe('distributeShares', () => {
  it('reconciles a partition to exactly 100%', () => {
    // Rounded independently these are 33.3 + 33.3 + 33.3 = 99.9.
    const shares = distributeShares([1, 1, 1], 3)
    expect(shares).toEqual([33.4, 33.3, 33.3])
    // Summed in tenths: adding the decimals back up is float arithmetic, and
    // what has to be exact is the printed set, not a float reduction of it.
    expect(shares.reduce<number>((sum, share) => sum + Math.round((share ?? 0) * 10), 0)).toBe(1000)
  })

  it('gives the correction to the largest remainders, not the largest slice', () => {
    // Remainders: 0.5 (a), 0.25 (b, c), 0.0 (d) — only `a` is topped up.
    const shares = distributeShares([4005, 3002.5, 3002.5, 9990], 20000)
    expect(shares).toEqual([20, 15, 15, 50])
  })

  it('measures against the declared total, not the rows', () => {
    // The rows are a subset — a collapsed tail, or a hidden legend entry.
    expect(distributeShares([30, 20], 200)).toEqual([15, 10])
  })

  it('leaves a non-partition alone rather than inventing share', () => {
    const shares = distributeShares([1, 1], 3)
    expect(shares).toEqual([33.3, 33.3])
  })

  it('does not reconcile across a negative value', () => {
    expect(distributeShares([-50, 150], 100)).toEqual([-50, 150])
  })

  it('has no shares without a usable total', () => {
    expect(distributeShares([1, 2], 0)).toEqual([undefined, undefined])
    expect(distributeShares([1, 2], undefined)).toEqual([undefined, undefined])
  })

  it('keeps a missing value missing', () => {
    expect(distributeShares([50, undefined], 100)).toEqual([50, undefined])
  })
})

describe('formatShare', () => {
  it('prints one decimal', () => {
    expect(formatShare(8.8)).toBe('8.8%')
    expect(formatShare(50)).toBe('50.0%')
  })

  it('prints nothing for an absent share', () => {
    expect(formatShare(undefined)).toBe('')
  })
})
