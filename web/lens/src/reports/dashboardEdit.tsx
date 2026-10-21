import { createEffect, createSignal, mergeProps, on, onCleanup, Show, untrack, type Accessor, type JSX } from 'solid-js'
import type { DashboardDocument } from '../contract/types'
import { PencilSimple, Plus } from '../icons'
import { LensDashboard } from '../LensDashboard'
import { useTranslate, type LensThemeMode } from '../runtime'
import type { Placement } from './contract/types'
import type { SaveIntent } from './draft'
import type { LensDashboardEditing } from './dashboardEditTypes'
import {
  createCompositionSession, createPlacementControls, PlacementHandle, PlacementResize, SessionAlerts, SessionDialogs, SessionProposal, SessionSaveActions,
  type CompositionSession, type CompositionSessionHost,
} from './editing'
import { createReportLeaveGuard } from './guard'
import { spanOf } from './layout'
import type { ReportPlacementHost } from './placementHost'

// The edit mode is its own chunk: LensDashboard loads it only for a host that
// passes `edit`, so a read-only board does not carry the picker and dialogs.

/** The live dashboard's editing state: a composition session plus the mode's own focus, Escape and lifecycle. */
function createDashboardEditor(edit: LensDashboardEditing, theme: Accessor<LensThemeMode>) {
  const translate = useTranslate()
  const host = mergeProps(edit, { get theme() { return theme() } }) as CompositionSessionHost
  const session = createCompositionSession(host)
  const draft = session.draft
  const guard = untrack(() => edit.guard) ?? createReportLeaveGuard(draft)
  const refs: { toggle?: HTMLElement; cancel?: HTMLElement } = {}
  const [modeAnnouncement, setModeAnnouncement] = createSignal('')
  const editing = () => edit.editing()
  const dialogOpen = () => session.picking() || Boolean(session.configuring()) || Boolean(session.impact()) || guard.pending()

  const start = () => {
    if (!editing()) edit.setEditing(true)
  }

  const addReport = (trigger?: HTMLElement) => {
    if (trigger) session.refs.add = trigger
    start()
    session.setPicking(true)
  }

  const cancel = () => {
    if (draft.saving()) return
    session.cancel({ announce: false })
    edit.setEditing(false)
    edit.onCancel?.()
  }

  const save = async (intent: SaveIntent) => {
    if (!await session.save(intent)) return
    edit.setEditing(false)
    edit.onSaved?.(draft.saved(), intent)
  }

  createEffect(on(editing, (now, before) => {
    if (now === before) return
    setModeAnnouncement(now
      ? translate('dashboard.edit.started', 'Editing the dashboard. Move, resize or remove cards, then save.')
      : translate('dashboard.edit.ended', 'Finished editing the dashboard.'))
    requestAnimationFrame(() => {
      if (now) {
        if (dialogOpen()) return
        const first = session.ids()[0]
        ;(first ? session.handles.get(first) : session.refs.add ?? refs.cancel)?.focus()
      } else if (!dialogOpen()) refs.toggle?.focus()
    })
  }, { defer: true }))

  createEffect(() => {
    if (!editing() || typeof document === 'undefined') return
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape' || event.defaultPrevented || dialogOpen() || draft.saving()) return
      const target = event.target instanceof Element ? event.target : undefined
      if (target?.closest('.lens-overlay-root, input, textarea, select, [contenteditable="true"]')) return
      event.preventDefault()
      cancel()
    }
    document.addEventListener('keydown', onKeyDown)
    onCleanup(() => document.removeEventListener('keydown', onKeyDown))
  })

  /**
   * The card menu while editing: configure and remove act on the draft, edit
   * report asks about the shared definition first, copy is the host's. Every
   * other member is the host's own placement host.
   */
  const placementHost = (outer: ReportPlacementHost | undefined): ReportPlacementHost => ({
    status: outer?.status ? (id) => outer.status!(id) : undefined,
    update: outer?.update,
    keep: outer?.keep,
    unavailable: outer?.unavailable ? (id) => outer.unavailable!(id) : undefined,
    capabilities: outer?.capabilities ? (id) => outer.capabilities!(id) : undefined,
    get actions() {
      if (!editing()) return outer?.actions
      const reportOf = (id: string) => session.byId(id)?.placement.reportId ?? id
      return {
        open: outer?.actions?.open,
        configure: (id: string) => { session.setConfiguring(id) },
        remove: (id: string) => { session.remove(id) },
        edit: host.onEditReport ? (id: string) => void session.openImpact(id, reportOf(id)) : undefined,
        copy: host.onCopyReport ? (id: string) => host.onCopyReport!(id) : undefined,
      }
    },
  })

  /** The edit chrome of one card: its handle, width slider and drop target, bound to the draft. */
  const cardEdit = (id: string, options: { title: () => string; preview: () => boolean; grid: () => HTMLElement | undefined }) => {
    const current = () => session.byId(id)
    const controls = createPlacementControls({
      id: () => id,
      span: () => spanOf(current()?.placement ?? {}),
      grid: options.grid,
      onMove: (direction) => session.move(id, direction),
      onResize: (span) => session.resize(id, span),
      onDropBefore: (dragged) => session.moveBefore(dragged, id),
    })
    return {
      get editing() { return editing() },
      get row() { return current()?.row ?? 0 },
      get preview() { return options.preview() },
      get added() { return session.added().includes(id) },
      span: controls.span,
      dropTarget: controls.dropTarget,
      dropProps: controls.dropProps,
      handle: () => <PlacementHandle controls={controls} describedBy={session.instructionsId} handleRef={(element) => session.handles.set(id, element)} id={id} title={options.title()} />,
      resize: () => <PlacementResize controls={controls} id={id} title={options.title()} />,
      cardRef: (element: HTMLElement) => session.cards.set(id, element),
    }
  }

  return { edit, session, guard, refs, editing, start, addReport, cancel, save, placementHost, modeAnnouncement, cardEdit }
}

type EditorState = ReturnType<typeof createDashboardEditor>

/** What the dashboard renders of the edit mode, bound to one editor. */
export interface DashboardEditor extends EditorState {
  Toggle: () => JSX.Element
  Bar: () => JSX.Element
  Notices: () => JSX.Element
  Dialogs: () => JSX.Element
  Added: (props: { placement: Placement; theme: LensThemeMode; locale?: string; csrf?: string; fetcher?: typeof fetch }) => JSX.Element
}

/** Creates the editor inside the dashboard's runtime and hands it to the board. */
export function DashboardEditScope(props: { edit: LensDashboardEditing; theme: Accessor<LensThemeMode>; children: (editor: DashboardEditor) => JSX.Element }): JSX.Element {
  const state = createDashboardEditor(untrack(() => props.edit), props.theme)
  const editor: DashboardEditor = {
    ...state,
    Toggle: () => <DashboardEditToggle editor={state} />,
    Bar: () => <DashboardEditBar editor={state} />,
    Notices: () => (
      <div class="lens-dashboard-edit-notices">
        <SessionAlerts session={state.session} />
        <SessionProposal session={state.session} />
      </div>
    ),
    Dialogs: () => <SessionDialogs guard={state.guard} session={state.session} />,
    Added: (added) => <AddedPlacementBody {...added} edit={state.edit} session={state.session} />,
  }
  return <>{untrack(() => props.children(editor))}</>
}

/** The view mode's Edit action. */
function DashboardEditToggle(props: { editor: EditorState }): JSX.Element {
  const translate = useTranslate()
  return (
    <button
      class="lens-export-button lens-dashboard-edit-toggle"
      data-testid="dashboard-edit"
      onClick={() => props.editor.start()}
      ref={(element) => { props.editor.refs.toggle = element }}
      type="button"
    >
      <PencilSimple />
      <span>{translate('dashboard.edit', 'Edit')}</span>
    </button>
  )
}

/** The edit mode's toolbar: whether anything changed, Add report, Cancel and the save actions. */
function DashboardEditBar(props: { editor: EditorState }): JSX.Element {
  const translate = useTranslate()
  const session = props.editor.session
  const draft = session.draft
  return (
    <div aria-label={translate('dashboard.edit.toolbar', 'Dashboard editing')} class="lens-dashboard-edit-bar" data-testid="dashboard-edit-bar" role="toolbar">
      <p aria-live="polite" class="lens-report-editor-status lens-dashboard-edit-status" data-dirty={draft.dirty() || undefined} data-testid="report-editor-status">{session.status()}</p>
      <Show when={session.host.catalog}>
        <button class="lens-context-action" data-testid="dashboard-edit-add" onClick={(event) => props.editor.addReport(event.currentTarget)} ref={(element) => { session.refs.add = element }} type="button">
          <Plus />
          <span>{translate('report.editor.add', 'Add report')}</span>
        </button>
      </Show>
      <button class="lens-context-action" data-testid="dashboard-edit-cancel" disabled={draft.saving()} onClick={() => props.editor.cancel()} ref={(element) => { props.editor.refs.cancel = element }} type="button">
        {translate('report.editor.cancel', 'Cancel')}
      </button>
      <SessionSaveActions onSave={(intent) => void props.editor.save(intent)} session={session} />
    </div>
  )
}

/**
 * The body of a card added in this session: the document of its report,
 * fetched once, rendered as a nested dashboard without its own header.
 */
function AddedPlacementBody(props: {
  placement: Placement
  session: CompositionSession
  edit: LensDashboardEditing
  theme: LensThemeMode
  locale?: string
  csrf?: string
  fetcher?: typeof fetch
}): JSX.Element {
  const translate = useTranslate()
  const placement = untrack(() => props.placement)
  const entry = untrack(() => props.session.pickedEntry(placement.id))
  const controller = new AbortController()
  onCleanup(() => controller.abort())
  const load = (): Promise<DashboardDocument> | undefined => {
    if (props.edit.placementDocument) return props.edit.placementDocument(placement, { signal: controller.signal })
    const catalog = props.session.host.catalog
    return entry && catalog?.preview ? catalog.preview(entry, { signal: controller.signal }) : undefined
  }
  const pending = load()
  const [document, setDocument] = createSignal<DashboardDocument>()
  const [failed, setFailed] = createSignal(false)
  pending?.then((value) => setDocument(() => value), (cause: unknown) => {
    if (controller.signal.aborted) return
    console.error(`[lens] the document of placement ${placement.id} failed`, cause)
    setFailed(true)
  })
  return (
    <div class="lens-dashboard-edit-preview" data-testid={`dashboard-edit-preview-${placement.id}`}>
      <Show
        fallback={<p class="lens-dashboard-edit-preview-state">{translate('dashboard.edit.previewPending', 'The figures appear once the dashboard is saved.')}</p>}
        when={pending}
      >
        <Show
          fallback={(
            <Show
              fallback={<p aria-busy="true" class="lens-dashboard-edit-preview-state">{translate('dashboard.edit.previewLoading', 'Loading the report…')}</p>}
              when={failed()}
            >
              <p class="lens-dashboard-edit-preview-state" role="alert">{translate('dashboard.edit.previewFailed', 'The report could not be loaded. It stays in the draft.')}</p>
            </Show>
          )}
          when={document()}
        >
          {(value) => (
            <LensDashboard chrome={{ header: false }} csrf={props.csrf} fetcher={props.fetcher} initialDocument={value()} locale={props.locale} theme={props.theme} />
          )}
        </Show>
      </Show>
    </div>
  )
}
