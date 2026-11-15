import { afterEach, describe, expect, it, vi } from 'vitest'
import { detectAppTheme, normalizeLensTheme, observeAppTheme, parseLensThemeSetting } from './theme'

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

describe('parseLensThemeSetting', () => {
  it.each([
    ['auto', 'auto'],
    ['dark', 'dark'],
    ['light', 'light'],
    ['system', 'light'],
    [null, 'light'],
  ] as const)('reads %s as %s', (input, expected) => {
    expect(parseLensThemeSetting(input)).toBe(expected)
  })
})

describe('detectAppTheme', () => {
  const root = (...classes: Array<string>) => {
    const element = globalThis.document.createElement('html')
    element.classList.add(...classes)
    return element
  }

  it('follows the dark class the host application sets', () => {
    expect(detectAppTheme(root('dark'), { matches: false })).toBe('dark')
  })

  it('lets an explicit light class beat a dark system scheme', () => {
    expect(detectAppTheme(root('light'), { matches: true })).toBe('light')
  })

  it('falls back to the system scheme when the page declares nothing', () => {
    expect(detectAppTheme(root(), { matches: true })).toBe('dark')
    expect(detectAppTheme(root(), { matches: false })).toBe('light')
    expect(detectAppTheme(root(), undefined)).toBe('light')
  })
})

describe('observeAppTheme', () => {
  afterEach(() => {
    globalThis.document.documentElement.classList.remove('dark')
  })

  it('reports class changes on <html> and stops after disposal', async () => {
    const seen: Array<string> = []
    const dispose = observeAppTheme((theme) => seen.push(theme))
    globalThis.document.documentElement.classList.add('dark')
    await vi.waitFor(() => expect(seen.at(-1)).toBe('dark'))
    globalThis.document.documentElement.classList.remove('dark')
    await vi.waitFor(() => expect(seen.at(-1)).toBe('light'))
    dispose()
    const count = seen.length
    globalThis.document.documentElement.classList.add('dark')
    await new Promise((resolve) => setTimeout(resolve, 20))
    expect(seen).toHaveLength(count)
  })
})
