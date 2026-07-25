import { useMemo } from 'react'
import type { Availability, Confidence, MetricFlowStage, MetricFlowStageRole, Panel } from '../contract'
import { useFormat, usePanelFrame, useTranslate, type PanelFrameState } from '../runtime'
import { useElementActionResolver } from './actions'
import { buildKeyedJoin, panelField, type KeyedJoinMessages, type ResolvedNumber } from './data'
import { PanelFrame } from './PanelFrame'
import { QualityChip, resolveQuality } from './QualityChip'

export interface MetricFlowPanelProps {
  panel: Panel
}

/** The running-formula grammar: which stages carry an operator and its sign. */
const OPERATOR_ROLES: ReadonlySet<MetricFlowStageRole> = new Set<MetricFlowStageRole>(['add', 'subtract'])

/** A stage's signed contribution to the reconciliation sum (grammar, not display). */
function contribution(role: MetricFlowStageRole, value: number): number {
  return role === 'subtract' ? -value : value
}

interface FlowStageView {
  stage: MetricFlowStage
  role: MetricFlowStageRole
  /** Rendered operator glyph, or '' for an input stage. */
  operator: string
  /** Spoken operator word folded into the aria sentence. */
  operatorWord: string
  /** True when the value slot shows an em dash rather than a number. */
  showDash: boolean
  /** Formatted magnitude (absolute for operands, signed for input/intermediate/result). */
  text: string
  confidence?: Confidence
  availability?: Availability
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
  const stages = useMemo(() => panel.metricFlow?.stages ?? [], [panel.metricFlow])
  const tolerance = panel.metricFlow?.reconcile?.tolerance

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

  const views: FlowStageView[] = useMemo(() => {
    if (join?.kind !== 'ok') return []
    const operatorWord = (operator: string): string => {
      if (operator === '+') return translate('flow.plus', 'plus')
      if (operator === '−') return translate('flow.minus', 'minus')
      if (operator === '=') return translate('flow.equals', 'equals')
      return ''
    }
    return stages.map((stage) => {
      const element = join.resolve(stage.key)
      const confidence = element.confidence ?? stage.confidence ?? panel.confidence
      const structuralAvailability = element.availability ?? stage.availability ?? panel.availability
      const missing = element.value.kind === 'unavailable'
      // A genuinely missing value is at least "unavailable"; a more specific
      // producer status (config_required / empty_source) is kept.
      const availability: Availability | undefined = missing
        ? (structuralAvailability && structuralAvailability !== 'available' ? structuralAvailability : 'unavailable')
        : structuralAvailability
      const showDash = missing || (availability !== undefined && availability !== 'available')

      const numeric = element.value.kind === 'value' ? element.value.value : 0
      const isOperand = OPERATOR_ROLES.has(stage.role)
      const magnitude = isOperand ? Math.abs(numeric) : numeric
      let operator = ''
      // When the amount is unavailable, the absent numeric payload must not
      // rewrite the declared formula grammar. Keep "+" for an add stage and
      // "−" for a subtract stage; signed-value normalization only applies to
      // values that are actually shown.
      if (showDash && stage.role === 'add') operator = '+'
      else if (showDash && stage.role === 'subtract') operator = '−'
      else if (isOperand) operator = contribution(stage.role, numeric) >= 0 ? '+' : '−'
      else if (stage.role === 'intermediate' || stage.role === 'result') operator = '='
      const text = formatValue(magnitude)

      const quality = resolveQuality({ confidence, availability })
      const qualityLabel = quality ? translate(quality.meta.labelKey, quality.meta.fallback) : undefined
      const word = operatorWord(operator)
      const spokenValue = showDash ? (qualityLabel ?? translate('availability.unavailable', 'Unavailable')) : text
      const trailingQuality = !showDash && qualityLabel ? `, ${qualityLabel}` : ''
      const ariaLabel = `${word ? `${word} ` : ''}${stage.label}, ${spokenValue}${trailingQuality}`

      return { stage, role: stage.role, operator, operatorWord: word, showDash, text, confidence, availability, ariaLabel }
    })
  }, [formatValue, join, panel.availability, panel.confidence, stages, translate])

  const reconcileNote = useMemo(() => {
    if (join?.kind !== 'ok' || panel.metricFlow?.reconcile === undefined) return undefined
    let sum = 0
    let resultValue: number | undefined
    for (const stage of stages) {
      const value: ResolvedNumber = join.resolve(stage.key).value
      if (stage.role === 'result') {
        if (value.kind !== 'value') return undefined
        resultValue = value.value
        continue
      }
      if (stage.role === 'intermediate') continue
      // input / add / subtract are operands; an unresolved operand makes the
      // sum meaningless, so no note is shown rather than a false discrepancy.
      if (value.kind !== 'value') return undefined
      sum += contribution(stage.role, value.value)
    }
    if (resultValue === undefined) return undefined
    const delta = resultValue - sum
    if (Math.abs(delta) <= (tolerance ?? 0)) return undefined
    return translate('flow.difference', 'Difference: {delta}', { delta: formatValue(delta) })
  }, [formatValue, join, panel.metricFlow?.reconcile, stages, tolerance, translate])

  return (
    <PanelFrame panel={panel} frame={effectiveFrame} allowEmptyContent>
      <ol className="lens-flow" role="list" aria-label={translate('flow.stages', '{name} stages', { name: panel.title })}>
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
              role="listitem"
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
