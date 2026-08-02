import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import fixture from '../../fixtures/small.json'
import { parseDocument } from '../contract'
import { DashboardPanels } from '../DashboardPanels'
import { DashboardRuntimeProvider, DocumentProvider } from '../runtime'
import * as runtime from '../runtime'
import { menuPlacement } from './useMenuButton'

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
    expect(screen.getByRole('menuitem', { name: 'Image (SVG)' })).toBeInTheDocument()
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
    fireEvent.keyDown(png, { key: 'End' })
    expect(screen.getByRole('menuitem', { name: 'Image (SVG)' })).toHaveFocus()
    fireEvent.keyDown(screen.getByRole('menuitem', { name: 'Image (SVG)' }), { key: 'Home' })
    expect(data).toHaveFocus()
    fireEvent.keyDown(data, { key: 'Escape' })
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

describe('menu placement', () => {
  const viewport = { width: 1280, height: 900 }
  const menu = { width: 200, height: 120 }

  it('keeps a menu on the trigger edge it prefers when there is room', () => {
    const trigger = { left: 600, right: 640, top: 100, bottom: 128 }
    expect(menuPlacement(trigger, menu, viewport, 'start')).toEqual({ side: 'down', align: 'start' })
    expect(menuPlacement(trigger, menu, viewport, 'end')).toEqual({ side: 'down', align: 'end' })
  })

  it('flips upward rather than running an item below the fold', () => {
    // The daily-revenue case: the trigger sits low enough that the third item
    // would land past the viewport's bottom edge.
    const trigger = { left: 600, right: 640, top: 820, bottom: 848 }
    expect(menuPlacement(trigger, menu, viewport, 'end').side).toBe('up')
  })

  it('takes the other edge when the preferred one leaves the viewport', () => {
    expect(menuPlacement({ left: 1200, right: 1240, top: 100, bottom: 128 }, menu, viewport, 'start').align)
      .toBe('end')
    expect(menuPlacement({ left: 20, right: 60, top: 100, bottom: 128 }, menu, viewport, 'end').align)
      .toBe('start')
  })
})
