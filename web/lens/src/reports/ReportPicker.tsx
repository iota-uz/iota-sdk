import { createEffect, createSignal, For, Match, on, onCleanup, Show, Switch, untrack, type JSX } from 'solid-js'
import type { DashboardDocument } from '../contract/types'
import { LensDashboard } from '../LensDashboard'
import { CaretDown, CaretRight, Sparkle } from '../icons'
import { LensMessagesProvider, useTranslate, type LensThemeMode } from '../runtime'
import type { CatalogEntry, CatalogScope } from './contract/types'
import { createReportPickerSelection, type ReportCardGenerateStage, type ReportCatalogHost, type ReportPickerConfirmation } from './picker'
import { ReportDialog } from './ReportDialog'

export interface ReportPickerProps {
  /** The composition receiving the reports; its title names the dialog. */
  target: { id: string; title: string }
  host: ReportCatalogHost
  /**
   * Adds the confirmed reports to the caller's draft. The picker closes only
   * after it resolves; a rejection keeps the search and the selection.
   */
  onConfirm: (confirmation: ReportPickerConfirmation) => Promise<void> | void
  /** Called on Cancel, Escape, the close button and after a confirmed add. */
  onClose: () => void
  /**
   * The placements of a report in the caller's current draft. Overrides the
   * catalog's count, which describes the saved composition.
   */
  placed?: (reportId: string) => number
  restoreFocus?: HTMLElement
  theme?: LensThemeMode
  /** Runtime messages (document.RuntimeI18nKeys) when rendered outside a document. */
  messages?: Record<string, string>
  pageSize?: number
  /** Search text and owner scope to open with, e.g. the list the reader left. */
  initialSearch?: string
  initialScope?: CatalogScope
}

const scopes: CatalogScope[] = ['all', 'personal', 'shared']
const searchDelay = 200

type ListState = 'loading' | 'ready' | 'error'

/**
 * Searches the host catalog, previews a report and adds the chosen ones to a
 * draft. It is a drawer that slides in from the inline end, so the dashboard
 * being edited stays in view beside it. When the host can generate, the reader
 * may also describe a new report in words.
 */
export function ReportPicker(props: ReportPickerProps): JSX.Element {
  return (
    <LensMessagesProvider messages={props.messages}>
      <PickerDialog {...props} />
    </LensMessagesProvider>
  )
}

function PickerDialog(props: ReportPickerProps): JSX.Element {
  const translate = useTranslate()
  const placedCount = (entry: CatalogEntry) => props.placed?.(entry.id) ?? entry.placed ?? 0
  const selection = createReportPickerSelection(placedCount)
  const [search, setSearch] = createSignal(props.initialSearch ?? '')
  const [query, setQuery] = createSignal(search().trim())
  const [scope, setScope] = createSignal<CatalogScope>(props.initialScope ?? 'all')
  const [entries, setEntries] = createSignal<CatalogEntry[]>([])
  const [next, setNext] = createSignal<string>()
  const [state, setState] = createSignal<ListState>('loading')
  const [loadingMore, setLoadingMore] = createSignal(false)
  const [moreFailed, setMoreFailed] = createSignal(false)
  const [reload, setReload] = createSignal(0)
  const [previewing, setPreviewing] = createSignal<CatalogEntry>()
  // Reports made here live apart from fetched pages: a search that finishes (or fails) after the
  // reader generated one must neither remove the selected row nor reorder it.
  const [generated, setGenerated] = createSignal<CatalogEntry[]>([])
  const created = () => new Set(generated().map((entry) => entry.id))
  const [confirming, setConfirming] = createSignal(false)
  const [confirmFailed, setConfirmFailed] = createSignal(false)
  let searchRef: HTMLInputElement | undefined
  let request: AbortController | undefined
  const previewButtons = new Map<string, HTMLButtonElement>()
  const tabRefs = new Map<CatalogScope, HTMLButtonElement>()

  createEffect(() => {
    const value = search()
    const timer = setTimeout(() => setQuery(value.trim()), searchDelay)
    onCleanup(() => clearTimeout(timer))
  })

  const fetchPage = (cursor?: string) => {
    request?.abort()
    const controller = new AbortController()
    request = controller
    return props.host.search({
      search: untrack(query) || undefined,
      scope: untrack(scope),
      target: props.target.id,
      limit: props.pageSize,
      cursor,
    }, { signal: controller.signal }).then((page) => (controller.signal.aborted ? undefined : page))
  }

  createEffect(on([query, scope, reload], () => {
    setState('loading')
    setMoreFailed(false)
    fetchPage().then((page) => {
      if (!page) return
      setEntries(page.entries)
      setNext(page.next || undefined)
      setState('ready')
    }, (cause: unknown) => {
      if (cause instanceof DOMException && cause.name === 'AbortError') return
      console.error('[lens] report catalog search failed', cause)
      setState('error')
    })
  }))
  onCleanup(() => request?.abort())

  const loadMore = () => {
    const cursor = next()
    if (!cursor || loadingMore()) return
    setLoadingMore(true)
    setMoreFailed(false)
    fetchPage(cursor).then((page) => {
      if (!page) return
      const known = new Set(entries().map((entry) => entry.id))
      setEntries([...entries(), ...page.entries.filter((entry) => !known.has(entry.id))])
      setNext(page.next || undefined)
    }, (cause: unknown) => {
      if (cause instanceof DOMException && cause.name === 'AbortError') return
      console.error('[lens] report catalog page failed', cause)
      setMoreFailed(true)
    }).finally(() => setLoadingMore(false))
  }

  const confirm = async () => {
    if (confirming() || selection.count() === 0) return
    const confirmation = selection.confirmation(props.target.id)
    setConfirming(true)
    setConfirmFailed(false)
    try {
      await props.onConfirm(confirmation)
      selection.settle()
      props.onClose()
    } catch (cause: unknown) {
      console.error('[lens] adding picked reports failed', cause)
      setConfirmFailed(true)
    } finally {
      setConfirming(false)
    }
  }

  const adopt = (entry: CatalogEntry) => {
    setGenerated([entry, ...generated().filter((known) => known.id !== entry.id)])
    if (!selection.selected(entry.id)) selection.toggle(entry)
  }
  // A generated report is the reader's own, so it stays while the search is empty or names it, under All or Mine.
  const pinned = () => generated().filter((entry) => scope() !== 'shared' && entry.title.toLowerCase().includes(query().toLowerCase()))
  const fetched = () => entries().filter((entry) => !created().has(entry.id))

  const closePreview = () => {
    const entry = previewing()
    setPreviewing(undefined)
    if (entry) requestAnimationFrame(() => previewButtons.get(entry.id)?.focus())
  }

  const onTabKeyDown = (event: KeyboardEvent, current: CatalogScope) => {
    const offset = event.key === 'ArrowRight' ? 1 : event.key === 'ArrowLeft' ? -1 : 0
    if (!offset && event.key !== 'Home' && event.key !== 'End') return
    event.preventDefault()
    const position = event.key === 'Home' ? 0 : event.key === 'End' ? scopes.length - 1 : (scopes.indexOf(current) + offset + scopes.length) % scopes.length
    const target = scopes[position]!
    setScope(target)
    tabRefs.get(target)?.focus()
  }

  const scopeLabel = (value: CatalogScope) => {
    switch (value) {
      case 'all': return translate('report.picker.scope.all', 'All')
      case 'personal': return translate('report.picker.scope.personal', 'Mine')
      case 'shared': return translate('report.picker.scope.shared', 'Shared with me')
    }
  }

  const ownership = (entry: CatalogEntry) => {
    switch (entry.ownership) {
      case 'system': return translate('report.picker.ownership.system', 'Built-in')
      case 'personal': return translate('report.picker.ownership.personal', 'Yours')
      case 'shared': return entry.owner
        ? translate('report.picker.ownership.sharedBy', 'Shared by {owner}', { owner: entry.owner.name })
        : translate('report.picker.ownership.shared', 'Shared')
    }
  }

  const selectedRestrictions = () => selection.picks().filter((pick) => pick.entry.restriction)
  const emptyCatalog = () => query() === '' && scope() === 'all'

  const renderRow = (entry: CatalogEntry) => (
    <PickerRow
      created={created().has(entry.id)}
      entry={entry}
      onAddAnother={() => selection.addAnother(entry)}
      onPreview={props.host.preview ? () => setPreviewing(entry) : undefined}
      onToggle={() => selection.toggle(entry)}
      ownership={ownership(entry)}
      placed={placedCount(entry)}
      previewRef={(element) => previewButtons.set(entry.id, element)}
      selected={selection.selected(entry.id)}
    />
  )

  const footer = (
    <div class="lens-report-picker-footer">
      <div class="lens-report-picker-summary">
        <p aria-live="polite" class="lens-report-picker-count">
          {selection.count() > 0
            ? translate('report.picker.count', '{count} selected', { count: selection.count() })
            : translate('report.picker.none', 'No reports selected')}
        </p>
        <For each={selectedRestrictions()}>
          {(pick) => (
            <p class="lens-report-picker-restriction">
              {translate('report.picker.restriction', '{title}: {restriction}', { title: pick.entry.title, restriction: pick.entry.restriction ?? '' })}
            </p>
          )}
        </For>
        <Show when={confirmFailed()}>
          <p class="lens-report-picker-error" role="alert">
            {translate('report.picker.addFailed', 'The reports could not be added. Your selection is kept; try again.')}
          </p>
        </Show>
      </div>
      <div class="lens-report-picker-actions">
        <button class="lens-context-action" disabled={confirming()} onClick={() => props.onClose()} type="button">
          {translate('report.picker.cancel', 'Cancel')}
        </button>
        <button
          aria-busy={confirming() || undefined}
          class="lens-context-action lens-report-primary"
          data-testid="report-picker-confirm"
          disabled={selection.count() === 0 || confirming()}
          onClick={() => void confirm()}
          type="button"
        >
          {confirming()
            ? translate('report.picker.adding', 'Adding…')
            : selection.count() > 0
              ? translate('report.picker.add', 'Add {count}', { count: selection.count() })
              : translate('report.picker.addNone', 'Add reports')}
        </button>
      </div>
    </div>
  )

  return (
    <ReportDialog
      description={translate('report.picker.description', 'Chosen reports are added to the draft. Nothing is saved until you save the dashboard.')}
      footer={footer}
      initialFocus={() => searchRef}
      onClose={() => { if (!confirming()) props.onClose() }}
      onEscape={() => {
        if (previewing()) closePreview()
        else if (!confirming()) props.onClose()
      }}
      restoreFocus={props.restoreFocus}
      testId="report-picker"
      theme={props.theme}
      variant="drawer"
      willClose={() => !confirming() && !previewing()}
      title={translate('report.picker.title', 'Add reports to {target}', { target: props.target.title })}
    >
      <div class="lens-report-picker" hidden={Boolean(previewing())}>
        <Show when={typeof props.host.generate === 'function'}>
          <PickerGenerate host={props.host} onCreated={adopt} target={props.target.id} />
        </Show>
        <div class="lens-report-picker-filters">
          <input
            aria-label={translate('report.picker.search', 'Search reports')}
            class="lens-report-picker-search"
            onInput={(event) => setSearch(event.currentTarget.value)}
            placeholder={translate('report.picker.search', 'Search reports')}
            ref={(element) => { searchRef = element }}
            type="search"
            value={search()}
          />
          <div aria-label={translate('report.picker.scopeLabel', 'Report owner')} class="lens-report-picker-scopes" role="tablist">
            <For each={scopes}>
              {(value) => (
                <button
                  aria-controls="lens-report-picker-results"
                  aria-selected={scope() === value}
                  class="lens-report-picker-scope"
                  onClick={() => setScope(value)}
                  onKeyDown={(event) => onTabKeyDown(event, value)}
                  ref={(element) => tabRefs.set(value, element)}
                  role="tab"
                  tabIndex={scope() === value ? 0 : -1}
                  type="button"
                >
                  {scopeLabel(value)}
                </button>
              )}
            </For>
          </div>
        </div>
        <div aria-busy={state() === 'loading' || undefined} class="lens-report-picker-results" id="lens-report-picker-results" role="tabpanel">
          <Show when={pinned().length > 0}>
            <ul aria-label={translate('report.picker.list', 'Reports')} class="lens-report-picker-list" data-generated>
              <For each={pinned()}>{renderRow}</For>
            </ul>
          </Show>
          <Switch>
            <Match when={state() === 'loading'}>
              <p class="lens-report-picker-state" role="status">{translate('report.picker.loading', 'Loading reports…')}</p>
            </Match>
            <Match when={state() === 'error'}>
              <div class="lens-report-picker-state" data-state="error" role="alert">
                <p>{translate('report.picker.loadFailed', 'The reports could not be loaded.')}</p>
                <button class="lens-context-action" onClick={() => setReload((value) => value + 1)} type="button">
                  {translate('report.picker.retry', 'Try again')}
                </button>
              </div>
            </Match>
            <Match when={fetched().length === 0 && pinned().length > 0}>{null}</Match>
            <Match when={fetched().length === 0}>
              <p class="lens-report-picker-state" data-state={emptyCatalog() ? 'empty' : 'no-matches'} role="status">
                {emptyCatalog()
                  ? translate('report.picker.empty', 'There are no reports to add yet.')
                  : translate('report.picker.noMatches', 'No reports match this search.')}
              </p>
            </Match>
            <Match when={true}>
              <ul aria-label={translate('report.picker.list', 'Reports')} class="lens-report-picker-list">
                <For each={fetched()}>{renderRow}</For>
              </ul>
              <Show when={next()}>
                <div class="lens-report-picker-more">
                  <button aria-busy={loadingMore() || undefined} class="lens-context-action" disabled={loadingMore()} onClick={loadMore} type="button">
                    {translate('report.picker.loadMore', 'Show more reports')}
                  </button>
                  <Show when={moreFailed()}>
                    <p class="lens-report-picker-error" role="alert">{translate('report.picker.loadFailed', 'The reports could not be loaded.')}</p>
                  </Show>
                </div>
              </Show>
            </Match>
          </Switch>
        </div>
      </div>
      <Show when={previewing()}>
        {(entry) => (
          <PickerPreview
            entry={entry()}
            host={props.host}
            onBack={closePreview}
            onToggle={() => (placedCount(entry()) > 0 ? selection.addAnother(entry()) : selection.toggle(entry()))}
            placed={placedCount(entry())}
            selected={selection.selected(entry().id)}
            theme={props.theme}
          />
        )}
      </Show>
    </ReportDialog>
  )
}

function PickerRow(props: {
  created: boolean
  entry: CatalogEntry
  ownership: string
  placed: number
  selected: boolean
  onToggle: () => void
  onAddAnother: () => void
  onPreview?: () => void
  previewRef: (element: HTMLButtonElement) => void
}): JSX.Element {
  const translate = useTranslate()
  const id = () => `lens-report-pick-${props.entry.id}`
  const placed = () => props.placed > 0
  const meta = () => [props.entry.source, props.ownership].filter(Boolean).join(' · ')
  // The whole row is a target for the pointer; the checkbox stays its one
  // keyboard and assistive-technology control. A click that already reached a
  // control (the label, the checkbox, Preview, Add another) is theirs, and a
  // click that ends a text selection is not a choice.
  const onRowClick = (event: MouseEvent) => {
    if (placed()) return
    if ((event.target as Element).closest('input, label, button, a, select, textarea')) return
    if (globalThis.getSelection?.()?.toString()) return
    props.onToggle()
  }
  return (
    // eslint-disable-next-line jsx-a11y/click-events-have-key-events, jsx-a11y/no-noninteractive-element-interactions -- pointer-only widening of the row's hit area; the checkbox inside is the keyboard and assistive-technology control.
    <li
      class="lens-report-picker-item"
      data-created={props.created || undefined}
      data-placed={placed() || undefined}
      data-report-id={props.entry.id}
      data-selected={props.selected || undefined}
      onClick={onRowClick}
    >
      <input
        aria-describedby={`${id()}-meta`}
        checked={placed() || props.selected}
        class="lens-report-picker-check"
        disabled={placed()}
        id={id()}
        onChange={() => props.onToggle()}
        type="checkbox"
      />
      <div class="lens-report-picker-text">
        <label class="lens-report-picker-name" for={id()}>{props.entry.title}</label>
        <Show when={props.entry.description}>
          <p class="lens-report-picker-summary-text">{props.entry.description}</p>
        </Show>
        <p class="lens-report-picker-meta" id={`${id()}-meta`}>
          <span>{meta()}</span>
          <Show when={(props.entry.members ?? 0) > 1}>
            <span class="lens-report-picker-badge">{translate('report.picker.compound', 'One block · {count} panels', { count: props.entry.members ?? 0 })}</span>
          </Show>
          <Show when={props.created}>
            <span class="lens-report-picker-badge" data-tone="new">{translate('report.picker.generate.new', 'New')}</span>
          </Show>
          <Show when={placed()}>
            <span class="lens-report-picker-badge" data-tone="placed">{translate('report.picker.placed', 'Already on this dashboard')}</span>
          </Show>
        </p>
        <Show when={props.entry.restriction}>
          <p class="lens-report-picker-restriction">{props.entry.restriction}</p>
        </Show>
      </div>
      <div class="lens-report-picker-item-actions">
        <Show when={placed()}>
          <button aria-pressed={props.selected} class="lens-context-action" onClick={() => props.onAddAnother()} type="button">
            {translate('report.picker.addAnother', 'Add another card')}
          </button>
        </Show>
        <Show when={props.onPreview}>
          <button
            aria-label={translate('report.picker.previewOf', 'Preview {title}', { title: props.entry.title })}
            class="lens-context-action"
            onClick={() => props.onPreview?.()}
            ref={props.previewRef}
            type="button"
          >
            {translate('report.picker.preview', 'Preview')}
          </button>
        </Show>
      </div>
    </li>
  )
}

function PickerPreview(props: {
  entry: CatalogEntry
  host: ReportCatalogHost
  placed: number
  selected: boolean
  theme?: LensThemeMode
  onBack: () => void
  onToggle: () => void
}): JSX.Element {
  const translate = useTranslate()
  const [document, setDocument] = createSignal<DashboardDocument>()
  const [failed, setFailed] = createSignal(false)
  const [attempt, setAttempt] = createSignal(0)
  let backRef: HTMLButtonElement | undefined

  createEffect(on(attempt, () => {
    const controller = new AbortController()
    setFailed(false)
    setDocument(undefined)
    props.host.preview!(props.entry, { signal: controller.signal }).then((value) => {
      if (!controller.signal.aborted) setDocument(value)
    }, (cause: unknown) => {
      if (controller.signal.aborted) return
      console.error('[lens] report preview failed', cause)
      setFailed(true)
    })
    onCleanup(() => controller.abort())
  }))
  requestAnimationFrame(() => backRef?.focus())

  return (
    <section aria-label={translate('report.picker.previewOf', 'Preview {title}', { title: props.entry.title })} class="lens-report-picker-preview">
      <div class="lens-report-picker-preview-bar">
        <button class="lens-context-action" onClick={() => props.onBack()} ref={(element) => { backRef = element }} type="button">
          {translate('report.picker.back', 'Back to reports')}
        </button>
        <h3 class="lens-report-picker-preview-title">{props.entry.title}</h3>
        <button aria-pressed={props.selected} class="lens-context-action" onClick={() => props.onToggle()} type="button">
          {props.placed > 0
            ? translate('report.picker.addAnother', 'Add another card')
            : translate('report.picker.selectReport', 'Select report')}
        </button>
      </div>
      <Show when={props.entry.restriction}>
        <p class="lens-report-picker-restriction">{props.entry.restriction}</p>
      </Show>
      <Switch>
        <Match when={failed()}>
          <div class="lens-report-picker-state" data-state="error" role="alert">
            <p>{translate('report.picker.previewFailed', 'The preview could not be loaded.')}</p>
            <button class="lens-context-action" onClick={() => setAttempt((value) => value + 1)} type="button">
              {translate('report.picker.retry', 'Try again')}
            </button>
          </div>
        </Match>
        <Match when={document()}>
          {(value) => (
            <div class="lens-report-picker-preview-body">
              <LensDashboard initialDocument={value()} theme={props.theme ?? 'light'} />
            </div>
          )}
        </Match>
        <Match when={true}>
          <p class="lens-report-picker-state" role="status">{translate('report.picker.previewLoading', 'Loading preview…')}</p>
        </Match>
      </Switch>
    </section>
  )
}

type GenerateState = { kind: 'idle' } | { kind: 'running'; stage: ReportCardGenerateStage } | { kind: 'failed'; message: string } | { kind: 'done'; title: string }

/** The "Create with AI" section: one description becomes one new report, selected here for the reader to add. */
function PickerGenerate(props: {
  host: ReportCatalogHost
  target: string
  onCreated: (entry: CatalogEntry) => void
}): JSX.Element {
  const translate = useTranslate()
  const [open, setOpen] = createSignal(false)
  const [prompt, setPrompt] = createSignal('')
  const [state, setState] = createSignal<GenerateState>({ kind: 'idle' })
  let controller: AbortController | undefined
  let inputRef: HTMLTextAreaElement | undefined
  onCleanup(() => controller?.abort())

  const running = () => state().kind === 'running'
  const stageLabel = (stage: ReportCardGenerateStage) => {
    switch (stage) {
      case 'starting': return translate('report.picker.generate.stage.starting', 'Starting…')
      case 'schema': return translate('report.picker.generate.stage.schema', 'Reading the available data…')
      case 'generating': return translate('report.picker.generate.stage.generating', 'Building the report…')
      case 'validating': return translate('report.picker.generate.stage.validating', 'Checking the result…')
    }
  }

  const submit = () => {
    const text = prompt().trim()
    if (!text || running()) return
    controller?.abort()
    const current = new AbortController()
    controller = current
    setState({ kind: 'running', stage: 'starting' })
    props.host.generate!({ prompt: text, target: props.target }, {
      signal: current.signal,
      onStage: (stage) => {
        if (!current.signal.aborted && untrack(running)) setState({ kind: 'running', stage })
      },
    }).then((entry) => {
      if (current.signal.aborted) return
      props.onCreated(entry)
      setPrompt('')
      setState({ kind: 'done', title: entry.title })
    }, (cause: unknown) => {
      if (current.signal.aborted || (cause instanceof DOMException && cause.name === 'AbortError')) return
      console.error('[lens] report generation failed', cause)
      const message = cause instanceof Error ? cause.message.trim() : ''
      setState({ kind: 'failed', message: message || translate('report.picker.generate.failed', 'The report could not be created. Rephrase the description or try again.') })
    })
  }

  const stop = () => {
    controller?.abort()
    setState({ kind: 'idle' })
    requestAnimationFrame(() => inputRef?.focus())
  }

  return (
    <section class="lens-report-picker-generate" data-open={open() || undefined} data-testid="report-picker-generate">
      <button
        aria-controls="lens-report-picker-generate-body"
        aria-expanded={open()}
        class="lens-report-picker-generate-toggle"
        onClick={() => {
          setOpen(!open())
          if (open()) requestAnimationFrame(() => inputRef?.focus())
        }}
        type="button"
      >
        <Sparkle className="lens-report-picker-generate-icon" />
        <span class="lens-report-picker-generate-title">{translate('report.picker.generate.title', 'Create with AI')}</span>
        {open() ? <CaretDown /> : <CaretRight />}
      </button>
      <Show when={open()}>
        <div class="lens-report-picker-generate-body" id="lens-report-picker-generate-body">
          <p class="lens-report-picker-generate-hint">
            {translate('report.picker.generate.hint', 'Describe the report you need. It is saved as your own report and selected here; nothing reaches the dashboard until you add it.')}
          </p>
          <textarea
            aria-label={translate('report.picker.generate.label', 'Describe the report')}
            class="lens-report-picker-generate-input"
            disabled={running()}
            maxLength={2000}
            onInput={(event) => setPrompt(event.currentTarget.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) {
                event.preventDefault()
                submit()
              }
            }}
            placeholder={translate('report.picker.generate.placeholder', 'For example: policies sold per region over the last quarter, as a bar chart')}
            ref={(element) => { inputRef = element; requestAnimationFrame(() => element.focus()) }}
            rows={3}
            value={prompt()}
          />
          <div class="lens-report-picker-generate-actions">
            <Show
              when={running()}
              fallback={
                <button class="lens-context-action lens-report-primary" data-testid="report-picker-generate-submit" disabled={prompt().trim() === ''} onClick={submit} type="button">
                  {state().kind === 'failed' ? translate('report.picker.generate.retry', 'Try again') : translate('report.picker.generate.submit', 'Generate')}
                </button>
              }
            >
              <button class="lens-context-action" data-testid="report-picker-generate-stop" onClick={stop} type="button">
                {translate('report.picker.generate.stop', 'Stop')}
              </button>
            </Show>
            <Show when={state()}>
              {(current) => (
                <Switch>
                  <Match when={current().kind === 'running'}>
                    <p aria-live="polite" class="lens-report-picker-generate-status" data-state="running" role="status">
                      <span aria-hidden="true" class="lens-report-picker-generate-spinner" />
                      {stageLabel((current() as { stage: ReportCardGenerateStage }).stage)}
                    </p>
                  </Match>
                  <Match when={current().kind === 'failed'}>
                    <p class="lens-report-picker-error" data-state="failed" role="alert">{(current() as { message: string }).message}</p>
                  </Match>
                  <Match when={current().kind === 'done'}>
                    <p class="lens-report-picker-generate-status" data-state="done" role="status">
                      {translate('report.picker.generate.created', 'Created “{title}” and selected it. Press Add to put it on the dashboard.', { title: (current() as { title: string }).title })}
                    </p>
                  </Match>
                </Switch>
              )}
            </Show>
          </div>
        </div>
      </Show>
    </section>
  )
}
