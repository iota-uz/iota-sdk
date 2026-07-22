import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { useState } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import fixture from '../../fixtures/small.json'
import { parseDocument } from '../contract'
import { StatPanel } from '../panels'
import { DashboardRuntimeProvider, DocumentProvider, useDashboard, useDrawer, useDrill, usePanelFrame } from './provider'
import { QueryClient } from './query'

const document = parseDocument({
  ...fixture,
  panels: [{ ...fixture.panels[0], drillRoot: 'root' }],
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
const dynamicDocument = parseDocument({
  ...fixture,
  panels: [{ ...fixture.panels[0], drillRoot: 'root' }],
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

function Controls() {
  const dashboard = useDashboard()
  const drawer = useDrawer()
  const drill = useDrill()
  const frame = usePanelFrame('total')
  return (
    <>
      <output data-testid="path">{dashboard.navigation.path.join('/')}</output>
      <output data-testid="can-go-back">{String(drill.canGoBack)}</output>
      <button type="button" onClick={() => drill.drillInto('root', 'total')}>Root</button>
      <button type="button" onClick={() => drill.drillInto('detail')}>Detail</button>
      <button type="button" onClick={() => drill.switchPerspective('missing')}>Missing perspective</button>
      <button type="button" onClick={frame.retry}>Refresh frame</button>
      <button type="button" onClick={(event) => drawer.open('/lens/drawer', event.currentTarget)}>Open drawer</button>
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

afterEach(() => {
  cleanup()
  window.history.replaceState(null, '', '/')
})

describe('DashboardRuntimeProvider', () => {
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
    expect(await screen.findByRole('alert')).toHaveTextContent('refresh failed')
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
})
