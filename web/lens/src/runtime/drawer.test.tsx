import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { ClientHostProvider } from '@iota-uz/client-host'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Action, DashboardDocument } from '../contract'
import { LensDashboard } from '../LensDashboard'
import { LensDrawer } from './drawer'

function statDocument(title: string, action?: Action): DashboardDocument {
  return {
    version: '1.0.0',
    snapshotId: `snapshot-${title}`,
    meta: { dashboardId: title, title, generatedAt: '2026-07-22T00:00:00Z', locale: 'en' },
    layout: { rows: [{ panels: [{ panelId: 'metric', span: 12 }] }] },
    panels: [{
      id: 'metric', kind: 'stat', semantics: 'series', title: `${title} metric`, frame: 'metric-frame',
      encoding: { value: 'value' }, format: {}, actions: action ? [action] : [], terminal: !action,
    }],
    frames: { 'metric-frame': { columns: [{ name: 'value', type: 'number' }], rows: [[42]] } },
    drill: { inlineDepth: 0, edges: {} },
    perspectives: [],
    endpoints: {},
    i18n: {},
    theme: { palette: {}, series: {} },
  }
}

const drawerAction: Action = {
  kind: 'open_drawer', method: 'GET', urlTemplate: '/drill/loss/lens/document?token=signed', params: [], payload: {},
}

function renderWithClientHost(children: React.ReactNode) {
  const background = globalThis.document.createElement('main')
  const portalOwner = globalThis.document.createElement('div')
  globalThis.document.body.append(background, portalOwner)
  return render(
    <ClientHostProvider background={background} portalOwner={portalOwner}>
      {children}
    </ClientHostProvider>,
    { container: background },
  )
}

// A drawer-hosted document carries its own identity block and an empty meta
// title, so the drawer chrome owns the single heading and the body does not
// repeat it.
function drawerHostedDocument(): DashboardDocument {
  const base = statDocument('')
  return {
    ...base,
    drawer: { eyebrow: 'Cash result', title: 'ОСАГО ОБ-10-1', caption: '2025\nПериод' },
  }
}

beforeEach(() => {
  window.history.replaceState(null, '', '/dashboard?tenant=kept')
})

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

describe('Lens drawer host', () => {
  it('resolves a stable metric key at open time and stores only the relative drawer URL', async () => {
    const action: Action = {
      kind: 'open_drawer', drawerKey: { kind: 'literal', value: 'loss-ratio' }, params: [], payload: {},
    }
    const initial = { ...statDocument('Profitability', action), endpoints: { drawer: '/lens/drawer' } }
    const calls: Array<{ url: string; body?: string }> = []
    const fetcher = vi.fn<typeof fetch>().mockImplementation((input, init) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
      calls.push({ url, ...(typeof init?.body === 'string' ? { body: init.body } : {}) })
      if (url === '/lens/drawer') return Promise.resolve(new Response(JSON.stringify({ url: 'http://localhost:3000/drill/loss/lens/document?ticket=short' }), { status: 200 }))
      return Promise.resolve(new Response(JSON.stringify(statDocument('Resolved detail')), { status: 200 }))
    })
    render(<LensDashboard initialDocument={initial} fetcher={fetcher} />)

    fireEvent.click(screen.getByRole('link', { name: 'Open Profitability metric' }))
    expect(await screen.findByRole('heading', { name: 'Resolved detail' })).toBeInTheDocument()
    expect(JSON.parse(calls[0]?.body ?? '{}')).toEqual({ snapshotId: 'snapshot-Profitability', metricKey: 'loss-ratio' })
    expect(new URL(window.location.href).searchParams.get('drawer')).toBe('/drill/loss/lens/document?ticket=short')
  })

  it('keeps the dashboard mounted, uses browser history, and restores focus on Back', async () => {
    const drawerDocument = statDocument('Loss ratio detail')
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(new Response(JSON.stringify(drawerDocument), {
      status: 200, headers: { 'Content-Type': 'application/json' },
    }))
    render(<LensDashboard initialDocument={statDocument('Profitability', drawerAction)} fetcher={fetcher} />)
    const opener = screen.getByRole('link', { name: 'Open Profitability metric' })

    opener.focus()
    fireEvent.click(opener)
    expect(await screen.findByRole('dialog', { name: 'Drill details' })).toBeInTheDocument()
    expect(opener.isConnected).toBe(true)
    expect(window.location.pathname).toBe('/dashboard')
    expect(new URL(window.location.href).searchParams.get('drawer')).toContain('/drill/loss/lens/document')
    expect(screen.getByRole('heading', { name: 'Profitability', hidden: true })).toBeInTheDocument()
    expect(await screen.findByRole('heading', { name: 'Loss ratio detail' })).toBeInTheDocument()
    expect(globalThis.document.documentElement.style.overflow).toBe('hidden')

    act(() => window.history.back())
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    await waitFor(() => expect(opener).toHaveFocus())
    expect(opener.isConnected).toBe(true)
    expect(fetcher).toHaveBeenCalledTimes(1)
    expect(globalThis.document.documentElement.style.overflow).toBe('')
  })

  it('traps focus and replaces the current drawer document instead of nesting another modal', async () => {
    const nextAction: Action = {
      ...drawerAction,
      urlTemplate: '/drill/expenses/lens/document?token=signed',
    }
    const nested = statDocument('Result waterfall', nextAction)
    const next = statDocument('Expenses focus')
    const fetcher = vi.fn<typeof fetch>().mockImplementation((input) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
      return Promise.resolve(
        new Response(JSON.stringify(url.includes('/expenses/') ? next : nested), { status: 200 }),
      )
    })
    const historyGo = vi.spyOn(window.history, 'go').mockImplementation(() => undefined)
    render(<LensDashboard initialDocument={statDocument('Dashboard', drawerAction)} fetcher={fetcher} />)
    fireEvent.click(screen.getByRole('link', { name: 'Open Dashboard metric' }))

    const dialog = await screen.findByRole('dialog')
    const close = screen.getByRole('button', { name: 'Close details' })
    expect(dialog).toContainElement(globalThis.document.activeElement as HTMLElement)
    fireEvent.click(await screen.findByRole('link', { name: 'Open Result waterfall metric' }))
    expect(await screen.findByRole('heading', { name: 'Expenses focus' })).toBeInTheDocument()
    expect(new URL(window.location.href).searchParams.get('drawer')).toContain('/drill/expenses/lens/document')
    expect(screen.getAllByRole('dialog')).toHaveLength(1)
    close.focus()
    fireEvent.keyDown(dialog, { key: 'Tab', shiftKey: true })
    expect(dialog).toContainElement(globalThis.document.activeElement as HTMLElement)
    fireEvent.keyDown(dialog, { key: 'Escape' })
    expect(historyGo).toHaveBeenCalledWith(-2)
  })

  it('renders the document drawer header once and drops the repeated body heading', async () => {
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(new Response(JSON.stringify(drawerHostedDocument()), {
      status: 200, headers: { 'Content-Type': 'application/json' },
    }))
    render(<LensDashboard initialDocument={statDocument('Profitability', drawerAction)} fetcher={fetcher} />)
    fireEvent.click(screen.getByRole('link', { name: 'Open Profitability metric' }))

    const dialog = await screen.findByRole('dialog', { name: 'Drill details' })
    // Eyebrow = metric, title = scope, caption = period/note — the drawer's own
    // top-bar identity block, not the generic 'Detail view' fallback.
    expect(screen.getByText('Cash result')).toBeInTheDocument()
    expect(screen.getByText('ОСАГО ОБ-10-1')).toBeInTheDocument()
    expect(screen.getByText(/Период/)).toBeInTheDocument()
    // An empty document title means the body renders no dashboard heading, so the
    // scope is stated exactly once (in the drawer chrome).
    expect(dialog.querySelector('.lens-dashboard-header')).toBeNull()
  })

  it('closes on a mousedown directly on the backdrop but not inside the dialog', () => {
    const onClose = vi.fn()
    renderWithClientHost(
      <LensDrawer closeLabel="Close details" eyebrow="Drill" label="Drill details" onClose={onClose}>
        <p>Body content</p>
      </LensDrawer>,
    )
    const dialog = screen.getByRole('dialog', { name: 'Drill details' })
    const backdrop = dialog.querySelector<HTMLElement>('.lens-drawer-backdrop')!

    // A mousedown that lands on a child of the dialog must not dismiss.
    fireEvent.mouseDown(screen.getByText('Body content'))
    fireEvent.mouseDown(dialog)
    expect(onClose).not.toHaveBeenCalled()

    // Only a mousedown directly on the backdrop dismisses.
    fireEvent.mouseDown(backdrop)
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('rejects a cross-origin drawer document', () => {
    const action: Action = { ...drawerAction, urlTemplate: 'https://example.test/lens/document' }
    const fetcher = vi.fn<typeof fetch>()
    render(<LensDashboard initialDocument={statDocument('Dashboard', action)} fetcher={fetcher} />)

    expect(screen.queryByRole('link', { name: 'Open Dashboard metric' })).not.toBeInTheDocument()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(fetcher).not.toHaveBeenCalled()
  })
})
