import { createSignal, createUniqueId, onMount, Show, untrack, type JSX } from 'solid-js'
import { LensMessagesProvider, useTranslate, type LensThemeMode } from '../runtime'
import type { ContextFilters, Definition, Parameters } from './contract/types'
import type { ReportDraft } from './draft'
import { periodText, ReportLeaveDialog, ReportUsageList, type ReportUsage } from './editing'
import { createReportLeaveGuard, type ReportLeaveGuard } from './guard'

/** Where the reader opened the shared report from, and the filters in effect there. */
export interface ReportEditContext {
  origin: {
    /** dashboard, section or card. */
    scope: string
    id: string
    title: string
  }
  filters: ContextFilters
}

export interface ReportDefinitionEditorProps {
  /** A draft of the shared report definition (createReportDraft). */
  draft: ReportDraft<Definition>
  /** Entered from a dashboard: its filters drive the preview and are shown with their origin. */
  context?: ReportEditContext
  /** The placements a saved change reaches, shown before the reader edits anything. */
  placements?: () => Promise<ReportUsage[]> | ReportUsage[]
  /** The host's rendering of the draft definition with the preview filters. Nothing it shows is saved. */
  renderPreview?: (definition: Definition, filters: ContextFilters | undefined) => JSX.Element
  /** The host's controls for the report's own parameters; a change goes to the draft and the preview. */
  renderParameters?: (props: { value: Parameters | undefined; onChange: (next: Parameters) => void }) => JSX.Element
  /** Saves the draft as an independent report instead of changing the shared one (impact and conflict). */
  onCopy?: (draft: Definition) => Promise<void> | void
  /**
   * Saves what the preview shows, the report with the dashboard's filters, as a new independent
   * report. It is offered while the preview uses those filters.
   */
  onSaveSlice?: (draft: Definition, filters: ContextFilters) => Promise<void> | void
  /** A readable name for a filter dimension. */
  dimensionLabel?: (dimension: string) => string
  /** In-app navigation guard; one with browser-unload protection is created when omitted. */
  guard?: ReportLeaveGuard
  theme?: LensThemeMode
  messages?: Record<string, string>
  /** Locale of the built-in chrome translations (ru, uz); English when omitted outside a document. */
  locale?: string
}

/**
 * Edits a shared report definition. It differs from configuring one card: a
 * save reaches every placement listed at the top, the dashboard's filters only
 * drive the preview, and parameter changes are previewed but saved only with
 * Save changes. Cancel restores the whole session.
 */
export function ReportDefinitionEditor(props: ReportDefinitionEditorProps): JSX.Element {
  return (
    <div class="lens-root lens-root-inline" data-theme={props.theme ?? 'light'}>
      <LensMessagesProvider locale={props.locale} messages={props.messages}>
        <DefinitionEditor {...props} />
      </LensMessagesProvider>
    </div>
  )
}

function filterText(filters: ContextFilters | undefined, translate: ReturnType<typeof useTranslate>, label: (dimension: string) => string): string[] {
  const parts: string[] = []
  const period = periodText(filters?.period, translate)
  if (period) parts.push(period)
  for (const constraint of filters?.constraints ?? []) parts.push(`${label(constraint.dimension)}: ${constraint.values.join(', ')}`)
  return parts
}

function DefinitionEditor(props: ReportDefinitionEditorProps): JSX.Element {
  const translate = useTranslate()
  const draft = untrack(() => props.draft)
  const guard = untrack(() => props.guard) ?? createReportLeaveGuard(draft)
  const id = createUniqueId()
  const [placements, setPlacements] = createSignal<ReportUsage[]>()
  const [useContext, setUseContext] = createSignal(true)
  const [saved, setSaved] = createSignal(false)
  const [notice, setNotice] = createSignal('')
  const [busy, setBusy] = createSignal<'copy' | 'slice'>()
  const [actionFailed, setActionFailed] = createSignal<'copy' | 'slice'>()
  const label = (dimension: string) => props.dimensionLabel?.(dimension) ?? dimension
  const defaults = () => filterText(draft.value().filters, translate, label)
  const previewFilters = () => (props.context && useContext() ? props.context.filters : draft.value().filters)
  const conflict = () => draft.conflict()
  const saveError = () => Boolean(draft.error()) && !conflict()

  onMount(() => {
    if (!props.placements) return
    Promise.resolve(props.placements()).then(setPlacements, (cause: unknown) => console.error('[lens] report placements failed', cause))
  })

  const edit = (change: (definition: Definition) => Definition) => {
    setSaved(false)
    setNotice('')
    draft.update(change)
  }

  // Rendered once: the host's controls read value as they need it, so an edit updates them in
  // place instead of replacing the input the reader is typing in.
  const parameterProps = {
    get value() { return draft.value().parameters },
    onChange: (next: Parameters) => edit((definition) => ({ ...definition, parameters: next })),
  }

  const save = async () => {
    setSaved(false)
    setNotice('')
    try {
      await draft.save('draft')
      setSaved(true)
    } catch (cause: unknown) {
      console.error('[lens] saving the report failed', cause)
    }
  }

  const cancel = () => {
    setSaved(false)
    draft.cancel()
    setNotice(translate('report.editor.cancelled', 'Changes discarded. The saved version is restored.'))
  }

  const run = async (kind: 'copy' | 'slice', action: () => Promise<void> | void) => {
    if (busy()) return
    setBusy(kind)
    setActionFailed(undefined)
    try {
      await action()
    } catch (cause: unknown) {
      console.error(`[lens] report ${kind} failed`, cause)
      setActionFailed(kind)
    } finally {
      setBusy(undefined)
    }
  }
  const copy = () => props.onCopy && void run('copy', () => props.onCopy!(draft.value()))

  const status = () => {
    if (draft.saving()) return translate('report.editor.saving', 'Saving…')
    if (draft.dirty()) return translate('report.editor.unsaved', 'Unsaved changes')
    if (saved()) return translate('report.editor.saved', 'All changes saved')
    return translate('report.editor.upToDate', 'No changes')
  }

  const usageText = () => {
    const count = placements()?.length
    return count !== undefined && count > 1
      ? translate('report.editor.impactCount', 'This report is used in {count} places. Changes apply to all of them.', { count })
      : translate('report.editor.impact', 'Changes apply everywhere this report is used.')
  }

  return (
    <div class="lens-report-editor lens-report-definition-editor" data-testid="report-definition-editor">
      <header class="lens-report-editor-header">
        <div class="lens-report-editor-identity">
          <h2 class="lens-report-definition-heading">{translate('report.editor.editReportTitle', 'Edit report: {title}', { title: draft.saved().title })}</h2>
          <label class="lens-report-editor-title-label" for={`${id}-title`}>{translate('report.definition.name', 'Report name')}</label>
          <input
            class="lens-report-editor-title"
            id={`${id}-title`}
            onChange={(event) => {
              const value = event.currentTarget.value.trim()
              if (value) edit((definition) => ({ ...definition, title: value }))
              else event.currentTarget.value = draft.value().title
            }}
            // A rename counts as unsaved while the reader types, so a leave request before blur still asks.
            onInput={(event) => {
              const value = event.currentTarget.value
              if (value.trim()) edit((definition) => ({ ...definition, title: value }))
            }}
            value={draft.value().title}
          />
          <p aria-live="polite" class="lens-report-editor-status" data-dirty={draft.dirty() || undefined} data-testid="report-definition-status">{status()}</p>
        </div>
        <div class="lens-report-editor-actions">
          <button class="lens-context-action" disabled={draft.saving() || !draft.dirty()} onClick={cancel} type="button">
            {translate('report.editor.cancel', 'Cancel')}
          </button>
          <button aria-busy={draft.saving() || undefined} class="lens-context-action lens-report-primary" data-testid="report-definition-save" disabled={draft.saving() || !draft.dirty()} onClick={() => void save()} type="button">
            {translate('report.editor.saveChanges', 'Save changes')}
          </button>
        </div>
      </header>

      <section aria-labelledby={`${id}-impact`} class="lens-report-editor-alert" data-testid="report-definition-impact" data-tone="warn">
        <p id={`${id}-impact`}>{usageText()}</p>
        <Show when={placements()?.length}>
          <ReportUsageList placements={placements()!} />
        </Show>
        <Show when={props.onCopy}>
          <p>{translate('report.editor.impactCopy', 'To change only this card, make an independent copy instead.')}</p>
          <div class="lens-report-editor-alert-actions">
            <button aria-busy={busy() === 'copy' || undefined} class="lens-context-action" data-testid="report-definition-copy" disabled={Boolean(busy())} onClick={copy} type="button">
              {translate('report.action.copy', 'Make an independent copy')}
            </button>
          </div>
        </Show>
      </section>

      <Show when={saveError()}>
        <p class="lens-report-editor-alert" data-tone="error" role="alert">
          {translate('report.editor.saveFailed', 'The changes could not be saved. They are kept here; try again.')}
        </p>
      </Show>
      <Show when={conflict()}>
        {(value) => (
          <div class="lens-report-editor-alert" data-testid="report-definition-conflict" data-tone="warn" role="alert">
            <p>{translate('report.definition.conflict', 'Someone else saved this report while you were editing. Your changes are kept.')}</p>
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
              <Show when={props.onCopy}>
                <button class="lens-context-action" disabled={Boolean(busy())} onClick={copy} type="button">
                  {translate('report.editor.conflictCopy', 'Save as a separate copy')}
                </button>
              </Show>
            </div>
          </div>
        )}
      </Show>
      <Show when={actionFailed()}>
        <p class="lens-report-editor-alert" data-tone="error" role="alert">{translate('report.action.failed', 'The action could not be completed.')}</p>
      </Show>
      <Show when={notice()}>
        <p class="lens-report-editor-notice" role="status">{notice()}</p>
      </Show>

      <section aria-labelledby={`${id}-filters`} class="lens-report-definition-section">
        <h3 class="lens-report-definition-section-title" id={`${id}-filters`}>{translate('report.definition.filters', 'Filters')}</h3>
        <p data-testid="report-definition-defaults">
          {defaults().length
            ? translate('report.definition.defaults', 'Report defaults: {filters}', { filters: defaults().join('; ') })
            : translate('report.definition.noDefaults', 'The report has no default filters.')}
        </p>
        <Show when={props.context}>
          {(context) => (
            <div class="lens-report-definition-origin" data-testid="report-definition-origin">
              <p>
                <strong>{translate('report.definition.origin', 'Opened from {title}', { title: context().origin.title })}</strong>
                {': '}
                {filterText(context().filters, translate, label).join('; ') || translate('report.definition.noFilters', 'no filters')}
              </p>
              <fieldset class="lens-report-form-field">
                <legend>{translate('report.definition.previewWith', 'Preview with')}</legend>
                <div class="lens-report-form-choice">
                  <input checked={useContext()} id={`${id}-context`} name={`${id}-preview`} onChange={() => setUseContext(true)} type="radio" />
                  <label for={`${id}-context`}>{translate('report.definition.previewContext', 'Filters from {title}', { title: context().origin.title })}</label>
                </div>
                <div class="lens-report-form-choice">
                  <input checked={!useContext()} id={`${id}-defaults`} name={`${id}-preview`} onChange={() => setUseContext(false)} type="radio" />
                  <label for={`${id}-defaults`}>{translate('report.definition.previewDefaults', 'Report defaults')}</label>
                </div>
              </fieldset>
              <p class="lens-report-definition-hint">{translate('report.definition.contextHint', 'These filters only drive the preview. Saving does not copy them into the shared report.')}</p>
              <Show when={props.onSaveSlice && useContext()}>
                <div class="lens-report-editor-alert-actions">
                  <button
                    aria-busy={busy() === 'slice' || undefined}
                    class="lens-context-action"
                    data-testid="report-definition-slice"
                    disabled={Boolean(busy())}
                    onClick={() => void run('slice', async () => {
                      await props.onSaveSlice!(draft.value(), structuredClone(context().filters))
                      setNotice(translate('report.definition.sliceSaved', 'Saved as a new report with these filters. The shared report is unchanged.'))
                    })}
                    type="button"
                  >
                    {translate('report.definition.saveSlice', 'Save this view as a new report')}
                  </button>
                </div>
              </Show>
            </div>
          )}
        </Show>
      </section>

      <Show when={props.renderParameters}>
        {(render) => (
          <section aria-labelledby={`${id}-parameters`} class="lens-report-definition-section">
            <h3 class="lens-report-definition-section-title" id={`${id}-parameters`}>{translate('report.definition.parameters', 'Parameters')}</h3>
            <p class="lens-report-definition-hint">{translate('report.definition.parametersHint', 'Changes show in the preview at once and are saved only with Save changes.')}</p>
            {untrack(() => render()(parameterProps))}
          </section>
        )}
      </Show>

      <Show when={props.renderPreview}>
        {(render) => (
          <section aria-labelledby={`${id}-preview`} class="lens-report-definition-section" data-testid="report-definition-preview">
            <div class="lens-report-definition-preview-bar">
              <h3 class="lens-report-definition-section-title" id={`${id}-preview`}>{translate('report.definition.preview', 'Preview')}</h3>
              <Show when={draft.dirty()}>
                <span class="lens-report-picker-badge" data-tone="change">{translate('report.definition.previewUnsaved', 'Not saved')}</span>
              </Show>
            </div>
            {render()(draft.value(), previewFilters())}
          </section>
        )}
      </Show>

      <Show when={guard.pending()}>
        <ReportLeaveDialog guard={guard} lifecycle="existing" theme={props.theme} />
      </Show>
    </div>
  )
}
