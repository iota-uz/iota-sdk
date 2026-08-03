import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { LayoutItem, Panel, PanelKind } from '../contract'
import { DashboardSkeleton, PanelSkeletonCard, skeletonRowsFromLayout } from './Skeleton'

/**
 * The placeholder's geometry is a contract with the server, not a local
 * decision: the runtime replaces the server's markup in place on the very first
 * paint, so a kind that reserves one height there and another here moves the
 * grid at the handoff. Neither side decides the shape now — both state the kind
 * and styles.css answers — so what these tests hold is that the card says which
 * kind it stands in for, which is the whole input the stylesheet gets.
 */
const kinds: PanelKind[] = ['stat', 'cascade', 'coverage', 'bar', 'line', 'pie', 'table', 'map', 'metric_hierarchy']

function panel(id: string, kind: PanelKind): Panel {
  return {
    id, kind, title: id, semantics: 'series', frame: `panel:${id}`,
    encoding: { label: 'label', value: 'value' }, format: {}, actions: [],
  }
}

describe('skeleton shapes', () => {
  it.each(kinds)('labels the %s placeholder with its kind', (kind) => {
    const view = render(<PanelSkeletonCard kind={kind} />)
    const card = view.container.querySelector('.lens-skeleton-card')
    expect(card).toHaveAttribute('data-kind', kind)
    expect(card).not.toHaveAttribute('data-metrics')
    view.unmount()
  })

  it('gives a metric group its own taller card, not a single stat card', () => {
    // A stat_group is several cells in one card: reserving a stat card for it
    // left the first row of every board short by ~150px. It is not a kind of
    // its own, so the card has to say so alongside the kind carrying it.
    const view = render(<PanelSkeletonCard kind="stat" metrics />)
    const card = view.container.querySelector('.lens-skeleton-card')
    expect(card).toHaveAttribute('data-kind', 'stat')
    expect(card).toHaveAttribute('data-metrics', 'true')
  })

  it('takes spans and metric groups from the layout the document declared', () => {
    const items: Array<LayoutItem> = [
      { panelId: 'kpis', span: 8, groups: [{ id: 'kpis', kind: 'metrics', span: 8 }] },
      { panelId: 'mix', span: 4 },
    ]
    const rows = skeletonRowsFromLayout(
      [{ heading: 'Portfolio', panels: items }],
      new Map([['kpis', panel('kpis', 'stat')], ['mix', panel('mix', 'pie')]]),
    )
    expect(rows).toEqual([{
      heading: true,
      items: [{ span: 8, kind: 'stat', metrics: true }, { span: 4, kind: 'pie', metrics: undefined }],
    }])

    const view = render(<DashboardSkeleton rows={rows} />)
    const cards = view.container.querySelectorAll('.lens-grid-item')
    expect(cards).toHaveLength(2)
    expect(cards[0]).toHaveStyle({ '--lens-panel-span': '8' })
    expect(cards[0]?.querySelector('.lens-skeleton-card')).toHaveAttribute('data-metrics', 'true')
    expect(cards[1]?.querySelector('.lens-skeleton-card')).toHaveAttribute('data-kind', 'pie')
  })
})
