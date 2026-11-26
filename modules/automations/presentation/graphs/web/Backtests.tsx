import { For, Match, Show, Switch, createEffect, createMemo, createSignal, on, onCleanup, onMount, type JSX } from 'solid-js'
import type { AutomationGraph, EditorCatalog } from '@iota-uz/sdk/automation-editor'
import { Button, Dialog, DialogBusyStatus, EmptyState, Input, Select, Skeleton, Switch as Toggle, Textarea } from '@iota-uz/sdk/solid-ui'
import {
  active, backtestErrorText, backtestFailureText, classifiersOf, eventSummary, importErrorText, integer, nodeTitle, nodeTitles, percent,
  progress, readBacktestsView, usd, writeBacktestsView,
  type BacktestView, type BacktestsResult, type BacktestsView, type Classifier, type CompareResult, type DatasetView, type EstimateResult,
  type ExampleView, type ExamplesResult, type ReportResult, type ResultsResult, type SourceView,
} from './backtests'
import { useApp } from './context'
import { Icon } from './Icon'
import { absoluteTime, counted, errorCode, graphPath, inputFields, inputValue, relativeTime, type GetResult } from './model'

/** How often the section re-reads jobs in progress. */
const POLL_MS = 2000

interface Graph {
  id: string
  name: string
  graph: AutomationGraph
  catalog: EditorCatalog
  versions: GetResult['versions']
  latest: number
}

/**
 * The section «Backtests» of a graph (/automations/{id}/backtests, #1389): its datasets of events, the
 * backtests over them and a backtest's report. The canvas is not on this page; the editor links here.
 * What is open lives in the URL (D1): ?dataset=, ?backtest=, ?compare=, ?node=, ?mismatch=1.
 */
export default function BacktestsPage(): JSX.Element {
  const app = useApp()
  const t = app.t
  const id = app.props.id ?? ''
  const [view, setView] = createSignal<BacktestsView>(readBacktestsView(window.location.search))
  const [graph, setGraph] = createSignal<Graph>()
  const [data, setData] = createSignal<BacktestsResult>()
  const [loadedAt, setLoadedAt] = createSignal<Date>()
  const [error, setError] = createSignal<string>()
  const [dialog, setDialog] = createSignal<{ kind: 'create' } | { kind: 'import'; dataset: DatasetView } | { kind: 'synthetic'; dataset: DatasetView } | { kind: 'run'; dataset?: DatasetView } | { kind: 'delete'; dataset: DatasetView } | null>(null)
  const [highlighted, setHighlighted] = createSignal<string>()
  let seq = 0
  let timer: ReturnType<typeof setTimeout> | undefined
  let highlightTimer: ReturnType<typeof setTimeout> | undefined
  // After the page goes, an answer on the way neither updates it nor schedules another poll.
  let disposed = false
  onCleanup(() => { disposed = true; seq++; clearTimeout(timer); clearTimeout(highlightTimer) })

  const load = async () => {
    if (disposed) return
    const mine = ++seq
    clearTimeout(timer)
    setError(undefined)
    try {
      const [g, b] = await Promise.all([
        graph() ? Promise.resolve(undefined) : app.call('automations.get', { id }),
        app.call('automations.backtests', { automation_id: id }),
      ])
      if (mine !== seq) return
      if (g) setGraph({ id, name: g.automation.name, graph: g.graph as AutomationGraph, catalog: g.catalog as unknown as EditorCatalog, versions: g.versions, latest: g.automation.latest_version })
      setData(b)
      setLoadedAt(new Date())
      if (b.backtests.some((x) => active(x.status)) || b.datasets.some((d) => d.imports.some((j) => active(j.status)))) {
        timer = setTimeout(() => void load(), POLL_MS)
      }
    } catch (e) {
      if (mine !== seq) return
      setError(backtestErrorText(t, e, t('Backtests.Actions.Load')))
    }
  }
  onMount(() => {
    void load()
    const onPop = () => setView(readBacktestsView(window.location.search))
    const up = () => void load()
    window.addEventListener('popstate', onPop)
    window.addEventListener('online', up)
    onCleanup(() => { window.removeEventListener('popstate', onPop); window.removeEventListener('online', up) })
  })
  const go = (next: Partial<BacktestsView>, push = true) => {
    const v = { ...view(), ...next }
    setView(v)
    const href = writeBacktestsView(window.location.href, v)
    if (push) window.history.pushState({}, '', href)
    else window.history.replaceState({}, '', href)
  }
  const home = () => go({ dataset: null, backtest: null, compare: null, node: null, mismatch: false })
  const changed = (rowID: string) => {
    setHighlighted(rowID)
    clearTimeout(highlightTimer)
    highlightTimer = setTimeout(() => setHighlighted(undefined), 4000)
    void load()
  }
  const classifiers = createMemo(() => {
    const g = graph()
    return g ? classifiersOf(g.graph, g.catalog, data()?.decision_nodes ?? [], app.lang) : []
  })
  const titles = createMemo(() => {
    const g = graph()
    return g ? nodeTitles(g.graph, g.catalog, app.lang) : undefined
  })
  const dataset = () => data()?.datasets.find((d) => d.id === view().dataset)
  const backtest = () => data()?.backtests.find((b) => b.id === view().backtest)

  return (
    <div class="flex flex-col gap-4 p-4 md:p-6 min-w-0" data-testid="backtests-page">
      <header class="flex items-center justify-between gap-3 min-w-0">
        <div class="flex min-w-0 flex-col gap-0.5">
          <nav aria-label={t('Backtests.Breadcrumbs')} class="flex min-w-0 items-center gap-1 text-sm text-200">
            <a href={app.props.basePath} class="hover:underline shrink-0">{t('Title')}</a>
            <span aria-hidden="true">/</span>
            <a href={graphPath(app.props.basePath, id)} class="hover:underline truncate" data-testid="backtests-graph">{graph()?.name || t('List.Untitled')}</a>
          </nav>
          <h1 class="text-xl font-semibold truncate">{t('Backtests.Title')}</h1>
        </div>
        <Show when={data() && data()!.datasets.length > 0}>
          <div class="flex shrink-0 gap-2">
            <Button variant="secondary" class="hidden sm:inline-flex" onClick={() => setDialog({ kind: 'run' })} icon={<Icon name="play" />} data-testid="backtest-new">{t('Backtests.Run.New')}</Button>
            <Button variant="primary" onClick={() => setDialog({ kind: 'create' })} icon={<Icon name="plus" />} data-testid="dataset-new">{t('Backtests.Datasets.New')}</Button>
          </div>
        </Show>
      </header>
      <Show when={error() && data()}>
        <p role="alert" class="flex flex-wrap items-center gap-2 text-sm text-300" data-testid="backtests-stale">
          <Icon name="alert" class="text-yellow" />{t('List.Stale', { time: loadedAt() ? absoluteTime(loadedAt()!.toISOString(), app.lang) : '' })}
          <Button variant="ghost" size="sm" onClick={() => void load()}>{t('Common.Retry')}</Button>
        </p>
      </Show>
      <Switch>
        <Match when={!data() && error()}>
          <div role="alert" class="flex flex-col items-start gap-3 rounded-lg border border-subtle p-6" data-testid="backtests-error">
            <p>{error()}</p>
            <Button variant="secondary" onClick={() => void load()} icon={<Icon name="refresh" />}>{t('Common.Retry')}</Button>
          </div>
        </Match>
        <Match when={!data()}>
          <div role="status" aria-busy="true" aria-label={t('Common.Loading')} class="flex flex-col gap-4" data-testid="backtests-loading">
            <Skeleton variant="table" lines={4} />
            <Skeleton variant="table" lines={3} />
          </div>
        </Match>
        <Match when={view().backtest}>
          <ReportPanel
            backtestID={view().backtest!} compare={view().compare} node={view().node} mismatch={view().mismatch}
            summary={backtest()} backtests={data()!.backtests} classifiers={classifiers()} titles={titles()}
            onView={(v) => go(v, false)} onBack={home} onChanged={() => void load()}
          />
        </Match>
        <Match when={view().dataset}>
          <Show when={dataset()} fallback={<NotFound onBack={home} />}>
            {(d) => (
              <DatasetPanel
                dataset={d()} classifiers={classifiers()} titles={titles()}
                onBack={home} onImport={() => setDialog({ kind: 'import', dataset: d() })} onSynthetic={() => setDialog({ kind: 'synthetic', dataset: d() })}
                onRun={() => setDialog({ kind: 'run', dataset: d() })} onDelete={() => setDialog({ kind: 'delete', dataset: d() })} onChanged={() => void load()}
              />
            )}
          </Show>
        </Match>
        <Match when={data()!.datasets.length === 0}>
          <EmptyState
            data-testid="backtests-empty"
            title={t('Backtests.Empty.Title')}
            description={
              <span class="flex flex-col items-center gap-3">
                <span>{t('Backtests.Empty.Text')}</span>
                <Button variant="primary" onClick={() => setDialog({ kind: 'create' })} icon={<Icon name="plus" />} data-testid="dataset-first">{t('Backtests.Empty.Action')}</Button>
              </span>
            }
          />
        </Match>
        <Match when={data()}>
          <Overview
            data={data()!} highlighted={highlighted()}
            onDataset={(ds) => go({ dataset: ds, backtest: null })} onBacktest={(b) => go({ backtest: b, dataset: null })}
            onRun={(d) => setDialog({ kind: 'run', dataset: d })}
          />
        </Match>
      </Switch>
      <Show when={dialog()}>
        {(dlg) => (
          <Switch>
            <Match when={dlg().kind === 'create' || dlg().kind === 'import'}>
              <ImportDialog
                automationID={id} dataset={(dlg() as { dataset?: DatasetView }).dataset} sources={data()?.sources ?? []} onCreated={() => void load()}
                onDone={(ds) => { setDialog(null); changed(ds) }} onClose={() => setDialog(null)}
              />
            </Match>
            <Match when={dlg().kind === 'synthetic'}>
              <SyntheticDialog
                dataset={(dlg() as { dataset: DatasetView }).dataset} graph={graph()} classifiers={classifiers()}
                onDone={(ds) => { setDialog(null); changed(ds) }} onClose={() => setDialog(null)}
              />
            </Match>
            <Match when={dlg().kind === 'run'}>
              <RunBacktestDialog
                datasets={data()?.datasets ?? []} dataset={(dlg() as { dataset?: DatasetView }).dataset} graph={graph()} live={data()?.live ?? false}
                onDone={(b) => { setDialog(null); void load(); go({ backtest: b, dataset: null }) }} onClose={() => setDialog(null)}
              />
            </Match>
            <Match when={dlg().kind === 'delete'}>
              <DeleteDatasetDialog
                dataset={(dlg() as { dataset: DatasetView }).dataset}
                onDone={() => { setDialog(null); home(); void load() }} onClose={() => setDialog(null)}
              />
            </Match>
          </Switch>
        )}
      </Show>
    </div>
  )
}

function NotFound(props: { onBack: () => void }): JSX.Element {
  const t = useApp().t
  return (
    <div role="alert" class="flex flex-col items-start gap-2 rounded-lg border border-subtle p-6" data-testid="backtests-not-found">
      <h2 class="text-lg font-medium">{t('Backtests.NotFound')}</h2>
      <Button variant="secondary" onClick={props.onBack}>{t('Backtests.Back')}</Button>
    </div>
  )
}

/** An example as a table names it; one the reader may not read says so. */
function exampleText(t: ReturnType<typeof useApp>['t'], ex: ExampleView): string {
  if (ex.hidden) return t('Backtests.Examples.Hidden')
  return eventSummary(ex) || t('Backtests.Examples.Untitled')
}

function BackLink(props: { onBack: () => void }): JSX.Element {
  const t = useApp().t
  return (
    <button type="button" class="inline-flex min-h-9 [@media(pointer:coarse)]:min-h-11 items-center gap-1 self-start text-sm text-200 hover:underline focus-visible:outline-2 focus-visible:outline-primary-500" onClick={props.onBack} data-testid="backtests-back">
      <Icon name="arrow" class="rotate-180" />{t('Backtests.Back')}
    </button>
  )
}

/** A job's status as a line with an icon (F7, A5): never a colored fill. */
function StatusLine(props: { status: BacktestView['status']; text?: string }): JSX.Element {
  const t = useApp().t
  const icon = () => (props.status === 'done' ? 'check' : props.status === 'failed' ? 'alert' : props.status === 'cancelled' ? 'x' : 'clock')
  const tone = () => (props.status === 'done' ? 'text-green' : props.status === 'failed' ? 'text-red-600' : props.status === 'cancelled' ? 'text-300' : 'text-yellow')
  return (
    <span class="inline-flex items-center gap-1.5 whitespace-nowrap" data-testid="job-status">
      <Icon name={icon()} class={tone()} />{props.text ?? t(`Backtests.Status.${props.status}`)}
    </span>
  )
}

/** A refresh that failed over data already on screen: the data stays, marked stale, with a retry (S4). */
function StaleLine(props: { message: string; onRetry: () => void; testid: string }): JSX.Element {
  const t = useApp().t
  return (
    <p role="alert" class="flex flex-wrap items-center gap-2 text-sm text-200" data-testid={props.testid}>
      <Icon name="alert" class="text-yellow" />{t('Backtests.Stale', { reason: props.message })}
      <Button variant="ghost" size="sm" onClick={props.onRetry}>{t('Common.Retry')}</Button>
    </p>
  )
}

function ProgressBar(props: { value: number; label: string }): JSX.Element {
  const pct = () => Math.round(props.value * 100)
  return (
    <div class="h-2 w-full overflow-hidden rounded-full bg-surface-400" role="progressbar" aria-label={props.label} aria-valuemin={0} aria-valuemax={100} aria-valuenow={pct()}>
      <div class="h-full rounded-full bg-brand motion-safe:transition-[width] motion-safe:duration-200" style={{ width: `${pct()}%` }} />
    </div>
  )
}

function backtestProgressText(t: ReturnType<typeof useApp>['t'], b: BacktestView, lang: string): string {
  if (b.status === 'queued') return t('Backtests.Run.Queued')
  return t('Backtests.Run.Progress', { done: integer(b.done, lang), total: integer(b.total, lang) })
}

function Overview(props: { data: BacktestsResult; highlighted?: string; onDataset: (id: string) => void; onBacktest: (id: string) => void; onRun: (d: DatasetView) => void }): JSX.Element {
  const app = useApp()
  const t = app.t
  const lastImport = (d: DatasetView) => d.imports[0]
  return (
    <div class="flex flex-col gap-6">
      <section aria-labelledby="datasets-title" class="flex flex-col gap-3">
        <h2 id="datasets-title" class="text-base font-medium">{t('Backtests.Datasets.Title')}</h2>
        <ul class="grid gap-3 md:grid-cols-2 xl:grid-cols-3" data-testid="datasets">
          <For each={props.data.datasets}>
            {(d) => (
              <li class="flex flex-col gap-3 rounded-lg border border-subtle bg-surface-600 p-4 min-w-0 transition-colors" classList={{ 'bg-brand/5': props.highlighted === d.id }} data-testid={`dataset-${d.id}`}>
                <div class="flex min-w-0 flex-col gap-1">
                  <button type="button" class="self-start text-left font-medium text-100 hover:underline break-words focus-visible:outline-2 focus-visible:outline-primary-500" onClick={() => props.onDataset(d.id)} data-testid="dataset-open">{d.name}</button>
                  <p class="text-sm text-200 tabular-nums">
                    {counted(t, 'Backtests.Datasets.Examples', d.examples, app.lang)} · {t('Backtests.Datasets.Labeled', { count: integer(d.labeled, app.lang) })}
                  </p>
                </div>
                <Show when={lastImport(d)}>
                  {(j) => <ImportLine job={j()} />}
                </Show>
                <div class="mt-auto flex flex-wrap gap-2 border-t border-subtle pt-3">
                  <Button variant="secondary" size="sm" onClick={() => props.onDataset(d.id)}>{t('Backtests.Datasets.Open')}</Button>
                  <Show when={d.examples > 0}>
                    <Button variant="ghost" size="sm" icon={<Icon name="play" />} onClick={() => props.onRun(d)} data-testid="dataset-run">{t('Backtests.Run.Start')}</Button>
                  </Show>
                </div>
              </li>
            )}
          </For>
        </ul>
      </section>
      <section aria-labelledby="runs-title" class="flex flex-col gap-3">
        <h2 id="runs-title" class="text-base font-medium">{t('Backtests.Runs.Title')}</h2>
        <Show when={props.data.backtests.length > 0} fallback={<p class="rounded-lg border border-dashed border-subtle p-4 text-sm text-200" data-testid="backtests-none">{t('Backtests.Runs.Empty')}</p>}>
          <ul class="flex flex-col divide-y divide-subtle rounded-lg border border-subtle bg-surface-600" data-testid="backtests">
            <For each={props.data.backtests}>
              {(b) => (
                <li class="flex flex-col gap-2 p-4 md:flex-row md:items-center md:gap-4 min-w-0" classList={{ 'bg-brand/5': props.highlighted === b.id }} data-testid={`backtest-${b.id}`}>
                  <div class="flex min-w-0 flex-1 flex-col gap-0.5">
                    <button type="button" class="self-start text-left font-medium hover:underline break-words focus-visible:outline-2 focus-visible:outline-primary-500" onClick={() => props.onBacktest(b.id)} data-testid="backtest-open">
                      {t('Backtests.Runs.Name', { version: b.version, dataset: b.dataset_name })}
                    </button>
                    <span class="flex flex-wrap items-center gap-x-2 text-xs text-300">
                      <span>{b.decisions === 'live' ? t('Backtests.Decisions.Live') : t('Backtests.Decisions.Cached')}</span>
                      <span aria-hidden="true">·</span>
                      <span class="inline-flex items-center gap-1">
                        <Show when={b.by_agent}><Icon name="robot" size={12} /></Show>{b.created_by}
                      </span>
                      <span aria-hidden="true">·</span>
                      <time class="tabular-nums" datetime={b.created_at} title={absoluteTime(b.created_at, app.lang)}>{relativeTime(b.created_at, app.now(), app.lang)}</time>
                    </span>
                  </div>
                  <div class="flex flex-col gap-1 md:w-64">
                    <StatusLine status={b.status} text={active(b.status) ? backtestProgressText(t, b, app.lang) : undefined} />
                    <Show when={active(b.status)}><ProgressBar value={progress(b)} label={t('Backtests.Run.ProgressLabel')} /></Show>
                    <Show when={b.status === 'failed'}><span class="text-xs text-200">{backtestFailureText(t, b.error)}</span></Show>
                  </div>
                </li>
              )}
            </For>
          </ul>
        </Show>
      </section>
    </div>
  )
}

function ImportLine(props: { job: DatasetView['imports'][number] }): JSX.Element {
  const app = useApp()
  const t = app.t
  const j = () => props.job
  const text = () => {
    const x = j()
    switch (x.status) {
      case 'queued': case 'running': return t('Backtests.Import.Running', { count: integer(x.imported, app.lang) })
      case 'failed': return importErrorText(t, x.error)
      default: return t('Backtests.Import.Done', { count: integer(x.imported, app.lang) })
    }
  }
  return (
    <div class="flex flex-col gap-0.5 text-sm" role={active(j().status) ? 'status' : undefined} data-testid="import-line">
      <StatusLine status={j().status} text={text()} />
      <Show when={j().status === 'done' && (j().duplicates > 0 || j().overflow > 0 || j().unreadable > 0)}>
        <span class="text-xs text-200" data-testid="import-left-out">
          <Show when={j().overflow > 0}>{t('Backtests.Import.Overflow', { count: integer(j().overflow, app.lang) })} </Show>
          <Show when={j().duplicates > 0}>{t('Backtests.Import.Duplicates', { count: integer(j().duplicates, app.lang) })} </Show>
          <Show when={j().unreadable > 0}>{t('Backtests.Import.Unreadable', { count: integer(j().unreadable, app.lang) })}</Show>
        </span>
      </Show>
    </div>
  )
}

const SOURCE_ICON: Record<string, 'clock' | 'plug' | 'hand'> = { journal: 'clock', history: 'plug', synthetic: 'hand' }

/** A dataset: its examples and their labels; a label is a choice per classifier, saved at once (F8). */
function DatasetPanel(props: {
  dataset: DatasetView; classifiers: Classifier[]; titles?: Map<string, string>
  onBack: () => void; onImport: () => void; onSynthetic: () => void; onRun: () => void; onDelete: () => void; onChanged: () => void
}): JSX.Element {
  const app = useApp()
  const t = app.t
  const [page, setPage] = createSignal<ExamplesResult>()
  const [error, setError] = createSignal<string>()
  const [more, setMore] = createSignal(false)
  // seq names the read in flight; committed the one whose first page is on screen. A continuation
  // appends only to the page it continues.
  let seq = 0
  let committed = 0
  onCleanup(() => { seq++ })
  // Saves of one example's node run one after another, in the order of the edits, so the server commits
  // them in that order. edits: the latest edit of each node; latest: the label it shows; pending: the
  // node's saves not answered yet; confirmed: the label the server holds, while saves are pending (with
  // none pending, the label on screen is the server's: the examples read last). It is the last of: the
  // label a save of this panel committed, and the label examples read after that commit hold (another
  // user's change). A failed save rolls back to the confirmed label, never to another unconfirmed edit,
  // and only while no later edit of that node took its place. Edits of different nodes stay independent.
  type Label = ExampleView['labels'][string] | undefined
  const edits = new Map<string, number>()
  const latest = new Map<string, Label>()
  const pending = new Map<string, number>()
  const confirmed = new Map<string, Label>()
  const queue = new Map<string, Promise<void>>()
  let editSeq = 0
  // saved counts the saves of this panel that committed. A read of examples that began before one of
  // them may predate it: it is not shown, the examples are read again. So every read shown is newer
  // than every commit of this panel.
  let saved = 0
  // Examples read while saves are pending keep the latest edit of those nodes on screen; the label they
  // read is the server's, the rollback of those saves.
  const overlay = (items: ExampleView[]): ExampleView[] => pending.size === 0 ? items : items.map((x) => {
    let labels: ExampleView['labels'] | undefined
    for (const [key, label] of latest) {
      const [id, node] = [key.slice(0, key.indexOf('/')), key.slice(key.indexOf('/') + 1)]
      if (id !== x.id || !pending.get(key)) continue
      confirmed.set(key, x.labels[node])
      labels ??= { ...x.labels }
      if (label) labels[node] = label
      else delete labels[node]
    }
    return labels ? { ...x, labels } : x
  })
  const load = async () => {
    const mine = ++seq
    const readAt = saved
    setError(undefined)
    try {
      const res = await app.call('automations.dataset_examples', { dataset_id: props.dataset.id, limit: 50 })
      if (mine !== seq) return
      if (saved !== readAt) { void load(); return }
      setPage({ ...res, items: overlay(res.items) })
      committed = mine
    } catch (e) {
      if (mine === seq) setError(backtestErrorText(t, e, t('Backtests.Actions.LoadExamples')))
    }
  }
  // The examples are read again when the dataset's count moves (an import brought more).
  createEffect(on(() => [props.dataset.id, props.dataset.examples, props.dataset.labeled] as const, () => void load()))
  const loadMore = async () => {
    const p = page()
    const of = committed
    if (!p?.next_cursor || more() || of !== seq) return
    const readAt = saved
    setMore(true)
    try {
      const res = await app.call('automations.dataset_examples', { dataset_id: props.dataset.id, cursor: p.next_cursor, limit: 50 })
      if (of !== seq || page() !== p) return
      // A continuation that may predate a save reads the examples again from the start.
      if (saved !== readAt) { void load(); return }
      setPage({ ...res, items: [...p.items, ...overlay(res.items)] })
    } catch (e) {
      app.toast.notify({ variant: 'error', title: backtestErrorText(t, e, t('Backtests.Actions.LoadExamples')), action: { label: t('Common.Retry'), onAction: () => void loadMore() } })
    } finally {
      setMore(false)
    }
  }
  const setLabel = (ex: ExampleView, node: string, outcome: string) => {
    const current = page()?.items.find((x) => x.id === ex.id) ?? ex
    const before = current.labels[node]
    const patchNode = (label: ExampleView['labels'][string] | undefined) => setPage((p) => p && {
      ...p,
      items: p.items.map((x) => {
        if (x.id !== ex.id) return x
        const labels = { ...x.labels }
        if (label) labels[node] = label
        else delete labels[node]
        return { ...x, labels }
      }),
    })
    const key = `${ex.id}/${node}`
    if (!pending.get(key)) confirmed.set(key, before)
    const mine = ++editSeq
    edits.set(key, mine)
    pending.set(key, (pending.get(key) ?? 0) + 1)
    const applied: Label = outcome
      ? { outcome, source: 'manual', ...(before?.source === 'expected' ? { expected: before.outcome } : before?.expected ? { expected: before.expected } : {}) }
      : before?.expected ? { outcome: before.expected, source: 'expected' } : undefined
    latest.set(key, applied)
    patchNode(applied)
    const save = async () => {
      try {
        await app.call('automations.example_label', { example_id: ex.id, node, outcome })
        confirmed.set(key, applied)
        saved++
        // The dataset's labeled count is the overview's.
        props.onChanged()
      } catch (e) {
        if (edits.get(key) === mine) { latest.set(key, confirmed.get(key)); patchNode(confirmed.get(key)) }
        // Retry repeats this edit only while it is still the node's latest: a later edit stands.
        app.toast.notify({ variant: 'error', title: backtestErrorText(t, e, t('Backtests.Actions.Label')), action: { label: t('Common.Retry'), onAction: () => { if (edits.get(key) === mine) void setLabel(ex, node, outcome) } } })
      } finally {
        const left = (pending.get(key) ?? 1) - 1
        if (left > 0) pending.set(key, left)
        else { pending.delete(key); latest.delete(key) }
      }
    }
    const next = (queue.get(key) ?? Promise.resolve()).then(save)
    queue.set(key, next)
    void next.then(() => {
      if (queue.get(key) === next) queue.delete(key)
    })
    return next
  }
  const labelCell = (ex: ExampleView, c: Classifier) => {
    const l = () => ex.labels[c.id]
    const title = () => nodeTitle(props.titles, c.id)
    return (
      <div class="flex min-w-36 flex-col gap-0.5">
        <Select
          aria-label={t('Backtests.Labels.Of', { node: title(), example: exampleText(t, ex) })}
          value={l()?.source === 'manual' ? l()!.outcome : ''}
          options={[{ value: '', label: l()?.source === 'expected' ? t('Backtests.Labels.Expected', { outcome: l()!.outcome }) : t('Backtests.Labels.None') }, ...c.outcomes.map((o) => ({ value: o, label: o }))]}
          onChange={(e) => void setLabel(ex, c.id, e.currentTarget.value)}
          data-testid={`label-${c.id}`}
        />
        <Show when={l()?.source === 'manual' && l()?.expected}>
          <span class="text-xs text-300">{t('Backtests.Labels.Overrides', { outcome: l()!.expected! })}</span>
        </Show>
      </div>
    )
  }
  const meta = (ex: ExampleView) => (
    <span class="flex flex-wrap items-center gap-x-2 text-xs text-300">
      <span class="inline-flex items-center gap-1"><Icon name={SOURCE_ICON[ex.source] ?? 'dot'} size={12} />{t(`Backtests.Source.${ex.source}`)}</span>
      <span aria-hidden="true">·</span>
      <time class="tabular-nums" datetime={ex.occurred_at} title={absoluteTime(ex.occurred_at, app.lang)}>{absoluteTime(ex.occurred_at, app.lang)}</time>
      <Show when={ex.snapshot_approximate}>
        <span aria-hidden="true">·</span>
        <span class="inline-flex items-center gap-1" data-testid="approximate"><Icon name="alert" size={12} class="text-yellow" />{t('Backtests.Examples.Approximate')}</span>
      </Show>
    </span>
  )
  return (
    <div class="flex flex-col gap-4" data-testid="dataset-panel">
      <BackLink onBack={props.onBack} />
      <div class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div class="min-w-0">
          <h2 class="text-lg font-medium break-words" data-testid="dataset-name">{props.dataset.name}</h2>
          <p class="text-sm text-200 tabular-nums">
            {counted(t, 'Backtests.Datasets.Examples', props.dataset.examples, app.lang)} · {t('Backtests.Datasets.Labeled', { count: integer(props.dataset.labeled, app.lang) })}
          </p>
          <Show when={props.dataset.imports[0]}>{(j) => <div class="mt-2"><ImportLine job={j()} /></div>}</Show>
        </div>
        <div class="flex flex-wrap gap-2">
          <Button variant="secondary" icon={<Icon name="refresh" />} onClick={props.onImport} data-testid="dataset-import">{t('Backtests.Import.Action')}</Button>
          <Button variant="secondary" icon={<Icon name="plus" />} onClick={props.onSynthetic} data-testid="dataset-synthetic">{t('Backtests.Synthetic.Action')}</Button>
          <Show when={props.dataset.examples > 0}>
            <Button variant="primary" icon={<Icon name="play" />} onClick={props.onRun} data-testid="dataset-run">{t('Backtests.Run.Start')}</Button>
          </Show>
          <button type="button" class="ae-iconbtn" aria-label={t('Backtests.Datasets.Delete')} title={t('Backtests.Datasets.Delete')} onClick={props.onDelete} data-testid="dataset-delete"><Icon name="trash" /></button>
        </div>
      </div>
      <Show when={page() && error()}>
        <StaleLine message={error()!} onRetry={() => void load()} testid="examples-stale" />
      </Show>
      <Switch>
        <Match when={!page() && error()}>
          <div role="alert" class="flex flex-col items-start gap-3 rounded-lg border border-subtle p-6">
            <p>{error()}</p>
            <Button variant="secondary" onClick={() => void load()}>{t('Common.Retry')}</Button>
          </div>
        </Match>
        <Match when={!page()}>
          <div role="status" aria-busy="true" aria-label={t('Common.Loading')}><Skeleton variant="table" lines={5} /></div>
        </Match>
        <Match when={page()!.items.length === 0}>
          <EmptyState
            data-testid="examples-empty"
            title={t('Backtests.Examples.EmptyTitle')}
            description={
              <span class="flex flex-col items-center gap-3">
                <span>{t('Backtests.Examples.EmptyText')}</span>
                <Button variant="primary" icon={<Icon name="refresh" />} onClick={props.onImport}>{t('Backtests.Import.Action')}</Button>
              </span>
            }
          />
        </Match>
        <Match when={page()}>
          <Show when={props.classifiers.length > 0} fallback={<p class="text-sm text-200" data-testid="no-classifiers">{t('Backtests.Labels.NoClassifiers')}</p>}>
            <p class="text-sm text-200">{t('Backtests.Labels.Hint')}</p>
          </Show>
          <div class="hidden md:block overflow-x-auto rounded-lg border border-subtle bg-surface-600">
            <table class="w-full text-sm" data-testid="examples-table">
              <thead class="bg-surface-500 text-left text-xs text-200">
                <tr>
                  <th scope="col" class="px-4 py-2 font-medium">{t('Backtests.Examples.Event')}</th>
                  <For each={props.classifiers}>{(c) => <th scope="col" class="px-4 py-2 font-medium">{c.title}</th>}</For>
                </tr>
              </thead>
              <tbody>
                <For each={page()!.items}>
                  {(ex) => (
                    <tr class="border-t border-subtle align-top" data-testid={`example-${ex.id}`}>
                      <td class="px-4 py-3 max-w-md">
                        <p class="break-words">{exampleText(t, ex)}</p>
                        {meta(ex)}
                      </td>
                      <For each={props.classifiers}>{(c) => <td class="px-4 py-3">{labelCell(ex, c)}</td>}</For>
                    </tr>
                  )}
                </For>
              </tbody>
            </table>
          </div>
          <ul class="flex flex-col gap-3 md:hidden" data-testid="examples-cards">
            <For each={page()!.items}>
              {(ex) => (
                <li class="flex flex-col gap-3 rounded-lg border border-subtle bg-surface-600 p-4 min-w-0">
                  <div class="min-w-0">
                    <p class="break-words">{exampleText(t, ex)}</p>
                    {meta(ex)}
                  </div>
                  <For each={props.classifiers}>
                    {(c) => <div class="flex flex-col gap-1"><span class="text-xs text-200">{c.title}</span>{labelCell(ex, c)}</div>}
                  </For>
                </li>
              )}
            </For>
          </ul>
          <Show when={page()!.next_cursor}>
            <div class="flex justify-center"><Button variant="secondary" loading={more()} onClick={() => void loadMore()}>{t('List.LoadMore')}</Button></div>
          </Show>
        </Match>
      </Switch>
    </div>
  )
}

/** A backtest: its progress while it runs, then its report and the cases table; compared with another. */
function ReportPanel(props: {
  backtestID: string; compare: string | null; node: string | null; mismatch: boolean
  summary?: BacktestView; backtests: BacktestView[]; classifiers: Classifier[]; titles?: Map<string, string>
  onView: (v: Partial<BacktestsView>) => void; onBack: () => void; onChanged: () => void
}): JSX.Element {
  const app = useApp()
  const t = app.t
  const [report, setReport] = createSignal<ReportResult>()
  const [error, setError] = createSignal<string>()
  const [cancelling, setCancelling] = createSignal(false)
  let seq = 0
  let timer: ReturnType<typeof setTimeout> | undefined
  // Leaving the report drops the answer on the way: it neither shows nor polls again.
  let disposed = false
  onCleanup(() => { disposed = true; seq++; clearTimeout(timer) })
  const load = async () => {
    if (disposed) return
    const mine = ++seq
    clearTimeout(timer)
    setError(undefined)
    try {
      const r = await app.call('automations.backtest_report', { backtest_id: props.backtestID })
      if (mine !== seq || disposed) return
      setReport(r)
      if (active(r.backtest.status)) timer = setTimeout(() => void load(), POLL_MS)
    } catch (e) {
      if (mine === seq) setError(backtestErrorText(t, e, t('Backtests.Actions.LoadReport')))
    }
  }
  createEffect(on(() => props.backtestID, () => { setReport(undefined); void load() }))
  // The list's poll brings the end of the backtest: the report is read again once.
  createEffect(on(() => props.summary?.status, (s, prev) => { if (prev && s !== prev) void load() }, { defer: true }))
  const cancel = async () => {
    if (cancelling()) return
    setCancelling(true)
    try {
      await app.call('automations.backtest_cancel', { backtest_id: props.backtestID })
      app.toast.notify({ variant: 'success', title: t('Backtests.Run.Cancelled') })
      props.onChanged()
      void load()
    } catch (e) {
      app.toast.notify({ variant: 'error', title: backtestErrorText(t, e, t('Backtests.Actions.Cancel')) })
    } finally {
      setCancelling(false)
    }
  }
  const title = (node: string) => nodeTitle(props.titles, node)
  const others = () => props.backtests.filter((b) => b.id !== props.backtestID && b.status === 'done')
  return (
    <div class="flex flex-col gap-5" data-testid="report-panel">
      <BackLink onBack={props.onBack} />
      <Show when={report() && error()}>
        <StaleLine message={error()!} onRetry={() => void load()} testid="report-stale" />
      </Show>
      <Switch>
        <Match when={!report() && error()}>
          <div role="alert" class="flex flex-col items-start gap-3 rounded-lg border border-subtle p-6" data-testid="report-error">
            <p>{error()}</p>
            <Button variant="secondary" onClick={() => void load()}>{t('Common.Retry')}</Button>
          </div>
        </Match>
        <Match when={!report()}>
          <div role="status" aria-busy="true" aria-label={t('Common.Loading')} class="flex flex-col gap-4" data-testid="report-loading">
            <Skeleton lines={2} /><Skeleton variant="table" lines={4} />
          </div>
        </Match>
        <Match when={report()}>
          {(r) => (
            <>
              <div class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
                <div class="min-w-0">
                  <h2 class="text-lg font-medium break-words" data-testid="report-title">{t('Backtests.Runs.Name', { version: r().backtest.version, dataset: r().backtest.dataset_name })}</h2>
                  <p class="flex flex-wrap items-center gap-x-2 text-sm text-200">
                    <span>{r().backtest.decisions === 'live' ? t('Backtests.Decisions.Live') : t('Backtests.Decisions.Cached')}</span>
                    <span aria-hidden="true">·</span>
                    <time class="tabular-nums" datetime={r().backtest.created_at}>{absoluteTime(r().backtest.created_at, app.lang)}</time>
                  </p>
                </div>
                <Show when={active(r().backtest.status)}>
                  <Button variant="secondary" loading={cancelling()} onClick={() => void cancel()} icon={<Icon name="x" />} data-testid="report-cancel">{t('Backtests.Run.Cancel')}</Button>
                </Show>
              </div>
              <div class="flex flex-col gap-2 border-t border-subtle pt-3" role="status" aria-live="polite" data-testid="report-status">
                <StatusLine status={r().backtest.status} text={active(r().backtest.status) ? backtestProgressText(t, r().backtest, app.lang) : r().backtest.status === 'failed' ? backtestFailureText(t, r().backtest.error) : undefined} />
                <Show when={active(r().backtest.status)}>
                  <ProgressBar value={progress(r().backtest)} label={t('Backtests.Run.ProgressLabel')} />
                  <p class="text-xs text-200">{t('Backtests.Run.Background')}</p>
                </Show>
              </div>
              <Show when={r().examples > 0}>
                <Summary report={r()} />
                <Show when={r().snapshots_withheld > 0}>
                  <p class="flex items-start gap-2 rounded-lg border border-subtle bg-surface-600 p-3 text-sm" data-testid="report-snapshots-withheld">
                    <Icon name="alert" class="mt-0.5 shrink-0 text-yellow" />
                    {t('Backtests.Report.SnapshotsWithheld', { count: integer(r().snapshots_withheld, app.lang), total: integer(r().examples, app.lang) })}
                  </p>
                </Show>
                <Metrics report={r()} title={title} />
                <Cases backtestID={props.backtestID} classifiers={props.classifiers} report={r()} node={props.node} mismatch={props.mismatch} title={title} onView={props.onView} />
                <Effects report={r()} title={title} />
                <Show when={r().backtest.status === 'done'}>
                  <Compare backtestID={props.backtestID} compare={props.compare} others={others()} title={title} onView={props.onView} />
                </Show>
              </Show>
            </>
          )}
        </Match>
      </Switch>
    </div>
  )
}

function Summary(props: { report: ReportResult }): JSX.Element {
  const app = useApp()
  const t = app.t
  const r = () => props.report
  const cost = () => r().cost
  const item = (label: string, value: string, testid?: string) => (
    <div class="flex flex-col gap-0.5 rounded-lg border border-subtle bg-surface-600 p-3" data-testid={testid}>
      <dt class="text-xs text-200">{label}</dt>
      <dd class="text-lg font-medium tabular-nums">{value}</dd>
    </div>
  )
  return (
    <dl class="grid grid-cols-2 gap-3 md:grid-cols-4" data-testid="report-summary">
      {item(t('Backtests.Report.Examples'), integer(r().examples, app.lang))}
      {item(t('Backtests.Report.Triggered'), integer(r().triggered, app.lang))}
      {item(t('Backtests.Report.Calls'), t('Backtests.Report.CallsValue', { live: integer(cost().live_calls, app.lang), cached: integer(cost().cached_calls, app.lang) }), 'report-calls')}
      {item(t('Backtests.Report.Cost'), cost().live_calls > 0 || cost().cached_calls > 0 ? usd(cost().live_usd, app.lang) : '—', 'report-cost')}
    </dl>
  )
}

function Metrics(props: { report: ReportResult; title: (node: string) => string }): JSX.Element {
  const app = useApp()
  const t = app.t
  const labeled = () => props.report.classifiers.filter((c) => c.labeled > 0 || c.undecided > 0 || c.decision)
  return (
    <section aria-labelledby="metrics-title" class="flex flex-col gap-3">
      <h3 id="metrics-title" class="text-base font-medium">{t('Backtests.Report.Metrics')}</h3>
      <Show when={labeled().some((c) => c.labeled > 0)} fallback={<p class="rounded-lg border border-dashed border-subtle p-4 text-sm text-200" data-testid="metrics-empty">{t('Backtests.Report.NoLabels')}</p>}>
        <div class="overflow-x-auto rounded-lg border border-subtle bg-surface-600" tabindex="0" role="region" aria-labelledby="metrics-title">
          <table class="w-full text-sm" data-testid="metrics-table">
            <thead class="bg-surface-500 text-left text-xs text-200">
              <tr>
                <th scope="col" class="px-4 py-2 font-medium">{t('Backtests.Report.Classifier')}</th>
                <th scope="col" class="px-4 py-2 font-medium">{t('Backtests.Report.Outcome')}</th>
                <th scope="col" class="px-4 py-2 font-medium text-right">{t('Backtests.Report.Precision')}</th>
                <th scope="col" class="px-4 py-2 font-medium text-right">{t('Backtests.Report.Recall')}</th>
                <th scope="col" class="px-4 py-2 font-medium text-right">{t('Backtests.Report.Support')}</th>
              </tr>
            </thead>
            <tbody>
              <For each={labeled()}>
                {(c) => (
                  <>
                    <tr class="border-t border-subtle bg-surface-500/40" data-testid={`metrics-${c.node}`}>
                      <th scope="row" class="px-4 py-2 text-left font-medium">{props.title(c.node)}</th>
                      <td class="px-4 py-2 text-200" colSpan={4}>
                        {t('Backtests.Report.Accuracy', { value: percent(c.accuracy, app.lang), correct: integer(c.correct, app.lang), labeled: integer(c.labeled, app.lang) })}
                        <Show when={c.undecided > 0}> · {t('Backtests.Report.Undecided', { count: integer(c.undecided, app.lang) })}</Show>
                      </td>
                    </tr>
                    <For each={c.outcomes}>
                      {(o) => (
                        <tr class="border-t border-subtle" data-testid={`metrics-${c.node}-${o.outcome}`}>
                          <td class="px-4 py-2" />
                          <td class="px-4 py-2 font-mono text-xs">{o.outcome}</td>
                          <td class="px-4 py-2 text-right tabular-nums" data-testid="precision">{percent(o.precision, app.lang)}</td>
                          <td class="px-4 py-2 text-right tabular-nums" data-testid="recall">{percent(o.recall, app.lang)}</td>
                          <td class="px-4 py-2 text-right tabular-nums">{integer(o.support, app.lang)}</td>
                        </tr>
                      )}
                    </For>
                  </>
                )}
              </For>
            </tbody>
          </table>
        </div>
      </Show>
    </section>
  )
}

const END_ICON: Record<string, 'check' | 'clock' | 'x' | 'alert'> = { ended: 'check', waiting: 'clock', not_triggered: 'x', error: 'alert' }

/** What the graph did for each example, with the classifier filter and «only mismatches» (D1). */
function Cases(props: {
  backtestID: string; classifiers: Classifier[]; report: ReportResult; node: string | null; mismatch: boolean
  title: (node: string) => string; onView: (v: Partial<BacktestsView>) => void
}): JSX.Element {
  const app = useApp()
  const t = app.t
  const [page, setPage] = createSignal<ResultsResult>()
  const [error, setError] = createSignal<string>()
  const [more, setMore] = createSignal(false)
  const [open, setOpen] = createSignal<string>()
  // seq names the read in flight; committed the read whose first page is on screen, of query (the
  // backtest and its filters). Another query's rows never stay under the filters; a continuation appends
  // only to the page it continues.
  let seq = 0
  let committed = 0
  let shownQuery = ''
  onCleanup(() => { seq++ })
  const params = () => ({ backtest_id: props.backtestID, limit: 50, ...(props.node ? { node: props.node } : {}), ...(props.mismatch ? { mismatch: true } : {}) })
  const load = async () => {
    const mine = ++seq
    const q = params()
    const query = JSON.stringify(q)
    if (query !== shownQuery) {
      setPage(undefined)
      shownQuery = query
    }
    setError(undefined)
    try {
      const res = await app.call('automations.backtest_results', q)
      if (mine === seq) { setPage(res); committed = mine }
    } catch (e) {
      if (mine === seq) setError(backtestErrorText(t, e, t('Backtests.Actions.LoadResults')))
    }
  }
  createEffect(on(() => [props.backtestID, props.node, props.mismatch, props.report.backtest.done] as const, () => void load()))
  const loadMore = async () => {
    const p = page()
    const of = committed
    if (!p?.next_cursor || more() || of !== seq) return
    setMore(true)
    try {
      const res = await app.call('automations.backtest_results', { ...params(), cursor: p.next_cursor })
      if (of !== seq || page() !== p) return
      setPage({ ...res, items: [...p.items, ...res.items] })
    } catch (e) {
      app.toast.notify({ variant: 'error', title: backtestErrorText(t, e, t('Backtests.Actions.LoadResults')) })
    } finally {
      setMore(false)
    }
  }
  const nodes = createMemo(() => {
    const ids = new Set<string>(props.report.classifiers.map((c) => c.node))
    for (const c of props.classifiers) ids.add(c.id)
    return [...ids]
  })
  const shown = () => (props.node ? [props.node] : props.report.classifiers.filter((c) => c.decision || c.labeled > 0).map((c) => c.node))
  const filtered = () => Boolean(props.node || props.mismatch)
  const outcomeCell = (r: ResultsResult['items'][number], node: string) => {
    const label = () => r.example.labels[node]
    const miss = () => r.mismatches.includes(node)
    const decision = () => r.decisions.find((d) => d.node === node)
    return (
      <div class="flex flex-col gap-0.5">
        <span class="inline-flex items-center gap-1 font-mono text-xs" classList={{ 'text-red-600': miss() }}>
          <Show when={miss()}><Icon name="alert" size={12} label={t('Backtests.Cases.Mismatch')} /></Show>
          {decision()?.status === 'missing' ? t('Backtests.Cases.Missing') : r.outcomes[node] ?? t('Backtests.Cases.NotReached')}
        </span>
        <Show when={label()}>
          <span class="text-xs text-300">{t('Backtests.Cases.Expected', { outcome: label()!.outcome })}</span>
        </Show>
      </div>
    )
  }
  return (
    <section aria-labelledby="cases-title" class="flex flex-col gap-3" data-testid="cases">
      <div class="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <h3 id="cases-title" class="text-base font-medium">
          {t('Backtests.Cases.Title')}
          <Show when={props.report.mismatch_total > 0}>
            <span class="ml-2 text-sm font-normal text-200 tabular-nums" data-testid="mismatch-total">{t('Backtests.Cases.MismatchTotal', { count: integer(props.report.mismatch_total, app.lang) })}</span>
          </Show>
        </h3>
        <div class="flex flex-col gap-3 rounded-lg border border-subtle bg-surface-600 p-3 sm:flex-row sm:items-end" role="group" aria-label={t('Backtests.Cases.Filters')}>
          <div class="sm:w-64">
            <Select
              label={t('Backtests.Cases.Classifier')} value={props.node ?? ''}
              options={[{ value: '', label: t('Backtests.Cases.AllClassifiers') }, ...nodes().map((n) => ({ value: n, label: props.title(n) }))]}
              onChange={(e) => props.onView({ node: e.currentTarget.value || null })}
              data-testid="cases-classifier"
            />
          </div>
          <Toggle label={t('Backtests.Cases.OnlyMismatches')} checked={props.mismatch} onChange={(e) => props.onView({ mismatch: e.currentTarget.checked })} data-testid="cases-mismatch" />
        </div>
      </div>
      <Show when={page() && error()}>
        <StaleLine message={error()!} onRetry={() => void load()} testid="cases-stale" />
      </Show>
      <Switch>
        <Match when={!page() && error()}>
          <div role="alert" class="flex flex-col items-start gap-2" data-testid="cases-error"><p>{error()}</p><Button variant="secondary" size="sm" onClick={() => void load()}>{t('Common.Retry')}</Button></div>
        </Match>
        <Match when={!page()}>
          <div role="status" aria-busy="true" aria-label={t('Common.Loading')}><Skeleton variant="table" lines={4} /></div>
        </Match>
        <Match when={page()!.items.length === 0 && filtered()}>
          <EmptyState
            data-testid="cases-empty-filtered"
            title={props.mismatch ? t('Backtests.Cases.NoMismatches') : t('Backtests.Cases.NothingFor', { node: props.title(props.node ?? '') })}
            description={<Button variant="secondary" onClick={() => props.onView({ node: null, mismatch: false })}>{t('List.ResetFilters')}</Button>}
          />
        </Match>
        <Match when={page()!.items.length === 0}>
          <p class="text-sm text-200">{t('Backtests.Cases.Empty')}</p>
        </Match>
        <Match when={page()}>
          <div class="overflow-x-auto rounded-lg border border-subtle bg-surface-600" tabindex="0" role="region" aria-labelledby="cases-title">
            <table class="w-full text-sm" data-testid="cases-table">
              <thead class="bg-surface-500 text-left text-xs text-200">
                <tr>
                  <th scope="col" class="px-4 py-2 font-medium">{t('Backtests.Examples.Event')}</th>
                  <th scope="col" class="px-4 py-2 font-medium">{t('Backtests.Cases.End')}</th>
                  <For each={shown()}>{(n) => <th scope="col" class="px-4 py-2 font-medium">{props.title(n)}</th>}</For>
                  <th scope="col" class="px-4 py-2 font-medium">{t('Backtests.Cases.Effects')}</th>
                </tr>
              </thead>
              <tbody>
                <For each={page()!.items}>
                  {(r) => (
                    <>
                      <tr class="border-t border-subtle align-top" data-testid={`case-${r.example.id}`}>
                        <td class="px-4 py-3 max-w-sm">
                          <button type="button" class="text-left break-words hover:underline focus-visible:outline-2 focus-visible:outline-primary-500" aria-expanded={open() === r.example.id}
                            onClick={() => setOpen(open() === r.example.id ? undefined : r.example.id)} data-testid="case-toggle">
                            {exampleText(t, r.example)}
                          </button>
                          <time class="block text-xs text-300 tabular-nums" datetime={r.example.occurred_at}>{absoluteTime(r.example.occurred_at, app.lang)}</time>
                        </td>
                        <td class="px-4 py-3"><span class="inline-flex items-center gap-1 whitespace-nowrap"><Icon name={END_ICON[r.end] ?? 'dot'} class="text-300" />{t(`Backtests.End.${r.end}`)}</span></td>
                        <For each={shown()}>{(n) => <td class="px-4 py-3">{outcomeCell(r, n)}</td>}</For>
                        <td class="px-4 py-3 tabular-nums">{r.effects.length > 0 ? counted(t, 'Backtests.Cases.EffectCount', r.effects.length, app.lang) : '—'}</td>
                      </tr>
                      <Show when={open() === r.example.id}>
                        <tr class="border-t border-subtle bg-surface-500/40" data-testid="case-detail">
                          <td class="px-4 py-3" colSpan={3 + shown().length}>
                            <div class="flex flex-col gap-2 text-sm">
                              <Show when={r.error}><p class="flex items-center gap-1"><Icon name="alert" class="text-red-600" />{t('Backtests.Cases.Error')}</p></Show>
                              <Show when={r.snapshot_withheld}><p class="flex items-center gap-1" data-testid="case-snapshot-withheld"><Icon name="alert" class="text-yellow" />{t('Backtests.Cases.SnapshotWithheld')}</p></Show>
                              <Show when={r.effects.length > 0} fallback={<p class="text-200">{t('Backtests.Cases.NoEffects')}</p>}>
                                <p class="font-medium">{t('Backtests.Cases.WouldDo')}</p>
                                <ul class="flex flex-col gap-2">
                                  <For each={r.effects}>
                                    {(e) => (
                                      <li class="flex flex-col gap-1">
                                        <span class="text-xs text-200">{props.title(e.node)}</span>
                                        <pre class="max-h-40 overflow-auto rounded-md border border-subtle bg-surface-600 p-2 text-xs font-mono" tabindex="0">{JSON.stringify(e.params, null, 2)}</pre>
                                      </li>
                                    )}
                                  </For>
                                </ul>
                              </Show>
                            </div>
                          </td>
                        </tr>
                      </Show>
                    </>
                  )}
                </For>
              </tbody>
            </table>
          </div>
          <Show when={page()!.next_cursor}>
            <div class="flex justify-center"><Button variant="secondary" loading={more()} onClick={() => void loadMore()}>{t('List.LoadMore')}</Button></div>
          </Show>
        </Match>
      </Switch>
    </section>
  )
}

function Effects(props: { report: ReportResult; title: (node: string) => string }): JSX.Element {
  const app = useApp()
  const t = app.t
  return (
    <section aria-labelledby="effects-title" class="flex flex-col gap-3">
      <h3 id="effects-title" class="text-base font-medium">{t('Backtests.Report.Effects')}</h3>
      <Show when={props.report.effects.length > 0} fallback={<p class="text-sm text-200" data-testid="effects-none">{t('Backtests.Report.NoEffects')}</p>}>
        <ul class="flex flex-col divide-y divide-subtle rounded-lg border border-subtle bg-surface-600" data-testid="effects">
          <For each={props.report.effects}>
            {(e) => (
              <li class="flex flex-col gap-1 p-3 text-sm">
                <span class="font-medium">{props.title(e.node)}</span>
                <span class="text-200 tabular-nums">{t('Backtests.Report.EffectLine', { count: integer(e.count, app.lang), examples: integer(e.examples, app.lang) })}</span>
              </li>
            )}
          </For>
        </ul>
        <p class="text-xs text-200">{t('Backtests.Report.NothingSent')}</p>
      </Show>
    </section>
  )
}

/** Two backtests side by side over their shared examples: where the outcome or the effects differ. */
function Compare(props: { backtestID: string; compare: string | null; others: BacktestView[]; title: (node: string) => string; onView: (v: Partial<BacktestsView>) => void }): JSX.Element {
  const app = useApp()
  const t = app.t
  const [cmp, setCmp] = createSignal<CompareResult>()
  const [error, setError] = createSignal<string>()
  let seq = 0
  const load = async () => {
    const other = props.compare
    const mine = ++seq
    setCmp(undefined)
    setError(undefined)
    if (!other) return
    try {
      const res = await app.call('automations.backtest_compare', { a: other, b: props.backtestID })
      if (mine === seq) setCmp(res)
    } catch (e) {
      if (mine === seq) setError(backtestErrorText(t, e, t('Backtests.Actions.Compare')))
    }
  }
  createEffect(on(() => [props.backtestID, props.compare] as const, () => void load()))
  const label = (b: BacktestView) => t('Backtests.Runs.Name', { version: b.version, dataset: b.dataset_name })
  return (
    <section aria-labelledby="compare-title" class="flex flex-col gap-3" data-testid="compare">
      <div class="flex flex-col gap-2 md:flex-row md:items-end md:justify-between">
        <h3 id="compare-title" class="text-base font-medium">{t('Backtests.Compare.Title')}</h3>
        <Show when={props.others.length > 0} fallback={<p class="text-sm text-200">{t('Backtests.Compare.NoOther')}</p>}>
          <div class="rounded-lg border border-subtle bg-surface-600 p-3 sm:w-80">
            <Select label={t('Backtests.Compare.With')} value={props.compare ?? ''}
              options={[{ value: '', label: t('Backtests.Compare.Pick') }, ...props.others.map((b) => ({ value: b.id, label: label(b) }))]}
              onChange={(e) => props.onView({ compare: e.currentTarget.value || null })} data-testid="compare-with" />
          </div>
        </Show>
      </div>
      <Switch>
        <Match when={error()}>
          <div role="alert" class="flex flex-col items-start gap-2"><p>{error()}</p><Button variant="secondary" size="sm" onClick={() => void load()}>{t('Common.Retry')}</Button></div>
        </Match>
        <Match when={props.compare && !cmp()}>
          <div role="status" aria-busy="true" aria-label={t('Common.Loading')}><Skeleton variant="table" lines={3} /></div>
        </Match>
        <Match when={cmp()}>
          {(c) => (
            <>
              <p class="text-sm text-200 tabular-nums" data-testid="compare-summary">
                {t('Backtests.Compare.Summary', { changed: integer(c().changed, app.lang), common: integer(c().common_examples, app.lang), a: c().a.backtest.version, b: c().b.backtest.version })}
              </p>
              <Show when={c().diffs.length > 0}>
                <div class="overflow-x-auto rounded-lg border border-subtle bg-surface-600" tabindex="0" role="region" aria-labelledby="compare-title">
                  <table class="w-full text-sm" data-testid="compare-table">
                    <thead class="bg-surface-500 text-left text-xs text-200">
                      <tr>
                        <th scope="col" class="px-4 py-2 font-medium">{t('Backtests.Examples.Event')}</th>
                        <th scope="col" class="px-4 py-2 font-medium">{t('Backtests.Compare.Changes', { a: c().a.backtest.version, b: c().b.backtest.version })}</th>
                        <th scope="col" class="px-4 py-2 font-medium">{t('Backtests.Cases.Effects')}</th>
                      </tr>
                    </thead>
                    <tbody>
                      <For each={c().diffs}>
                        {(d) => (
                          <tr class="border-t border-subtle align-top" data-testid={`diff-${d.example_id}`}>
                            <td class="px-4 py-3 max-w-sm break-words">{d.title || t('Backtests.Examples.Untitled')}</td>
                            <td class="px-4 py-3">
                              <ul class="flex flex-col gap-1">
                                <For each={d.changes}>
                                  {(ch) => (
                                    <li class="flex flex-wrap items-center gap-1">
                                      <span class="text-200">{props.title(ch.node)}:</span>
                                      <span class="font-mono text-xs">{ch.a || t('Backtests.Cases.NotReached')}</span>
                                      <Icon name="arrow" size={12} label={t('Backtests.Compare.Became')} />
                                      <span class="font-mono text-xs">{ch.b || t('Backtests.Cases.NotReached')}</span>
                                      <Show when={ch.expected}><span class="text-xs text-300">{t('Backtests.Cases.Expected', { outcome: ch.expected! })}</span></Show>
                                    </li>
                                  )}
                                </For>
                                <Show when={d.end_a !== d.end_b}>
                                  <li class="text-200">{t(`Backtests.End.${d.end_a}`)} → {t(`Backtests.End.${d.end_b}`)}</li>
                                </Show>
                              </ul>
                            </td>
                            <td class="px-4 py-3 text-200">
                              <Show when={d.effects_a.length > 0 || d.effects_b.length > 0} fallback="—">
                                <Show when={d.params_changed} fallback={<>{(d.effects_a.map(props.title).join(', ') || '—')} → {(d.effects_b.map(props.title).join(', ') || '—')}</>}>
                                  {t('Backtests.Compare.ParamsChanged', { effects: d.effects_b.map(props.title).join(', ') })}
                                </Show>
                              </Show>
                            </td>
                          </tr>
                        )}
                      </For>
                    </tbody>
                  </table>
                </div>
              </Show>
            </>
          )}
        </Match>
      </Switch>
    </section>
  )
}

function DialogFrame(props: { id: string; title: string; busy: boolean; onClose: () => void; testid: string; children: JSX.Element; footer: JSX.Element; onSubmit: () => void; wide?: boolean }): JSX.Element {
  const t = useApp().t
  return (
    <Dialog open busy={props.busy} onOpenChange={(open) => { if (!open) props.onClose() }} class={props.wide ? 'md:w-full md:max-w-2xl' : 'md:w-full md:max-w-lg'} aria-labelledby={props.id} data-testid={props.testid}>
      <form onSubmit={(e) => { e.preventDefault(); props.onSubmit() }}>
        <header class="flex items-center justify-between gap-3 border-b border-subtle py-1 pl-4 pr-2">
          <h3 id={props.id} class="font-medium truncate" data-dialog-title tabindex="-1">{props.title}</h3>
          <button type="button" class="drawer-close" aria-label={t('Common.Close')} data-dialog-close onClick={props.onClose}><Icon name="x" size={20} /></button>
        </header>
        <div class="flex max-h-[65vh] flex-col gap-4 overflow-y-auto px-4 py-3" data-dialog-body>{props.children}</div>
        <footer class="border-t border-subtle px-4 py-3 pb-[max(0.75rem,env(safe-area-inset-bottom))]">
          <DialogBusyStatus class="mb-2" />
          <menu class="flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
            <Button type="button" variant="secondary" class="min-h-11 justify-center" data-dialog-close onClick={props.onClose} aria-disabled={props.busy || undefined}>{t('Common.Cancel')}</Button>
            {props.footer}
          </menu>
        </footer>
      </form>
    </Dialog>
  )
}

/** A new dataset with its first import, or an import into an existing one (F9: the import runs in the background). */
function ImportDialog(props: { automationID: string; dataset?: DatasetView; sources: SourceView[]; onCreated: () => void; onDone: (dataset: string) => void; onClose: () => void }): JSX.Element {
  const app = useApp()
  const t = app.t
  const creating = !props.dataset
  const [name, setName] = createSignal('')
  const [source, setSource] = createSignal(creating ? 'journal' : 'journal')
  const [days, setDays] = createSignal('30')
  const [busy, setBusy] = createSignal(false)
  const [error, setError] = createSignal<string>()
  const [nameError, setNameError] = createSignal<string>()
  // One dataset per intent: a retry after a lost answer imports into the dataset already created (F3).
  let created: string | undefined = props.dataset?.id
  const options = () => [
    { value: 'journal', label: t('Backtests.Import.Journal') },
    ...props.sources.map((s) => ({ value: s.kind === 'connection' ? `connection:${s.connection_id}` : `module:${s.source}`, label: t('Backtests.Import.From', { source: s.title }) })),
    ...(creating ? [{ value: 'none', label: t('Backtests.Import.None') }] : []),
  ]
  const submit = async () => {
    if (busy()) return
    if (creating && !name().trim()) { setNameError(t('Backtests.Datasets.NameRequired')); return }
    setNameError(undefined)
    setBusy(true)
    setError(undefined)
    try {
      if (!created) {
        created = (await app.call('automations.dataset_create', { automation_id: props.automationID, name: name().trim() })).id
        // The dataset exists whatever its import does: the overview shows it now.
        props.onCreated()
      }
      const src = source()
      if (src !== 'none') {
        const [kind, ref] = src.split(':') as ['journal' | 'connection' | 'module', string | undefined]
        await app.call('automations.dataset_import', {
          dataset_id: created, kind, days: Number(days()),
          ...(kind === 'connection' ? { connection_id: ref } : {}), ...(kind === 'module' ? { source: ref } : {}),
        })
      }
      app.toast.notify({ variant: 'success', title: src === 'none' ? t('Backtests.Datasets.Created') : t('Backtests.Import.Started') })
      props.onDone(created)
    } catch (e) {
      setError(backtestErrorText(t, e, creating ? t('Backtests.Actions.Create') : t('Backtests.Actions.Import')))
    } finally {
      setBusy(false)
    }
  }
  return (
    <DialogFrame
      id="import-title" testid="import-dialog" busy={busy()} onClose={props.onClose} onSubmit={() => void submit()}
      title={creating ? t('Backtests.Datasets.New') : t('Backtests.Import.Title', { name: props.dataset!.name })}
      footer={<Button type="submit" variant="primary" class="min-h-11 justify-center" loading={busy()} data-testid="import-submit">{busy() ? t('Backtests.Import.Submitting') : creating ? t('Backtests.Datasets.Create') : t('Backtests.Import.Submit')}</Button>}
    >
      <Show when={creating}>
        <Input type="text" label={`${t('Backtests.Datasets.Name')} *`} value={name()} error={nameError()} maxLength={200} onInput={(e) => setName(e.currentTarget.value)} data-testid="dataset-name-input" />
      </Show>
      <Select label={t('Backtests.Import.Source')} value={source()} options={options()} onChange={(e) => setSource(e.currentTarget.value)} data-testid="import-source" />
      <Show when={source() !== 'none'}>
        <Select label={t('Backtests.Import.Window')} value={days()} onChange={(e) => setDays(e.currentTarget.value)} data-testid="import-days"
          options={[7, 30, 90].map((d) => ({ value: String(d), label: counted(t, 'Backtests.Import.Days', d, app.lang) }))} />
        <p class="text-sm text-200">{source() === 'journal' ? t('Backtests.Import.JournalHint') : t('Backtests.Import.HistoryHint')}</p>
      </Show>
      <Show when={error()}><p role="alert" class="text-sm text-red-600" data-testid="import-error">{error()}</p></Show>
    </DialogFrame>
  )
}

/** A synthetic event: its fields by the trigger's payload schema, and the outcome each classifier should take. */
function SyntheticDialog(props: { dataset: DatasetView; graph?: Graph; classifiers: Classifier[]; onDone: (dataset: string) => void; onClose: () => void }): JSX.Element {
  const app = useApp()
  const t = app.t
  const events = createMemo(() => {
    const g = props.graph
    if (!g) return []
    const keys = [...new Set(g.graph.triggers.map((tr) => tr.event))]
    return keys.map((k) => g.catalog.events.find((e) => e.key === k)).filter((e): e is EditorCatalog['events'][number] => Boolean(e))
  })
  const [kind, setKind] = createSignal(events()[0]?.key ?? '')
  const fields = createMemo(() => inputFields(events().find((e) => e.key === kind())?.payload))
  const [values, setValues] = createSignal<Record<string, string | boolean>>({})
  const [fieldErrors, setFieldErrors] = createSignal<Record<string, string>>({})
  const [title, setTitle] = createSignal('')
  const [expected, setExpected] = createSignal<Record<string, string>>({})
  const [busy, setBusy] = createSignal(false)
  const [error, setError] = createSignal<string>()
  const set = (name: string, v: string | boolean) => setValues((cur) => ({ ...cur, [name]: v }))
  const submit = async () => {
    if (busy()) return
    const { input, errors } = inputValue(fields(), values())
    const messages: Record<string, string> = {}
    for (const [k, v] of Object.entries(errors)) messages[k] = t(`Run.Field.${v === 'required' ? 'Required' : v === 'number' ? 'Number' : 'Json'}`)
    setFieldErrors(messages)
    if (Object.keys(messages).length > 0 || !kind()) return
    setBusy(true)
    setError(undefined)
    try {
      await app.call('automations.dataset_synthetic', { dataset_id: props.dataset.id, kind: kind(), payload: input, title: title().trim(), expected: expected() })
      app.toast.notify({ variant: 'success', title: t('Backtests.Synthetic.Added') })
      props.onDone(props.dataset.id)
    } catch (e) {
      setError(backtestErrorText(t, e, t('Backtests.Actions.Synthetic')))
    } finally {
      setBusy(false)
    }
  }
  const field = (f: ReturnType<typeof fields>[number]): JSX.Element => {
    const err = () => fieldErrors()[f.name]
    const label = <>{f.title}{f.required ? ' *' : ''}</>
    return (
      <Switch>
        <Match when={f.kind === 'boolean'}>
          <Toggle label={f.title} checked={Boolean(values()[f.name])} onChange={(e) => set(f.name, e.currentTarget.checked)} />
        </Match>
        <Match when={f.kind === 'enum'}>
          <Select label={label} error={err()} placeholder={t('Run.Choose')} value={String(values()[f.name] ?? '')}
            options={(f.options ?? []).map((o, i) => ({ value: String(i), label: o }))} onChange={(e) => set(f.name, e.currentTarget.value)} />
        </Match>
        <Match when={f.kind === 'textarea' || f.kind === 'json'}>
          <Textarea label={label} description={f.kind === 'json' ? t('Run.JsonHint') : f.description} error={err()} value={String(values()[f.name] ?? '')}
            onInput={(e) => set(f.name, e.currentTarget.value)} rows={3} data-field={f.name} />
        </Match>
        <Match when={true}>
          <Input type={f.kind === 'number' || f.kind === 'integer' ? 'number' : 'text'} step={f.kind === 'number' ? 'any' : undefined} label={label} description={f.description} error={err()}
            value={String(values()[f.name] ?? '')} onInput={(e) => set(f.name, e.currentTarget.value)} data-field={f.name} />
        </Match>
      </Switch>
    )
  }
  return (
    <DialogFrame
      id="synthetic-title" testid="synthetic-dialog" busy={busy()} onClose={props.onClose} onSubmit={() => void submit()} wide
      title={t('Backtests.Synthetic.Title')}
      footer={<Button type="submit" variant="primary" class="min-h-11 justify-center" loading={busy()} disabled={!kind()} data-testid="synthetic-submit">{busy() ? t('Backtests.Synthetic.Adding') : t('Backtests.Synthetic.Add')}</Button>}
    >
      <Show when={events().length > 0} fallback={<p class="text-sm text-200">{t('Backtests.Synthetic.NoEvents')}</p>}>
        <Show when={events().length > 1}>
          <Select label={t('Backtests.Synthetic.Event')} value={kind()} options={events().map((e) => ({ value: e.key, label: e.title }))}
            onChange={(e) => { setKind(e.currentTarget.value); setValues({}); setFieldErrors({}) }} />
        </Show>
        <Input type="text" label={t('Backtests.Synthetic.Name')} description={t('Backtests.Synthetic.NameHint')} value={title()} maxLength={200} onInput={(e) => setTitle(e.currentTarget.value)} data-testid="synthetic-name" />
        <fieldset class="flex flex-col gap-3">
          <legend class="mb-1 text-sm font-medium">{t('Backtests.Synthetic.Payload')}</legend>
          <Show when={fields().length > 0} fallback={<p class="text-sm text-300">{t('Run.NoInput')}</p>}>
            <For each={fields()}>{(f) => field(f)}</For>
          </Show>
        </fieldset>
        <Show when={props.classifiers.length > 0}>
          <fieldset class="flex flex-col gap-3 border-t border-subtle pt-3">
            <legend class="mb-1 text-sm font-medium">{t('Backtests.Synthetic.Expected')}</legend>
            <For each={props.classifiers}>
              {(c) => (
                <Select label={c.title} value={expected()[c.id] ?? ''}
                  options={[{ value: '', label: t('Backtests.Labels.None') }, ...c.outcomes.map((o) => ({ value: o, label: o }))]}
                  onChange={(e) => setExpected((cur) => ({ ...cur, [c.id]: e.currentTarget.value }))} data-testid={`expected-${c.id}`} />
              )}
            </For>
          </fieldset>
        </Show>
      </Show>
      <Show when={error()}><p role="alert" class="text-sm text-red-600" data-testid="synthetic-error">{error()}</p></Show>
    </DialogFrame>
  )
}

/**
 * A backtest: the dataset, the version and how decisions are made. Live decisions are a person's, and
 * what they may cost is shown before the run (from the decisions of earlier backtests).
 */
function RunBacktestDialog(props: { datasets: DatasetView[]; dataset?: DatasetView; graph?: Graph; live: boolean; onDone: (backtest: string) => void; onClose: () => void }): JSX.Element {
  const app = useApp()
  const t = app.t
  const runnable = () => props.datasets.filter((d) => d.examples > 0)
  const [dataset, setDataset] = createSignal(props.dataset?.id ?? runnable()[0]?.id ?? '')
  const [version, setVersion] = createSignal(String(props.graph?.latest ?? 0))
  const [decisions, setDecisions] = createSignal<'cached' | 'live'>('cached')
  const [estimate, setEstimate] = createSignal<EstimateResult>()
  const [estimateError, setEstimateError] = createSignal<string>()
  const [busy, setBusy] = createSignal(false)
  const [error, setError] = createSignal<string>()
  let seq = 0
  // The estimate is read again when the dataset's contents move (an import brought more), not only
  // when the dataset, the version or the mode change; again names a refresh after a stale run.
  const [again, setAgain] = createSignal(0)
  // A scalar memo: a fresh overview (another job's progress) with the same count changes nothing.
  const contents = createMemo(() => props.datasets.find((d) => d.id === dataset())?.examples ?? 0)
  createEffect(on(() => [dataset(), version(), decisions(), contents(), again()] as const, async ([ds, v, d]) => {
    const mine = ++seq
    setEstimate(undefined)
    setEstimateError(undefined)
    if (!ds) return
    try {
      const est = await app.call('automations.backtest_estimate', { dataset_id: ds, version: Number(v), decisions: d })
      if (mine === seq) setEstimate(est)
    } catch (e) {
      if (mine === seq) setEstimateError(backtestErrorText(t, e, t('Backtests.Actions.Estimate')))
    }
  }))
  // A run that may pay (live, and a person's cached run on cache misses) starts only once its cost is
  // shown: the estimate of the dataset, version and mode on screen
  // (one whose dollar cost is unknown still says how many calls it may make).
  const blocked = () => !dataset() || (decisions() === 'live' && !estimate()) || (decisions() === 'cached' && !app.props.agent && !estimate())
  const submit = async () => {
    if (busy() || blocked()) return
    setBusy(true)
    setError(undefined)
    try {
      const est = estimate()
      const res = await app.call('automations.backtest_run', { dataset_id: dataset(), version: Number(version()), decisions: decisions(), ...(est ? { revision: est.revision } : {}) })
      app.toast.notify({ variant: 'success', title: t('Backtests.Run.Started') })
      props.onDone(res.id)
    } catch (e) {
      setError(backtestErrorText(t, e, t('Backtests.Actions.Run')))
      // The examples changed since the estimate on screen: it is read again before another try.
      if (errorCode(e) === 'estimate_stale') setAgain((n) => n + 1)
    } finally {
      setBusy(false)
    }
  }
  const costText = (est: EstimateResult) => {
    if (est.decision_nodes.length === 0) return t('Backtests.Run.NoDecisions')
    const calls = counted(t, 'Backtests.Run.Calls', est.max_calls, app.lang)
    // A person's cached run pays for the decisions the cache lacks: their bound is shown as the live one.
    const text = decisions() === 'cached'
      ? (est.cost_usd !== null && !app.props.agent ? t('Backtests.Run.CachedCostUSD', { calls, cost: usd(est.cost_usd, app.lang) }) : t('Backtests.Run.CachedCost', { calls }))
      : est.cost_usd !== null ? t('Backtests.Run.LiveCost', { calls, cost: usd(est.cost_usd, app.lang) }) : t('Backtests.Run.LiveCostUnknown', { calls })
    return est.per_item ? `${text} ${t('Backtests.Run.PerItem')}` : text
  }
  return (
    <DialogFrame
      id="backtest-run-title" testid="backtest-run-dialog" busy={busy()} onClose={props.onClose} onSubmit={() => void submit()}
      title={t('Backtests.Run.Title')}
      footer={<Button type="submit" variant="primary" class="min-h-11 justify-center" loading={busy()} disabled={blocked()} icon={<Icon name="play" />} data-testid="backtest-run-submit">{busy() ? t('Backtests.Run.Starting') : t('Backtests.Run.Start')}</Button>}
    >
      <Show when={runnable().length > 0} fallback={<p class="text-sm text-200">{t('Backtests.Errors.EmptyDataset')}</p>}>
        <Select label={t('Backtests.Run.Dataset')} value={dataset()} onChange={(e) => setDataset(e.currentTarget.value)} data-testid="backtest-run-dataset"
          options={runnable().map((d) => ({ value: d.id, label: `${d.name} · ${counted(t, 'Backtests.Datasets.Examples', d.examples, app.lang)}` }))} />
        <Select label={t('Backtests.Run.Version')} value={version()} onChange={(e) => setVersion(e.currentTarget.value)} data-testid="backtest-run-version"
          options={(props.graph?.versions ?? []).map((v) => ({ value: String(v.version), label: v.version === props.graph?.latest ? t('Backtests.Run.VersionLatest', { version: v.version }) : `v${v.version}` }))} />
        <fieldset class="flex flex-col gap-2">
          <legend class="mb-1 text-sm font-medium">{t('Backtests.Run.Decisions')}</legend>
          <label class="flex min-h-11 items-start gap-2 text-sm">
            <input type="radio" name="decisions" value="cached" checked={decisions() === 'cached'} onChange={() => setDecisions('cached')} class="mt-1" data-testid="decisions-cached" />
            <span class="flex flex-col"><span class="font-medium">{t('Backtests.Decisions.Cached')}</span><span class="text-200">{app.props.agent ? t('Backtests.Decisions.CachedAgentHint') : t('Backtests.Decisions.CachedHint')}</span></span>
          </label>
          <Show when={props.live} fallback={<p class="flex items-center gap-1 text-sm text-200" data-testid="live-note"><Icon name="lock" />{t('Backtests.Decisions.LiveHuman')}</p>}>
            <label class="flex min-h-11 items-start gap-2 text-sm">
              <input type="radio" name="decisions" value="live" checked={decisions() === 'live'} onChange={() => setDecisions('live')} class="mt-1" data-testid="decisions-live" />
              <span class="flex flex-col"><span class="font-medium">{t('Backtests.Decisions.Live')}</span><span class="text-200">{t('Backtests.Decisions.LiveHint')}</span></span>
            </label>
          </Show>
        </fieldset>
        <div class="min-h-10 rounded-md border border-subtle p-3 text-sm" role="status" aria-live="polite" data-testid="backtest-estimate">
          <Switch fallback={<span class="text-300">{t('Backtests.Run.Estimating')}</span>}>
            <Match when={estimateError()}><span>{estimateError()}</span></Match>
            <Match when={estimate()}>{(est) => <span>{costText(est())}</span>}</Match>
          </Switch>
        </div>
        <p class="flex items-start gap-2 text-sm text-200"><Icon name="lock" class="mt-0.5" />{t('Backtests.Run.NothingSent')}</p>
      </Show>
      <Show when={error()}><p role="alert" class="text-sm text-red-600" data-testid="backtest-run-error">{error()}</p></Show>
    </DialogFrame>
  )
}

function DeleteDatasetDialog(props: { dataset: DatasetView; onDone: () => void; onClose: () => void }): JSX.Element {
  const app = useApp()
  const t = app.t
  const [busy, setBusy] = createSignal(false)
  const [error, setError] = createSignal<string>()
  const submit = async () => {
    if (busy()) return
    setBusy(true)
    setError(undefined)
    try {
      await app.call('automations.dataset_delete', { dataset_id: props.dataset.id })
      app.toast.notify({ variant: 'success', title: t('Backtests.Datasets.Deleted', { name: props.dataset.name }) })
      props.onDone()
    } catch (e) {
      setError(backtestErrorText(t, e, t('Backtests.Actions.Delete')))
    } finally {
      setBusy(false)
    }
  }
  return (
    <DialogFrame
      id="dataset-delete-title" testid="dataset-delete-dialog" busy={busy()} onClose={props.onClose} onSubmit={() => void submit()}
      title={t('Backtests.Datasets.DeleteTitle', { name: props.dataset.name })}
      footer={<Button type="submit" variant="danger" class="min-h-11 justify-center" loading={busy()} data-testid="dataset-delete-confirm">{busy() ? t('Backtests.Datasets.Deleting') : t('Backtests.Datasets.DeleteConfirm')}</Button>}
    >
      <p class="text-sm">{t('Backtests.Datasets.DeleteText', { examples: counted(t, 'Backtests.Datasets.Examples', props.dataset.examples, app.lang) })}</p>
      <Show when={error()}><p role="alert" class="text-sm text-red-600">{error()}</p></Show>
    </DialogFrame>
  )
}
