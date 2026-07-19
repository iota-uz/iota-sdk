import { act, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import fixture from '../fixtures/small.json'
import { LensDashboard } from './LensDashboard'
import { parseDocument } from './contract'
import { registerLensDashboardElement } from './element'

afterEach(() => {
  document.body.replaceChildren()
  vi.unstubAllGlobals()
})

describe('LensDashboard', () => {
  it('renders the bundled document when src is omitted', () => {
    const view = render(<LensDashboard locale="en" />)

    expect(screen.getByText('$4,286,000')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Operations overview' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Headline metrics' })).toBeInTheDocument()
    expect(view.container.querySelector('[style*="--lens-panel-span: 4"]')).not.toBeNull()
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

  it('renders initial loading and fetch error states', async () => {
    let rejectRequest: ((reason: Error) => void) | undefined
    const fetcher = vi.fn(() => new Promise<Response>((_resolve, reject) => { rejectRequest = reject }))
    render(<LensDashboard src="/lens/document" fetcher={fetcher} />)

    expect(screen.getByText('Loading dashboard…')).toHaveAttribute('aria-busy', 'true')
    rejectRequest?.(new Error('offline'))
    expect(await screen.findByRole('alert')).toHaveTextContent('Unable to load Lens document: offline')
  })

  it('renders the no-panel fallback', () => {
    const empty = parseDocument({
      ...fixture,
      layout: { rows: [] },
      panels: [],
    })
    render(<LensDashboard initialDocument={empty} />)
    expect(screen.getByText('The document contains no panels.')).toBeInTheDocument()
  })

  it('aborts the document fetch on unmount', () => {
    let signal: AbortSignal | undefined
    const fetcher = vi.fn<typeof fetch>().mockImplementation((_input, init) => {
      signal = init?.signal as AbortSignal
      return new Promise<Response>(() => undefined)
    })
    const view = render(<LensDashboard src="/lens/document" fetcher={fetcher} />)
    view.unmount()
    expect(signal?.aborted).toBe(true)
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
    expect(Reflect.get(element, 'root')).toBeUndefined()
  })
})
