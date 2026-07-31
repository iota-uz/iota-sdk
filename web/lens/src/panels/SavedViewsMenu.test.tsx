import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import fixture from '../../fixtures/small.json'
import { parseDocument } from '../contract'
import { DashboardPanels } from '../DashboardPanels'
import { DashboardRuntimeProvider, DocumentProvider } from '../runtime'
import { roleDefaultURL, SavedViewsMenu } from './SavedViewsMenu'

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  document.head.querySelectorAll('meta[name="csrf-token"], meta[name="csrf"]').forEach((element) => element.remove())
  document.body.replaceChildren()
  sessionStorage.clear()
})

describe('SavedViewsMenu', () => {
  it('applies a role default only to a clean Lens URL and guards repeat redirects', () => {
    const document_ = parseDocument(fixture)
    const defaultView = {
      id: 'role-default', dashboardId: document_.meta.dashboardId, name: 'Analyst default', scope: 'team' as const,
      stateUrl: '/analytics/claims?_f=status%3Aopen&lensHidden=%5B%22losses%22%2C%22paid%22%5D', defaultRoleId: 4,
    }
    const clean = new URL('https://example.com/analytics/claims?lang=en')
    expect(roleDefaultURL(document_, clean, defaultView, sessionStorage)).toBe(defaultView.stateUrl)
    expect(roleDefaultURL(document_, clean, defaultView, sessionStorage)).toBeUndefined()
    expect(roleDefaultURL(document_, new URL('https://example.com/analytics/claims?path=root'), defaultView, sessionStorage)).toBeUndefined()
    expect(roleDefaultURL(document_, new URL('https://example.com/analytics/claims?_f=status%3Aclosed'), defaultView, sessionStorage)).toBeUndefined()
  })

  it('lists, saves, and schedules exact dashboard slices', async () => {
    history.replaceState({}, '', '/analytics/claims?year=2026&lensHidden=paid#panel-losses')
    const meta = document.createElement('meta')
    meta.name = 'csrf-token'
    meta.content = 'csrf-meta-token'
    document.head.append(meta)
    const requests: Array<{ url: string; init?: RequestInit }> = []
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
      requests.push({ url, init })
      if (init?.method === 'POST' && url === '/lens/share/views') {
        return Promise.resolve(new Response(JSON.stringify({ id: 'view-new' }), { status: 200 }))
      }
      if (init?.method === 'POST' && url === '/lens/share/schedules') {
        return Promise.resolve(new Response(JSON.stringify({ id: 'schedule-new' }), { status: 200 }))
      }
      if (url.startsWith('/lens/share/schedules?')) {
        return Promise.resolve(new Response(JSON.stringify({ schedules: [] }), { status: 200 }))
      }
      return Promise.resolve(new Response(JSON.stringify({
        views: [{ id: 'view-1', dashboardId: 'small', name: 'Team baseline', scope: 'team', stateUrl: '/claims?year=2026' }],
        capabilities: { manageTeam: true, scheduleMail: true, roles: [{ id: 4, name: 'Analyst' }] },
      }), { status: 200 }))
    }))
    const document_ = parseDocument({
      ...fixture,
      endpoints: { ...fixture.endpoints, views: '/lens/share/views', schedules: '/lens/share/schedules' },
    })
    render(
      <DocumentProvider initialDocument={document_}>
        <DashboardRuntimeProvider locale="en"><DashboardPanels /></DashboardRuntimeProvider>
      </DocumentProvider>,
    )

    fireEvent.click(screen.getByRole('button', { name: 'Views' }))
    expect((await screen.findAllByText('Team baseline'))[0]).toBeInTheDocument()
    fireEvent.change(screen.getByRole('textbox', { name: 'View name' }), { target: { value: 'My current slice' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(requests.some(({ url, init }) => url === '/lens/share/views' && init?.method === 'POST')).toBe(true))
    const saved = requests.find(({ url, init }) => url === '/lens/share/views' && init?.method === 'POST')
    const body = saved?.init?.body
    expect(typeof body === 'string' ? JSON.parse(body) : undefined).toMatchObject({
      name: 'My current slice', scope: 'personal', stateUrl: '/analytics/claims?year=2026&lensHidden=paid#panel-losses',
    })
    expect(new Headers(saved?.init?.headers).get('X-CSRF-Token')).toBe('csrf-meta-token')

    fireEvent.change(screen.getByRole('textbox', { name: 'Recipients' }), { target: { value: 'analyst@example.com' } })
    fireEvent.click(screen.getByRole('button', { name: 'Schedule XLSX' }))
    await waitFor(() => expect(requests.some(({ url, init }) => url === '/lens/share/schedules' && init?.method === 'POST')).toBe(true))
  })

  it('hides unauthorized destructive and scheduling controls and restores focus on Escape', async () => {
    const fetcher = vi.fn((input: RequestInfo | URL) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
      if (url.includes('/schedules')) throw new Error('schedule endpoint must remain permission-gated')
      return Promise.resolve(new Response(JSON.stringify({
        views: [
          { id: 'team', dashboardId: 'small', name: 'Team baseline', scope: 'team', stateUrl: '/claims?year=2026' },
          { id: 'mine', dashboardId: 'small', name: 'My slice', scope: 'personal', stateUrl: '/claims?year=2025' },
        ],
        capabilities: { manageTeam: false, scheduleMail: false },
      }), { status: 200 }))
    })
    vi.stubGlobal('fetch', fetcher)
    const document_ = parseDocument({ ...fixture, endpoints: { ...fixture.endpoints, views: '/lens/share/views', schedules: '/lens/share/schedules' } })
    render(
      <DocumentProvider initialDocument={document_}>
        <DashboardRuntimeProvider locale="en"><DashboardPanels /></DashboardRuntimeProvider>
      </DocumentProvider>,
    )

    const trigger = screen.getByRole('button', { name: 'Views' })
    fireEvent.click(trigger)
    await screen.findByRole('link', { name: /Team baseline/ })
    expect(screen.queryByRole('button', { name: 'Delete Team baseline' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Delete My slice' })).toBeInTheDocument()
    expect(screen.queryByText('Email schedule')).not.toBeInTheDocument()
    expect(fetcher.mock.calls.some(([input]) => {
      const href = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
      return href.includes('/schedules')
    })).toBe(false)
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(screen.queryByRole('dialog', { name: 'Saved views' })).not.toBeInTheDocument()
    expect(trigger).toHaveFocus()
  })

  it('reads the CSRF token from the Lens shadow host', async () => {
    const requests: RequestInit[] = []
    vi.stubGlobal('fetch', vi.fn((_input: RequestInfo | URL, init?: RequestInit) => {
      requests.push(init ?? {})
      return Promise.resolve(new Response(JSON.stringify({ views: [], capabilities: { manageTeam: false, scheduleMail: false } }), { status: 200 }))
    }))
    const host = document.createElement('lens-dashboard')
    host.setAttribute('csrf', 'csrf-shadow-token')
    const shadow = host.attachShadow({ mode: 'open' })
    const mount = document.createElement('div')
    shadow.append(mount)
    document.body.append(host)
    const document_ = parseDocument({ ...fixture, endpoints: { ...fixture.endpoints, views: '/lens/share/views' } })
    const view = render(
      <DocumentProvider initialDocument={document_}>
        <DashboardRuntimeProvider locale="en"><SavedViewsMenu /></DashboardRuntimeProvider>
      </DocumentProvider>,
      { container: mount },
    )
    fireEvent.click(view.getByRole('button', { name: 'Views' }))
    await view.findByText('No saved views')
    expect(new Headers(requests[0]?.headers).get('X-CSRF-Token')).toBe('csrf-shadow-token')
  })

  it('does not reload views when progressive panel hydration replaces the document', async () => {
    const fetcher = vi.fn(() => Promise.resolve(new Response(JSON.stringify({
      views: [], capabilities: { manageTeam: false, scheduleMail: false },
    }), { status: 200 })))
    vi.stubGlobal('fetch', fetcher)
    const initial = parseDocument({ ...fixture, endpoints: { ...fixture.endpoints, views: '/lens/share/views' } })
    const hydrated = parseDocument({
      ...fixture,
      meta: { ...fixture.meta, generatedAt: '2026-07-31T12:01:00Z' },
      endpoints: { ...fixture.endpoints, views: '/lens/share/views' },
    })
    const view = render(
      <DocumentProvider initialDocument={initial}>
        <DashboardRuntimeProvider locale="en"><SavedViewsMenu /></DashboardRuntimeProvider>
      </DocumentProvider>,
    )
    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(1))

    view.rerender(
      <DocumentProvider initialDocument={hydrated}>
        <DashboardRuntimeProvider locale="en"><SavedViewsMenu /></DashboardRuntimeProvider>
      </DocumentProvider>,
    )
    await waitFor(() => expect(screen.getByRole('button', { name: 'Views' })).toBeInTheDocument())
    expect(fetcher).toHaveBeenCalledTimes(1)
  })

  it('never sends credentials or the CSRF token to a document-supplied cross-origin sharing endpoint', async () => {
    const fetcher = vi.fn()
    vi.stubGlobal('fetch', fetcher)
    const document_ = parseDocument({
      ...fixture,
      endpoints: { ...fixture.endpoints, views: 'https://attacker.example/lens/share/views' },
    })
    render(
      <DocumentProvider initialDocument={document_}>
        <DashboardRuntimeProvider locale="en"><SavedViewsMenu /></DashboardRuntimeProvider>
      </DocumentProvider>,
    )

    fireEvent.click(screen.getByRole('button', { name: 'Views' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('sharing endpoint must be site-relative')
    expect(fetcher).not.toHaveBeenCalled()
  })
})
