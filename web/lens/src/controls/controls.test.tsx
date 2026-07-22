import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import fixture from '../../fixtures/small.json'
import { parseDocument, type DashboardDocument } from '../contract'
import { DashboardRuntimeProvider, DocumentProvider } from '../runtime'
import { Calendar } from './Calendar'
import { FilterBar } from './FilterBar'
import type { RangeSelection } from './model'

const identityTranslate = (_key: string, fallback: string, vars?: Readonly<Record<string, string | number>>) => {
  if (!vars) return fallback
  return fallback.replace(/\{(\w+)\}/g, (match, name: string) => (name in vars ? String(vars[name]) : match))
}

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  window.history.replaceState(null, '', '/')
})

describe('Calendar', () => {
  const baseProps = {
    locale: 'en',
    draft: {},
    today: { year: 2026, month: 7, day: 22 },
    translate: identityTranslate,
  }

  it('navigates the grid by keyboard and completes a range with Enter', () => {
    const picks: Array<RangeSelection> = []
    render(<Calendar {...baseProps} onPick={(selection) => picks.push(selection)} />)
    const grid = screen.getByRole('grid')
    const focused = () => grid.querySelector<HTMLElement>('[data-focused="true"]')!

    expect(focused().getAttribute('aria-label')).toContain('Jul 22')
    fireEvent.keyDown(grid, { key: 'ArrowRight' })
    expect(focused().getAttribute('aria-label')).toContain('Jul 23')
    fireEvent.keyDown(grid, { key: 'ArrowDown' })
    expect(focused().getAttribute('aria-label')).toContain('Jul 30')
    // Bare "en" maximizes to en-US: Sunday-first per CLDR.
    fireEvent.keyDown(grid, { key: 'Home' })
    expect(focused().getAttribute('aria-label')).toContain('Jul 26')
    fireEvent.keyDown(grid, { key: 'End' })
    expect(focused().getAttribute('aria-label')).toContain('Aug 1')

    fireEvent.keyDown(grid, { key: 'Enter' })
    expect(picks).toHaveLength(1)
    expect(picks[0]!.complete).toBeUndefined()
  })

  it('changes month with PageDown and announces it', () => {
    render(<Calendar {...baseProps} onPick={() => undefined} />)
    const grid = screen.getByRole('grid')
    fireEvent.keyDown(grid, { key: 'PageDown' })
    expect(grid.getAttribute('aria-label')).toContain('August 2026')
    expect(screen.getByRole('status').textContent).toContain('August 2026')
  })

  it('shows a live hover preview between the anchor and the hovered day', () => {
    render(
      <Calendar
        {...baseProps}
        draft={{ start: { year: 2026, month: 7, day: 3 } }}
        onPick={() => undefined}
      />,
    )
    const day = (label: string) => screen.getByRole('gridcell', { name: label })
    fireEvent.mouseEnter(day('Jul 6, 2026'))
    expect(day('Jul 4, 2026').dataset.state).toBe('preview')
    expect(day('Jul 5, 2026').dataset.state).toBe('preview')
    expect(day('Jul 6, 2026').dataset.state).toBe('previewEdge')
    expect(day('Jul 7, 2026').dataset.state).toBeUndefined()
  })

  it('disables days outside min/max and refuses to pick them', () => {
    const picks: Array<RangeSelection> = []
    render(
      <Calendar
        {...baseProps}
        max={{ year: 2026, month: 7, day: 25 }}
        min={{ year: 2026, month: 7, day: 10 }}
        onPick={(selection) => picks.push(selection)}
      />,
    )
    const day = screen.getByRole('gridcell', { name: 'Jul 28, 2026' })
    expect(day).toBeDisabled()
    fireEvent.click(day)
    expect(picks).toHaveLength(0)
  })

  it('announces range completion through the live region', () => {
    render(
      <Calendar
        {...baseProps}
        draft={{ start: { year: 2026, month: 7, day: 3 } }}
        onPick={() => undefined}
      />,
    )
    fireEvent.click(screen.getByRole('gridcell', { name: 'Jul 9, 2026' }))
    expect(screen.getByRole('status').textContent).toContain('Jul 3, 2026')
    expect(screen.getByRole('status').textContent).toContain('Jul 9, 2026')
  })

  it('renders localized weekday headers per first day of week', () => {
    const { unmount } = render(<Calendar {...baseProps} locale="ru" onPick={() => undefined} />)
    const headersRu = screen.getAllByRole('columnheader').map((cell) => cell.textContent?.toLowerCase())
    expect(headersRu[0]).toContain('пн')
    unmount()
    render(<Calendar {...baseProps} locale="en-US" onPick={() => undefined} />)
    const headersUs = screen.getAllByRole('columnheader').map((cell) => cell.textContent)
    expect(headersUs[0]).toBe('Sun')
  })
})

function documentWithPeriod(value: { start: string; end: string }): DashboardDocument {
  return parseDocument({
    ...fixture,
    filters: [{
      id: 'period',
      kind: 'period',
      label: 'Period',
      period: {
        startParam: 'ActualRangeStart',
        endParam: 'ActualRangeEnd',
        value,
        allowEmpty: true,
        presets: [
          { id: 'year-2025', label: '2025', value: { start: '2025-01-01', end: '2025-12-31' } },
          { id: 'year-2026', label: '2026', value: { start: '2026-01-01', end: '2026-12-31' } },
          { id: 'all', label: 'All time', value: { start: '', end: '' } },
        ],
      },
    }],
  })
}

/** A fake document endpoint that echoes the requested period back, like the
 * server's normalization echo. */
function periodFetcher(calls: Array<string>): typeof fetch {
  return (input: RequestInfo | URL) => {
    const url = new URL(String(input), 'http://localhost/')
    calls.push(`${url.pathname}${url.search}`)
    const start = url.searchParams.get('ActualRangeStart')
    const end = url.searchParams.get('ActualRangeEnd')
    const value = start !== null && end !== null
      ? { start, end }
      : { start: '2026-01-01', end: '2026-07-22' }
    return Promise.resolve(new Response(
      JSON.stringify(documentWithPeriod(value)),
      { status: 200, headers: { 'Content-Type': 'application/json' } },
    ))
  }
}

function FiltersFixture({ fetcher }: { fetcher: typeof fetch }) {
  return (
    <div className="lens-root" data-theme="light">
      <DocumentProvider fetcher={fetcher} src="/lens/document">
        <DashboardRuntimeProvider fetcher={fetcher} locale="en">
          <FilterBar today={{ year: 2026, month: 7, day: 22 }} />
        </DashboardRuntimeProvider>
      </DocumentProvider>
    </div>
  )
}

describe('FilterBar runtime integration', () => {
  it('renders declared presets with the active one pressed', async () => {
    window.history.replaceState(null, '', '/dash')
    render(<FiltersFixture fetcher={periodFetcher([])} />)
    const chip = await screen.findByRole('button', { name: '2025' })
    expect(chip.getAttribute('aria-pressed')).toBe('false')
    // Document default is 2026-01-01..2026-07-22, matching no preset.
    expect(screen.getByRole('button', { name: '2026' }).getAttribute('aria-pressed')).toBe('false')
  })

  it('drives the URL and refetches on preset click; Back restores without timers', async () => {
    window.history.replaceState(null, '', '/dash')
    const calls: Array<string> = []
    render(<FiltersFixture fetcher={periodFetcher(calls)} />)

    const chip = await screen.findByRole('button', { name: '2025' })
    fireEvent.click(chip)

    expect(window.location.search).toBe('?ActualRangeStart=2025-01-01&ActualRangeEnd=2025-12-31')
    await waitFor(() => {
      expect(calls.at(-1)).toBe('/lens/document?ActualRangeStart=2025-01-01&ActualRangeEnd=2025-12-31')
    })
    await waitFor(() => {
      expect(screen.getByRole('button', { name: '2025' }).getAttribute('aria-pressed')).toBe('true')
    })

    // Browser Back: the URL is the whole state; popstate re-reads it. The spy
    // brackets only the restore itself, proving no resync timer is armed.
    const timeoutSpy = vi.spyOn(globalThis, 'setTimeout')
    const intervalSpy = vi.spyOn(globalThis, 'setInterval')
    act(() => {
      window.history.replaceState(null, '', '/dash')
      window.dispatchEvent(new PopStateEvent('popstate'))
    })
    const delayedTimers = timeoutSpy.mock.calls.filter(([, delay]) => (delay ?? 0) > 0)
    const intervals = intervalSpy.mock.calls.length
    timeoutSpy.mockRestore()
    intervalSpy.mockRestore()
    expect(delayedTimers).toHaveLength(0)
    expect(intervals).toBe(0)
    await waitFor(() => {
      expect(calls.at(-1)).toBe('/lens/document')
    })
    await waitFor(() => {
      expect(screen.getByRole('button', { name: '2025' }).getAttribute('aria-pressed')).toBe('false')
    })
  })

  it('submits the present-but-empty all-time form', async () => {
    window.history.replaceState(null, '', '/dash')
    const calls: Array<string> = []
    render(<FiltersFixture fetcher={periodFetcher(calls)} />)
    fireEvent.click(await screen.findByRole('button', { name: 'All time' }))
    expect(window.location.search).toBe('?ActualRangeStart=&ActualRangeEnd=')
    await waitFor(() => {
      expect(calls.at(-1)).toBe('/lens/document?ActualRangeStart=&ActualRangeEnd=')
    })
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'All time' }).getAttribute('aria-pressed')).toBe('true')
    })
  })

  it('opens the calendar popover and submits a picked range as wire dates', async () => {
    window.history.replaceState(null, '', '/dash')
    const calls: Array<string> = []
    render(<FiltersFixture fetcher={periodFetcher(calls)} />)
    fireEvent.click(await screen.findByRole('button', { name: /Change period/ }))
    const dialog = await screen.findByRole('dialog')
    expect(dialog).toBeInTheDocument()

    fireEvent.click(screen.getByRole('gridcell', { name: 'Jan 3, 2026' }))
    fireEvent.click(screen.getByRole('gridcell', { name: 'Jan 9, 2026' }))

    expect(window.location.search).toBe('?ActualRangeStart=2026-01-03&ActualRangeEnd=2026-01-09')
    await waitFor(() => {
      expect(calls.at(-1)).toBe('/lens/document?ActualRangeStart=2026-01-03&ActualRangeEnd=2026-01-09')
    })
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('ignores URL values the declaration cannot have produced', async () => {
    window.history.replaceState(null, '', '/dash?ActualRangeStart=garbage&ActualRangeEnd=2026-01-01')
    const calls: Array<string> = []
    render(<FiltersFixture fetcher={periodFetcher(calls)} />)
    await screen.findByRole('button', { name: '2025' })
    // The invalid pair is dropped: only the plain document fetch happened.
    expect(calls).toEqual(['/lens/document'])
  })
})
