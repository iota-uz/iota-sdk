import { For, Show, createEffect, createSignal, on, onCleanup, onMount, type Component } from 'solid-js'
import { fetchTypes } from './api'
import { location, type Problem } from './diagnostics'
import { EditorBinding, type MonacoLike } from './editorBinding'
import { describeError, shortHash, useConfig } from './context'
import { loadMonaco, type MonacoApi } from './monaco'
import { TypeLibraries, type TsDefaults } from './monacoTypes'
import type { RunResult } from './workspace'
import { Workspace } from './workspace'
import type { PackageInfo, VersionInfo } from './types'

export interface EditorTabProps {
  pkg: PackageInfo
  /** Called after a successful publish so the versions list refreshes. */
  onPublished(version: VersionInfo): void
}

/**
 * EditorTab is the multi-file editor of one package draft. Everything with
 * state lives in Workspace (store, diagnostics, requests); this component
 * draws it and owns the Monaco lifecycle: lazy load, theme, type libraries,
 * and disposal of editor, models and requests on unmount.
 */
export const EditorTab: Component<EditorTabProps> = (props) => {
  const cfg = useConfig()
  const { t, rights } = cfg
  const ns = props.pkg.namespace
  let container!: HTMLDivElement
  let binding: EditorBinding | undefined
  let disposed = false
  const [rev, setRev] = createSignal(0)
  // diagnostics change outside the store, so the markers are redrawn on every change
  const bump = () => { setRev((n) => n + 1); binding?.sync() }
  const [active, setActive] = createSignal<string>()
  const [editorError, setEditorError] = createSignal<string>()
  const [run, setRun] = createSignal<RunResult>()
  const [newPath, setNewPath] = createSignal('')
  const [renaming, setRenaming] = createSignal<{ from: string; to: string }>()
  const [version, setVersion] = createSignal('')
  const [publishing, setPublishing] = createSignal(false)

  const ws = new Workspace(cfg.api, ns, {
    changed: bump,
    problem: (m) => cfg.notify('error', m === 'unsaved' ? t('Unsaved') : m),
  })
  const unsubscribe = ws.store.subscribe(bump)
  const kitLibs: { current?: TypeLibraries } = {}
  const sdkLibs: { current?: TypeLibraries } = {}

  const entries = () => { rev(); return ws.store.entries() }
  const current = () => { rev(); const p = active(); return p ? ws.store.get(p) : undefined }
  const dirtyCount = () => { rev(); return ws.store.dirtyPaths().length }
  const problems = (): Problem[] => { rev(); return ws.diagnostics.problems(ws.fileStates()) }
  const errorCount = () => problems().filter((p) => p.diagnostic.severity === 'error' && !p.stale).length

  const open = (path: string) => {
    setActive(path)
    binding?.open(path)
  }

  const startMonaco = async () => {
    setEditorError(undefined)
    try {
      const monaco: MonacoApi = await loadMonaco()
      if (disposed) return
      binding = new EditorBinding(monaco as unknown as MonacoLike, container, ws.store, ws.diagnostics, {
        theme: cfg.host.theme.current(), readOnly: !rights.author,
      })
      const off = cfg.host.theme.subscribe((theme) => binding?.setTheme(theme))
      onCleanup(off)
      const a = active()
      if (a) binding.open(a)
      const ts = monaco.languages.typescript
      const defaults = [ts.typescriptDefaults, ts.javascriptDefaults] as unknown as TsDefaults[]
      kitLibs.current = new TypeLibraries(defaults)
      sdkLibs.current = new TypeLibraries(defaults)
      void loadTypes()
    } catch (err) {
      if (!disposed) setEditorError(`${t('EditorLoadFailed')}: ${describeError(err)}`)
    }
  }

  const abort = new AbortController()
  const loadTypes = async () => {
    for (const [name, libs] of [['kit', kitLibs], ['sdk', sdkLibs]] as const) {
      try {
        const files = await fetchTypes(`${cfg.typesBase}/${name}`, abort.signal)
        if (!disposed) libs.current?.set(files)
      } catch (err) {
        if (!disposed && !abort.signal.aborted) cfg.notify('error', `${t('TypesLoadFailed')} (${name}): ${describeError(err)}`)
      }
    }
  }

  onMount(() => {
    void startMonaco()
    void ws.open().then(() => {
      if (disposed || active()) return
      const first = ['src/contracts.ts', 'src/contracts.tsx', 'package.json'].find((p) => ws.store.get(p)) ?? ws.store.paths()[0]
      if (first) open(first)
    })
  })
  onCleanup(() => {
    disposed = true
    abort.abort()
    unsubscribe()
    binding?.dispose()
    binding = undefined
    kitLibs.current?.dispose()
    sdkLibs.current?.dispose()
    ws.dispose()
  })
  // a kit change after check/publish (new activation elsewhere) is picked up on the next mount
  createEffect(on(() => cfg.host.theme.current(), (theme) => binding?.setTheme(theme)))

  const doRun = async (kind: 'check' | 'build' | 'test') => {
    const r = await ws.run(kind)
    if (r && !disposed) setRun(r)
  }

  const save = async () => {
    const p = active()
    if (p) await ws.save(p)
  }

  const doPublish = async () => {
    setPublishing(true)
    try {
      if (!(await ws.saveAll())) { cfg.notify('error', t('Unsaved')); return }
      const v = await cfg.api.publish(ns, version().trim() || undefined)
      if (disposed) return
      cfg.notify('info', `${t('Publish')}: ${v.version} (${shortHash(v.sourceHash)})`)
      props.onPublished(v)
    } catch (err) {
      ws.recordDiagnosticsFrom(err)
      cfg.notify('error', describeError(err))
    } finally {
      setPublishing(false)
    }
  }

  const addFile = async () => {
    const p = newPath().trim()
    if (!p) return
    if (await ws.createFile(p)) { setNewPath(''); open(p) } else cfg.notify('error', `${t('NewFilePath')}: ${p}`)
  }

  const removeFile = async (p: string) => {
    if (!window.confirm(`${t('DeleteConfirm')} ${p}?`)) return
    if (await ws.deleteFile(p) && active() === p) setActive(undefined)
  }

  const doRename = async () => {
    const r = renaming()
    if (!r) return
    if (await ws.rename(r.from, r.to.trim())) {
      if (active() === r.from) open(r.to.trim())
      setRenaming(undefined)
    } else cfg.notify('error', `${t('Rename')}: ${r.to}`)
  }

  const onKeyDown = (e: KeyboardEvent) => {
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') {
      e.preventDefault()
      void save()
    }
  }

  const badge = (p: string) => {
    const e = ws.store.get(p)
    if (!e) return ''
    if (e.status === 'conflict') return '⚠'
    if (e.status === 'error') return '!'
    if (e.status === 'saving') return '…'
    return ws.store.isDirty(p) ? '●' : ''
  }

  return (
    <div class="iota-cfg-editor-tab" onKeyDown={onKeyDown}>
      <div class="row" style={{ 'margin-bottom': '8px' }}>
        <button type="button" disabled={!rights.author || !current() || ws.busy} onClick={() => void save()}>{t('Save')}</button>
        <button type="button" disabled={!rights.author || dirtyCount() === 0 || ws.busy} onClick={() => void ws.saveAll()}>{t('SaveAll')}</button>
        <button type="button" disabled={!rights.author || ws.busy} onClick={() => void doRun('check')}>{t('Check')}</button>
        <button type="button" disabled={!rights.author || ws.busy} onClick={() => void doRun('build')}>{t('Build')}</button>
        <button type="button" disabled={!rights.author || ws.busy} onClick={() => void doRun('test')}>{t('Test')}</button>
        <span class="grow" />
        <Show when={rights.publish}>
          <input type="text" placeholder={t('Version')} value={version()} onInput={(e) => setVersion(e.currentTarget.value)} size={10} />
          <button type="button" class="primary" disabled={ws.busy || publishing()} onClick={() => void doPublish()}>{t('Publish')}</button>
        </Show>
        <a href={cfg.api.exportUrl(ns)} download="">{t('Export')}</a>
        <Show when={ws.busy}><span class="muted">{t('Working')}</span></Show>
        <Show when={dirtyCount() > 0}><span class="warning">{t('Unsaved')}: {dirtyCount()}</span></Show>
      </div>

      <div class="iota-cfg-editor">
        <div class="iota-cfg-files">
          <h3>{t('Files')}</h3>
          <ul style={{ 'list-style': 'none', margin: 0, padding: 0 }}>
            <For each={entries()}>{(e) => (
              <li classList={{ selected: active() === e.path }}>
                <button type="button" class="file" title={e.path} onClick={() => open(e.path)}>{e.path} {badge(e.path)}</button>
                <Show when={rights.author}>
                  <button type="button" title={t('Rename')} onClick={() => setRenaming({ from: e.path, to: e.path })}>✎</button>
                  <button type="button" class="danger" title={t('Delete')} onClick={() => void removeFile(e.path)}>×</button>
                </Show>
              </li>
            )}</For>
          </ul>
          <Show when={rights.author}>
            <div class="row" style={{ 'margin-top': '8px' }}>
              <input type="text" placeholder={t('NewFilePath')} value={newPath()} onInput={(e) => setNewPath(e.currentTarget.value)} style={{ width: '100%' }}
                onKeyDown={(e) => { if (e.key === 'Enter') void addFile() }} />
              <button type="button" onClick={() => void addFile()}>{t('NewFile')}</button>
            </div>
          </Show>
          <Show when={renaming()}>{(r) => (
            <div class="row" style={{ 'margin-top': '8px' }}>
              <input type="text" value={r().to} onInput={(e) => setRenaming({ from: r().from, to: e.currentTarget.value })} style={{ width: '100%' }} />
              <button type="button" onClick={() => void doRename()}>{t('Rename')}</button>
              <button type="button" onClick={() => setRenaming(undefined)}>{t('Cancel')}</button>
            </div>
          )}</Show>
        </div>

        <div>
          <Show when={current()?.status === 'conflict'}>
            <div class="iota-cfg-notice error" role="alert">
              <span><strong>{t('Conflict')}</strong> {t('ConflictText')}</span>
              <span class="row">
                <button type="button" onClick={() => { const p = active(); if (p) ws.store.resolveConflict(p, 'mine') }} disabled={!current()?.remote}>{t('KeepMine')}</button>
                <button type="button" onClick={() => { const p = active(); if (p) ws.store.resolveConflict(p, 'theirs') }} disabled={!current()?.remote}>{t('TakeTheirs')}</button>
              </span>
            </div>
          </Show>
          <Show when={current()?.status === 'error'}><div class="error" role="alert">{current()?.error}</div></Show>
          <Show when={editorError()}>
            <div class="error" role="alert">{editorError()} <button type="button" onClick={() => void startMonaco()}>{t('Retry')}</button></div>
          </Show>
          <Show when={!active() && !editorError()}><p class="muted">{t('SelectFile')}</p></Show>
          <div ref={container} class="iota-cfg-monaco" data-testid="monaco-container" />
        </div>
      </div>

      <div style={{ 'margin-top': '8px' }}>
        <h3>{t('Problems')} <span class="muted">({errorCount()})</span></h3>
        <Show when={problems().length > 0} fallback={<p class="muted">{t('NoProblems')}</p>}>
          <div data-testid="problems">
            <For each={problems()}>{(p) => (
              <div classList={{ 'iota-cfg-problem': true, stale: p.stale }} role="button" tabindex="0"
                onClick={() => { open(p.path); binding?.reveal(p.path, p.diagnostic.line ?? 1, p.diagnostic.column ?? 1) }}>
                <span classList={{ error: p.diagnostic.severity === 'error', warning: p.diagnostic.severity === 'warning' }}>{p.diagnostic.severity}</span>
                <code>{location(p.diagnostic)}</code>
                <span>{p.diagnostic.message}</span>
                <span class="muted">{p.diagnostic.code}{p.stale ? ` · ${t('StaleProblems')}` : ''}</span>
              </div>
            )}</For>
          </div>
        </Show>
        <Show when={run()}>{(r) => <RunSummary run={r()} />}</Show>
      </div>
    </div>
  )
}

const RunSummary: Component<{ run: RunResult }> = (p) => {
  const { t } = useConfig()
  return (
    <div class="iota-cfg-card" style={{ 'margin-top': '8px' }} data-testid="run-summary">
      <h3>{t('Output')}</h3>
      <Show when={p.run.kind === 'build' ? p.run.report : undefined}>{(r) => (
        <dl class="mono">
          <dt>{t('SourceHash')}</dt><dd title={r().sourceHash}>{r().sourceHash ?? '—'}</dd>
          <dt>{t('ArtifactHash')}</dt><dd title={r().artifactHash}>{r().artifactHash ?? '—'}</dd>
          <dt>{t('KitHash')}</dt><dd>{r().kitHash}</dd>
        </dl>
      )}</Show>
      <Show when={p.run.kind === 'check' ? p.run.report : undefined}>{(r) => (
        <p classList={{ ok: !r().hasErrors, error: r().hasErrors }}>{r().hasErrors ? t('Problems') : t('NoProblems')} · {t('KitHash')}: <code>{shortHash(r().kitHash)}</code></p>
      )}</Show>
      <Show when={p.run.kind === 'test' ? p.run.report : undefined}>{(r) => (
        <>
          <p classList={{ ok: r().passed, error: !r().passed }}>{r().passed ? t('TestsPassed') : t('TestsFailed')}</p>
          <ul>
            <For each={r().cases}>{(c) => (
              <li classList={{ ok: c.passed, error: !c.passed }}>{c.passed ? '✓' : '✗'} {c.name} <span class="muted">{c.file} · {c.durationMs} ms</span>
                <Show when={c.message}><pre class="mono">{c.message}</pre></Show></li>
            )}</For>
          </ul>
        </>
      )}</Show>
    </div>
  )
}
