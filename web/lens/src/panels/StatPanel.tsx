import type { MouseEventHandler, ReactNode } from 'react'
import type { Panel, Sparkline } from '../contract'
import { useFormat, useFormatExact, usePanelFrame, useTranslate } from '../runtime'
import { ArrowUpRight } from '../icons'
import { usePanelNavigation, usePrefetch, type PrefetchHandlers } from './actions'
import { StatValueTicker } from './StatValueTicker'
import { cell, displayText, panelField } from './data'
import { InfoTip } from './InfoTip'
import { PanelFrame } from './PanelFrame'
import { TrendChip } from './PanelFrame'

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
export function StatSparkline({ sparkline }: { sparkline: Sparkline }) {
  const width = 44
  const height = 18
  const values = sparkline.values.filter((value) => Number.isFinite(value))
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
  const color = sparkline.color?.trim() || 'var(--lens-accent-500)'
  return (
    <svg
      aria-hidden="true"
      className="lens-stat-sparkline"
      height={height}
      viewBox={`0 0 ${width} ${height}`}
      width={width}
    >
      <polyline
        fill="none"
        opacity={0.8}
        points={points.join(' ')}
        strokeWidth={1.25}
        style={{ stroke: color }}
      />
      <circle cx={lastX} cy={lastY} r={1.8} style={{ fill: color }} />
    </svg>
  )
}

export function StatusChip({ status }: { status: NonNullable<Panel['status']> }) {
  return (
    <span
      className={`lens-status-chip ${status.tone === 'positive'
        ? 'lens-status-chip-positive'
        : status.tone === 'warning' ? 'lens-status-chip-warning' : 'lens-status-chip-neutral'}`}
    >
      {status.label}
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
  // The dataset may repeat the panel title in its label column; only a label
  // that says something the header does not is worth a second line.
  const label = displayText(cell(frame.data, panelField(panel, 'label')), panel.title)
  const delta = deltaField ? cell(frame.data, deltaField) : undefined
  return {
    frame,
    label,
    showLabel: label !== panel.title,
    value: cell(frame.data, valueField),
    formatValue,
    formatValueExact,
    formatDelta,
    delta,
    deltaNumber: numeric(delta),
  }
}

/**
 * A stat card that carries a panel-level navigate action is a link in full.
 */
export function StatLink({ href, label, children, onClick, prefetch }: {
  href?: string
  label: string
  children: ReactNode
  onClick?: MouseEventHandler<HTMLAnchorElement>
  prefetch?: PrefetchHandlers
}) {
  const translate = useTranslate()
  if (!href) return <>{children}</>
  return (
    <div className="lens-stat-linked">
      <a
        aria-label={translate('panel.openMetric', 'Open {name}', { name: label })}
        className="lens-card-link"
        href={href}
        onClick={onClick}
        {...prefetch}
      >
        <span aria-hidden="true" className="lens-card-link-affordance"><ArrowUpRight /></span>
      </a>
      {children}
    </div>
  )
}

export function StatPanel({ panel }: StatPanelProps) {
  const { frame, label, showLabel, value, formatValue, formatValueExact, formatDelta, delta, deltaNumber } = useStatValues(panel)
  const navigation = usePanelNavigation(panel)
  const href = navigation.cardURL(frame.data)
  const prefetch = usePrefetch(href, navigation.action)

  return (
    <PanelFrame panel={panel} frame={frame} variant="stat">
      <StatLink href={href} label={panel.title} onClick={navigation.onClick(href)} prefetch={prefetch}>
      <div className="lens-stat-content">
        {(showLabel || panel.status) && (
          <p className="lens-stat-label">
            {showLabel && <span className="lens-stat-label-text" title={label}>{label}</span>}
            {panel.status && <StatusChip status={panel.status} />}
          </p>
        )}
        <div className="lens-stat-value-row">
          {/* The abbreviated value keeps its exact grouped figure reachable
              on hover: «106.03 млрд UZS» titles «106 034 767 694 UZS». */}
          <p className="lens-stat-value" title={formatValueExact(value)}><StatValueTicker text={formatValue(value)} /></p>
          {delta !== undefined && (
            <span className={`lens-stat-delta${deltaNumber !== undefined && deltaNumber < 0 ? ' lens-stat-delta-negative' : ''}`}>
              {deltaNumber !== undefined && deltaNumber > 0 ? '+' : ''}{formatDelta(delta)}
            </span>
          )}
          {panel.sparkline && <StatSparkline sparkline={panel.sparkline} />}
        </div>
      </div>
      </StatLink>
    </PanelFrame>
  )
}

/**
 * StatMetric is the chrome-free form of a stat panel used inside a metrics
 * group card: an accent bullet, a truncated uppercase label with an optional
 * status chip, and a compact value.
 */
export function StatMetric({ panel }: StatPanelProps) {
  const { frame, label, showLabel, value, formatValue, formatValueExact } = useStatValues(panel)
  const caption = showLabel ? label : panel.title
  const navigation = usePanelNavigation(panel)
  const href = navigation.cardURL(frame.data)
  const prefetch = usePrefetch(href, navigation.action)

  return (
    <StatLink href={href} label={caption} onClick={navigation.onClick(href)} prefetch={prefetch}>
    <div className="lens-stat-metric" data-panel-kind="stat" aria-busy={frame.isLoading || undefined}>
      <p className="lens-stat-metric-label" title={caption}>
        {panel.accent && <span aria-hidden="true" className="lens-stat-metric-bullet" style={{ background: panel.accent }} />}
        <span className="lens-stat-metric-label-text">{caption}</span>
        {panel.status && <StatusChip status={panel.status} />}
        {/* The compact form drops the card header, and with it the ⓘ that
            explains how a figure is obtained. A metric that carries that note
            keeps it here, next to the name it belongs to; the caption below
            stays visible prose. */}
        {panel.info && <InfoTip inline text={panel.info} />}
      </p>
      {/* A metric that carries a wire sparkline shows it inline to the right of
          the value, echoing the hero card's trend line; a metric without one
          keeps the bare value element so its layout stays pixel-identical. */}
      {panel.sparkline ? (
        <div className="lens-stat-metric-main">
          <p className="lens-stat-metric-value" title={formatValueExact(value)}>
            {frame.error && !frame.data ? '—' : <StatValueTicker text={formatValue(value)} />}
          </p>
          <StatSparkline sparkline={panel.sparkline} />
        </div>
      ) : (
        <p className="lens-stat-metric-value" title={formatValueExact(value)}>
          {frame.error && !frame.data ? '—' : <StatValueTicker text={formatValue(value)} />}
        </p>
      )}
	  {panel.trend && frame.data?.rows.length ? <TrendChip panel={panel} frame={frame.data} /> : null}
      {panel.caption && <p className="lens-stat-metric-caption">{panel.caption}</p>}
    </div>
    </StatLink>
  )
}
