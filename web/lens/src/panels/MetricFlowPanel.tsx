import { useMemo } from 'react'
import type { Panel } from '../contract'
import { useFormat, usePanelFrame, useTranslate, type PanelFrameState } from '../runtime'
import { useElementActionResolver } from './actions'
import { buildKeyedJoin, panelField, type KeyedJoinMessages } from './data'
import { flowReconcileDelta, flowStageViews, type FlowStageView } from './metricViews'
import { PanelFrame } from './PanelFrame'
import { QualityChip, resolveQuality } from './QualityChip'

export interface MetricFlowPanelProps {
  panel: Panel
}

interface FlowStageRow extends FlowStageView {
  /** Spoken operator word folded into the aria sentence. */
  operatorWord: string
  ariaLabel: string
}

function useMessages(): KeyedJoinMessages {
  const translate = useTranslate()
  return useMemo<KeyedJoinMessages>(() => ({
    missingColumn: (column) => translate('panel.missingColumn', 'Panel data is missing the “{column}” column.', { column }),
    duplicateKey: (key) => translate('panel.duplicateKey', 'Panel data has a duplicate key “{key}”.', { key }),
  }), [translate])
}

export function MetricFlowPanel({ panel }: MetricFlowPanelProps) {
  const frame = usePanelFrame(panel.id)
  const translate = useTranslate()
  const messages = useMessages()
  const valueField = panelField(panel, 'value') ?? 'value'
  const formatValue = useFormat(panel.format[valueField])
  const resolveAction = useElementActionResolver()

  const join = useMemo(
    () => (frame.data ? buildKeyedJoin(panel, frame.data, messages) : undefined),
    [frame.data, messages, panel],
  )
  const contractError = join?.kind === 'contract-error' ? join.message : undefined
  // A mis-contracted panel is a wiring error, not a business state: it takes
  // over the body through PanelFrame's own error path, keeping loading / stale /
  // empty as PanelFrame's responsibility.
  const effectiveFrame: PanelFrameState = contractError
    ? { ...frame, data: undefined, error: new Error(contractError) }
    : frame

  const views: FlowStageRow[] = useMemo(() => {
    if (!join) return []
    const operatorWord = (operator: string): string => {
      if (operator === '+') return translate('flow.plus', 'plus')
      if (operator === '−') return translate('flow.minus', 'minus')
      if (operator === '=') return translate('flow.equals', 'equals')
      return ''
    }
    return flowStageViews(panel, join, formatValue).map((view) => {
      const quality = resolveQuality(view)
      const qualityLabel = quality ? translate(quality.meta.labelKey, quality.meta.fallback) : undefined
      const word = operatorWord(view.operator)
      const spokenValue = view.showDash ? (qualityLabel ?? translate('availability.unavailable', 'Unavailable')) : view.text
      const trailingQuality = !view.showDash && qualityLabel ? `, ${qualityLabel}` : ''
      return {
        ...view,
        operatorWord: word,
        ariaLabel: `${word ? `${word} ` : ''}${view.stage.label}, ${spokenValue}${trailingQuality}`,
      }
    })
  }, [formatValue, join, panel, translate])

  const reconcileNote = useMemo(() => {
    if (!join) return undefined
    const delta = flowReconcileDelta(panel, join)
    if (delta === undefined) return undefined
    return translate('flow.difference', 'Difference: {delta}', { delta: formatValue(delta) })
  }, [formatValue, join, panel, translate])

  return (
    <PanelFrame panel={panel} frame={effectiveFrame} allowEmptyContent>
      <ol className="lens-flow" aria-label={translate('flow.stages', '{name} stages', { name: panel.title })}>
        {views.map((view, index) => {
          const target = resolveAction(view.stage.action)
          const main = (
            <>
              <span className="lens-flow-stage-label">{view.stage.label}</span>
              <span className="lens-flow-stage-value">
                {view.showDash ? '—' : view.text}
              </span>
              <QualityChip confidence={view.confidence} availability={view.availability} className="lens-flow-stage-chip" />
              {view.stage.caption && <span className="lens-flow-stage-caption">{view.stage.caption}</span>}
            </>
          )
          return (
            <li
              aria-label={view.ariaLabel}
              className={`lens-flow-stage lens-flow-stage-${view.role}`}
              data-result={view.role === 'result' || undefined}
              key={`${view.stage.key}-${index}`}
            >
              {view.operator && <span aria-hidden="true" className="lens-flow-op">{view.operator}</span>}
              {target ? (
                <a
                  aria-label={translate('panel.openMetric', 'Open {name}', { name: view.stage.label })}
                  className="lens-flow-stage-main lens-flow-stage-link"
                  href={target.href}
                  onClick={target.onClick}
                >
                  {main}
                </a>
              ) : (
                <div className="lens-flow-stage-main">{main}</div>
              )}
            </li>
          )
        })}
      </ol>
      {reconcileNote && <p className="lens-flow-reconcile">{reconcileNote}</p>}
    </PanelFrame>
  )
}
