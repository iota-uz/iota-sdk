import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import type { Panel } from '../contract'
import { clampedDeltaPercent, type PanelFrameState, useDocumentRefreshing, useFormat, useTranslate } from '../runtime'
import { ExportButton } from './ExportButton'
import { ArrowsIn, ArrowsOut } from '../icons'
import { usePanelChrome } from './context'
import { PanelOverlay } from './PanelOverlay'
import { PanelSkeletonBody } from './Skeleton'

export interface PanelFrameProps {
  panel: Panel
  frame: PanelFrameState
  children: ReactNode
  variant?: 'stat' | 'chart'
  allowEmptyContent?: boolean
  /**
   * The total the header badge prints. Overrides `panel.total`, which is the
   * root frame's total and is wrong once the panel is showing a drill level:
   * the badge must name the level on screen, not the panel's origin.
   */
  total?: number
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
      <strong>{clampedDeltaPercent(trend.percent) ?? `${sign}${trend.percent.toFixed(1)}%`}</strong>
      {trend.label && <span className="lens-trend-chip-label">{trend.label}</span>}
    </span>
  )
}

export function PanelFrame({ panel, frame, children, variant = 'chart', allowEmptyContent = false, total: totalOverride }: PanelFrameProps) {
  const translate = useTranslate()
  const chrome = usePanelChrome()
  const [expanded, setExpanded] = useState(false)
  const [overlayTheme, setOverlayTheme] = useState<{ theme?: string; dark: boolean }>({ dark: false })
  const expandRef = useRef<HTMLButtonElement>(null)
  const restoreFocus = useRef(false)
  const formatTotal = useFormat(panel.encoding.value ? panel.format[panel.encoding.value] : undefined)
  const total = totalOverride ?? panel.total
  const hasRows = Boolean(frame.data?.rows.length)
  // A date/period change refetches the whole document; the panel's own drill
  // query sets isLoading. Either way the panel is recomputing, so it shows the
  // same skeleton as the first load instead of a subtle dim — an unmistakable
  // "this is being recalculated" affordance, and the exact shape the data will
  // land in.
  const isRefreshing = useDocumentRefreshing()
  const showLoading = frame.isLoading || isRefreshing
  const badgePlacement = panel.presentation?.totalBadge ?? 'header'
  const showTotal = variant === 'chart' && total !== undefined && badgePlacement === 'header'
  const totalLabel = translate('panel.total', 'Total')
  const expandLabel = expanded ? translate('panel.collapse', 'Collapse panel') : translate('panel.expand', 'Expand panel')

  const toggleExpanded = useCallback(() => {
    setExpanded((current) => {
      if (current) return false
      // The dialog is portaled out of the dashboard subtree, so its theme has
      // to be read from the root it is leaving.
      const root = expandRef.current?.closest<HTMLElement>('.lens-root')
      setOverlayTheme({ theme: root?.dataset.theme, dark: root?.classList.contains('dark') ?? false })
      return true
    })
  }, [])

  const collapse = useCallback(() => {
    restoreFocus.current = true
    setExpanded(false)
  }, [])

  // The button is re-parented out of the portal on collapse, so focus can only
  // be restored once React has committed the node back into the grid.
  useEffect(() => {
    if (expanded || !restoreFocus.current) return
    restoreFocus.current = false
    expandRef.current?.focus()
  }, [expanded])

  const section = (
    <section
      className={[
        'lens-panel',
        variant === 'stat' ? 'lens-panel-stat' : 'lens-panel-chart',
        // The skeleton replaces the content outright, so it must not also carry
        // the stale dim — that treatment is only for the moment before a refetch
        // takes over the body.
        frame.isStale && !showLoading ? 'lens-panel-stale' : '',
        panel.presentation?.fill ? 'lens-panel-fill' : '',
        expanded ? 'lens-panel-expanded' : '',
      ].filter(Boolean).join(' ')}
      data-expanded={expanded || undefined}
      aria-label={panel.title}
      aria-busy={showLoading}
      data-panel-kind={panel.kind}
      data-stale={frame.isStale || undefined}
    >
      <header className="lens-panel-header">
        {/* A drill trail replaces the static title: it says where the panel is
            and how to get back without spending a row of the grid. */}
        {chrome?.trail ?? <h3 className="lens-panel-title" title={panel.title}>{panel.title}</h3>}
        {chrome?.explore}
        <div className="lens-panel-actions">
          {showTotal && (
            <span className="lens-panel-total" title={`${totalLabel}: ${formatTotal(total)}`}>
              <span className="lens-panel-total-label">{totalLabel}:</span>
              {' '}
              {formatTotal(total)}
            </span>
          )}
          {frame.isStale && !showLoading && <span className="lens-panel-status" role="status">{translate('panel.updating', 'Updating')}</span>}
          <ExportButton panelId={panel.id} iconOnly />
          <button
            aria-label={expandLabel}
            aria-pressed={expanded}
            className="lens-export-button lens-icon-button"
            onClick={expanded ? collapse : toggleExpanded}
            ref={expandRef}
            title={expandLabel}
            type="button"
          >
            {expanded ? <ArrowsIn /> : <ArrowsOut />}
          </button>
        </div>
      </header>
      {panel.caption && <p className="lens-panel-caption">{panel.caption}</p>}
      <div className="lens-panel-body">
        {showLoading ? (
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

  if (!expanded) return section
  return (
    <>
      {/* A placeholder keeps the grid from reflowing while the panel is away. */}
      <div aria-hidden="true" className="lens-panel-placeholder" />
      <PanelOverlay label={panel.title} theme={overlayTheme.theme} dark={overlayTheme.dark} onClose={collapse}>
        {section}
      </PanelOverlay>
    </>
  )
}
