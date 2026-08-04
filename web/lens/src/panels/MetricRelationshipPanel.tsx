import { useMemo, type ReactNode } from 'react'
import type {
  MetricRelationshipConfig,
  Panel,
} from '../contract'
import { useFormat, usePanelFrame, useTranslate, type PanelFrameState } from '../runtime'
import { useElementActionResolver } from './actions'
import { buildKeyedJoin, panelField, type KeyedJoinMessages } from './data'
import {
  connectorGlyphs,
  relationshipEndView,
  relationshipOperands,
  relationshipTypeFallback,
  type RelationshipEndView,
} from './metricViews'
import { PanelFrame } from './PanelFrame'
import { QualityChip } from './QualityChip'

export interface MetricRelationshipPanelProps {
  panel: Panel
}

function useMessages(): KeyedJoinMessages {
  const translate = useTranslate()
  return useMemo<KeyedJoinMessages>(() => ({
    missingColumn: (column) => translate('panel.missingColumn', 'Panel data is missing the “{column}” column.', { column }),
    duplicateKey: (key) => translate('panel.duplicateKey', 'Panel data has a duplicate key “{key}”.', { key }),
  }), [translate])
}

export function MetricRelationshipPanel({ panel }: MetricRelationshipPanelProps) {
  const frame = usePanelFrame(panel.id)
  const translate = useTranslate()
  const messages = useMessages()
  const valueField = panelField(panel, 'value') ?? 'value'
  const formatValue = useFormat(panel.format[valueField])
  const resolveAction = useElementActionResolver()
  const config = panel.metricRelationship

  const join = useMemo(
    () => (frame.data ? buildKeyedJoin(panel, frame.data, messages) : undefined),
    [frame.data, messages, panel],
  )
  const contractError = join?.kind === 'contract-error' ? join.message : undefined
  const effectiveFrame: PanelFrameState = contractError
    ? { ...frame, data: undefined, error: new Error(contractError) }
    : frame

  const source = relationshipEndView(panel, join, config?.source, formatValue)
  const target = relationshipEndView(panel, join, config?.target, formatValue)
  const glyphs = config ? connectorGlyphs(config) : { horizontal: '⇄', vertical: '⇵' }

  const type = config?.type ?? 'association'
  const typeLabel = translate(`relationship.type.${type}`, relationshipTypeFallback[type])
  const sentence = relationshipSentence(translate, config, source?.end.label ?? '', target?.end.label ?? '')

  const renderEnd = (view: RelationshipEndView | undefined, role: 'source' | 'target'): ReactNode => {
    if (!view) return <div className={`lens-relationship-end lens-relationship-end-${role}`} />
    const targetAction = resolveAction(view.end.action)
    const body = (
      <>
        <span className="lens-relationship-end-label" title={view.end.label}>{view.end.label}</span>
        <span className="lens-relationship-end-value">
          {view.showDash ? '—' : view.valueText}
        </span>
        <QualityChip confidence={view.confidence} availability={view.availability} className="lens-relationship-chip" />
      </>
    )
    return (
      <div className={`lens-relationship-end lens-relationship-end-${role}`}>
        {targetAction ? (
          <a
            aria-label={translate('panel.openMetric', 'Open {name}', { name: view.end.label })}
            className="lens-relationship-end-inner lens-relationship-end-link"
            href={targetAction.href}
            onClick={targetAction.onClick}
          >
            {body}
          </a>
        ) : (
          <div className="lens-relationship-end-inner">{body}</div>
        )}
      </div>
    )
  }

  return (
    <PanelFrame panel={panel} frame={effectiveFrame} allowEmptyContent>
      <div className="lens-relationship" data-type={config?.type}>
        <p className="lens-visually-hidden">{sentence}</p>
        {renderEnd(source, 'source')}
        <div aria-hidden="true" className="lens-relationship-connector" data-type={config?.type}>
          <span className="lens-relationship-glyph lens-relationship-glyph-h">{glyphs.horizontal}</span>
          <span className="lens-relationship-glyph lens-relationship-glyph-v">{glyphs.vertical}</span>
          <span className="lens-relationship-type">{typeLabel}</span>
          {config?.note && <span className="lens-relationship-note">{config.note}</span>}
        </div>
        {renderEnd(target, 'target')}
      </div>
    </PanelFrame>
  )
}

type Translate = (key: string, fallback: string, vars?: Record<string, string | number>) => string

/**
 * What the relationship claims, in words. On screen it is the screen-reader
 * sentence behind the glyph; on paper it is the only form the claim has, since
 * a printed page carries no connector animation and no tooltip.
 */
export function relationshipSentence(
  translate: Translate,
  config: MetricRelationshipConfig | undefined,
  sourceLabel: string,
  targetLabel: string,
): string {
  const type = config?.type ?? 'association'
  const { source, target } = relationshipOperands(config, sourceLabel, targetLabel)
  if (type === 'association') {
    return translate('relationship.association', '{source} is economically linked to {target}', { source, target })
  }
  if (type === 'reconciliation') {
    return translate('relationship.reconciliation', '{source} reconciles with {target}', { source, target })
  }
  return translate('relationship.derivation', '{source} derives {target}', { source, target })
}
