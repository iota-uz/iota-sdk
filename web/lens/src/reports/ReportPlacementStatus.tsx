import { createMemo, Show } from 'solid-js'
import { useDashboard, useDocumentState, usePanelFrames, useTranslate, type PanelFrameState } from '../runtime'

export type PlacementFreshness =
  | { state: 'failed'; failed: PanelFrameState[] }
  | { state: 'updating' }
  | { state: 'calculated'; at: number }

/**
 * One reading for a whole card. A member that failed outranks everything: a
 * dashboard refresh that left this card's request broken has not updated it,
 * whatever the board's own stamp says. The calculation time is the oldest of
 * the members, so the card never claims to be newer than its stalest figure.
 */
export function placementFreshness(frames: PanelFrameState[], generatedAt: string, refreshing: boolean): PlacementFreshness | undefined {
  const failed = frames.filter((frame) => frame.error)
  if (failed.length) return { state: 'failed', failed }
  if (refreshing || frames.some((frame) => frame.isLoading || frame.isStale)) return { state: 'updating' }
  const fallback = Date.parse(generatedAt)
  const times = frames.map((frame) => frame.calculation ? Date.parse(frame.calculation.calculatedAt) : fallback).filter(Number.isFinite)
  return times.length ? { state: 'calculated', at: Math.min(...times) } : undefined
}

/** A wall-clock time that names its zone, in the card's calendar timezone when it has one. */
export function formatCalculatedAt(at: number, locale: string, timeZone?: string): string {
  // dateStyle/timeStyle cannot carry a zone name, so the fields are explicit.
  const options: Intl.DateTimeFormatOptions = { year: 'numeric', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }
  try {
    return new Intl.DateTimeFormat(locale, { ...options, timeZone: timeZone || undefined, timeZoneName: 'short' }).format(at)
  } catch {
    return new Intl.DateTimeFormat(locale, { ...options, timeZoneName: 'short' }).format(at)
  }
}

/**
 * How current a placement's figures are, and the card's own retry. Retry
 * re-requests only this card's failed members; the dashboard's refresh is a
 * separate control and never marks this card updated while it is failing.
 */
export function ReportPlacementStatus(props: { placementId: string; panelIds: string[]; timeZone?: string }) {
  const { document } = useDashboard()
  const documentState = useDocumentState()
  const translate = useTranslate()
  const frames = usePanelFrames()
  const freshness = createMemo(() => placementFreshness(
    props.panelIds.map((id) => frames(id)), document.meta.generatedAt, documentState.isRefreshing,
  ))
  return (
    <Show when={freshness()}>
      {(value) => (
        <footer class="lens-report-card-status" data-state={value().state} data-testid={`report-status-${props.placementId}`}>
          <Show when={value().state === 'failed'} fallback={
            <p aria-live="polite">
              {value().state === 'updating'
                ? translate('panel.updating', 'Updating')
                : translate('report.card.calculated', 'Calculated {time}', {
                  time: formatCalculatedAt((value() as { at: number }).at, document.meta.locale, props.timeZone),
                })}
            </p>
          }>
            <p role="status">{translate('report.card.failed', 'Not updated: some figures in this card could not be loaded.')}</p>
            <button
              class="lens-context-action"
              data-testid={`report-retry-${props.placementId}`}
              onClick={() => { for (const frame of (value() as { failed: PanelFrameState[] }).failed) frame.retry() }}
              type="button"
            >
              {translate('report.card.retry', 'Retry this card')}
            </button>
          </Show>
        </footer>
      )}
    </Show>
  )
}
