import { catalog as editorCatalog } from '../../../../../../../web/automation-editor/fixture/catalog'
import type { RPC } from '../../Automations.solid.generated'

/**
 * An in-memory stand-in for the Go RPC (modules/automations/presentation/graphs): the same JSON-RPC
 * envelope, results and refusals, so the screens run unchanged. The Go tests own the real contract;
 * this fixture drives screenshots and browser scenarios.
 */
export interface FakeOptions {
  scenario: 'default' | 'empty' | 'loading' | 'error'
  /** Delay of every action (ms); lists answer at once. */
  delay: number
  /** The code of the graph changes once between the review and the enable (a digest refusal). */
  changeOnce: boolean
  /** The first layout save meets a newer revision (another tab). */
  layoutConflict: boolean
  /** The first save meets a version saved by another user. */
  conflictOnce: boolean
  /** The first count of live runs fails (the confirmation offers a retry). */
  liveFailsOnce: boolean
  /** The review after a digest refusal fails once (a network error). */
  reviewFailsAfterChange: boolean
  /** Delay of automations.get for an old version (ms): a request still on the way when Back is pressed. */
  oldVersionDelay: number
  can: { author: boolean; enable: boolean; operate: boolean }
  calls: { method: string; params: unknown }[]
  /** Validation warns about the first node (a pinned tool schema the server changed), with its fix. */
  warn?: boolean
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

function seed(): Stored[] {
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
      versions: [{ version: 2, graph: { ...welcome, name: 'Close stale deals' }, by: 'Aziz Rakhimov', origin: 'human', at: '2026-11-10T07:00:00Z' }],
      layout: {},
      revision: 0,
      live: { runs: 0, waits: 0 },
      code: [],
    },
  ]
}

function rpcError(code: string, message: string): never {
  throw { code, message, data: { code } }
}

export function installFake(rpcPath: string, opts: FakeOptions): void {
  const store = opts.scenario === 'empty' ? [] : seed()
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
  const find = (id: string) => store.find((s) => s.item.id === id) ?? rpcError('not_found', 'automation not found')
  const digestOf = (s: Stored) => (s.code.length > 0 ? `digest-${s.item.latest_version}-${s.code.map((c) => c.source.length).join('.')}` : '')
  const handlers: { [K in keyof RPC]?: (p: RPC[K]['params']) => RPC[K]['result'] } = {
    'automations.list': (p) => {
      let items = store.map((s) => s.item)
      if (p.query) items = items.filter((x) => x.name.toLowerCase().includes(p.query!.toLowerCase()))
      if (p.status === 'enabled') items = items.filter((x) => x.enabled)
      if (p.status === 'disabled') items = items.filter((x) => !x.enabled)
      if (p.status === 'attention') items = items.filter((x) => x.attention > 0)
      return { items, next_cursor: null, can: opts.can }
    },
    'automations.catalog': () => ({ catalog: catalog as never, connections: { telegram: [{ id: 'c-1', name: 'Sales bot' }] }, can: opts.can }),
    'automations.get': (p) => {
      const s = find(p.id)
      const v = s.versions.find((x) => x.version === (p.version || s.item.latest_version)) ?? s.versions[s.versions.length - 1]!
      return {
        automation: { id: s.item.id, name: s.item.name, enabled: s.item.enabled, enabled_version: s.item.enabled_version, latest_version: s.item.latest_version, owner: { user_id: 1, name: 'Dilnoza Karimova' }, layout: s.layout, layout_revision: s.revision },
        graph: v.graph, version: v.version, saved_by: { name: v.by, origin: v.origin },
        versions: [...s.versions].reverse().map((x) => ({ version: x.version, enabled: x.version === s.item.enabled_version, by: x.by, origin: x.origin, at: x.at })),
        catalog: catalog as never, connections: { telegram: [{ id: 'c-1', name: 'Sales bot' }] },
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
      return { ok: true, run_id: run }
    },
    'automations.search_subjects': (p) => [{ subject: { entity: p.entity, key: '42' }, label: `${p.query} (42)` }],
    'automations.runs': (p) => {
      const s = find(p.automation_id)
      return {
        items: s.item.last_run ? [{
          run_id: s.item.last_run.run_id, automation_id: s.item.id, version: s.item.enabled_version ?? 1, state: s.item.last_run.status === 'done' ? 'done' : 'failed', status: s.item.last_run.status,
          trigger_kind: 'event', trigger_id: 'created', subject: { entity: 'none', key: '' }, current_nodes: [],
          error: s.item.last_run.status === 'failed' ? { code: 'effect_failed', message: 'The mail server refused the message' } : null, created_at: s.item.last_run.at, updated_at: s.item.last_run.at,
        }] : [],
        next_cursor: null,
      }
    },
    'automations.run': (p) => ({
      run: {
        run_id: p.run_id, automation_id: store[0]?.item.id ?? '', version: 1, state: 'done', status: 'done', trigger_kind: 'event', trigger_id: 'created',
        subject: { entity: 'none', key: '' }, current_nodes: [], error: null, created_at: now, updated_at: now,
      },
      tokens: [], steps: [{ token_id: 't', seq: 1, node_id: 'fin', attempt: 1, outcome: 'done', error: null, started_at: now, ended_at: now }], waits: [],
    }),
    'automations.dry_run': () => ({ trace: [] }),
    'automations.test_node': () => ({ outcome: 'done' }),
    'automations.state_list': (p) => {
      find(p.id)
      const q = (p.search ?? '').toLowerCase()
      const all = [...memory.values()].filter((e) => e.key.toLowerCase().includes(q)).sort((a, b) => (a.key < b.key ? -1 : 1))
      const after = p.cursor ? all.filter((e) => e.key > p.cursor!) : all
      const limit = p.limit ?? 100
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
  const original = window.fetch.bind(window)
  window.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    if (!url.endsWith(rpcPath)) return original(input, init)
    const body = JSON.parse(String(init?.body ?? '{}')) as { id: number; method: keyof RPC; params: never }
    opts.calls.push({ method: body.method, params: body.params })
    const isList = body.method === 'automations.list' || body.method === 'automations.get' || body.method === 'automations.catalog'
    if (opts.scenario === 'loading' && isList) await new Promise(() => {})
    if (opts.oldVersionDelay > 0 && body.method === 'automations.get' && (body.params as { version?: number }).version) await new Promise((r) => setTimeout(r, opts.oldVersionDelay))
    if (opts.delay > 0 && !isList) await new Promise((r) => setTimeout(r, opts.delay))
    let payload: unknown
    try {
      if (opts.scenario === 'error' && isList) rpcError('internal', 'internal error')
      const handler = handlers[body.method] as ((p: unknown) => unknown) | undefined
      if (!handler) rpcError('not_found', `no fake for ${body.method}`)
      payload = { jsonrpc: '2.0', id: body.id, result: handler(body.params) }
    } catch (error) {
      payload = { jsonrpc: '2.0', id: body.id, error }
    }
    return new Response(JSON.stringify(payload), { status: 200, headers: { 'content-type': 'application/json' } })
  }
}
