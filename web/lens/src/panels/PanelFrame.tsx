/* eslint-disable react/no-unknown-property -- Solid JSX uses `class`, the React-era rule expects `className`; the lint config migrates with the Solid port. */
import { createEffect, createSignal, Show } from 'solid-js'
import type { JSX, JSXElement } from 'solid-js'
import type { Frame, Panel } from '../contract'
import { clampedDeltaPercent, type PanelFrameState, useDashboard, useFormat, useTranslate } from '../runtime'
import { PanelExportMenu } from './PanelExportMenu'
import { InfoTip } from './InfoTip'
import { ArrowsIn, ArrowsOut, ChartLine, TrendDown, TrendFlat, TrendUp } from '../icons'
import { usePanelChrome } from './context'
import { panelFailureCopy } from './panelFailure'
import { PanelOverlay } from './PanelOverlay'
import { PanelSkeletonBody } from './Skeleton'

export interface PanelFrameProps {
  panel: Panel
  frame: PanelFrameState
  children: JSXElement
  variant?: 'stat' | 'chart'
  allowEmptyContent?: boolean
  /** Reader controls that belong to this panel, placed before export chrome. */
  headerActions?: JSXElement
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

/**
 * The verdict a metric's movement earns, or nothing when it has not earned one.
 *
 * The strip has exactly one colour channel and this is it: a figure is tinted
 * when — and only when — the producer has said which direction is good news and
 * the reading actually moved that way. Everything else stays ink, which is what
 * keeps the tint worth reading.
 */
/* eslint-disable react-refresh/only-export-components */
export function trendTone(panel: Panel, frame: Frame | undefined): 'positive' | 'negative' | undefined {
  const trend = panel.trend
  if (!trend) return undefined
  const percent = numericFrameValue(frame, trend.percentField) ?? trend.percent
  if (!Number.isFinite(percent) || percent === 0) return undefined
  if (trend.percentField && numericFrameValue(frame, trend.percentField) === undefined) return undefined
  const polarity = trendPolarity(trend)
  if (polarity === 'neutral') return undefined
  const up = percent > 0
  return (polarity === 'lower_better' ? !up : up) ? 'positive' : 'negative'
}

export function TrendChip(props: { panel: Panel; frame?: Frame }) {
  const trend = props.panel.trend!
  const absolute = () => numericFrameValue(props.frame, trend.absoluteField)
  const framePercent = () => numericFrameValue(props.frame, trend.percentField)
  const percent = () => framePercent() ?? trend.percent
  const formatAbsolute = useFormat(trend.absoluteField ? props.panel.format[trend.absoluteField] : undefined)
  const formatValue = useFormat(props.panel.encoding.value ? props.panel.format[props.panel.encoding.value] : undefined)
  const formatPercentagePoints = useFormat({ kind: 'number', minorUnits: false, precision: 1 })
  const formatPercent = useFormat(trend.percentField
    ? props.panel.format[trend.percentField]
    : { kind: 'percent', minorUnits: false, precision: 1 })
  const translate = useTranslate()
  const { document } = useDashboard()
  // What the metric is being compared against, which the chip never said. A
  // delta without its baseline is half a fact, and the baseline is derivable:
  // the reading on screen minus the absolute change that produced it.
  // A percentage-point delta is stated in the same units as the ratio it moved
  // («3,0 %» after «−21,1 pp» was «24,1 %»), so one subtraction covers both
  // kinds of delta.
  const current = () => numericFrameValue(props.frame, props.panel.encoding.value)
  const baseline = () => {
    const base = current()
    return base !== undefined && absolute() !== undefined ? formatValue(base - absolute()!) : undefined
  }
  const movement = () => trend.absoluteDeltaUnit === 'percentage_points' && absolute() !== undefined ? absolute()! : percent()
  const up = () => movement() > 0
  const flat = () => movement() === 0
  // The polarity is the producer's; the arrow is always the sign's.
  const polarity = trendPolarity(trend)
  const good = () => polarity === 'lower_better' ? !up() : up()
  const tone = () => flat() || polarity === 'neutral'
    ? 'lens-trend-chip-flat'
    : good() ? 'lens-trend-chip-positive' : 'lens-trend-chip-negative'
  const formattedPercent = () => `${up() ? '+' : ''}${formatPercent(percent())}`
  // A change, and the figure it is a change from — which is what the chip never
  // said: «−49,8 %» against nothing named is half a fact, and «Сравнение
  // тренда» beside it named the comparison without ever giving its value. Three
  // numbers would not fit a strip cell and the third is redundant anyway: the
  // reading sits directly above the chip, so the absolute delta is the
  // difference between the two figures already on screen. A trend with no
  // absolute delta to subtract keeps the producer's label.
  const detail = () => {
    const base = baseline()
    return base !== undefined ? translate('panel.trend.baseline', 'was {value}', { value: base }) : undefined
  }
  const absoluteText = () => {
    const value = absolute()
    if (value === undefined) return undefined
    return `${value > 0 ? '+' : ''}${trend.absoluteDeltaUnit === 'percentage_points'
      ? `${formatPercentagePoints(value)} ${translate('panel.trend.percentagePoints', 'pp')}`
      : formatAbsolute(value)}`
  }
  const deltaText = () => trend.absoluteDeltaUnit === 'percentage_points' && absoluteText() !== undefined
    ? absoluteText()
    : clampedDeltaPercent(percent(), document?.meta?.locale) ?? formattedPercent()
  const label = () => trend.label || translate('panel.trend.comparison', 'vs comparison')

  return (
    <Show
      when={!(trend.percentField && framePercent() === undefined)}
      fallback={
        <span class="lens-trend-chip lens-trend-chip-flat">
          <strong>
            {absolute() !== undefined && absolute() !== 0
              ? translate('panel.trend.new', 'New')
              : translate('panel.trend.notAvailable', 'N/A')}
          </strong>
          <span class="lens-trend-chip-label">{trend.label || translate('panel.trend.comparison', 'vs comparison')}</span>
        </span>
      }
    >
      <span
        class={`lens-trend-chip ${tone()}`}
        title={[deltaText(), trend.absoluteDeltaUnit === 'percentage_points' ? undefined : absoluteText(), detail() ?? label()].filter(Boolean).join(' · ')}
      >
        {flat() ? <TrendFlat /> : up() ? <TrendUp /> : <TrendDown />}
        <strong>{deltaText()}</strong>
        <Show when={detail() === undefined && absoluteText() !== undefined}>
          <span class="lens-trend-chip-absolute">({absoluteText()})</span>
        </Show>
        <span class="lens-trend-chip-label">{detail() ?? label()}</span>
      </span>
    </Show>
  )
}

export function PanelFrame(props: PanelFrameProps) {
  const translate = useTranslate()
  const { document: dashboard } = useDashboard()
  const chrome = usePanelChrome()
  const [expanded, setExpanded] = createSignal(false)
  let expandRef: HTMLButtonElement | undefined
  let placeholderRef: HTMLDivElement | undefined
  let restoreFocus = false
  createEffect(() => {
    if (props.frame.error) console.error(`[lens] panel ${props.panel.id} request failed`, props.frame.error)
  })
  // What went wrong, said in the reader's terms. The panel used to print one
  // sentence for every cause — a slice too large to finish, a load the reader
  // themselves abandoned, a genuine fault — which told nobody what to do next.
  const failure = () => panelFailureCopy(props.frame.error, translate)
  const retryLabel = translate('panel.retry', 'Retry')
  const formatTotal = useFormat(props.panel.encoding.value ? props.panel.format[props.panel.encoding.value] : undefined)
  const total = () => props.total === undefined ? props.panel.total : props.total ?? undefined
  const hasRows = () => Boolean(props.frame.data?.rows.length)
  // Loading is panel-local. A sibling calculation or a background document
  // refresh must never replace this panel's usable data with a skeleton.
  const showLoading = () => props.frame.isLoading
  // Dimmed data being replaced is as busy as an empty skeleton is: the figures
  // under the cursor are not the answer to the question that was just asked.
  // An error left the panel stale with no request in flight, so that state is
  // idle — the retry control beside it is the thing to act on.
  const busy = () => showLoading() || (props.frame.isStale && !props.frame.error)
  const badgePlacement = () => props.panel.presentation?.totalBadge ?? 'header'
  const showTotal = () => props.variant === 'chart' && total() !== undefined && badgePlacement() === 'header'
  const totalLabel = translate('panel.total', 'Total')
  const expandLabel = () => expanded() ? translate('panel.collapse', 'Collapse panel') : translate('panel.expand', 'Expand panel')
  // Opt-out chrome: a drawer-hosted panel disables expand (an overlay over a
  // modal is meaningless), and a derived/headline panel disables export.
  const expandable = props.panel.presentation?.expandable !== false
  const exportable = props.panel.presentation?.exportable !== false
  // A stat headline reads number-first: the value leads, and its supporting
  // caption (exact figure, then the muted explainer + period) sits beneath it
  // rather than pushing the number below the fold.
  //
  // A chart panel gets no caption band at all: on a card whose whole job is a
  // plot, a paragraph of prose above it is a permanent tax that pushes the
  // figure below the fold. The caption joins `info` behind the header's ⓘ,
  // which is what the templ runtime already does for stat descriptions.
  const captionBelow = props.variant === 'stat'
  const calculationInfo = () => props.frame.calculation
    ? translate('panel.calculation', 'Calculated in {duration} · cache {cache}', {
      duration: props.frame.calculation.durationMs < 1000
        ? `${props.frame.calculation.durationMs} ms`
        : `${(props.frame.calculation.durationMs / 1000).toFixed(1)} s`,
      cache: props.frame.calculation.cacheHit
        ? translate('panel.cacheHit', 'hit')
        : translate('panel.cacheMiss', 'miss'),
    })
    : ''
  const infoText = () => [props.variant === 'chart' ? props.panel.caption : '', props.panel.info, calculationInfo()]
    .map((part) => part?.trim() ?? '')
    .filter(Boolean)
    .join('\n\n')
  // A drill trail replaces the static title: it says where the panel is and how
  // to get back without spending a row of the grid. A host that already prints
  // the name (a tab label naming its only panel) suppresses it entirely.
  const titleNode = (): JSX.Element => chrome?.trail
    ?? (chrome?.titleIsRedundant ? undefined : <h3 class="lens-panel-title" title={props.panel.title}>{props.panel.title}</h3>)
  const showHeading = () => Boolean(chrome?.trail) || !chrome?.titleIsRedundant || Boolean(infoText())

  const toggleExpanded = () => setExpanded(true)

  const collapse = () => {
    restoreFocus = true
    setExpanded(false)
  }

  // The button is re-parented out of the portal on collapse, so focus can only
  // be restored once the node has been committed back into the grid.
  createEffect(() => {
    if (expanded() || !restoreFocus) return
    restoreFocus = false
    expandRef?.focus()
  })

  const section = (
    <section
      class={[
        'lens-panel',
        props.variant === 'stat' ? 'lens-panel-stat' : 'lens-panel-chart',
        // The skeleton replaces the content outright, so it must not also carry
        // the stale dim — that treatment is only for the moment before a refetch
        // takes over the body.
        props.frame.isStale && !showLoading() ? 'lens-panel-stale' : '',
        props.panel.presentation?.fill ? 'lens-panel-fill' : '',
        expanded() ? 'lens-panel-expanded' : '',
      ].filter(Boolean).join(' ')}
      data-expanded={expanded() || undefined}
      aria-label={props.panel.title}
      aria-busy={busy()}
      data-calculation-cache={props.frame.calculation ? (props.frame.calculation.cacheHit ? 'hit' : 'miss') : undefined}
      data-calculation-ms={props.frame.calculation?.durationMs}
      data-panel-kind={props.panel.kind}
      data-panel-id={props.panel.id}
      data-testid={`lens-panel-${props.panel.id}`}
      data-stale={props.frame.isStale || undefined}
    >
      <header class="lens-panel-header">
        {/* The panel's identity travels as one item. The note explains the
            panel's subject, so it hangs off the title rather than joining the
            controls: export and expand are things you do to the panel, this is
            something the panel says — and when the header wrapped, a loose ⓘ
            was reparented next to download/expand, where it read as a third
            action rather than an annotation. */}
        <Show when={showHeading()}>
          <div class="lens-panel-heading">
            {titleNode()}
            <Show when={infoText()}>
              <InfoTip subject={props.panel.title} text={infoText()} />
            </Show>
          </div>
        </Show>
        {chrome?.explore}
        <div class="lens-panel-actions">
          {props.headerActions}
          <Show when={showTotal()}>
            <span
              class="lens-panel-total"
              data-testid={`lens-panel-${props.panel.id}-total`}
              data-value={total() ?? undefined}
              title={`${totalLabel}: ${formatTotal(total())}`}
            >
              <span class="lens-panel-total-label">{totalLabel}:</span>
              {' '}
              {formatTotal(total())}
            </span>
          </Show>
          <Show when={busy() && !showLoading()}>
            <span class="lens-panel-status" role="status">{translate('panel.updating', 'Updating')}</span>
          </Show>
          <Show when={exportable}>
            <PanelExportMenu panelId={props.panel.id} title={props.panel.title} />
          </Show>
          <Show when={expandable}>
            <button
              aria-label={expandLabel()}
              aria-expanded={expanded()}
              aria-haspopup="dialog"
              class="lens-export-button lens-icon-button"
              onClick={expanded() ? collapse : toggleExpanded}
              ref={(el) => { expandRef = el }}
              title={expandLabel()}
              type="button"
            >
              {expanded() ? <ArrowsIn /> : <ArrowsOut />}
            </button>
          </Show>
        </div>
      </header>
      <Show when={dashboard.header?.subtitle}>
        <p class="lens-panel-export-scope">{dashboard.header?.subtitle}</p>
      </Show>
      <Show when={props.panel.comparisonUnsupported}>
        <p class="lens-panel-comparison-note" role="note">
          {translate('panel.comparisonUnsupported', 'Comparison is not available for this panel.')}
        </p>
      </Show>
      <div class="lens-panel-body">
        <Show when={showLoading()} fallback={
          <Show when={props.frame.error && !props.frame.data} fallback={
            <Show when={!hasRows() && !props.allowEmptyContent} fallback={props.children}>
              <div class="lens-panel-state lens-panel-state-empty">
                <ChartLine className="lens-empty-mark" />
                <span>{translate('panel.empty', 'No data')}</span>
              </div>
            </Show>
          }>
            <div class="lens-panel-state lens-panel-state-error" role={failure().alert ? 'alert' : undefined}>
              <span>{failure().message}</span>
              <button type="button" onClick={props.frame.retry}>{retryLabel}</button>
            </div>
          </Show>
        }>
          <PanelSkeletonBody kind={props.panel.kind} />
        </Show>
      </div>
      <Show when={captionBelow && props.panel.caption}>
        <p class="lens-panel-caption">{props.panel.caption}</p>
      </Show>
      <Show when={props.panel.trend && hasRows()}>
        <footer class="lens-panel-footer"><TrendChip panel={props.panel} frame={props.frame.data} /></footer>
      </Show>
      <Show when={props.frame.error && props.frame.data}>
        <div class="lens-panel-error" role={failure().alert ? 'alert' : undefined}>
          <span>{failure().message}</span>
          <button type="button" onClick={props.frame.retry}>{retryLabel}</button>
        </div>
      </Show>
    </section>
  )

  return (
    <Show when={!expanded()} fallback={
      <>
        {/* A placeholder keeps the grid from reflowing while the panel is away. */}
        <div aria-hidden="true" class="lens-panel-placeholder" ref={(el) => { placeholderRef = el }} />
        <PanelOverlay label={props.panel.title} source={() => placeholderRef} onClose={collapse}>
          {section}
        </PanelOverlay>
      </>
    }>
      {section}
    </Show>
  )
}
