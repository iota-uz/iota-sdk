import { useMemo, type ReactNode } from 'react'
import type {
  Availability,
  Confidence,
  MetricRelationshipConfig,
  MetricRelationshipDirection,
  MetricRelationshipEnd,
  Panel,
} from '../contract'
import { useFormat, usePanelFrame, useTranslate, type PanelFrameState } from '../runtime'
import { useElementActionResolver } from './actions'
import { buildKeyedJoin, panelField, type KeyedJoinMessages, type ResolvedElement } from './data'
import { PanelFrame } from './PanelFrame'
import { QualityChip } from './QualityChip'

export interface MetricRelationshipPanelProps {
  panel: Panel
}

interface Glyphs {
  /** Horizontal connector glyph. */
  horizontal: string
  /** Vertical connector glyph for the narrow column layout (no CSS rotation). */
  vertical: string
}

function connectorGlyphs(config: MetricRelationshipConfig): Glyphs {
  const direction: MetricRelationshipDirection = config.direction ?? 'bidirectional'
  // association and reconciliation are symmetric by nature — a neutral
  // double-headed connector that never implies containment or derivation.
  if (config.type !== 'derivation' || direction === 'bidirectional') {
    return { horizontal: '⇄', vertical: '⇵' }
  }
  if (direction === 'target_to_source') return { horizontal: '←', vertical: '↑' }
  return { horizontal: '→', vertical: '↓' }
}

function useMessages(): KeyedJoinMessages {
  const translate = useTranslate()
  return useMemo<KeyedJoinMessages>(() => ({
    missingColumn: (column) => translate('panel.missingColumn', 'Panel data is missing the “{column}” column.', { column }),
    duplicateKey: (key) => translate('panel.duplicateKey', 'Panel data has a duplicate key “{key}”.', { key }),
  }), [translate])
}

interface EndView {
  end: MetricRelationshipEnd
  showDash: boolean
  valueText: string
  confidence?: Confidence
  availability?: Availability
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

  const endView = (end: MetricRelationshipEnd | undefined): EndView | undefined => {
    if (!end) return undefined
    const element: ResolvedElement = join?.kind === 'ok' ? join.resolve(end.key) : { value: { kind: 'unavailable' } }
    const confidence = element.confidence ?? end.confidence ?? panel.confidence
    const structuralAvailability = element.availability ?? end.availability ?? panel.availability
    const missing = element.value.kind === 'unavailable'
    const availability: Availability | undefined = missing
      ? (structuralAvailability && structuralAvailability !== 'available' ? structuralAvailability : 'unavailable')
      : structuralAvailability
    const showDash = missing || (availability !== undefined && availability !== 'available')
    return {
      end,
      showDash,
      valueText: element.value.kind === 'value' ? formatValue(element.value.value) : '—',
      confidence,
      availability,
    }
  }

  const source = endView(config?.source)
  const target = endView(config?.target)
  const glyphs = config ? connectorGlyphs(config) : { horizontal: '⇄', vertical: '⇵' }

  const typeLabel = translate(`relationship.type.${config?.type ?? 'association'}`, config?.type ?? 'association')
  const sentence = relationshipSentence(translate, config, source?.end.label ?? '', target?.end.label ?? '')

  const renderEnd = (view: EndView | undefined, role: 'source' | 'target'): ReactNode => {
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

function relationshipSentence(
  translate: Translate,
  config: MetricRelationshipConfig | undefined,
  source: string,
  target: string,
): string {
  const type = config?.type ?? 'association'
  if (type === 'association') {
    return translate('relationship.association', '{source} is economically linked to {target}', { source, target })
  }
  if (type === 'reconciliation') {
    return translate('relationship.reconciliation', '{source} reconciles with {target}', { source, target })
  }
  // derivation: the arrow follows the declared direction, so the sentence names
  // the deriving end first.
  if (config?.direction === 'target_to_source') {
    return translate('relationship.derivation', '{source} derives {target}', { source: target, target: source })
  }
  return translate('relationship.derivation', '{source} derives {target}', { source, target })
}
