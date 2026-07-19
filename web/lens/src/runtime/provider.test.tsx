import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import fixture from '../../fixtures/small.json'
import { parseDocument } from '../contract'
import { PlaceholderPanel } from '../PlaceholderPanel'
import { DashboardRuntimeProvider, DocumentProvider, useDashboard, useDrill, usePanelFrame } from './provider'

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
  const drill = useDrill()
  const frame = usePanelFrame('total')
  return (
    <>
      <output data-testid="path">{dashboard.navigation.path.join('/')}</output>
      <button type="button" onClick={() => drill.drillInto('root', 'total')}>Root</button>
      <button type="button" onClick={() => drill.drillInto('detail')}>Detail</button>
      <button type="button" onClick={frame.retry}>Refresh frame</button>
    </>
  )
}

function RuntimeFixture({ fetcher }: { fetcher: typeof fetch }) {
  return (
    <div className="lens-root">
      <DocumentProvider initialDocument={document} fetcher={fetcher}>
        <DashboardRuntimeProvider locale="en" fetcher={fetcher}>
          <Controls />
          <PlaceholderPanel />
        </DashboardRuntimeProvider>
      </DocumentProvider>
    </div>
  )
}

afterEach(() => {
  cleanup()
  window.history.replaceState(null, '', '/')
})

describe('DashboardRuntimeProvider', () => {
  it('keeps cached data dimmed through refresh, then exposes error and retry', async () => {
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
    render(<RuntimeFixture fetcher={fetcher} />)

    fireEvent.click(screen.getByRole('button', { name: 'Root' }))
    expect(screen.getByLabelText('Total')).toHaveAttribute('data-stale', 'true')
    expect(screen.getByText('42')).toBeInTheDocument()
    expect(await screen.findByText('43')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Refresh frame' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('refresh failed')
    expect(screen.getByText('43')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(await screen.findByText('44')).toBeInTheDocument()
    expect(fetcher).toHaveBeenCalledTimes(3)
  })

  it('restores deep links and lets popstate replace the reducer view', async () => {
    window.history.replaceState(null, '', '/?path=root&path=detail')
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(response(43))
    render(<RuntimeFixture fetcher={fetcher} />)

    expect(screen.getByTestId('path')).toHaveTextContent('root/detail')
    window.history.replaceState(null, '', '/?path=root')
    window.dispatchEvent(new PopStateEvent('popstate'))
    await waitFor(() => expect(screen.getByTestId('path')).toHaveTextContent('root'))
  })
})
