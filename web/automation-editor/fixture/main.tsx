// Fixture host for the editor: a fake EditorApi with switches in the query string, for manual checks,
// e2e and the benchmark. Query: graph=sample|connected|empty|bench|multi|foreach, locale, theme=dark, readonly=1,
// latency=<ms>, fail=save|validate|savelayout, conflict=1, pending=1, overlay=1, catalog=empty,
// mode=canvas (the Canvas alone, with counters), errors=1 (validate reports an error on has_email).
// Runs (#1347): tab=runs, run=<id>, node=<id>, runs=empty|error|slowops|notfound|noop (no Operate methods),
// can=read (no author, no operate), manual=1 («Run» on the empty tab), test=secret|slow.
import { batch, createSignal, Show } from 'solid-js'
import { render } from 'solid-js/web'
import { Canvas } from '../src/Canvas'
import { Editor } from '../src/Editor'
import { EditorSkeleton } from '../src/lazy'
import { ensureLayout } from '../src/layout'
import type { AutomationGraph, AutomationLayout, EditorApi, EditorLocale, GraphError, Overlay, RunData, RunsView, RunState, SaveResult } from '../src/types'
import { catalog as fullCatalog, emptyCatalog } from './catalog'
import { bench, connected, empty, sample } from './graphs'
import { foreachJoin, layoutCatalog, multiTrigger } from './layoutGraphs'
import { approval, approvalWaiting, sampleRuns, sampleV2, toSummary } from './runs'
import './solid-ui.generated.css'
import '../src/styles.css'
import './fixture.css'

const params = new URLSearchParams(location.search)
const locale = (params.get('locale') ?? 'en') as EditorLocale
const latency = Number(params.get('latency') ?? '30')
const fail = new Set((params.get('fail') ?? '').split(',').filter(Boolean))
if (params.get('theme') === 'dark') document.documentElement.classList.add('dark')
document.documentElement.lang = locale

const layoutGraph = params.get('graph') === 'multi' || params.get('graph') === 'foreach' || params.get('graph') === 'approval'
const catalog = params.get('catalog') === 'empty' ? emptyCatalog : layoutGraph ? layoutCatalog : fullCatalog
const pick = (): { graph: AutomationGraph; layout: AutomationLayout } => {
  switch (params.get('graph')) {
    case 'bench': return bench()
    case 'connected': return { graph: connected, layout: {} }
    case 'empty': return { graph: empty, layout: {} }
    case 'multi': return { graph: multiTrigger, layout: {} }
    case 'foreach': return { graph: foreachJoin, layout: {} }
    case 'approval': return { graph: approval, layout: {} }
    default: return { graph: sample, layout: {} }
  }
}
const picked = pick()
// connection=lost: a Telegram step whose connection the host reports unavailable (validate diagnostic),
// and a step of a catalog type without any working connection (connected: false).
const initial = params.get('connection') === 'lost'
  ? {
    layout: picked.layout,
    graph: {
      ...picked.graph,
      nodes: [
        ...picked.graph.nodes,
        { id: 'notify_sales', type: 'telegram.send_message', params: { connection: 'conn_sales', chat: '@sales', text: 'New user {{ user.email }}' } },
        { id: 'classify', type: 'ai.classify', params: { text: 'hello' } },
      ],
    },
  }
  : picked

type Log = {
  replaceJsMs?: number
  mountMs?: number
  mountJsMs?: number
  saves: { base: number; graph: AutomationGraph }[]
  saveLayouts: AutomationLayout[]
  saveLayoutCalls: number
  openedVersions: number[]
  initialLayout: AutomationLayout
  validates: number
  moves: AutomationLayout[]
  changes: AutomationGraph[]
  reloads: number
  runCalls: number
  runsCalls: { state?: RunState[]; subject?: string }[]
  ops: string[]
  dryRuns: unknown[]
  tests: unknown[]
  graph: AutomationGraph
  layout: AutomationLayout
}
const log: Log = { saves: [], saveLayouts: [], saveLayoutCalls: 0, openedVersions: [], initialLayout: ensureLayout(initial.graph, catalog, initial.layout), validates: 0, moves: [], changes: [], reloads: 0, runCalls: 0, runsCalls: [], ops: [], dryRuns: [], tests: [], graph: initial.graph, layout: initial.layout }
window.__fixture = Object.assign(log, {
  failNext: (kind: string) => fail.add(kind),
  recover: (kind: string) => fail.delete(kind),
})

const wait = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms))

let serverVersion = initial.graph.nodes.length > 0 ? 3 : 0
const api: EditorApi = {
  async validate(graph) {
    log.validates += 1
    await wait(latency)
    if (fail.has('validate')) throw new Error('network')
    const errors: GraphError[] = []
    graph.nodes.forEach((node, index) => {
      if (node.type === 'branch' && !String(node.params.expression ?? '').trim()) errors.push({ path: `nodes[${index}].params.expression`, message: 'An expression is required.', code: 'required' })
      if (node.type === 'telegram.send_message' && node.params.connection === 'conn_sales') {
        errors.push({ path: `nodes[${index}].params.connection`, code: 'connection_unavailable', message: 'Connection “Sales bot” is unavailable: reconnect it in Integrations.' })
      }
      if (params.get('errors') === '1' && node.id === 'has_email') errors.push({ path: `nodes[${index}].params.expression`, message: 'Unknown field user.mail.', code: 'type' })
    })
    if (graph.triggers.length === 0) errors.push({ path: 'triggers', message: 'Add at least one trigger.', code: 'required' })
    return { ok: errors.length === 0, errors, warnings: graph.nodes.some((n) => n.type === 'end') ? [] : [{ path: 'graph', message: 'No step ends the run explicitly.' }] }
  },
  async dryRun(input) {
    log.dryRuns.push(input)
    await wait(latency)
    if (fail.has('dryrun')) throw new Error('network')
    return {
      trace: [
        { node: 'user_created', kind: 'triggered' },
        { node: 'schedule', kind: 'not_triggered', path: 'filter', message: 'event.hour == 20' },
        { node: 'has_email', kind: 'step', outcome: 'true' },
        { node: 'send_email', kind: 'step', outcome: 'sent', status: 'captured', effects: [{ to: 'dilnoza@acme.test', subject: 'Welcome, Dilnoza', body: 'Hello!' }] },
        ...(input.decisions === 'live' ? [{ node: 'send_email', kind: 'step', outcome: 'sent', status: 'executed' }] : []),
        { node: 'delay', kind: 'wait' },
      ],
      live_calls: input.decisions === 'live' ? 1 : 0,
    }
  },
  async testNode(input) {
    log.tests.push(input)
    const test = params.get('test')
    await wait(test === 'slow' ? 5000 : latency)
    if (test === 'secret') return { outcome: 'failed', duration_ms: 12, error: { reason: 'secret_missing', message: 'secret crm.token is not set' } }
    return { outcome: 'sent', duration_ms: 184, output: { message_id: 'm-42' }, logs: ['rendered the subject', 'token ***', '<script>alert(1)</script>'] }
  },
}

// ——— runs (#1347) ———
const runsMode = params.get('runs') ?? ''
const runs: RunData[] = runsMode === 'empty' ? [] : [...(initial.graph === approval ? [approvalWaiting] : []), ...sampleRuns()]
let polls = 0
const progress = (run: RunData): RunData => {
  // The live run moves on after two polls: the email is sent and the run waits on the delay.
  if (!run.run_id.endsWith('r1') || polls < 2) return run
  return {
    ...run, state: 'waiting', updated_at: '2026-11-20T09:01:00.000Z', current_nodes: ['delay'],
    tokens: [{ token_id: 'a', node: 'delay', state: 'waiting' }],
    steps: run.steps.map((s) => (s.node === 'send_email' ? { ...s, outcome: 'sent', ended_at: '2026-11-20T09:00:40.000Z', output: { message_id: 'm-77' } } : s)),
    waits: [{ node: 'delay', token_id: 'a', kind: 'timer', since: '2026-11-20T09:00:41.000Z', timeout_at: '2026-11-21T09:00:41.000Z' }],
  }
}
const noOps = runsMode === 'noop' || params.get('can') === 'read'
Object.assign(api, {
  async runs(input: { state?: RunState[]; subject?: string; version?: number }) {
    log.runsCalls.push({ ...(input.state ? { state: input.state } : {}), ...(input.subject ? { subject: input.subject } : {}) })
    await wait(latency)
    if (runsMode === 'error') throw new Error('network')
    const q = (input.subject ?? '').toLowerCase()
    const items = runs.filter((r) => (!input.version || r.version === input.version) && (!input.state || input.state.length === 0 || input.state.includes(r.state)) &&
      (!q || (r.subject?.key ?? '').toLowerCase().startsWith(q))).map(toSummary)
    return { items, next_cursor: null }
  },
  async run(id: string) {
    log.runCalls += 1
    polls += id.endsWith('r1') ? 1 : 0
    await wait(latency)
    const found = runs.find((r) => r.run_id === id)
    if (!found || runsMode === 'notfound') throw Object.assign(new Error('run not found'), { code: 'not_found' })
    return JSON.parse(JSON.stringify(progress(found))) as RunData
  },
  async version(v: number) {
    await wait(latency)
    return { graph: v === 2 ? sampleV2 : initial.graph, layout: {} }
  },
  ...(noOps ? {} : {
    async cancelRun(id: string) {
      log.ops.push(`cancel:${id}`)
      await wait(runsMode === 'slowops' ? 5000 : latency)
      const r = runs.find((x) => x.run_id === id)
      if (r) r.state = 'cancelled'
    },
    async retryRun(input: { run_id: string; wait_id?: string }) {
      log.ops.push(`retry:${input.run_id}:${input.wait_id ?? ''}`)
      await wait(runsMode === 'slowops' ? 5000 : latency)
      if (fail.has('retry')) throw new Error('network')
    },
    async resolveRun(input: { run_id: string; outcome: string; wait_id?: string }) {
      log.ops.push(`resolve:${input.run_id}:${input.outcome}`)
      await wait(latency)
      const r = runs.find((x) => x.run_id === input.run_id)
      if (r) { r.state = 'done'; r.waits = [] }
    },
  }),
})
const initialRunsView = (): RunsView => {
  const state = (params.get('state') ?? '').split(',').filter(Boolean) as RunState[]
  return { run: params.get('run'), node: params.get('node'), state, subject: params.get('subject') ?? '' }
}

const overlay: Overlay | null = params.get('overlay') === '1'
  ? { states: new Map([['user_created', 'passed'], ['has_email', 'passed'], ['send_email', 'failed']]), badges: new Map([['send_email', '2×']]), taken: new Set(['user_created.fired', 'has_email.true']) }
  : null

function App() {
  const [enabled, setEnabled] = createSignal(true)
  const [tab, setTab] = createSignal<'edit' | 'runs'>(params.get('tab') === 'runs' ? 'runs' : 'edit')
  const [runsView, setRunsView] = createSignal<RunsView>(initialRunsView())
  // The fixture keeps the Runs tab's state in the URL like a host does (D1).
  const writeURL = (tabNow: 'edit' | 'runs', view: RunsView, push: boolean) => {
    const url = new URL(location.href)
    const set = (k: string, v: string | null | undefined) => { if (v) url.searchParams.set(k, v); else url.searchParams.delete(k) }
    set('tab', tabNow === 'runs' ? 'runs' : null)
    set('run', view.run); set('node', view.node); set('state', (view.state ?? []).join(',')); set('subject', view.subject)
    if (push) history.pushState({}, '', url)
    else history.replaceState({}, '', url)
  }
  window.addEventListener('popstate', () => {
    const q = new URLSearchParams(location.search)
    setTab(q.get('tab') === 'runs' ? 'runs' : 'edit')
    setRunsView({ run: q.get('run'), node: q.get('node'), state: (q.get('state') ?? '').split(',').filter(Boolean) as RunState[], subject: q.get('subject') ?? '' })
  })
  if (params.get('loading') === '1') return <EditorSkeleton locale={locale} />
  if (params.get('mode') === 'canvas') {
    const [graph, setGraph] = createSignal(initial.graph)
    const [layout, setLayout] = createSignal(ensureLayout(initial.graph, catalog, initial.layout))
    const [selected, setSelected] = createSignal<string | null>(null)
    window.__fixture.setGraph = setGraph
    // Benchmark hook: replace the whole graph and layout at once, as an undo of many edits does.
    let flip = false
    window.__fixture.replace = () => {
      flip = !flip
      const next = flip ? bench(200, 500, 1) : initial
      const started = performance.now()
      batch(() => {
        setGraph(next.graph)
        setLayout(ensureLayout(next.graph, catalog, next.layout))
      })
      log.replaceJsMs = performance.now() - started
      void document.body.offsetHeight
      return performance.now() - started
    }
    return (
      <div style={{ position: 'relative', height: '100%' }}>
        <Canvas
          graph={graph()}
          catalog={catalog}
          layout={layout()}
          selected={selected()}
          onSelect={setSelected}
          onChange={params.get('readonly') === '1' ? undefined : (next) => {
            log.changes.push(next)
            setGraph(next)
          }}
          onMove={(positions) => {
            log.moves.push(positions)
            setLayout({ ...layout(), ...positions })
          }}
          minimap={params.get('minimap') !== '0'}
          locale={locale}
        />
      </div>
    )
  }
  return (
    <Editor
      graph={initial.graph}
      version={serverVersion}
      catalog={catalog}
      api={api}
      layout={initial.layout}
      locale={locale}
      enabled={enabled()}
      toggleEnabled={() => setEnabled((v) => !v)}
      readOnly={params.get('readonly') === '1' ? { reason: 'This is version 2; the current version is 3.' } : undefined}
      overlay={overlay}
      crumb={<a href="#automations">Automations</a>}
      tab={tab()}
      onTab={(next) => { setTab(next); writeURL(next, runsView(), true) }}
      automationId="auto-1"
      runsView={runsView()}
      onRunsView={(view, push) => { setRunsView(view); writeURL(tab(), view, push) }}
      can={params.get('can') === 'read' ? { author: false, operate: false } : { author: true, operate: true }}
      subjectHref={(subject) => `#${subject.entity}/${subject.key}`}
      connectionName={(id) => (id === 'conn_sales' ? 'Sales bot' : undefined)}
      {...(params.get('manual') === '1' ? { onStartRun: () => log.ops.push('start') } : {})}
      versions={[{ version: 3, enabled: true, by: 'Aziz' }, { version: 2, note: 'Before review' }, { version: 1 }]}
      onOpenVersion={(version) => log.openedVersions.push(version)}
      saveLayout={serverVersion > 0 ? async (positions) => {
        log.saveLayoutCalls += 1
        await wait(latency)
        if (fail.has('savelayout')) throw new Error('network')
        log.saveLayouts.push(positions)
        log.layout = positions
      } : undefined}
      save={async (graph, positions, base): Promise<SaveResult> => {
        log.saves.push({ base, graph })
        await wait(latency)
        if (fail.has('save')) throw new Error('network')
        if (params.get('conflict') === '1' && log.saves.length === 1) return { conflict: { by: 'Aziz', version: serverVersion + 1 } }
        serverVersion = Math.max(serverVersion, base) + 1
        log.graph = graph
        log.layout = positions
        return { ok: true, graph, version: serverVersion, pending_review: params.get('pending') === '1' }
      }}
      reload={async () => {
        log.reloads += 1
        await wait(latency)
        serverVersion = Math.max(serverVersion, 4)
        return { graph: { ...connected, name: 'Saved by Aziz' }, version: serverVersion, layout: {} }
      }}
      banner={<Show when={params.get('pending') === '1'}><a href="#review">Ask for review</a></Show>}
    />
  )
}

const started = performance.now()
render(() => <App />, document.getElementById('root')!)
log.mountJsMs = performance.now() - started
void document.body.offsetHeight
log.mountMs = performance.now() - started
document.documentElement.dataset.fixtureReady = 'true'
