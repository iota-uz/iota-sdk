import { cleanup, render } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { parseFormattedValue, StatValueTicker } from './StatValueTicker'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

describe('parseFormattedValue', () => {
  it('recovers value, decimals and separators from grouped money', () => {
    const parsed = parseFormattedValue('1 250 000 UZS')
    expect(parsed).toMatchObject({ prefix: '', suffix: ' UZS', value: 1250000, decimals: 0 })
  })

  it('recovers a comma decimal percent', () => {
    const parsed = parseFormattedValue('42,5%')
    expect(parsed).toMatchObject({ suffix: '%', value: 42.5, decimals: 1, decimalSep: ',' })
  })

  it('recovers a negative value from a leading sign', () => {
    const parsed = parseFormattedValue('-3.4%')
    expect(parsed?.value).toBe(-3.4)
    expect(parsed?.decimals).toBe(1)
  })

  it('returns null when there is no numeric core', () => {
    expect(parseFormattedValue('—')).toBeNull()
    expect(parseFormattedValue('N/A')).toBeNull()
  })
})

describe('StatValueTicker', () => {
  it('renders the formatted string verbatim on first mount', () => {
    const { container } = render(<StatValueTicker text="1 250 000 UZS" />)
    expect(container.textContent).toBe('1 250 000 UZS')
  })

  it('renders the final value immediately under reduced motion', () => {
    vi.stubGlobal('matchMedia', vi.fn().mockReturnValue({ matches: true, addEventListener: vi.fn(), removeEventListener: vi.fn() }))
    const { container, rerender } = render(<StatValueTicker text="10,0%" />)
    rerender(<StatValueTicker text="42,5%" />)
    expect(container.textContent).toBe('42,5%')
  })

  it('swaps directly when the value is not parseable', () => {
    const { container, rerender } = render(<StatValueTicker text="42,5%" />)
    rerender(<StatValueTicker text="—" />)
    expect(container.textContent).toBe('—')
  })
})
