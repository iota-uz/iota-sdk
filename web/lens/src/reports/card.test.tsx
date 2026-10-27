import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library'
import { afterEach, describe, expect, it, vi } from 'vitest'
import fixture from '../../fixtures/report-card.json'
import { LensDashboard } from '../LensDashboard'
import { parseDocument, type DashboardDocument } from '../contract'
import type { PanelFrameState } from '../runtime'
import type { HostPanelRendererProps } from '../panels/hostRenderers'
import { reportPlacementMenu, reportPlacementUnavailable, type ReportPlacementHost } from './placementHost'
import { formatCalculatedAt, placementFreshness } from './ReportPlacementStatus'

afterEach(() => {
  cleanup()
  document.body.replaceChildren()
})

const embedded = () => parseDocument(structuredClone(fixture.embedded))
const standalone = () => parseDocument(structuredClone(fixture.standalone))
const allActions = () => ({ open: vi.fn(), edit: vi.fn(), configure: vi.fn(), copy: vi.fn(), remove: vi.fn() })
const placement = (id: string) => document.querySelector<HTMLElement>(`[data-report-placement="${id}"]`)!

function mount(doc: DashboardDocument, host?: ReportPlacementHost, extra: Partial<Parameters<typeof LensDashboard>[0]> = {}) {
  return render(() => <LensDashboard initialDocument={doc} reportPlacementHost={host} fetcher={() => new Promise<Response>(() => undefined)} {...extra} />)
}

// False green: checking that items render at all would pass while an
// unsupported handler or capability still offered a hidden save.
describe('reportPlacementMenu', () => {
  const capabilities = { drill: true, save: true, export: true }
  it('offers only handled actions the source and state allow', () => {
    const actions = allActions()
    expect(reportPlacementMenu(undefined, 'p')).toEqual([])
    expect(reportPlacementMenu({ actions }, 'p')).toEqual(['open', 'configure', 'remove'])
    expect(reportPlacementMenu({ actions, capabilities: () => capabilities }, 'p')).toEqual(['open', 'edit', 'configure', 'copy', 'remove'])
    expect(reportPlacementMenu({ actions: { open: actions.open }, capabilities: () => capabilities }, 'p')).toEqual(['open'])
    expect(reportPlacementMenu({ actions, capabilities: () => ({ ...capabilities, save: false }) }, 'p')).toEqual(['open', 'configure', 'remove'])
    expect(reportPlacementMenu({ actions, capabilities: () => capabilities }, 'p', { context: { conflicts: ['product'], unsupported: [] } }))
      .toEqual(['open', 'configure', 'remove'])
    expect(reportPlacementMenu({ actions, capabilities: () => capabilities }, 'p', { unavailable: 'report-deleted' })).toEqual(['remove'])
    expect(reportPlacementMenu({ actions, capabilities: () => capabilities }, 'p', { unavailable: 'report-forbidden' })).toEqual(['remove'])
    expect(reportPlacementMenu({ actions, capabilities: () => capabilities }, 'p', { unavailable: 'report-incompatible' })).toEqual(['open', 'edit', 'remove'])
  })

  // False green: reading only the wire would miss a report the host learns was deleted after opening.
  it('prefers the wire reason, then the host mark, then an incompatible version', () => {
    expect(reportPlacementUnavailable(undefined, { id: 'p' })).toBeUndefined()
    expect(reportPlacementUnavailable({ unavailable: () => 'report-deleted' }, { id: 'p', unavailable: 'report-forbidden' })).toBe('report-forbidden')
    expect(reportPlacementUnavailable({ unavailable: () => 'report-deleted', status: () => 'incompatible' }, { id: 'p' })).toBe('report-deleted')
    expect(reportPlacementUnavailable({ status: () => 'incompatible' }, { id: 'p' })).toBe('report-incompatible')
  })
})

// False green: reporting the newest member time, or ignoring a failed member, would call a broken card fresh.
describe('placementFreshness', () => {
  const frame = (patch: Partial<PanelFrameState>): PanelFrameState => ({ isStale: false, isLoading: false, error: null, retry: () => undefined, ...patch })
  it('lets a failed member outrank the board and reports the oldest calculation', () => {
    const at = (iso: string) => ({ durationMs: 1, cacheHit: false, calculatedAt: iso })
    const fresh = [frame({ calculation: at('2026-10-12T06:00:00Z') }), frame({ calculation: at('2026-10-12T05:00:00Z') })]
    expect(placementFreshness(fresh, '2026-10-12T07:00:00Z', false)).toEqual({ state: 'calculated', at: Date.parse('2026-10-12T05:00:00Z') })
    expect(placementFreshness([frame({})], '2026-10-12T07:00:00Z', false)).toEqual({ state: 'calculated', at: Date.parse('2026-10-12T07:00:00Z') })
    expect(placementFreshness([...fresh, frame({ isLoading: true })], '', false)).toEqual({ state: 'updating' })
    expect(placementFreshness(fresh, '', true)).toEqual({ state: 'updating' })
    const failing = frame({ error: new Error('x') })
    expect(placementFreshness([...fresh, failing], '', true)).toEqual({ state: 'failed', failed: [failing] })
    expect(formatCalculatedAt(Date.parse('2026-10-12T06:30:00Z'), 'en-US', 'Asia/Tashkent')).toMatch(/11:30.*GMT\+5/)
    expect(formatCalculatedAt(0, 'en-US', 'Not/AZone')).toBeTruthy()
  })
})

describe('report card states', () => {
  // False green: a dropped placement would also show no hidden data; the slot and its reason must remain.
  it('keeps deleted and forbidden placements in place without revealing the hidden report', () => {
    mount(embedded(), { actions: allActions() })
    const deleted = placement('deleted')
    expect(deleted).toHaveAttribute('data-state', 'deleted')
    expect(within(deleted).getByRole('heading', { name: 'Archived forecast' })).toBeInTheDocument()
    expect(within(deleted).getByRole('alert')).toHaveTextContent('This report was deleted.')
    const forbidden = placement('forbidden')
    expect(forbidden).toHaveAttribute('data-state', 'forbidden')
    expect(within(forbidden).getByRole('heading', { name: 'Unavailable report' })).toBeInTheDocument()
    expect(within(forbidden).getByRole('alert')).toHaveTextContent('You do not have access to this report.')
    expect(within(forbidden).queryByRole('table')).toBeNull()
    expect(within(forbidden).queryByTestId('report-status-forbidden')).toBeNull()
    expect(placement('revenue')).toHaveAttribute('data-state', 'ready')
  })

  // False green: an unmarked board would also render; the host's late mark must withhold figures.
  it('withholds a placement the host marks deleted after opening', () => {
    mount(embedded(), { unavailable: (id) => id === 'revenue' ? 'report-deleted' : undefined })
    expect(placement('revenue')).toHaveAttribute('data-state', 'deleted')
    expect(within(placement('revenue')).queryByText('Online revenue')).toBeNull()
  })

  // False green: a timestamp without a zone reads differently for readers in other zones.
  it('states the calculation time in the card calendar timezone', () => {
    mount(embedded(), { actions: allActions() })
    expect(screen.getByTestId('report-status-revenue')).toHaveTextContent(/Calculated Oct 12, 2026, 11:30 AM GMT\+5/)
  })
})

describe('flat placements', () => {
  // False green: hiding the card with CSS would leave the menu and status in the DOM for assistive tech.
  it('renders a placement as its panels alone, and keeps the card chrome by default', () => {
    mount(embedded(), { actions: allActions() }, { chrome: { placements: 'flat' } })
    const flat = placement('revenue')
    expect(flat).toHaveClass('lens-panel-group-flat')
    expect(flat.querySelector(':scope > .lens-panel-group-header')).toBeNull()
    expect(flat.querySelector('[data-testid^="report-menu-"], [data-testid^="report-status-"]')).toBeNull()
    expect(flat.querySelector('.lens-panel')).not.toBeNull()
    cleanup()
    mount(embedded(), { actions: allActions() })
    expect(placement('revenue')).not.toHaveClass('lens-panel-group-flat')
    expect(placement('revenue').querySelector(':scope > .lens-panel-group-header')).not.toBeNull()
  })

  // False green: a flat board that swallows the refusal would show a blank gap for a deleted report.
  it('keeps the card for a placement that cannot render', () => {
    mount(embedded(), { unavailable: (id) => id === 'revenue' ? 'report-deleted' : undefined }, { chrome: { placements: 'flat' } })
    expect(placement('revenue')).not.toHaveClass('lens-panel-group-flat')
    expect(placement('revenue')).toHaveAttribute('data-state', 'deleted')
  })
})

describe('flat placements on a single report', () => {
  // False green: a flat standalone report would lose the only menu that opens, copies and edits it.
  it('keeps the card of a report opened on its own', () => {
    mount(standalone(), { actions: allActions() }, { chrome: { placements: 'flat' } })
    const card = document.querySelector<HTMLElement>('[data-report-placement]')!
    expect(card).not.toHaveClass('lens-panel-group-flat')
  })
})

describe('report card menu', () => {
  // False green: a menu opened by pointer alone, or one leaving focus on body, would pass a click-only check.
  it('is one keyboard menu per card that calls only the chosen host handler', async () => {
    const actions = allActions()
    mount(embedded(), { actions, capabilities: () => ({ drill: true, save: true, export: true }) })
    const revenue = placement('revenue')
    expect(within(revenue).getAllByRole('button', { name: /^Card actions/ })).toHaveLength(1)
    const trigger = within(revenue).getByRole('button', { name: 'Card actions: Revenue' })
    fireEvent.click(trigger)
    const menu = await screen.findByRole('menu', { name: 'Card actions: Revenue' })
    const items = within(menu).getAllByRole('menuitem').map((item) => item.textContent)
    expect(items).toEqual(['Open report', 'Edit report', 'Configure card', 'Make an independent copy', 'Remove from dashboard'])
    await waitFor(() => expect(document.activeElement).toBe(within(menu).getAllByRole('menuitem')[0]))
    fireEvent.keyDown(menu, { key: 'ArrowDown' })
    expect(document.activeElement).toHaveTextContent('Edit report')
    fireEvent.keyDown(document, { key: 'Escape' })
    await waitFor(() => expect(screen.queryByRole('menu')).toBeNull())
    expect(document.activeElement).toBe(trigger)
    expect(Object.values(actions).every((handler) => handler.mock.calls.length === 0)).toBe(true)

    fireEvent.click(trigger)
    fireEvent.click(await screen.findByRole('menuitem', { name: 'Configure card' }))
    expect(actions.configure).toHaveBeenCalledWith('revenue')
    expect(actions.edit).not.toHaveBeenCalled()
    expect(within(placement('deleted')).getByRole('button', { name: 'Card actions: Archived forecast' })).toBeInTheDocument()
  })

  // False green: focus on the removed trigger is lost to body, which a screen reader announces as nothing.
  it('moves focus to the next card when the host removes this one', async () => {
    const remove = vi.fn((id: string) => { placement(id).closest('.lens-grid-item')!.remove() })
    mount(embedded(), { actions: { remove } })
    fireEvent.click(within(placement('deleted')).getByRole('button', { name: /^Card actions/ }))
    fireEvent.click(await screen.findByRole('menuitem', { name: 'Remove from dashboard' }))
    expect(remove).toHaveBeenCalledWith('deleted')
    await waitFor(() => expect(document.activeElement).toBe(placement('forbidden')))
  })

  // False green: a rejected handler that silently closed the menu would look like success.
  it('reports a failed host action next to the card', async () => {
    mount(embedded(), { actions: { open: () => Promise.reject(new Error('denied')) } })
    vi.spyOn(console, 'error').mockImplementation(() => undefined)
    fireEvent.click(within(placement('revenue')).getByRole('button', { name: /^Card actions/ }))
    fireEvent.click(await screen.findByRole('menuitem', { name: 'Open report' }))
    expect(await within(placement('revenue')).findByRole('alert')).toHaveTextContent('The action could not be completed.')
  })

  // False green: hiding the card menu's export while each member still offered its own would leak the operation.
  it('hides member export when the source cannot export', () => {
    mount(embedded(), { capabilities: (id) => ({ drill: false, save: false, export: id !== 'revenue' }) })
    expect(within(placement('revenue')).queryAllByRole('button', { name: 'Export panel' })).toHaveLength(0)
    expect(within(placement('legacy')).getAllByRole('button', { name: 'Export panel' }).length).toBeGreaterThan(0)
  })

  // False green: the revenue members are still loading, and unlabelled rows render without a nested card; a labelled inner group provides its own chrome and must still inherit the refusal.
  it('keeps member export hidden inside nested labelled groups', () => {
    const labelled = () => {
      const doc = embedded()
      for (const item of doc.layout.rows.flatMap((row) => row.panels)) {
        const inner = item.groups?.find((group) => group.id === 'legacy::row-0')
        if (inner) inner.label = 'Breakdown'
      }
      return doc
    }
    mount(labelled(), { capabilities: () => ({ drill: false, save: false, export: true }) })
    expect(within(placement('legacy')).getByRole('region', { name: 'Breakdown' })).toBeInTheDocument()
    expect(within(placement('legacy')).getAllByRole('button', { name: 'Export panel' }).length).toBeGreaterThan(0)
    cleanup()

    mount(labelled(), { capabilities: (id) => ({ drill: false, save: false, export: id !== 'legacy' }) })
    expect(within(placement('legacy')).getByRole('region', { name: 'Breakdown' })).toBeInTheDocument()
    expect(within(placement('legacy')).queryAllByRole('button', { name: 'Export panel' })).toHaveLength(0)
  })
})

describe('host renderers', () => {
  // False green: rendering the table kind for an unregistered renderer would substitute a lookalike view.
  it('renders a registered host view inside the SDK frame and a compatibility state otherwise', () => {
    const LegacyBars = (props: HostPanelRendererProps) => (
      <ol data-testid="legacy-bars">{props.frame.data?.rows.map((row) => <li>{`${String(row[0])}=${String(row[1])}`}</li>)}{JSON.stringify(props.options)}</ol>
    )
    mount(embedded(), { actions: allActions() }, { renderers: { 'legacy-bars': LegacyBars } })
    const legacy = placement('legacy')
    expect(within(legacy).getByTestId('legacy-bars')).toHaveTextContent('Aug=3Sep=5{"field":"value","label":"month"}')
    expect(within(legacy).getByRole('button', { name: 'Export panel' })).toBeInTheDocument()
    expect(within(legacy).getAllByRole('heading', { name: 'Legacy chart' })).toHaveLength(1)
    cleanup()

    mount(embedded(), { actions: allActions() })
    const fallback = placement('legacy')
    expect(within(fallback).getByRole('status')).toHaveTextContent('This view cannot be shown here.')
    expect(within(fallback).queryByRole('table')).toBeNull()
    expect(within(fallback).getByRole('button', { name: 'Export panel' })).toBeInTheDocument()
  })
})

describe('card retry', () => {
  // False green: a retry that refetched the whole board would also clear the error.
  it('re-requests only the failed members of this card', async () => {
    const doc = embedded()
    doc.endpoints.panel = '/lens/panel'
    for (const panel of doc.panels) {
      if (panel.id === 'revenue::panel:table' || panel.id === 'revenue::panel:channels') {
        panel.deferred = true
        delete doc.frames[panel.frame]
      }
    }
    const tableFrame = fixture.embedded.frames[doc.panels.find((panel) => panel.id === 'revenue::panel:table')!.frame as keyof typeof fixture.embedded.frames]
    const chartFrame = fixture.embedded.frames[doc.panels.find((panel) => panel.id === 'revenue::panel:channels')!.frame as keyof typeof fixture.embedded.frames]
    const bodies: Array<{ panels: Array<{ panelId: string }> }> = []
    let failTable = true
    const fetcher = vi.fn<typeof fetch>((_input, init) => {
      const body = JSON.parse(init?.body as string) as { panels: Array<{ panelId: string }> }
      bodies.push(body)
      const calculation = { durationMs: 5, cacheHit: false, calculatedAt: '2026-10-12T06:00:00Z' }
      const events: unknown[] = body.panels.map(({ panelId }) => ({
        panelId,
        result: panelId === 'revenue::panel:table' && failTable
          ? { error: { error: 'internal', message: 'boom' } }
          : { frames: { primary: panelId.endsWith('table') ? tableFrame : chartFrame }, calculation },
      }))
      events.push({ complete: true })
      return Promise.resolve(new Response(`${events.map((event) => JSON.stringify(event)).join('\n')}\n`, { status: 200, headers: { 'Content-Type': 'application/x-ndjson' } }))
    })
    vi.spyOn(console, 'error').mockImplementation(() => undefined)
    render(() => <LensDashboard initialDocument={doc} fetcher={fetcher} reportPlacementHost={{ actions: allActions() }} />)
    await waitFor(() => expect(placement('revenue')).toHaveAttribute('data-state', 'error'))
    expect(screen.getByTestId('report-status-revenue')).toHaveTextContent('Not updated')
    expect(bodies.flatMap((body) => body.panels.map(({ panelId }) => panelId)).sort()).toEqual(['revenue::panel:channels', 'revenue::panel:table'])

    failTable = false
    fireEvent.click(screen.getByTestId('report-retry-revenue'))
    await waitFor(() => expect(placement('revenue')).toHaveAttribute('data-state', 'ready'))
    expect(bodies.at(-1)!.panels.map(({ panelId }) => panelId)).toEqual(['revenue::panel:table'])
    expect(screen.getByTestId('report-status-revenue')).toHaveTextContent('Calculated Oct 12, 2026, 11:00 AM GMT+5')
  })
})

describe('standalone and embedded parity', () => {
  // False green: two hand-written fixtures could agree by construction; both documents come from one Go Compose run.
  it('renders the same card for one report opened alone and placed on a dashboard', async () => {
    const host = (): ReportPlacementHost => ({ actions: allActions(), capabilities: () => ({ drill: true, save: true, export: true }) })
    const read = async (doc: DashboardDocument) => {
      mount(doc, host())
      const card = placement('revenue')
      await waitFor(() => expect(within(card).queryAllByRole('table').length).toBeGreaterThan(0))
      const snapshot = {
        text: card.textContent,
        kinds: [...card.querySelectorAll('[data-panel-kind]')].map((node) => node.getAttribute('data-panel-kind')),
        exports: within(card).queryAllByRole('button', { name: 'Export panel' }).length,
        menu: within(card).getAllByRole('button', { name: /^Card actions/ }).length,
        state: card.getAttribute('data-state'),
      }
      cleanup()
      document.body.replaceChildren()
      return snapshot
    }
    const alone = await read(standalone())
    const placed = await read(embedded())
    expect(alone.text).toContain('7,200')
    expect(alone.text).toContain('Refunds0')
    expect(alone.text).not.toContain('No data')
    expect(placed).toEqual(alone)
  })
})
