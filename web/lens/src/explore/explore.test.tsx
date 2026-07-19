import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import fixture from '../../fixtures/explore.json'
import { parseDocument, type Panel } from '../contract'
import type { ChartPanelProps, PanelRegistry, StatPanelProps } from '../panels'
import { DashboardRuntimeProvider, DocumentProvider, navigationToURL } from '../runtime'
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

function ProbePanel({ panel: current }: ChartPanelProps | StatPanelProps) {
  return <section aria-label={current.title} data-kind={current.kind}>{current.title}</section>
}

const registry: PanelRegistry = {
  pie: ProbePanel,
  donut: ProbePanel,
  bar: ProbePanel,
  hbar: ProbePanel,
  line: ProbePanel,
  area: ProbePanel,
  stat: ProbePanel,
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

function tabStops(): Array<HTMLElement> {
  return Array.from(document.querySelectorAll<HTMLElement>('a[href], button, [tabindex]'))
    .filter((element) => element.tabIndex >= 0 && !element.hasAttribute('disabled'))
}

function pressTab(): void {
  const current = document.activeElement as HTMLElement | null
  if (current) fireEvent.keyDown(current, { key: 'Tab' })
  const stops = tabStops()
  const currentIndex = current ? stops.indexOf(current) : -1
  stops[(currentIndex + 1) % stops.length]?.focus()
}

afterEach(() => {
  cleanup()
  window.history.replaceState(null, '', '/')
  Object.defineProperty(document, 'startViewTransition', { configurable: true, value: undefined })
  Object.defineProperty(window, 'matchMedia', { configurable: true, value: undefined })
  vi.restoreAllMocks()
})

describe('explore semantics', () => {
  it('maps each semantic shape to an honest supported or interim view', () => {
    expect(viewForSemantics('partition', 'pie')).toBe('pie')
    expect(viewForSemantics('partition', 'line')).toBe('donut')
    expect(viewForSemantics('reconciliation', 'pie')).toBe('cascade')
    expect(viewForSemantics('series', 'bar')).toBe('line')
    expect(viewForSemantics('evidence', 'line')).toBe('table')
  })
})

describe('ExplorePanel', () => {
  it('shows a perspective affordance only on segments with multiple views', async () => {
    renderExplore()

    expect(screen.getByLabelText('Operating margin has 4 perspectives')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('treeitem', { name: /Operating margin/ }))
    expect(await screen.findByRole('listbox', { name: 'Perspectives for Operating margin' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('option', { name: /Composition/ }))
    await screen.findByRole('treeitem', { name: /Services/ })
    expect(screen.queryByLabelText(/has \d+ perspectives/)).not.toBeInTheDocument()
  })

  it('derives breadcrumbs from URL-restored NodePath labels and jumpTo updates the URL', async () => {
    const path = [
      'profitability',
      'profitability/operating-margin',
      'profitability/operating-margin/composition',
      'profitability/operating-margin/composition/cost-centers',
    ]
    const url = navigationToURL({ path, perspectiveId: 'profitability/operating-margin/composition' }, new URL(window.location.href))
    window.history.replaceState(null, '', url)
    renderExplore()

    expect(screen.getByRole('button', { name: 'Profitability' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Operating margin Composition/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Cost centers' })).toHaveAttribute('aria-current', 'page')

    fireEvent.click(screen.getByRole('button', { name: /Operating margin Composition/ }))
    await waitFor(() => {
      expect(new URL(window.location.href).searchParams.getAll('path')).toEqual(path.slice(0, 2))
    })
    expect(new URL(window.location.href).searchParams.get('perspective')).toBe('profitability/operating-margin/composition')
    expect(screen.getByRole('listbox', { name: 'Perspectives for Operating margin' })).toBeInTheDocument()
  })

  it('moves focus to the changed level and supports sibling arrows and Escape', async () => {
    renderExplore()
    fireEvent.click(screen.getByRole('treeitem', { name: /Operating margin/ }))

    const composition = await screen.findByRole('option', { name: /Composition/ })
    await waitFor(() => expect(composition).toHaveFocus())
    fireEvent.click(composition)
    const services = await screen.findByRole('treeitem', { name: /Services/ })
    await waitFor(() => expect(document.activeElement).toHaveAttribute('data-explore-view', 'donut'))

    services.focus()
    fireEvent.keyDown(services, { key: 'ArrowRight' })
    expect(document.activeElement).toHaveTextContent('Products')
    expect(services).toHaveAttribute('tabindex', '-1')
    expect(document.activeElement).toHaveAttribute('tabindex', '0')
    fireEvent.keyDown(document.activeElement!, { key: 'Escape' })
    expect(await screen.findByRole('listbox', { name: 'Perspectives for Operating margin' })).toBeInTheDocument()
  })

  it('exposes one tab stop per composite widget and Tab skips the remaining items', async () => {
    renderExplore()
    fireEvent.click(screen.getByRole('treeitem', { name: /Operating margin/ }))

    const listbox = await screen.findByRole('listbox', { name: 'Perspectives for Operating margin' })
    const options = within(listbox).getAllByRole('option')
    expect(options.filter((option) => option.tabIndex === 0)).toHaveLength(1)
    fireEvent.keyDown(options[0]!, { key: 'ArrowRight' })
    expect(options[0]).toHaveAttribute('tabindex', '-1')
    expect(options[1]).toHaveAttribute('tabindex', '0')
    expect(options[1]).toHaveFocus()

    screen.getByRole('button', { name: /Back/ }).focus()
    pressTab()
    expect(options[1]).toHaveFocus()
    pressTab()
    expect(screen.getByRole('button', { name: 'After explore' })).toHaveFocus()

    fireEvent.click(options[0]!)
    const tree = await screen.findByRole('tree', { name: /Segments below Composition root/ })
    const treeitems = within(tree).getAllByRole('treeitem')
    expect(treeitems.filter((item) => item.tabIndex === 0)).toHaveLength(1)

    screen.getByRole('button', { name: /Back/ }).focus()
    pressTab()
    expect(treeitems[0]).toHaveFocus()
    pressTab()
    expect(screen.getByRole('button', { name: 'After explore' })).toHaveFocus()
  })

  it('activates anchor treeitems with Space', async () => {
    const path = [
      'profitability',
      'profitability/operating-margin',
      'profitability/operating-margin/composition',
      'profitability/operating-margin/composition/transactions',
    ]
    const url = navigationToURL({
      path,
      perspectiveId: 'profitability/operating-margin/composition',
    }, new URL(window.location.href))
    window.history.replaceState(null, '', url)
    renderExplore()

    const leaf = await screen.findByRole('treeitem', { name: /Invoice TX-1042/ })
    const activated = vi.fn((event: Event) => event.preventDefault())
    leaf.addEventListener('click', activated)
    fireEvent.keyDown(leaf, { key: ' ' })

    expect(activated).toHaveBeenCalledOnce()
  })

  it('replaces a single-perspective auto-switch so browser Back reaches the parent', async () => {
    const pushState = vi.spyOn(window.history, 'pushState')
    renderExplore(singlePerspectiveDocument.panels[0], singlePerspectiveDocument)
    const parentURL = window.location.href
    const parentState: unknown = window.history.state

    fireEvent.click(screen.getByRole('treeitem', { name: /Operating margin/ }))
    await waitFor(() => {
      expect(new URL(window.location.href).searchParams.get('perspective'))
        .toBe('profitability/operating-margin/composition')
    })
    expect(pushState).toHaveBeenCalledTimes(1)

    window.history.replaceState(parentState, '', parentURL)
    window.dispatchEvent(new PopStateEvent('popstate', { state: parentState }))

    await waitFor(() => expect(screen.getByRole('treeitem', { name: /Operating margin/ })).toBeInTheDocument())
    expect(screen.queryByRole('treeitem', { name: /Services/ })).not.toBeInTheDocument()
    expect(new URL(window.location.href).searchParams.has('perspective')).toBe(false)
  })

  it('skips View Transitions when reduced motion is requested', () => {
    const startViewTransition = vi.fn((update: () => void) => {
      update()
      return { ready: Promise.resolve(), finished: Promise.resolve() }
    })
    Object.defineProperty(document, 'startViewTransition', { configurable: true, value: startViewTransition })
    Object.defineProperty(window, 'matchMedia', {
      configurable: true,
      value: vi.fn(() => ({ matches: true })),
    })
    renderExplore()

    fireEvent.click(screen.getByRole('treeitem', { name: /Operating margin/ }))

    expect(startViewTransition).not.toHaveBeenCalled()
  })

  it('renders evidence as an interim row list with resolved full-page leaf links', async () => {
    const path = [
      'profitability',
      'profitability/operating-margin',
      'profitability/operating-margin/evidence',
      'profitability/operating-margin/evidence/root',
    ]
    const url = navigationToURL({ path, perspectiveId: 'profitability/operating-margin/evidence' }, new URL(window.location.href))
    window.history.replaceState(null, '', url)
    renderExplore()

    expect(await screen.findByText('Interim table view')).toBeInTheDocument()
    const links = screen.getAllByRole('link', { name: 'Open record' })
    expect(links).toHaveLength(2)
    expect(links[0]).toHaveAttribute('href', expect.stringContaining('/transactions/TX-1042'))
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

    expect(await screen.findByText('Interim table view')).toBeInTheDocument()
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
