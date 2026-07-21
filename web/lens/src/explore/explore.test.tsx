import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import fixture from '../../fixtures/explore.json'
import { parseDocument, type Panel } from '../contract'
import {
  CascadePanel,
  TablePanel,
  useMarkSelection,
  usePanelChrome,
  type ChartPanelProps,
  type PanelRegistry,
  type StatPanelProps,
} from '../panels'
import { DashboardRuntimeProvider, DocumentProvider, navigationToURL, usePanelFrame } from '../runtime'
import { resolveLeafActionURL } from './actions'
import { ExplorePanel } from './ExplorePanel'
import { viewForSemantics } from './model'

const exploreDocument = parseDocument(fixture)
const panel = exploreDocument.panels[0]!
const singlePerspectiveDocument = parseDocument({
  ...fixture,
  drill: {
    ...fixture.drill,
    edges: {
      ...fixture.drill.edges,
      'profitability/operating-margin': {
        ...fixture.drill.edges['profitability/operating-margin'],
        perspectives: [{ id: 'profitability/operating-margin/composition' }],
      },
    },
  },
})

/**
 * Stands in for a chart: one activatable mark per frame row, reporting the
 * row's id exactly like the ECharts adapter does.
 */
function MarkProbe({ panel: current }: ChartPanelProps | StatPanelProps) {
  const frame = usePanelFrame(current.id)
  const onSelect = useMarkSelection()
  // PanelFrame renders the host's chrome in the card header; the probe mirrors
  // that contract so the trail and the explore affordance stay testable.
  const chrome = usePanelChrome()
  const idIndex = frame.data?.columns.findIndex((column) => column.name === current.encoding.id) ?? -1
  const labelIndex = frame.data?.columns.findIndex((column) => column.name === current.encoding.label) ?? -1
  return (
    <section aria-label={current.title} data-kind={current.kind}>
      {chrome?.trail}
      {chrome?.explore}
      {(frame.data?.rows ?? []).map((row, index) => (
        <button key={index} onClick={() => onSelect?.(String(row[idIndex]))} type="button">
          {typeof row[labelIndex] === 'string' ? row[labelIndex] : String(index)}
        </button>
      ))}
    </section>
  )
}

const registry: PanelRegistry = {
  pie: MarkProbe,
  donut: MarkProbe,
  bar: MarkProbe,
  hbar: MarkProbe,
  line: MarkProbe,
  area: MarkProbe,
  stat: MarkProbe,
  cascade: CascadePanel,
  table: TablePanel,
}

function renderExplore(currentPanel: Panel = panel, currentDocument = exploreDocument) {
  return render(
    <>
      <button type="button">Before explore</button>
      <div className="lens-root">
        <DocumentProvider initialDocument={currentDocument}>
          <DashboardRuntimeProvider locale="en">
            <ExplorePanel panel={currentPanel} registry={registry} />
          </DashboardRuntimeProvider>
        </DocumentProvider>
      </div>
      <button type="button">After explore</button>
    </>,
  )
}

function overlay(): HTMLElement {
  return screen.getByRole('dialog')
}

afterEach(() => {
  cleanup()
  window.history.replaceState(null, '', '/')
  Object.defineProperty(document, 'startViewTransition', { configurable: true, value: undefined })
  Object.defineProperty(window, 'matchMedia', { configurable: true, value: undefined })
  vi.restoreAllMocks()
})

describe('explore semantics', () => {
  it('maps each semantic shape to its supported view', () => {
    expect(viewForSemantics('partition', 'pie')).toBe('pie')
    expect(viewForSemantics('partition', 'line')).toBe('donut')
    expect(viewForSemantics('reconciliation', 'pie')).toBe('cascade')
    expect(viewForSemantics('series', 'bar')).toBe('line')
    expect(viewForSemantics('evidence', 'line')).toBe('table')
  })
})

describe('explore panel at rest', () => {
  it('shows only the chart plus a header affordance — no strips, chips or breadcrumb row', () => {
    const { container } = renderExplore()

    expect(screen.queryByRole('dialog')).toBeNull()
    expect(container.querySelector('.lens-segment-tree')).toBeNull()
    expect(container.querySelector('.lens-perspective-set')).toBeNull()
    expect(container.querySelector('.lens-explore-path')).toBeNull()
    expect(screen.getByRole('button', { name: 'Show breakdown' })).toBeInTheDocument()
  })

  it('opens the level breakdown from the header affordance', () => {
    renderExplore()
    fireEvent.click(screen.getByRole('button', { name: 'Show breakdown' }))

    const rows = within(overlay()).getAllByRole('button', { name: /Operating margin/ })
    expect(rows[0]).toHaveTextContent('$1,840,000')
    expect(rows[0]).toHaveTextContent('100.0%')
  })
})

describe('level data integrity', () => {
  it('never shows the parent level\'s numbers under a child level\'s title', () => {
    // Entering the fork without picking a perspective: the level owns no data,
    // so the panel must ask for a view instead of keeping the root's rows.
    renderExplore()
    fireEvent.click(screen.getByRole('button', { name: 'Operating margin' }))
    fireEvent.click(within(overlay()).getByRole('button', { name: /Expand segment/ }))

    const trail = screen.getByRole('navigation', { name: /exploration path/ })
    expect(within(trail).getByRole('button', { name: /Operating margin/ })).toBeInTheDocument()
    // The root frame's only row was «Operating margin» at $1,840,000; no mark
    // from it may survive into this level — the only element still carrying
    // that name is the trail crumb.
    expect(screen.getAllByRole('button', { name: /Operating margin/ })).toHaveLength(1)
    expect(document.querySelector('section[data-kind]')).toBeNull()
    expect(screen.getByText(/Choose a view/)).toBeInTheDocument()
  })

  it('keeps showing a level that does own a frame', async () => {
    renderExplore()
    fireEvent.click(screen.getByRole('button', { name: 'Operating margin' }))
    fireEvent.click(within(overlay()).getByRole('option', { name: 'Composition' }))

    await waitFor(() => expect(screen.getByRole('button', { name: 'Services' })).toBeInTheDocument())
    expect(screen.queryByText(/Choose a view/)).toBeNull()
  })
})

describe('drill overlay', () => {
  function openMarkOverlay() {
    renderExplore()
    fireEvent.click(screen.getByRole('button', { name: 'Operating margin' }))
    return overlay()
  }

  it('describes the activated mark with its value, share and perspectives', () => {
    const dialog = openMarkOverlay()

    expect(dialog).toHaveAttribute('aria-label', 'Operating margin')
    expect(dialog).toHaveTextContent('$1,840,000')
    expect(dialog).toHaveTextContent('100.0%')
    const options = within(dialog).getAllByRole('option')
    expect(options.map((option) => option.textContent)).toEqual(['Composition', 'Trend', 'Bridge', 'Evidence'])
    expect(within(dialog).getByRole('button', { name: /Expand segment/ })).toBeInTheDocument()
  })

  it('renders in a portal at the end of body so no panel stacking context can bury it', () => {
    const { container } = renderExplore()
    fireEvent.click(screen.getByRole('button', { name: 'Operating margin' }))

    const dialog = overlay()
    expect(container.contains(dialog)).toBe(false)
    expect(dialog.closest('.lens-root')).toHaveClass('lens-overlay-root')
    expect(document.body.lastElementChild).toContainElement(dialog)
  })

  it('expands the segment into the panel and moves the path into the header trail', async () => {
    openMarkOverlay()
    fireEvent.click(within(overlay()).getByRole('option', { name: 'Composition' }))

    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    const trail = screen.getByRole('navigation', { name: /exploration path/ })
    expect(within(trail).getByRole('button', { name: /Composition root/ })).toHaveAttribute('aria-current', 'page')
    expect(new URL(window.location.href).searchParams.get('perspective'))
      .toBe('profitability/operating-margin/composition')
  })

  it('drills a breakdown row one level down', async () => {
    const path = [
      'profitability',
      'profitability/operating-margin',
      'profitability/operating-margin/composition',
      'profitability/operating-margin/composition/root',
    ]
    window.history.replaceState(null, '', navigationToURL(
      { path, perspectiveId: 'profitability/operating-margin/composition' }, new URL(window.location.href),
    ))
    renderExplore()

    fireEvent.click(screen.getByRole('button', { name: 'Services' }))
    const dialog = overlay()
    // The breakdown lists what the segment expands into, with values from the
    // target level's own frame.
    expect(within(dialog).getByRole('button', { name: /Sales/ })).toHaveTextContent('$730,000')

    // Landing on a breakdown row enters the mark and then the row, so the view
    // is the level that row itself expands to.
    fireEvent.click(within(dialog).getByRole('button', { name: /Sales/ }))
    await waitFor(() => {
      expect(new URL(window.location.href).searchParams.getAll('path').at(-1))
        .toBe('profitability/operating-margin/composition/transactions')
    })
  })

  it('closes on Escape, on an outside press, and restores focus to the affordance', () => {
    renderExplore()
    const affordance = screen.getByRole('button', { name: 'Show breakdown' })
    fireEvent.click(affordance)
    fireEvent.keyDown(document, { key: 'Escape' })

    expect(screen.queryByRole('dialog')).toBeNull()
    expect(affordance).toHaveFocus()

    fireEvent.click(affordance)
    fireEvent.mouseDown(document.querySelector('.lens-drill-scrim')!)
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('offers the leaf link a segment carries', async () => {
    const path = [
      'profitability',
      'profitability/operating-margin',
      'profitability/operating-margin/composition',
      'profitability/operating-margin/composition/transactions',
    ]
    window.history.replaceState(null, '', navigationToURL(
      { path, perspectiveId: 'profitability/operating-margin/composition' }, new URL(window.location.href),
    ))
    renderExplore()

    fireEvent.click(await screen.findByRole('button', { name: 'Invoice TX-1042' }))
    const link = within(overlay()).getByRole('link', { name: /Open record/ })
    expect(link).toHaveAttribute('href', expect.stringContaining('/transactions/TX-1042'))
  })

  it('says so when a segment has no further detail', async () => {
    const path = [
      'profitability',
      'profitability/operating-margin',
      'profitability/operating-margin/composition',
      'profitability/operating-margin/composition/transactions',
    ]
    window.history.replaceState(null, '', navigationToURL(
      { path, perspectiveId: 'profitability/operating-margin/composition' }, new URL(window.location.href),
    ))
    renderExplore()

    fireEvent.click(await screen.findByRole('button', { name: 'Invoice TX-1098' }))
    expect(within(overlay()).queryByRole('button', { name: /Expand segment/ })).toBeNull()
  })
})

describe('header trail', () => {
  it('jumps back to an earlier level and keeps Escape as one level up', async () => {
    const path = [
      'profitability',
      'profitability/operating-margin',
      'profitability/operating-margin/composition',
      'profitability/operating-margin/composition/cost-centers',
    ]
    window.history.replaceState(null, '', navigationToURL(
      { path, perspectiveId: 'profitability/operating-margin/composition' }, new URL(window.location.href),
    ))
    renderExplore()

    // The header carries the back button and the current level only; the full
    // path lives in the overlay that level opens.
    const trail = screen.getByRole('navigation', { name: /exploration path/ })
    const current = within(trail).getByRole('button', { name: /Cost centers/ })
    expect(within(trail).queryByRole('button', { name: /Profitability/ })).toBeNull()
    fireEvent.click(current)

    // The perspective segment carries no level of its own, so it contributes no
    // step: the path is Profitability › Operating margin › Cost centers.
    const steps = within(overlay()).getAllByRole('button', { name: /Profitability|Operating margin|Cost centers/ })
    expect(steps.map((step) => step.textContent)).toEqual(['Profitability', 'Operating margin', 'Cost centers'])
    fireEvent.click(steps[1]!)
    await waitFor(() => {
      expect(new URL(window.location.href).searchParams.getAll('path')).toEqual(path.slice(0, 2))
    })
  })
})

describe('explore navigation', () => {
  it('replaces a single-perspective auto-switch so browser Back reaches the parent', async () => {
    const pushState = vi.spyOn(window.history, 'pushState')
    renderExplore(singlePerspectiveDocument.panels[0], singlePerspectiveDocument)
    const parentURL = window.location.href
    const parentState: unknown = window.history.state

    fireEvent.click(screen.getByRole('button', { name: 'Operating margin' }))
    fireEvent.click(within(overlay()).getByRole('button', { name: /Expand segment/ }))
    await waitFor(() => {
      expect(new URL(window.location.href).searchParams.get('perspective'))
        .toBe('profitability/operating-margin/composition')
    })
    expect(pushState).toHaveBeenCalledTimes(1)

    window.history.replaceState(parentState, '', parentURL)
    window.dispatchEvent(new PopStateEvent('popstate', { state: parentState }))

    await waitFor(() => expect(screen.getByRole('button', { name: 'Operating margin' })).toBeInTheDocument())
    expect(new URL(window.location.href).searchParams.has('perspective')).toBe(false)
  })

  it('skips View Transitions when reduced motion is requested', () => {
    const startViewTransition = vi.fn((update: () => void) => {
      update()
      return { ready: Promise.resolve(), finished: Promise.resolve() }
    })
    Object.defineProperty(document, 'startViewTransition', { configurable: true, value: startViewTransition })
    Object.defineProperty(window, 'matchMedia', { configurable: true, value: vi.fn(() => ({ matches: true })) })
    renderExplore()

    fireEvent.click(screen.getByRole('button', { name: 'Operating margin' }))
    fireEvent.click(within(overlay()).getByRole('button', { name: /Expand segment/ }))

    expect(startViewTransition).not.toHaveBeenCalled()
  })

  it('renders evidence with the registered table view and resolved full-page leaf links', async () => {
    const path = [
      'profitability',
      'profitability/operating-margin',
      'profitability/operating-margin/evidence',
      'profitability/operating-margin/evidence/root',
    ]
    const url = navigationToURL({ path, perspectiveId: 'profitability/operating-margin/evidence' }, new URL(window.location.href))
    window.history.replaceState(null, '', url)
    renderExplore()

    expect(await screen.findByRole('region', { name: 'Source transactions' })).toHaveAttribute('data-panel-kind', 'table')
    const links = screen.getAllByRole('link', { name: 'Open record' })
    expect(links).toHaveLength(2)
    expect(links[0]).toHaveAttribute('href', expect.stringContaining('/transactions/TX-1042'))
    expect(screen.getByText('$284,000')).toBeInTheDocument()
  })

  it('does not attach a row action when the ID does not match a declared child', async () => {
    const evidence = fixture.drill.edges['profitability/operating-margin/evidence/root']
    const mismatchedDocument = parseDocument({
      ...fixture,
      drill: {
        ...fixture.drill,
        edges: {
          ...fixture.drill.edges,
          'profitability/operating-margin/evidence/root': {
            ...evidence,
            children: [{
              key: 'profitability/operating-margin/evidence/root/not-in-frame',
              path: [...evidence.path, 'profitability/operating-margin/evidence/root/not-in-frame'],
              label: 'Not in frame',
              action: panel.actions[0]!,
            }],
          },
        },
      },
    })
    const url = navigationToURL({
      path: evidence.path,
      perspectiveId: 'profitability/operating-margin/evidence',
    }, new URL(window.location.href))
    window.history.replaceState(null, '', url)
    renderExplore(mismatchedDocument.panels[0], mismatchedDocument)

    expect(await screen.findByRole('region', { name: 'Source transactions' })).toHaveAttribute('data-panel-kind', 'table')
    expect(screen.queryByRole('link', { name: 'Open record' })).not.toBeInTheDocument()
  })
})

describe('leaf actions', () => {
  it('resolves field and variable parameters while preserving the host query', () => {
    const href = resolveLeafActionURL({
      kind: 'navigate_to_leaf',
      urlTemplate: '/transactions/{id}?mode=detail',
      params: [{ name: 'id', source: { kind: 'field', name: 'transactionId' } }],
      payload: {},
      preserveQuery: true,
    }, {
      fields: { transactionId: 'TX 1042' },
      variables: { region: 'north' },
      location: new URL('https://example.test/dashboard?region=north'),
    })

    expect(href).toBe('https://example.test/transactions/TX%201042?mode=detail&region=north')
  })
})
