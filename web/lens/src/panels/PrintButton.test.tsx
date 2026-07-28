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

describe('PrintButton', () => {
  it('builds the flat audit report before opening the browser print dialog', async () => {
    const print = vi.fn()
    vi.stubGlobal('print', print)
    const dashboard = parseDocument(fixture)

    render(
      <div className="lens-root">
        <DocumentProvider initialDocument={dashboard}>
          <DashboardRuntimeProvider locale="en">
            <DashboardPanels />
          </DashboardRuntimeProvider>
        </DocumentProvider>
      </div>,
    )

    fireEvent.click(await screen.findByRole('button', { name: 'Export to PDF' }))
    expect(screen.getByRole('button', { name: 'Preparing report…' })).toBeDisabled()
    await waitFor(() => expect(print).toHaveBeenCalledOnce(), { timeout: 2_000 })

    expect(document.documentElement).toHaveClass('lens-print-active')
    expect(screen.getByText('Management audit report')).toBeInTheDocument()
    expect(screen.getAllByText('Total').length).toBeGreaterThan(1)

    window.dispatchEvent(new Event('afterprint'))
    await waitFor(() => expect(document.documentElement).not.toHaveClass('lens-print-active'))
  })
})
