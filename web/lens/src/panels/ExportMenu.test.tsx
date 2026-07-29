import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import fixture from '../../fixtures/small.json'
import { parseDocument } from '../contract'
import { DashboardPanels } from '../DashboardPanels'
import { DashboardRuntimeProvider, DocumentProvider } from '../runtime'

afterEach(() => {
  cleanup()
  document.documentElement.classList.remove('lens-print-active')
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

function renderDashboard(document_: ReturnType<typeof parseDocument>) {
  render(
    <div className="lens-root">
      <DocumentProvider initialDocument={document_}>
        <DashboardRuntimeProvider locale="en">
          <DashboardPanels />
        </DashboardRuntimeProvider>
      </DocumentProvider>
    </div>,
  )
}

describe('ExportMenu', () => {
  it('asks which artefact, then builds the report the answer names', async () => {
    const print = vi.fn()
    vi.stubGlobal('print', print)
    renderDashboard(parseDocument({ ...fixture, endpoints: { ...fixture.endpoints, export: '/lens/export' } }))

    // One control in the header, not one per format.
    fireEvent.click(await screen.findByRole('button', { name: 'Export' }))
    expect(screen.getByRole('menuitem', { name: 'Data (XLSX)' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('menuitem', { name: 'Report (PDF)' }))

    expect(screen.getByRole('button', { name: 'Preparing report…' })).toBeDisabled()
    await waitFor(() => expect(print).toHaveBeenCalledOnce(), { timeout: 2_000 })

    expect(document.documentElement).toHaveClass('lens-print-active')
    expect(screen.getByText('Management audit report')).toBeInTheDocument()
    expect(screen.getAllByText('Total').length).toBeGreaterThan(1)

    window.dispatchEvent(new Event('afterprint'))
    await waitFor(() => expect(document.documentElement).not.toHaveClass('lens-print-active'))
  })

  it('exports in one click when there is only one artefact to choose', async () => {
    const print = vi.fn()
    vi.stubGlobal('print', print)
    // No export endpoint: the PDF is the only thing this dashboard produces,
    // and a menu of one is a question with no answer.
    renderDashboard(parseDocument({ ...fixture, endpoints: { ...fixture.endpoints, export: undefined } }))

    fireEvent.click(await screen.findByRole('button', { name: 'Report (PDF)' }))

    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
    await waitFor(() => expect(print).toHaveBeenCalledOnce(), { timeout: 2_000 })
  })
})
