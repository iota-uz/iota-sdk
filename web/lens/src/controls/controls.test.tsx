import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import fixture from '../../fixtures/small.json'
import { parseDocument, type DashboardDocument } from '../contract'
import { DashboardPanels } from '../DashboardPanels'
import { DashboardRuntimeProvider, DocumentProvider } from '../runtime'
import { stackedCalendarMediaQuery } from '../breakpoints'
import { Calendar } from './Calendar'
import { FilterBar } from './FilterBar'
import { stagedFilterURL } from './FacetFilterMenu'
import type { RangeSelection } from './model'

const identityTranslate = (_key: string, fallback: string, vars?: Readonly<Record<string, string | number>>) => {
  if (!vars) return fallback
  return fallback.replace(/\{(\w+)\}/g, (match, name: string) => (name in vars ? String(vars[name]) : match))
}

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
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

  // Six rows per pane whatever the month: a month can otherwise end in a row
  // holding a single date, and the card changes height as the window steps.
  // The surplus rows are padding, so they add no gridcell to either pane.
  it('draws every month on the same six-row grid', () => {
    // February 2026 starts on Sunday and fills exactly four Sunday-first weeks.
    render(<Calendar {...baseProps} onPick={() => undefined} today={{ year: 2026, month: 2, day: 10 }} />)
    for (const grid of grids()) {
      const weeks = grid.querySelectorAll('.lens-calendar-week')
      expect(weeks).toHaveLength(6)
    }
    const february = firstGrid()
    expect(february.getAttribute('aria-label')).toContain('February 2026')
    expect(within(february).getAllByRole('gridcell')).toHaveLength(28)
    const lastWeek = february.querySelectorAll('.lens-calendar-week')[5]!
    expect(within(lastWeek as HTMLElement).queryAllByRole('gridcell')).toHaveLength(0)
  })

  it('uses one pane whenever the full popover would exceed the viewport', () => {
    const addEventListener = vi.fn()
    const removeEventListener = vi.fn()
    vi.stubGlobal('matchMedia', vi.fn((query: string) => ({
      matches: query === stackedCalendarMediaQuery,
      addEventListener,
      removeEventListener,
    })))

    render(<Calendar {...baseProps} onPick={() => undefined} />)

    expect(grids()).toHaveLength(1)
    expect(document.querySelector('.lens-calendar')).toHaveAttribute('data-panes', '1')
    expect(addEventListener).toHaveBeenCalledWith('change', expect.any(Function))
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

  it('steps the window by month through the header buttons', () => {
    render(<Calendar {...baseProps} onPick={() => undefined} />)
    fireEvent.click(screen.getByRole('button', { name: 'Next month' }))
    expect(grids()[0]!.getAttribute('aria-label')).toContain('August 2026')
    fireEvent.click(screen.getByRole('button', { name: 'Previous month' }))
    expect(grids()[0]!.getAttribute('aria-label')).toContain('July 2026')
  })

  it('travels by year through the month panel behind the heading', () => {
    render(<Calendar {...baseProps} onPick={() => undefined} />)

    // The heading opens the month panel for its own year; the panel replaces
    // the day grid rather than resizing the popover.
    fireEvent.click(screen.getByRole('button', { name: /Choose month: July 2026/ }))
    expect(screen.queryAllByRole('grid')).toHaveLength(0)
    expect(screen.getByRole('button', { name: '2026' })).toBeInTheDocument()

    // One more level up: the twelve-year block, aligned to multiples of twelve.
    fireEvent.click(screen.getByRole('button', { name: '2026' }))
    expect(screen.getByRole('button', { name: '2016 – 2027' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Previous' }))
    expect(screen.getByRole('button', { name: '2004 – 2015' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Next' }))
    expect(screen.getByRole('button', { name: '2016 – 2027' })).toBeInTheDocument()

    // Year, then month: the window lands there and the day grid comes back.
    fireEvent.click(screen.getByRole('button', { name: '2019' }))
    fireEvent.click(screen.getByRole('button', { name: 'Mar' }))
    expect(grids()[0]!.getAttribute('aria-label')).toContain('March 2019')
    expect(grids()[1]!.getAttribute('aria-label')).toContain('April 2019')
  })

  it('refuses months and years the min/max bounds exclude', () => {
    render(
      <Calendar
        {...baseProps}
        max={{ year: 2026, month: 7, day: 25 }}
        min={{ year: 2026, month: 3, day: 10 }}
        onPick={() => undefined}
      />,
    )
    fireEvent.click(screen.getByRole('button', { name: 'Choose month: July 2026' }))
    expect(screen.getByRole('button', { name: 'Feb' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Mar' })).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Aug' })).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: '2026' }))
    expect(screen.getByRole('button', { name: '2025' })).toBeDisabled()
    expect(screen.getByRole('button', { name: '2026' })).toBeEnabled()
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

function documentWithCompare(value: { mode: 'off' | 'previous_period' | 'year_ago' | 'custom'; start?: string; end?: string }): DashboardDocument {
  return parseDocument({
    ...fixture,
    filters: [{
      id: 'compare',
      kind: 'compare',
      label: 'Compare with',
      compare: {
        modeParam: 'compare',
        startParam: 'compare_start',
        endParam: 'compare_end',
        compareTo: 'period',
        value,
      },
    }],
  })
}

function allTimeDocumentWithCompareOff(): DashboardDocument {
  const period = documentWithPeriod({ start: '', end: '' }).filters![0]!
  const compare = documentWithCompare({ mode: 'off' }).filters![0]!
  return parseDocument({ ...fixture, filters: [period, compare] })
}

function compareFetcher(calls: Array<string>): typeof fetch {
  return (input: RequestInfo | URL) => {
    const raw = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    const url = new URL(raw, 'http://localhost/')
    calls.push(`${url.pathname}${url.search}`)
    const mode = url.searchParams.get('compare')
    const start = url.searchParams.get('compare_start')
    const end = url.searchParams.get('compare_end')
    const value = mode === 'custom' && start && end
      ? { mode: 'custom' as const, start, end }
      : { mode: 'off' as const }
    return Promise.resolve(new Response(
      JSON.stringify(documentWithCompare(value)),
      { status: 200, headers: { 'Content-Type': 'application/json' } },
    ))
  }
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

  // The comparison control is a trigger + listbox (see CompareFilterControl):
  // the trigger names the applied mode, the options live in a portalled popover.
  const compareTrigger = () => screen.findByRole('button', { name: /Compare with/ })
  const compareOption = (name: string) => screen.getByRole('option', { name })

  // A declared preset is a rail entry in the picker, not a chip in the header:
  // the header carries the applied range and the two arrows that move it.
  const openPeriod = async () => {
    // Idempotent: the trigger toggles, so a second open would close the picker.
    if (!screen.queryByRole('dialog')) {
      fireEvent.click(await screen.findByRole('button', { name: /Change period/ }))
    }
    return screen.findByRole('dialog')
  }
  const applyPreset = async (name: string) => {
    fireEvent.click(within(await openPeriod()).getByRole('button', { name }))
  }
  const presetState = async (name: string) => {
    const state = within(await openPeriod()).getByRole('button', { name }).getAttribute('aria-pressed')
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    return state
  }

  it('replaces an impossible all-time comparison deep link after the normalized document loads', async () => {
    window.history.replaceState(null, '', '/dash?ActualRangeStart=&ActualRangeEnd=&compare=custom&compare_start=2025-01-01&compare_end=2025-08-01')
    const fetcher: typeof fetch = () => Promise.resolve(new Response(
      JSON.stringify(allTimeDocumentWithCompareOff()),
      { status: 200, headers: { 'Content-Type': 'application/json' } },
    ))
    render(<FiltersFixture fetcher={fetcher} />)

    const trigger = await compareTrigger()
    await waitFor(() => expect(window.location.search).toBe('?ActualRangeStart=&ActualRangeEnd=&compare=off'))
    expect(trigger).toHaveTextContent('Comparison off')
  })

  it('stages a custom comparison interval before applying it', async () => {
    window.history.replaceState(null, '', '/dash')
    const calls: Array<string> = []
    render(<FiltersFixture fetcher={compareFetcher(calls)} />)

    fireEvent.click(await compareTrigger())
    fireEvent.click(compareOption('Custom interval'))

    // Selecting "custom" only stages: the fields it needs appear inside the
    // popover and nothing is requested until Apply.
    expect(screen.getByLabelText('Comparison start')).toBeInTheDocument()
    expect(screen.getByLabelText('Comparison end')).toBeInTheDocument()
    expect(calls).toEqual(['/lens/document'])

    // The boundaries are the runtime's own masked dd.mm.yyyy fields, not the
    // platform's date widget: they commit on blur, like the period picker's.
    const start = screen.getByLabelText('Comparison start')
    const end = screen.getByLabelText('Comparison end')
    fireEvent.change(start, { target: { value: '01062026' } })
    expect(start).toHaveValue('01.06.2026')
    fireEvent.blur(start)
    fireEvent.change(end, { target: { value: '30.06.2026' } })
    fireEvent.blur(end)
    fireEvent.click(screen.getByRole('button', { name: 'Apply' }))

    await waitFor(() => expect(calls).toContain('/lens/document?compare=custom&compare_start=2026-06-01&compare_end=2026-06-30'))
    await waitFor(() => expect(screen.getByRole('button', { name: /Compare with/ })).toHaveTextContent('Custom interval'))

    fireEvent.click(await compareTrigger())
    fireEvent.click(compareOption('Comparison off'))
    await waitFor(() => expect(calls).toContain('/lens/document?compare=off'))
    expect(window.location.search).toBe('?compare=off')
  })

  it('operates the comparison listbox from the keyboard and leaves it unchanged on Escape', async () => {
    window.history.replaceState(null, '', '/dash')
    const calls: Array<string> = []
    render(<FiltersFixture fetcher={compareFetcher(calls)} />)

    const trigger = await compareTrigger()
    trigger.focus()
    fireEvent.keyDown(trigger, { key: 'ArrowDown' })

    const listbox = await screen.findByRole('listbox', { name: 'Compare with' })
    // The applied mode is where the roving tabindex starts.
    await waitFor(() => expect(compareOption('Comparison off')).toHaveFocus())

    fireEvent.keyDown(compareOption('Comparison off'), { key: 'ArrowDown' })
    expect(compareOption('Previous period')).toHaveFocus()
    // Focus moved; nothing was selected, and nothing was requested.
    expect(compareOption('Comparison off')).toHaveAttribute('aria-selected', 'true')
    expect(calls).toEqual(['/lens/document'])

    // Type-ahead jumps to the first option matching what was typed.
    fireEvent.keyDown(compareOption('Previous period'), { key: 'y' })
    expect(compareOption('Year ago')).toHaveFocus()

    fireEvent.keyDown(compareOption('Year ago'), { key: 'Escape' })
    await waitFor(() => expect(listbox).not.toBeInTheDocument())
    expect(trigger).toHaveFocus()
    expect(trigger).toHaveTextContent('Comparison off')
    expect(calls).toEqual(['/lens/document'])

    // Enter on a focused option is what commits it.
    fireEvent.keyDown(trigger, { key: 'ArrowDown' })
    await waitFor(() => expect(compareOption('Comparison off')).toHaveFocus())
    fireEvent.keyDown(compareOption('Comparison off'), { key: 'ArrowDown' })
    fireEvent.keyDown(compareOption('Previous period'), { key: 'Enter' })
    await waitFor(() => expect(calls).toContain('/lens/document?compare=previous_period'))
  })

  it('keeps the previous document visible and shows a dismissable refetch error', async () => {
    window.history.replaceState(null, '', '/dash')
    render(<DashboardFiltersFixture fetcher={refetchFailureFetcher([])} />)

    await applyPreset('2025')

    const banner = await screen.findByRole('alert')
    expect(banner).toHaveTextContent('Unable to refresh the dashboard. The previous data is still shown.')
    expect(screen.getByRole('heading', { name: 'Overview' })).toBeInTheDocument()
    expect(screen.getByText('42')).toBeInTheDocument()
    expect(await presetState('2025')).toBe('true')

    fireEvent.click(screen.getByRole('button', { name: 'Dismiss notice' }))
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('retries the same filtered document request', async () => {
    window.history.replaceState(null, '', '/dash')
    const calls: Array<string> = []
    render(<DashboardFiltersFixture fetcher={refetchFailureFetcher(calls)} />)

    await applyPreset('2025')
    fireEvent.click(await screen.findByRole('button', { name: 'Retry' }))

    const filteredURL = '/lens/document?ActualRangeStart=2025-01-01&ActualRangeEnd=2025-12-31'
    await waitFor(() => expect(calls.filter((call) => call === filteredURL)).toHaveLength(2))
  })

  it('clears the refetch error after a successful retry', async () => {
    window.history.replaceState(null, '', '/dash')
    render(<DashboardFiltersFixture fetcher={refetchFailureFetcher([])} />)

    await applyPreset('2025')
    fireEvent.click(await screen.findByRole('button', { name: 'Retry' }))

    expect(await screen.findByRole('heading', { name: 'Refreshed Overview' })).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument())
  })

  it('steps a running year back to the whole year before it, and announces where', async () => {
    window.history.replaceState(null, '', '/dash')
    const calls: Array<string> = []
    render(<FiltersFixture fetcher={periodFetcher(calls)} />)

    // Document default 2026-01-01..2026-07-22 is the current year still
    // running. A step back is the comparison the year chips existed to offer:
    // the whole of 2025, not January-to-July of it.
    const back = await screen.findByRole('button', { name: /Previous period/ })
    expect(back).toHaveAccessibleName('Previous period: 01.01.2025 – 31.12.2025')
    fireEvent.click(back)

    expect(window.location.search).toBe('?ActualRangeStart=2025-01-01&ActualRangeEnd=2025-12-31')
    await waitFor(() => {
      expect(calls.at(-1)).toBe('/lens/document?ActualRangeStart=2025-01-01&ActualRangeEnd=2025-12-31')
    })
    // And forward re-opens the current year at today rather than running it to
    // a December that has not happened.
    expect(await screen.findByRole('button', { name: /Next period/ }))
      .toHaveAccessibleName('Next period: 01.01.2026 – 22.07.2026')
  })

  it('steps a hand-drawn range by its own length', async () => {
    window.history.replaceState(null, '', '/dash?ActualRangeStart=2026-07-06&ActualRangeEnd=2026-07-22')
    render(<FiltersFixture fetcher={periodFetcher([])} />)

    // 17 days lands on the 17 days before it: no gap, no overlap.
    const back = await screen.findByRole('button', { name: /Previous period/ })
    expect(back).toHaveAccessibleName('Previous period: 19.06.2026 – 05.07.2026')
    fireEvent.click(back)
    expect(window.location.search).toBe('?ActualRangeStart=2026-06-19&ActualRangeEnd=2026-07-05')
  })

  it('offers no step for a period with no length', async () => {
    window.history.replaceState(null, '', '/dash?ActualRangeStart=&ActualRangeEnd=')
    render(<FiltersFixture fetcher={periodFetcher([])} />)

    // "All time" has no length to step by. The arrows stay in place rather
    // than vanishing, which would move the range out from under the pointer.
    expect(await screen.findByRole('button', { name: /Previous period/ })).toBeDisabled()
    expect(screen.getByRole('button', { name: /Next period/ })).toBeDisabled()
  })

  it('renders declared presets in the picker with the active one pressed', async () => {
    window.history.replaceState(null, '', '/dash')
    render(<FiltersFixture fetcher={periodFetcher([])} />)
    const dialog = await openPeriod()
    expect(within(dialog).getByRole('button', { name: '2025' }).getAttribute('aria-pressed')).toBe('false')
    // Document default is 2026-01-01..2026-07-22, matching no preset.
    expect(within(dialog).getByRole('button', { name: '2026' }).getAttribute('aria-pressed')).toBe('false')
  })

  it('drives the URL and refetches on preset click; Back restores without timers', async () => {
    window.history.replaceState(null, '', '/dash')
    const calls: Array<string> = []
    render(<FiltersFixture fetcher={periodFetcher(calls)} />)

    await applyPreset('2025')

    expect(window.location.search).toBe('?ActualRangeStart=2025-01-01&ActualRangeEnd=2025-12-31')
    await waitFor(() => {
      expect(calls.at(-1)).toBe('/lens/document?ActualRangeStart=2025-01-01&ActualRangeEnd=2025-12-31')
    })
    await waitFor(async () => {
      expect(await presetState('2025')).toBe('true')
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
    await waitFor(async () => {
      expect(await presetState('2025')).toBe('false')
    })
  })

  it('submits the present-but-empty all-time form', async () => {
    window.history.replaceState(null, '', '/dash')
    const calls: Array<string> = []
    render(<FiltersFixture fetcher={periodFetcher(calls)} />)
    await applyPreset('All time')
    expect(window.location.search).toBe('?ActualRangeStart=&ActualRangeEnd=')
    await waitFor(() => {
      expect(calls.at(-1)).toBe('/lens/document?ActualRangeStart=&ActualRangeEnd=')
    })
    await waitFor(async () => {
      expect(await presetState('All time')).toBe('true')
    })
  })

  it('opens the calendar popover and submits a picked range as wire dates', async () => {
    window.history.replaceState(null, '', '/dash')
    const calls: Array<string> = []
    render(<FiltersFixture fetcher={periodFetcher(calls)} />)
    fireEvent.click(await screen.findByRole('button', { name: /Change period/ }))
    const dialog = await screen.findByRole('dialog')
    expect(dialog).toBeInTheDocument()

    // Calendar picks are a draft: nothing is applied until Apply, so the
    // popover has exactly one commit path however the range was entered.
    fireEvent.click(screen.getByRole('gridcell', { name: 'Jan 3, 2026' }))
    fireEvent.click(screen.getByRole('gridcell', { name: 'Jan 9, 2026' }))
    expect(window.location.search).toBe('')
    expect(within(dialog).getByText('Duration: 7 d.')).toBeInTheDocument()

    fireEvent.click(within(dialog).getByRole('button', { name: 'Apply' }))
    expect(window.location.search).toBe('?ActualRangeStart=2026-01-03&ActualRangeEnd=2026-01-09')
    await waitFor(() => {
      expect(calls.at(-1)).toBe('/lens/document?ActualRangeStart=2026-01-03&ActualRangeEnd=2026-01-09')
    })
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('abandons the draft on Cancel and reopens from the applied range', async () => {
    window.history.replaceState(null, '', '/dash')
    const calls: Array<string> = []
    render(<FiltersFixture fetcher={periodFetcher(calls)} />)
    fireEvent.click(await screen.findByRole('button', { name: /Change period/ }))
    const dialog = await screen.findByRole('dialog')

    fireEvent.click(within(dialog).getByRole('gridcell', { name: 'Jan 3, 2026' }))
    fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(window.location.search).toBe('')
    expect(calls).toEqual(['/lens/document'])

    // Reopened, the fields state the applied range again, not the abandoned pick.
    fireEvent.click(screen.getByRole('button', { name: /Change period/ }))
    await screen.findByRole('dialog')
    expect(screen.getByLabelText('From')).toHaveValue('01.01.2026')
    expect(screen.getByLabelText('To')).toHaveValue('22.07.2026')
  })

  it('states the applied period once, on the trigger, whatever produced it', async () => {
    window.history.replaceState(null, '', '/dash')
    render(<FiltersFixture fetcher={periodFetcher([])} />)
    const group = await screen.findByRole('group', { name: 'Period' })

    // Three buttons and no caption: back, the applied range, forward. The
    // declared years are in the picker, so nothing in the header competes with
    // the trigger to say which period is on screen.
    expect(within(group).getAllByRole('button')).toHaveLength(3)
    expect(within(group).getByRole('button', { name: /Change period/ }))
      .toHaveTextContent('01.01.2026 – 22.07.2026')
    expect(within(group).queryByRole('button', { name: '2025' })).toBeNull()
    expect(screen.queryByText('Period')).not.toBeInTheDocument()

    await applyPreset('2025')
    await waitFor(() => {
      expect(within(group).getByRole('button', { name: /Change period/ }))
        .toHaveTextContent('01.01.2025 – 31.12.2025')
    })
  })

  it('keeps the built-in catalog in the popover instead of a second preset row', async () => {
    window.history.replaceState(null, '', '/dash')
    const calls: Array<string> = []
    render(<FiltersFixture fetcher={presetlessFetcher(calls)} />)

    // A document that declares no presets gets no preset row: the relative
    // catalog is the popover's, and the bar carries the trigger alone. The row
    // used to repeat the same seven presets the popover lists, in a different
    // grouping, in a tray too narrow to hold them.
    const trigger = await screen.findByRole('button', { name: /Change period/ })
    expect(screen.queryByRole('button', { name: 'Current month' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Last fiscal year' })).toBeNull()
    // The applied range is on the trigger whether or not it came from a preset.
    expect(trigger).toHaveTextContent('01.01.2026 – 22.07.2026')

    fireEvent.click(trigger)
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByRole('button', { name: '30 days' })).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: '12 months' })).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: 'Current fiscal year' })).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: 'Last month' })).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: 'Last fiscal year' })).toBeInTheDocument()

    fireEvent.click(within(dialog).getByRole('button', { name: 'Current month' }))
    // today = 2026-07-22 → this month resolves to 2026-07-01..2026-07-22.
    expect(window.location.search).toBe('?ActualRangeStart=2026-07-01&ActualRangeEnd=2026-07-22')
    await waitFor(() => {
      expect(calls.at(-1)).toBe('/lens/document?ActualRangeStart=2026-07-01&ActualRangeEnd=2026-07-22')
    })
  })

  it('surfaces the relative catalog in the popover alongside the declared years', async () => {
    window.history.replaceState(null, '', '/dash')
    const calls: Array<string> = []
    render(<FiltersFixture fetcher={periodFetcher(calls)} />)

    const dialog = await openPeriod()

    // The producer's own years lead the rail and the quick-range catalog fills
    // in what they do not cover.
    expect(within(dialog).getByRole('button', { name: '2025' })).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: '2026' })).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: 'Current month' })).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: '30 days' })).toBeInTheDocument()
    // "Last fiscal year" resolves to exactly the 2025 entry above it, so it is
    // dropped: two highlighted rows for one applied period give a reader no way
    // to tell what distinguishes them. Same range, one row — the producer's.
    expect(within(dialog).queryByRole('button', { name: 'Last fiscal year' })).toBeNull()
    // "Current fiscal year" survives here because this document's 2026 entry
    // runs to 31 December while the catalog's stops at today. The match is on
    // the resolved range, not on what a preset is called.
    expect(within(dialog).getByRole('button', { name: 'Current fiscal year' })).toBeInTheDocument()

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
    expect(within(dialog).getByText('Duration: 203 d.')).toBeInTheDocument()
  })

  it('ignores URL values the declaration cannot have produced', async () => {
    window.history.replaceState(null, '', '/dash?ActualRangeStart=garbage&ActualRangeEnd=2026-01-01')
    const calls: Array<string> = []
    render(<FiltersFixture fetcher={periodFetcher(calls)} />)
    await screen.findByRole('button', { name: /Change period/ })
    // The invalid pair is dropped: only the plain document fetch happened.
    expect(calls).toEqual(['/lens/document'])
  })
})

function facetDocument(): DashboardDocument {
  return parseDocument({
    ...fixture,
    filters: [{
      id: 'facet-product',
      kind: 'facet',
      label: 'Product',
      facet: {
        dimension: 'product',
        optionsEndpoint: '/lens/facets?_facet=product',
        selections: [{ label: 'OSAGO', removeUrl: '/report?_f=region%3Atashkent' }],
        clearUrl: '/report',
      },
    }, {
      id: 'facet-region',
      kind: 'facet',
      label: 'Region',
      facet: {
        dimension: 'region',
        optionsEndpoint: '/lens/facets?_facet=region',
        selections: [],
        clearUrl: '/report',
      },
    }],
    activeFilters: [
      { dimension: 'product', value: 'osago', label: 'OSAGO', removeUrl: '/report?_f=region%3Atashkent' },
    ],
    resetFiltersUrl: '/report',
  })
}

function optionsResponse(count: number): Response {
  return new Response(JSON.stringify({
    applyUrl: '/report?_f=region%3Atashkent',
    options: Array.from({ length: count }, (_, index) => ({
      label: `Option ${index + 1}`,
      value: `option-${index + 1}`,
      count: 10 - index,
      toggleUrl: '/report',
    })),
  }), { status: 200, headers: { 'Content-Type': 'application/json' } })
}

function FacetFixture() {
  return (
    <div className="lens-root" data-theme="light">
      <DocumentProvider initialDocument={facetDocument()}>
        <DashboardRuntimeProvider locale="en">
          <FilterBar today={{ year: 2026, month: 7, day: 22 }} />
        </DashboardRuntimeProvider>
      </DocumentProvider>
    </div>
  )
}

describe('facet filter menu', () => {
  it('restates only the dimensions the reader staged', () => {
    window.history.replaceState(null, '', '/report')
    const url = stagedFilterURL(
      '/report?page=2&_f=region%3Atashkent&_f=channel%3Aagent',
      new Map<string, ReadonlySet<string>>([
        ['region', new Set(['samarkand', 'bukhara'])],
        ['product', new Set(['osago'])],
      ]),
    )
    const applied = new URL(url, 'http://localhost/')
    // The untouched dimension keeps its value, the staged ones are replaced,
    // and every unrelated parameter survives.
    expect(applied.searchParams.getAll('_f')).toEqual([
      'channel:agent', 'region:samarkand', 'region:bukhara', 'product:osago',
    ])
    expect(applied.searchParams.get('page')).toBe('2')
  })

  it('collapses every facet behind one trigger and keeps the chips off the trigger row', async () => {
    window.history.replaceState(null, '', '/report')
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(optionsResponse(8))))
    const view = render(<FacetFixture />)

    const trigger = screen.getByRole('button', { name: /Filters/ })
    // One trigger, whatever the dashboard declares, and it counts what is on.
    expect(view.container.querySelectorAll('.lens-dashboard-scope button')).toHaveLength(1)
    expect(trigger).toHaveTextContent('1')
    // Applied selections are a row of their own, so applying one never reflows
    // the row of triggers above it.
    const chips = view.container.querySelector('.lens-filter-bar-chips')!
    expect(within(chips as HTMLElement).getByRole('link', { name: /Remove filter: OSAGO/ })).toBeInTheDocument()
    expect(within(chips as HTMLElement).getByRole('button', { name: 'Clear all' })).toBeInTheDocument()

    fireEvent.click(trigger)
    await screen.findByRole('checkbox', { name: /Option 1/ })
    const menu = screen.getByRole('dialog', { name: 'Filters' })
    // Placement is measured in viewport coordinates, so the menu must live in
    // the shared body portal. Keeping it under the trigger would add the
    // trigger row's offset a second time and push the card to the screen edge.
    expect(view.container).not.toContainElement(menu)
    expect(menu.parentElement).toHaveClass('lens-menu-overlay-root')
    // A staged selection is counted before it is applied, not after.
    expect(screen.getByText('Selected: 1')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('checkbox', { name: /Option 1/ }))
    fireEvent.click(screen.getByRole('checkbox', { name: /Option 2/ }))
    expect(screen.getByText('Selected: 2')).toBeInTheDocument()
    expect(window.location.search).toBe('')

    fireEvent.click(screen.getByRole('button', { name: 'Apply' }))
    expect(new URL(window.location.href).searchParams.getAll('_f')).toEqual([
      'region:tashkent', 'product:option-1', 'product:option-2',
    ])
  })

  it('drops the search box from a list too short to search', async () => {
    window.history.replaceState(null, '', '/report')
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(optionsResponse(3))))
    render(<FacetFixture />)

    fireEvent.click(screen.getByRole('button', { name: /Filters/ }))
    await screen.findByRole('checkbox', { name: /Option 1/ })
    expect(screen.queryByRole('searchbox')).toBeNull()

    cleanup()
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(optionsResponse(9))))
    render(<FacetFixture />)
    fireEvent.click(screen.getByRole('button', { name: /Filters/ }))
    await screen.findByRole('checkbox', { name: /Option 9/ })
    expect(screen.getByRole('searchbox')).toBeInTheDocument()
  })
})

/** Period + granularity, the shape /analytics/trends declares. */
function documentWithGranularity(grain: string): DashboardDocument {
  const period = documentWithPeriod({ start: '2022-01-01', end: '2026-07-22' }).filters![0]!
  return parseDocument({
    ...fixture,
    filters: [period, {
      id: 'grain',
      kind: 'segmented',
      label: 'Periodicity',
      segmented: {
        param: 'PeriodGrain',
        value: grain,
        options: [
          { value: 'year', label: 'By year' },
          { value: 'quarter', label: 'By quarter' },
        ],
      },
    }],
  })
}

function granularityFetcher(calls: Array<string>): typeof fetch {
  return (input: RequestInfo | URL) => {
    const raw = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    const url = new URL(raw, 'http://localhost/')
    calls.push(`${url.pathname}${url.search}`)
    // The server echoes its own normalization, as the document endpoint does.
    const grain = url.searchParams.get('PeriodGrain') === 'year' ? 'year' : 'quarter'
    return Promise.resolve(new Response(
      JSON.stringify(documentWithGranularity(grain)),
      { status: 200, headers: { 'Content-Type': 'application/json' } },
    ))
  }
}

describe('SegmentedFilterControl', () => {
  const segment = (name: string) => screen.getByRole('button', { name })

  it('writes the choice to the URL and refetches the document', async () => {
    window.history.replaceState(null, '', '/trends')
    const calls: Array<string> = []
    render(<FiltersFixture fetcher={granularityFetcher(calls)} />)

    await screen.findByRole('group', { name: 'Periodicity' })
    expect(segment('By quarter')).toHaveAttribute('aria-pressed', 'true')
    expect(segment('By year')).toHaveAttribute('aria-pressed', 'false')

    fireEvent.click(segment('By year'))

    await waitFor(() => expect(segment('By year')).toHaveAttribute('aria-pressed', 'true'))
    expect(new URL(window.location.href).searchParams.get('PeriodGrain')).toBe('year')
    // The document is re-requested through the same funnel as every other
    // filter, so the panels below are rebuilt by the server, not the browser.
    expect(calls).toEqual(['/lens/document', '/lens/document?PeriodGrain=year'])
  })

  it('restores a shared link, and ignores a value the declaration does not offer', async () => {
    window.history.replaceState(null, '', '/trends?PeriodGrain=year')
    render(<FiltersFixture fetcher={granularityFetcher([])} />)
    await screen.findByRole('group', { name: 'Periodicity' })
    expect(segment('By year')).toHaveAttribute('aria-pressed', 'true')

    cleanup()
    window.history.replaceState(null, '', '/trends?PeriodGrain=decade')
    render(<FiltersFixture fetcher={granularityFetcher([])} />)
    await screen.findByRole('group', { name: 'Periodicity' })
    expect(segment('By quarter')).toHaveAttribute('aria-pressed', 'true')
  })

  it('keeps the period beside it untouched', async () => {
    window.history.replaceState(null, '', '/trends?ActualRangeStart=2025-01-01&ActualRangeEnd=2025-12-31')
    render(<FiltersFixture fetcher={granularityFetcher([])} />)

    await screen.findByRole('group', { name: 'Periodicity' })
    fireEvent.click(segment('By year'))

    await waitFor(() => expect(segment('By year')).toHaveAttribute('aria-pressed', 'true'))
    const params = new URL(window.location.href).searchParams
    expect(params.get('ActualRangeStart')).toBe('2025-01-01')
    expect(params.get('ActualRangeEnd')).toBe('2025-12-31')
    expect(params.get('PeriodGrain')).toBe('year')
  })
})
