import { act, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import fixture from '../fixtures/small.json'
import { LensDashboard } from './LensDashboard'
import { registerLensDashboardElement } from './element'

afterEach(() => {
  document.body.replaceChildren()
  vi.unstubAllGlobals()
})

describe('LensDashboard', () => {
  it('renders the bundled document when src is omitted', () => {
    render(<LensDashboard locale="en" />)

    expect(screen.getByText('42')).toBeInTheDocument()
    expect(screen.getByText(/Bundled fixture/)).toBeInTheDocument()
  })

  it('loads a document with same-origin credentials and csrf', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(fixture),
    })
    vi.stubGlobal('fetch', fetchMock)

    render(<LensDashboard src="/lens/example" locale="en" csrf="token" />)

    await waitFor(() => expect(screen.getByText('42')).toBeInTheDocument())
    expect(fetchMock).toHaveBeenCalledWith(
      '/lens/example',
      expect.objectContaining({
        credentials: 'same-origin',
        headers: { 'X-CSRF-Token': 'token' },
      }),
    )
  })
})

describe('<lens-dashboard>', () => {
  it('re-renders on attribute changes and unmounts on disconnect', () => {
    registerLensDashboardElement()
    const element = document.createElement('lens-dashboard')

    act(() => document.body.append(element))
    expect(element.querySelector('[data-theme="light"]')).not.toBeNull()

    act(() => element.setAttribute('theme', 'dark'))
    expect(element.querySelector('[data-theme="dark"]')).not.toBeNull()

    act(() => element.remove())
    expect(element.childElementCount).toBe(0)
  })
})
