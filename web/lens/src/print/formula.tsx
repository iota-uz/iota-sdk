import { useMemo, type CSSProperties } from 'react'
import type { Panel } from '../contract'
import { buildKeyedJoin, type KeyedJoin } from '../panels/data'
import {
  connectorGlyphs,
  flowReconcileDelta,
  flowStageViews,
  hierarchyRowViews,
  relationshipEndView,
  relationshipTypeFallback,
} from '../panels/metricViews'
import { relationshipSentence } from '../panels/MetricRelationshipPanel'
import { formatFieldValue, useTranslate } from '../runtime'
import type { PrintSection } from '../runtime/print'
import { PrintQualityChip } from './quality'
import { sectionPanel } from './values'

/**
 * The three metric panels, printed as what they are.
 *
 * On screen a ratio is a formula: an operator per stage, a running result, a
 * reconciliation note when the parts miss the whole. Printing them through the
 * generic evidence table threw the grammar away and left five one-row tables
 * called «Итог», «Числитель — Итог», «Числитель — Разбивка» — the arithmetic
 * survived, the argument did not. These forms print the argument.
 */

function useJoin(panel: Panel, section: PrintSection): KeyedJoin | undefined {
  const translate = useTranslate()
  return useMemo(() => {
    if (!section.frame) return undefined
    return buildKeyedJoin(panel, section.frame, {
      missingColumn: (column) => translate('panel.missingColumn', 'Panel data is missing the “{column}” column.', { column }),
      duplicateKey: (key) => translate('panel.duplicateKey', 'Panel data has a duplicate key “{key}”.', { key }),
    })
  }, [panel, section.frame, translate])
}

function useValueFormatter(panel: Panel, locale: string, role: 'value' | 'share' = 'value') {
  const field = panel.encoding[role]
  return useMemo(
    () => (value: number) => formatFieldValue(value, field ? panel.format[field] : undefined, locale),
    [field, locale, panel.format],
  )
}

function FlowFormula({ join, panel, locale }: { join: KeyedJoin; panel: Panel; locale: string }) {
  const translate = useTranslate()
  const formatValue = useValueFormatter(panel, locale)
  const views = flowStageViews(panel, join, formatValue)
  const delta = flowReconcileDelta(panel, join)
  if (views.length === 0) return null
  return (
    <div className="lens-print-formula">
      <ol className="lens-print-flow">
        {views.map((view, index) => (
          <li
            className="lens-print-flow-stage"
            data-result={view.role === 'result' || undefined}
            key={`${view.stage.key}-${index}`}
          >
            <span className="lens-print-flow-op">{view.operator}</span>
            <span className="lens-print-flow-label">
              {view.stage.label}
              {view.stage.caption && <small>{view.stage.caption}</small>}
            </span>
            <span className="lens-print-flow-value">
              {view.showDash ? '—' : view.text}
              <PrintQualityChip confidence={view.confidence} availability={view.availability} />
            </span>
          </li>
        ))}
      </ol>
      {delta !== undefined && (
        <p className="lens-print-formula-note">
          {translate('flow.difference', 'Difference: {delta}', { delta: formatValue(delta) })}
        </p>
      )}
    </div>
  )
}

function HierarchyFormula({ join, panel, locale }: { join: KeyedJoin; panel: Panel; locale: string }) {
  const translate = useTranslate()
  const formatValue = useValueFormatter(panel, locale)
  const formatShare = useValueFormatter(panel, locale, 'share')
  const views = hierarchyRowViews(panel, join, formatValue, formatShare)
  if (views.length === 0) return null
  return (
    <div className="lens-print-formula">
      <ul className="lens-print-hierarchy">
        {views.map((view) => (
          <li
            className="lens-print-hierarchy-row"
            data-parent={view.isParent || undefined}
            key={view.row.key}
            style={{ '--lens-depth': view.depth } as CSSProperties}
          >
            <span className="lens-print-hierarchy-label">
              {view.row.label}
              {view.row.unallocated && (
                <em>{translate('hierarchy.unallocated', 'Unallocated')}</em>
              )}
              {view.row.description && <small>{view.row.description}</small>}
            </span>
            <span className="lens-print-hierarchy-value">
              {view.showDash ? '—' : view.valueText}
              <PrintQualityChip confidence={view.confidence} availability={view.availability} />
            </span>
            <span className="lens-print-hierarchy-share">{view.shareText ?? ''}</span>
            {view.reconcile && (
              <span className="lens-print-hierarchy-reconcile">
                {view.reconcile.balanced
                  ? translate('hierarchy.allocated', 'Allocated 100%')
                  : translate('hierarchy.difference', 'Difference: {delta}', {
                      delta: formatValue(view.reconcile.delta),
                    })}
              </span>
            )}
          </li>
        ))}
      </ul>
    </div>
  )
}

function RelationshipFormula({ join, panel, locale }: { join: KeyedJoin | undefined; panel: Panel; locale: string }) {
  const translate = useTranslate()
  const formatValue = useValueFormatter(panel, locale)
  const config = panel.metricRelationship
  if (!config) return null
  const source = relationshipEndView(panel, join, config.source, formatValue)
  const target = relationshipEndView(panel, join, config.target, formatValue)
  const glyph = connectorGlyphs(config).horizontal
  return (
    <div className="lens-print-formula lens-print-relationship">
      <p className="lens-print-relationship-claim">
        {relationshipSentence(translate, config, source?.end.label ?? '', target?.end.label ?? '')}
      </p>
      <div className="lens-print-relationship-ends">
        {[source, target].map((view, index) => (
          <div className="lens-print-relationship-end" key={view?.end.key ?? index}>
            <span className="lens-print-relationship-label">{view?.end.label ?? '—'}</span>
            <span className="lens-print-relationship-value">
              {!view || view.showDash ? '—' : view.valueText}
              <PrintQualityChip confidence={view?.confidence} availability={view?.availability} />
            </span>
            {index === 0 && <span aria-hidden="true" className="lens-print-relationship-glyph">{glyph}</span>}
          </div>
        ))}
      </div>
      <p className="lens-print-formula-note">
        {translate(`relationship.type.${config.type}`, relationshipTypeFallback[config.type])}
        {config.note ? ` · ${config.note}` : ''}
      </p>
    </div>
  )
}

/**
 * The printed form of a metric panel, or nothing when the panel is not one of
 * the three — the caller then falls back to a chart and its evidence table.
 */
export function PrintFormula({ section }: { section: PrintSection }) {
  const panel = useMemo(() => sectionPanel(section), [section])
  const join = useJoin(panel, section)
  const locale = section.document.meta.locale
  if (panel.kind === 'metric_relationship') {
    return <RelationshipFormula join={join} locale={locale} panel={panel} />
  }
  if (!join || join.kind !== 'ok') return null
  if (panel.kind === 'metric_flow') return <FlowFormula join={join} locale={locale} panel={panel} />
  if (panel.kind === 'metric_hierarchy') return <HierarchyFormula join={join} locale={locale} panel={panel} />
  return null
}
