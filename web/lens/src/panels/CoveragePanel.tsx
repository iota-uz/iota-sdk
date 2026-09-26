/* eslint-disable react/no-unknown-property -- Solid JSX uses `class`, the React-era rule expects `className`; the lint config migrates with the Solid port. */
import { createMemo, createSignal, For, Show } from 'solid-js'
import type { Frame, Panel } from '../contract'
import { useDashboard, useFormat, usePanelFrame, useTranslate } from '../runtime'
import { usePanelNavigation } from './actions'
import { colorLabels, columnIndex, displayText, panelField, seriesColorResolver } from './data'
import { PanelFrame } from './PanelFrame'
import { StatLink } from './StatPanel'

/* eslint-disable react-refresh/only-export-components */
export interface CoveragePanelProps {
  panel: Panel
}

export interface CoverageSegment {
  key: string
  label: string
  value: number
  share: number
  color?: string
}

function numeric(value: unknown): number {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim()) {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return 0
}

export function buildCoverageSegments(
  panel: Panel,
  frame: Frame,
  seriesColor: (label: string, index: number) => string | undefined,
): {
    segments: CoverageSegment[]
    total: number
  } {
  const labelIndex = columnIndex(frame, panelField(panel, 'label') ?? panelField(panel, 'category') ?? 'label')
  const valueIndex = columnIndex(frame, panelField(panel, 'value') ?? 'value')
  const idIndex = columnIndex(frame, panelField(panel, 'id'))
  const values = frame.rows.map((row) => Math.max(0, numeric(row[valueIndex])))
  const total = values.reduce((sum, value) => sum + value, 0)
  const segments = frame.rows.map((row, index) => {
    const key = idIndex >= 0 ? displayText(row[idIndex], String(index)) : String(index)
    const value = values[index] ?? 0
    return {
      key,
      label: displayText(row[labelIndex], `#${index + 1}`),
      value,
      share: total > 0 ? value / total : 0,
      color: seriesColor(displayText(row[labelIndex], key), index),
    }
  })
  return { segments, total }
}

/**
 * Bullet-style variant of the coverage track, rendered when the panel carries
 * a `target`: the segments keep their proportional widths against a scale that
 * also fits the target, and a labelled tick marks the target value — the
 * measure-vs-goal reading (e.g. reserves against liquid assets) the plain
 * 100%-wide track cannot express.
 */
function CoverageBullet(props: {
  activeSegment?: string
  segments: CoverageSegment[]
  total: number
  target: NonNullable<Panel['target']>
  formatValue: (value: unknown) => string
  onSegmentEnter: (key: string) => void
  onSegmentLeave: (key: string) => void
}) {
  // A hair of headroom keeps a marker at the scale edge from clipping.
  const scaleMax = Math.max(props.total, props.target.value) * 1.04
  if (scaleMax <= 0) return null
  const percent = (value: number) => `${((value / scaleMax) * 100).toFixed(3)}%`
  const markerShare = props.target.value / scaleMax
  const markerLabel = [props.target.label?.trim(), props.formatValue(props.target.value)].filter(Boolean).join(' ')
  return (
    <div class="lens-coverage-bullet">
      <div class="lens-coverage-track" aria-hidden="true">
        <For each={props.segments}>
          {(segment) => (
            <Show when={segment.value > 0}>
              <span
                class="lens-coverage-track-segment"
                data-highlighted={props.activeSegment === segment.key || undefined}
                onPointerEnter={() => props.onSegmentEnter(segment.key)}
                onPointerLeave={() => props.onSegmentLeave(segment.key)}
                style={{ width: percent(segment.value), background: segment.color }}
              />
            </Show>
          )}
        </For>
      </div>
      <span
        aria-hidden="true"
        class="lens-coverage-bullet-marker"
        style={{ left: percent(props.target.value) }}
      />
      <Show when={markerLabel}>
        {/* The label hangs off the tick rather than straddling it: a centred
            label whose text is wider than twice the tick's offset spills past
            the card's left edge, where the panel clips it mid-word. Anchoring
            one edge to the tick and capping the width at the room actually
            available on that side keeps every label inside the track — long
            ones ellipsize (the full text stays in the title) instead of
            escaping. */}
        <span
          class={`lens-coverage-bullet-label${markerShare > 0.5 ? ' lens-coverage-bullet-label-end' : ''}`}
          style={markerShare > 0.5
            ? { right: percent(scaleMax - props.target.value), 'max-width': percent(props.target.value) }
            : { left: percent(props.target.value), 'max-width': percent(scaleMax - props.target.value) }}
          title={markerLabel}
        >
          {markerLabel}
        </span>
      </Show>
    </div>
  )
}

export function CoveragePanel(props: CoveragePanelProps) {
  const panel = props.panel
  const frame = usePanelFrame(panel.id)
  const translate = useTranslate()
  const valueField = panelField(panel, 'value') ?? 'value'
  const formatValue = useFormat(panel.format[valueField])
  const formatPercent = useFormat({ kind: 'percent', minorUnits: false, precision: 0 })
  const { document } = useDashboard()
  const coverage = createMemo(() => frame.data
    ? buildCoverageSegments(panel, frame.data, seriesColorResolver(document.theme, panel, { labels: colorLabels(frame.data, panel) }))
    : { segments: [] as CoverageSegment[], total: 0 })
  const headline = () => panel.headline ?? panel.total ?? coverage().total
  // A plain track needs at least two positive segments; a targeted bullet also
  // has the goal marker as a second reference, so one positive segment remains
  // meaningful there.
  const positiveCount = () => coverage().segments.reduce((count, segment) => count + (segment.value > 0 ? 1 : 0), 0)
  const showTrack = () => positiveCount() > 1 || (Boolean(panel.target) && positiveCount() > 0)
  // Legacy parity: a card-scoped action makes the whole card a link, a
  // row-scoped one makes each track segment and legend row its own link.
  const navigation = usePanelNavigation(panel)
  const cardHref = () => navigation.cardURL(frame.data)
  const segmentHref = (index: number) => (
    navigation.rowScoped ? navigation.urlForRow(frame.data, frame.data?.rows[index]) : undefined
  )
  const [activeSegment, setActiveSegment] = createSignal<string>()
  const highlightSegment = (key: string) => setActiveSegment(key)
  const clearSegment = (key: string) => setActiveSegment((current) => current === key ? undefined : current)

  return (
    <PanelFrame panel={panel} frame={frame}>
      <StatLink href={cardHref()} label={panel.title} onClick={navigation.onClick(cardHref())}>
        <div class="lens-coverage" data-segment-active={activeSegment() ? 'true' : undefined}>
          <p class="lens-coverage-headline">
            <span class="lens-coverage-headline-value">{formatValue(headline)}</span>
            <span class="lens-coverage-headline-label">{translate('panel.total', 'Total')}</span>
          </p>
          {/* The segments answer a pointer by highlighting their legend row, and
              the row states the label, the amount and the share in the sheet's
              own type. They used to carry a native `title` saying two of those
              three as well: a second answer, a second later, in the operating
              system's styling — and on a track the panel marks `aria-hidden`,
              so a screen reader never had it at all. A native tooltip is kept
              in this runtime for text the layout clips, not as a data channel
              beside one that is already on screen. */}
          {showTrack() && !panel.target && (
            <div class="lens-coverage-track" aria-hidden={navigation.rowScoped || undefined} aria-label={navigation.rowScoped ? undefined : panel.title} role={navigation.rowScoped ? undefined : 'img'}>
              <For each={coverage().segments}>
                {(segment) => (
                  <Show when={segment.value > 0}>
                    <span
                      class="lens-coverage-track-segment"
                      data-highlighted={activeSegment() === segment.key || undefined}
                      onPointerEnter={() => highlightSegment(segment.key)}
                      onPointerLeave={() => clearSegment(segment.key)}
                      style={{ width: `${segment.share * 100}%`, background: segment.color }}
                    />
                  </Show>
                )}
              </For>
            </div>
          )}
          {showTrack() && panel.target && (
            <CoverageBullet
              activeSegment={activeSegment()}
              formatValue={formatValue}
              onSegmentEnter={highlightSegment}
              onSegmentLeave={clearSegment}
              segments={coverage().segments}
              target={panel.target}
              total={coverage().total}
            />
          )}
          <ul class="lens-coverage-legend">
            <For each={coverage().segments}>
              {(segment, index) => {
                const href = segmentHref(index())
                const content = (
                  <>
                    <span aria-hidden="true" class="lens-coverage-legend-bullet" style={{ background: segment.color }} />
                    {/* The one native tooltip this panel keeps, and it says nothing
                        the row does not already print: the label is truncated to
                        keep the value and share columns aligned, so this is the
                        full name for the readers that clip costs it. */}
                    <span class="lens-coverage-legend-label" title={segment.label}>{segment.label}</span>
                    <span class="lens-coverage-legend-value">{formatValue(segment.value)}</span>
                    <span class="lens-coverage-legend-share">{formatPercent(segment.share * 100)}</span>
                  </>
                )
                return (
                  <li
                    class="lens-coverage-legend-row"
                    data-highlighted={activeSegment() === segment.key || undefined}
                    onBlur={() => clearSegment(segment.key)}
                    onFocus={() => highlightSegment(segment.key)}
                    onPointerEnter={() => highlightSegment(segment.key)}
                    onPointerLeave={() => clearSegment(segment.key)}
                  >
                    <Show when={href} fallback={content}>
                      <a class="lens-coverage-legend-link" href={href} onClick={navigation.onClick(href)}>{content}</a>
                    </Show>
                  </li>
                )
              }}
            </For>
          </ul>
        </div>
      </StatLink>
    </PanelFrame>
  )
}
