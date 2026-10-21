import { createUniqueId, Show, type JSX } from 'solid-js'
import { Plus, SquaresFour } from '../icons'
import { useTranslate } from '../runtime'

/** A dashboard without cards: what it is for and, when the reader can edit it, the way to fill it. */
export function DashboardEmptyState(props: { onAdd?: (trigger: HTMLElement) => void }): JSX.Element {
  const translate = useTranslate()
  const titleId = createUniqueId()
  return (
    <section aria-labelledby={titleId} class="lens-dashboard-empty" data-testid="dashboard-empty">
      <span aria-hidden="true" class="lens-dashboard-empty-icon"><SquaresFour /></span>
      <h2 class="lens-dashboard-empty-title" id={titleId}>{translate('dashboard.empty.title', 'This dashboard has no reports yet')}</h2>
      <p class="lens-dashboard-empty-hint">
        {props.onAdd
          ? translate('dashboard.empty.hintEditable', 'Add reports from the library to see their figures here.')
          : translate('dashboard.empty.hint', 'Reports placed on this dashboard will appear here.')}
      </p>
      <Show when={props.onAdd}>
        {(add) => (
          <button class="lens-context-action lens-report-primary lens-dashboard-empty-action" data-testid="dashboard-empty-add" onClick={(event) => add()(event.currentTarget)} type="button">
            <Plus />
            <span>{translate('report.editor.add', 'Add report')}</span>
          </button>
        )}
      </Show>
    </section>
  )
}
