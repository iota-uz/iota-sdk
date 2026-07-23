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
// A drill level that shares its panel's frame shape declares no encoding of its
// own; the chart reads the panel's. This mirrors the real profitability pie,
// where the overlay used to open with no value, share or total because the
// model read the (absent) level encoding instead of the panel's.
const levelWithoutEncodingFixture = JSON.parse(JSON.stringify(fixture)) as { drill: { edges: Record<string, { encoding?: unknown }> } }
delete levelWithoutEncodingFixture.drill.edges['profitability']!.encoding
const levelWithoutEncodingDocument = parseDocument(levelWithoutEncodingFixture)

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

const dynamicDocumentFixture = structuredClone(exploreDocument)
const dynamicRoot = dynamicDocumentFixture.drill.edges['profitability/operating-margin/composition/root']!
dynamicRoot.dynamicChildren = {
  key: { kind: 'field', name: 'id' }, label: { kind: 'field', name: 'label' },
  target: { kind: 'literal', value: 'profitability/operating-margin/composition/cost-centers' },
}
dynamicRoot.children = []
dynamicDocumentFixture.frames['composition:root']!.children = exploreDocument.drill.edges['profitability/operating-margin/composition/root']!.children
const dynamicLeaf = dynamicDocumentFixture.drill.edges['profitability/operating-margin/composition/transactions']!
dynamicLeaf.dynamicChildren = {
  key: { kind: 'field', name: 'id' }, label: { kind: 'field', name: 'label' },
  action: {
    kind: 'navigate_to_leaf', urlSource: { kind: 'field', name: 'url' }, params: [], payload: {},
  },
}
dynamicLeaf.children = []
dynamicDocumentFixture.frames['composition:leaf']!.columns.push({ name: 'url', type: 'string' })
dynamicDocumentFixture.frames['composition:leaf']!.rows[0]!.push('/transactions/TX-1042')
dynamicDocumentFixture.frames['composition:leaf']!.rows[1]!.push('/transactions/TX-1098')
dynamicDocumentFixture.frames['composition:leaf']!.children = exploreDocument.drill.edges['profitability/operating-margin/composition/transactions']!.children.map(
  (node) => {
    const key = node.key.split('/').at(-1)!
    return { key, path: [...dynamicLeaf.path, key], label: node.label, action: dynamicLeaf.dynamicChildren!.action }
  },
)
const dynamicDocument = parseDocument(dynamicDocumentFixture)

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
    // Standing on the fork without a perspective: the level owns no data, so
    // the panel must ask for a view instead of keeping the root's rows. The
    // overlay no longer offers a way in — a fork with several perspectives is
    // entered by choosing one — but a stored URL still addresses it, and that
    // is exactly when showing the parent's numbers would be a lie.
    window.history.replaceState(null, '', navigationToURL(
      { path: ['profitability', 'profitability/operating-margin'] }, new URL(window.location.href),
    ))
    renderExplore()

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
  })

  it('populates the mark stats through the builder even when the level declares no encoding', () => {
    // The real regression: a fork/choosable mark on a level with no encoding of
    // its own. The chart draws it from the panel encoding, so the overlay must
    // read value, share and the copy affordance from the same fallback rather
    // than degrading to a bare header.
    renderExplore(levelWithoutEncodingDocument.panels[0], levelWithoutEncodingDocument)
    fireEvent.click(screen.getByRole('button', { name: 'Operating margin' }))

    const dialog = overlay()
    expect(dialog).toHaveTextContent('$1,840,000')
    expect(dialog).toHaveTextContent('100.0%')
    expect(within(dialog).getByRole('button', { name: 'Copy value' })).toBeInTheDocument()
    // It is still the fork case, so the expansion stays suppressed for it.
    expect(within(dialog).getAllByRole('option')).toHaveLength(4)
  })

  it('offers the perspectives or the expansion, never both for the same choice', () => {
    // What this segment expands to is a fork whose only content is the four
    // views listed above. Offering «Expand segment» as well would put the same
    // question in two places and land whoever takes the second one on a card
    // that asks it again.
    const dialog = openMarkOverlay()

    expect(within(dialog).getAllByRole('option')).toHaveLength(4)
    expect(within(dialog).queryByRole('button', { name: /Expand segment/ })).toBeNull()
    expect(within(dialog).queryByText(/No further detail/)).toBeNull()
  })

  it('keeps the expansion when it leads somewhere the overlay does not', async () => {
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

    fireEvent.click(await screen.findByRole('button', { name: 'Services' }))
    expect(within(overlay()).getByRole('button', { name: /Expand segment/ })).toBeInTheDocument()
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

  it('spends one history entry on picking a view for a segment', async () => {
    // Entering the segment and resolving its fork are one user action. Charging
    // two history entries for it made Back land on the fork — a card that only
    // asks which view to use — instead of the chart the segment came from.
    const pushState = vi.spyOn(window.history, 'pushState')
    openMarkOverlay()
    fireEvent.click(within(overlay()).getByRole('option', { name: 'Composition' }))

    await waitFor(() => expect(screen.getByRole('button', { name: 'Services' })).toBeInTheDocument())
    expect(pushState).toHaveBeenCalledTimes(1)

    fireEvent.click(screen.getByRole('button', { name: 'Back' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Operating margin' })).toBeInTheDocument())
    expect(screen.queryByText(/Choose a view/)).toBeNull()
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
    // is the level that row itself expands to — and the path records both
    // selections rather than collapsing onto the target node's ancestry.
    fireEvent.click(within(dialog).getByRole('button', { name: /Sales/ }))
    await waitFor(() => {
      expect(new URL(window.location.href).searchParams.getAll('path')).toEqual([
        ...path,
        'profitability/operating-margin/composition/root/services',
        'profitability/operating-margin/composition/cost-centers/sales',
      ])
    })
  })

  it('expands a concrete point and keeps the selection in the path', async () => {
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

    fireEvent.click(await screen.findByRole('button', { name: 'Services' }))
    fireEvent.click(within(overlay()).getByRole('button', { name: /Expand segment/ }))

    // The path ends at the point that was entered, not at the target node's
    // canonical ancestry: the level it opens is parameterised by that point.
    await waitFor(() => {
      expect(new URL(window.location.href).searchParams.getAll('path')).toEqual([
        ...path,
        'profitability/operating-margin/composition/root/services',
      ])
    })
    expect(screen.getByRole('button', { name: 'Sales' })).toBeInTheDocument()
  })

  it('fetches sibling point drills as distinct levels', async () => {
    // Same target node, two different points: each drill must issue its own
    // query with the point interleaved into the wire path, and must not replay
    // the sibling's cached frame.
    const lazyFixture = JSON.parse(JSON.stringify(fixture)) as {
      endpoints: Record<string, string>
      drill: { edges: Record<string, { frame?: string }> }
    }
    delete lazyFixture.drill.edges['profitability/operating-margin/composition/cost-centers']!.frame
    lazyFixture.endpoints = { query: '/lens/query' }
    const lazyDocument = parseDocument(lazyFixture)

    const paths: Array<Array<string>> = []
    const fetcher = vi.fn<typeof fetch>().mockImplementation((_input, init) => {
      const request = JSON.parse(typeof init?.body === 'string' ? init.body : '{}') as { path: Array<string> }
      paths.push(request.path)
      const services = request.path.includes('profitability/operating-margin/composition/root/services')
      return Promise.resolve(new Response(JSON.stringify({
        frames: {
          level: {
            columns: [
              { name: 'id', type: 'string' },
              { name: 'label', type: 'string' },
              { name: 'value', type: 'number' },
            ],
            rows: [services ? ['sales', 'Sales', 730000] : ['ops', 'Ops', 410000]],
          },
        },
      }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
    })

    const rootPath = [
      'profitability',
      'profitability/operating-margin',
      'profitability/operating-margin/composition',
      'profitability/operating-margin/composition/root',
    ]
    window.history.replaceState(null, '', navigationToURL(
      { path: rootPath, perspectiveId: 'profitability/operating-margin/composition' }, new URL(window.location.href),
    ))
    render(
      <div className="lens-root">
        <DocumentProvider initialDocument={lazyDocument} fetcher={fetcher}>
          <DashboardRuntimeProvider fetcher={fetcher} locale="en">
            <ExplorePanel panel={lazyDocument.panels[0]!} registry={registry} />
          </DashboardRuntimeProvider>
        </DocumentProvider>
      </div>,
    )

    fireEvent.click(await screen.findByRole('button', { name: 'Services' }))
    fireEvent.click(within(overlay()).getByRole('button', { name: /Expand segment/ }))
    expect(await screen.findByRole('button', { name: 'Sales' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Back' }))
    fireEvent.click(await screen.findByRole('button', { name: 'Products' }))
    fireEvent.click(within(overlay()).getByRole('button', { name: /Expand segment/ }))
    expect(await screen.findByRole('button', { name: 'Ops' })).toBeInTheDocument()

    // The wire paths interleave each point with the node it selects into.
    expect(paths).toEqual([
      [...rootPath, 'profitability/operating-margin/composition/root/services', 'profitability/operating-margin/composition/cost-centers'],
      [...rootPath, 'profitability/operating-margin/composition/root/products', 'profitability/operating-margin/composition/cost-centers'],
    ])
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

  it('keeps the overlay open while its own body scrolls but dismisses on a page scroll', () => {
    renderExplore()
    fireEvent.click(screen.getByRole('button', { name: 'Show breakdown' }))
    expect(screen.queryByRole('dialog')).not.toBeNull()

    // Scrolling inside the overlay (a long breakdown/structure list) must not
    // close it, or the list is unreachable and the segment can never expand.
    fireEvent.scroll(overlay())
    expect(screen.queryByRole('dialog')).not.toBeNull()

    // Scrolling the page underneath moves the anchor the popover is pinned to,
    // so it dismisses.
    fireEvent.scroll(document)
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

describe('dynamic children', () => {
  it('descends when a frame-resolved child mark is clicked', async () => {
    const rootPath = dynamicRoot.path
    window.history.replaceState(null, '', navigationToURL(
      { panelId: panel.id, path: rootPath, perspectiveId: 'profitability/operating-margin/composition' },
      new URL(window.location.href),
    ))
    renderExplore(dynamicDocument.panels[0], dynamicDocument)

    fireEvent.click(await screen.findByRole('button', { name: 'Services' }))
    fireEvent.click(within(overlay()).getByRole('button', { name: /Expand segment/ }))
    await waitFor(() => expect(screen.getByRole('region', { name: 'Cost centers' })).toBeInTheDocument())
  })

  it('resolves a frame child leaf action URL from its row', async () => {
    const path = dynamicLeaf.path
    window.history.replaceState(null, '', navigationToURL(
      { panelId: panel.id, path, perspectiveId: 'profitability/operating-margin/composition' },
      new URL(window.location.href),
    ))
    renderExplore(dynamicDocument.panels[0], dynamicDocument)

    fireEvent.click(await screen.findByRole('button', { name: 'Invoice TX-1042' }))
    expect(within(overlay()).getByRole('link', { name: /Open record/ })).toHaveAttribute(
      'href', expect.stringContaining('/transactions/TX-1042'),
    )
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
    fireEvent.click(within(overlay()).getByRole('option', { name: 'Composition' }))

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

  it('treats an empty field URL as inert instead of resolving it to the current page', () => {
    // An inert segment (e.g. the aggregate «Ceded» slice) carries an empty
    // action_url. Without the guard `new URL('', location)` would resolve to the
    // dashboard page, so an OpenDrawer would open the page itself as a document.
    const href = resolveLeafActionURL({
      kind: 'open_drawer',
      urlSource: { kind: 'field', name: 'action_url' },
      params: [],
      payload: {},
    }, {
      fields: { action_url: '' },
      variables: {},
      location: new URL('https://example.test/analytics/profitability?ActualRangeStart=2026-01-01'),
    })

    expect(href).toBeUndefined()
  })
})
