import { describe, expect, it } from 'vitest'
import { describeError, formatTime, rightsOf, shortHash } from './context'

describe('context helpers', () => {
  it('derives the rights the page offers from the session permissions', () => {
    expect(rightsOf(['Extensions.Read', 'Extensions.Author'])).toEqual({ author: true, review: false, publish: false, operate: false })
    expect(rightsOf(undefined)).toEqual({ author: false, review: false, publish: false, operate: false })
  })

  it('abbreviates hashes and formats times without throwing on bad input', () => {
    expect(shortHash('src-0123456789abcdef')).toBe('0123456789ab')
    expect(shortHash(undefined)).toBe('')
    expect(formatTime('not a date')).toBe('not a date')
    expect(formatTime(null)).toBe('')
    expect(describeError(new Error('x'))).toBe('x')
    expect(describeError('y')).toBe('y')
  })
})
