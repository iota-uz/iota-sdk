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
   *
   * `null` and `undefined` are different answers. `null` is the caller saying
   * "there is no total here" — every series hidden, or a frame whose rows do
   * not sum to a fact — and it wins. `undefined` is the caller not having an
   * opinion, which is what falls through to the panel's own total.
   *
   * They used to be the same `??`, so a panel that had just decided it could
   * state no total still printed the root figure in its header while its body
   * said nothing was shown.
   */
  total?: number | null
}

function numericFrameValue(frame: Frame | undefined, field: string | undefined): number | undefined {
  if (!frame || !field) return undefined
  const index = frame.columns.findIndex((column) => column.name === field)
  const value = index < 0 ? undefined : frame.rows[0]?.[index]
  const number = typeof value === 'number' ? value : typeof value === 'string' ? Number(value) : NaN
  return Number.isFinite(number) ? number : undefined
}

/**
 * Whether a rise in this metric is good, bad or neither.
 *
 * Unset is neutral. The renderer cannot know: a rise in earned premium is good,
 * a rise in loss ratio is bad, and a rise in sum insured is exposure — more of a
 * thing, with no verdict. Colouring by sign alone painted the third case green
 * on every board. `invert` is the producers' existing two-state declaration and
 * still means "lower is better"; anything else has to say so explicitly.
 */
function trendPolarity(trend: NonNullable<Panel['trend']>): 'higher_better' | 'lower_better' | 'neutral' {
  if (trend.polarity) return trend.polarity
  return trend.invert ? 'lower_better' : 'neutral'
}

export function TrendChip({ panel, frame }: { panel: Panel; frame?: Frame }) {
  const trend = panel.trend!
  const absolute = numericFrameValue(frame, trend.absoluteField)
  const framePercent = numericFrameValue(frame, trend.percentField)
  const percent = framePercent ?? trend.percent
  const formatAbsolute = useFormat(trend.absoluteField ? panel.format[trend.absoluteField] : undefined)
  const formatValue = useFormat(panel.encoding.value ? panel.format[panel.encoding.value] : undefined)
  const formatPercentagePoints = useFormat({ kind: 'number', minorUnits: false, precision: 1 })
  const formatPercent = useFormat(trend.percentField
    ? panel.format[trend.percentField]
    : { kind: 'percent', minorUnits: false, precision: 1 })
  const translate = useTranslate()
  const { document } = useDashboard()
  // What the metric is being compared against, which the chip never said. A
  // delta without its baseline is half a fact, and the baseline is derivable:
  // the reading on screen minus the absolute change that produced it.
  // A percentage-point delta is stated in the same units as the ratio it moved
  // («3,0 %» after «−21,1 pp» was «24,1 %»), so one subtraction covers both
  // kinds of delta.
  const current = numericFrameValue(frame, panel.encoding.value)
  const baseline = current !== undefined && absolute !== undefined
    ? formatValue(current - absolute)
    : undefined
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
  // The polarity is the producer's; the arrow is always the sign's.
  const polarity = trendPolarity(trend)
  const good = polarity === 'lower_better' ? !up : up
  const tone = flat || polarity === 'neutral'
    ? 'lens-trend-chip-flat'
    : good ? 'lens-trend-chip-positive' : 'lens-trend-chip-negative'
  const sign = up ? '+' : ''
  const formattedPercent = `${sign}${formatPercent(percent)}`
  const TrendIcon = flat ? TrendFlat : up ? TrendUp : TrendDown
  // A change, and the figure it is a change from — which is what the chip never
  // said: «−49,8 %» against nothing named is half a fact, and «Сравнение
  // тренда» beside it named the comparison without ever giving its value. Three
  // numbers would not fit a strip cell and the third is redundant anyway: the
  // reading sits directly above the chip, so the absolute delta is the
  // difference between the two figures already on screen. A trend with no
  // absolute delta to subtract keeps the producer's label.
  const detail = baseline !== undefined
    ? translate('panel.trend.baseline', 'was {value}', { value: baseline })
    : undefined
  const absoluteText = absolute === undefined
    ? undefined
    : `${absolute > 0 ? '+' : ''}${trend.absoluteDeltaUnit === 'percentage_points'
      ? `${formatPercentagePoints(absolute)} ${translate('panel.trend.percentagePoints', 'pp')}`
      : formatAbsolute(absolute)}`
  const label = trend.label || translate('panel.trend.comparison', 'vs comparison')
  return (
    <span
      className={`lens-trend-chip ${tone}`}
      title={[formattedPercent, absoluteText, detail ?? label].filter(Boolean).join(' · ')}
    >
      <TrendIcon />
      <strong>{clampedDeltaPercent(percent, document?.meta?.locale) ?? formattedPercent}</strong>
      {detail === undefined && absoluteText !== undefined && (
        <span className="lens-trend-chip-absolute">({absoluteText})</span>
      )}
      <span className="lens-trend-chip-label">{detail ?? label}</span>
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
  const total = totalOverride === undefined ? panel.total : totalOverride ?? undefined
  const hasRows = Boolean(frame.data?.rows.length)
  // Loading is panel-local. A sibling calculation or a background document
  // refresh must never replace this panel's usable data with a skeleton.
  const showLoading = frame.isLoading
  // Dimmed data being replaced is as busy as an empty skeleton is: the figures
  // under the cursor are not the answer to the question that was just asked.
  // An error left the panel stale with no request in flight, so that state is
  // idle — the retry control beside it is the thing to act on.
  const busy = showLoading || (frame.isStale && !frame.error)
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
  // What the ⓘ is for: the definition of the figure, and nothing else. It used
  // to append «Calculated in 0 ms · cache hit» to every panel, which made the
  // affordance unconditional — a glyph promising an explanation and paying out
  // query telemetry, on cards that had no explanation to give. Cache state and
  // millisecond timings are a developer's question, so they are answered where
  // a developer asks it: on the panel element, readable from the console and
  // from an automated check, invisible to a reader.
  const infoText = [variant === 'chart' ? panel.caption : '', panel.info]
    .map((part) => part?.trim() ?? '')
    .filter(Boolean)
    .join('\n\n')
  // A drill trail replaces the static title: it says where the panel is and how
  // to get back without spending a row of the grid. A host that already prints
  // the name (a tab label naming its only panel) suppresses it entirely.
  const titleNode = chrome?.trail
    ?? (chrome?.titleIsRedundant ? undefined : <h3 className="lens-panel-title" title={panel.title}>{panel.title}</h3>)

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
      aria-busy={busy}
      data-calculation-cache={frame.calculation ? (frame.calculation.cacheHit ? 'hit' : 'miss') : undefined}
      data-calculation-ms={frame.calculation?.durationMs}
      data-panel-kind={panel.kind}
      data-panel-id={panel.id}
      data-stale={frame.isStale || undefined}
    >
      <header className="lens-panel-header">
        {/* The panel's identity travels as one item. The note explains the
            panel's subject, so it hangs off the title rather than joining the
            controls: export and expand are things you do to the panel, this is
            something the panel says — and when the header wrapped, a loose ⓘ
            was reparented next to download/expand, where it read as a third
            action rather than an annotation. */}
        {(titleNode || infoText) && (
          <div className="lens-panel-heading">
            {titleNode}
            {infoText && <InfoTip subject={panel.title} text={infoText} />}
          </div>
        )}
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
          {busy && !showLoading && <span className="lens-panel-status" role="status">{translate('panel.updating', 'Updating')}</span>}
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
