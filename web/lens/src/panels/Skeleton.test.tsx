import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { LayoutItem, Panel, PanelKind } from '../contract'
import { DashboardSkeleton, PanelSkeletonCard, skeletonRowsFromLayout, skeletonShape } from './Skeleton'

/**
 * The placeholder's geometry is a contract with the server, not a local
 * decision: `pkg/lens/render/react/skeleton.go` emits the same classes for the
 * same panel kinds, and the runtime replaces that server markup in place on the
 * very first paint. If the two ever disagree the grid moves at the handoff, and
 * nothing but this table and its Go counterpart says they must not.
 */
const shapeByKind: Array<[PanelKind, string]> = [
  ['stat', 'stat'],
  ['cascade', 'compact'],
  ['coverage', 'compact'],
  ['bar', 'plot'],
  ['line', 'plot'],
  ['pie', 'plot'],
  ['table', 'plot'],
  ['map', 'plot'],
  ['metric_hierarchy', 'plot'],
]

function panel(id: string, kind: PanelKind): Panel {
  return {
    id, kind, title: id, semantics: 'series', frame: `panel:${id}`,
    encoding: { label: 'label', value: 'value' }, format: {}, actions: [],
  }
}

describe('skeleton shapes', () => {
  it.each(shapeByKind)('maps %s to the %s card', (kind, shape) => {
    expect(skeletonShape(kind)).toBe(shape)
    const view = render(<PanelSkeletonCard kind={kind} />)
    expect(view.container.querySelector(`.lens-skeleton-card-${shape}`)).not.toBeNull()
    view.unmount()
  })

  it('gives a metric group its own taller card, not a single stat card', () => {
    // A stat_group is several cells in one card: reserving a stat card for it
    // left the first row of every board short by ~150px.
    expect(skeletonShape('stat', true)).toBe('metrics')
    const view = render(<PanelSkeletonCard kind="stat" metrics />)
    expect(view.container.querySelector('.lens-skeleton-card-metrics')).not.toBeNull()
    expect(view.container.querySelector('.lens-skeleton-card-stat')).toBeNull()
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
    expect(cards[0]?.querySelector('.lens-skeleton-card-metrics')).not.toBeNull()
    expect(cards[1]?.querySelector('.lens-skeleton-card-plot')).not.toBeNull()
  })
})
