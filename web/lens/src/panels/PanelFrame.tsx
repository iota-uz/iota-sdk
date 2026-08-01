import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import type { Frame, Panel } from '../contract'
import { clampedDeltaPercent, type PanelFrameState, useDashboard, useFormat, useTranslate } from '../runtime'
import { PanelExportMenu } from './PanelExportMenu'
import { InfoTip } from './InfoTip'
import { ArrowsIn, ArrowsOut, ChartLine, TrendDown, TrendFlat, TrendUp } from '../icons'
import { usePanelChrome } from './context'
import { PanelOverlay } from './PanelOverlay'
import { PanelSkeletonBody } from './Skeleton'

export interface PanelFrameProps {
  panel: Panel
  frame: PanelFrameState
  children: ReactNode
  variant?: 'stat' | 'chart'
  allowEmptyContent?: boolean
  /** Reader controls that belong to this panel, placed before export chrome. */
  headerActions?: ReactNode
  /**
   * The total the header badge prints. Overrides `panel.total`, which is the
   * root frame's total and is wrong once the panel is showing a drill level:
   * the badge must name the level on screen, not the panel's origin.
   */
  total?: number
}

function numericFrameValue(frame: Frame | undefined, field: string | undefined): number | undefined {
  if (!frame || !field) return undefined
  const index = frame.columns.findIndex((column) => column.name === field)
  const value = index < 0 ? undefined : frame.rows[0]?.[index]
  const number = typeof value === 'number' ? value : typeof value === 'string' ? Number(value) : NaN
  return Number.isFinite(number) ? number : undefined
}

export function TrendChip({ panel, frame }: { panel: Panel; frame?: Frame }) {
  const trend = panel.trend!
  const absolute = numericFrameValue(frame, trend.absoluteField)
  const framePercent = numericFrameValue(frame, trend.percentField)
  const percent = framePercent ?? trend.percent
  const formatAbsolute = useFormat(trend.absoluteField ? panel.format[trend.absoluteField] : undefined)
  const formatPercentagePoints = useFormat({ kind: 'number', minorUnits: false, precision: 1 })
  const formatPercent = useFormat(trend.percentField
    ? panel.format[trend.percentField]
    : { kind: 'percent', minorUnits: false, precision: 1 })
  const translate = useTranslate()
  const { document } = useDashboard()
  if (trend.percentField && framePercent === undefined) {
    const state = absolute !== undefined && absolute !== 0
      ? translate('panel.trend.new', 'New')
      : translate('panel.trend.notAvailable', 'N/A')
    return (
      <span className="lens-trend-chip lens-trend-chip-flat">
        <strong>{state}</strong>
        <span className="lens-trend-chip-label">{trend.label || translate('panel.trend.comparison', 'vs comparison')}</span>
      </span>
    )
  }
  const up = percent > 0
  const flat = percent === 0
  // Invert flips the good/bad mapping for down-is-good metrics; the arrow
  // always follows the sign.
  const good = trend.invert ? !up : up
  const tone = flat ? 'lens-trend-chip-flat' : good ? 'lens-trend-chip-positive' : 'lens-trend-chip-negative'
  const sign = up ? '+' : ''
  const formattedPercent = `${sign}${formatPercent(percent)}`
  const TrendIcon = flat ? TrendFlat : up ? TrendUp : TrendDown
  return (
    <span className={`lens-trend-chip ${tone}`}>
      <TrendIcon />
      <strong>{clampedDeltaPercent(percent, document?.meta?.locale) ?? formattedPercent}</strong>
      {absolute !== undefined && (
        <span className="lens-trend-chip-absolute">
          ({absolute > 0 ? '+' : ''}{trend.absoluteDeltaUnit === 'percentage_points'
            ? `${formatPercentagePoints(absolute)} ${translate('panel.trend.percentagePoints', 'pp')}`
            : formatAbsolute(absolute)})
        </span>
      )}
      <span className="lens-trend-chip-label">{trend.label || translate('panel.trend.comparison', 'vs comparison')}</span>
    </span>
  )
}

export function PanelFrame({
  panel, frame, children, variant = 'chart', allowEmptyContent = false, total: totalOverride, headerActions,
}: PanelFrameProps) {
  const translate = useTranslate()
  const { document: dashboard } = useDashboard()
  const chrome = usePanelChrome()
  const [expanded, setExpanded] = useState(false)
  const expandRef = useRef<HTMLButtonElement>(null)
  const placeholderRef = useRef<HTMLDivElement>(null)
  const restoreFocus = useRef(false)
  useEffect(() => {
    if (frame.error) console.error(`[lens] panel ${panel.id} request failed`, frame.error)
  }, [frame.error, panel.id])
  const formatTotal = useFormat(panel.encoding.value ? panel.format[panel.encoding.value] : undefined)
  const total = totalOverride ?? panel.total
  const hasRows = Boolean(frame.data?.rows.length)
  // Loading is panel-local. A sibling calculation or a background document
  // refresh must never replace this panel's usable data with a skeleton.
  const showLoading = frame.isLoading
  const badgePlacement = panel.presentation?.totalBadge ?? 'header'
  const showTotal = variant === 'chart' && total !== undefined && badgePlacement === 'header'
  const totalLabel = translate('panel.total', 'Total')
  const expandLabel = expanded ? translate('panel.collapse', 'Collapse panel') : translate('panel.expand', 'Expand panel')
  // Opt-out chrome: a drawer-hosted panel disables expand (an overlay over a
  // modal is meaningless), and a derived/headline panel disables export.
  const expandable = panel.presentation?.expandable !== false
  const exportable = panel.presentation?.exportable !== false
  // A stat headline reads number-first: the value leads, and its supporting
  // caption (exact figure, then the muted explainer + period) sits beneath it
  // rather than pushing the number below the fold.
  //
  // A chart panel gets no caption band at all: on a card whose whole job is a
  // plot, a paragraph of prose above it is a permanent tax that pushes the
  // figure below the fold. The caption joins `info` behind the header's ⓘ,
  // which is what the templ runtime already does for stat descriptions.
  const captionBelow = variant === 'stat'
  const captionNode = captionBelow && panel.caption ? <p className="lens-panel-caption">{panel.caption}</p> : null
  const calculationText = frame.calculation
    ? translate(
      'panel.calculation',
      'Calculated in {duration} · {cache}',
      {
        duration: frame.calculation.durationMs < 1000
          ? `${frame.calculation.durationMs} ms`
          : `${(frame.calculation.durationMs / 1000).toFixed(1)} s`,
        cache: frame.calculation.cacheHit
          ? translate('panel.cacheHit', 'cache hit')
          : translate('panel.cacheMiss', 'computed'),
      },
    )
    : ''
  const infoText = [variant === 'chart' ? panel.caption : '', panel.info, calculationText]
    .map((part) => part?.trim() ?? '')
    .filter(Boolean)
    .join('\n\n')

  const toggleExpanded = useCallback(() => {
    setExpanded((current) => {
      if (current) return false
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
      data-panel-id={panel.id}
      data-stale={frame.isStale || undefined}
    >
      <header className="lens-panel-header">
        {/* A drill trail replaces the static title: it says where the panel is
            and how to get back without spending a row of the grid. */}
        {chrome?.trail ?? <h3 className="lens-panel-title" title={panel.title}>{panel.title}</h3>}
        {/* The note explains the panel's subject, so it hangs off the title
            rather than joining the controls: export and expand are things you
            do to the panel, this is something the panel says. */}
        {infoText && <InfoTip text={infoText} />}
        {chrome?.explore}
        <div className="lens-panel-actions">
          {headerActions}
          {showTotal && (
            <span className="lens-panel-total" title={`${totalLabel}: ${formatTotal(total)}`}>
              <span className="lens-panel-total-label">{totalLabel}:</span>
              {' '}
              {formatTotal(total)}
            </span>
          )}
          {frame.isStale && !showLoading && <span className="lens-panel-status" role="status">{translate('panel.updating', 'Updating')}</span>}
          {exportable && <PanelExportMenu panelId={panel.id} title={panel.title} />}
          {expandable && (
            <button
              aria-label={expandLabel}
              aria-expanded={expanded}
              aria-haspopup="dialog"
              className="lens-export-button lens-icon-button"
              onClick={expanded ? collapse : toggleExpanded}
              ref={expandRef}
              title={expandLabel}
              type="button"
            >
              {expanded ? <ArrowsIn /> : <ArrowsOut />}
            </button>
          )}
        </div>
      </header>
      {dashboard.header?.subtitle && <p className="lens-panel-export-scope">{dashboard.header.subtitle}</p>}
      {panel.comparisonUnsupported && (
        <p className="lens-panel-comparison-note" role="note">
          {translate('panel.comparisonUnsupported', 'Comparison is not available for this panel.')}
        </p>
      )}
      <div className="lens-panel-body">
        {showLoading ? (
          <PanelSkeletonBody kind={panel.kind} />
        ) : frame.error && !frame.data ? (
          <div className="lens-panel-state lens-panel-state-error" role="alert">
            <span>{translate('panel.error', 'This panel could not be rendered.')}</span>
            <button type="button" onClick={frame.retry}>{translate('panel.retry', 'Retry')}</button>
          </div>
        ) : !hasRows && !allowEmptyContent ? (
          <div className="lens-panel-state lens-panel-state-empty">
            <ChartLine className="lens-empty-mark" />
            <span>{translate('panel.empty', 'No data')}</span>
          </div>
        ) : children}
      </div>
      {captionBelow && captionNode}
      {panel.trend && hasRows && (
        <footer className="lens-panel-footer"><TrendChip panel={panel} frame={frame.data} /></footer>
      )}
      {frame.error && frame.data && (
        <div className="lens-panel-error" role="alert">
          <span>{translate('panel.error', 'This panel could not be rendered.')}</span>
          <button type="button" onClick={frame.retry}>{translate('panel.retry', 'Retry')}</button>
        </div>
      )}
    </section>
  )

  if (!expanded) return section
  return (
    <>
      {/* A placeholder keeps the grid from reflowing while the panel is away. */}
      <div aria-hidden="true" className="lens-panel-placeholder" ref={placeholderRef} />
      <PanelOverlay label={panel.title} sourceRef={placeholderRef} onClose={collapse}>
        {section}
      </PanelOverlay>
    </>
  )
}
