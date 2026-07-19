import { describe, expect, it } from 'vitest'
import { normalizeLensTheme } from './theme'

describe('normalizeLensTheme', () => {
  it.each([
    ['dark', 'dark'],
    [' DARK ', 'light'],
    ['light', 'light'],
    ['system', 'light'],
    [null, 'light'],
  ] as const)('normalizes %s to %s', (input, expected) => {
    expect(normalizeLensTheme(input)).toBe(expected)
  })
})
