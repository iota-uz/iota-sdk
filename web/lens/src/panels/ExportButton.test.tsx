import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import fixture from '../../fixtures/small.json'
import { parseDocument } from '../contract'
import { DashboardRuntimeProvider, DocumentProvider } from '../runtime'
import { ExportButton } from './ExportButton'

const firstDocument = parseDocument({
  ...fixture,
  snapshotId: 'expired-snapshot',
  endpoints: { ...fixture.endpoints, export: '/lens/export?format=xlsx' },
})
const refreshedDocument = parseDocument({ ...firstDocument, snapshotId: 'fresh-snapshot' })

afterEach(() => {
  cleanup()
  Reflect.deleteProperty(URL, 'createObjectURL')
  Reflect.deleteProperty(URL, 'revokeObjectURL')
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe('ExportButton snapshot recovery', () => {
  it('refreshes after 410, offers retry, and uses the server filename', async () => {
    let documentRequests = 0
    const exportSnapshots: string[] = []
    const fetcher = vi.fn<typeof fetch>().mockImplementation((input) => {
      const target = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
      const url = new URL(target, 'https://example.test')
      if (url.pathname === '/lens/document') {
        documentRequests += 1
        const payload = documentRequests === 1 ? firstDocument : refreshedDocument
        return Promise.resolve(new Response(JSON.stringify(payload), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }))
      }
      exportSnapshots.push(url.searchParams.get('snapshot') ?? '')
      if (exportSnapshots.length === 1) {
        return Promise.resolve(new Response(JSON.stringify({
          error: 'snapshot_gone', message: 'snapshot is unknown or expired',
        }), { status: 410, headers: { 'Content-Type': 'application/json' } }))
      }
      return Promise.resolve(new Response(new Blob(['workbook']), {
        status: 200,
        headers: { 'Content-Disposition': `attachment; filename="report.xlsx"; filename*=UTF-8''server-report.xlsx` },
      }))
    })
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined)
    const createObjectURL = vi.fn(() => 'blob:workbook')
    const revokeObjectURL = vi.fn()
    Object.defineProperty(URL, 'createObjectURL', { configurable: true, value: createObjectURL })
    Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, value: revokeObjectURL })

    render(
      <DocumentProvider src="/lens/document" fetcher={fetcher}>
        <DashboardRuntimeProvider locale="en" fetcher={fetcher}>
          <ExportButton panelId="total" />
        </DashboardRuntimeProvider>
      </DocumentProvider>,
    )

    fireEvent.click(await screen.findByRole('button', { name: 'Export panel' }))
    expect(await screen.findByRole('button', { name: 'Retry export' })).toBeInTheDocument()
    expect(screen.getByText('Snapshot refreshed. Retry export.')).toBeInTheDocument()
    expect(documentRequests).toBe(2)
    expect(exportSnapshots).toEqual(['expired-snapshot'])

    fireEvent.click(screen.getByRole('button', { name: 'Retry export' }))
    await waitFor(() => expect(click).toHaveBeenCalledOnce())
    expect(exportSnapshots).toEqual(['expired-snapshot', 'fresh-snapshot'])
    expect(createObjectURL).toHaveBeenCalledOnce()
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:workbook')
    expect(click.mock.instances[0]).toHaveProperty('download', 'server-report.xlsx')
  })
})
