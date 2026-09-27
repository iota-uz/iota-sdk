/* eslint-disable react-refresh/only-export-components */
import type { CSSProperties } from 'react'
import type { LayoutItem, Panel, PanelKind } from '../contract'

/**
 * Loading placeholders mirror the layout they replace instead of showing a
 * spinner: the same rows, the same 12-column spans and a shape per panel kind,
 * so nothing jumps when the data lands. The shape is styles.css's answer to the
 * kind each card states, which is the same answer the server fallback in
 * pkg/lens/render/react gets, so the handoff does not shift the grid.
 */

export function ShimmerBar({ className, style }: { className?: string; style?: CSSProperties }) {
  return <span className={`lens-shimmer ${className ?? ''}`.trim()} style={style} />
}

function spanStyle(span: number): CSSProperties {
  const bounded = Number.isFinite(span) ? Math.min(12, Math.max(1, Math.round(span))) : 12
  return { '--lens-panel-span': bounded } as CSSProperties
}

/**
 * A placeholder states the kind it stands in for and lets the stylesheet decide
 * what that kind reserves. The server fallback in
 * pkg/lens/render/react/skeleton.go emits the same `data-kind` — the runtime
 * replaces that markup in place on the first paint, so the two must reserve the
 * same height or the grid moves at the handoff. One rule in styles.css now
 * answers for both, instead of a Go switch and this one agreeing by hand.
 *
 * `metrics` is the one thing the kind cannot say: a layout item carrying a
 * metric group is a strip of cells in one card, far taller than the single stat
 * card its kind suggests, and it reaches the runtime as stat panels under a
 * group rather than as a kind of its own.
 */
export function PanelSkeletonCard({ kind, metrics }: { kind: PanelKind; metrics?: boolean }) {
  return (
    <div className="lens-skeleton-card" data-kind={kind} data-metrics={metrics ? 'true' : undefined}>
      <ShimmerBar className="lens-shimmer-label" />
      <ShimmerBar className="lens-shimmer-body" />
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
