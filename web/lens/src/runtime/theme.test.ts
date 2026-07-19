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

  it('uses a dark root class when no explicit valid theme is set', () => {
    expect(normalizeLensTheme(undefined, true)).toBe('dark')
    expect(normalizeLensTheme('system', true)).toBe('dark')
  })

  it('gives an explicit theme precedence over the root class', () => {
    expect(normalizeLensTheme('light', true)).toBe('light')
    expect(normalizeLensTheme('dark', false)).toBe('dark')
  })
})
