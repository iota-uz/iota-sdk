// The Runs tab (RV1–RV10): the list of the graph's runs beside the open run on the canvas. Its view state
// (run, node, state filter, subject search) belongs to the host when it passes onRunsView (the URL, D1);
// otherwise the tab keeps it. With no run chosen the newest one opens. On phones the list is full width
// and a run is its own screen with the read-only outline and the node's sheet.
import { batch, createEffect, createMemo, createSignal, on, Show, type JSX } from 'solid-js'
import type { AutomationGraph, AutomationLayout, EditorApi, EditorCatalog, EditorLocale, RunData, RunsView, RunSummary, Subject } from '../types'
import { Canvas } from '../Canvas'
import { Icon } from '../icons'
import { RunsList } from './RunsList'
import { runStrings } from './strings'
import { RunView, type VersionGraph } from './RunView'
import { createMinuteClock } from './util'

export const RUNS_PAGE = 50

export interface RunsTabProps {
  api: EditorApi
  catalog: EditorCatalog
  locale: EditorLocale
  automationId: string
  /** The saved version open in the editor and its graph. */
  version: number
  graph: AutomationGraph
  layout: AutomationLayout
  view?: RunsView
  onView?(view: RunsView, push: boolean): void
  operate: boolean
  onOpenVersion?(version: number): void
  subjectHref?(subject: Subject): string | undefined
  onStart?(): void
  mobile: boolean
  /** Kept by the editor across tab switches. */
  cache: Map<string, RunData>
  versions: Map<number, Promise<VersionGraph>>
}

export function RunsTab(props: RunsTabProps): JSX.Element {
  const now = createMinuteClock()
  // ——— view state (controlled by the host, or own) ———
  const [own, setOwn] = createSignal<RunsView>(props.view ?? {})
  const view = (): RunsView => (props.onView ? props.view ?? {} : own())
  const setView = (patch: Partial<RunsView>, push: boolean) => {
    const next: RunsView = { ...view(), ...patch }
    if (props.onView) props.onView(next, push)
    else setOwn(next)
  }
  const states = () => view().state ?? []
  const subject = () => view().subject ?? ''

  // ——— the list ———
  const [items, setItems] = createSignal<RunSummary[] | undefined>()
  const [next, setNext] = createSignal<string | null>(null)
  const [error, setError] = createSignal(false)
  const [refreshedAt, setRefreshedAt] = createSignal<string | null>(null)
  const [loadingMore, setLoadingMore] = createSignal(false)
  let listSeq = 0
  const load = async () => {
    if (!props.api.runs) return
    // An answer to an older filter never replaces a newer one (S6).
    const mine = ++listSeq
    setError(false)
    try {
      const page = await props.api.runs({
        automation_id: props.automationId, limit: RUNS_PAGE, ...(props.version > 0 ? { version: props.version } : {}),
        ...(states().length > 0 ? { state: states() } : {}),
        ...(subject().trim() ? { subject: subject().trim() } : {}),
      })
      if (mine !== listSeq) return
      batch(() => {
        setItems(page.items)
        setNext(page.next_cursor)
        setRefreshedAt(new Date().toISOString())
      })
    } catch (e) {
      if (mine !== listSeq) return
      console.warn('automation-editor: runs failed to load', e)
      setError(true)
    }
  }
  const more = async () => {
    const cursor = next()
    if (!cursor || !props.api.runs || loadingMore()) return
    const mine = listSeq
    setLoadingMore(true)
    try {
      const page = await props.api.runs({
        automation_id: props.automationId, limit: RUNS_PAGE, ...(props.version > 0 ? { version: props.version } : {}), cursor,
        ...(states().length > 0 ? { state: states() } : {}),
        ...(subject().trim() ? { subject: subject().trim() } : {}),
      })
      if (mine !== listSeq) return
      batch(() => {
        setItems((list) => [...(list ?? []), ...page.items.filter((r) => !(list ?? []).some((x) => x.run_id === r.run_id))])
        setNext(page.next_cursor)
      })
    } catch (e) {
      console.warn('automation-editor: more runs failed to load', e)
    } finally {
      setLoadingMore(false)
    }
  }
  // The filters are the key of the list: a change reloads it, a node or run change does not.
  // The history of the open version (a run of another version still opens by its link).
  const filterKey = createMemo(() => `${props.version}\u0000${states().join(',')}\u0000${subject().trim()}`)
  // A new filter drops the old list at once: nothing of it may open a run or answer for the new filter.
  createEffect(on(filterKey, (_key, previous) => {
    if (previous !== undefined) {
      listSeq += 1
      setItems(undefined)
      setNext(null)
    }
    void load()
  }))

  // With no run chosen, the newest opens (not on a phone, where the list is the screen).
  createEffect(() => {
    const list = items()
    if (props.mobile || view().run || !list || list.length === 0) return
    setView({ run: list[0]!.run_id, node: null }, false)
  })

  // A poll of the open run updates its row; a row that did not change is not replaced (P4).
  const onLoaded = (run: RunData) => {
    setItems((list) => {
      if (!list) return list
      const i = list.findIndex((r) => r.run_id === run.run_id)
      if (i < 0) return list
      const row = list[i]!
      // Where it stands: its active tokens and its open waits (a fork's token waiting on its branches is not).
      const nodes = [...new Set([...run.tokens.filter((tk) => tk.state === 'active').map((tk) => tk.node), ...run.waits.map((w) => w.node)].filter((n): n is string => Boolean(n)))]
      if (row.state === run.state && row.updated_at === run.updated_at && row.error === run.error && (row.current_nodes ?? []).join(',') === nodes.join(',')) return list
      const copy = [...list]
      copy[i] = { ...row, state: run.state, updated_at: run.updated_at, error: run.error, current_nodes: nodes }
      return copy
    })
  }

  // ——— the graph of a run's version ———
  const graphFor = (version: number): Promise<VersionGraph> => {
    if (version === props.version || !props.api.version) return Promise.resolve({ graph: props.graph, layout: props.layout })
    let p = props.versions.get(version)
    if (!p) {
      p = props.api.version(version)
      // A failed read is not kept: «Retry» asks again.
      p.catch(() => props.versions.delete(version))
      props.versions.set(version, p)
    }
    return p
  }

  const runId = createMemo(() => view().run ?? null)
  const list = () => (
    <RunsList
      items={items()}
      error={error()}
      refreshedAt={refreshedAt()}
      selected={runId()}
      states={states()}
      subject={subject()}
      graph={props.graph}
      catalog={props.catalog}
      locale={props.locale}
      now={now()}
      more={Boolean(next())}
      loadingMore={loadingMore()}
      onOpen={(run) => { if (run !== runId()) setView({ run, node: null }, true) }}
      onStates={(s) => setView({ state: s, run: null, node: null }, false)}
      onSubject={(q) => { if (q !== subject()) setView({ subject: q, run: null, node: null }, false) }}
      onReset={() => setView({ state: [], subject: '', run: null, node: null }, false)}
      onRetry={() => void load()}
      onMore={() => void more()}
      {...(props.onStart ? { onStart: props.onStart } : {})}
    />
  )
  const run = () => (
    <Show when={runId()} keyed>
      {(id) => (
        <RunView
          runId={id}
          api={props.api}
          catalog={props.catalog}
          locale={props.locale}
          now={now()}
          cache={props.cache}
          version={props.version}
          graphFor={graphFor}
          operate={props.operate}
          node={view().node ?? null}
          onNode={(node) => { if (node !== (view().node ?? null)) setView({ node }, node !== null) }}
          {...(props.onOpenVersion ? { onOpenVersion: props.onOpenVersion } : {})}
          {...(props.subjectHref ? { subjectHref: props.subjectHref } : {})}
          onLoaded={onLoaded}
          mobile={props.mobile}
          {...(props.mobile ? { onBack: () => setView({ run: null, node: null }, true) } : {})}
        />
      )}
    </Show>
  )

  // No run open beside the list (none yet, or none matches the filter): the graph of this version, read
  // only, with how to see a run on it; the pane never stands empty.
  const idle = () => (
    <div class="ae-runs-idle">
      <div class="ae-runs-idle__canvas">
        <Canvas graph={props.graph} catalog={props.catalog} layout={props.layout} selected={null} onSelect={() => {}} label={runStrings(props.locale).list.pick} locale={props.locale} />
      </div>
      <p class="ae-runs-idle__hint" role="note"><Icon name="history" size={16} />{runStrings(props.locale).list.pick}</p>
    </div>
  )
  return (
    <div class="ae-runs-tab" classList={{ 'ae-runs-tab--mobile': props.mobile }}>
      <Show when={props.mobile} fallback={<>{list()}<div class="ae-runs-tab__run"><Show when={runId()} fallback={idle()}>{run()}</Show></div></>}>
        <Show when={runId()} fallback={list()}>{run()}</Show>
      </Show>
    </div>
  )
}
