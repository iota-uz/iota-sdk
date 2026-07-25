import { useMemo, type CSSProperties } from 'react'
import type { Availability, Confidence, MetricHierarchyRow, Panel } from '../contract'
import { useFormat, usePanelFrame, useTranslate, type PanelFrameState } from '../runtime'
import { useElementActionResolver } from './actions'
import { buildKeyedJoin, panelField, type KeyedJoinMessages } from './data'
import { PanelFrame } from './PanelFrame'
import { QualityChip, resolveQuality } from './QualityChip'

export interface MetricHierarchyPanelProps {
  panel: Panel
}

interface HierarchyRowView {
  row: MetricHierarchyRow
  depth: number
  isParent: boolean
  showDash: boolean
  valueText: string
  shareText?: string
  confidence?: Confidence
  availability?: Availability
  /** Per-parent reconciliation summary, when reconciliation is enabled. */
  reconcile?: { balanced: boolean; text: string }
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
  const rows = useMemo(() => panel.metricHierarchy?.rows ?? [], [panel.metricHierarchy])
  const reconcile = panel.metricHierarchy?.reconcile

  const join = useMemo(
    () => (frame.data ? buildKeyedJoin(panel, frame.data, messages) : undefined),
    [frame.data, messages, panel],
  )
  const contractError = join?.kind === 'contract-error' ? join.message : undefined
  const effectiveFrame: PanelFrameState = contractError
    ? { ...frame, data: undefined, error: new Error(contractError) }
    : frame

  const views: HierarchyRowView[] = useMemo(() => {
    if (join?.kind !== 'ok') return []
    const byKey = new Map(rows.map((row) => [row.key, row]))
    const parentKeys = new Set(rows.map((row) => row.parent).filter((key): key is string => Boolean(key)))
    const childrenByParent = new Map<string, MetricHierarchyRow[]>()
    for (const row of rows) {
      if (!row.parent) continue
      const list = childrenByParent.get(row.parent) ?? []
      list.push(row)
      childrenByParent.set(row.parent, list)
    }

    // Depth: parent is authoritative; the wire may carry a derived depth, but
    // it is reconstructed here from the parent chain when absent (guarding a
    // detached parent reference against a runaway walk).
    const depthOf = (row: MetricHierarchyRow): number => {
      if (typeof row.depth === 'number') return row.depth
      let depth = 0
      let current: MetricHierarchyRow | undefined = row
      const seen = new Set<string>()
      while (current?.parent && !seen.has(current.key)) {
        seen.add(current.key)
        current = byKey.get(current.parent)
        if (!current) break
        depth += 1
      }
      return depth
    }

    return rows.map((row) => {
      const element = join.resolve(row.key)
      const confidence = element.confidence ?? row.confidence ?? panel.confidence
      const structuralAvailability = element.availability ?? row.availability ?? panel.availability
      const missing = element.value.kind === 'unavailable'
      const availability: Availability | undefined = missing
        ? (structuralAvailability && structuralAvailability !== 'available' ? structuralAvailability : 'unavailable')
        : structuralAvailability
      const showDash = missing || (availability !== undefined && availability !== 'available')
      const valueText = element.value.kind === 'value' ? formatValue(element.value.value) : '—'
      const shareText = element.share !== undefined ? formatShare(element.share) : undefined

      const children = childrenByParent.get(row.key)
      let reconcileSummary: HierarchyRowView['reconcile']
      if (reconcile && children && children.length > 0 && element.value.kind === 'value') {
        let sum = 0
        let complete = true
        for (const child of children) {
          const childValue = join.resolve(child.key).value
          if (childValue.kind !== 'value') { complete = false; break }
          sum += childValue.value
        }
        if (complete) {
          const delta = element.value.value - sum
          const balanced = Math.abs(delta) <= (reconcile.tolerance ?? 0)
          reconcileSummary = balanced
            ? { balanced: true, text: translate('hierarchy.allocated', 'Allocated 100%') }
            : { balanced: false, text: translate('hierarchy.difference', 'Difference: {delta}', { delta: formatValue(delta) }) }
        }
      }

      return {
        row,
        depth: depthOf(row),
        isParent: parentKeys.has(row.key),
        showDash,
        valueText,
        shareText,
        confidence,
        availability,
        reconcile: reconcileSummary,
      }
    })
  }, [formatShare, formatValue, join, panel.availability, panel.confidence, reconcile, rows, translate])

  return (
    <PanelFrame panel={panel} frame={effectiveFrame} allowEmptyContent>
      <ul className="lens-hierarchy" role="list">
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
                  {view.reconcile.text}
                </p>
              )}
            </li>
          )
        })}
      </ul>
    </PanelFrame>
  )
}
