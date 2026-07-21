import type { MouseEventHandler, ReactNode } from 'react'
import type { Panel } from '../contract'
import { useFormat, usePanelFrame, useTranslate } from '../runtime'
import { ArrowUpRight } from '../icons'
import { usePanelNavigation } from './actions'
import { cell, displayText, panelField } from './data'
import { PanelFrame } from './PanelFrame'

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
    formatDelta,
    delta,
    deltaNumber: numeric(delta),
  }
}

/**
 * A stat card that carries a panel-level navigate action is a link in full:
 * the legacy renderer covered the card with an absolutely positioned anchor,
 * and losing it is what made the KPI strips inert.
 */
export function StatLink({ href, label, children, onClick }: {
  href?: string
  label: string
  children: ReactNode
  onClick?: MouseEventHandler<HTMLAnchorElement>
}) {
  const translate = useTranslate()
  if (!href) return <>{children}</>
  return (
    <div className="lens-stat-linked">
      <a aria-label={translate('panel.openMetric', 'Open {name}', { name: label })} className="lens-card-link" href={href} onClick={onClick}>
        <span aria-hidden="true" className="lens-card-link-affordance"><ArrowUpRight /></span>
      </a>
      {children}
    </div>
  )
}

export function StatPanel({ panel }: StatPanelProps) {
  const { frame, label, showLabel, value, formatValue, formatDelta, delta, deltaNumber } = useStatValues(panel)
  const navigation = usePanelNavigation(panel)
  const href = navigation.cardURL(frame.data)

  return (
    <PanelFrame panel={panel} frame={frame} variant="stat">
      <StatLink href={href} label={panel.title} onClick={navigation.onClick(href)}>
      <div className="lens-stat-content">
        {(showLabel || panel.status) && (
          <p className="lens-stat-label">
            {showLabel && <span>{label}</span>}
            {panel.status && <StatusChip status={panel.status} />}
          </p>
        )}
        <div className="lens-stat-value-row">
          <p className="lens-stat-value">{formatValue(value)}</p>
          {delta !== undefined && (
            <span className={`lens-stat-delta${deltaNumber !== undefined && deltaNumber < 0 ? ' lens-stat-delta-negative' : ''}`}>
              {deltaNumber !== undefined && deltaNumber > 0 ? '+' : ''}{formatDelta(delta)}
            </span>
          )}
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
  const { frame, label, showLabel, value, formatValue } = useStatValues(panel)
  const caption = showLabel ? label : panel.title
  const navigation = usePanelNavigation(panel)
  const href = navigation.cardURL(frame.data)

  return (
    <StatLink href={href} label={caption} onClick={navigation.onClick(href)}>
    <div className="lens-stat-metric" data-panel-kind="stat" aria-busy={frame.isLoading || undefined}>
      <p className="lens-stat-metric-label" title={caption}>
        {panel.accent && <span aria-hidden="true" className="lens-stat-metric-bullet" style={{ background: panel.accent }} />}
        <span className="lens-stat-metric-label-text">{caption}</span>
        {panel.status && <StatusChip status={panel.status} />}
      </p>
      <p className="lens-stat-metric-value">
        {frame.error && !frame.data ? '—' : formatValue(value)}
      </p>
    </div>
    </StatLink>
  )
}
