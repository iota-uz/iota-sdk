import { createMemo, createSignal, createUniqueId, For, Show, untrack, type JSX } from 'solid-js'
import { compactRangeLabel, parseISODate } from '../controls/model'
import { LensMessagesProvider, useTranslate, type LensThemeMode } from '../runtime'
import type { Composition, ContextPeriod, Placement, Proposal } from './contract/types'
import { parseReportComposition, reportDraftToken, type ReportDraft, type SaveIntent } from './draft'
import { createReportLeaveGuard, type ReportLeaveGuard } from './guard'
import {
  configurePlacement, diffCompositions, fullSpan, minimumSpan, movePlacement, movePlacementBefore, removePlacement, resizePlacement, spanOf,
  type CompositionChange, type PlacementDirection,
} from './layout'
import { appendReportPicks, type ReportCatalogHost, type ReportPickerConfirmation } from './picker'
import { ReportDialog } from './ReportDialog'
import { ReportPicker } from './ReportPicker'

export type ReportProposal = Proposal

/** new: an unpublished draft (Save draft / Publish). existing: a saved object (Save changes / Cancel). */
export type ReportEditorLifecycle = 'new' | 'existing'

export interface ReportProposalHandlers {
  /** Called after the reader applied or rejected the proposal. */
  settled?: (proposalId: string, outcome: 'applied' | 'rejected') => void
  /** Prepare the proposal again against the current draft (reportDraftToken). */
  regenerate?: (proposal: ReportProposal) => void
  /** Keep a stale proposal as a separate draft instead of applying it here. */
  openAsDraft?: (proposal: ReportProposal) => void
}

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

/** Edits a composition draft: layout, captions, card-local periods; Save and Cancel act on the whole session. */
export function ReportCompositionEditor(props: ReportCompositionEditorProps): JSX.Element {
  return (
    <LensMessagesProvider messages={props.messages}>
      <Editor {...props} />
    </LensMessagesProvider>
  )
}

function periodText(period: ContextPeriod | undefined, translate: ReturnType<typeof useTranslate>): string | undefined {
  if (!period) return undefined
  if (period.mode === 'fixed' && period.start && period.end) {
    const start = parseISODate(period.start)
    const end = parseISODate(period.end)
    if (start && end) return compactRangeLabel(start, end)
  }
  if (period.mode === 'relative' && period.days) return translate('report.editor.period.days', 'Last {count} days', { count: period.days })
  if (period.mode === 'relative' && period.months) return translate('report.editor.period.months', 'Last {count} months', { count: period.months })
  return translate('report.editor.period.custom', 'Card period')
}

function Editor(props: ReportCompositionEditorProps): JSX.Element {
  const translate = useTranslate()
  const draft = untrack(() => props.draft)
  const guard = untrack(() => props.guard) ?? createReportLeaveGuard(draft)
  const [picking, setPicking] = createSignal(false)
  const [configuring, setConfiguring] = createSignal<string>()
  const [impact, setImpact] = createSignal<{ placementId: string; reportId: string; usage?: number }>()
  const [notice, setNotice] = createSignal('')
  const [announcement, setAnnouncement] = createSignal('')
  const [added, setAdded] = createSignal<string[]>([])
  const [saved, setSaved] = createSignal(false)
  const [settledProposal, setSettledProposal] = createSignal<string>()
  const [copyFailed, setCopyFailed] = createSignal(false)
  const cards = new Map<string, HTMLElement>()
  const handles = new Map<string, HTMLButtonElement>()
  let addButton: HTMLButtonElement | undefined
  let gridRef: HTMLDivElement | undefined
  const instructionsId = createUniqueId()

  const placements = createMemo(() => draft.value().rows.flatMap((row, rowIndex) => row.map((placement, index) => ({ placement, row: rowIndex, index, rowSize: row.length }))))
  const ids = createMemo(() => placements().map((item) => item.placement.id), undefined, { equals: (a, b) => a.length === b.length && a.every((id, index) => id === b[index]) })
  const byId = (id: string) => placements().find((item) => item.placement.id === id)
  const title = (placement: Placement) => placement.title ?? props.reportTitle(placement.reportId) ?? placement.reportId
  const placedCount = (reportId: string) => draft.value().rows.flat().filter((placement) => placement.reportId === reportId).length

  const edit = (change: (composition: Composition) => Composition) => {
    setSaved(false)
    setAdded([])
    setNotice('')
    draft.update(change)
  }

  const focusHandle = (id: string) => requestAnimationFrame(() => handles.get(id)?.focus())

  const move = (id: string, direction: PlacementDirection) => {
    const before = byId(id)
    edit((composition) => movePlacement(composition, id, direction))
    const after = byId(id)
    if (after && before && (after.row !== before.row || after.index !== before.index)) {
      setAnnouncement(translate('report.editor.moved', 'Moved {title} to row {row}, position {position}', { title: title(after.placement), row: after.row + 1, position: after.index + 1 }))
    }
    focusHandle(id)
  }

  const resize = (id: string, span: number) => {
    edit((composition) => resizePlacement(composition, id, span))
    const after = byId(id)
    if (after) setAnnouncement(translate('report.editor.resized', '{title} is {span} of 12 columns wide', { title: title(after.placement), span: spanOf(after.placement) }))
  }

  const remove = (id: string) => {
    const list = ids()
    const position = list.indexOf(id)
    const removed = byId(id)
    edit((composition) => removePlacement(composition, id))
    if (removed) setAnnouncement(translate('report.editor.removed', 'Removed {title} from the draft', { title: title(removed.placement) }))
    const next = list[position + 1] ?? list[position - 1]
    requestAnimationFrame(() => (next ? handles.get(next) : addButton)?.focus())
  }

  const addPicked = (confirmation: ReportPickerConfirmation) => {
    let placementsAdded: Placement[] = []
    edit((composition) => {
      const result = appendReportPicks(composition, confirmation, { span: props.addSpan })
      placementsAdded = result.added
      return result.composition
    })
    const newIds = placementsAdded.map((placement) => placement.id)
    setAdded(newIds)
    setNotice(translate('report.editor.added', 'Added {count} reports to the draft', { count: newIds.length }))
    const first = newIds[0]
    if (first) {
      requestAnimationFrame(() => {
        cards.get(first)?.scrollIntoView({ block: 'nearest' })
        handles.get(first)?.focus({ preventScroll: true })
      })
    }
  }

  const save = async (intent: SaveIntent) => {
    setSaved(false)
    setNotice('')
    try {
      await draft.save(intent)
      setSaved(true)
    } catch (cause: unknown) {
      console.error('[lens] saving the composition failed', cause)
    }
  }

  const cancel = () => {
    setSaved(false)
    setAdded([])
    draft.cancel()
    setNotice(translate('report.editor.cancelled', 'Changes discarded. The saved version is restored.'))
  }

  const openImpact = async (placementId: string, reportId: string) => {
    setImpact({ placementId, reportId })
    if (!props.usage) return
    try {
      const usage = await props.usage(reportId)
      if (impact()?.placementId === placementId) setImpact({ placementId, reportId, usage })
    } catch (cause: unknown) {
      console.error('[lens] report usage failed', cause)
    }
  }

  const conflict = () => draft.conflict()
  const saveError = () => Boolean(draft.error()) && !conflict()

  const status = () => {
    if (draft.saving()) return translate('report.editor.saving', 'Saving…')
    if (draft.dirty()) return translate('report.editor.unsaved', 'Unsaved changes')
    if (saved()) return translate('report.editor.saved', 'All changes saved')
    return props.lifecycle === 'new' ? translate('report.editor.draft', 'Draft') : translate('report.editor.upToDate', 'No changes')
  }

  const proposal = () => (props.proposal && props.proposal.id !== settledProposal() ? props.proposal : undefined)
  // A proposal comes from outside the editor: it must pass the same validation as a saved
  // composition (unique placement ids, rows within twelve columns, no empty rows) and
  // target this draft, or it never reaches the draft.
  const proposalValid = (value: ReportProposal): boolean => {
    try {
      return parseReportComposition(value.composition).id === draft.value().id
    } catch {
      return false
    }
  }

  return (
    <div class="lens-report-editor" data-testid="report-editor">
      <header class="lens-report-editor-header">
        <div class="lens-report-editor-identity">
          <label class="lens-report-editor-title-label" for={`${instructionsId}-title`}>{translate('report.editor.name', 'Dashboard name')}</label>
          <input
            class="lens-report-editor-title"
            id={`${instructionsId}-title`}
            onChange={(event) => {
              const value = event.currentTarget.value.trim()
              if (value) edit((composition) => ({ ...composition, title: value }))
              else event.currentTarget.value = draft.value().title
            }}
            value={draft.value().title}
          />
          <p aria-live="polite" class="lens-report-editor-status" data-dirty={draft.dirty() || undefined} data-testid="report-editor-status">{status()}</p>
        </div>
        <div class="lens-report-editor-actions">
          <Show when={props.catalog}>
            <button class="lens-context-action" onClick={() => setPicking(true)} ref={(element) => { addButton = element }} type="button">
              {translate('report.editor.add', 'Add report')}
            </button>
          </Show>
          <button class="lens-context-action" disabled={draft.saving() || !draft.dirty()} onClick={cancel} type="button">
            {translate('report.editor.cancel', 'Cancel')}
          </button>
          <Show
            fallback={(
              <button aria-busy={draft.saving() || undefined} class="lens-context-action lens-report-primary" disabled={draft.saving() || !draft.dirty()} onClick={() => void save('draft')} type="button">
                {translate('report.editor.saveChanges', 'Save changes')}
              </button>
            )}
            when={props.lifecycle === 'new'}
          >
            <button aria-busy={draft.saving() || undefined} class="lens-context-action" disabled={draft.saving()} onClick={() => void save('draft')} type="button">
              {translate('report.editor.saveDraft', 'Save draft')}
            </button>
            <button aria-busy={draft.saving() || undefined} class="lens-context-action lens-report-primary" disabled={draft.saving()} onClick={() => void save('publish')} type="button">
              {translate('report.editor.publish', 'Publish')}
            </button>
          </Show>
        </div>
      </header>

      <Show when={saveError()}>
        <p class="lens-report-editor-alert" data-tone="error" role="alert">
          {translate('report.editor.saveFailed', 'The changes could not be saved. They are kept here; try again.')}
        </p>
      </Show>
      <Show when={conflict()}>
        {(value) => (
          <div class="lens-report-editor-alert" data-tone="warn" data-testid="report-editor-conflict" role="alert">
            <p>{translate('report.editor.conflict', 'Someone else saved this dashboard while you were editing. Your changes are kept.')}</p>
            <div class="lens-report-editor-alert-actions">
              <Show when={value().latest}>
                {(latest) => (
                  <>
                    <button class="lens-context-action" onClick={() => draft.rebase(latest(), 'discard')} type="button">
                      {translate('report.editor.conflictReload', 'Load their version')}
                    </button>
                    <button class="lens-context-action lens-report-primary" onClick={() => draft.rebase(latest(), 'keep')} type="button">
                      {translate('report.editor.conflictKeep', 'Keep my changes')}
                    </button>
                  </>
                )}
              </Show>
              <Show when={props.onSaveCopy}>
                <button
                  class="lens-context-action"
                  onClick={() => {
                    setCopyFailed(false)
                    Promise.resolve(props.onSaveCopy!(draft.value())).catch((cause: unknown) => {
                      console.error('[lens] saving an independent copy failed', cause)
                      setCopyFailed(true)
                    })
                  }}
                  type="button"
                >
                  {translate('report.editor.conflictCopy', 'Save as a separate copy')}
                </button>
              </Show>
            </div>
            <Show when={copyFailed()}>
              <p>{translate('report.editor.saveFailed', 'The changes could not be saved. They are kept here; try again.')}</p>
            </Show>
          </div>
        )}
      </Show>
      <Show when={notice()}>
        <p class="lens-report-editor-notice" data-testid="report-editor-notice" role="status">{notice()}</p>
      </Show>
      <p aria-live="polite" class="lens-visually-hidden" data-testid="report-editor-announcement">{announcement()}</p>
      <p class="lens-visually-hidden" id={instructionsId}>
        {translate('report.editor.moveHint', 'Use the arrow keys to move the card. Drag the handle with a pointer.')}
      </p>

      <Show when={proposal()}>
        {(value) => (
          <ProposalPanel
            base={draft.value()}
            handlers={props.proposalHandlers}
            invalid={!proposalValid(value())}
            onApply={() => {
              const current = value()
              if (!proposalValid(current) || current.base !== reportDraftToken(draft)) return
              edit((composition) => ({ ...structuredClone(current.composition), id: composition.id, revision: composition.revision }))
              setSettledProposal(current.id)
              setNotice(translate('report.editor.proposalApplied', 'The proposal was applied to the draft. Nothing is published until you save.'))
              props.proposalHandlers?.settled?.(current.id, 'applied')
            }}
            onReject={() => {
              const current = value()
              setSettledProposal(current.id)
              props.proposalHandlers?.settled?.(current.id, 'rejected')
            }}
            proposal={value()}
            stale={value().base !== reportDraftToken(draft)}
            title={(placementId, reportId) => {
              const placement = [...value().composition.rows.flat(), ...draft.value().rows.flat()].find((candidate) => candidate.id === placementId)
              return placement ? title(placement) : props.reportTitle(reportId) ?? reportId
            }}
          />
        )}
      </Show>

      <Show
        fallback={(
          <div class="lens-report-editor-empty" data-testid="report-editor-empty">
            <p>{translate('report.editor.empty', 'This dashboard has no reports yet.')}</p>
          </div>
        )}
        when={ids().length > 0}
      >
        <div class="lens-report-editor-grid" ref={(element) => { gridRef = element }}>
          <For each={ids()}>
            {(id) => {
              const item = () => byId(id)
              return (
                <Show when={item()}>
                  {(current) => (
                    <EditorCard
                      added={added().includes(id)}
                      cardRef={(element) => cards.set(id, element)}
                      describedBy={instructionsId}
                      grid={() => gridRef}
                      handleRef={(element) => handles.set(id, element)}
                      index={current().index}
                      onConfigure={() => setConfiguring(id)}
                      onDropBefore={(dragged) => edit((composition) => movePlacementBefore(composition, dragged, id))}
                      onEditReport={props.onEditReport ? () => void openImpact(id, current().placement.reportId) : undefined}
                      onMove={(direction) => move(id, direction)}
                      onRemove={() => remove(id)}
                      onResize={(span) => resize(id, span)}
                      period={periodText(current().placement.filters?.period, translate)}
                      placement={current().placement}
                      render={props.renderPlacement}
                      row={current().row}
                      title={title(current().placement)}
                    />
                  )}
                </Show>
              )
            }}
          </For>
        </div>
      </Show>

      <Show when={picking() && props.catalog}>
        <ReportPicker
          host={props.catalog!}
          onClose={() => setPicking(false)}
          onConfirm={addPicked}
          placed={placedCount}
          restoreFocus={addButton}
          target={{ id: draft.value().id, title: draft.value().title }}
          theme={props.theme}
        />
      </Show>
      <Show when={configuring() ? byId(configuring()!) : undefined}>
        {(current) => (
          <ConfigureDialog
            onApply={(settings) => {
              edit((composition) => configurePlacement(composition, current().placement.id, settings))
              setConfiguring(undefined)
            }}
            onClose={() => setConfiguring(undefined)}
            placement={current().placement}
            reportTitle={props.reportTitle(current().placement.reportId) ?? current().placement.reportId}
            restoreFocus={handles.get(current().placement.id)}
            theme={props.theme}
          />
        )}
      </Show>
      <Show when={impact()}>
        {(value) => (
          <ImpactDialog
            onClose={() => setImpact(undefined)}
            onCopy={props.onCopyReport ? () => props.onCopyReport!(value().placementId) : undefined}
            onEdit={() => props.onEditReport!(value().reportId, value().placementId)}
            restoreFocus={handles.get(value().placementId)}
            theme={props.theme}
            title={props.reportTitle(value().reportId) ?? value().reportId}
            usage={value().usage}
          />
        )}
      </Show>
      <Show when={guard.pending()}>
        <LeaveDialog guard={guard} lifecycle={props.lifecycle} theme={props.theme} />
      </Show>
    </div>
  )
}

const spanSteps = [3, 4, 6, 8, 9, 12]

function nextSpan(span: number, direction: 1 | -1): number {
  if (direction > 0) return spanSteps.find((step) => step > span) ?? fullSpan
  return [...spanSteps].reverse().find((step) => step < span) ?? minimumSpan
}

function EditorCard(props: {
  placement: Placement
  title: string
  period?: string
  row: number
  index: number
  added: boolean
  describedBy: string
  render?: (placement: Placement) => JSX.Element
  grid: () => HTMLElement | undefined
  cardRef: (element: HTMLElement) => void
  handleRef: (element: HTMLButtonElement) => void
  onMove: (direction: PlacementDirection) => void
  onResize: (span: number) => void
  onDropBefore: (draggedId: string) => void
  onConfigure: () => void
  onEditReport?: () => void
  onRemove: () => void
}): JSX.Element {
  const translate = useTranslate()
  const [preview, setPreview] = createSignal<number>()
  const [dropTarget, setDropTarget] = createSignal(false)
  const span = () => preview() ?? spanOf(props.placement)

  const onHandleKey = (event: KeyboardEvent) => {
    const direction = ({ ArrowLeft: 'left', ArrowRight: 'right', ArrowUp: 'up', ArrowDown: 'down' } as const)[event.key as 'ArrowLeft']
    if (!direction) return
    event.preventDefault()
    props.onMove(direction)
  }

  const onResizeKey = (event: KeyboardEvent) => {
    const step = event.key === 'ArrowRight' || event.key === 'ArrowUp' ? 1 : event.key === 'ArrowLeft' || event.key === 'ArrowDown' ? -1 : 0
    if (step) {
      event.preventDefault()
      props.onResize(nextSpan(spanOf(props.placement), step))
    } else if (event.key === 'Home' || event.key === 'End') {
      event.preventDefault()
      props.onResize(event.key === 'Home' ? minimumSpan : fullSpan)
    }
  }

  const onResizePointer = (event: PointerEvent) => {
    const grid = props.grid()
    if (!grid || event.button !== 0) return
    event.preventDefault()
    const target = event.currentTarget as HTMLElement
    target.setPointerCapture?.(event.pointerId)
    const column = grid.getBoundingClientRect().width / fullSpan
    const start = event.clientX
    const initial = spanOf(props.placement)
    const width = (clientX: number) => Math.max(minimumSpan, Math.min(fullSpan, Math.round(initial + (clientX - start) / column)))
    const move = (next: PointerEvent) => setPreview(width(next.clientX))
    const finish = (next: PointerEvent) => {
      target.removeEventListener('pointermove', move)
      target.removeEventListener('pointerup', finish)
      target.removeEventListener('pointercancel', cancel)
      setPreview(undefined)
      const value = width(next.clientX)
      if (value !== initial) props.onResize(value)
    }
    const cancel = () => {
      target.removeEventListener('pointermove', move)
      target.removeEventListener('pointerup', finish)
      target.removeEventListener('pointercancel', cancel)
      setPreview(undefined)
    }
    target.addEventListener('pointermove', move)
    target.addEventListener('pointerup', finish)
    target.addEventListener('pointercancel', cancel)
  }

  return (
    <section
      aria-label={props.title}
      class="lens-report-editor-card"
      data-added={props.added || undefined}
      data-drop-target={dropTarget() || undefined}
      data-placement-editor={props.placement.id}
      onDragLeave={() => setDropTarget(false)}
      onDragOver={(event) => {
        if (!event.dataTransfer?.types.includes('application/x-lens-placement')) return
        event.preventDefault()
        setDropTarget(true)
      }}
      onDrop={(event) => {
        setDropTarget(false)
        const dragged = event.dataTransfer?.getData('application/x-lens-placement')
        if (!dragged) return
        event.preventDefault()
        props.onDropBefore(dragged)
      }}
      ref={props.cardRef}
      style={{ '--lens-editor-span': String(span()), '--lens-editor-row': String(props.row + 1) }}
    >
      <header class="lens-report-editor-card-header">
        <button
          aria-describedby={props.describedBy}
          aria-label={translate('report.editor.move', 'Move {title}', { title: props.title })}
          class="lens-icon-button lens-report-editor-handle"
          data-testid={`report-editor-handle-${props.placement.id}`}
          draggable="true"
          onDragStart={(event) => {
            event.dataTransfer?.setData('application/x-lens-placement', props.placement.id)
            if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move'
          }}
          onKeyDown={onHandleKey}
          ref={props.handleRef}
          title={translate('report.editor.move', 'Move {title}', { title: props.title })}
          type="button"
        >
          <span aria-hidden="true" class="lens-report-editor-grip" />
        </button>
        <h3 class="lens-report-editor-card-title">{props.title}</h3>
        <Show when={props.period}>
          <span class="lens-report-picker-badge" data-testid={`report-editor-period-${props.placement.id}`}>
            {translate('report.editor.cardPeriod', 'Card period: {period}', { period: props.period! })}
          </span>
        </Show>
        <div class="lens-report-editor-card-actions">
          <button class="lens-context-action" onClick={() => props.onConfigure()} type="button">
            {translate('report.action.configure', 'Configure card')}
          </button>
          <Show when={props.onEditReport}>
            <button class="lens-context-action" onClick={() => props.onEditReport?.()} type="button">
              {translate('report.action.edit', 'Edit report')}
            </button>
          </Show>
          <button
            aria-label={translate('report.editor.remove', 'Remove {title}', { title: props.title })}
            class="lens-context-action"
            data-danger
            onClick={() => props.onRemove()}
            type="button"
          >
            {translate('report.editor.removeShort', 'Remove')}
          </button>
        </div>
      </header>
      <div class="lens-report-editor-card-body">
        <Show fallback={<p class="lens-report-editor-card-placeholder">{props.title}</p>} when={props.render}>
          {(render) => render()(props.placement)}
        </Show>
      </div>
      <div
        aria-label={translate('report.editor.width', 'Width of {title}', { title: props.title })}
        aria-orientation="horizontal"
        aria-valuemax={fullSpan}
        aria-valuemin={minimumSpan}
        aria-valuenow={span()}
        aria-valuetext={translate('report.editor.widthValue', '{span} of 12 columns', { span: span() })}
        class="lens-report-editor-resize"
        data-testid={`report-editor-resize-${props.placement.id}`}
        onKeyDown={onResizeKey}
        onPointerDown={onResizePointer}
        role="slider"
        tabIndex={0}
      />
    </section>
  )
}

function ProposalPanel(props: {
  proposal: ReportProposal
  base: Composition
  stale: boolean
  invalid: boolean
  handlers?: ReportProposalHandlers
  title: (placementId: string, reportId: string) => string
  onApply: () => void
  onReject: () => void
}): JSX.Element {
  const translate = useTranslate()
  const changes = () => (props.invalid ? [] : diffCompositions(props.base, props.proposal.composition))
  const blocked = () => props.stale || props.invalid
  const kind = (change: CompositionChange) => {
    switch (change.kind) {
      case 'add': return translate('report.proposal.add', 'Add')
      case 'change': return translate('report.proposal.change', 'Change')
      case 'remove': return translate('report.proposal.remove', 'Remove')
    }
  }
  return (
    <section aria-label={translate('report.proposal.title', 'Proposed changes')} class="lens-report-proposal" data-invalid={props.invalid || undefined} data-stale={blocked() || undefined} data-testid="report-proposal">
      <h3 class="lens-report-proposal-title">{translate('report.proposal.title', 'Proposed changes')}</h3>
      <Show when={props.proposal.summary}>
        <p class="lens-report-proposal-summary">{props.proposal.summary}</p>
      </Show>
      <Show
        fallback={(
          <Show when={!props.invalid}>
            <p class="lens-report-proposal-summary">{translate('report.proposal.none', 'The proposal does not change the draft.')}</p>
          </Show>
        )}
        when={changes().length > 0}
      >
        <ul class="lens-report-proposal-list">
          <For each={changes()}>
            {(change) => (
              <li data-kind={change.kind}>
                <span class="lens-report-picker-badge" data-tone={change.kind}>{kind(change)}</span>
                <span>{props.title(change.placementId, change.reportId)}</span>
              </li>
            )}
          </For>
        </ul>
      </Show>
      <Show when={props.invalid}>
        <p class="lens-report-proposal-stale" role="alert">
          {translate('report.proposal.invalid', 'This proposal cannot be applied: its layout is not valid for this dashboard.')}
        </p>
      </Show>
      <Show when={props.stale && !props.invalid}>
        <p class="lens-report-proposal-stale" role="alert">
          {translate('report.proposal.stale', 'The draft changed after this proposal was prepared, so it can no longer be applied.')}
        </p>
      </Show>
      <div class="lens-report-proposal-actions">
        <Show
          fallback={(
            <>
              <Show when={props.handlers?.regenerate && !props.invalid}>
                <button class="lens-context-action" onClick={() => props.handlers?.regenerate?.(props.proposal)} type="button">
                  {translate('report.proposal.regenerate', 'Prepare again')}
                </button>
              </Show>
              <Show when={props.handlers?.openAsDraft}>
                <button class="lens-context-action" onClick={() => props.handlers?.openAsDraft?.(props.proposal)} type="button">
                  {translate('report.proposal.openAsDraft', 'Open as a separate draft')}
                </button>
              </Show>
            </>
          )}
          when={!blocked()}
        >
          <button class="lens-context-action lens-report-primary" disabled={changes().length === 0} onClick={() => props.onApply()} type="button">
            {translate('report.proposal.apply', 'Apply to draft')}
          </button>
        </Show>
        <button class="lens-context-action" onClick={() => props.onReject()} type="button">
          {translate('report.proposal.reject', 'Reject')}
        </button>
      </div>
    </section>
  )
}

function ConfigureDialog(props: {
  placement: Placement
  reportTitle: string
  restoreFocus?: HTMLElement
  theme?: LensThemeMode
  onApply: (settings: { title: string; span: number; period?: ContextPeriod }) => void
  onClose: () => void
}): JSX.Element {
  const translate = useTranslate()
  const id = createUniqueId()
  const existing = props.placement.filters?.period
  const [caption, setCaption] = createSignal(props.placement.title ?? '')
  const [span, setSpan] = createSignal(spanOf(props.placement))
  const [local, setLocal] = createSignal(Boolean(existing))
  const [start, setStart] = createSignal(existing?.mode === 'fixed' ? existing.start ?? '' : '')
  const [end, setEnd] = createSignal(existing?.mode === 'fixed' ? existing.end ?? '' : '')
  let captionRef: HTMLInputElement | undefined
  const invalid = () => local() && Boolean(start() || end()) && !(start() && end() && start() <= end())
  const period = (): ContextPeriod | undefined => {
    if (!local()) return undefined
    if (start() && end()) return { mode: 'fixed', start: start(), end: end() }
    return existing
  }
  const apply = () => {
    if (invalid() || (local() && !period())) return
    props.onApply({ title: caption(), span: span(), period: period() })
  }

  const footer = (
    <div class="lens-report-picker-actions">
      <button class="lens-context-action" onClick={() => props.onClose()} type="button">{translate('report.picker.cancel', 'Cancel')}</button>
      <button class="lens-context-action lens-report-primary" disabled={invalid() || (local() && !period())} onClick={apply} type="button">
        {translate('report.editor.configureApply', 'Apply to draft')}
      </button>
    </div>
  )

  return (
    <ReportDialog
      description={translate('report.editor.configureHint', 'These settings change only this card. The report stays as it is.')}
      footer={footer}
      initialFocus={() => captionRef}
      onClose={() => props.onClose()}
      restoreFocus={props.restoreFocus}
      testId="report-configure"
      theme={props.theme}
      title={translate('report.editor.configureTitle', 'Configure card: {title}', { title: props.placement.title ?? props.reportTitle })}
    >
      <div class="lens-report-form">
        <label class="lens-report-form-field" for={`${id}-caption`}>
          <span>{translate('report.editor.caption', 'Card caption')}</span>
          <input
            id={`${id}-caption`}
            onInput={(event) => setCaption(event.currentTarget.value)}
            placeholder={props.reportTitle}
            ref={(element) => { captionRef = element }}
            value={caption()}
          />
        </label>
        <label class="lens-report-form-field" for={`${id}-span`}>
          <span>{translate('report.editor.widthLabel', 'Width')}</span>
          <select id={`${id}-span`} onChange={(event) => setSpan(Number(event.currentTarget.value))} value={String(span())}>
            <For each={spanSteps}>
              {(value) => <option value={String(value)}>{translate('report.editor.widthValue', '{span} of 12 columns', { span: value })}</option>}
            </For>
          </select>
        </label>
        <fieldset class="lens-report-form-field">
          <legend>{translate('report.editor.periodLabel', 'Period')}</legend>
          <div class="lens-report-form-choice">
            <input checked={!local()} id={`${id}-inherit`} name={`${id}-period`} onChange={() => setLocal(false)} type="radio" />
            <label for={`${id}-inherit`}>{translate('report.editor.periodInherit', 'Use the dashboard period')}</label>
          </div>
          <div class="lens-report-form-choice">
            <input checked={local()} id={`${id}-local`} name={`${id}-period`} onChange={() => setLocal(true)} type="radio" />
            <label for={`${id}-local`}>{translate('report.editor.periodLocal', 'Use a period for this card')}</label>
          </div>
          <Show when={local()}>
            <div class="lens-report-form-range">
              <label for={`${id}-start`}>
                <span>{translate('report.editor.periodStart', 'From')}</span>
                <input id={`${id}-start`} onInput={(event) => setStart(event.currentTarget.value)} type="date" value={start()} />
              </label>
              <label for={`${id}-end`}>
                <span>{translate('report.editor.periodEnd', 'To')}</span>
                <input id={`${id}-end`} onInput={(event) => setEnd(event.currentTarget.value)} type="date" value={end()} />
              </label>
            </div>
            <Show when={invalid()}>
              <p class="lens-report-picker-error" role="alert">{translate('report.editor.periodInvalid', 'Enter a start date on or before the end date.')}</p>
            </Show>
          </Show>
        </fieldset>
      </div>
    </ReportDialog>
  )
}

function ImpactDialog(props: {
  title: string
  usage?: number
  restoreFocus?: HTMLElement
  theme?: LensThemeMode
  onEdit: () => Promise<void> | void
  onCopy?: () => Promise<void> | void
  onClose: () => void
}): JSX.Element {
  const translate = useTranslate()
  const [failed, setFailed] = createSignal(false)
  const [busy, setBusy] = createSignal(false)
  let cancelRef: HTMLButtonElement | undefined
  const run = async (action: () => Promise<void> | void) => {
    if (busy()) return
    setBusy(true)
    setFailed(false)
    try {
      await action()
      props.onClose()
    } catch (cause: unknown) {
      console.error('[lens] report action failed', cause)
      setFailed(true)
    } finally {
      setBusy(false)
    }
  }
  const footer = (
    <div class="lens-report-picker-actions">
      <button class="lens-context-action" onClick={() => props.onClose()} ref={(element) => { cancelRef = element }} type="button">{translate('report.picker.cancel', 'Cancel')}</button>
      <Show when={props.onCopy}>
        <button class="lens-context-action" disabled={busy()} onClick={() => void run(props.onCopy!)} type="button">
          {translate('report.action.copy', 'Make an independent copy')}
        </button>
      </Show>
      <button class="lens-context-action lens-report-primary" disabled={busy()} onClick={() => void run(props.onEdit)} type="button">
        {translate('report.editor.editShared', 'Edit the shared report')}
      </button>
    </div>
  )
  return (
    <ReportDialog
      footer={footer}
      initialFocus={() => cancelRef}
      onClose={() => props.onClose()}
      restoreFocus={props.restoreFocus}
      role="alertdialog"
      testId="report-impact"
      theme={props.theme}
      title={translate('report.editor.editReportTitle', 'Edit report: {title}', { title: props.title })}
    >
      <div class="lens-report-form">
        <p data-testid="report-impact-usage">
          {props.usage !== undefined && props.usage > 1
            ? translate('report.editor.impactCount', 'This report is used in {count} places. Changes apply to all of them.', { count: props.usage })
            : translate('report.editor.impact', 'Changes apply everywhere this report is used.')}
        </p>
        <Show when={props.onCopy}>
          <p>{translate('report.editor.impactCopy', 'To change only this card, make an independent copy instead.')}</p>
        </Show>
        <Show when={failed()}>
          <p class="lens-report-picker-error" role="alert">{translate('report.action.failed', 'The action could not be completed.')}</p>
        </Show>
      </div>
    </ReportDialog>
  )
}

function LeaveDialog(props: { guard: ReportLeaveGuard; lifecycle: ReportEditorLifecycle; theme?: LensThemeMode }): JSX.Element {
  const translate = useTranslate()
  let stayRef: HTMLButtonElement | undefined
  const footer = (
    <div class="lens-report-picker-actions">
      <button class="lens-context-action" disabled={props.guard.saving()} onClick={() => void props.guard.choose('stay')} ref={(element) => { stayRef = element }} type="button">
        {translate('report.leave.stay', 'Stay')}
      </button>
      <button class="lens-context-action" data-danger disabled={props.guard.saving()} onClick={() => void props.guard.choose('discard')} type="button">
        {translate('report.leave.discard', 'Leave without saving')}
      </button>
      <button aria-busy={props.guard.saving() || undefined} class="lens-context-action lens-report-primary" disabled={props.guard.saving()} onClick={() => void props.guard.choose('save')} type="button">
        {props.lifecycle === 'new' ? translate('report.editor.saveDraft', 'Save draft') : translate('report.editor.saveChanges', 'Save changes')}
      </button>
    </div>
  )
  return (
    <ReportDialog
      description={translate('report.leave.description', 'You have unsaved changes on this dashboard.')}
      footer={footer}
      initialFocus={() => stayRef}
      onClose={() => void props.guard.choose('stay')}
      role="alertdialog"
      testId="report-leave"
      theme={props.theme}
      title={translate('report.leave.title', 'Save changes before leaving?')}
    >
      <Show when={props.guard.failed()}>
        <p class="lens-report-picker-error lens-report-form" role="alert">
          {props.guard.failure() === 'discard'
            ? translate('report.leave.discardFailed', 'The changes could not be discarded, so you are still here. Try again or stay.')
            : translate('report.leave.saveFailed', 'The changes could not be saved, so you are still here. Try again or leave without saving.')}
        </p>
      </Show>
    </ReportDialog>
  )
}
