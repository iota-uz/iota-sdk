import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library'
import { afterEach, describe, expect, it, vi } from 'vitest'
import fixtures from '../../fixtures/scoped-filters.json'
import { LensDashboard } from '../LensDashboard'
import { parseDocument } from '../contract'
import type { ContextEditor, EffectiveContext } from '../contract/types'
import { calendarToday, contextDiffersFromDashboard, localOverrides } from './ScopedFilterControls'
import { decodeFilterBoard, filterContextParam } from '../reports/filterDraft'

afterEach(() => {
  cleanup()
  document.body.replaceChildren()
})

const scenario = (name: string) => {
  const fixture = fixtures.find((candidate) => candidate.name === name)
  if (!fixture) throw new Error(`missing scenario ${name}`)
  return structuredClone(fixture.document)
}
const section = (label: string) => screen.getAllByRole('region', { name: label })[0]!
const mount = (document: ReturnType<typeof scenario>) =>
  render(() => <LensDashboard initialDocument={parseDocument(document)} fetcher={() => new Promise<Response>(() => undefined)} />)

describe('group filter context', () => {
  // False green: a dashboard-only document would never exercise the per-group editors this collapses.
  it('collapses the section and card editors behind one trigger that counts local overrides', () => {
    mount(scenario('June card'))
    const revenue = section('Revenue by channel')
    expect(within(revenue).queryByTestId('context-controls-card-revenue')).toBeNull()
    expect(within(revenue).queryByTestId('context-controls-section-sales')).toBeNull()
    const trigger = within(revenue).getByTestId('context-local-revenue')
    expect(trigger).toHaveAttribute('aria-expanded', 'false')
    expect(trigger).toHaveTextContent('Filters1')
    expect(within(revenue).getByText('01.06.2026 – 30.06.2026', { selector: '.lens-active-filter-chip > span' })).toBeInTheDocument()
    expect(within(section('All orders')).getByTestId('context-local-orders')).not.toHaveTextContent(/\d/)

    fireEvent.click(trigger)
    expect(trigger).toHaveAttribute('aria-expanded', 'true')
    const panel = within(revenue).getByTestId('context-panel-revenue')
    expect(trigger).toHaveAttribute('aria-controls', panel.id)
    expect(within(panel).getAllByTestId('context-controls-section-sales')).toHaveLength(1)
    expect(within(panel).getAllByTestId('context-controls-card-revenue')).toHaveLength(1)
    expect(within(panel).getByText('Whole section: sales')).toBeInTheDocument()
    expect(within(panel).getByText('This card')).toBeInTheDocument()

    fireEvent.keyDown(within(panel).getByTestId('context-inherit'), { key: 'Escape' })
    expect(within(revenue).queryByTestId('context-panel-revenue')).toBeNull()
    expect(trigger).toHaveAttribute('aria-expanded', 'false')
    expect(document.activeElement).toBe(trigger)
  })

  // False green: asserting only the hidden case would pass if the line were removed everywhere.
  it('prints the effective context only where it says more than the dashboard bar', () => {
    mount(scenario('June card'))
    expect(within(section('All orders')).queryByTestId('effective-context')).toBeNull()
    expect(within(section('Revenue by channel')).getByTestId('effective-context')).toHaveTextContent('2026-06-01 – 2026-06-30')
  })

  // False green: hiding the line on identical periods must not swallow conflict and unsupported notices.
  it('keeps conflict and unsupported notices visible', () => {
    mount(scenario('Conflict and unsupported'))
    expect(within(section('Revenue by channel')).getByTestId('context-conflict')).toBeInTheDocument()
    expect(within(section('All orders')).getByTestId('context-unsupported')).toBeInTheDocument()
  })

  it('offers no trigger on a read-only editor', () => {
    const document = scenario('June card')
    ;(document.contextEditor as { readOnly?: boolean }).readOnly = true
    mount(document)
    expect(screen.queryByTestId('context-local-revenue')).toBeNull()
    expect(within(section('Revenue by channel')).getByText('01.06.2026 – 30.06.2026', { selector: '.lens-active-filter-chip > span' })).toBeInTheDocument()
  })
})

describe('dashboard period', () => {
  // False green: checking the trigger label alone would pass while the board still awaits a separate Apply (EAI-UZ/tasks#555).
  it('applies the year-to-date preset to the document request without a further click', async () => {
    const requests: string[] = []
    const fetcher = vi.fn((input: RequestInfo | URL) => {
      requests.push(typeof input === 'string' ? input : input instanceof URL ? input.href : input.url)
      return new Promise<Response>(() => undefined)
    })
    render(() => <LensDashboard src="/scoped-document" initialDocument={parseDocument(scenario('June card'))} fetcher={fetcher as typeof fetch} />)
    const controls = screen.getByTestId('context-controls-dashboard-dashboard')
    fireEvent.click(controls.querySelector<HTMLButtonElement>('.lens-filter-trigger')!)
    fireEvent.click(await screen.findByRole('button', { name: /Current fiscal year/ }))
    await waitFor(() => expect(requests.some((url) => url.includes(filterContextParam))).toBe(true))
    const url = new URL(requests.find((candidate) => candidate.includes(filterContextParam))!, 'http://localhost')
    const board = decodeFilterBoard(url.searchParams.get(filterContextParam)!)
    expect(board.dashboard.period).toEqual({ mode: 'fixed', start: '2026-01-01', end: '2026-07-22' })
    expect(board.cards?.revenue).toEqual({ period: { mode: 'fixed', start: '2026-06-01', end: '2026-06-30' } })
    expect(screen.queryByTestId('context-apply')).toBeNull()
  })
})

describe('contextDiffersFromDashboard', () => {
  const base: EffectiveContext = {
    identity: 'x',
    calendar: { timezone: 'UTC', asOf: '2026-07-01T00:00:00Z', defaultPeriod: { mode: 'all' } },
    period: { selection: { mode: 'fixed' }, origin: { scope: 'dashboard', id: 'd' }, start: '2026-01-01', end: '2026-12-31' },
    restrictions: [], intersection: [], conflicts: [], unsupported: [],
    input: { calendar: { timezone: 'UTC', asOf: '2026-07-01T00:00:00Z', defaultPeriod: { mode: 'all' } }, layers: [], supportedDimensions: [] },
  }
  // False green: testing a single positive case would not catch a predicate that ignores one of the triggers.
  it.each([
    ['identical', base, false],
    ['card period', { ...base, period: { ...base.period, origin: { scope: 'card', id: 'c' } } }, true],
    ['report period', { ...base, period: { ...base.period, origin: { scope: 'report', id: 'r' } } }, true],
    ['section restriction', { ...base, restrictions: [{ dimension: 'p', values: ['a'], origin: { scope: 'section', id: 's' } }] }, true],
    ['card restriction', { ...base, restrictions: [{ dimension: 'p', values: ['a'], origin: { scope: 'card', id: 'c' } }] }, true],
    ['dashboard restriction only', { ...base, restrictions: [{ dimension: 'p', values: ['a'], origin: { scope: 'dashboard', id: 'd' } }] }, false],
    ['conflict', { ...base, conflicts: ['p'] }, true],
    ['unsupported', { ...base, unsupported: [{ dimension: 'p', values: ['a'], origin: { scope: 'dashboard', id: 'd' } }] }, true],
    ['unsupported export', { ...base, unsupportedOperations: { export: [{ dimension: 'p', values: ['a'], origin: { scope: 'dashboard', id: 'd' } }] } }, true],
    ['empty unsupported operations', { ...base, unsupportedOperations: { export: [] } }, false],
  ] as const)('%s', (_name, context, expected) => {
    expect(contextDiffersFromDashboard(context as EffectiveContext)).toBe(expected)
  })
})

describe('localOverrides', () => {
  // False green: a section period resolved through the effective context only works while no card overrides it.
  it('dates a relative section period on the report calendar beneath a card override, and names its scope', () => {
    const document = scenario('June card')
    const editor = document.contextEditor as ContextEditor
    editor.value.sections = { sales: { period: { mode: 'relative', days: 30 }, constraints: [{ dimension: 'product', values: ['A'] }] } }
    const group = document.layout.rows[0]!.panels[0]!.groups.find((candidate) => candidate.id === 'revenue')!
    const context = group.filterContext!
    expect(context.period.origin).toEqual({ scope: 'card', id: 'revenue' })
    const labels = localOverrides(editor, context, 'revenue', (_key, fallback) => fallback).map((override) => override.label)
    expect(labels).toEqual(['Section · 23.06.2026 – 22.07.2026', 'Section · Product: A', '01.06.2026 – 30.06.2026'])
  })
})

describe('calendarToday', () => {
  it('reads the calendar date in the report timezone', () => {
    expect(calendarToday({ timezone: 'Asia/Tashkent', asOf: '2026-07-22T20:30:00Z', defaultPeriod: { mode: 'all' } })).toEqual({ year: 2026, month: 7, day: 23 })
    expect(calendarToday({ timezone: 'America/New_York', asOf: '2026-07-22T02:30:00Z', defaultPeriod: { mode: 'all' } })).toEqual({ year: 2026, month: 7, day: 21 })
  })
})
