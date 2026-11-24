// One run on the canvas (RV3–RV6, RV10): the graph of the run's version with the run's overlay, a strip
// with its subject, state, connection indicator and version, the node drawer, and Cancel / Retry /
// Resolve for an operator. A live run is polled every 2 s while it runs or waits; the poll stops for
// good when the run ends, and a finished run kept in the cache is not asked for again.
import { Combobox, ConfirmationDialog, Dialog, DialogBusyStatus, useToast } from '@iota-uz/sdk/solid-ui'
import { createEffect, createMemo, createSignal, Match, on, onCleanup, onMount, Show, Switch, type JSX } from 'solid-js'
import { Canvas } from '../Canvas'
import { findItem, itemOutcomes } from '../graph'
import { Icon } from '../icons'
import { ensureLayout } from '../layout'
import { describeItem } from '../names'
import { Outline } from '../Outline'
import { strings } from '../strings'
import type { AutomationGraph, AutomationLayout, CanvasApi, EditorApi, EditorCatalog, EditorLocale, Overlay, RunData, RunWait, Subject } from '../types'
import { CopyText } from './Data'
import { isFinal, isLive, overlaysEqual, POLL_MS, runOverlay, sameRun } from './model'
import { NodeDrawer } from './NodeDrawer'
import { STATE_ICON } from './RunsList'
import { runStrings } from './strings'
import { absoluteTime, createAction, isNotFound, relativeTime, subjectName } from './util'

export type VersionGraph = { graph: AutomationGraph; layout: AutomationLayout }

export interface RunViewProps {
  runId: string
  api: EditorApi
  catalog: EditorCatalog
  locale: EditorLocale
  now: number
  /** Runs read in this editor: a finished one is shown from here and not polled again. */
  cache: Map<string, RunData>
  /** The version open in the editor. */
  version: number
  graphFor(version: number): Promise<VersionGraph>
  operate: boolean
  node: string | null
  onNode(id: string | null): void
  onOpenVersion?(version: number): void
  subjectHref?(subject: Subject): string | undefined
  onLoaded?(run: RunData): void
  onNotFound?(): void
  mobile: boolean
  onBack?(): void
}

type Conn = 'live' | 'reconnecting' | 'finished' | null

export function RunView(props: RunViewProps): JSX.Element {
  const t = () => runStrings(props.locale)
  const base = () => strings(props.locale)
  const toast = useToast()
  const cached = props.cache.get(props.runId)
  const [data, setData] = createSignal<RunData | null>(cached ?? null, { equals: sameRun })
  const [failed, setFailed] = createSignal(false)
  const [missing, setMissing] = createSignal(false)
  let disposed = false
  let timer: ReturnType<typeof setTimeout> | undefined
  let seq = 0
  onCleanup(() => {
    disposed = true
    clearTimeout(timer)
  })

  const schedule = () => {
    clearTimeout(timer)
    const d = data()
    if (disposed || !d || !isLive(d.state)) return
    timer = setTimeout(() => void fetchRun(), POLL_MS)
  }
  const fetchRun = async () => {
    if (!props.api.run) return
    const mine = ++seq
    try {
      const next = await props.api.run(props.runId)
      if (disposed || mine !== seq) return
      setData(next)
      props.cache.set(props.runId, next)
      setFailed(false)
      props.onLoaded?.(next)
    } catch (error) {
      if (disposed || mine !== seq) return
      if (isNotFound(error)) {
        setMissing(true)
        props.cache.delete(props.runId)
        props.onNotFound?.()
        return
      }
      console.warn('automation-editor: run failed to load', error)
      setFailed(true)
      // A live run keeps trying; the indicator says «Reconnecting…».
      if (data() && isLive(data()!.state)) {
        clearTimeout(timer)
        timer = setTimeout(() => void fetchRun(), POLL_MS)
      }
      return
    }
    schedule()
  }
  onMount(() => {
    // A finished run from the cache is final: no request, no poll (it does not resume on a tab switch).
    if (cached && isFinal(cached.state)) return
    void fetchRun()
  })

  // The graph of the run's version (not the draft): steps removed since are still drawn.
  const [view, setView] = createSignal<VersionGraph | null>(null)
  const [graphFailed, setGraphFailed] = createSignal(false)
  const loadGraph = (version: number) => {
    setGraphFailed(false)
    props.graphFor(version).then(
      (g) => { if (!disposed) setView({ graph: g.graph, layout: ensureLayout(g.graph, props.catalog, g.layout) }) },
      (error) => {
        console.warn('automation-editor: version failed to load', error)
        if (!disposed) setGraphFailed(true)
      },
    )
  }
  createEffect(on(() => data()?.version, (v) => { if (v !== undefined) loadGraph(v) }))

  const overlay = createMemo<Overlay | null>(() => {
    const d = data()
    const v = view()
    return d && v ? runOverlay(v.graph, d, props.locale, props.catalog) : null
  }, null, { equals: overlaysEqual })

  const conn = (): Conn => {
    const d = data()
    if (!d) return null
    if (isFinal(d.state)) return 'finished'
    if (!isLive(d.state)) return null
    return failed() ? 'reconnecting' : 'live'
  }

  // A node the run's graph does not have (a stale link) is dropped.
  createEffect(() => {
    const v = view()
    if (v && props.node && !findItem(v.graph, props.node)) props.onNode(null)
  })

  // ——— operate (RV10) ———
  const blocked = createMemo<RunWait | null>(() => {
    const d = data()
    if (!d || d.state !== 'blocked') return null
    return d.waits.find((w) => w.kind === 'call') ?? d.waits[0] ?? null
  })
  const canCancel = () => props.operate && Boolean(props.api.cancelRun) && Boolean(data()) && ['running', 'waiting', 'blocked'].includes(data()!.state)
  const canRetry = () => props.operate && Boolean(props.api.retryRun) && blocked() !== null
  const canResolve = () => props.operate && Boolean(props.api.resolveRun) && blocked() !== null
  const after = (title: string) => {
    toast.notify({ variant: 'success', title })
    seq += 1
    void fetchRun()
  }
  const opError = (error: unknown, what: string) => {
    const code = (error as { code?: string; data?: { code?: string } } | null)?.data?.code ?? (error as { code?: string } | null)?.code
    return code === 'run_finished' ? t().run.finishedAlready : t().run.actionFailed(what)
  }

  const [confirmCancel, setConfirmCancel] = createSignal(false)
  const [cancelError, setCancelError] = createSignal<string | null>(null)
  const cancel = createAction()
  const doCancel = () => {
    setCancelError(null)
    void cancel.run(() => props.api.cancelRun!(props.runId)).then(
      () => { setConfirmCancel(false); after(t().run.cancelled) },
      (error) => setCancelError(opError(error, t().run.what.cancel)),
    )
  }

  const retry = createAction()
  const [retryError, setRetryError] = createSignal<string | null>(null)
  const doRetry = () => {
    setRetryError(null)
    void retry.run(() => props.api.retryRun!({ run_id: props.runId, ...(blocked()?.wait_id ? { wait_id: blocked()!.wait_id } : {}) })).then(
      () => after(t().run.retried),
      (error) => setRetryError(opError(error, t().run.what.retry)),
    )
  }

  const [resolveOpen, setResolveOpen] = createSignal(false)
  const [outcome, setOutcome] = createSignal('')
  const [resolveError, setResolveError] = createSignal<string | null>(null)
  const resolve = createAction()
  const doResolve = () => {
    if (!outcome()) return
    setResolveError(null)
    void resolve.run(() => props.api.resolveRun!({ run_id: props.runId, outcome: outcome(), ...(blocked()?.wait_id ? { wait_id: blocked()!.wait_id } : {}) })).then(
      () => { setResolveOpen(false); after(t().run.resolved) },
      (error) => setResolveError(opError(error, t().run.what.resolve)),
    )
  }
  const outcomes = createMemo(() => {
    const v = view()
    const w = blocked()
    if (!v || !w) return []
    return itemOutcomes(v.graph, props.catalog, w.node).map((o) => {
      const edge = v.graph.edges.find((e) => e.from.node === w.node && e.from.outcome === o)
      const target = edge && edge.to !== 'ignore' && findItem(v.graph, edge.to) ? describeItem(v.graph, props.catalog, edge.to, props.locale).title : null
      return { value: o, label: target ? `${o} → ${target}` : o }
    })
  })

  let canvasApi: CanvasApi | null = null
  const nodeName = (id: string) => (view() && findItem(view()!.graph, id) ? describeItem(view()!.graph, props.catalog, id, props.locale).title : id)
  // On a phone the drawer is a sheet over the outline (K3): it takes the focus, keeps it inside, and gives
  // it back to the outline item that opened it.
  let sheet: HTMLDivElement | undefined
  let opener: HTMLElement | null = null
  const focusTitle = () => sheet?.querySelector<HTMLElement>('.ae-panel__title')?.focus({ preventScroll: true })
  // The sheet mounts once the run's graph is there (a link that names a node opens it after loading):
  // its ref takes the focus then; another node in the open sheet takes it again.
  const mountSheet = (el: HTMLDivElement) => {
    sheet = el
    if (props.mobile) queueMicrotask(focusTitle)
  }
  createEffect(on(() => props.node, (node, previous) => {
    if (!props.mobile || !node) return
    if (!previous) opener = document.activeElement instanceof HTMLElement && !sheet?.contains(document.activeElement) ? document.activeElement : null
    else queueMicrotask(focusTitle)
  }))
  const trapFocus = (event: KeyboardEvent) => {
    if (!props.mobile || event.key !== 'Tab' || !sheet) return
    const focusable = [...sheet.querySelectorAll<HTMLElement>('a[href], button:not([disabled]), input:not([disabled]), textarea, select, [tabindex]:not([tabindex="-1"]), summary')]
      .filter((el) => el.offsetParent !== null || el === document.activeElement)
    if (focusable.length === 0) return
    const at = focusable.indexOf(document.activeElement as HTMLElement)
    // From the title (focused by script, not in the Tab order) or outside: Tab goes to the first control,
    // Shift+Tab to the last; from the ends it wraps.
    if (event.shiftKey && at <= 0) {
      event.preventDefault()
      focusable[focusable.length - 1]!.focus()
    } else if (!event.shiftKey && (at === -1 || at === focusable.length - 1)) {
      event.preventDefault()
      focusable[0]!.focus()
    }
  }
  const closeNode = () => {
    const current = props.node
    props.onNode(null)
    if (props.mobile) {
      const back = opener ?? (current ? document.querySelector<HTMLElement>(`[data-pick="${CSS.escape(current)}"]`) : null)
      opener = null
      queueMicrotask(() => back?.focus())
      return
    }
    if (current) queueMicrotask(() => canvasApi?.focus(current))
  }

  const strip = (d: RunData) => {
    const subject = () => subjectName(d.subject)
    const href = () => (d.subject ? d.subject.url || props.subjectHref?.(d.subject) : undefined)
    return (
      <div class="ae-runstrip" data-testid="run-strip">
        <Show when={props.mobile && props.onBack}>
          <button type="button" class="ae-btn ae-btn--ghost ae-runstrip__back" onClick={() => props.onBack?.()}><Icon name="undo" />{t().run.back}</button>
        </Show>
        <span class={`ae-runstate ae-runstate--${d.state}`} data-testid="run-state">
          <Icon name={STATE_ICON[d.state]} size={14} />{t().list.states[d.state]}
        </span>
        <span class="ae-runstrip__subject" title={subject() ?? t().list.noSubject}>{subject() ?? t().list.noSubject}</span>
        <Show when={subject() && href()}>
          <a class="ae-link" href={href()} data-testid="open-subject">{t().run.openSubject(subject()!)}</a>
        </Show>
        <span class="ae-tag">v{d.version}</span>
        <span class="ae-muted ae-runstrip__time">
          <time datetime={d.created_at} title={absoluteTime(d.created_at, props.locale)}>{t().run.started(relativeTime(d.created_at, props.now, props.locale))}</time>
          <Show when={view() && findItem(view()!.graph, d.trigger)}> · {t().run.startedBy(nodeName(d.trigger))}</Show>
        </span>
        <span class="ae-runstrip__conn" role="status" aria-live="polite" data-testid="run-conn">
          <Show when={conn()}>
            {(c) => <span class={`ae-conn ae-conn--${c()}`}><span class="ae-conn__dot" aria-hidden="true" />{c() === 'live' ? t().run.live : c() === 'reconnecting' ? t().run.reconnecting : t().run.finished}</span>}
          </Show>
        </span>
        <span class="ae-runstrip__actions">
          <Show when={canRetry()}>
            <button type="button" class="ae-btn ae-btn--secondary" aria-busy={retry.busy()} onClick={doRetry} data-testid="run-retry">
              <Show when={retry.busy()} fallback={<Icon name="redo" />}><span class="ae-spinner" aria-hidden="true" /></Show>
              {retry.busy() ? t().run.retrying : retry.state().status === 'failed' ? base().common.retry : t().run.retry}
            </button>
          </Show>
          <Show when={canResolve()}>
            <button type="button" class="ae-btn ae-btn--secondary" onClick={() => { setOutcome(''); setResolveError(null); setResolveOpen(true) }} data-testid="run-resolve">
              <Icon name="check" />{t().run.resolve}
            </button>
          </Show>
          <Show when={canCancel()}>
            <button type="button" class="ae-btn ae-btn--ghost ae-btn--danger" onClick={() => { setCancelError(null); setConfirmCancel(true) }} data-testid="run-cancel">
              <Icon name="cancelled" />{t().run.cancel}
            </button>
          </Show>
        </span>
        <Show when={retry.state().status === 'busy' && (retry.state() as { slow: boolean }).slow}>
          <p class="ae-runstrip__line" role="status">{t().run.stillWaiting(t().run.what.retry)}</p>
        </Show>
        <Show when={retryError()}>{(e) => <p class="ae-runstrip__line ae-danger" role="alert">{e()}</p>}</Show>
        <Show when={d.state === 'failed' && d.error}>
          {(e) => <p class="ae-runstrip__line ae-danger" data-testid="run-error"><Icon name="error" size={14} />{e()}</p>}
        </Show>
      </div>
    )
  }

  return (
    <div class="ae-runview" classList={{ 'ae-runview--mobile': props.mobile }} data-testid="run-view">
      <Switch>
        <Match when={missing()}>
          <div class="ae-runlist__state" role="alert" data-testid="run-not-found">
            <p class="ae-runlist__title">{t().run.notFound}</p>
            <p class="ae-muted">{t().run.notFoundText}</p>
            <Show when={props.onBack}><button type="button" class="ae-btn ae-btn--secondary" onClick={() => props.onBack?.()}>{t().run.back}</button></Show>
          </div>
        </Match>
        <Match when={!data() && failed()}>
          <div class="ae-runlist__state" role="alert" data-testid="run-error-state">
            <p>{t().run.loadFailed}</p>
            <button type="button" class="ae-btn ae-btn--secondary" onClick={() => void fetchRun()}>{base().common.retry}</button>
          </div>
        </Match>
        <Match when={!data()}>
          <div class="ae-runstrip ae-runstrip--skeleton" role="status" aria-busy="true" aria-label={base().states.loading}>
            <span class="ae-skeleton__bar" style={{ width: '40%' }} />
          </div>
          <div class="ae-runview__canvas"><div class="ae-canvas ae-skeleton__canvas" /></div>
        </Match>
        <Match when={data()}>
          {(d) => (
            <>
              {strip(d())}
              <Show when={d().version !== props.version}>
                <div class="ae-banner" role="status" data-testid="run-version-banner">
                  <Icon name="dot" /><span>{t().run.onVersion(d().version)}</span>
                  <Show when={props.onOpenVersion}>
                    <button type="button" class="ae-btn ae-btn--secondary" onClick={() => props.onOpenVersion?.(d().version)} data-testid="open-run-version">{t().run.openVersion(d().version)}</button>
                  </Show>
                </div>
              </Show>
              <div class="ae-runview__body">
                <Switch>
                  <Match when={graphFailed()}>
                    <div class="ae-runlist__state" role="alert">
                      <p>{t().run.loadFailed}</p>
                      <button type="button" class="ae-btn ae-btn--secondary" onClick={() => loadGraph(d().version)}>{base().common.retry}</button>
                    </div>
                  </Match>
                  <Match when={!view()}>
                    <div class="ae-runview__canvas"><div class="ae-canvas ae-skeleton__canvas" role="status" aria-busy="true" aria-label={base().states.loading} /></div>
                  </Match>
                  <Match when={props.mobile}>
                    <div class="ae-mobile">
                      <Outline graph={view()!.graph} catalog={props.catalog} overlay={overlay()} locale={props.locale} onPick={(id) => props.onNode(id)} />
                    </div>
                  </Match>
                  <Match when={view()}>
                    {(v) => (
                      <div class="ae-runview__canvas" aria-label={t().run.canvas} role="region">
                        <Canvas
                          graph={v().graph}
                          catalog={props.catalog}
                          layout={v().layout}
                          overlay={overlay()}
                          selected={props.node}
                          onSelect={(id) => props.onNode(id)}
                          onOpen={(id) => props.onNode(id)}
                          onReady={(api) => { canvasApi = api }}
                          minimap
                          label={t().run.canvas}
                          locale={props.locale}
                        />
                      </div>
                    )}
                  </Match>
                </Switch>
                <Show when={props.node && view()}>
                  <div
                    ref={mountSheet}
                    class="ae-side"
                    classList={{ 'ae-side--sheet': props.mobile }}
                    {...(props.mobile ? { role: 'dialog', 'aria-modal': 'true', 'aria-labelledby': `ae-run-node-${props.node}` } : {})}
                    onKeyDown={(event) => {
                      trapFocus(event)
                      if (event.key === 'Escape' && !event.defaultPrevented) {
                        event.preventDefault()
                        closeNode()
                      }
                    }}
                  >
                    <NodeDrawer graph={view()!.graph} catalog={props.catalog} run={d()} node={props.node!} state={overlay()?.states.get(props.node!)} locale={props.locale} onClose={closeNode} sheet={props.mobile} />
                  </div>
                </Show>
              </div>
              <p class="ae-runview__id ae-muted">
                <span>{t().run.id}</span>
                <CopyText text={d().run_id} label={t().run.id} locale={props.locale} />
              </p>
            </>
          )}
        </Match>
      </Switch>
      <ConfirmationDialog
        open={confirmCancel()}
        onOpenChange={(open) => { if (!open && !cancel.busy()) setConfirmCancel(false) }}
        heading={t().run.cancelTitle}
        text={t().run.cancelText}
        confirmText={cancel.busy() ? t().run.cancelling : t().run.cancel}
        cancelText={t().run.keep}
        closeLabel={base().common.close}
        danger
        busy={cancel.busy()}
        initialFocus={() => document.querySelector<HTMLElement>('dialog[open] [data-test-id="dialog-cancel-btn"]') ?? undefined}
        onConfirm={doCancel}
        data-testid="cancel-dialog"
      >
        <Show when={cancel.state().status === 'busy' && (cancel.state() as { slow: boolean }).slow}>
          <p role="status">{t().run.stillWaiting(t().run.what.cancel)}</p>
        </Show>
        <Show when={cancelError()}>{(e) => <p class="ae-danger" role="alert">{e()}</p>}</Show>
      </ConfirmationDialog>
      <Show when={resolveOpen() && blocked()}>
        <Dialog open busy={resolve.busy()} onOpenChange={(open) => { if (!open) setResolveOpen(false) }} class="md:w-full md:max-w-md" aria-labelledby="ae-resolve-title" data-testid="resolve-dialog">
          <header class="border-b border-subtle py-3 pl-4 pr-2"><h3 id="ae-resolve-title" class="font-medium" data-dialog-title tabindex="-1">{t().run.resolveTitle(nodeName(blocked()!.node))}</h3></header>
          <div class="flex flex-col gap-3 px-4 py-3">
            <p>{t().run.resolveText}</p>
            <Combobox
              label={t().run.outcome}
              placeholder={t().run.pickOutcome}
              options={outcomes()}
              value={outcome()}
              onValueChange={(v) => setOutcome(typeof v === 'string' ? v : v[0] ?? '')}
              data-testid="resolve-outcome"
            />
            <Show when={resolve.state().status === 'busy' && (resolve.state() as { slow: boolean }).slow}>
              <p role="status">{t().run.stillWaiting(t().run.what.resolve)}</p>
            </Show>
            <Show when={resolveError()}>{(e) => <p class="ae-danger" role="alert">{e()}</p>}</Show>
          </div>
          <footer class="border-t border-subtle px-4 py-3">
            <DialogBusyStatus class="mb-2" />
            <menu class="flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
              <button type="button" class="btn btn-secondary btn-normal min-h-11 justify-center" data-dialog-close onClick={() => { if (!resolve.busy()) setResolveOpen(false) }}>{runStrings(props.locale).test.cancel}</button>
              <button type="button" class="btn btn-primary btn-normal min-h-11 justify-center" aria-disabled={!outcome() || undefined} aria-busy={resolve.busy()} onClick={doResolve} data-testid="resolve-confirm">
                <Show when={resolve.busy()}><span class="ae-spinner" aria-hidden="true" /></Show>
                {resolve.busy() ? t().run.resolving : resolveError() ? base().common.retry : t().run.resolve}
              </button>
            </menu>
          </footer>
        </Dialog>
      </Show>
    </div>
  )
}
