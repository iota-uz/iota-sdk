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
        {showTrack && (
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
