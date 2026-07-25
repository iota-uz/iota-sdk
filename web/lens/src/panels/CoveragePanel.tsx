import { useMemo } from 'react'
import type { Frame, Panel } from '../contract'
import { useDashboard, useFormat, usePanelFrame, useTranslate } from '../runtime'
import { usePanelNavigation } from './actions'
import { columnIndex, displayText, panelField, seriesColorResolver } from './data'
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
function CoverageBullet({ panel, segments, total, target, tooltip, segmentHref, navigation, formatValue }: {
  panel: Panel
  segments: CoverageSegment[]
  total: number
  target: NonNullable<Panel['target']>
  tooltip: (segment: CoverageSegment) => string
  segmentHref: (index: number) => string | undefined
  navigation: ReturnType<typeof usePanelNavigation>
  formatValue: (value: unknown) => string
}) {
  // A hair of headroom keeps a marker at the scale edge from clipping.
  const scaleMax = Math.max(total, target.value) * 1.04
  if (scaleMax <= 0) return null
  const percent = (value: number) => `${((value / scaleMax) * 100).toFixed(3)}%`
  const markerShare = target.value / scaleMax
  const markerLabel = [target.label?.trim(), formatValue(target.value)].filter(Boolean).join(' ')
  return (
    <div className="lens-coverage-bullet">
      <div className="lens-coverage-track" aria-label={panel.title} role={navigation.rowScoped ? 'group' : 'img'}>
        {segments.map((segment, index) => segment.value > 0 && (
          segmentHref(index)
            ? (
              <a
                aria-label={tooltip(segment)}
                className="lens-coverage-track-segment lens-coverage-track-segment-link"
                href={segmentHref(index)}
                onClick={navigation.onClick(segmentHref(index))}
                key={segment.key}
                style={{ width: percent(segment.value), background: segment.color }}
                title={tooltip(segment)}
              />
            )
            : (
              <span
                className="lens-coverage-track-segment"
                key={segment.key}
                style={{ width: percent(segment.value), background: segment.color }}
                title={tooltip(segment)}
              />
            )
        ))}
      </div>
      <span
        aria-hidden="true"
        className="lens-coverage-bullet-marker"
        style={{ left: percent(target.value) }}
      />
      {markerLabel && (
        <span
          className={`lens-coverage-bullet-label${markerShare > 0.55 ? ' lens-coverage-bullet-label-end' : ''}`}
          style={{ left: percent(target.value) }}
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
      ? buildCoverageSegments(panel, frame.data, seriesColorResolver(document.theme, panel))
      : { segments: [], total: 0 },
    [document.theme, frame.data, panel],
  )
  const headline = panel.headline ?? panel.total ?? total
  // A meaningful track needs at least two positive segments; a lone 100%
  // segment is a full bar that says nothing, so the card degrades to its
  // headline plus legend rows (e.g. claims entirely «within reserve»).
  const positiveCount = segments.reduce((count, segment) => count + (segment.value > 0 ? 1 : 0), 0)
  const showTrack = positiveCount > 1
  // Legacy parity: a card-scoped action makes the whole card a link, a
  // row-scoped one makes each track segment and legend row its own link.
  const navigation = usePanelNavigation(panel)
  const cardHref = navigation.cardURL(frame.data)
  const segmentHref = (index: number) => (
    navigation.rowScoped ? navigation.urlForRow(frame.data, frame.data?.rows[index]) : undefined
  )
  const tooltip = (segment: CoverageSegment) => `${segment.label}: ${formatValue(segment.value)}`

  return (
    <PanelFrame panel={panel} frame={frame}>
      <StatLink href={cardHref} label={panel.title} onClick={navigation.onClick(cardHref)}>
      <div className="lens-coverage">
        <p className="lens-coverage-headline">
          <span className="lens-coverage-headline-value">{formatValue(headline)}</span>
          <span className="lens-coverage-headline-label">{translate('panel.total', 'Total')}</span>
        </p>
        {showTrack && !panel.target && (
          <div className="lens-coverage-track" aria-label={panel.title} role={navigation.rowScoped ? 'group' : 'img'}>
            {segments.map((segment, index) => segment.value > 0 && (
              segmentHref(index)
                ? (
                  <a
                    aria-label={tooltip(segment)}
                    className="lens-coverage-track-segment lens-coverage-track-segment-link"
                    href={segmentHref(index)}
                    onClick={navigation.onClick(segmentHref(index))}
                    key={segment.key}
                    style={{ width: `${segment.share * 100}%`, background: segment.color }}
                    title={tooltip(segment)}
                  />
                )
                : (
                  <span
                    className="lens-coverage-track-segment"
                    key={segment.key}
                    style={{ width: `${segment.share * 100}%`, background: segment.color }}
                    title={tooltip(segment)}
                  />
                )
            ))}
          </div>
        )}
        {showTrack && panel.target && (
          <CoverageBullet
            formatValue={formatValue}
            navigation={navigation}
            panel={panel}
            segmentHref={segmentHref}
            segments={segments}
            target={panel.target}
            tooltip={tooltip}
            total={total}
          />
        )}
        <ul className="lens-coverage-legend">
          {segments.map((segment, index) => {
            const href = segmentHref(index)
            const content = (
              <>
                <span aria-hidden="true" className="lens-coverage-legend-bullet" style={{ background: segment.color }} />
                <span className="lens-coverage-legend-label">{segment.label}</span>
                <span className="lens-coverage-legend-value">{formatValue(segment.value)}</span>
                <span className="lens-coverage-legend-share">{formatPercent(segment.share * 100)}</span>
              </>
            )
            return (
              <li className="lens-coverage-legend-row" key={segment.key}>
                {href
                  ? <a className="lens-coverage-legend-link" href={href} onClick={navigation.onClick(href)} title={tooltip(segment)}>{content}</a>
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
