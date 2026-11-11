import { For, Show, createSignal, onCleanup, type Component } from 'solid-js'
import { useClientHost, type SolidRouteProps } from '@iota-uz/sdk/solid'
import './configurator.css'
import { ManageApi } from './api'
import { ConfigCtx, MSG_PREFIX, describeError, rightsOf, useConfig, type Ctx, type NoticeKind } from './context'
import { AuditTab } from './AuditTab'
import { EditorTab } from './EditorTab'
import { createLoader } from './load'
import { VersionsTab } from './VersionsTab'
import { WorkflowsTab } from './WorkflowsTab'
import type { PackageInfo } from './types'

/** Initial props of the screen (Go: configurator.Props). */
export interface ScreenProps {
  basePath: string
  apiBase: string
  typesBase: string
}

type Tab = 'editor' | 'versions' | 'workflows' | 'audit'

interface Notice { id: number; kind: NoticeKind; text: string }

/**
 * Configurator is the SDK's extension configurator screen. It is a standard
 * Solid screen of the canonical client host: the host owns loading, mounting
 * and disposal; this component owns what is inside. All data comes from the
 * /_ext/manage API with the user's session, so it shows and does exactly what
 * the MCP tools of the same service do.
 */
const Configurator: Component<SolidRouteProps<ScreenProps>> = (props) => {
  const host = useClientHost()
  const init = props.route.initial
  const [notices, setNotices] = createSignal<Notice[]>([])
  let noticeId = 0
  const timers = new Set<ReturnType<typeof setTimeout>>()
  onCleanup(() => { for (const id of timers) clearTimeout(id); timers.clear() })

  const ctx: Ctx = {
    api: new ManageApi({ base: init.apiBase, csrf: () => host.session.snapshot().csrf }),
    host,
    rights: rightsOf(props.route.permissions),
    typesBase: init.typesBase,
    t: (key) => host.locale.t(MSG_PREFIX + key),
    notify(kind, text) {
      const id = ++noticeId
      setNotices((n) => [...n.slice(-4), { id, kind, text }])
      if (kind === 'info') {
        const timer = setTimeout(() => { timers.delete(timer); dismiss(id) }, 6000)
        timers.add(timer)
      }
    },
  }
  const dismiss = (id: number) => setNotices((n) => n.filter((x) => x.id !== id))

  return (
    <ConfigCtx.Provider value={ctx}>
      <div class="iota-cfg" data-testid="extension-configurator">
        <h2>{ctx.t('Title')}</h2>
        <div class="iota-cfg-notices" aria-live="polite">
          <For each={notices()}>{(n) => (
            <div classList={{ 'iota-cfg-notice': true, error: n.kind === 'error' }} role={n.kind === 'error' ? 'alert' : 'status'}>
              <span>{n.text}</span>
              <button type="button" aria-label={ctx.t('Close')} onClick={() => dismiss(n.id)}>×</button>
            </div>
          )}</For>
        </div>
        <Shell basePath={init.basePath} />
      </div>
    </ConfigCtx.Provider>
  )
}

export default Configurator

const queryNamespace = (): string | undefined => new URLSearchParams(window.location.search).get('package') ?? undefined

const Shell: Component<{ basePath: string }> = (props) => {
  const cfg = useConfig()
  const { t, api, rights } = cfg
  const packages = createLoader((s) => api.listPackages(s))
  const [selected, setSelected] = createSignal<string | undefined>(queryNamespace())
  const [tab, setTab] = createSignal<Tab>('editor')
  const [refresh, setRefresh] = createSignal(0)
  const [form, setForm] = createSignal<'none' | 'new' | 'git'>('none')
  const [ns, setNs] = createSignal('')
  const [git, setGit] = createSignal({ repo: '', commit: '', subdir: '' })
  const [busy, setBusy] = createSignal(false)

  const choose = (namespace: string | undefined) => {
    setSelected(namespace)
    const url = new URL(window.location.href)
    if (namespace) url.searchParams.set('package', namespace); else url.searchParams.delete('package')
    window.history.replaceState(null, '', url)
  }
  const pkg = (): PackageInfo | undefined => (packages.data() ?? []).find((p) => p.namespace === selected())

  const create = async () => {
    setBusy(true)
    try {
      const p = await api.createPackage(ns().trim())
      await packages.reload()
      choose(p.namespace)
      setNs(''); setForm('none')
    } catch (err) { cfg.notify('error', describeError(err)) } finally { setBusy(false) }
  }
  const importGit = async () => {
    setBusy(true)
    try {
      const g = git()
      const p = await api.importGit(g.repo.trim(), g.commit.trim(), g.subdir.trim())
      await packages.reload()
      choose(p.namespace)
      setForm('none')
    } catch (err) { cfg.notify('error', describeError(err)) } finally { setBusy(false) }
  }

  const tabs: Array<{ id: Tab; label: string }> = [
    { id: 'editor', label: t('Editor') }, { id: 'versions', label: t('Versions') },
    { id: 'workflows', label: t('Workflows') }, { id: 'audit', label: t('Audit') },
  ]

  return (
    <div class="iota-cfg-shell">
      <aside class="iota-cfg-side" aria-label={t('Packages')}>
        <div class="row"><h3>{t('Packages')}</h3><button type="button" onClick={() => void packages.reload()} title={t('Refresh')}>⟳</button></div>
        <Show when={packages.error()}><p class="error" role="alert">{packages.error()?.message} <button type="button" onClick={() => void packages.reload()}>{t('Retry')}</button></p></Show>
        <Show when={packages.loading() && !packages.data()}><p class="muted">{t('Loading')}</p></Show>
        <Show when={(packages.data() ?? []).length === 0 && !packages.loading() && !packages.error()}><p class="muted">{t('NoPackages')}</p></Show>
        <ul>
          <For each={packages.data() ?? []}>{(p) => (
            <li classList={{ selected: selected() === p.namespace }}>
              <button type="button" onClick={() => choose(p.namespace)}>
                {p.namespace}
                <Show when={p.activeVersion}> <span class="iota-cfg-badge ok">{p.activeVersion}</span></Show>
              </button>
            </li>
          )}</For>
        </ul>
        <Show when={rights.author}>
          <div class="row" style={{ 'margin-top': '8px' }}>
            <button type="button" onClick={() => setForm(form() === 'new' ? 'none' : 'new')}>{t('NewPackage')}</button>
            <button type="button" onClick={() => setForm(form() === 'git' ? 'none' : 'git')}>{t('ImportGit')}</button>
          </div>
          <Show when={form() === 'new'}>
            <div class="row" style={{ 'margin-top': '6px' }}>
              <input type="text" placeholder={t('Namespace')} value={ns()} onInput={(e) => setNs(e.currentTarget.value)} style={{ width: '100%' }} />
              <button type="button" class="primary" disabled={busy() || !ns().trim()} onClick={() => void create()}>{t('Create')}</button>
            </div>
          </Show>
          <Show when={form() === 'git'}>
            <div style={{ display: 'flex', 'flex-direction': 'column', gap: '4px', 'margin-top': '6px' }}>
              <input type="text" placeholder={t('RepoURL')} value={git().repo} onInput={(e) => setGit({ ...git(), repo: e.currentTarget.value })} />
              <input type="text" placeholder={t('Commit')} value={git().commit} onInput={(e) => setGit({ ...git(), commit: e.currentTarget.value })} />
              <input type="text" placeholder={t('Subdir')} value={git().subdir} onInput={(e) => setGit({ ...git(), subdir: e.currentTarget.value })} />
              <button type="button" class="primary" disabled={busy() || !git().repo.trim() || !git().commit.trim()} onClick={() => void importGit()}>{t('Import')}</button>
            </div>
          </Show>
        </Show>
      </aside>

      <main class="iota-cfg-main">
        <Show when={pkg()} keyed fallback={<p class="muted">{t('SelectPackage')}</p>}>{(p) => (
          <>
            <div class="row">
              <h3>{p.namespace}</h3>
              <span class="iota-cfg-badge">{p.provenance}{p.repoCommit ? ` ${p.repoCommit.slice(0, 8)}` : ''}</span>
              <Show when={p.activeVersion}><span class="iota-cfg-badge ok">{t('Active')}: {p.activeVersion}</span></Show>
              <Show when={!rights.author}><span class="muted">{t('ReadOnly')}</span></Show>
            </div>
            <div class="iota-cfg-tabs" role="tablist">
              <For each={tabs}>{(x) => (
                <button type="button" role="tab" aria-selected={tab() === x.id} onClick={() => setTab(x.id)}>{x.label}</button>
              )}</For>
            </div>
            {/* the editor stays mounted across tabs so unsaved buffers and models survive; hidden, not destroyed */}
            <div style={{ display: tab() === 'editor' ? 'block' : 'none' }}>
              <EditorTab pkg={p} onPublished={() => { setRefresh((n) => n + 1); void packages.reload(); setTab('versions') }} />
            </div>
            <Show when={tab() === 'versions'}><VersionsTab pkg={p} refreshKey={refresh()} onChanged={() => void packages.reload()} /></Show>
            <Show when={tab() === 'workflows'}><WorkflowsTab namespace={p.namespace} /></Show>
            <Show when={tab() === 'audit'}><AuditTab namespace={p.namespace} refreshKey={refresh()} /></Show>
          </>
        )}</Show>
      </main>
    </div>
  )
}
