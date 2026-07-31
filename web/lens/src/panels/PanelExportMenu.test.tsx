import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import fixture from '../../fixtures/small.json'
import { parseDocument } from '../contract'
import { DashboardPanels } from '../DashboardPanels'
import { DashboardRuntimeProvider, DocumentProvider } from '../runtime'
import * as runtime from '../runtime'

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

describe('PanelExportMenu', () => {
  it('offers XLSX, PNG, and SVG for an individual panel', async () => {
    const document_ = parseDocument({ ...fixture, endpoints: { ...fixture.endpoints, export: '/lens/export' } })
    render(
      <DocumentProvider initialDocument={document_}>
        <DashboardRuntimeProvider locale="en">
          <DashboardPanels />
        </DashboardRuntimeProvider>
      </DocumentProvider>,
    )

    fireEvent.click((await screen.findAllByRole('button', { name: 'Export panel' }))[0]!)
    expect(screen.getByRole('menuitem', { name: 'Data (XLSX)' })).toBeInTheDocument()
    expect(screen.getByRole('menuitem', { name: 'Image (PNG)' })).toBeInTheDocument()
    expect(screen.getByRole('menuitem', { name: 'Vector (SVG)' })).toBeInTheDocument()
  })

  it('supports keyboard navigation and returns focus to the trigger on Escape', async () => {
    const document_ = parseDocument({ ...fixture, endpoints: { ...fixture.endpoints, export: '/lens/export' } })
    render(
      <DocumentProvider initialDocument={document_}>
        <DashboardRuntimeProvider locale="en">
          <DashboardPanels />
        </DashboardRuntimeProvider>
      </DocumentProvider>,
    )

    const trigger = (await screen.findAllByRole('button', { name: 'Export panel' }))[0]!
    fireEvent.click(trigger)
    const data = screen.getByRole('menuitem', { name: 'Data (XLSX)' })
    const png = screen.getByRole('menuitem', { name: 'Image (PNG)' })
    await waitFor(() => expect(data).toHaveFocus())
    fireEvent.keyDown(data, { key: 'ArrowDown' })
    expect(png).toHaveFocus()
    fireEvent.keyDown(png, { key: 'Escape' })
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
    expect(trigger).toHaveFocus()
  })

  it('unmounts the open menu before capturing the panel image', async () => {
    const capture = vi.spyOn(runtime, 'downloadPanelImage').mockImplementation(() => {
      expect(screen.queryByRole('menu')).not.toBeInTheDocument()
      return Promise.resolve()
    })
    const document_ = parseDocument({ ...fixture, endpoints: { ...fixture.endpoints, export: '/lens/export' } })
    render(
      <DocumentProvider initialDocument={document_}>
        <DashboardRuntimeProvider locale="en"><DashboardPanels /></DashboardRuntimeProvider>
      </DocumentProvider>,
    )
    fireEvent.click((await screen.findAllByRole('button', { name: 'Export panel' }))[0]!)
    fireEvent.click(screen.getByRole('menuitem', { name: 'Image (PNG)' }))
    await waitFor(() => expect(capture).toHaveBeenCalled())
  })
})
