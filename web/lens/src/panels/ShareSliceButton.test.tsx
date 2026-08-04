import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import fixture from '../../fixtures/small.json'
import { parseDocument } from '../contract'
import { DashboardPanels } from '../DashboardPanels'
import { DashboardRuntimeProvider, DocumentProvider } from '../runtime'

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

describe('ShareSliceButton', () => {
  it('copies the complete current URL and confirms without dropping slice state', async () => {
    history.replaceState({}, '', '/analytics/trends?from=2026-01-01&hidden=losses')
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } })

    render(
      <DocumentProvider initialDocument={parseDocument(fixture)}>
        <DashboardRuntimeProvider locale="en">
          <DashboardPanels />
        </DashboardRuntimeProvider>
      </DocumentProvider>,
    )

    fireEvent.click(screen.getByRole('button', { name: 'Copy slice link' }))
    await waitFor(() => expect(writeText).toHaveBeenCalledWith(globalThis.location.href))
    expect(writeText.mock.calls[0]?.[0]).toContain('from=2026-01-01&hidden=losses')
    expect(screen.getByRole('status')).toHaveTextContent('Link copied')
  })
})
