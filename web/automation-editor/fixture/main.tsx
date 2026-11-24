// Fixture host for the editor: a fake EditorApi with switches in the query string, for manual checks,
// e2e and the benchmark. Query: graph=sample|connected|empty|bench, locale, theme=dark, readonly=1,
// latency=<ms>, fail=save|validate|savelayout, conflict=1, pending=1, overlay=1, catalog=empty|notify (graph=notify|notify_plain),
// mode=canvas (the Canvas alone, with counters), errors=1 (validate reports an error on has_email).
import { batch, createSignal, Show } from 'solid-js'
import { render } from 'solid-js/web'
import { Canvas } from '../src/Canvas'
import { Editor } from '../src/Editor'
import { EditorSkeleton } from '../src/lazy'
import { ensureLayout } from '../src/layout'
import type { AutomationGraph, AutomationLayout, EditorApi, EditorLocale, GraphError, Overlay, SaveResult } from '../src/types'
import { catalog as fullCatalog, emptyCatalog, notifyCatalog } from './catalog'
import { bench, connected, empty, notifyGraph, sample } from './graphs'
import './solid-ui.generated.css'
import '../src/styles.css'
import './fixture.css'

const params = new URLSearchParams(location.search)
const locale = (params.get('locale') ?? 'en') as EditorLocale
const latency = Number(params.get('latency') ?? '30')
const fail = new Set((params.get('fail') ?? '').split(',').filter(Boolean))
if (params.get('theme') === 'dark') document.documentElement.classList.add('dark')
document.documentElement.lang = locale

const catalog = params.get('catalog') === 'empty' ? emptyCatalog : params.get('catalog') === 'notify' ? notifyCatalog : fullCatalog
const pick = (): { graph: AutomationGraph; layout: AutomationLayout } => {
  switch (params.get('graph')) {
    case 'bench': return bench()
    case 'connected': return { graph: connected, layout: {} }
    case 'empty': return { graph: empty, layout: {} }
    case 'notify': return { graph: notifyGraph('acme.policy.created.v1'), layout: {} }
    case 'notify_plain': return { graph: notifyGraph('acme.policy.deleted.v1'), layout: {} }
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
  graph: AutomationGraph
  layout: AutomationLayout
}
const log: Log = { saves: [], saveLayouts: [], saveLayoutCalls: 0, openedVersions: [], initialLayout: ensureLayout(initial.graph, catalog, initial.layout), validates: 0, moves: [], changes: [], reloads: 0, graph: initial.graph, layout: initial.layout }
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
  async dryRun() {
    return { trace: [] }
  },
  async testNode() {
    return { outcome: 'ok' }
  },
}

const overlay: Overlay | null = params.get('overlay') === '1'
  ? { states: new Map([['user_created', 'passed'], ['has_email', 'passed'], ['send_email', 'failed']]), badges: new Map([['send_email', '2×']]), taken: new Set(['user_created.fired', 'has_email.true']) }
  : null

function App() {
  const [enabled, setEnabled] = createSignal(true)
  const [tab, setTab] = createSignal<'edit' | 'runs'>('edit')
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
      onTab={setTab}
      runsCount={12}
      runsBody={<p style={{ padding: '1rem' }}>Runs live in #1347.</p>}
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
