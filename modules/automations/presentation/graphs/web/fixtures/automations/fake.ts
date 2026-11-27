import { catalog as editorCatalog } from '../../../../../../../web/automation-editor/fixture/catalog'
import type { RPC } from '../../Automations.solid.generated'
import { BT_GRAPH, jevNode, messageEvent, reportOf, resultsOf, routeGraph, seedBacktests } from './backtests'

/**
 * An in-memory stand-in for the Go RPC (modules/automations/presentation/graphs): the same JSON-RPC
 * envelope, results and refusals, so the screens run unchanged. The Go tests own the real contract;
 * this fixture drives screenshots and browser scenarios.
 */
export interface FakeOptions {
  scenario: 'default' | 'empty' | 'loading' | 'error'
  /** Delay of every action (ms); lists answer at once. */
  delay: number
  /** Calls of these methods wait until the scenario releases them, oldest first (?holdOn=<method>,<method>;
   * window.__releaseCall(method), window.__heldCalls(method)). */
  holdOn?: string
  /** A call waits (held or delayed) after its answer is read, so it holds the state at its start (?staleReads). */
  staleReads?: boolean
  /** With delay: only these methods wait, lists among them (?delayOnly=<method>,<method>). */
  delayOnly?: string
  /** The first event of each backtest ran without its subject's snapshot (?withheld). */
  withheld?: boolean
  /** A label save of this outcome fails (?labelFails=<outcome>). */
  labelFails?: string
  /** The code of the graph changes once between the review and the enable (a digest refusal). */
  changeOnce: boolean
  /** The first layout save meets a newer revision (another tab). */
  layoutConflict: boolean
  /** The first save meets a version saved by another user. */
  conflictOnce: boolean
  /** The first count of live runs fails (the confirmation offers a retry). */
  liveFailsOnce: boolean
  /** The answer of the first node test is lost after the test ran (the retry must not run it again). */
  testLostOnce: boolean
  /** The keys of a page of the graph's memory (0: the request's limit). */
  statePage?: number
  /** The review after a digest refusal fails once (a network error). */
  reviewFailsAfterChange: boolean
  /** Delay of automations.get for an old version (ms): a request still on the way when Back is pressed. */
  oldVersionDelay: number
  /** Every automations.get after the first waits until the scenario releases it (window.__release). */
  hold: boolean
  /** With hold: the first automations.get (the page's initial load) is held too (?hold=all). */
  holdFirst: boolean
  can: { author: boolean; enable: boolean; operate: boolean }
  /** The session is an agent's. */
  agent?: boolean
  calls: { method: string; params: unknown }[]
  /** Validation warns about the first node (a pinned tool schema the server changed), with its fix. */
  warn?: boolean
  /** «Close stale deals» sends a Telegram message whose chat is picked from the connection's known chats. */
  telegram?: boolean
  /** The backtests of BT_GRAPH: seeded, none yet, or the section's read fails. */
  backtests?: 'default' | 'empty' | 'error'
}

const now = '2026-11-20T09:30:00Z'
type ListItem = RPC['automations.list']['result']['items'][number]
type Graph = { name: string; description?: string; run_as: 'owner' | 'initiator'; triggers: { id: string; event: string; filter?: string; params?: Record<string, unknown> }[]; nodes: { id: string; type: string; params: Record<string, unknown> }[]; edges: { from: { node: string; outcome: string }; to: string }[] }

interface Stored {
  item: ListItem
  versions: { version: number; graph: Graph; by: string; origin: 'human' | 'agent'; at: string }[]
  layout: Record<string, { x: number; y: number }>
  revision: number
  live: { runs: number; waits: number }
  code: { node_id: string; source: string; secrets: string[]; fetch: boolean }[]
}

const manualEvent = 'core.manual'
const catalog = {
  ...editorCatalog,
  events: editorCatalog.events.map((e) => (e.key === manualEvent ? { ...e, title: 'Manual start' } : e)),
  subjects: ['core.user', 'crm.client'],
}

function digestGraph(text: string): Graph {
  return {
    name: 'Daily sales digest', run_as: 'owner',
    triggers: [{ id: 'every_day', event: 'core.schedule', params: { cron: '0 20 * * *', timezone: 'Asia/Tashkent' } }],
    nodes: [
      { id: 'email', type: 'core.send_email', params: { to: 'sales@acme.test', subject: 'Digest', body: text } },
      { id: 'fin', type: 'end', params: {} },
    ],
    edges: [
      { from: { node: 'every_day', outcome: 'fired' }, to: 'email' },
      { from: { node: 'email', outcome: 'sent' }, to: 'fin' },
      { from: { node: 'email', outcome: 'failed' }, to: 'fin' },
    ],
  }
}

const welcome: Graph = {
  name: 'Welcome new users', run_as: 'owner',
  triggers: [
    { id: 'created', event: 'core.user.created.v1' },
    { id: 'by_hand', event: manualEvent, params: { input_schema: { type: 'object', required: ['email'], properties: { email: { type: 'string', title: 'Email' }, vip: { type: 'boolean', title: 'VIP' } } } } },
  ],
  nodes: [{ id: 'fin', type: 'end', params: {} }],
  edges: [{ from: { node: 'created', outcome: 'fired' }, to: 'fin' }, { from: { node: 'by_hand', outcome: 'fired' }, to: 'fin' }],
}

// The Telegram node's chat names the connection's known chats as its options source (x-options-source).
const telegramCatalog = {
  ...catalog,
  nodes: catalog.nodes.map((n) => (n.type !== 'telegram.send_message' ? n : {
    ...n,
    params: { ...n.params, properties: { ...(n.params.properties as Record<string, unknown>), chat: { oneOf: [{ type: 'integer' }, { type: 'string', minLength: 1 }], title: 'Chat', 'x-options-source': 'telegram.chats' } } },
  })),
}

const telegramGraph: Graph = {
  name: 'Close stale deals', run_as: 'owner',
  triggers: [{ id: 'by_hand', event: manualEvent }],
  nodes: [{ id: 'notify', type: 'telegram.send_message', params: { connection: 'c-1', text: 'A deal is stale' } }, { id: 'fin', type: 'end', params: {} }],
  edges: [{ from: { node: 'by_hand', outcome: 'fired' }, to: 'notify' }, { from: { node: 'notify', outcome: 'ok' }, to: 'fin' }, { from: { node: 'notify', outcome: 'failed' }, to: 'fin' }],
}

/** The chats a bot knows, by connection: what automations.search_options answers. */
const knownChats: Record<string, { value: string; label: string; group?: string }[]> = {
  'c-1': [
    { value: '501', label: 'Alice Karimova (@alice)', group: 'private' },
    { value: '502', label: 'Bobur Aliev (@bobur)', group: 'private' },
    { value: '-1001', label: 'Sales team', group: 'group' },
  ],
}

function seed(telegram = false): Stored[] {
  return [
    {
      item: {
        id: '00000000-0000-4000-8000-0000000000a1', name: 'Daily sales digest', description: 'Sends the day\'s sales to the sales team.',
        enabled: true, enabled_version: 2, latest_version: 3, pending_version: true, saved_by_agent: true,
        triggers: [{ kind: 'schedule', title: 'On a schedule · 0 20 * * * (Asia/Tashkent)' }],
        last_run: { run_id: '00000000-0000-4000-8000-0000000000f1', status: 'failed', at: '2026-11-20T09:00:00Z' },
        attention: 2, updated_at: '2026-11-20T08:00:00Z', updated_by: 'Sales assistant',
      },
      versions: [
        { version: 1, graph: digestGraph('v1'), by: 'Dilnoza Karimova', origin: 'human', at: '2026-11-18T08:00:00Z' },
        { version: 2, graph: digestGraph('v2'), by: 'Dilnoza Karimova', origin: 'human', at: '2026-11-19T08:00:00Z' },
        { version: 3, graph: digestGraph('v3'), by: 'Sales assistant', origin: 'agent', at: '2026-11-20T08:00:00Z' },
      ],
      layout: { every_day: { x: 48, y: 96 }, email: { x: 368, y: 96 }, fin: { x: 688, y: 96 } },
      revision: 4,
      live: { runs: 3, waits: 2 },
      code: [{
        node_id: 'format', fetch: true, secrets: ['crm.api_token', 'telegram.bot_token'],
        source: '// Formats the digest\nexport default async function run(ctx) {\n  const total = ctx.orders.reduce((s, o) => s + o.amount, 0)\n  const res = await fetch("https://api.acme.test/rates")\n  return { text: `Total: ${total}`, rate: (await res.json()).usd }\n}\n',
      }],
    },
    {
      item: {
        id: '00000000-0000-4000-8000-0000000000a2', name: 'Welcome new users', description: null,
        enabled: true, enabled_version: 1, latest_version: 1, pending_version: false, saved_by_agent: false,
        triggers: [{ kind: 'event', title: 'User created' }, { kind: 'manual', title: 'Manual start' }],
        last_run: { run_id: '00000000-0000-4000-8000-0000000000f2', status: 'done', at: '2026-11-20T09:25:00Z' },
        attention: 0, updated_at: '2026-11-19T12:00:00Z', updated_by: 'Aziz Rakhimov',
      },
      versions: [{ version: 1, graph: welcome, by: 'Aziz Rakhimov', origin: 'human', at: '2026-11-19T12:00:00Z' }],
      layout: { created: { x: 48, y: 96 }, by_hand: { x: 48, y: 224 }, fin: { x: 368, y: 160 } },
      revision: 1,
      live: { runs: 0, waits: 0 },
      code: [],
    },
    {
      item: {
        id: '00000000-0000-4000-8000-0000000000a3', name: 'Close stale deals', description: null,
        enabled: false, enabled_version: null, latest_version: 2, pending_version: false, saved_by_agent: false,
        triggers: [{ kind: 'inbound', title: 'Telegram message received' }],
        last_run: null, attention: 0, updated_at: '2026-11-10T07:00:00Z', updated_by: 'Aziz Rakhimov',
      },
      versions: [{ version: 2, graph: telegram ? telegramGraph : { ...welcome, name: 'Close stale deals' }, by: 'Aziz Rakhimov', origin: 'human', at: '2026-11-10T07:00:00Z' }],
      layout: telegram ? { by_hand: { x: 48, y: 96 }, notify: { x: 368, y: 96 }, fin: { x: 688, y: 96 } } : {},
      revision: 0,
      live: { runs: 0, waits: 0 },
      code: [],
    },
  ]
}

type RunData = RPC['automations.run']['result']
type WireState = RunData['run']['state']

function wireRun(id: string, automation: string, version: number, state: WireState, o: { trigger?: string; nodes?: string[]; subject?: { entity: string; key: string }; error?: string; at?: string } = {}): RunData {
  return {
    run: {
      run_id: id, automation_id: automation, version, state, status: state, trigger_kind: 'schedule', trigger_id: o.trigger ?? 'every_day',
      subject: o.subject ?? { entity: 'crm.client', key: 'c-7' }, current_nodes: o.nodes ?? [],
      error: o.error ? { code: 'effect_failed', message: o.error } : null, created_at: o.at ?? now, updated_at: o.at ?? now,
    },
    tokens: [{ id: 'tk-1', parent: null, node_id: o.nodes?.[0] ?? 'fin', status: state === 'running' ? 'running' : state === 'blocked' || state === 'waiting' ? 'waiting' : 'done', foreach_index: null }],
    steps: [],
    waits: [],
  }
}

/** Runs of the digest (on v3: a failed one and a blocked call; on v2: a done one) and of the welcome graph. */
function seedRuns(): RunData[] {
  const digest = '00000000-0000-4000-8000-0000000000a1'
  const failed = wireRun('00000000-0000-4000-8000-0000000000f1', digest, 3, 'failed', { error: 'The mail server refused the message', at: '2026-11-20T09:00:00Z' })
  failed.steps = [{
    token_id: 'tk-1', seq: 1, node_id: 'email', attempt: 2, input: { to: 'sales@acme.test', subject: 'Digest' }, outcome: 'failed',
    output: { error: { message: 'The mail server refused the message' } }, error: { code: 'effect_failed', message: 'The mail server refused the message' },
    started_at: '2026-11-20T09:00:00Z', ended_at: '2026-11-20T09:00:02Z',
  }]
  const blocked = wireRun('00000000-0000-4000-8000-0000000000f3', digest, 3, 'blocked', { nodes: ['email'], at: '2026-11-20T08:00:00Z', subject: { entity: 'crm.client', key: 'c-9' } })
  blocked.waits = [{ token_id: 'tk-1', node_id: 'email', wait_id: 'call-1', kind: 'call', timeout_at: null, status: 'open' }]
  const done = wireRun('00000000-0000-4000-8000-0000000000f2', '00000000-0000-4000-8000-0000000000a2', 1, 'done', { trigger: 'created', at: '2026-11-20T09:25:00Z', subject: { entity: 'core.user', key: '42' } })
  done.steps = [{ token_id: 'tk-1', seq: 1, node_id: 'fin', attempt: 1, outcome: 'done', error: null, started_at: '2026-11-20T09:25:00Z', ended_at: '2026-11-20T09:25:00Z' }]
  const old = wireRun('00000000-0000-4000-8000-0000000000f5', digest, 2, 'done', { at: '2026-11-20T07:00:00Z', subject: { entity: 'crm.client', key: 'c-3' } })
  old.steps = [{ token_id: 'tk-1', seq: 1, node_id: 'email', attempt: 1, outcome: 'sent', output: { message_id: 'm-0' }, error: null, started_at: '2026-11-20T07:00:00Z', ended_at: '2026-11-20T07:00:01Z' }]
  return [failed, blocked, old, done]
}

function rpcError(code: string, message: string): never {
  throw { code, message, data: { code } }
}

export function installFake(rpcPath: string, opts: FakeOptions): void {
  const store = opts.scenario === 'empty' ? [] : seed(opts.telegram)
  const shown = opts.telegram ? telegramCatalog : catalog
  let changePending = opts.changeOnce
  let layoutConflict = opts.layoutConflict
  let conflictOnce = opts.conflictOnce
  let runSeq = 0
  let liveFails = opts.liveFailsOnce
  let reviewFails = false
  // The graph's memory between runs (#1390): what its Set state nodes keep.
  const memory = new Map<string, RPC['automations.state_list']['result']['items'][number]>([
    ['chat:42', { key: 'chat:42', value: { task: 'T-17', opened: '2026-11-19' }, expires_at: '2026-11-27T10:00:00Z', updated_at: '2026-11-20T09:00:00Z', updated_by_run: '00000000-0000-4000-8000-0000000000f1' }],
    ['release:v2.4', { key: 'release:v2.4', value: true, expires_at: null, updated_at: '2026-11-18T08:30:00Z', updated_by_run: '00000000-0000-4000-8000-0000000000f2' }],
    ['rule:daily:count', { key: 'rule:daily:count', value: 3, expires_at: '2026-11-21T00:00:00Z', updated_at: '2026-11-20T09:55:00Z', updated_by_run: null }],
  ])
  const runs: RunData[] = opts.scenario === 'empty' ? [] : seedRuns()
  const tests = new Map<string, RPC['automations.test_node']['result']>()
  let testRuns = 0
  let testLost = opts.testLostOnce
  // The graph with a classifier the backtests run on: reachable by id, not in the list (the list's baseline stays).
  const hidden: Stored[] = [{
    item: { id: BT_GRAPH, name: 'Route client messages', description: null, enabled: true, enabled_version: 2, latest_version: 3, pending_version: true, saved_by_agent: false,
      triggers: [{ kind: 'inbound', title: 'Telegram message received' }], last_run: null, attention: 0, updated_at: now, updated_by: 'Dilnoza Karimova' },
    versions: [1, 2, 3].map((v) => ({ version: v, graph: routeGraph(v) as Graph, by: 'Dilnoza Karimova', origin: 'human' as const, at: `2026-11-1${v}T08:00:00Z` })),
    layout: { msg: { x: 48, y: 96 }, intent: { x: 368, y: 96 }, to_sales: { x: 688, y: 32 }, to_support: { x: 688, y: 192 }, fin: { x: 1008, y: 96 } }, revision: 1, live: { runs: 0, waits: 0 }, code: [],
  }]
  // The message event replaces the base catalog's own (an empty payload) under the same key.
  const btCatalog = { ...catalog, events: [...catalog.events.filter((e) => e.key !== messageEvent.key), messageEvent], nodes: [...catalog.nodes, jevNode] }
  const bt = seedBacktests(opts.backtests === 'empty')
  bt.withheld = opts.withheld
  let btSeq = 0
  const backtestOf = (id: string) => bt.backtests.find((b) => b.id === id) ?? rpcError('not_found', 'not found')
  const datasetOf = (id: string) => bt.datasets.find((d) => d.id === id) ?? rpcError('not_found', 'not found')
  // The revision of an estimate, as the server's: the examples by id (a dataset seeded without its rows by
  // their count) and the version.
  const revisionOf = (d: { id: string; examples: number }, version: number) => {
    const ids = (bt.examples.get(d.id) ?? []).map((x) => x.id).sort()
    return `rev-${ids.length > 0 ? ids.join(',') : `n${d.examples}`}.v${version || 3}`
  }
  const find = (id: string) => store.find((s) => s.item.id === id) ?? hidden.find((s) => s.item.id === id) ?? rpcError('not_found', 'automation not found')
  const digestOf = (s: Stored) => (s.code.length > 0 ? `digest-${s.item.latest_version}-${s.code.map((c) => c.source.length).join('.')}` : '')
  const handlers: { [K in keyof RPC]?: (p: RPC[K]['params']) => RPC[K]['result'] } = {
    // #1389: the section «Backtests».
    'automations.backtests': (p) => {
      find(p.automation_id)
      if (opts.backtests === 'error') rpcError('internal', 'internal error')
      return { datasets: bt.datasets, backtests: bt.backtests, sources: [{ kind: 'connection', connection_id: 'c-1', title: 'Sales bot', connector_kind: 'telegram' }], live: !opts.agent, decision_nodes: p.automation_id === BT_GRAPH ? ['intent'] : [] }
    },
    'automations.backtest_count': (p) => ({ count: p.automation_id === BT_GRAPH ? bt.backtests.length : 0 }),
    'automations.dataset_create': (p) => {
      if (bt.datasets.length >= 20) rpcError('limit_datasets', 'an automation has at most 20 datasets')
      const id = `00000000-0000-4000-8000-0000000d${String(++btSeq).padStart(4, '0')}`
      bt.datasets.unshift({ id, name: p.name, description: p.description ?? '', examples: 0, labeled: 0, created_by: 'You', created_at: now, imports: [] })
      bt.examples.set(id, [])
      return { id }
    },
    'automations.dataset_delete': (p) => { bt.datasets = bt.datasets.filter((d) => d.id !== p.dataset_id); bt.backtests = bt.backtests.filter((b) => b.dataset_id !== p.dataset_id); return { ok: true } },
    'automations.dataset_import': (p) => {
      const d = datasetOf(p.dataset_id)
      const id = `i-${++btSeq}`
      d.imports = [{ id, kind: p.kind, status: 'running', imported: 0, duplicates: 0, overflow: 0, unreadable: 0, error: null, created_at: now, finished_at: null }, ...d.imports]
      return { id }
    },
    'automations.dataset_examples': (p) => {
      const items = bt.examples.get(p.dataset_id) ?? []
      return { items, next_cursor: null, total: items.length }
    },
    'automations.dataset_synthetic': (p) => {
      const d = datasetOf(p.dataset_id)
      const items = bt.examples.get(d.id) ?? []
      const id = `00000000-0000-4000-8000-0000000c${String(++btSeq).padStart(4, '0')}`
      const labels: Record<string, { outcome: string; source: 'expected' }> = {}
      for (const [node, outcome] of Object.entries(p.expected ?? {})) labels[node] = { outcome, source: 'expected' }
      items.push({ id, source: 'synthetic', title: p.title ?? '', event_kind: p.kind, event: { kind: p.kind, ...(p.payload as object) }, subject: { entity: 'conversation', key: '' },
        snapshot_approximate: false, has_snapshot: false, occurred_at: now, labels })
      bt.examples.set(d.id, items)
      d.examples = items.length
      d.labeled = items.filter((x) => Object.keys(x.labels).length > 0).length
      return { id }
    },
    'automations.example_delete': (p) => {
      for (const [ds, items] of bt.examples) {
        const left = items.filter((x) => x.id !== p.example_id)
        if (left.length === items.length) continue
        bt.examples.set(ds, left)
        const d = datasetOf(ds)
        d.examples = left.length
        d.labeled = left.filter((x) => Object.keys(x.labels).length > 0).length
      }
      return { ok: true }
    },
    'automations.example_label': (p) => {
      if (opts.labelFails && p.outcome === opts.labelFails) rpcError('internal', 'internal error')
      for (const [ds, items] of bt.examples) {
        const ex = items.find((x) => x.id === p.example_id)
        if (!ex) continue
        if (p.outcome) ex.labels = { ...ex.labels, [p.node]: { outcome: p.outcome, source: 'manual' } }
        else { const { [p.node]: _, ...rest } = ex.labels; ex.labels = rest }
        const d = datasetOf(ds)
        d.labeled = items.filter((x) => Object.keys(x.labels).length > 0).length
      }
      return { ok: true }
    },
    'automations.backtest_estimate': (p) => {
      const d = datasetOf(p.dataset_id)
      return { examples: d.examples, revision: revisionOf(d, p.version ?? 0), decision_nodes: ['intent'], max_calls: d.examples, per_item: false, cost_usd: p.decisions === 'live' ? d.examples * 0.0021 : null }
    },
    'automations.backtest_run': (p) => {
      const d = datasetOf(p.dataset_id)
      if (p.decisions === 'live' && opts.agent) rpcError('HUMAN_REQUIRED', 'only a person runs a backtest with live decisions')
      if (p.revision && p.revision !== revisionOf(d, p.version ?? 0)) rpcError('estimate_stale', 'the dataset changed since the estimate')
      const id = `00000000-0000-4000-8000-0000000e${String(++btSeq).padStart(4, '0')}`
      bt.backtests.unshift({ id, dataset_id: d.id, dataset_name: d.name, version: p.version || 3, decisions: p.decisions === 'live' ? 'live' : 'cached', status: 'queued', total: d.examples, done: 0,
        live_calls: 0, error: null, created_by: 'You', by_agent: opts.agent ?? false, created_at: now, finished_at: null })
      return { id }
    },
    'automations.backtest_cancel': (p) => {
      const b = backtestOf(p.backtest_id)
      if (b.status !== 'queued' && b.status !== 'running') rpcError('backtest_finished', 'the backtest has finished')
      b.status = 'cancelled'
      return { ok: true }
    },
    'automations.backtest_report': (p) => reportOf(bt, backtestOf(p.backtest_id)),
    'automations.backtest_results': (p) => {
      let items = resultsOf(bt, backtestOf(p.backtest_id))
      if (p.node) items = items.filter((r) => r.example.labels[p.node!])
      if (p.mismatch) items = items.filter((r) => (p.node ? r.mismatches.includes(p.node) : r.mismatches.length > 0))
      return { items, next_cursor: null, total: items.length }
    },
    'automations.backtest_compare': (p) => {
      const a = backtestOf(p.a)
      const b = backtestOf(p.b)
      const ra = resultsOf(bt, a)
      const rb = resultsOf(bt, b)
      const diffs = ra.flatMap((x, i) => {
        const y = rb[i]
        if (!y || x.outcomes.intent === y.outcomes.intent) return []
        return [{ example_id: x.example.id, title: x.example.title || (x.example.event as { text: string }).text, occurred_at: x.example.occurred_at, end_a: x.end, end_b: y.end,
          changes: [{ node: 'intent', a: x.outcomes.intent!, b: y.outcomes.intent!, ...(x.example.labels.intent ? { expected: x.example.labels.intent.outcome } : {}) }],
          effects_a: x.effects.map((e) => e.node), effects_b: y.effects.map((e) => e.node) }]
      })
      return { a: reportOf(bt, a), b: reportOf(bt, b), common_examples: Math.min(ra.length, rb.length), changed: diffs.length, diffs }
    },
    'automations.list': (p) => {
      let items = store.map((s) => s.item)
      if (p.query) items = items.filter((x) => x.name.toLowerCase().includes(p.query!.toLowerCase()))
      if (p.status === 'enabled') items = items.filter((x) => x.enabled)
      if (p.status === 'disabled') items = items.filter((x) => !x.enabled)
      if (p.status === 'attention') items = items.filter((x) => x.attention > 0)
      return { items, next_cursor: null, can: opts.can }
    },
    'automations.catalog': () => ({ catalog: shown as never, connections: { telegram: [{ id: 'c-1', name: 'Sales bot' }] }, can: opts.can }),
    'automations.get': (p) => {
      const s = find(p.id)
      const v = s.versions.find((x) => x.version === (p.version || s.item.latest_version)) ?? s.versions[s.versions.length - 1]!
      return {
        automation: { id: s.item.id, name: s.item.name, enabled: s.item.enabled, enabled_version: s.item.enabled_version, latest_version: s.item.latest_version, owner: { user_id: 1, name: 'Dilnoza Karimova' }, layout: s.layout, layout_revision: s.revision },
        graph: v.graph, version: v.version, saved_by: { name: v.by, origin: v.origin },
        versions: [...s.versions].reverse().map((x) => ({ version: x.version, enabled: x.version === s.item.enabled_version, by: x.by, origin: x.origin, at: x.at })),
        catalog: (s.item.id === BT_GRAPH ? btCatalog : shown) as never, connections: { telegram: [{ id: 'c-1', name: 'Sales bot' }] },
        read_only: !opts.can.author ? { reason: 'You may not change automations', code: 'no_author' } : v.version !== s.item.latest_version ? { reason: 'An old version is open', code: 'old_version' } : null,
        can: opts.can,
        manual_triggers: s.item.enabled_version === null ? [] : (s.versions.find((x) => x.version === s.item.enabled_version) ?? v).graph.triggers
          .filter((tr) => tr.event === manualEvent)
          .map((tr) => ({ trigger_id: tr.id, title: 'Manual start', ...(tr.params?.input_schema ? { input_schema: tr.params.input_schema as Record<string, unknown> } : {}) })),
      }
    },
    'automations.create': (p) => {
      const id = `00000000-0000-4000-8000-${String(store.length + 1).padStart(12, '0')}`
      const g = p.graph as Graph
      store.unshift({
        item: { id, name: g.name, description: null, enabled: false, enabled_version: null, latest_version: 1, pending_version: false, saved_by_agent: false, triggers: [], last_run: null, attention: 0, updated_at: now, updated_by: 'You' },
        versions: [{ version: 1, graph: g, by: 'You', origin: 'human', at: now }], layout: p.layout, revision: 1, live: { runs: 0, waits: 0 }, code: [],
      })
      return { ok: true, id, graph: g, version: 1, layout_revision: 1 }
    },
    'automations.save': (p) => {
      const s = find(p.id)
      if (conflictOnce) {
        conflictOnce = false
        return { ok: false, conflict: { by: 'Aziz Rakhimov', version: s.item.latest_version } }
      }
      if (p.base !== s.item.latest_version) return { ok: false, conflict: { by: s.versions[s.versions.length - 1]!.by, version: s.item.latest_version } }
      const version = s.item.latest_version + 1
      s.versions.push({ version, graph: p.graph as Graph, by: 'You', origin: 'human', at: now })
      s.item.latest_version = version
      s.item.name = (p.graph as Graph).name
      s.revision++
      const pending = s.item.enabled && s.code.length > 0
      if (s.item.enabled && !pending) s.item.enabled_version = version
      s.item.pending_version = pending
      return { ok: true, graph: p.graph, version, pending_review: pending, layout_revision: s.revision }
    },
    'automations.save_layout': (p) => {
      const s = find(p.id)
      if (layoutConflict) {
        layoutConflict = false
        s.revision++
        s.layout = { ...s.layout, email: { x: 368, y: 320 } }
        return { ok: false, error: 'VERSION_CONFLICT', revision: s.revision, positions: s.layout }
      }
      if (p.base_revision !== s.revision) return { ok: false, error: 'VERSION_CONFLICT', revision: s.revision, positions: s.layout }
      s.layout = { ...s.layout, ...p.positions }
      s.revision++
      return { ok: true, revision: s.revision }
    },
    'automations.validate': () => ({
      ok: true, errors: [],
      warnings: opts.warn ? [{ path: 'nodes[0].params.tool_schema', code: 'tool_schema_changed', message: 'the server changed the schema of weather since it was pinned: runs keep the pinned one; update the pin to use the new schema', fix: { remove: 'nodes[0].params.tool_schema', label: 'Update the pinned schema' } }] : [],
    }),
    'automations.enable_review': (p) => {
      const s = find(p.id)
      if (reviewFails) { reviewFails = false; rpcError('internal', 'internal error') }
      return {
        version: p.version || s.item.latest_version,
        code_nodes: s.code.map((c) => ({ ...c, title: 'Format the digest', lang: 'ts' as const })),
        triggers: s.item.triggers.map((x, i) => ({ trigger_id: `t${i}`, kind: x.kind, title: x.title, event: null, schedule: null })),
        run_as: 'owner', code_digest: digestOf(s), live: s.live,
        // The stale graph listens to a connector the tenant has no working connection of.
        problems: s.item.id.endsWith('a3') ? [{ path: 'triggers[0].event', code: 'connection_unavailable', message: 'telegram.message.v1 needs a connection' }] : [],
      }
    },
    'automations.enable': (p) => {
      const s = find(p.id)
      if (!opts.can.enable) return { ok: false, error: 'forbidden' }
      if (changePending) {
        changePending = false
        s.item.latest_version++
        s.versions.push({ ...s.versions[s.versions.length - 1]!, version: s.item.latest_version, origin: 'agent' })
        s.code = s.code.map((c) => ({ ...c, source: c.source + '// changed by the agent\n' }))
        reviewFails = opts.reviewFailsAfterChange
        return { ok: false, error: 'review_required' }
      }
      if (p.version !== s.item.latest_version || p.code_digest !== digestOf(s)) return { ok: false, error: 'review_required' }
      s.item.enabled = true
      s.item.enabled_version = p.version
      s.item.pending_version = false
      s.item.saved_by_agent = false
      return { ok: true, enabled_version: p.version }
    },
    'automations.disable': (p) => {
      const s = find(p.id)
      const cancelled = s.live.runs
      s.item.enabled = false
      s.item.enabled_version = null
      s.item.pending_version = false
      s.live = { runs: 0, waits: 0 }
      return { ok: true, cancelled_runs: cancelled }
    },
    'automations.delete': (p) => {
      const i = store.findIndex((s) => s.item.id === p.id)
      if (i < 0) rpcError('not_found', 'automation not found')
      store.splice(i, 1)
      return { ok: true }
    },
    'automations.live': (p) => {
      if (liveFails) { liveFails = false; rpcError('internal', 'internal error') }
      return find(p.id).live
    },
    'automations.run_manual': (p) => {
      const s = find(p.id)
      if (!s.item.enabled) return { ok: false, error: 'not_enabled' }
      runSeq++
      const run = `00000000-0000-4000-8000-${String(900 + runSeq).padStart(12, '0')}`
      s.item.last_run = { run_id: run, status: 'running', at: now }
      runs.unshift(wireRun(run, s.item.id, s.item.enabled_version ?? 1, 'running', { trigger: p.trigger_id, nodes: ['fin'] }))
      return { ok: true, run_id: run }
    },
    'automations.search_options': (p) => ({
      options: (knownChats[p.connection] ?? []).filter((o) => o.label.toLowerCase().includes(p.query.toLowerCase())),
    }),
    // «many» finds six subjects: more than five choices are a searchable Combobox in the run dialog (K5).
    'automations.search_subjects': (p) => p.query === 'many'
      ? Array.from({ length: 6 }, (_, i) => ({ subject: { entity: p.entity, key: String(40 + i) }, label: `Subject ${i + 1}` }))
      : [{ subject: { entity: p.entity, key: '42' }, label: `${p.query} (42)` }],
    'automations.runs': (p) => {
      find(p.automation_id)
      const q = (p.subject ?? '').toLowerCase()
      const items = runs
        .filter((r) => r.run.automation_id === p.automation_id && (!p.version || r.run.version === p.version) && (!p.state || p.state.length === 0 || p.state.includes(r.run.state)))
        .filter((r) => !q || r.run.subject.key.toLowerCase().startsWith(q) || (r.run.subject.label ?? '').toLowerCase().includes(q))
        .filter((r) => !p.node || r.steps.some((st) => st.node_id === p.node))
        .map((r) => r.run)
      return { items, next_cursor: null }
    },
    'automations.run': (p) => {
      const r = runs.find((x) => x.run.run_id === p.run_id) ?? rpcError('not_found', 'run not found')
      return JSON.parse(JSON.stringify(r)) as RunData
    },
    'automations.run_cancel': (p) => {
      const r = runs.find((x) => x.run.run_id === p.run_id) ?? rpcError('not_found', 'run not found')
      if (['done', 'failed', 'cancelled'].includes(r.run.state)) return { ok: false, error: 'run_finished' }
      r.run.state = 'cancelled'
      r.run.status = 'cancelled'
      r.waits = []
      return { ok: true }
    },
    'automations.run_retry': (p) => {
      const r = runs.find((x) => x.run.run_id === p.run_id) ?? rpcError('not_found', 'run not found')
      if (!r.waits.some((w) => w.wait_id === p.wait_id)) rpcError('invalid_input', 'the call is not blocked')
      r.run.state = 'running'
      r.run.status = 'running'
      r.waits = []
      return { ok: true }
    },
    'automations.run_resolve': (p) => {
      const r = runs.find((x) => x.run.run_id === p.run_id) ?? rpcError('not_found', 'run not found')
      if (!r.waits.some((w) => w.wait_id === p.wait_id)) rpcError('invalid_input', 'the call is not blocked')
      r.steps.push({ token_id: 'tk-1', seq: r.steps.length + 1, node_id: 'email', attempt: 2, outcome: p.outcome, error: null, started_at: now, ended_at: now })
      r.run.state = 'done'
      r.run.status = 'done'
      r.waits = []
      return { ok: true }
    },
    'automations.dry_run': () => ({
      trace: [
        { node: 'every_day', kind: 'triggered' },
        { node: 'email', kind: 'step', outcome: 'sent', status: 'captured', effects: [{ to: 'sales@acme.test', subject: 'Digest' }] },
        { node: 'fin', kind: 'ended' },
      ],
      live_calls: 0,
    }),
    // As the server: a repeat with the same key gets the first answer, nothing runs twice.
    'automations.test_node': (p) => {
      const known = p.idempotency_key ? tests.get(p.idempotency_key) : undefined
      if (known) return known
      testRuns++
      const res = { outcome: 'sent', duration_ms: 120, output: { message_id: `m-${testRuns}` }, logs: ['sent'] }
      if (p.idempotency_key) tests.set(p.idempotency_key, res)
      if (testLost) {
        testLost = false
        rpcError('internal', 'the connection dropped')
      }
      return res
    },
    'automations.state_list': (p) => {
      find(p.id)
      const q = (p.search ?? '').toLowerCase()
      const all = [...memory.values()].filter((e) => e.key.toLowerCase().includes(q)).sort((a, b) => (a.key < b.key ? -1 : 1))
      const after = p.cursor ? all.filter((e) => e.key > p.cursor!) : all
      const limit = opts.statePage || (p.limit ?? 100)
      const items = after.slice(0, limit)
      return { items, next_cursor: after.length > limit ? items[items.length - 1]!.key : null, total: memory.size }
    },
    'automations.state_set': (p) => {
      find(p.id)
      if (JSON.stringify(p.value).length > 16384) rpcError('invalid_input', 'state_value_too_large: the value has more than 16384 bytes')
      const entry = { key: p.key, value: p.value, expires_at: p.ttl ? '2026-11-20T12:00:00Z' : null, updated_at: '2026-11-20T10:00:00Z', updated_by_run: null }
      memory.set(p.key, entry)
      return { entry }
    },
    'automations.state_delete': (p) => {
      find(p.id)
      if (!memory.delete(p.key)) rpcError('not_found', 'the key is not found')
      return { ok: true }
    },
  }
  // Held answers: the scenario sees which versions are on the way (__held), releases one (__release)
  // and reads which were aborted by the page (__aborted). 0 is the latest.
  const held: { version: number; release: () => void }[] = []
  const aborted: number[] = []
  let gets = 0
  const w = window as unknown as { __held: () => number[]; __release: (v?: number) => boolean; __aborted: number[] }
  w.__held = () => held.map((h) => h.version)
  w.__release = (v) => {
    const i = v === undefined ? 0 : held.findIndex((h) => h.version === v)
    const h = i >= 0 ? held.splice(i, 1)[0] : undefined
    h?.release()
    return Boolean(h)
  }
  w.__aborted = aborted
  const abortError = () => new DOMException('The request was aborted', 'AbortError')
  const pause = (ms: number, signal?: AbortSignal | null) => new Promise<void>((resolve, reject) => {
    if (signal?.aborted) { reject(abortError()); return }
    const timer = setTimeout(resolve, ms)
    signal?.addEventListener('abort', () => { clearTimeout(timer); reject(abortError()) }, { once: true })
  })
  const hold = (version: number, signal?: AbortSignal | null) => new Promise<void>((resolve, reject) => {
    const entry = { version, release: resolve }
    held.push(entry)
    signal?.addEventListener('abort', () => {
      const i = held.indexOf(entry)
      if (i >= 0) held.splice(i, 1)
      aborted.push(version)
      reject(abortError())
    }, { once: true })
  })
  const heldCalls = new Map<string, (() => void)[]>()
  const calls = window as unknown as { __releaseCall: (method: string) => boolean; __heldCalls: (method: string) => number }
  calls.__releaseCall = (method) => {
    const next = heldCalls.get(method)?.shift()
    next?.()
    return next !== undefined
  }
  calls.__heldCalls = (method) => heldCalls.get(method)?.length ?? 0
  // Another user's call, straight to the store (browser scenarios: a change made elsewhere).
  ;(window as unknown as { __elsewhere: (method: keyof RPC, params: unknown) => unknown }).__elsewhere = (method, params) =>
    (handlers[method] as ((p: unknown) => unknown) | undefined)?.(params)
  const original = window.fetch.bind(window)
  window.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    if (!url.endsWith(rpcPath)) return original(input, init)
    const body = JSON.parse(String(init?.body ?? '{}')) as { id: number; method: keyof RPC; params: never }
    opts.calls.push({ method: body.method, params: body.params })
    const isList = body.method === 'automations.list' || body.method === 'automations.get' || body.method === 'automations.catalog' || body.method === 'automations.backtests'
    if (opts.scenario === 'loading' && isList) await new Promise(() => {})
    const signal = init?.signal
    if (body.method === 'automations.get' && (gets++ > 0 || opts.holdFirst) && opts.hold) await hold((body.params as { version?: number }).version ?? 0, signal)
    if (opts.oldVersionDelay > 0 && body.method === 'automations.get' && (body.params as { version?: number }).version) await pause(opts.oldVersionDelay, signal)
    const held = opts.holdOn?.split(',').includes(body.method) ?? false
    const holdCall = () => new Promise<void>((resolve) => {
      const q = heldCalls.get(body.method) ?? []
      q.push(resolve)
      heldCalls.set(body.method, q)
    })
    const waits = opts.delay > 0 && (opts.delayOnly ? opts.delayOnly.split(',').includes(body.method) : !isList)
    // staleReads: the answer is read first and then waits (held or delayed), so it holds the state at
    // the call's start.
    if (!opts.staleReads) {
      if (held) await holdCall()
      if (waits) await pause(opts.delay, signal)
    }
    let payload: unknown
    try {
      if (opts.scenario === 'error' && isList) rpcError('internal', 'internal error')
      const handler = handlers[body.method] as ((p: unknown) => unknown) | undefined
      if (!handler) rpcError('not_found', `no fake for ${body.method}`)
      payload = { jsonrpc: '2.0', id: body.id, result: handler(body.params) }
    } catch (error) {
      payload = { jsonrpc: '2.0', id: body.id, error }
    }
    const text = JSON.stringify(payload)
    if (opts.staleReads) {
      if (held) await holdCall()
      if (waits) await pause(opts.delay, signal)
    }
    return new Response(text, { status: 200, headers: { 'content-type': 'application/json' } })
  }
}
