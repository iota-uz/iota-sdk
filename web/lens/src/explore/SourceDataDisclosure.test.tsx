import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import fixture from '../../fixtures/explore.json'
import { parseDocument } from '../contract'
import { DashboardRuntimeProvider, DocumentProvider, useDrill } from '../runtime'
import { SourceDataDisclosure } from './SourceDataDisclosure'

const document = parseDocument({
  ...fixture,
  endpoints: { ...fixture.endpoints, query: '/lens/query' },
})

function DisclosureHarness() {
  const drill = useDrill()
  return (
    <>
      <button onClick={() => drill.drillInto('profitability', 'margin')} type="button">Change path</button>
      <SourceDataDisclosure source={{ frame: 'source:rows' }} />
    </>
  )
}

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

describe('SourceDataDisclosure', () => {
  it('retries after a navigation change aborts an in-flight request', async () => {
    const fetcher = vi.fn<typeof fetch>()
      .mockImplementationOnce(() => new Promise<Response>(() => undefined))
      .mockResolvedValueOnce(new Response(JSON.stringify({
        frames: {
          'source:rows': {
            columns: [{ name: 'record', type: 'string' }],
            rows: [['Recovered row']],
          },
        },
      }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
    vi.stubGlobal('fetch', fetcher)
    render(
      <DocumentProvider initialDocument={document}>
        <DashboardRuntimeProvider locale="en" drawerDepth={1}>
          <DisclosureHarness />
        </DashboardRuntimeProvider>
      </DocumentProvider>,
    )

    fireEvent.click(screen.getByRole('button', { name: 'Source data' }))
    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(1))
    fireEvent.click(screen.getByRole('button', { name: 'Change path' }))

    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2))
    expect(await screen.findByText('Recovered row')).toBeInTheDocument()
  })
})
