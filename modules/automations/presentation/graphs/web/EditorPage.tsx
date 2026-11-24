import { For, Match, Show, Switch, createMemo, createSignal, onCleanup, onMount, type JSX } from 'solid-js'
import {
  Editor, EditorSkeleton,
  type AutomationGraph, type AutomationLayout, type EditorApi, type EditorCatalog, type FieldSlot, type SaveResult, type VersionEntry,
} from '@iota-uz/sdk/automation-editor'
import { Button, CopyButton, Dialog, Select } from '@iota-uz/sdk/solid-ui'
import type { RPC } from './Automations.solid.generated'
import { useApp } from './context'
import { DeleteDialog, DisableDialog, ReviewDialog } from './Dialogs'
import { Icon } from './Icon'
import {
  absoluteTime, editorLocale, emptyGraph, errorText, graphPath, relativeTime, runIcon, runStatusKey, runTone,
  type ConnectionOptions, type GetResult, type ManualTrigger,
} from './model'
import { RunDialog } from './RunDialog'

type CatalogResult = RPC['automations.catalog']['result']
type RunsResult = RPC['automations.runs']['result']
type RunData = RPC['automations.run']['result']

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

const TONE: Record<string, string> = { ok: 'text-green', bad: 'text-red-600', warn: 'text-yellow', muted: 'text-300' }

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
  const [selected, setSelected] = createSignal<string | null>(search.get('node'))
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
      setTab(q.get('tab') === 'runs' ? 'runs' : 'edit')
      setSelected(q.get('node'))
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
  const [discard, setDiscard] = createSignal<{ version: number; revert?: string } | null>(null)
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
    if (!d.revert) setURL((url) => { if (d.version === 0) url.searchParams.delete('version'); else url.searchParams.set('version', String(d.version)) }, true)
    void load(d.version)
  }
  const cancelDiscard = () => {
    const d = discard()
    setDiscard(null)
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
    setURL((url) => { if (next === 'runs') url.searchParams.set('tab', 'runs'); else url.searchParams.delete('tab') }, true)
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
    if (phone()) {
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
                  {...(info() ? { tab: tab(), onTab: changeTab, runsBody: <Runs id={info()!.id} run={app.props.run ?? ''} /> } : {})}
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

/** The runs of the graph (the Runs tab's list; the run view on the canvas is #1347's). */
function Runs(props: { id: string; run: string }): JSX.Element {
  const app = useApp()
  const t = app.t
  const [data, setData] = createSignal<RunsResult>()
  const [error, setError] = createSignal<string>()
  const [open, setOpen] = createSignal<RunData>()
  const load = async () => {
    setError(undefined)
    try { setData(await app.call('automations.runs', { automation_id: props.id })) } catch (e) { setError(errorText(t, e, t('Actions.LoadRuns'))) }
  }
  const show = async (run: string) => {
    try { setOpen(await app.call('automations.run', { run_id: run })) } catch (e) { app.toast.notify({ variant: 'error', title: errorText(t, e, t('Actions.LoadRuns')) }) }
  }
  onMount(() => { void load(); if (props.run) void show(props.run) })
  return (
    <div class="flex flex-col gap-3 p-4" data-testid="runs">
      <Switch>
        <Match when={error()}>
          <div role="alert" class="flex items-center gap-2"><span>{error()}</span><Button variant="secondary" size="sm" onClick={() => void load()}>{t('Common.Retry')}</Button></div>
        </Match>
        <Match when={!data()}><p role="status" class="text-300">{t('Common.Loading')}</p></Match>
        <Match when={data()!.items.length === 0}><p class="text-300" data-testid="runs-empty">{t('Runs.Empty')}</p></Match>
        <Match when={data()}>
          <ul class="flex flex-col divide-y divide-subtle rounded-lg border border-subtle bg-surface-600">
            <For each={data()!.items}>
              {(r) => (
                <li>
                  <button type="button" class="flex w-full flex-wrap items-center gap-3 px-4 py-2 text-left text-sm hover:bg-surface-400" classList={{ 'bg-brand/5': open()?.run.run_id === r.run_id }} onClick={() => void show(r.run_id)} data-testid="run-row">
                    <span class={`flex items-center gap-1.5 ${TONE[runTone(r.status)]}`}><Icon name={runIcon(r.status)} />{t(runStatusKey(r.status))}</span>
                    <span class="text-300">v{r.version}</span>
                    <time class="tabular-nums text-300" datetime={r.created_at} title={absoluteTime(r.created_at, app.lang)}>{relativeTime(r.created_at, app.now(), app.lang)}</time>
                    <Show when={r.error}><span class="truncate text-red-600">{r.error!.message}</span></Show>
                  </button>
                </li>
              )}
            </For>
          </ul>
        </Match>
      </Switch>
      <Show when={open()}>
        {(d) => (
          <section class="flex flex-col gap-2 rounded-lg border border-subtle p-4 text-sm" aria-label={t('Runs.Steps')} data-testid="run-steps">
            <h3 class="font-medium">{t('Runs.Steps')}</h3>
            <Show when={d().steps.length > 0} fallback={<p class="text-300">{t('Runs.NoSteps')}</p>}>
              <ol class="flex flex-col gap-1">
                <For each={d().steps}>{(s) => <li class="flex flex-wrap gap-2"><span class="font-mono">{s.node_id}</span><span class="text-300">{s.outcome ?? ''}</span><Show when={s.error}><span class="text-red-600">{s.error!.message}</span></Show></li>}</For>
              </ol>
            </Show>
          </section>
        )}
      </Show>
    </div>
  )
}

function manualOf(g: GetResult): ManualTrigger[] {
  return g.manual_triggers.map((m) => ({ id: m.trigger_id, title: m.title, schema: m.input_schema ?? null }))
}
