/* eslint-disable react-refresh/only-export-components */
import type { CSSProperties } from 'react'
import type { LayoutItem, Panel, PanelKind } from '../contract'

/**
 * Loading placeholders mirror the layout they replace instead of showing a
 * spinner: the same rows, the same 12-column spans and a shape per panel kind,
 * so nothing jumps when the data lands. Shapes match the server fallback in
 * pkg/lens/render/react so the handoff does not shift the grid.
 */

export function ShimmerBar({ className, style }: { className?: string; style?: CSSProperties }) {
  return <span className={`lens-shimmer ${className ?? ''}`.trim()} style={style} />
}

function spanStyle(span: number): CSSProperties {
  const bounded = Number.isFinite(span) ? Math.min(12, Math.max(1, Math.round(span))) : 12
  return { '--lens-panel-span': bounded } as CSSProperties
}

/**
 * The shape a placeholder takes, which is not quite the panel kind: a layout
 * item carrying a metric group is a strip of cells in one card and is far
 * taller than the single stat card its kind would suggest. Mirrors
 * `skeletonCardClass` in pkg/lens/render/react/skeleton.go — the two emit the
 * same class for the same shape, or the server placeholder and the runtime one
 * reserve different heights and the grid moves at handoff.
 */
export type SkeletonShape = 'stat' | 'metrics' | 'compact' | 'plot'

export function skeletonShape(kind: PanelKind, metrics = false): SkeletonShape {
  if (metrics) return 'metrics'
  if (kind === 'stat') return 'stat'
  if (kind === 'cascade' || kind === 'coverage') return 'compact'
  return 'plot'
}

export function PanelSkeletonCard({ kind, metrics }: { kind: PanelKind; metrics?: boolean }) {
  const shape = skeletonShape(kind, metrics)
  if (shape === 'stat' || shape === 'metrics') {
    return (
      <div className={`lens-skeleton-card lens-skeleton-card-${shape}`}>
        <ShimmerBar className="lens-shimmer-label" style={{ width: '60%' }} />
        <ShimmerBar className="lens-shimmer-value" style={{ width: '70%' }} />
      </div>
    )
  }
  if (shape === 'compact') {
    return (
      <div className="lens-skeleton-card lens-skeleton-card-compact">
        <ShimmerBar className="lens-shimmer-label" style={{ width: '35%' }} />
        <ShimmerBar className="lens-shimmer-label" style={{ width: '100%' }} />
      </div>
    )
  }
  return (
    <div className="lens-skeleton-card lens-skeleton-card-plot">
      <ShimmerBar className="lens-shimmer-label" style={{ width: '35%' }} />
      <ShimmerBar className="lens-shimmer-block" />
    </div>
  )
}

/** The body-only shape used inside an existing panel card. */
export function PanelSkeletonBody({ kind }: { kind: PanelKind }) {
  return (
    <div aria-hidden="true" className="lens-panel-skeleton" role="presentation">
      <PanelSkeletonCard kind={kind} />
    </div>
  )
}

export interface SkeletonRow {
  heading?: boolean
  items: Array<{ span: number; kind: PanelKind; metrics?: boolean }>
}

/**
 * Derives a placeholder from a layout the runtime already knows. Before the
 * first document arrives the runtime knows nothing, so the server-rendered
 * fallback is used instead and this shape only backs refreshes and stories.
 */
export function DashboardSkeleton({ rows }: { rows: SkeletonRow[] }) {
  return (
    <div aria-hidden="true" className="lens-dashboard-skeleton" role="presentation">
      {rows.map((row, rowIndex) => (
        <section className="lens-dashboard-row" key={rowIndex}>
          {row.heading && (
            <div className="lens-skeleton-heading">
              <ShimmerBar className="lens-shimmer-label" style={{ width: '8rem' }} />
              <span className="lens-skeleton-heading-rule" />
            </div>
          )}
          <div className="lens-panel-grid">
            {row.items.map((item, itemIndex) => (
              <div className="lens-grid-item" key={itemIndex} style={spanStyle(item.span)}>
                <PanelSkeletonCard kind={item.kind} metrics={item.metrics} />
              </div>
            ))}
          </div>
        </section>
      ))}
    </div>
  )
}

export function skeletonRowsFromLayout(
  rows: Array<{ heading?: string; panels: LayoutItem[] }>,
  panels: Map<string, Panel>,
): SkeletonRow[] {
  return rows.map((row) => ({
    heading: Boolean(row.heading),
    items: row.panels.map((item) => ({
      span: item.span,
      kind: panels.get(item.panelId)?.kind ?? 'bar',
      metrics: item.groups?.some((group) => group.kind === 'metrics'),
    })),
  }))
}

/** A neutral three-card placeholder for the pre-document moment. */
export const defaultSkeletonRows: SkeletonRow[] = [
  { items: [{ span: 3, kind: 'stat' }, { span: 3, kind: 'stat' }, { span: 3, kind: 'stat' }, { span: 3, kind: 'stat' }] },
  { heading: true, items: [{ span: 6, kind: 'pie' }, { span: 6, kind: 'bar' }] },
  { heading: true, items: [{ span: 12, kind: 'table' }] },
]

/**
 * The drawer's own pre-document placeholder. A drill drawer's median content is
 * a single full-width headline stat over one records/breakdown table — never
 * the dashboard's stat strip + chart pair. Shaping the drawer skeleton to that
 * (one full-width headline card, one full-width table block) keeps the drawer
 * from jumping when the document lands. The runtime knows nothing about the
 * incoming shape before the fetch, so this is a fixed drawer-median default
 * rather than a per-document hint.
 */
export const drawerSkeletonRows: SkeletonRow[] = [
  { items: [{ span: 12, kind: 'stat' }] },
  { items: [{ span: 12, kind: 'table' }] },
]
