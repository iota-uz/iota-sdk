/* eslint-disable react/no-unknown-property -- Solid JSX uses `class`, the React-era rule expects `className`; the lint config migrates with the Solid port. */
import { Show } from 'solid-js'
import type { JSX } from 'solid-js'
import type { Panel, Sparkline } from '../contract'
import { useFormat, useFormatExact, usePanelFrame, useTranslate } from '../runtime'
import { ArrowUpRight } from '../icons'
import { usePanelNavigation, usePrefetch, type PrefetchHandlers } from './actions'
import { StatValueTicker } from './StatValueTicker'
import { cell, displayText, panelField } from './data'
import { InfoTip } from './InfoTip'
import { PanelFrame, TrendChip, trendTone } from './PanelFrame'
import { useIsClamped } from './useIsClamped'

export interface StatPanelProps {
  panel: Panel
}

function numeric(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim()) {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return undefined
}

/**
 * A quiet trend line riding beside the stat value — the same footprint as the
 * legacy KPI-strip sparkline: a 1px polyline with a dot on the latest point.
 * Decorative by contract (`aria-hidden`); the trend chip carries the words.
 */
export function StatSparkline(props: { sparkline: Sparkline; tone?: 'positive' | 'negative' }) {
  const width = 44
  const height = 18
  const values = props.sparkline.values.filter((value) => Number.isFinite(value))
  if (values.length < 2) return null
  const min = Math.min(...values)
  const max = Math.max(...values)
  const span = max - min || 1
  const points = values.map((value, index) => {
    const x = (index / (values.length - 1)) * (width - 4) + 2
    const y = height - 2.5 - ((value - min) / span) * (height - 5)
    return `${x.toFixed(1)},${y.toFixed(1)}`
  })
  const [lastX, lastY] = points[points.length - 1]!.split(',')
  // A producer colour wins; otherwise the line carries the same verdict its
  // delta chip carries, so the strip reads at a glance rather than only after
  // the chips are parsed one by one. No verdict keeps the neutral accent.
  const toneColor = props.tone === 'positive' ? 'var(--lens-pos)' : props.tone === 'negative' ? 'var(--lens-neg)' : undefined
  const color = props.sparkline.color?.trim() || toneColor || 'var(--lens-accent-500)'
  return (
    <svg
      aria-hidden="true"
      class="lens-stat-sparkline"
      height={height}
      viewBox={`0 0 ${width} ${height}`}
      width={width}
    >
      <polyline
        fill="none"
        opacity={0.8}
        points={points.join(' ')}
        stroke-width={1.25}
        style={{ stroke: color }}
      />
      <circle cx={lastX} cy={lastY} r={1.8} style={{ fill: color }} />
    </svg>
  )
}

export function StatusChip(props: { status: NonNullable<Panel['status']> }) {
  return (
    <span
      class={`lens-status-chip ${props.status.tone === 'positive'
        ? 'lens-status-chip-positive'
        : props.status.tone === 'warning' ? 'lens-status-chip-warning' : 'lens-status-chip-neutral'}`}
    >
      {props.status.label}
    </span>
  )
}

function useStatValues(panel: Panel) {
  const frame = usePanelFrame(panel.id)
  const valueField = panelField(panel, 'value')
  const deltaField = panelField(panel, 'final')
  const formatValue = useFormat(valueField ? panel.format[valueField] : undefined)
  const formatValueExact = useFormatExact(valueField ? panel.format[valueField] : undefined)
  const formatDelta = useFormat(deltaField ? panel.format[deltaField] : undefined)
  return { frame, valueField, deltaField, formatValue, formatValueExact, formatDelta }
}

/**
 * A stat card that carries a panel-level navigate action is a link in full.
 *
 * The anchor is the whole card and nothing else: the glyph that says so rides
 * next to the value (`StatDrillMark`), where a reader is already looking, rather
 * than in the card's top-right corner — which was 10px of arrow far from the
 * figure it belonged to, sharing its hit area with the ⓘ button beside it.
 */
export function StatLink(props: {
  href?: string
  label: string
  children: JSX.Element
  onClick?: (event: MouseEvent) => void
  prefetch?: PrefetchHandlers
}) {
  const translate = useTranslate()
  return (
    <Show when={props.href} fallback={<>{props.children}</>}>
      <div class="lens-stat-linked">
        <a
          aria-label={translate('panel.openMetric', 'Open {name}', { name: props.label })}
          class="lens-card-link"
          href={props.href}
          onClick={props.onClick}
          {...props.prefetch}
        />
        {props.children}
      </div>
    </Show>
  )
}

/** The "this figure opens" mark, drawn beside the value it opens. */
function StatDrillMark() {
  return <span aria-hidden="true" class="lens-stat-drill-mark"><ArrowUpRight /></span>
}

export function StatPanel(props: StatPanelProps) {
  const panel = props.panel
  const { frame, valueField, deltaField, formatValue, formatValueExact, formatDelta } = useStatValues(panel)
  // The dataset may repeat the panel title in its label column; only a label
  // that says something the header does not is worth a second line.
  const label = () => displayText(cell(frame.data, panelField(panel, 'label')), panel.title)
  const showLabel = () => label() !== panel.title
  const delta = () => deltaField ? cell(frame.data, deltaField) : undefined
  const navigation = usePanelNavigation(panel)
  const href = () => navigation.cardURL(frame.data)
  const prefetch = usePrefetch(href, () => navigation.action, navigation.prefetchIdle)

  return (
    <PanelFrame panel={panel} frame={frame} variant="stat">
      <StatLink href={href()} label={panel.title} onClick={navigation.onClick(href())} prefetch={prefetch}>
        <div class="lens-stat-content">
          <Show when={showLabel() || panel.status}>
            <p class="lens-stat-label">
              <Show when={showLabel()}>
                <span class="lens-stat-label-text" title={label()}>{label()}</span>
              </Show>
              <Show when={panel.status}>
                <StatusChip status={panel.status!} />
              </Show>
            </p>
          </Show>
          <div class="lens-stat-value-row">
            {/* The abbreviated value keeps its exact grouped figure reachable
              on hover: «106.03 млрд UZS» titles «106 034 767 694 UZS». */}
            <p class="lens-stat-value" title={formatValueExact(cell(frame.data, valueField))}><StatValueTicker text={formatValue(cell(frame.data, valueField))} /></p>
            <Show when={href()}><StatDrillMark /></Show>
            <Show when={delta() !== undefined}>
              <span class={`lens-stat-delta${numeric(delta()) !== undefined && numeric(delta())! < 0 ? ' lens-stat-delta-negative' : ''}`}>
                {numeric(delta()) !== undefined && numeric(delta())! > 0 ? '+' : ''}{formatDelta(delta())}
              </span>
            </Show>
            <Show when={panel.sparkline}>
              <StatSparkline sparkline={panel.sparkline!} tone={trendTone(panel, frame.data)} />
            </Show>
          </div>
        </div>
      </StatLink>
    </PanelFrame>
  )
}

/**
 * StatMetric is the chrome-free form of a stat panel used inside a metrics
 * group card.
 *
 * One anatomy, in one order, on every dashboard: name, then figure (with its
 * trend line and its drill mark on the same row), then the change against the
 * comparison, then the note. Each of those is a slot of its own height, so a
 * two-line name does not push its figure 20px below its neighbours' and a
 * one-line note does not lift a card off the row's baseline. Sharing one
 * horizontal line of numbers is the entire point of a strip.
 *
 * There is no accent bullet. It was a coloured square with no legend, and it
 * said different things on each board — the section it already sat under on
 * claims, an arbitrary hue per metric on sales, "red means bad" on exactly one
 * card. Colour that looks semantic and isn't is worse than no colour.
 */
export function StatMetric(props: StatPanelProps) {
  const panel = props.panel
  const { frame, valueField, formatValue, formatValueExact } = useStatValues(panel)
  // The dataset may repeat the panel title in its label column; only a label
  // that says something the header does not is worth a second line.
  const label = () => displayText(cell(frame.data, panelField(panel, 'label')), panel.title)
  const showLabel = () => label() !== panel.title
  const caption = () => (showLabel() ? label() : panel.title)
  const navigation = usePanelNavigation(panel)
  const href = () => navigation.cardURL(frame.data)
  const prefetch = usePrefetch(href, () => navigation.action, navigation.prefetchIdle)
  const loading = () => frame.isLoading && !frame.data
  let captionRef: HTMLParagraphElement | undefined
  const captionClamped = useIsClamped(() => captionRef)
  // The note the ⓘ carries is the producer's explanation, plus the caption when
  // — and only when — the strip's two-line slot cut it. What the clamp hides
  // has to be readable somewhere, and this affordance is already here,
  // keyboard-reachable and portalled; what the clamp did *not* hide is on the
  // card already and does not need saying twice.
  const note = () => [panel.info, captionClamped() ? panel.caption : undefined]
    .map((part) => part?.trim() ?? '')
    .filter(Boolean)
    .join('\n\n')
  const figure = () => loading()
    ? <span aria-hidden="true" class="lens-shimmer lens-stat-metric-value-shimmer" />
    : frame.error && !frame.data ? '—' : <StatValueTicker text={formatValue(cell(frame.data, valueField))} />

  return (
    <StatLink href={href()} label={caption()} onClick={navigation.onClick(href())} prefetch={prefetch}>
      {/* A refetch in flight is stated on the figures, not only on the button
          that started it. The card form learned this (`.lens-panel-stale` plus
          an «Updating» chip) and the chrome-free metric form did not, so during
          a recompute every KPI in the strip — the numbers a reader is most
          likely to act on — sat at full opacity with values that were about to
          change. */}
      <div
        aria-busy={frame.isLoading || (frame.isStale && !frame.error) || undefined}
        class={`lens-stat-metric${frame.isStale && !loading() ? ' lens-stat-metric-stale' : ''}`}
        data-panel-kind="stat"
        data-stale={frame.isStale || undefined}
      >
        <p class="lens-stat-metric-label" title={caption()}>
          <span class="lens-stat-metric-label-text">{caption()}</span>
          <Show when={panel.status}><StatusChip status={panel.status!} /></Show>
          {/* The compact form drops the card header, and with it the ⓘ that
            explains how a figure is obtained. A metric that carries that note
            keeps it here, next to the name it belongs to; the caption below
            stays visible prose. */}
          <Show when={note()}><InfoTip inline subject={caption()} text={note()} /></Show>
        </p>
        <div class="lens-stat-metric-main">
          <p class="lens-stat-metric-value" title={formatValueExact(cell(frame.data, valueField))}>{figure()}</p>
          <Show when={href()}><StatDrillMark /></Show>
          {/* A metric that carries a wire sparkline shows it inline to the right
            of the value, echoing the hero card's trend line. */}
          <Show when={panel.sparkline}>
            <StatSparkline sparkline={panel.sparkline!} tone={trendTone(panel, frame.data)} />
          </Show>
        </div>
        <div class="lens-stat-metric-delta">
          <Show when={panel.trend && frame.data?.rows.length}>
            <TrendChip panel={panel} frame={frame.data} />
          </Show>
        </div>
        {/* Both trailing slots are always in the DOM and collapse when empty.
          A strip where one card carries a note and its neighbour does not used
          to end up two different heights; now the slot is reserved for the
          whole strip or for none of it (see the `:has` rules in the sheet). */}
        <p class="lens-stat-metric-caption" ref={(el) => { captionRef = el }}>{panel.caption}</p>
      </div>
    </StatLink>
  )
}
