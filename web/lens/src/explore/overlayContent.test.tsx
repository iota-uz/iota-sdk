import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import fixture from '../../fixtures/explore.json'
import { parseDocument, type Level, type Node } from '../contract'
import { DashboardRuntimeProvider, DocumentProvider } from '../runtime'
import { DrillOverlay } from './DrillOverlay'
import type { DrillTarget } from './model'

/**
 * The overlay is rendered directly here with hand-built targets so each header,
 * action and keyboard behaviour can be asserted without steering the whole
 * drill graph into the exact shape a case needs.
 */

const exploreDocument = parseDocument(fixture)

const node = { key: 'services', path: ['services'], label: 'Services' } as Node
const level = {} as Level

function renderOverlay(target: DrillTarget, props: Partial<Parameters<typeof DrillOverlay>[0]> = {}) {
  return render(
    <div className="lens-root">
      <DocumentProvider initialDocument={exploreDocument}>
        <DashboardRuntimeProvider locale="en">
          <DrillOverlay
            anchor={{ x: 200, y: 200 }}
            onClose={() => {}}
            onDrillChild={() => {}}
            onDrillInto={() => {}}
            onPerspective={() => {}}
            target={target}
            {...props}
          />
        </DashboardRuntimeProvider>
      </DocumentProvider>
    </div>,
  )
}

function dialog(): HTMLElement {
  return screen.getByRole('dialog')
}

function focusables(): Array<HTMLElement> {
  return Array.from(dialog().querySelectorAll<HTMLElement>(
    'button:not([disabled]), a[href], [tabindex]:not([tabindex="-1"])',
  ))
}

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

describe('overlay segment header', () => {
  it('prints the value, the swatch color, and the share of the total once, rounded like a slice label', () => {
    renderOverlay({
      node,
      label: 'Services',
      value: 8_765_432,
      share: 0.8765432,
      total: 10_000_000,
      breakdown: [],
      perspectives: [],
    }, { accentColor: '#7c3aed' })

    const header = dialog().querySelector('.lens-drill-header')!
    // The eyebrow marks it as a segment; the title carries the mark's name.
    expect(within(header as HTMLElement).getByText('Segment')).toBeInTheDocument()
    expect(within(header as HTMLElement).getByText('Services')).toBeInTheDocument()
    const swatch = header.querySelector<HTMLElement>('.lens-drill-swatch')
    expect(swatch?.style.background).toBe('rgb(124, 58, 237)')
    // 87.65432% rounds to one decimal exactly as the pie's slice label does.
    expect(header.textContent).toContain('87.7%')
    expect(header.textContent).toContain('of')
  })

  it('omits the swatch and the segment eyebrow when it describes a level, not a mark', () => {
    renderOverlay({ label: 'Operating margin', breakdown: [], perspectives: [] })

    const header = dialog().querySelector('.lens-drill-header')!
    expect(header.querySelector('.lens-drill-swatch')).toBeNull()
    expect(within(header as HTMLElement).queryByText('Segment')).toBeNull()
    expect(within(header as HTMLElement).getByText('Operating margin')).toBeInTheDocument()
  })
})

describe('copy the segment value', () => {
  function valueTarget(): DrillTarget {
    return { node, label: 'Services', value: 8_765_432, breakdown: [], perspectives: [] }
  }

  it('copies the raw machine value (not the formatted figure) and confirms on the button', async () => {
    const writeText = vi.fn(() => Promise.resolve())
    Object.defineProperty(globalThis.navigator, 'clipboard', { configurable: true, value: { writeText } })
    renderOverlay(valueTarget())

    // The on-screen figure is formatted (separators/unit); the clipboard must
    // receive the paste-ready raw number instead.
    const figure = dialog().querySelector('.lens-drill-value-figure')!.textContent
    const button = screen.getByRole('button', { name: 'Copy value' })
    fireEvent.click(button)

    expect(writeText).toHaveBeenCalledWith('8765432')
    expect(writeText).not.toHaveBeenCalledWith(figure)
    await waitFor(() => expect(screen.getByRole('button', { name: 'Copied' })).toBeInTheDocument())
  })

  it('falls back to the selection copy when the async clipboard rejects', async () => {
    const writeText = vi.fn(() => Promise.reject(new Error('denied')))
    Object.defineProperty(globalThis.navigator, 'clipboard', { configurable: true, value: { writeText } })
    const execCommand = vi.fn(() => true)
    Object.defineProperty(document, 'execCommand', { configurable: true, value: execCommand })
    renderOverlay(valueTarget())

    fireEvent.click(screen.getByRole('button', { name: 'Copy value' }))

    await waitFor(() => expect(execCommand).toHaveBeenCalledWith('copy'))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Copied' })).toBeInTheDocument())
  })

  it('stays silent when neither clipboard path is available', async () => {
    Object.defineProperty(globalThis.navigator, 'clipboard', { configurable: true, value: undefined })
    Object.defineProperty(document, 'execCommand', {
      configurable: true,
      value: () => { throw new Error('unsupported') },
    })
    renderOverlay(valueTarget())

    // No throw escapes the handler, and the button still confirms.
    fireEvent.click(screen.getByRole('button', { name: 'Copy value' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Copied' })).toBeInTheDocument())
  })
})

describe('overlay action hierarchy', () => {
  it('promotes the expansion to the single primary action', () => {
    renderOverlay({ node, label: 'Services', target: level, breakdown: [], perspectives: [] })

    const primary = screen.getByRole('button', { name: /Expand segment/ })
    expect(primary).toHaveClass('lens-drill-action-primary')
    expect(screen.queryByRole('link', { name: /Open record/ })).toBeNull()
  })

  it('promotes the leaf link when the segment does not expand', () => {
    renderOverlay({ node, label: 'Invoice', breakdown: [], perspectives: [], leafHref: '/records/1' })

    const primary = screen.getByRole('link', { name: /Open record/ })
    expect(primary).toHaveClass('lens-drill-action-primary')
    expect(screen.queryByRole('button', { name: /Expand segment/ })).toBeNull()
  })

  it('keeps the perspectives without an expansion for a choosable-only segment', () => {
    renderOverlay({
      node,
      label: 'Operating margin',
      breakdown: [],
      perspectives: [
        { id: 'a', label: 'Composition' } as never,
        { id: 'b', label: 'Trend' } as never,
      ],
    })

    expect(within(dialog()).getAllByRole('option')).toHaveLength(2)
    expect(screen.queryByRole('button', { name: /Expand segment/ })).toBeNull()
    expect(within(dialog()).queryByText(/No further detail/)).toBeNull()
  })

  it('says so when a segment carries nothing to act on', () => {
    renderOverlay({ node, label: 'Invoice', breakdown: [], perspectives: [] })

    expect(within(dialog()).getByText('No further detail')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Expand segment/ })).toBeNull()
  })
})

describe('overlay keyboard navigation', () => {
  function walkableTarget(): DrillTarget {
    return {
      node,
      label: 'Operating margin',
      value: 100,
      breakdown: [],
      perspectives: [
        { id: 'a', label: 'Composition' } as never,
        { id: 'b', label: 'Trend' } as never,
      ],
    }
  }

  it('rolls focus through every interactive element with the arrow keys', async () => {
    renderOverlay(walkableTarget())
    await waitFor(() => expect(dialog()).toHaveFocus())
    const order = focusables()
    expect(order.length).toBeGreaterThan(2)

    fireEvent.keyDown(dialog(), { key: 'ArrowDown' })
    expect(order[0]).toHaveFocus()

    fireEvent.keyDown(order[0]!, { key: 'ArrowDown' })
    expect(order[1]).toHaveFocus()

    fireEvent.keyDown(order[1]!, { key: 'End' })
    expect(order.at(-1)).toHaveFocus()

    fireEvent.keyDown(order.at(-1)!, { key: 'ArrowDown' })
    expect(order[0]).toHaveFocus()

    fireEvent.keyDown(order[0]!, { key: 'ArrowUp' })
    expect(order.at(-1)).toHaveFocus()

    fireEvent.keyDown(order.at(-1)!, { key: 'Home' })
    expect(order[0]).toHaveFocus()
  })
})
