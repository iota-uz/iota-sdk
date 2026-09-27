import { useMemo, useState } from 'react'
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
function CoverageBullet({ activeSegment, formatValue, onSegmentEnter, onSegmentLeave, segments, total, target }: {
  activeSegment?: string
  segments: CoverageSegment[]
  total: number
  target: NonNullable<Panel['target']>
  formatValue: (value: unknown) => string
  onSegmentEnter: (key: string) => void
  onSegmentLeave: (key: string) => void
}) {
  // A hair of headroom keeps a marker at the scale edge from clipping.
  const scaleMax = Math.max(total, target.value) * 1.04
  if (scaleMax <= 0) return null
  const percent = (value: number) => `${((value / scaleMax) * 100).toFixed(3)}%`
  const markerShare = target.value / scaleMax
  const markerLabel = [target.label?.trim(), formatValue(target.value)].filter(Boolean).join(' ')
  return (
    <div className="lens-coverage-bullet">
      <div className="lens-coverage-track" aria-hidden="true">
        {segments.map((segment) => segment.value > 0 && (
          <span
            className="lens-coverage-track-segment"
            data-highlighted={activeSegment === segment.key || undefined}
            key={segment.key}
            onPointerEnter={() => onSegmentEnter(segment.key)}
            onPointerLeave={() => onSegmentLeave(segment.key)}
            style={{ width: percent(segment.value), background: segment.color }}
          />
        ))}
      </div>
      <span
        aria-hidden="true"
        className="lens-coverage-bullet-marker"
        style={{ left: percent(target.value) }}
      />
      {markerLabel && (
        // The label hangs off the tick rather than straddling it: a centred
        // label whose text is wider than twice the tick's offset spills past
        // the card's left edge, where the panel clips it mid-word. Anchoring
        // one edge to the tick and capping the width at the room actually
        // available on that side keeps every label inside the track — long
        // ones ellipsize (the full text stays in the title) instead of
        // escaping.
        <span
          className={`lens-coverage-bullet-label${markerShare > 0.5 ? ' lens-coverage-bullet-label-end' : ''}`}
          style={markerShare > 0.5
            ? { right: percent(scaleMax - target.value), maxWidth: percent(target.value) }
            : { left: percent(target.value), maxWidth: percent(scaleMax - target.value) }}
          title={markerLabel}
        >
          {markerLabel}
        </span>
      )}
    </div>
  )
}

export function CoveragePanel({ panel }: CoveragePanelProps) {
  const frame = usePanelFrame(panel.id)
  const translate = useTranslate()
  const valueField = panelField(panel, 'value') ?? 'value'
  const formatValue = useFormat(panel.format[valueField])
  const formatPercent = useFormat({ kind: 'percent', minorUnits: false, precision: 0 })
  const { document } = useDashboard()
  const { segments, total } = useMemo(
    () => frame.data
      ? buildCoverageSegments(panel, frame.data, seriesColorResolver(document.theme, panel, { labels: colorLabels(frame.data, panel) }))
      : { segments: [], total: 0 },
    [document.theme, frame.data, panel],
  )
  const headline = panel.headline ?? panel.total ?? total
  // A plain track needs at least two positive segments; a targeted bullet also
  // has the goal marker as a second reference, so one positive segment remains
  // meaningful there.
  const positiveCount = segments.reduce((count, segment) => count + (segment.value > 0 ? 1 : 0), 0)
  const showTrack = positiveCount > 1 || (Boolean(panel.target) && positiveCount > 0)
  // Legacy parity: a card-scoped action makes the whole card a link, a
  // row-scoped one makes each track segment and legend row its own link.
  const navigation = usePanelNavigation(panel)
  const cardHref = navigation.cardURL(frame.data)
  const segmentHref = (index: number) => (
    navigation.rowScoped ? navigation.urlForRow(frame.data, frame.data?.rows[index]) : undefined
  )
  const [activeSegment, setActiveSegment] = useState<string>()
  const highlightSegment = (key: string) => setActiveSegment(key)
  const clearSegment = (key: string) => setActiveSegment((current) => current === key ? undefined : current)

  return (
    <PanelFrame panel={panel} frame={frame}>
      <StatLink href={cardHref} label={panel.title} onClick={navigation.onClick(cardHref)}>
        <div className="lens-coverage" data-segment-active={activeSegment ? 'true' : undefined}>
          <p className="lens-coverage-headline">
            <span className="lens-coverage-headline-value">{formatValue(headline)}</span>
            <span className="lens-coverage-headline-label">{translate('panel.total', 'Total')}</span>
          </p>
          {/* The segments answer a pointer by highlighting their legend row, and
              the row states the label, the amount and the share in the sheet's
              own type. They used to carry a native `title` saying two of those
              three as well: a second answer, a second later, in the operating
              system's styling — and on a track the panel marks `aria-hidden`,
              so a screen reader never had it at all. A native tooltip is kept
              in this runtime for text the layout clips, not as a data channel
              beside one that is already on screen. */}
          {showTrack && !panel.target && (
            <div className="lens-coverage-track" aria-hidden={navigation.rowScoped || undefined} aria-label={navigation.rowScoped ? undefined : panel.title} role={navigation.rowScoped ? undefined : 'img'}>
              {segments.map((segment) => segment.value > 0 && (
                <span
                  className="lens-coverage-track-segment"
                  data-highlighted={activeSegment === segment.key || undefined}
                  key={segment.key}
                  onPointerEnter={() => highlightSegment(segment.key)}
                  onPointerLeave={() => clearSegment(segment.key)}
                  style={{ width: `${segment.share * 100}%`, background: segment.color }}
                />
              ))}
            </div>
          )}
          {showTrack && panel.target && (
            <CoverageBullet
              activeSegment={activeSegment}
              formatValue={formatValue}
              onSegmentEnter={highlightSegment}
              onSegmentLeave={clearSegment}
              segments={segments}
              target={panel.target}
              total={total}
            />
          )}
          <ul className="lens-coverage-legend">
            {segments.map((segment, index) => {
              const href = segmentHref(index)
              const content = (
                <>
                  <span aria-hidden="true" className="lens-coverage-legend-bullet" style={{ background: segment.color }} />
                  {/* The one native tooltip this panel keeps, and it says nothing
                      the row does not already print: the label is truncated to
                      keep the value and share columns aligned, so this is the
                      full name for the readers that clip costs it. */}
                  <span className="lens-coverage-legend-label" title={segment.label}>{segment.label}</span>
                  <span className="lens-coverage-legend-value">{formatValue(segment.value)}</span>
                  <span className="lens-coverage-legend-share">{formatPercent(segment.share * 100)}</span>
                </>
              )
              return (
                <li
                  className="lens-coverage-legend-row"
                  data-highlighted={activeSegment === segment.key || undefined}
                  key={segment.key}
                  onBlur={() => clearSegment(segment.key)}
                  onFocus={() => highlightSegment(segment.key)}
                  onPointerEnter={() => highlightSegment(segment.key)}
                  onPointerLeave={() => clearSegment(segment.key)}
                >
                  {href
                    ? <a className="lens-coverage-legend-link" href={href} onClick={navigation.onClick(href)}>{content}</a>
                    : content}
                </li>
              )
            })}
          </ul>
        </div>
      </StatLink>
    </PanelFrame>
  )
}
