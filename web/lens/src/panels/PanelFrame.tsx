import { useState, type ReactNode } from 'react'
import type { Panel } from '../contract'
import { type PanelFrameState, useFormat, useTranslate } from '../runtime'
import { ExportButton } from './ExportButton'
import { PanelSkeletonBody } from './Skeleton'

export interface PanelFrameProps {
  panel: Panel
  frame: PanelFrameState
  children: ReactNode
  variant?: 'stat' | 'chart'
  allowEmptyContent?: boolean
}

export function TrendChip({ trend }: { trend: NonNullable<Panel['trend']> }) {
  const up = trend.percent > 0
  const flat = trend.percent === 0
  // Invert flips the good/bad mapping for down-is-good metrics; the arrow
  // always follows the sign.
  const good = trend.invert ? !up : up
  const tone = flat ? 'lens-trend-chip-flat' : good ? 'lens-trend-chip-positive' : 'lens-trend-chip-negative'
  const sign = up ? '+' : ''
  return (
    <span className={`lens-trend-chip ${tone}`}>
      <span aria-hidden="true">{flat ? '▬' : up ? '▲' : '▼'}</span>
      <strong>{sign}{trend.percent.toFixed(1)}%</strong>
      {trend.label && <span className="lens-trend-chip-label">{trend.label}</span>}
    </span>
  )
}

export function PanelFrame({ panel, frame, children, variant = 'chart', allowEmptyContent = false }: PanelFrameProps) {
  const translate = useTranslate()
  const [expanded, setExpanded] = useState(false)
  const formatTotal = useFormat(panel.encoding.value ? panel.format[panel.encoding.value] : undefined)
  const hasRows = Boolean(frame.data?.rows.length)
  const showInitialLoading = frame.isLoading && !frame.data
  const badgePlacement = panel.presentation?.totalBadge ?? 'header'
  const showTotal = variant === 'chart' && panel.total !== undefined && badgePlacement === 'header'
  const expandLabel = expanded ? translate('panel.collapse', 'Collapse panel') : translate('panel.expand', 'Expand panel')

  return (
    <section
      className={[
        'lens-panel',
        variant === 'stat' ? 'lens-panel-stat' : 'lens-panel-chart',
        frame.isStale ? 'lens-panel-stale' : '',
        panel.presentation?.fill ? 'lens-panel-fill' : '',
        expanded ? 'lens-panel-expanded' : '',
      ].filter(Boolean).join(' ')}
      aria-label={panel.title}
      aria-busy={frame.isLoading}
      data-panel-kind={panel.kind}
      data-stale={frame.isStale || undefined}
    >
      <header className="lens-panel-header">
        <h3 className="lens-panel-title">{panel.title}</h3>
        <div className="lens-panel-actions">
          {showTotal && (
            <span className="lens-panel-total">
              <span className="lens-panel-total-label">{translate('panel.total', 'Total')}:</span>
              {' '}
              {formatTotal(panel.total)}
            </span>
          )}
          {frame.isStale && <span className="lens-panel-status" role="status">{translate('panel.updating', 'Updating')}</span>}
          <ExportButton panelId={panel.id} iconOnly />
          <button
            aria-label={expandLabel}
            aria-pressed={expanded}
            className="lens-export-button lens-icon-button"
            onClick={() => setExpanded((current) => !current)}
            title={expandLabel}
            type="button"
          >
            <span aria-hidden="true">{expanded ? '⤡' : '⤢'}</span>
          </button>
        </div>
      </header>
      <div className="lens-panel-body">
        {showInitialLoading ? (
          <PanelSkeletonBody kind={panel.kind} />
        ) : frame.error && !frame.data ? (
          <div className="lens-panel-state lens-panel-state-error" role="alert">
            <span>{frame.error.message}</span>
            <button type="button" onClick={frame.retry}>{translate('panel.retry', 'Retry')}</button>
          </div>
        ) : !hasRows && !allowEmptyContent ? (
          <div className="lens-panel-state lens-panel-state-empty">
            <span className="lens-empty-mark" aria-hidden="true">—</span>
            <span>{translate('panel.empty', 'No data')}</span>
          </div>
        ) : children}
      </div>
      {panel.trend && hasRows && (
        <footer className="lens-panel-footer"><TrendChip trend={panel.trend} /></footer>
      )}
      {frame.error && frame.data && (
        <div className="lens-panel-error" role="alert">
          <span>{frame.error.message}</span>
          <button type="button" onClick={frame.retry}>{translate('panel.retry', 'Retry')}</button>
        </div>
      )}
    </section>
  )
}
