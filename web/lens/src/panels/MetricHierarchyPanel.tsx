import { useMemo, type CSSProperties } from 'react'
import type { Panel } from '../contract'
import { useFormat, usePanelFrame, useTranslate, type PanelFrameState } from '../runtime'
import { useElementActionResolver } from './actions'
import { buildKeyedJoin, panelField, type KeyedJoinMessages } from './data'
import { hierarchyRowViews, type HierarchyRowView } from './metricViews'
import { PanelFrame } from './PanelFrame'
import { QualityChip, resolveQuality } from './QualityChip'

export interface MetricHierarchyPanelProps {
  panel: Panel
}

function useMessages(): KeyedJoinMessages {
  const translate = useTranslate()
  return useMemo<KeyedJoinMessages>(() => ({
    missingColumn: (column) => translate('panel.missingColumn', 'Panel data is missing the “{column}” column.', { column }),
    duplicateKey: (key) => translate('panel.duplicateKey', 'Panel data has a duplicate key “{key}”.', { key }),
  }), [translate])
}

export function MetricHierarchyPanel({ panel }: MetricHierarchyPanelProps) {
  const frame = usePanelFrame(panel.id)
  const translate = useTranslate()
  const messages = useMessages()
  const valueField = panelField(panel, 'value') ?? 'value'
  const shareField = panelField(panel, 'share')
  const formatValue = useFormat(panel.format[valueField])
  const formatShare = useFormat(shareField ? panel.format[shareField] : undefined)
  const resolveAction = useElementActionResolver()

  const join = useMemo(
    () => (frame.data ? buildKeyedJoin(panel, frame.data, messages) : undefined),
    [frame.data, messages, panel],
  )
  const contractError = join?.kind === 'contract-error' ? join.message : undefined
  const effectiveFrame: PanelFrameState = contractError
    ? { ...frame, data: undefined, error: new Error(contractError) }
    : frame

  const views: Array<HierarchyRowView> = useMemo(
    () => (join ? hierarchyRowViews(panel, join, formatValue, formatShare) : []),
    [formatShare, formatValue, join, panel],
  )

  const reconcileText = (summary: NonNullable<HierarchyRowView['reconcile']>): string => (
    summary.balanced
      ? translate('hierarchy.allocated', 'Allocated 100%')
      : translate('hierarchy.difference', 'Difference: {delta}', { delta: formatValue(summary.delta) })
  )

  return (
    <PanelFrame panel={panel} frame={effectiveFrame} allowEmptyContent>
      <ul className="lens-hierarchy">
        {views.map((view) => {
          const { row } = view
          const quality = resolveQuality({ confidence: view.confidence, availability: view.availability })
          const qualityLabel = quality ? translate(quality.meta.labelKey, quality.meta.fallback) : undefined
          const spokenValue = view.showDash ? (qualityLabel ?? translate('availability.unavailable', 'Unavailable')) : view.valueText
          const ariaLabel = `${row.label}, ${spokenValue}${view.shareText ? `, ${view.shareText}` : ''}`
          const target = resolveAction(row.action)
          const body = (
            <>
              <span className="lens-hierarchy-label">
                {view.depth > 0 && <span aria-hidden="true" className="lens-hierarchy-guide" />}
                <span className="lens-hierarchy-label-text" title={row.label}>{row.label}</span>
                {row.unallocated && (
                  <span className="lens-hierarchy-unallocated-tag">
                    {translate('hierarchy.unallocated', 'Unallocated')}
                  </span>
                )}
                {row.description && <span className="lens-hierarchy-description" title={row.description}>{row.description}</span>}
              </span>
              <span className="lens-hierarchy-value">
                {view.showDash ? '—' : view.valueText}
                <QualityChip confidence={view.confidence} availability={view.availability} className="lens-hierarchy-chip" />
              </span>
              {view.shareText !== undefined && <span className="lens-hierarchy-share">{view.shareText}</span>}
            </>
          )
          return (
            <li
              aria-current={row.selected ? 'true' : undefined}
              className={[
                'lens-hierarchy-row',
                view.isParent ? 'lens-hierarchy-row-parent' : '',
                row.unallocated ? 'lens-hierarchy-row-unallocated' : '',
                row.selected ? 'lens-hierarchy-row-selected' : '',
              ].filter(Boolean).join(' ')}
              key={row.key}
              style={{ '--lens-depth': view.depth } as CSSProperties}
            >
              {target ? (
                <a
                  aria-label={ariaLabel}
                  className="lens-hierarchy-row-inner lens-hierarchy-row-link"
                  href={target.href}
                  onClick={target.onClick}
                >
                  {body}
                </a>
              ) : (
                <div aria-label={ariaLabel} className="lens-hierarchy-row-inner" role="group">{body}</div>
              )}
              {view.reconcile && (
                <p
                  className={`lens-hierarchy-reconcile${view.reconcile.balanced ? ' lens-hierarchy-reconcile-balanced' : ' lens-hierarchy-reconcile-off'}`}
                >
                  {reconcileText(view.reconcile)}
                </p>
              )}
            </li>
          )
        })}
      </ul>
    </PanelFrame>
  )
}
