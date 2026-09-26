/* eslint-disable react/no-unknown-property -- Solid JSX uses `class`, the React-era rule expects `className`; the lint config migrates with the Solid port. */
import { createMemo, For, Show } from 'solid-js'
import type { JSX } from 'solid-js'
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
  return createMemo<KeyedJoinMessages>(() => ({
    missingColumn: (column) => translate('panel.missingColumn', 'Panel data is missing the “{column}” column.', { column }),
    duplicateKey: (key) => translate('panel.duplicateKey', 'Panel data has a duplicate key “{key}”.', { key }),
  }))()
}

export function MetricHierarchyPanel(props: MetricHierarchyPanelProps) {
  const panel = props.panel
  const frame = usePanelFrame(panel.id)
  const translate = useTranslate()
  const messages = useMessages()
  const valueField = panelField(panel, 'value') ?? 'value'
  const shareField = panelField(panel, 'share')
  const formatValue = useFormat(panel.format[valueField])
  const formatShare = useFormat(shareField ? panel.format[shareField] : undefined)
  const resolveAction = useElementActionResolver()

  const join = createMemo(
    () => (frame.data ? buildKeyedJoin(panel, frame.data, messages) : undefined),
  )
  const contractError = createMemo(() => {
    const current = join()
    return current?.kind === 'contract-error' ? current.message : undefined
  })
  const effectiveFrame = createMemo((): PanelFrameState => contractError()
    ? { ...frame, data: undefined, error: new Error(contractError()) }
    : frame)

  const views = createMemo<Array<HierarchyRowView>>(
    () => (join() ? hierarchyRowViews(panel, join()!, formatValue, formatShare) : []),
  )

  const reconcileText = (summary: NonNullable<HierarchyRowView['reconcile']>): string => (
    summary.balanced
      ? translate('hierarchy.allocated', 'Allocated 100%')
      : translate('hierarchy.difference', 'Difference: {delta}', { delta: formatValue(summary.delta) })
  )

  return (
    <PanelFrame panel={panel} frame={effectiveFrame()} allowEmptyContent>
      <ul class="lens-hierarchy">
        <For each={views()}>
          {(view) => {
            const { row } = view
            const quality = resolveQuality({ confidence: view.confidence, availability: view.availability })
            const qualityLabel = quality ? translate(quality.meta.labelKey, quality.meta.fallback) : undefined
            const spokenValue = view.showDash ? (qualityLabel ?? translate('availability.unavailable', 'Unavailable')) : view.valueText
            const ariaLabel = `${row.label}, ${spokenValue}${view.shareText ? `, ${view.shareText}` : ''}`
            const target = resolveAction(row.action)
            const body = (
              <>
                <span class="lens-hierarchy-label">
                  {view.depth > 0 && <span aria-hidden="true" class="lens-hierarchy-guide" />}
                  <span class="lens-hierarchy-label-text" title={row.label}>{row.label}</span>
                  {row.unallocated && (
                    <span class="lens-hierarchy-unallocated-tag">
                      {translate('hierarchy.unallocated', 'Unallocated')}
                    </span>
                  )}
                  {row.description && <span class="lens-hierarchy-description" title={row.description}>{row.description}</span>}
                </span>
                <span class="lens-hierarchy-value">
                  {view.showDash ? '—' : view.valueText}
                  <QualityChip confidence={view.confidence} availability={view.availability} className="lens-hierarchy-chip" />
                </span>
                {view.shareText !== undefined && <span class="lens-hierarchy-share">{view.shareText}</span>}
              </>
            )
            return (
              <li
                aria-current={row.selected ? 'true' : undefined}
                class={[
                  'lens-hierarchy-row',
                  view.isParent ? 'lens-hierarchy-row-parent' : '',
                  row.unallocated ? 'lens-hierarchy-row-unallocated' : '',
                  row.selected ? 'lens-hierarchy-row-selected' : '',
                ].filter(Boolean).join(' ')}
                style={{ '--lens-depth': view.depth } as JSX.CSSProperties}
              >
                <Show when={target} fallback={<div aria-label={ariaLabel} class="lens-hierarchy-row-inner" role="group">{body}</div>}>
                  <a
                    aria-label={ariaLabel}
                    class="lens-hierarchy-row-inner lens-hierarchy-row-link"
                    href={target!.href}
                    onClick={target!.onClick}
                  >
                    {body}
                  </a>
                </Show>
                {view.reconcile && (
                  <p
                    class={`lens-hierarchy-reconcile${view.reconcile.balanced ? ' lens-hierarchy-reconcile-balanced' : ' lens-hierarchy-reconcile-off'}`}
                  >
                    {reconcileText(view.reconcile)}
                  </p>
                )}
              </li>
            )
          }}
        </For>
      </ul>
    </PanelFrame>
  )
}
