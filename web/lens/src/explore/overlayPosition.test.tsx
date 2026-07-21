import { cleanup, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { DashboardRuntimeProvider, DocumentProvider } from '../runtime'
import fixture from '../../fixtures/explore.json'
import { parseDocument } from '../contract'
import { DrillOverlay, positionOverlay } from './DrillOverlay'
import type { DrillTarget } from './model'

/**
 * The overlay is placed from measured rectangles, so its resting position must
 * not depend on when the measurement happened. A rect read at click time is a
 * snapshot of a layout that may still move — web fonts land, the expanded
 * panel's dialog mounts — and the popover has to catch up.
 */

const exploreDocument = parseDocument(fixture)

const target: DrillTarget = {
  label: 'Operating margin',
  value: 1_840_000,
  breakdown: [],
  perspectives: [],
}

function stubRect(element: HTMLElement, rect: Partial<DOMRect>) {
  element.getBoundingClientRect = () => ({
    x: 0, y: 0, width: 0, height: 0, top: 0, left: 0, right: 0, bottom: 0,
    toJSON: () => ({}), ...rect,
  }) as DOMRect
}

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

describe('positionOverlay', () => {
  const size = { width: 320, height: 200 }
  const viewport = { width: 1600, height: 1000 }

  it('prefers the right of the anchor, flips left, then drops below', () => {
    expect(positionOverlay({ x: 400, y: 500 }, size, viewport).placement).toBe('right')
    expect(positionOverlay({ x: 1560, y: 500 }, size, viewport).placement).toBe('left')
    expect(positionOverlay({ x: 320, y: 500 }, size, { width: 640, height: 1000 }).placement).toBe('below')
  })

  it('keeps the popover inside the viewport', () => {
    const position = positionOverlay({ x: 10, y: 990 }, size, viewport)
    expect(position.left).toBeGreaterThanOrEqual(12)
    expect(position.top + size.height).toBeLessThanOrEqual(viewport.height)
  })
})

describe('DrillOverlay placement', () => {
  function renderOverlay(anchorElement: HTMLElement) {
    return render(
      <div className="lens-root">
        <DocumentProvider initialDocument={exploreDocument}>
          <DashboardRuntimeProvider locale="en">
            <DrillOverlay
              anchor={{ x: 100, y: 100 }}
              anchorElement={anchorElement}
              onClose={() => {}}
              onDrillChild={() => {}}
              onDrillInto={() => {}}
              onPerspective={() => {}}
              target={target}
            />
          </DashboardRuntimeProvider>
        </DocumentProvider>
      </div>,
    )
  }

  it('re-measures the anchor after the layout settles instead of trusting the click-time rect', async () => {
    const anchorElement = document.createElement('button')
    document.body.append(anchorElement)
    stubRect(anchorElement, { left: 100, top: 100, width: 20, height: 20, right: 120, bottom: 120 })
    renderOverlay(anchorElement)

    const dialog = await screen.findByRole('dialog')
    const initial = dialog.style.left

    // Web fonts land and the header reflows: the anchor moves 9px, exactly the
    // drift that made the expanded-panel story non-deterministic.
    stubRect(anchorElement, { left: 118, top: 100, width: 20, height: 20, right: 138, bottom: 120 })
    globalThis.dispatchEvent(new Event('resize'))

    await waitFor(() => expect(dialog.style.left).not.toBe(initial))
    expect(Number.parseFloat(dialog.style.left) - Number.parseFloat(initial)).toBe(18)
  })

  it('leaves a pointer-anchored popover where the pointer was', async () => {
    const anchorElement = document.createElement('button')
    stubRect(anchorElement, { left: 0, top: 0, width: 0, height: 0 })
    renderOverlay(anchorElement)

    const dialog = await screen.findByRole('dialog')
    const initial = dialog.style.left
    globalThis.dispatchEvent(new Event('resize'))

    // A zero-sized anchor is not a layout the popover can trust; the pointer
    // coordinates stay authoritative.
    await waitFor(() => expect(dialog.style.left).toBe(initial))
  })
})
