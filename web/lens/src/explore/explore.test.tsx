import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import fixture from '../../fixtures/explore.json'
import { parseDocument, type Panel } from '../contract'
import type { ChartPanelProps, PanelRegistry, StatPanelProps } from '../panels'
import { DashboardRuntimeProvider, DocumentProvider, navigationToURL } from '../runtime'
import { resolveLeafActionURL } from './actions'
import { ExplorePanel } from './ExplorePanel'
import { viewForSemantics } from './model'

const exploreDocument = parseDocument(fixture)
const panel = exploreDocument.panels[0]!

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

function renderExplore(currentPanel: Panel = panel) {
  return render(
    <div className="lens-root">
      <DocumentProvider initialDocument={exploreDocument}>
        <DashboardRuntimeProvider locale="en">
          <ExplorePanel panel={currentPanel} registry={registry} />
        </DashboardRuntimeProvider>
      </DocumentProvider>
    </div>,
  )
}

afterEach(() => {
  cleanup()
  window.history.replaceState(null, '', '/')
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
    fireEvent.keyDown(document.activeElement!, { key: 'Escape' })
    expect(await screen.findByRole('listbox', { name: 'Perspectives for Operating margin' })).toBeInTheDocument()
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
