import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
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
      encoding: { value: 'value' }, format: {}, actions: action ? [action] : [],
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

beforeEach(() => {
  window.history.replaceState(null, '', '/dashboard?tenant=kept')
})

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

describe('Lens drawer host', () => {
  it('keeps the dashboard mounted, uses browser history, and restores focus on Back', async () => {
    const drawerDocument = statDocument('Loss ratio detail')
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(new Response(JSON.stringify(drawerDocument), {
      status: 200, headers: { 'Content-Type': 'application/json' },
    }))
    render(<LensDashboard initialDocument={statDocument('Profitability', drawerAction)} fetcher={fetcher} />)
    const opener = screen.getByRole('link', { name: 'Open Profitability metric' })

    fireEvent.click(opener)
    expect(await screen.findByRole('dialog', { name: 'Drill details' })).toBeInTheDocument()
    expect(opener.isConnected).toBe(true)
    expect(window.location.pathname).toBe('/dashboard')
    expect(new URL(window.location.href).searchParams.get('drawer')).toContain('/drill/loss/lens/document')
    expect(screen.getByRole('heading', { name: 'Profitability', hidden: true })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Loss ratio detail' })).toBeInTheDocument()
    expect(globalThis.document.body.style.overflow).toBe('hidden')

    act(() => window.history.back())
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    await waitFor(() => expect(opener).toHaveFocus())
    expect(opener.isConnected).toBe(true)
    expect(fetcher).toHaveBeenCalledTimes(1)
    expect(globalThis.document.body.style.overflow).toBe('')
  })

  it('traps focus, closes through history on Escape, and rejects a nested drawer', async () => {
    const nested = statDocument('Drawer document', drawerAction)
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(new Response(JSON.stringify(nested), { status: 200 }))
    const historyGo = vi.spyOn(window.history, 'go').mockImplementation(() => undefined)
    render(<LensDashboard initialDocument={statDocument('Dashboard', drawerAction)} fetcher={fetcher} />)
    fireEvent.click(screen.getByRole('link', { name: 'Open Dashboard metric' }))

    const dialog = await screen.findByRole('dialog')
    const close = screen.getByRole('button', { name: 'Close details' })
    expect(dialog).toContainElement(globalThis.document.activeElement as HTMLElement)
    expect(screen.queryByRole('link', { name: 'Open Drawer document metric' })).not.toBeInTheDocument()
    close.focus()
    fireEvent.keyDown(dialog, { key: 'Tab', shiftKey: true })
    expect(dialog).toContainElement(globalThis.document.activeElement as HTMLElement)
    fireEvent.keyDown(dialog, { key: 'Escape' })
    expect(historyGo).toHaveBeenCalledWith(-1)
  })

  it('closes on a mousedown directly on the backdrop but not inside the dialog', () => {
    const onClose = vi.fn()
    render(
      <LensDrawer closeLabel="Close details" eyebrow="Drill" label="Drill details" onClose={onClose}>
        <p>Body content</p>
      </LensDrawer>,
    )
    const dialog = screen.getByRole('dialog', { name: 'Drill details' })
    const backdrop = dialog.parentElement as HTMLElement

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
