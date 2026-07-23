import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import fixture from '../../fixtures/small.json'
import { parseDocument, type DashboardDocument } from '../contract'
import { DashboardPanels } from '../DashboardPanels'
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

  const grids = () => screen.getAllByRole('grid')
  const firstGrid = () => grids()[0]!
  const focused = (root: HTMLElement) =>
    root.closest('.lens-calendar')!.querySelector<HTMLElement>('[data-focused="true"]')!

  it('renders two consecutive month panes without duplicate day cells', () => {
    render(<Calendar {...baseProps} onPick={() => undefined} />)
    const labels = grids().map((grid) => grid.getAttribute('aria-label'))
    expect(labels).toHaveLength(2)
    expect(labels[0]).toContain('July 2026')
    expect(labels[1]).toContain('August 2026')
    // Out-of-month padding days are decorative spans, so a date visible in
    // both panes surfaces exactly one accessible gridcell.
    expect(screen.getAllByRole('gridcell', { name: 'Jul 26, 2026' })).toHaveLength(1)
    expect(screen.getAllByRole('gridcell', { name: 'Aug 1, 2026' })).toHaveLength(1)
  })

  it('navigates the grid by keyboard and completes a range with Enter', () => {
    const picks: Array<RangeSelection> = []
    render(<Calendar {...baseProps} onPick={(selection) => picks.push(selection)} />)
    const grid = firstGrid()

    expect(focused(grid).getAttribute('aria-label')).toContain('Jul 22')
    fireEvent.keyDown(grid, { key: 'ArrowRight' })
    expect(focused(grid).getAttribute('aria-label')).toContain('Jul 23')
    fireEvent.keyDown(grid, { key: 'ArrowDown' })
    expect(focused(grid).getAttribute('aria-label')).toContain('Jul 30')
    // Bare "en" maximizes to en-US: Sunday-first per CLDR.
    fireEvent.keyDown(grid, { key: 'Home' })
    expect(focused(grid).getAttribute('aria-label')).toContain('Jul 26')
    fireEvent.keyDown(grid, { key: 'End' })
    expect(focused(grid).getAttribute('aria-label')).toContain('Aug 1')

    fireEvent.keyDown(grid, { key: 'Enter' })
    expect(picks).toHaveLength(1)
    expect(picks[0]!.complete).toBeUndefined()
  })

  it('shifts the pane window when focus moves past it and announces the month', () => {
    render(<Calendar {...baseProps} onPick={() => undefined} />)
    // August is already visible in the second pane: no window shift.
    fireEvent.keyDown(firstGrid(), { key: 'PageDown' })
    expect(grids()[0]!.getAttribute('aria-label')).toContain('July 2026')
    expect(focused(firstGrid()).getAttribute('aria-label')).toContain('Aug 22')
    // September is not: the window slides so September lands in the last pane.
    fireEvent.keyDown(firstGrid(), { key: 'PageDown' })
    expect(grids()[0]!.getAttribute('aria-label')).toContain('August 2026')
    expect(grids()[1]!.getAttribute('aria-label')).toContain('September 2026')
    expect(screen.getByRole('status').textContent).toContain('September 2026')
  })

  it('steps the window by month and year through the header buttons', () => {
    render(<Calendar {...baseProps} onPick={() => undefined} />)
    fireEvent.click(screen.getByRole('button', { name: 'Next month' }))
    expect(grids()[0]!.getAttribute('aria-label')).toContain('August 2026')
    fireEvent.click(screen.getByRole('button', { name: 'Previous year' }))
    expect(grids()[0]!.getAttribute('aria-label')).toContain('August 2025')
    expect(grids()[1]!.getAttribute('aria-label')).toContain('September 2025')
    fireEvent.click(screen.getByRole('button', { name: 'Next year' }))
    expect(grids()[0]!.getAttribute('aria-label')).toContain('August 2026')
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
    // The preview band rounds off at its outer edges: the anchor washes toward
    // the pointer, the hovered edge back toward the anchor.
    expect(day('Jul 3, 2026').dataset.band).toBe('right-preview')
    expect(day('Jul 6, 2026').dataset.band).toBe('left-preview')
    expect(day('Jul 4, 2026').dataset.band).toBeUndefined()
  })

  it('marks committed range endpoints with the band side toward the interior', () => {
    render(
      <Calendar
        {...baseProps}
        draft={{ start: { year: 2026, month: 7, day: 3 }, end: { year: 2026, month: 7, day: 18 } }}
        onPick={() => undefined}
      />,
    )
    const day = (label: string) => screen.getByRole('gridcell', { name: label })
    expect(day('Jul 3, 2026').dataset.state).toBe('start')
    expect(day('Jul 3, 2026').dataset.band).toBe('right')
    expect(day('Jul 18, 2026').dataset.state).toBe('end')
    expect(day('Jul 18, 2026').dataset.band).toBe('left')
    expect(day('Jul 10, 2026').dataset.state).toBe('inRange')
    expect(day('Jul 10, 2026').dataset.band).toBeUndefined()
    // The wash rounds off at week-row edges: Sunday-first (en), so Saturdays
    // cap right, Sundays cap left, and mid-row in-range days carry no cap.
    expect(day('Jul 4, 2026').dataset.cap).toBe('right')
    expect(day('Jul 5, 2026').dataset.cap).toBe('left')
    expect(day('Jul 10, 2026').dataset.cap).toBeUndefined()
    // Endpoints are pills, never capped washes.
    expect(day('Jul 3, 2026').dataset.cap).toBeUndefined()
  })

  it('summarizes a complete range as a day count and prompts otherwise', () => {
    const { unmount } = render(
      <Calendar
        {...baseProps}
        draft={{ start: { year: 2026, month: 7, day: 3 }, end: { year: 2026, month: 7, day: 18 } }}
        onPick={() => undefined}
      />,
    )
    expect(screen.getByText('16 d.')).toBeInTheDocument()
    unmount()
    render(<Calendar {...baseProps} onPick={() => undefined} />)
    expect(screen.getByText('Select a start date')).toBeInTheDocument()
  })

  it('draws no band for a single-day range', () => {
    render(
      <Calendar
        {...baseProps}
        draft={{ start: { year: 2026, month: 7, day: 3 }, end: { year: 2026, month: 7, day: 3 } }}
        onPick={() => undefined}
      />,
    )
    const day = screen.getByRole('gridcell', { name: 'Jul 3, 2026' })
    expect(day.dataset.state).toBe('start')
    expect(day.dataset.band).toBeUndefined()
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

/** Like documentWithPeriod, but declares no server presets so the control
 * falls back to its built-in, today-relative preset catalog. */
function documentWithoutPresets(value: { start: string; end: string }): DashboardDocument {
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
      },
    }],
  })
}

function presetlessFetcher(calls: Array<string>): typeof fetch {
  return (input: RequestInfo | URL) => {
    const raw = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    const url = new URL(raw, 'http://localhost/')
    calls.push(`${url.pathname}${url.search}`)
    const start = url.searchParams.get('ActualRangeStart')
    const end = url.searchParams.get('ActualRangeEnd')
    const value = start !== null && end !== null ? { start, end } : { start: '2026-01-01', end: '2026-07-22' }
    return Promise.resolve(new Response(
      JSON.stringify(documentWithoutPresets(value)),
      { status: 200, headers: { 'Content-Type': 'application/json' } },
    ))
  }
}

/** A fake document endpoint that echoes the requested period back, like the
 * server's normalization echo. */
function periodFetcher(calls: Array<string>): typeof fetch {
  return (input: RequestInfo | URL) => {
    const raw = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    const url = new URL(raw, 'http://localhost/')
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

function DashboardFiltersFixture({ fetcher }: { fetcher: typeof fetch }) {
  return (
    <div className="lens-root" data-theme="light">
      <DocumentProvider fetcher={fetcher} src="/lens/document">
        <DashboardRuntimeProvider fetcher={fetcher} locale="en">
          <DashboardPanels filterToday={{ year: 2026, month: 7, day: 22 }} />
        </DashboardRuntimeProvider>
      </DocumentProvider>
    </div>
  )
}

function refetchFailureFetcher(calls: Array<string>): typeof fetch {
  let filteredRequests = 0
  return (input: RequestInfo | URL) => {
    const raw = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    const url = new URL(raw, 'http://localhost/')
    const requestURL = `${url.pathname}${url.search}`
    calls.push(requestURL)
    const isFiltered = url.searchParams.has('ActualRangeStart')
    if (isFiltered && filteredRequests++ === 0) {
      return Promise.resolve(new Response(JSON.stringify({ message: 'document refetch failed' }), {
        status: 500,
        headers: { 'Content-Type': 'application/json' },
      }))
    }
    const value = isFiltered
      ? { start: url.searchParams.get('ActualRangeStart')!, end: url.searchParams.get('ActualRangeEnd')! }
      : { start: '2026-01-01', end: '2026-07-22' }
    const next = documentWithPeriod(value)
    return Promise.resolve(new Response(JSON.stringify({
      ...next,
      meta: { ...next.meta, title: isFiltered ? 'Refreshed Overview' : 'Overview' },
    }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
  }
}

describe('FilterBar runtime integration', () => {
  it('keeps the previous document visible and shows a dismissable refetch error', async () => {
    window.history.replaceState(null, '', '/dash')
    render(<DashboardFiltersFixture fetcher={refetchFailureFetcher([])} />)

    fireEvent.click(await screen.findByRole('button', { name: '2025' }))

    const banner = await screen.findByRole('alert')
    expect(banner).toHaveTextContent('Unable to refresh the dashboard. The previous data is still shown.')
    expect(screen.getByRole('heading', { name: 'Overview' })).toBeInTheDocument()
    expect(screen.getByText('42')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '2025' })).toHaveAttribute('aria-pressed', 'true')

    fireEvent.click(screen.getByRole('button', { name: 'Dismiss notice' }))
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('retries the same filtered document request', async () => {
    window.history.replaceState(null, '', '/dash')
    const calls: Array<string> = []
    render(<DashboardFiltersFixture fetcher={refetchFailureFetcher(calls)} />)

    fireEvent.click(await screen.findByRole('button', { name: '2025' }))
    fireEvent.click(await screen.findByRole('button', { name: 'Retry' }))

    const filteredURL = '/lens/document?ActualRangeStart=2025-01-01&ActualRangeEnd=2025-12-31'
    await waitFor(() => expect(calls.filter((call) => call === filteredURL)).toHaveLength(2))
  })

  it('clears the refetch error after a successful retry', async () => {
    window.history.replaceState(null, '', '/dash')
    render(<DashboardFiltersFixture fetcher={refetchFailureFetcher([])} />)

    fireEvent.click(await screen.findByRole('button', { name: '2025' }))
    fireEvent.click(await screen.findByRole('button', { name: 'Retry' }))

    expect(await screen.findByRole('heading', { name: 'Refreshed Overview' })).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument())
  })

  it('marks the trigger as the active period source when no chip matches', async () => {
    window.history.replaceState(null, '', '/dash')
    render(<FiltersFixture fetcher={periodFetcher([])} />)

    // Document default 2026-01-01..2026-07-22 matches no chip: the custom
    // range on the trigger is the active source.
    const trigger = await screen.findByRole('button', { name: /Change period/ })
    expect(trigger.dataset.active).toBe('true')

    // Once a chip's range applies, the chip is the source and the cue moves.
    fireEvent.click(screen.getByRole('button', { name: '2025' }))
    await waitFor(() => {
      expect(screen.getByRole('button', { name: '2025' })).toHaveAttribute('aria-pressed', 'true')
    })
    expect(screen.getByRole('button', { name: /Change period/ }).dataset.active).toBeUndefined()
  })

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

  it('falls back to built-in presets and resolves them against today', async () => {
    window.history.replaceState(null, '', '/dash')
    const calls: Array<string> = []
    render(<FiltersFixture fetcher={presetlessFetcher(calls)} />)

    // The legacy quick-range catalog is present even though the document
    // declared none: DefaultQuickRanges parity.
    await screen.findByRole('button', { name: 'Current month' })
    expect(screen.getByRole('button', { name: '30 days' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '12 months' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Current fiscal year' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Last month' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Last fiscal year' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Current month' }))
    // today = 2026-07-22 → this month resolves to 2026-07-01..2026-07-22.
    expect(window.location.search).toBe('?ActualRangeStart=2026-07-01&ActualRangeEnd=2026-07-22')
    await waitFor(() => {
      expect(calls.at(-1)).toBe('/lens/document?ActualRangeStart=2026-07-01&ActualRangeEnd=2026-07-22')
    })
  })

  it('surfaces the relative catalog in the popover even with server year-chips', async () => {
    window.history.replaceState(null, '', '/dash')
    const calls: Array<string> = []
    render(<FiltersFixture fetcher={periodFetcher(calls)} />)

    // Server year-chips drive the top row.
    await screen.findByRole('button', { name: '2025' })
    expect(screen.getByRole('button', { name: '2026' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /Change period/ }))
    const dialog = await screen.findByRole('dialog')

    // The full legacy quick-range catalog appears inside the popover
    // regardless of server presets — including last fiscal year, even though
    // it resolves to the same range as the server's previous-year chip.
    expect(within(dialog).getByRole('button', { name: 'Current month' })).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: '30 days' })).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: 'Current fiscal year' })).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: 'Last fiscal year' })).toBeInTheDocument()

    // Selecting a popover preset applies its resolved bounds and closes the popover.
    fireEvent.click(within(dialog).getByRole('button', { name: 'Current month' }))
    expect(window.location.search).toBe('?ActualRangeStart=2026-07-01&ActualRangeEnd=2026-07-22')
    await waitFor(() => {
      expect(calls.at(-1)).toBe('/lens/document?ActualRangeStart=2026-07-01&ActualRangeEnd=2026-07-22')
    })
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('surfaces All time in the popover preset pane and applies it', async () => {
    window.history.replaceState(null, '', '/dash')
    const calls: Array<string> = []
    render(<FiltersFixture fetcher={presetlessFetcher(calls)} />)

    fireEvent.click(await screen.findByRole('button', { name: /Change period/ }))
    const dialog = await screen.findByRole('dialog')

    const allTime = within(dialog).getByRole('button', { name: 'All time' })
    expect(allTime).toHaveAttribute('aria-pressed', 'false')
    fireEvent.click(allTime)

    expect(window.location.search).toBe('?ActualRangeStart=&ActualRangeEnd=')
    await waitFor(() => {
      expect(calls.at(-1)).toBe('/lens/document?ActualRangeStart=&ActualRangeEnd=')
    })
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('commits typed dd.mm.yyyy From/To dates through Apply', async () => {
    window.history.replaceState(null, '', '/dash')
    const calls: Array<string> = []
    render(<FiltersFixture fetcher={presetlessFetcher(calls)} />)

    fireEvent.click(await screen.findByRole('button', { name: /Change period/ }))
    await screen.findByRole('dialog')

    // The masked fields open pre-filled from the applied range.
    const from = screen.getByLabelText('From')
    const to = screen.getByLabelText('To')
    expect(from).toHaveValue('01.01.2026')
    expect(to).toHaveValue('22.07.2026')

    fireEvent.change(from, { target: { value: '05.03.2026' } })
    fireEvent.blur(from)
    fireEvent.change(to, { target: { value: '10.03.2026' } })
    fireEvent.blur(to)
    fireEvent.click(screen.getByRole('button', { name: 'Apply' }))

    expect(window.location.search).toBe('?ActualRangeStart=2026-03-05&ActualRangeEnd=2026-03-10')
    await waitFor(() => {
      expect(calls.at(-1)).toBe('/lens/document?ActualRangeStart=2026-03-05&ActualRangeEnd=2026-03-10')
    })
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('masks typed input, flags an unparseable date, and reverts it on Apply', async () => {
    window.history.replaceState(null, '', '/dash')
    render(<FiltersFixture fetcher={presetlessFetcher([])} />)

    fireEvent.click(await screen.findByRole('button', { name: /Change period/ }))
    const dialog = await screen.findByRole('dialog')

    const from = screen.getByLabelText('From')
    // The mask strips separators and non-digits while typing.
    fireEvent.change(from, { target: { value: '05032026' } })
    expect(from).toHaveValue('05.03.2026')

    // An impossible date marks the field invalid on blur without touching the
    // draft; an Apply attempt then reverts to the last valid value.
    fireEvent.change(from, { target: { value: '99.99.2026' } })
    fireEvent.blur(from)
    const wrapper = from.closest('.lens-filter-range-input') as HTMLElement
    expect(wrapper.dataset.invalid).toBe('true')
    fireEvent.click(within(dialog).getByRole('button', { name: 'Apply' }))
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(screen.getByLabelText('From')).toHaveValue('01.01.2026')
    expect(window.location.search).toBe('')
  })

  it('labels the preset rail and summarizes the open range as a day count', async () => {
    window.history.replaceState(null, '', '/dash')
    render(<FiltersFixture fetcher={presetlessFetcher([])} />)

    fireEvent.click(await screen.findByRole('button', { name: /Change period/ }))
    const dialog = await screen.findByRole('dialog')

    expect(within(dialog).getByText('Quick select')).toBeInTheDocument()
    // 2026-01-01 .. 2026-07-22 inclusive.
    expect(within(dialog).getByText('203 d.')).toBeInTheDocument()
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
