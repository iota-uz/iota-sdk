import { Match, Show, Switch, createMemo, createSignal, onCleanup, onMount, type JSX } from 'solid-js'
import {
  Editor, EditorSkeleton,
  type AutomationGraph, type AutomationLayout, type EditorApi, type EditorCatalog, type FieldSlot, type RunsView, type SaveResult, type VersionEntry,
} from '@iota-uz/sdk/automation-editor'
import { Button, CopyButton, Dialog, Select } from '@iota-uz/sdk/solid-ui'
import type { RPC } from './Automations.solid.generated'
import { useApp } from './context'
import { DeleteDialog, DisableDialog, ReviewDialog } from './Dialogs'
import { Icon } from './Icon'
import {
  editorLocale, emptyGraph, errorCode, errorText, graphPath, readRunsView, runDataOf, runSummaryOf, writeRunsView,
  type ConnectionOptions, type GetResult, type ManualTrigger,
} from './model'
import { RunDialog } from './RunDialog'

type CatalogResult = RPC['automations.catalog']['result']

/** What the page holds of the graph besides the editor's own draft. */
interface Info {
  id: string
  name: string
  enabled: boolean
  enabledVersion: number | null
  latestVersion: number
  /** The version open in the editor and who saved it. */
  version: number
  savedByAgent: boolean
  versions: GetResult['versions']
  /** The manual triggers of the enabled version: what «Run» starts. */
  manual: ManualTrigger[]
}

/** The editor page: /automations/new and /automations/{id} (AL3–AL9). */
export default function EditorPage(): JSX.Element {
  const app = useApp()
  const t = app.t
  const locale = editorLocale(app.lang)
  const search = new URLSearchParams(window.location.search)
  const requestedVersion = Number(search.get('version') ?? '0') || 0

  const [loaded, setLoaded] = createSignal<{ graph: AutomationGraph; layout: AutomationLayout; version: number; catalog: EditorCatalog; connections: ConnectionOptions; readOnly: { reason: string } | null; key: number }>()
  const [loadError, setLoadError] = createSignal<string>()
  const [info, setInfo] = createSignal<Info>()
  const [can, setCan] = createSignal(app.props.can)
  const [tab, setTab] = createSignal<'edit' | 'runs'>(app.props.run || search.get('tab') === 'runs' ? 'runs' : 'edit')
  // The Runs tab's state lives in the URL (D1, RV2): /automations/{id}/runs/{run}?node=&state=&subject=.
  const initialRuns = readRunsView(window.location.pathname, window.location.search)
  const [runsView, setRunsView] = createSignal<RunsView>(app.props.run ? { ...initialRuns, run: app.props.run, node: search.get('node') } : initialRuns)
  const [selected, setSelected] = createSignal<string | null>(tab() === 'runs' ? null : search.get('node'))
  const [remoteLayout, setRemoteLayout] = createSignal<{ positions: AutomationLayout } | null>(null)
  const [dialog, setDialog] = createSignal<'review' | 'disable' | 'delete' | 'run' | null>(null)
  const [phone, setPhone] = createSignal(false)
  // layoutRevision is the stored layout's revision as last heard; positionsBase is the revision the
  // positions on screen were made over. They differ after another tab moved steps, until «Refresh».
  let layoutRevision = 0
  let positionsBase = 0
  let layoutStale = false
  let loadSeq = 0

  onMount(() => {
    const mq = window.matchMedia('(max-width: 767px)')
    const update = () => setPhone(mq.matches)
    update()
    mq.addEventListener('change', update)
    onCleanup(() => mq.removeEventListener('change', update))
  })

  const setURL = (mutate: (url: URL) => void, push = false) => {
    const url = new URL(window.location.href)
    mutate(url)
    if (url.href === window.location.href) return
    if (push) window.history.pushState({}, '', url)
    else window.history.replaceState({}, '', url)
    shownHref = window.location.href
  }
  // The URL the page shows; a Back that is cancelled returns to it.
  let shownHref = window.location.href

  const applyGet = (g: GetResult, key: number) => {
    layoutRevision = g.automation.layout_revision
    positionsBase = layoutRevision
    layoutStale = false
    clearConflict()
    setCan(g.can)
    setInfo({
      id: g.automation.id, name: g.automation.name, enabled: g.automation.enabled, enabledVersion: g.automation.enabled_version,
      latestVersion: g.automation.latest_version, version: g.version, savedByAgent: g.saved_by.origin === 'agent', versions: g.versions,
      manual: manualOf(g),
    })
    setLoaded({
      graph: g.graph as AutomationGraph, layout: g.automation.layout, version: g.version, catalog: g.catalog as unknown as EditorCatalog,
      connections: g.connections, readOnly: g.read_only ? { reason: g.read_only.code === 'old_version' ? t('Editor.OldVersion', { version: g.version }) : g.read_only.reason } : null, key,
    })
  }

  // The version the last request asked for (0: the latest): Back compares with it, not with what is
  // rendered, so an answer to a superseded request never wins (loadSeq drops it).
  let requested = requestedVersion
  const load = async (version = requestedVersion) => {
    requested = version
    const mine = ++loadSeq
    setLoadError(undefined)
    try {
      if (!app.props.id) {
        const c: CatalogResult = await app.call('automations.catalog', {})
        if (mine !== loadSeq) return
        setCan(c.can)
        setLoaded({ graph: emptyGraph(), layout: {}, version: 0, catalog: c.catalog as unknown as EditorCatalog, connections: c.connections, readOnly: null, key: mine })
        return
      }
      const g = await app.call('automations.get', { id: app.props.id, ...(version > 0 ? { version } : {}) })
      if (mine !== loadSeq) return
      // Edits made while the snapshot was on its way are not replaced without the person's consent.
      if (loaded() && dirty()) {
        setDiscard({ version, held: g })
        return
      }
      applyGet(g, mine)
    } catch (e) {
      if (mine === loadSeq) setLoadError(errorText(t, e, t('Actions.LoadGraph')))
    }
  }
  onMount(() => void load())
  // D1: Back and Forward restore what the URL says: the tab, the selected step and the open version.
  onMount(() => {
    const onPop = () => {
      const q = new URLSearchParams(window.location.search)
      const view = readRunsView(window.location.pathname, window.location.search)
      const runs = Boolean(view.run) || q.get('tab') === 'runs'
      setTab(runs ? 'runs' : 'edit')
      setRunsView(view)
      setSelected(runs ? null : q.get('node'))
      const wanted = Number(q.get('version') ?? '0') || 0
      const back = shownHref
      shownHref = window.location.href
      if (info() && wanted !== requested) switchVersion(wanted, back)
    }
    window.addEventListener('popstate', onPop)
    onCleanup(() => window.removeEventListener('popstate', onPop))
  })

  /** Re-reads the graph's state (enabled, versions) without touching the editor's draft. */
  const refreshInfo = async () => {
    const i = info()
    if (!i) return
    try {
      // The layout revision moves only with the positions it stands for (save, saveLayout, reload).
      const g = await app.call('automations.get', { id: i.id })
      setInfo({ ...i, name: g.automation.name, enabled: g.automation.enabled, enabledVersion: g.automation.enabled_version, latestVersion: g.automation.latest_version, versions: g.versions, manual: manualOf(g) })
    } catch (e) {
      console.warn('automations: refresh failed', e)
    }
  }

  const [dirty, setDirty] = createSignal(false)
  // A version switch that would drop unsaved edits: asked before it starts (revert: the URL Back left),
  // or once its snapshot arrived while edits were made (held).
  const [discard, setDiscard] = createSignal<{ version: number; revert?: string; held?: GetResult } | null>(null)
  /** Opens a version (0: the latest); unsaved edits are not dropped without the person's consent. */
  const switchVersion = (version: number, revert?: string) => {
    if (dirty()) {
      setDiscard({ version, ...(revert ? { revert } : {}) })
      return
    }
    void load(version)
  }
  const openVersion = (version: number) => {
    const v = version === info()?.latestVersion ? 0 : version
    const push = () => setURL((url) => { if (v === 0) url.searchParams.delete('version'); else url.searchParams.set('version', String(v)) }, true)
    if (dirty()) {
      setDiscard({ version: v })
      return
    }
    push()
    void load(v)
  }
  const confirmDiscard = () => {
    const d = discard()
    if (!d) return
    setDiscard(null)
    setDirty(false)
    if (d.held) {
      // The URL already names the version (the menu or Back set it before the request).
      applyGet(d.held, ++loadSeq)
      return
    }
    if (!d.revert) setURL((url) => { if (d.version === 0) url.searchParams.delete('version'); else url.searchParams.set('version', String(d.version)) }, true)
    void load(d.version)
  }
  const cancelDiscard = () => {
    const d = discard()
    setDiscard(null)
    if (d?.held) {
      // The snapshot is dropped: the URL and the next Back name the version still open.
      const i = info()
      const open = i && i.version !== i.latestVersion ? i.version : 0
      requested = open
      setURL((url) => { if (open === 0) url.searchParams.delete('version'); else url.searchParams.set('version', String(open)) }, true)
      return
    }
    // Back was pressed: the URL returns to the version still open.
    if (d?.revert) {
      window.history.pushState({}, '', d.revert)
      shownHref = window.location.href
    }
  }

  // ——— the editor's backend ———
  const toSave = (res: RPC['automations.save']['result']): SaveResult => {
    if (res.ok) return { ok: true, graph: res.graph as AutomationGraph, version: res.version ?? 0, ...(res.pending_review ? { pending_review: true } : {}) }
    if (res.conflict) return { conflict: res.conflict }
    return { errors: res.errors ?? [], ...(res.warnings ? { warnings: res.warnings } : {}) }
  }
  const save = async (graph: AutomationGraph, layout: AutomationLayout, base: number): Promise<SaveResult> => {
    const i = info()
    if (!i) {
      const res = await app.call('automations.create', { graph, layout })
      if (res.ok && res.id) {
        const id = res.id
        layoutRevision = res.layout_revision ?? 0
        positionsBase = layoutRevision
        setInfo({ id, name: graph.name, enabled: false, enabledVersion: null, latestVersion: res.version ?? 1, version: res.version ?? 1, savedByAgent: false, versions: [], manual: [] })
        window.history.replaceState({}, '', graphPath(app.props.basePath, id))
        void refreshInfo()
      }
      return toSave(res)
    }
    // The revision the positions were made over, also after a conflict: the server keeps the newer ones.
    const res = await app.call('automations.save', { id: i.id, base, graph, layout, layout_revision: positionsBase })
    if (res.ok) {
      layoutRevision = res.layout_revision ?? layoutRevision
      if (!res.layout_changed && !layoutStale) positionsBase = layoutRevision
      if (res.layout_changed) {
        // Another tab moved steps meanwhile: its positions were kept; «Refresh» shows them.
        layoutStale = true
        conflict = { positions: res.positions ?? {}, revision: layoutRevision }
        notifyLayoutConflict()
      }
      const v = res.version ?? base + 1
      setInfo({ ...i, name: graph.name, latestVersion: v, version: v, savedByAgent: false, enabledVersion: i.enabled && !res.pending_review ? v : i.enabledVersion })
      void refreshInfo()
    }
    return toSave(res)
  }
  const saveLayout = async (positions: AutomationLayout): Promise<void> => {
    const i = info()
    if (!i) return
    if (layoutStale) {
      notifyLayoutConflict()
      throw new Error('the layout changed in another tab')
    }
    const res = await app.call('automations.save_layout', { id: i.id, base_revision: layoutRevision, positions })
    if (res.ok) {
      layoutRevision = res.revision
      positionsBase = res.revision
      return
    }
    // Another tab saved positions meanwhile: they are not overwritten; «Refresh» shows them.
    layoutRevision = res.revision
    layoutStale = true
    conflict = { positions: res.positions ?? {}, revision: res.revision }
    notifyLayoutConflict()
    throw new Error('the layout changed in another tab')
  }
  // The positions another tab saved, with the revision they are: «Refresh» applies them only while they
  // are still the stored layout's (a fresher snapshot replaces them and withdraws the notice).
  let conflict: { positions: AutomationLayout; revision: number } | null = null
  const LAYOUT_TOAST = 'layout-conflict'
  const clearConflict = () => {
    conflict = null
    app.toast.remove(LAYOUT_TOAST)
  }
  const notifyLayoutConflict = () => {
    app.toast.notify({
      id: LAYOUT_TOAST, variant: 'warning', title: t('Editor.LayoutChanged'),
      action: {
        label: t('Editor.LayoutRefresh'),
        onAction: () => {
          const p = conflict
          if (!p || p.revision !== layoutRevision) return
          conflict = null
          layoutStale = false
          positionsBase = p.revision
          setRemoteLayout({ positions: p.positions })
        },
      },
    })
  }
  const reload = async () => {
    const i = info()
    if (!i) throw new Error('nothing to reload')
    const g = await app.call('automations.get', { id: i.id })
    layoutRevision = g.automation.layout_revision
    positionsBase = layoutRevision
    layoutStale = false
    clearConflict()
    setInfo({ ...i, enabled: g.automation.enabled, enabledVersion: g.automation.enabled_version, latestVersion: g.automation.latest_version, version: g.version, versions: g.versions, savedByAgent: g.saved_by.origin === 'agent', manual: manualOf(g) })
    return { graph: g.graph as AutomationGraph, version: g.version, layout: g.automation.layout }
  }
  const api: EditorApi = {
    validate: (graph) => app.call('automations.validate', { graph }),
    dryRun: (input) => app.call('automations.dry_run', { graph: input.graph, subject: input.subject, events: input.events as never, ...(input.context ? { context: input.context as never } : {}) }) as never,
    testNode: async (input) => {
      const res = await app.call('automations.test_node', { graph: input.graph, node_id: input.node_id, ...(input.subject ? { subject: input.subject } : {}), ...(input.context ? { context: input.context as never } : {}) })
      return { outcome: res.outcome, ...(res.output !== undefined ? { output: res.output } : {}), ...(res.logs ? { logs: res.logs } : {}), ...(res.duration_ms ? { duration_ms: res.duration_ms } : {}), ...(res.error ? { error: res.error } : {}) }
    },
    searchSubjects: (entity, query) => app.call('automations.search_subjects', { entity, query }),
    runs: async (input) => {
      const page = await app.call('automations.runs', {
        automation_id: input.automation_id, ...(input.state && input.state.length > 0 ? { state: input.state } : {}),
        ...(input.subject ? { subject: input.subject } : {}), ...(input.cursor ? { cursor: input.cursor } : {}), ...(input.limit ? { limit: input.limit } : {}),
      })
      return { items: page.items.map(runSummaryOf), next_cursor: page.next_cursor }
    },
    run: async (runId) => {
      try {
        const data = await app.call('automations.run', { run_id: runId })
        // A run of another graph is not this page's (RV6: one id opens its own graph's tab).
        if (info() && data.run.automation_id !== info()!.id) {
          window.location.replace(graphPath(app.props.basePath, data.run.automation_id, { run: runId }))
        }
        return runDataOf(data)
      } catch (e) {
        if (errorCode(e) === 'not_found') throw Object.assign(new Error('run not found'), { code: 'not_found' })
        throw e
      }
    },
    version: async (version) => {
      const g = await app.call('automations.get', { id: info()!.id, version })
      return { graph: g.graph as AutomationGraph, layout: g.automation.layout }
    },
    cancelRun: async (runId) => {
      const res = await app.call('automations.run_cancel', { run_id: runId })
      if (!res.ok) throw Object.assign(new Error(res.error ?? 'refused'), { code: res.error ?? 'refused' })
    },
    retryRun: async (input) => {
      await app.call('automations.run_retry', { run_id: input.run_id, wait_id: input.wait_id ?? '' })
    },
    resolveRun: async (input) => {
      await app.call('automations.run_resolve', { run_id: input.run_id, wait_id: input.wait_id ?? '', outcome: input.outcome })
    },
  }

  // A node's connection: the tenant's working connections of the node's connector kinds.
  const fieldSlot: FieldSlot = (field) => {
    if (field.param !== 'connection') return undefined
    const data = loaded()
    if (!data) return undefined
    const node = data.catalog.nodes.find((n) => n.type === field.nodeType) as (EditorCatalog['nodes'][number] & { requires?: string[] }) | undefined
    const event = data.catalog.events.find((e) => e.key === field.nodeType) as (EditorCatalog['events'][number] & { requires?: string[] }) | undefined
    const kinds = node?.requires ?? (node?.connector ? [node.connector] : event?.requires ?? [])
    if (kinds.length === 0) return undefined
    const options = kinds.flatMap((k) => data.connections[k] ?? [])
    return (
      <div class="flex flex-col gap-1" data-testid="connection-field">
        <Show when={options.length > 0} fallback={
          <p class="text-sm text-300">{t('Editor.NoConnection')} <a class="text-brand underline" href={app.props.integrationsPath}>{t('Editor.OpenIntegrations')}</a></p>
        }>
          <Select
            label={t('Editor.Connection')}
            value={typeof field.value === 'string' ? field.value : ''}
            disabled={field.readOnly}
            placeholder={options.length > 1 ? t('Editor.PickConnection') : t('Editor.OnlyConnection', { name: options[0]?.name ?? '' })}
            options={options.map((o) => ({ value: o.id, label: o.name }))}
            onChange={(e) => field.onChange(e.currentTarget.value || undefined)}
          />
        </Show>
      </div>
    )
  }

  const versions = createMemo<VersionEntry[]>(() => (info()?.versions ?? []).map((v) => ({
    version: v.version, ...(v.note ? { note: v.note } : {}), ...(v.enabled ? { enabled: true } : {}),
    by: v.origin === 'agent' ? t('Editor.ByAgent', { name: v.by ?? '' }) : v.by ?? '',
  })))
  // «Run» starts the enabled version: its manual triggers and input schemas, whatever version is open.
  const manual = () => info()?.manual ?? []
  const isLatest = () => (info()?.version ?? 0) === (info()?.latestVersion ?? 0)
  const pending = () => {
    const i = info()
    return Boolean(i && i.latestVersion > 0 && i.enabledVersion !== i.latestVersion && (i.savedByAgent || i.enabledVersion !== null))
  }

  const changeTab = (next: 'edit' | 'runs') => {
    setTab(next)
    const i = info()
    if (!i) return
    // Edit drops the run from the path; Runs returns to the run and filters it showed.
    const href = next === 'runs'
      ? writeRunsView(app.props.basePath, i.id, runsView(), window.location.href)
      : (() => {
        const url = new URL(graphPath(app.props.basePath, i.id), window.location.origin)
        const v = new URLSearchParams(window.location.search).get('version')
        if (v) url.searchParams.set('version', v)
        if (selected()) url.searchParams.set('node', selected()!)
        return url.pathname + url.search
      })()
    if (href !== window.location.pathname + window.location.search) {
      window.history.pushState({}, '', href)
      shownHref = window.location.href
    }
  }
  const changeRunsView = (view: RunsView, push: boolean) => {
    setRunsView(view)
    const i = info()
    if (!i) return
    const href = writeRunsView(app.props.basePath, i.id, view, window.location.href)
    if (href === window.location.pathname + window.location.search) return
    if (push) window.history.pushState({}, '', href)
    else window.history.replaceState({}, '', href)
    shownHref = window.location.href
  }
  const select = (id: string | null) => {
    setSelected(id)
    setURL((url) => { if (id) url.searchParams.set('node', id); else url.searchParams.delete('node') })
  }

  const crumb = (
    <a href={app.props.basePath} class="hover:underline" data-testid="crumb">{t('Title')}</a>
  )
  const actions = () => (
    <>
      <Show when={info() && can().operate && info()!.enabled && manual().length > 0}>
        <button type="button" class="ae-btn ae-btn--ghost" onClick={() => setDialog('run')} data-testid="run-manual"><Icon name="play" />{t('Editor.Run')}</button>
      </Show>
      <Show when={info() && can().author && isLatest() && (!info()!.enabled || can().enable)}>
        <button type="button" class="ae-iconbtn" aria-label={t('Editor.Delete')} title={t('Editor.Delete')} onClick={() => setDialog('delete')} data-testid="delete"><Icon name="trash" /></button>
      </Show>
    </>
  )
  const banner = () => {
    const i = info()
    const parts: JSX.Element[] = []
    // The Runs tab works on a phone (a read-only outline): the «open on a computer» note is the editor's.
    if (phone() && tab() === 'edit') {
      parts.push(
        <span class="flex flex-wrap items-center gap-2" data-testid="phone-banner">
          <span>{t('Editor.DesktopOnly')}</span>
          <CopyButton text={window.location.href} showText labels={{ copy: t('Editor.CopyLink'), copied: t('Editor.LinkCopied') }} />
        </span>,
      )
    }
    if (i && !can().enable) {
      // AL7: why there is no switch, in words.
      parts.push(<span class="flex items-center gap-1" data-testid="enable-note"><Icon name="lock" />{app.props.agent ? t('Enable.AgentNote') : t('Enable.NoRight')}</span>)
    }
    if (i && !isLatest()) {
      parts.push(<Button variant="secondary" size="sm" onClick={() => openVersion(i.latestVersion)} data-testid="open-latest">{t('Editor.OpenLatest', { version: i.latestVersion })}</Button>)
    } else if (i && pending()) {
      parts.push(
        <span class="flex flex-wrap items-center gap-2" data-testid="pending-banner">
          <Show when={i.savedByAgent} fallback={<span>{t('Editor.PendingVersion', { version: i.latestVersion })}</span>}>
            <span class="flex items-center gap-1"><Icon name="robot" />{t('Editor.SavedByAgent', { version: i.latestVersion })}</span>
          </Show>
          <Show when={can().enable}>
            <Button variant="primary" size="sm" onClick={() => setDialog('review')} data-testid="review-and-enable">{t('Editor.ReviewAndEnable', { version: i.latestVersion })}</Button>
          </Show>
        </span>,
      )
    }
    return parts.length > 0 ? <>{parts}</> : undefined
  }

  return (
    <div class="h-[calc(100vh-4rem)] min-h-[32rem] min-w-0" data-testid="editor-page">
      <Switch>
        <Match when={loadError()}>
          <div role="alert" class="m-4 flex flex-col items-start gap-3 rounded-lg border border-subtle p-6" data-testid="graph-load-error">
            <p>{loadError()}</p>
            <div class="flex gap-2">
              <Button variant="secondary" onClick={() => void load()} icon={<Icon name="refresh" />}>{t('Common.Retry')}</Button>
              <Button variant="ghost" href={app.props.basePath}>{t('Editor.BackToList')}</Button>
            </div>
          </div>
        </Match>
        <Match when={!loaded()}>
          <EditorSkeleton locale={locale} />
        </Match>
        <Match when={loaded()}>
          {(data) => (
            <Show when={data().key} keyed>
              {(_key) => (
                <Editor
                  graph={data().graph}
                  version={data().version}
                  catalog={data().catalog}
                  layout={data().layout}
                  api={api}
                  save={save}
                  {...(info() && !data().readOnly ? { saveLayout } : {})}
                  remoteLayout={remoteLayout()}
                  reload={reload}
                  versions={versions()}
                  onOpenVersion={openVersion}
                  onDirtyChange={setDirty}
                  enabled={info()?.enabled ?? false}
                  {...(info() && can().enable ? { toggleEnabled: () => setDialog(info()!.enabled ? 'disable' : 'review') } : {})}
                  {...(data().readOnly ? { readOnly: data().readOnly! } : {})}
                  fieldSlot={fieldSlot}
                  {...(info() ? {
                    tab: tab(), onTab: changeTab, automationId: info()!.id, runsView: runsView(), onRunsView: changeRunsView,
                    ...(can().operate && info()!.enabled && manual().length > 0 ? { onStartRun: () => setDialog('run') } : {}),
                  } : {})}
                  can={{ author: can().author, operate: can().operate }}
                  connectionName={(id) => Object.values(data().connections).flat().find((c) => c.id === id)?.name}
                  crumb={crumb}
                  actions={actions()}
                  banner={banner()}
                  locale={locale}
                  selected={selected()}
                  onSelect={select}
                />
              )}
            </Show>
          )}
        </Match>
      </Switch>
      <Show when={dialog() === 'review' && info()}>
        <ReviewDialog
          id={info()!.id} version={info()!.latestVersion} name={info()!.name}
          onEnabled={(v) => { setDialog(null); setInfo({ ...info()!, enabled: true, enabledVersion: v, savedByAgent: false }); void refreshInfo() }}
          onClose={() => setDialog(null)}
        />
      </Show>
      <Show when={dialog() === 'disable' && info()}>
        <DisableDialog id={info()!.id} name={info()!.name} onDisabled={() => { setDialog(null); setInfo({ ...info()!, enabled: false, enabledVersion: null }); void refreshInfo() }} onClose={() => setDialog(null)} />
      </Show>
      <Show when={dialog() === 'delete' && info()}>
        <DeleteDialog id={info()!.id} name={info()!.name} onDeleted={() => app.navigate(app.props.basePath)} onClose={() => setDialog(null)} />
      </Show>
      <Show when={discard()}>
        <Dialog open onOpenChange={(open) => { if (!open) cancelDiscard() }} class="md:w-full md:max-w-md" aria-labelledby="discard-title" data-testid="discard-dialog">
          <header class="border-b border-subtle py-3 pl-4 pr-2"><h3 id="discard-title" class="font-medium" data-dialog-title tabindex="-1">{t('Editor.DiscardTitle')}</h3></header>
          <p class="px-4 py-3">{t('Editor.DiscardText')}</p>
          <footer class="border-t border-subtle px-4 py-3">
            <menu class="flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
              <Button variant="secondary" class="min-h-11 justify-center" data-dialog-close onClick={cancelDiscard} data-testid="discard-cancel">{t('Editor.DiscardKeep')}</Button>
              <Button variant="danger" class="min-h-11 justify-center" onClick={confirmDiscard} data-testid="discard-confirm">{t('Editor.DiscardConfirm')}</Button>
            </menu>
          </footer>
        </Dialog>
      </Show>
      <Show when={dialog() === 'run' && info()}>
        <RunDialog id={info()!.id} triggers={manual()} subjects={(loaded()?.catalog as { subjects?: string[] } | undefined)?.subjects ?? []} onClose={() => setDialog(null)} />
      </Show>
    </div>
  )
}

function manualOf(g: GetResult): ManualTrigger[] {
  return g.manual_triggers.map((m) => ({ id: m.trigger_id, title: m.title, schema: m.input_schema ?? null }))
}
