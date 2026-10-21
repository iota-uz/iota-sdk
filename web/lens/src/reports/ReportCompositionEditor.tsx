import { For, Show, untrack, type JSX } from 'solid-js'
import { LensMessagesProvider, useTranslate, type LensThemeMode } from '../runtime'
import type { Composition, Placement } from './contract/types'
import type { ReportDraft } from './draft'
import {
  createCompositionSession, createPlacementControls, periodText, PlacementHandle, PlacementResize, SessionAlerts, SessionDialogs, SessionProposal,
  SessionSaveActions, type CompositionSession, type ReportEditorLifecycle, type ReportProposal, type ReportProposalHandlers,
} from './editing'
import { createReportLeaveGuard, type ReportLeaveGuard } from './guard'
import { spanOf } from './layout'
import type { ReportCatalogHost } from './picker'

export type { ReportEditorLifecycle, ReportProposal, ReportProposalHandlers } from './editing'

export interface ReportCompositionEditorProps {
  draft: ReportDraft<Composition>
  lifecycle: ReportEditorLifecycle
  /** The saved report's title for a placement without its own caption. */
  reportTitle: (reportId: string) => string | undefined
  /** Enables "Add report" through the picker. */
  catalog?: ReportCatalogHost
  /** The card body, typically the host's rendering of the placement. */
  renderPlacement?: (placement: Placement) => JSX.Element
  /** How many placements use a report, shown before editing the shared definition. */
  usage?: (reportId: string) => Promise<number> | number
  /** Opens the shared report definition for editing. */
  onEditReport?: (reportId: string, placementId: string) => Promise<void> | void
  /** Replaces the placement's report with an independent copy. */
  onCopyReport?: (placementId: string) => Promise<void> | void
  /** After a revision conflict: keep the local draft as an independent object. */
  onSaveCopy?: (draft: Composition) => Promise<void> | void
  /** In-app navigation guard; the editor creates one with browser-unload protection when omitted. */
  guard?: ReportLeaveGuard
  proposal?: ReportProposal
  proposalHandlers?: ReportProposalHandlers
  /** Placement span of cards added from the picker (12 by default). */
  addSpan?: number
  theme?: LensThemeMode
  messages?: Record<string, string>
}

/**
 * Edits a composition draft: layout, captions, card-local periods; Save and
 * Cancel act on the whole session. It renders its own Lens root, so the
 * runtime's tokens apply wherever a host mounts it. LensDashboard's edit mode
 * offers the same session over the live cards.
 */
export function ReportCompositionEditor(props: ReportCompositionEditorProps): JSX.Element {
  return (
    <div class="lens-root lens-root-inline" data-theme={props.theme ?? 'light'}>
      <LensMessagesProvider messages={props.messages}>
        <Editor {...props} />
      </LensMessagesProvider>
    </div>
  )
}

function Editor(props: ReportCompositionEditorProps): JSX.Element {
  const translate = useTranslate()
  const draft = untrack(() => props.draft)
  const guard = untrack(() => props.guard) ?? createReportLeaveGuard(draft)
  const session = createCompositionSession(props)
  let gridRef: HTMLDivElement | undefined

  return (
    <div class="lens-report-editor" data-testid="report-editor">
      <header class="lens-report-editor-header">
        <div class="lens-report-editor-identity">
          <label class="lens-report-editor-title-label" for={`${session.instructionsId}-title`}>{translate('report.editor.name', 'Dashboard name')}</label>
          <input
            class="lens-report-editor-title"
            id={`${session.instructionsId}-title`}
            onChange={(event) => {
              const value = event.currentTarget.value.trim()
              if (value) session.edit((composition) => ({ ...composition, title: value }))
              else event.currentTarget.value = draft.value().title
            }}
            value={draft.value().title}
          />
          <p aria-live="polite" class="lens-report-editor-status" data-dirty={draft.dirty() || undefined} data-testid="report-editor-status">{session.status()}</p>
        </div>
        <div class="lens-report-editor-actions">
          <Show when={props.catalog}>
            <button class="lens-context-action" onClick={() => session.setPicking(true)} ref={(element) => { session.refs.add = element }} type="button">
              {translate('report.editor.add', 'Add report')}
            </button>
          </Show>
          <button class="lens-context-action" disabled={draft.saving() || !draft.dirty()} onClick={() => session.cancel()} type="button">
            {translate('report.editor.cancel', 'Cancel')}
          </button>
          <SessionSaveActions onSave={(intent) => void session.save(intent)} session={session} />
        </div>
      </header>

      <SessionAlerts session={session} />
      <SessionProposal session={session} />

      <Show
        fallback={(
          <div class="lens-report-editor-empty" data-testid="report-editor-empty">
            <p>{translate('report.editor.empty', 'This dashboard has no reports yet.')}</p>
          </div>
        )}
        when={session.ids().length > 0}
      >
        <div class="lens-report-editor-grid" ref={(element) => { gridRef = element }}>
          <For each={session.ids()}>
            {(id) => (
              <Show when={session.byId(id)}>
                {(current) => <EditorCard current={current().placement} grid={() => gridRef} id={id} render={props.renderPlacement} row={current().row} session={session} />}
              </Show>
            )}
          </For>
        </div>
      </Show>

      <SessionDialogs guard={guard} session={session} />
    </div>
  )
}

function EditorCard(props: {
  id: string
  current: Placement
  row: number
  session: CompositionSession
  grid: () => HTMLElement | undefined
  render?: (placement: Placement) => JSX.Element
}): JSX.Element {
  const translate = useTranslate()
  const session = props.session
  const title = () => session.title(props.current)
  const period = () => periodText(props.current.filters?.period, translate)
  const controls = createPlacementControls({
    id: () => props.id,
    span: () => spanOf(props.current),
    grid: props.grid,
    onMove: (direction) => session.move(props.id, direction),
    onResize: (span) => session.resize(props.id, span),
    onDropBefore: (dragged) => session.moveBefore(dragged, props.id),
  })

  return (
    <section
      aria-label={title()}
      class="lens-report-editor-card"
      data-added={session.added().includes(props.id) || undefined}
      data-drop-target={controls.dropTarget() || undefined}
      data-placement-editor={props.id}
      onDragLeave={controls.dropProps.onDragLeave}
      onDragOver={controls.dropProps.onDragOver}
      onDrop={controls.dropProps.onDrop}
      ref={(element) => session.cards.set(props.id, element)}
      style={{ '--lens-editor-span': String(controls.span()), '--lens-editor-row': String(props.row + 1) }}
    >
      <header class="lens-report-editor-card-header">
        <PlacementHandle controls={controls} describedBy={session.instructionsId} handleRef={(element) => session.handles.set(props.id, element)} id={props.id} title={title()} />
        <h3 class="lens-report-editor-card-title">{title()}</h3>
        <Show when={period()}>
          <span class="lens-report-picker-badge" data-testid={`report-editor-period-${props.id}`}>
            {translate('report.editor.cardPeriod', 'Card period: {period}', { period: period()! })}
          </span>
        </Show>
        <div class="lens-report-editor-card-actions">
          <button class="lens-context-action" onClick={() => session.setConfiguring(props.id)} type="button">
            {translate('report.action.configure', 'Configure card')}
          </button>
          <Show when={session.host.onEditReport}>
            <button class="lens-context-action" onClick={() => void session.openImpact(props.id, props.current.reportId)} type="button">
              {translate('report.action.edit', 'Edit report')}
            </button>
          </Show>
          <button
            aria-label={translate('report.editor.remove', 'Remove {title}', { title: title() })}
            class="lens-context-action"
            data-danger
            onClick={() => session.remove(props.id)}
            type="button"
          >
            {translate('report.editor.removeShort', 'Remove')}
          </button>
        </div>
      </header>
      <div class="lens-report-editor-card-body">
        <Show fallback={<p class="lens-report-editor-card-placeholder">{title()}</p>} when={props.render}>
          {(render) => render()(props.current)}
        </Show>
      </div>
      <PlacementResize controls={controls} id={props.id} title={title()} />
    </section>
  )
}
