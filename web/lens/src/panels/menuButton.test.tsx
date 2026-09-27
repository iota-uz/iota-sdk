import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import fixture from '../../fixtures/small.json'
import { parseDocument } from '../contract'
import { DashboardPanels } from '../DashboardPanels'
import { DashboardRuntimeProvider, DocumentProvider } from '../runtime'

/**
 * The dismissal half of `useMenuButton`, which every Lens menu shares.
 *
 * Its keyboard half — arrow/Home/End roving and Escape returning focus — is
 * covered against the panel export menu. What had no coverage at all is how a
 * menu closes by pointer, and that is the half with the subtler contract: the
 * menu is drawn in a body portal, so the "did the press land outside?" test
 * cannot be `container.contains(target)` alone. Getting that wrong dismissed a
 * portalled item on pointerdown, before the click that chose it could land.
 */

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

function renderDashboard(exportEndpoint: string | undefined) {
  render(
    <div className="lens-root">
      <DocumentProvider initialDocument={parseDocument({
        ...fixture,
        endpoints: { ...fixture.endpoints, export: exportEndpoint },
      })}>
        <DashboardRuntimeProvider locale="en">
          <DashboardPanels />
        </DashboardRuntimeProvider>
      </DocumentProvider>
    </div>,
  )
}

async function openPanelMenu() {
  const trigger = (await screen.findAllByRole('button', { name: 'Export panel' }))[0]!
  fireEvent.click(trigger)
  await screen.findByRole('menu')
  return trigger
}

describe('menu dismissal', () => {
  it('closes on a press outside and leaves focus where the reader put it', async () => {
    renderDashboard('/lens/export')
    const trigger = await openPanelMenu()
    await waitFor(() => expect(screen.getByRole('menuitem', { name: 'Data (XLSX)' })).toHaveFocus())

    fireEvent.pointerDown(globalThis.document.body)

    await waitFor(() => expect(screen.queryByRole('menu')).not.toBeInTheDocument())
    // Escape is a retreat and hands the trigger back; pressing somewhere else
    // is a move towards that other thing, so the menu gets out of the way
    // without stealing the focus the press is about to place.
    expect(trigger).not.toHaveFocus()
  })

  it('keeps a portalled item alive long enough to receive the click that chose it', async () => {
    renderDashboard('/lens/export')
    await openPanelMenu()
    const item = screen.getByRole('menuitem', { name: 'Data (XLSX)' })
    // The menu is portalled out of the trigger's container, so a naive
    // outside-press test would treat its own items as outside and close on
    // this event — the item would be gone before the click arrived.
    fireEvent.pointerDown(item)
    expect(screen.getByRole('menu')).toBeInTheDocument()

    fireEvent.click(item)
    await waitFor(() => expect(screen.queryByRole('menu')).not.toBeInTheDocument())
  })

  it('is reachable outside the panel that owns it', async () => {
    renderDashboard('/lens/export')
    const trigger = await openPanelMenu()
    const menu = screen.getByRole('menu')

    // Drawn in a body portal so no panel's overflow can clip it, and still
    // announced as the trigger's menu rather than orphaned.
    expect(menu.closest('.lens-panel')).toBeNull()
    expect(trigger).toHaveAttribute('aria-haspopup', 'menu')
    expect(trigger).toHaveAttribute('aria-expanded', 'true')
  })
})

describe('dashboard export trigger', () => {
  it('advertises a menu only when there is a choice to make', async () => {
    renderDashboard('/lens/export')
    const trigger = await screen.findByRole('button', { name: 'Export', expanded: false })
    expect(trigger).toHaveAttribute('aria-haspopup', 'menu')

    fireEvent.click(trigger)
    await waitFor(() => expect(trigger).toHaveAttribute('aria-expanded', 'true'))
    fireEvent.click(trigger)
    await waitFor(() => expect(trigger).toHaveAttribute('aria-expanded', 'false'))
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
  })

  it('promises no menu when the only artefact is the one on the button', async () => {
    // Without an export endpoint the report is the whole offer, so the trigger
    // runs it directly. Advertising a popup it never opens would be a lie to
    // anyone navigating by the accessibility tree.
    renderDashboard(undefined)
    const trigger = await screen.findByRole('button', { name: 'Report (PDF)' })

    expect(trigger).not.toHaveAttribute('aria-haspopup')
    expect(trigger).not.toHaveAttribute('aria-expanded')
  })
})
