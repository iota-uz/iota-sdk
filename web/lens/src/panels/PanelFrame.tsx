import type { ReactNode } from 'react'
import type { Panel } from '../contract'
import type { PanelFrameState } from '../runtime'

export interface PanelFrameProps {
  panel: Panel
  frame: PanelFrameState
  children: ReactNode
  variant?: 'stat' | 'chart'
}

function PanelSkeleton({ variant }: { variant: 'stat' | 'chart' }) {
  return (
    <div className={`lens-panel-skeleton lens-panel-skeleton-${variant}`} role="status" aria-label="Loading panel">
      <span className="lens-skeleton-line lens-skeleton-line-label" />
      <span className="lens-skeleton-line lens-skeleton-line-value" />
      {variant === 'chart' && <span className="lens-skeleton-chart" />}
    </div>
  )
}

export function PanelFrame({ panel, frame, children, variant = 'chart' }: PanelFrameProps) {
  const hasRows = Boolean(frame.data?.rows.length)
  const showInitialLoading = frame.isLoading && !frame.data

  return (
    <section
      className={`lens-panel lens-panel-${variant}${frame.isStale ? ' lens-panel-stale' : ''}`}
      aria-label={panel.title}
      aria-busy={frame.isLoading}
      data-panel-kind={panel.kind}
      data-stale={frame.isStale || undefined}
    >
      <header className="lens-panel-header">
        <h3 className="lens-panel-title">{panel.title}</h3>
        {frame.isStale && <span className="lens-panel-status" role="status">Updating</span>}
      </header>
      <div className="lens-panel-body">
        {showInitialLoading ? (
          <PanelSkeleton variant={variant} />
        ) : frame.error && !frame.data ? (
          <div className="lens-panel-state lens-panel-state-error" role="alert">
            <span>{frame.error.message}</span>
            <button type="button" onClick={frame.retry}>Retry</button>
          </div>
        ) : !hasRows ? (
          <div className="lens-panel-state lens-panel-state-empty">
            <span className="lens-empty-mark" aria-hidden="true">—</span>
            <span>No data</span>
          </div>
        ) : children}
      </div>
      {frame.error && frame.data && (
        <div className="lens-panel-error" role="alert">
          <span>{frame.error.message}</span>
          <button type="button" onClick={frame.retry}>Retry</button>
        </div>
      )}
    </section>
  )
}
