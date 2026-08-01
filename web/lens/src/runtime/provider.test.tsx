import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { useState } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import fixture from '../../fixtures/small.json'
import { parseDocument } from '../contract'
import { StatPanel } from '../panels'
import { DashboardRuntimeProvider, DocumentProvider, useDashboard, useDrawer, useDrill, usePanelFrame } from './provider'
import { QueryClient } from './query'

const document = parseDocument({
  ...fixture,
  panels: [{ ...fixture.panels[0], drillRoot: 'root', terminal: false }],
  drill: {
    inlineDepth: 0,
    edges: {
      root: {
        path: ['root'], label: 'Root', perspectives: [],
        children: [{ key: 'detail', path: ['root', 'detail'], label: 'Detail', target: 'detail' }],
      },
      detail: { path: ['root', 'detail'], label: 'Detail', children: [], perspectives: [] },
    },
  },
})
const statPanel = document.panels[0]!
const progressiveDocument = parseDocument({
  ...fixture,
  snapshotId: 'progressive-snapshot',
  panels: [
    { ...fixture.panels[0], id: 'ready', title: 'Ready', frame: 'panel:ready', deferred: true },
    { ...fixture.panels[0], id: 'retrying', title: 'Retrying', frame: 'panel:retrying', deferred: true },
  ],
  frames: {},
  layout: { rows: [{ panels: [{ panelId: 'ready', span: 6 }, { panelId: 'retrying', span: 6 }] }] },
  endpoints: { ...fixture.endpoints, panel: '/lens/panel' },
})
const recurringProgressiveDocument = parseDocument({
  ...progressiveDocument,
  snapshotId: 'recurring-snapshot',
  panels: [progressiveDocument.panels[0]],
  layout: { rows: [{ panels: [{ panelId: 'ready', span: 12 }] }] },
})
const filteredProgressiveDocument = parseDocument({
  ...recurringProgressiveDocument,
  snapshotId: 'filtered-snapshot',
})
const dynamicDocument = parseDocument({
  ...fixture,
  panels: [{ ...fixture.panels[0], drillRoot: 'root', terminal: false }],
  drill: {
    inlineDepth: 0,
    edges: {
      root: {
        path: ['root'], label: 'Root', children: [], perspectives: [],
        dynamicChildren: {
          key: { kind: 'field', name: 'id' }, label: { kind: 'field', name: 'label' },
          target: { kind: 'field', name: 'target' },
        },
      },
      detail: { path: ['root', 'detail'], label: 'Detail', children: [], perspectives: [] },
    },
  },
})

function response(value: number): Response {
  return new Response(JSON.stringify({
    frames: {
      detail: {
        columns: document.frames['panel:total']?.columns ?? [],
        rows: [['Total', value]],
      },
    },
  }), { status: 200, headers: { 'Content-Type': 'application/json' } })
}

function panelStream(panels: Record<string, unknown>): Response {
  const events = [
    ...Object.entries(panels).map(([panelId, result]) => ({ panelId, result })),
    { complete: true },
  ]
  return new Response(`${events.map((event) => JSON.stringify(event)).join('\n')}\n`, {
    status: 200, headers: { 'Content-Type': 'application/x-ndjson' },
  })
}

function Controls() {
  const dashboard = useDashboard()
  const drawer = useDrawer()
  const drill = useDrill()
  const frame = usePanelFrame('total')
  return (
    <>
      <output data-testid="path">{dashboard.navigation.path.join('/')}</output>
      <output data-testid="panel">{dashboard.navigation.panelId ?? ''}</output>
      <output data-testid="can-go-back">{String(drill.canGoBack)}</output>
      <button type="button" onClick={() => drill.drillInto('root', 'total')}>Root</button>
      <button type="button" onClick={() => drill.drillInto('detail')}>Detail</button>
      <button type="button" onClick={() => drill.switchPerspective('missing')}>Missing perspective</button>
      <button type="button" onClick={frame.retry}>Refresh frame</button>
      <button type="button" onClick={(event) => drawer.open('/lens/drawer', event.currentTarget)}>Open drawer</button>
      <button
        type="button"
        onClick={(event) => drawer.open('/lens/drawer?path=root&path=detail', event.currentTarget)}
      >
        Open drawer at detail
      </button>
    </>
  )
}

function RuntimeFixture({ fetcher }: { fetcher: typeof fetch }) {
  return (
    <div className="lens-root">
      <DocumentProvider initialDocument={document} fetcher={fetcher}>
        <DashboardRuntimeProvider locale="en" fetcher={fetcher}>
          <Controls />
          <StatPanel panel={statPanel} />
        </DashboardRuntimeProvider>
      </DocumentProvider>
    </div>
  )
}

function FrameProbe({ panelId, onRender }: { panelId: string; onRender: () => void }) {
  usePanelFrame(panelId)
  onRender()
  return null
}

function RecomputeProbe() {
  const { recompute, isRecomputing } = useDashboard()
  return <button type="button" disabled={isRecomputing} onClick={recompute}>Recompute panels</button>
}

afterEach(() => {
  cleanup()
  window.history.replaceState(null, '', '/')
})

describe('DashboardRuntimeProvider', () => {
  it('rehydrates a previously seen snapshot after browser Back restores it', async () => {
    const attempts: Record<string, number> = {}
    const fetcher = vi.fn<typeof fetch>().mockImplementation((_input, init) => {
      const request = JSON.parse(typeof init?.body === 'string' ? init.body : '{}') as { snapshotId: string; panels: Array<{ panelId: string }> }
      attempts[request.snapshotId] = (attempts[request.snapshotId] ?? 0) + 1
      const value = request.snapshotId === 'recurring-snapshot' ? attempts[request.snapshotId] : 20
      return Promise.resolve(panelStream({ ready: {
        frames: {
          'panel:ready': {
            columns: fixture.frames['panel:total'].columns,
            rows: [['Ready', value]],
          },
        },
        calculation: { durationMs: 1, cacheHit: false, calculatedAt: '2026-07-31T10:00:00Z' },
      } }))
    })

    function SnapshotSwitcher() {
      const [current, setCurrent] = useState(recurringProgressiveDocument)
      return (
        <div className="lens-root">
          <button type="button" onClick={() => setCurrent(filteredProgressiveDocument)}>Filter</button>
          <button type="button" onClick={() => setCurrent(recurringProgressiveDocument)}>Back</button>
          <DocumentProvider initialDocument={current} fetcher={fetcher}>
            <DashboardRuntimeProvider locale="en" fetcher={fetcher}>
              <StatPanel panel={current.panels[0]!} />
            </DashboardRuntimeProvider>
          </DocumentProvider>
        </div>
      )
    }

    render(<SnapshotSwitcher />)
    expect(await screen.findByText('1')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Filter' }))
    expect(await screen.findByText('20')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Back' }))
    expect(await screen.findByText('2')).toBeInTheDocument()
    expect(attempts).toEqual({ 'recurring-snapshot': 2, 'filtered-snapshot': 1 })
  })

  it('loads, fails, and retries deferred siblings independently', async () => {
    const attempts: Record<string, number> = {}
    const recomputed: string[] = []
    const batchBodies: Array<{ snapshotId?: string; panels: Array<{ panelId: string; recompute?: boolean }> }> = []
    const fetcher = vi.fn<typeof fetch>().mockImplementation((_input, init) => {
      const request = JSON.parse(typeof init?.body === 'string' ? init.body : '{}') as { panels: Array<{ panelId: string; recompute?: boolean }> }
      batchBodies.push(request)
      const panels: Record<string, unknown> = {}
      for (const item of request.panels) {
        if (item.recompute) recomputed.push(item.panelId)
        attempts[item.panelId] = (attempts[item.panelId] ?? 0) + 1
        if (item.panelId === 'retrying' && attempts[item.panelId] === 1) {
          panels[item.panelId] = { error: { error: 'internal', message: 'retrying failed' } }
          continue
        }
        const value = item.panelId === 'ready' ? 11 : 22
        panels[item.panelId] = { frames: {
          [`panel:${item.panelId}`]: {
            columns: fixture.frames['panel:total'].columns,
            rows: [[item.panelId, value]],
          },
        },
        calculation: { durationMs: 1250, cacheHit: item.panelId === 'ready', calculatedAt: '2026-07-31T10:00:00Z' } }
      }
      return Promise.resolve(panelStream(panels))
    })
    render(
      <div className="lens-root">
        <DocumentProvider initialDocument={progressiveDocument} fetcher={fetcher}>
          <DashboardRuntimeProvider locale="en" fetcher={fetcher}>
            <RecomputeProbe />
            <StatPanel panel={progressiveDocument.panels[0]!} />
            <StatPanel panel={progressiveDocument.panels[1]!} />
          </DashboardRuntimeProvider>
        </DocumentProvider>
      </div>,
    )

    const ready = screen.getByLabelText('Ready')
    const retrying = screen.getByLabelText('Retrying')
    expect(await within(ready).findByText('11')).toBeInTheDocument()
    expect(batchBodies).toHaveLength(1)
    expect(batchBodies[0]?.panels.map(({ panelId }) => panelId)).toEqual(['ready', 'retrying'])
    expect(await within(retrying).findByRole('alert')).toHaveTextContent('This panel could not be rendered.')
    expect(within(retrying).getByRole('alert')).not.toHaveTextContent('retrying failed')
    expect(within(ready).getByText('11')).toBeInTheDocument()
    fireEvent.click(within(ready).getByRole('button', { name: 'About this metric' }))
    expect(await screen.findByText('Calculated in 1.3 s · cache hit')).toBeInTheDocument()

    fireEvent.click(within(retrying).getByRole('button', { name: 'Retry' }))
    expect(within(retrying).queryByRole('alert')).toBeNull()
    expect(within(ready).getByText('11')).toBeInTheDocument()
    expect(await within(retrying).findByText('22')).toBeInTheDocument()
    expect(batchBodies).toHaveLength(2)
    expect(batchBodies[1]?.panels).toEqual([{ panelId: 'retrying' }])
    expect(attempts).toEqual({ ready: 1, retrying: 2 })

    fireEvent.click(screen.getByRole('button', { name: 'Recompute panels' }))
    await waitFor(() => expect(attempts).toEqual({ ready: 2, retrying: 3 }))
    expect(recomputed.sort()).toEqual(['ready', 'retrying'])
    expect(batchBodies).toHaveLength(3)
    expect(batchBodies[2]?.panels).toEqual([
      { panelId: 'ready', recompute: true }, { panelId: 'retrying', recompute: true },
    ])
    expect(screen.getByRole('button', { name: 'Recompute panels' })).not.toBeDisabled()
  })

  it('reveals streamed batch members without waiting for a slow sibling', async () => {
    let controller: ReadableStreamDefaultController<Uint8Array> | undefined
    const stream = new ReadableStream<Uint8Array>({ start(value) { controller = value } })
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(new Response(stream, {
      status: 200,
      headers: { 'Content-Type': 'application/x-ndjson' },
    }))
    render(
      <div className="lens-root">
        <DocumentProvider initialDocument={progressiveDocument} fetcher={fetcher}>
          <DashboardRuntimeProvider locale="en" fetcher={fetcher}>
            <StatPanel panel={progressiveDocument.panels[0]!} />
            <StatPanel panel={progressiveDocument.panels[1]!} />
          </DashboardRuntimeProvider>
        </DocumentProvider>
      </div>,
    )

    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(1))
    const encode = new TextEncoder()
    act(() => {
      controller?.enqueue(encode.encode(`${JSON.stringify({
        panelId: 'ready',
        result: { frames: { 'panel:ready': { columns: fixture.frames['panel:total'].columns, rows: [['Ready', 11]] } } },
      })}\n`))
    })
    expect(await within(screen.getByLabelText('Ready')).findByText('11')).toBeInTheDocument()
    expect(screen.getByLabelText('Retrying')).toHaveAttribute('aria-busy', 'true')

    act(() => {
      controller?.enqueue(encode.encode(`${JSON.stringify({
        panelId: 'retrying', result: { error: { error: 'internal', message: 'failed independently' } },
      })}\n${JSON.stringify({ complete: true })}\n`))
      controller?.close()
    })
    expect(await within(screen.getByLabelText('Retrying')).findByRole('alert')).toBeInTheDocument()
    expect(within(screen.getByLabelText('Ready')).getByText('11')).toBeInTheDocument()
    expect(fetcher).toHaveBeenCalledTimes(1)
  })

  it('replaces cached data with the skeleton through refresh, then exposes error and retry', async () => {
    let request = 0
    const fetcher = vi.fn<typeof fetch>().mockImplementation(() => {
      request += 1
      if (request === 2) {
        return Promise.resolve(new Response(JSON.stringify({ error: 'internal', message: 'refresh failed' }), {
          status: 500,
          headers: { 'Content-Type': 'application/json' },
        }))
      }
      return Promise.resolve(response(request === 1 ? 43 : 44))
    })
    const view = render(<RuntimeFixture fetcher={fetcher} />)

    fireEvent.click(screen.getByRole('button', { name: 'Root' }))
    // A refetch swaps the stale value for the initial-load skeleton rather than
    // dimming it in place, so the panel unmistakably reads as recomputing.
    expect(screen.getByLabelText('Total')).toHaveAttribute('data-stale', 'true')
    expect(screen.queryByText('42')).toBeNull()
    expect(view.container.querySelector('.lens-panel-skeleton')).not.toBeNull()
    expect(await screen.findByText('43')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Refresh frame' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('This panel could not be rendered.')
    expect(screen.getByRole('alert')).not.toHaveTextContent('refresh failed')
    expect(screen.getByText('43')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(await screen.findByText('44')).toBeInTheDocument()
    expect(fetcher).toHaveBeenCalledTimes(3)
  })

  it('notifies only subscribers for the panel whose frame changed', async () => {
    const fetcher = vi.fn<typeof fetch>().mockImplementation(() => Promise.resolve(response(43)))
    const unrelatedRender = vi.fn()
    render(
      <div className="lens-root">
        <DocumentProvider initialDocument={document}>
          <DashboardRuntimeProvider locale="en" fetcher={fetcher}>
            <Controls />
            <StatPanel panel={statPanel} />
            <FrameProbe panelId="unrelated" onRender={unrelatedRender} />
          </DashboardRuntimeProvider>
        </DocumentProvider>
      </div>,
    )
    const rendersBeforeQuery = unrelatedRender.mock.calls.length

    fireEvent.click(screen.getByRole('button', { name: 'Root' }))
    expect(await screen.findByText('43')).toBeInTheDocument()
    expect(unrelatedRender).toHaveBeenCalledTimes(rendersBeforeQuery)
  })

  it('restores deep links and lets popstate replace the reducer view', async () => {
    window.history.replaceState(null, '', '/?path=root&path=detail')
    const fetcher = vi.fn<typeof fetch>().mockImplementation(() => Promise.resolve(response(43)))
    render(<RuntimeFixture fetcher={fetcher} />)

    expect(screen.getByTestId('path')).toHaveTextContent('root/detail')
    expect(screen.getByTestId('can-go-back')).toHaveTextContent('true')
    window.history.replaceState(null, '', '/?path=root')
    window.dispatchEvent(new PopStateEvent('popstate'))
    await waitFor(() => expect(screen.getByTestId('path')).toHaveTextContent('root'))
    expect(screen.getByTestId('can-go-back')).toHaveTextContent('true')
  })

  it('restores a deep link after fetching its dynamic parent step', async () => {
    window.history.replaceState(null, '', '/?panel=total&path=root&path=year-2025')
    const requests: Array<{ path: Array<string> }> = []
    const fetcher = vi.fn<typeof fetch>().mockImplementation((_input, init) => {
      const body = typeof init?.body === 'string' ? init.body : '{}'
      requests.push(JSON.parse(body) as { path: Array<string> })
      const root = requests.length === 1
      return Promise.resolve(new Response(JSON.stringify({
        frames: {
          [root ? 'root' : 'detail']: root ? {
            columns: [
              { name: 'id', type: 'string' }, { name: 'label', type: 'string' }, { name: 'target', type: 'string' },
            ],
            rows: [['year-2025', '2025', 'detail']],
            children: [{ key: 'year-2025', path: ['root', 'year-2025'], label: '2025', target: 'detail' }],
          } : {
            columns: document.frames['panel:total']!.columns,
            rows: [['Total', 2025]],
          },
        },
      }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
    })
    render(
      <div className="lens-root">
        <DocumentProvider initialDocument={dynamicDocument} fetcher={fetcher}>
          <DashboardRuntimeProvider locale="en" fetcher={fetcher}>
            <Controls />
            <StatPanel panel={dynamicDocument.panels[0]!} />
          </DashboardRuntimeProvider>
        </DocumentProvider>
      </div>,
    )

    await waitFor(() => expect(screen.getByTestId('path')).toHaveTextContent('root/year-2025'))
    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2))
    expect(requests.map(({ path }) => path)).toEqual([['root'], ['root', 'year-2025', 'detail']])
    expect(await screen.findByText('2,025')).toBeInTheDocument()
  })

  it('restores the in-app history stack stored in browser history', async () => {
    const fetcher = vi.fn<typeof fetch>().mockImplementation(() => Promise.resolve(response(43)))
    render(<RuntimeFixture fetcher={fetcher} />)

    fireEvent.click(screen.getByRole('button', { name: 'Root' }))
    const rootState: unknown = window.history.state
    fireEvent.click(screen.getByRole('button', { name: 'Detail' }))
    expect(screen.getByTestId('path')).toHaveTextContent('root/detail')

    window.history.replaceState(rootState, '', '/?path=root')
    window.dispatchEvent(new PopStateEvent('popstate', { state: rootState }))

    await waitFor(() => expect(screen.getByTestId('path')).toHaveTextContent('root'))
    expect(screen.getByTestId('can-go-back')).toHaveTextContent('true')
  })

  it('pushes a drill that is batched with popstate', async () => {
    const fetcher = vi.fn<typeof fetch>().mockImplementation(() => Promise.resolve(response(43)))
    render(<RuntimeFixture fetcher={fetcher} />)

    fireEvent.click(screen.getByRole('button', { name: 'Root' }))
    const rootState: unknown = window.history.state
    fireEvent.click(screen.getByRole('button', { name: 'Detail' }))

    act(() => {
      window.history.replaceState(rootState, '', '/?path=root')
      window.dispatchEvent(new PopStateEvent('popstate', { state: rootState }))
      fireEvent.click(screen.getByRole('button', { name: 'Detail' }))
    })

    await waitFor(() => expect(screen.getByTestId('path')).toHaveTextContent('root/detail'))
    expect(new URL(window.location.href).searchParams.getAll('path')).toEqual(['root', 'detail'])
  })

  it('ignores invalid drill transitions without resetting or showing a notice', async () => {
    const fetcher = vi.fn<typeof fetch>().mockImplementation(() => Promise.resolve(response(43)))
    render(<RuntimeFixture fetcher={fetcher} />)

    fireEvent.click(screen.getByRole('button', { name: 'Root' }))
    fireEvent.click(screen.getByRole('button', { name: 'Detail' }))
    await waitFor(() => expect(screen.getByTestId('path')).toHaveTextContent('root/detail'))
    fireEvent.click(screen.getByRole('button', { name: 'Detail' }))
    fireEvent.click(screen.getByRole('button', { name: 'Missing perspective' }))

    expect(screen.getByTestId('path')).toHaveTextContent('root/detail')
    expect(screen.queryByText('The previous drill path is no longer available. Lens returned to the root view.'))
      .not.toBeInTheDocument()
  })

  it('does not refetch when an inline fetcher prop changes identity', async () => {
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(new Response(JSON.stringify(document), { status: 200 }))

    function InlineFetcherFixture() {
      const [, setRender] = useState(0)
      return (
        <DocumentProvider src="/lens/document" fetcher={(input, init) => fetcher(input, init)}>
          <button type="button" onClick={() => setRender((value) => value + 1)}>Rerender</button>
        </DocumentProvider>
      )
    }

    render(<InlineFetcherFixture />)
    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(1))
    fireEvent.click(screen.getByRole('button', { name: 'Rerender' }))
    await Promise.resolve()
    expect(fetcher).toHaveBeenCalledTimes(1)
  })

  it('does not rerun the dashboard query when a drawer opens or closes', async () => {
    const query = vi.spyOn(QueryClient.prototype, 'query')
    const fetcher = vi.fn<typeof fetch>().mockImplementation((input) => {
      if (input === '/lens/drawer') {
        return Promise.resolve(new Response(JSON.stringify(document), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }))
      }
      return Promise.resolve(response(43))
    })
    render(<RuntimeFixture fetcher={fetcher} />)

    fireEvent.click(screen.getByRole('button', { name: 'Root' }))
    expect(await screen.findByText('43')).toBeInTheDocument()
    expect(query).toHaveBeenCalledTimes(1)

    fireEvent.click(screen.getByRole('button', { name: 'Open drawer' }))
    expect(await screen.findByRole('dialog')).toBeInTheDocument()
    expect(query).toHaveBeenCalledTimes(1)

    act(() => window.history.back())
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(query).toHaveBeenCalledTimes(1)
  })

  it('opens a drawer source on its declared initial Lens view', async () => {
    const query = vi.spyOn(QueryClient.prototype, 'query')
    const fetcher = vi.fn<typeof fetch>().mockImplementation((input) => {
      const href = typeof input === 'string'
        ? input
        : input instanceof URL
          ? input.toString()
          : input.url
      if (href.startsWith('/lens/drawer?')) {
        return Promise.resolve(new Response(JSON.stringify(document), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }))
      }
      return Promise.resolve(response(43))
    })
    render(<RuntimeFixture fetcher={fetcher} />)

    fireEvent.click(screen.getByRole('button', { name: 'Open drawer at detail' }))
    const dialog = await screen.findByRole('dialog')
    await waitFor(() => expect(within(dialog).getByTestId('path')).toHaveTextContent('root/detail'))
    expect(within(dialog).getByTestId('panel')).toHaveTextContent('total')
    await waitFor(() => expect(query).toHaveBeenCalledTimes(1))
    expect(within(dialog).getByText('43')).toBeInTheDocument()
  })

  it('preserves a data-theme dark dashboard in its drawer portal', async () => {
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(new Response(JSON.stringify(document), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }))
    render(
      <div className="lens-root" data-theme="dark">
        <DocumentProvider initialDocument={document} fetcher={fetcher}>
          <DashboardRuntimeProvider locale="en" fetcher={fetcher}>
            <Controls />
          </DashboardRuntimeProvider>
        </DocumentProvider>
      </div>,
    )

    fireEvent.click(screen.getByRole('button', { name: 'Open drawer' }))

    const dialog = await screen.findByRole('dialog')
    expect(dialog.closest('.lens-root')).toHaveAttribute('data-theme', 'dark')
    expect(dialog.closest('.lens-root')).toHaveClass('dark')
  })
})
