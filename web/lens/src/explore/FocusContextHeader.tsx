import { Fragment } from 'react'
import type { FieldFormat } from '../contract'
import { CaretRight } from '../icons'
import { StatusChip } from '../panels/StatPanel'
import { StatValueTicker } from '../panels/StatValueTicker'
import { useFormat, useTranslate } from '../runtime'
import { ContextMiniChart } from './ContextMiniChart'
import type { FocusContext } from './focusModel'
import type { ExploreBreadcrumb } from './model'

/**
 * The always-visible context block of a focus canvas: the breadcrumb back to
 * the metric's ancestors, the metric name with its counting value, the share
 * of the parent, the reporting period, the level's quality status, and the
 * parent mini-chart with the focused element highlighted.
 */
export interface FocusContextHeaderProps {
  context: FocusContext
  breadcrumbs: Array<ExploreBreadcrumb>
  valueFormat?: FieldFormat
  colorFor: (label: string, index: number) => string | undefined
  periodLabel?: string
  onCrumb: (pathIndex: number) => void
  onParent?: () => void
}

export function FocusContextHeader({
  context, breadcrumbs, valueFormat, colorFor, periodLabel, onCrumb, onParent,
}: FocusContextHeaderProps) {
  const translate = useTranslate()
  const formatValue = useFormat(valueFormat)
  const formatShare = useFormat({ kind: 'percent', minorUnits: false, precision: 1, decimalSeparator: '.' })
  const collapsible = breadcrumbs.length > 3
  const share = context.share !== undefined && context.total !== undefined
    ? translate('explore.shareOfTotal', '{share} of {total}', {
      share: formatShare(context.share * 100),
      total: formatValue(context.total),
    })
    : undefined

  return (
    <div className="lens-focus-header">
      {breadcrumbs.length > 1 && (
        <nav aria-label={translate('focus.path', 'Drill path')} className="lens-focus-breadcrumb">
          <ol>
            {breadcrumbs.map((crumb, index) => {
              // In a narrow container the middle of a long path collapses to a
              // static ellipsis: the first crumb keeps the origin, the last two
              // keep the immediate context, and the overlay still has the rest.
              const middle = collapsible && index > 0 && index < breadcrumbs.length - 2
              return (
                <Fragment key={`${crumb.label}-${crumb.pathIndex}`}>
                  {collapsible && index === breadcrumbs.length - 2 && (
                    <li aria-hidden="true" className="lens-focus-crumb-gap">
                      <CaretRight className="lens-focus-crumb-sep" size={12} />
                      <span>…</span>
                    </li>
                  )}
                  <li className={middle ? 'lens-focus-crumb-middle' : undefined}>
                    {index > 0 && <CaretRight className="lens-focus-crumb-sep" size={12} />}
                    {crumb.current ? (
                      <span aria-current="page" className="lens-focus-crumb lens-focus-crumb-current">
                        {crumb.label}
                      </span>
                    ) : (
                      <button className="lens-focus-crumb" onClick={() => onCrumb(crumb.pathIndex)} type="button">
                        {crumb.label}
                      </button>
                    )}
                  </li>
                </Fragment>
              )
            })}
          </ol>
        </nav>
      )}
      <div className="lens-focus-context">
        <div className="lens-focus-context-body">
          <span className="lens-focus-name">{context.label}</span>
          {context.value !== undefined && (
            <span className="lens-focus-value">
              <StatValueTicker text={formatValue(context.value)} />
            </span>
          )}
          {share && <span className="lens-focus-share">{share}</span>}
          {periodLabel && <span className="lens-focus-period">{periodLabel}</span>}
          {context.status && <StatusChip status={context.status} />}
        </div>
        {context.parent && (
          <ContextMiniChart
            caption={onParent ? translate('focus.toParent', 'To parent') : undefined}
            centerLabel={context.share !== undefined ? `${Math.round(context.share * 100)}%` : undefined}
            colorFor={colorFor}
            label={translate('focus.backToParent', 'Back to {name}', { name: context.parent.label })}
            onClick={onParent}
            parent={context.parent}
          />
        )}
      </div>
    </div>
  )
}
