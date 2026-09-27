import { createMemo, For, Show, type JSX } from 'solid-js'
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
import { columnUnit } from './units'
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
  return createMemo(() => {
    if (!section.frame) return undefined
    return buildKeyedJoin(panel, section.frame, {
      missingColumn: (column) => translate('panel.missingColumn', 'Panel data is missing the “{column}” column.', { column }),
      duplicateKey: (key) => translate('panel.duplicateKey', 'Panel data has a duplicate key “{key}”.', { key }),
    })
  })()
}

function valueFormatter(panel: Panel, locale: string, role: 'value' | 'share' = 'value') {
  const field = panel.encoding[role]
  return (value: number) => formatFieldValue(value, field ? panel.format[field] : undefined, locale)
}

function FlowFormula(props: { join: KeyedJoin; panel: Panel; locale: string }): JSX.Element | null {
  const translate = useTranslate()
  const formatValue = valueFormatter(props.panel, props.locale)
  // The unit is a property of the calculation, not of each of its six lines.
  // Collect the amounts, choose one magnitude for all of them, and say it once
  // above the column — the same contract the printed tables follow.
  const unit = createMemo(() => {
    const amounts: Array<number> = []
    flowStageViews(props.panel, props.join, (value) => {
      amounts.push(value)
      return ''
    })
    return columnUnit(amounts, props.panel.encoding.value ? props.panel.format[props.panel.encoding.value] : undefined, props.locale)
  })
  const views = createMemo(() => flowStageViews(props.panel, props.join, (value) => unit().format(value)))
  const delta = createMemo(() => flowReconcileDelta(props.panel, props.join))
  return (
    <Show when={views().length > 0}>
      <div class="lens-print-formula">
        <Show when={unit().note}>
          <p class="lens-print-formula-unit">{unit().note}</p>
        </Show>
        <ol class="lens-print-flow">
          <For each={views()}>
            {(view) => (
              <li
                class="lens-print-flow-stage"
                data-result={view.role === 'result' || undefined}
              >
                <span class="lens-print-flow-op">{view.operator}</span>
                <span class="lens-print-flow-label">
                  {view.stage.label}
                  <Show when={view.stage.caption}>
                    <small>{view.stage.caption}</small>
                  </Show>
                </span>
                <span class="lens-print-flow-value">
                  {view.showDash ? '—' : view.text}
                  <PrintQualityChip confidence={view.confidence} availability={view.availability} />
                </span>
              </li>
            )}
          </For>
        </ol>
        <Show when={delta() !== undefined}>
          <p class="lens-print-formula-note">
            {translate('flow.difference', 'Difference: {delta}', { delta: formatValue(delta()!) })}
          </p>
        </Show>
      </div>
    </Show>
  )
}

function HierarchyFormula(props: { join: KeyedJoin; panel: Panel; locale: string }): JSX.Element | null {
  const translate = useTranslate()
  const formatValue = valueFormatter(props.panel, props.locale)
  const formatShare = valueFormatter(props.panel, props.locale, 'share')
  const views = createMemo(() => hierarchyRowViews(props.panel, props.join, formatValue, formatShare))
  return (
    <Show when={views().length > 0}>
      <div class="lens-print-formula">
        <ul class="lens-print-hierarchy">
          <For each={views()}>
            {(view) => (
              <li
                class="lens-print-hierarchy-row"
                data-parent={view.isParent || undefined}
                style={{ '--lens-depth': view.depth } as JSX.CSSProperties}
              >
                <span class="lens-print-hierarchy-label">
                  {view.row.label}
                  <Show when={view.row.unallocated}>
                    <em>{translate('hierarchy.unallocated', 'Unallocated')}</em>
                  </Show>
                  <Show when={view.row.description}>
                    <small>{view.row.description}</small>
                  </Show>
                </span>
                <span class="lens-print-hierarchy-value">
                  {view.showDash ? '—' : view.valueText}
                  <PrintQualityChip confidence={view.confidence} availability={view.availability} />
                </span>
                <span class="lens-print-hierarchy-share">{view.shareText ?? ''}</span>
                <Show when={view.reconcile}>
                  <span class="lens-print-hierarchy-reconcile">
                    {view.reconcile!.balanced
                      ? translate('hierarchy.allocated', 'Allocated 100%')
                      : translate('hierarchy.difference', 'Difference: {delta}', {
                        delta: formatValue(view.reconcile!.delta),
                      })}
                  </span>
                </Show>
              </li>
            )}
          </For>
        </ul>
      </div>
    </Show>
  )
}

function RelationshipFormula(props: { join: KeyedJoin | undefined; panel: Panel; locale: string }): JSX.Element | null {
  const translate = useTranslate()
  const formatValue = valueFormatter(props.panel, props.locale)
  const config = props.panel.metricRelationship
  return (
    <Show when={config}>
      {(value) => {
        const source = relationshipEndView(props.panel, props.join, value().source, formatValue)
        const target = relationshipEndView(props.panel, props.join, value().target, formatValue)
        const glyph = connectorGlyphs(value()).horizontal
        return (
          <div class="lens-print-formula lens-print-relationship">
            <p class="lens-print-relationship-claim">
              {relationshipSentence(translate, value(), source?.end.label ?? '', target?.end.label ?? '')}
            </p>
            <div class="lens-print-relationship-ends">
              <For each={[source, target]}>
                {(view, index) => (
                  <div class="lens-print-relationship-end">
                    <span class="lens-print-relationship-label">{view?.end.label ?? '—'}</span>
                    <span class="lens-print-relationship-value">
                      {!view || view.showDash ? '—' : view.valueText}
                      <PrintQualityChip confidence={view?.confidence} availability={view?.availability} />
                    </span>
                    <Show when={index() === 0}>
                      <span aria-hidden="true" class="lens-print-relationship-glyph">{glyph}</span>
                    </Show>
                  </div>
                )}
              </For>
            </div>
            <p class="lens-print-formula-note">
              {translate(`relationship.type.${value().type}`, relationshipTypeFallback[value().type])}
              {value().note ? ` · ${value().note}` : ''}
            </p>
          </div>
        )
      }}
    </Show>
  )
}

/**
 * The printed form of a metric panel, or nothing when the panel is not one of
 * the three — the caller then falls back to a chart and its evidence table.
 */
export function PrintFormula(props: { section: PrintSection }): JSX.Element | null {
  const panel = createMemo(() => sectionPanel(props.section))
  const join = useJoin(panel(), props.section)
  const locale = props.section.document.meta.locale
  if (panel().kind === 'metric_relationship') {
    return <RelationshipFormula join={join} locale={locale} panel={panel()} />
  }
  if (!join || join.kind !== 'ok') return null
  if (panel().kind === 'metric_flow') return <FlowFormula join={join} locale={locale} panel={panel()} />
  if (panel().kind === 'metric_hierarchy') return <HierarchyFormula join={join} locale={locale} panel={panel()} />
  return null
}
