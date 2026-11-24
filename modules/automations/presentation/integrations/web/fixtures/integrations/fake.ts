import type { RPC } from '../../Integrations.solid.generated'
import type { ConnectionView, ConnectorView, UsageView } from '../../model'

/**
 * An in-memory stand-in for the Go RPC (modules/automations/presentation/integrations): the same
 * JSON-RPC envelope, results and refusals, so the screen runs unchanged. The Go tests own the real
 * contract; this fixture drives screenshots and browser scenarios.
 */
export interface FakeOptions {
  scenario: 'default' | 'empty' | 'loading' | 'error' | 'fresh' | 'mcp'
  delay: number
  platformFails: boolean
  /** false: the caller lacks Connections.Manage (an author). */
  manage: boolean
  /** Delay of connections.test only (ms). */
  testDelay: number
  /** The first create or update is done but its answer is lost (a 502). */
  lossy: boolean
  /** The OAuth2 return fails with this code (refused, exchange, name_taken, denied). */
  oauthFail: string
  /** The first connections.test fails in transport (a 502). */
  testFailOnce: boolean
  /** Every list after the first fails (a refresh that does not arrive). */
  refreshFails: boolean
  /** The first update meets a version conflict (another manager saved the connection meanwhile):
   * as a result ('result') or as an RPC error ('error'); '' for none. */
  conflictOnce: '' | 'result' | 'error'

  calls: { method: string; params: unknown }[]
}

const now = '2026-11-20T09:30:00Z'

const tokenSchema = {
  type: 'object', required: ['bot_token'],
  properties: {
    bot_token: { type: 'string', title: 'Bot token', 'x-secret': true, description: 'From @BotFather' },
    chat_id: { type: 'string', title: 'Default chat', examples: ['-1001234567890'] },
  },
}
const smtpSchema = {
  type: 'object', required: ['host', 'username', 'password'],
  properties: {
    host: { type: 'string', title: 'Server', examples: ['smtp.example.com'] },
    port: { type: 'integer', title: 'Port', examples: ['587'] },
    username: { type: 'string', title: 'Login' },
    password: { type: 'string', title: 'Password', 'x-secret': true },
  },
}

let seeded = 0

function conn(kind: string, name: string, display: string, extra: Partial<ConnectionView> = {}): ConnectionView {
  return {
    id: `00000000-0000-4000-8000-${String(++seeded).padStart(12, '0')}`,
    kind, name, display_name: display, version: 1, identity: null, status: 'connected', status_reason: null, used_by: 0,
    created_at: now, updated_at: now, last_checked_at: now, fields: {}, ...extra,
  }
}

const mcpSchema = {
  type: 'object', required: ['server_url', 'auth'],
  properties: {
    server_url: { type: 'string', title: 'Server URL', examples: ['https://mcp.example.com/mcp'] },
    auth: { type: 'string', title: 'Authentication', enum: ['none', 'bearer', 'oauth2'] },
    token: { type: 'string', title: 'Bearer token', 'x-secret': true },
    oauth_client_id: { type: 'string', title: 'OAuth client ID' },
    oauth_client_secret: { type: 'string', title: 'OAuth client secret', 'x-secret': true },
    oauth_scopes: { type: 'string', title: 'OAuth scopes' },
  },
}

function seed(scenario: FakeOptions['scenario']): ConnectorView[] {
  if (scenario === 'empty') return []
  if (scenario === 'mcp') {
    return [{
      kind: 'mcp', title: 'MCP server', icon: '', description: 'Tools of a remote MCP server (Streamable HTTP)', category: 'ai', auth: 'custom',
      credential_schema: mcpSchema, nodes: ['MCP tool'], platform: null,
      connections: [
        conn('mcp', 'crm_tools', 'CRM tools', { status: 'needs_reauth', status_reason: 'reauth', sign_in: true, fields: { server_url: 'https://mcp.crm.example.com/mcp', auth: 'oauth2' } }),
        conn('mcp', 'docs_search', 'Docs search', { fields: { server_url: 'https://mcp.docs.example.com/mcp', auth: 'bearer' } }),
      ],
    }]
  }
  const fresh = scenario === 'fresh'
  return [
    {
      kind: 'telegram_bot', title: 'Telegram bot', icon: '', description: 'Send messages and receive replies through a Telegram bot.',
      category: 'messaging', auth: 'api_key', credential_schema: tokenSchema, nodes: ['Send a Telegram message', 'Ask in Telegram'], platform: null,
      connections: fresh ? [] : [
        conn('telegram_bot', 'sales_bot', 'Sales bot', { identity: '@acme_sales_bot', used_by: 2, fields: { chat_id: '-1001234567890' } }),
        conn('telegram_bot', 'support_bot', 'Support bot', { identity: '@acme_help_bot', status: 'error', status_reason: 'auth' }),
      ],
    },
    {
      kind: 'smtp', title: 'SMTP mail', icon: '', description: 'Send email through your mail server.', category: 'email', auth: 'basic',
      credential_schema: smtpSchema, nodes: ['Send an email'], platform: null,
      connections: fresh ? [] : [conn('smtp', 'office_mail', 'Office mail', { identity: 'robot@acme.uz', status: 'disabled', fields: { host: 'smtp.acme.uz', port: '587', username: 'robot@acme.uz' } })],
    },
    {
      kind: 'openai', title: 'OpenAI', icon: '', description: 'Classify, summarize and extract with language models.', category: 'ai', auth: 'api_key',
      credential_schema: { type: 'object', required: ['api_key'], properties: { api_key: { type: 'string', title: 'API key', 'x-secret': true } } },
      nodes: ['Ask a model', 'Classify text'], platform: { policy: 'opt_in', available: true, enabled_for_tenant: false }, connections: [],
    },
    {
      kind: 'google_drive', title: 'Google Drive', icon: '', description: 'Save and read documents in Google Drive.', category: 'documents', auth: 'oauth2',
      credential_schema: {}, nodes: ['Upload a file'], platform: null,
      connections: fresh ? [] : [conn('google_drive', 'drive', 'Company drive', { identity: 'docs@acme.uz', status: 'needs_reauth', status_reason: 'reauth' })],
    },
  ]
}

const usages: Record<string, UsageView[]> = {}

type Handler = (params: never) => unknown

// An OAuth2 start leaves the page for the provider and comes back to a new document: the catalog it
// changed is carried across that one navigation (and only that one), as the server would keep it.
const CARRIED = 'automations-fake-oauth'

function carried(): { connectors: ConnectorView[]; seq: number } | undefined {
  try {
    const raw = window.sessionStorage.getItem(CARRIED)
    window.sessionStorage.removeItem(CARRIED)
    return raw ? JSON.parse(raw) as { connectors: ConnectorView[]; seq: number } : undefined
  } catch {
    return undefined
  }
}

export function installFake(rpcPath: string, opts: FakeOptions): void {
  const kept = carried()
  const connectors = kept?.connectors ?? seed(opts.scenario)
  const carry = () => {
    try { window.sessionStorage.setItem(CARRIED, JSON.stringify({ connectors, seq })) } catch { /* a private window: the return reseeds */ }
  }
  const sales = connectors[0]?.connections[0]
  if (sales) {
    usages[sales.id] = [
      { automation_id: 'a1b2c3d4-0000-4000-8000-000000000001', name: 'New order → notify sales', version: 3, enabled: true, node_ids: ['notify'] },
      { automation_id: 'a1b2c3d4-0000-4000-8000-000000000002', name: 'Daily summary', version: 1, enabled: false, node_ids: ['send', 'send_2'] },
    ]
  }
  const find = (id: string) => {
    for (const c of connectors) {
      const x = c.connections.find((y) => y.id === id)
      if (x) return { connector: c, connection: x }
    }
    return undefined
  }
  const notFound = () => { throw { code: 'not_found', message: 'connection not found', data: { code: 'not_found' } } }
  let seq = kept?.seq ?? 100
  let conflicted = false
  let listed = 0
  const keys = new Map<string, ConnectionView>()
  // Like the service, the key of a request that was saved is bound to its payload (a refused one is
  // released and may be reused).
  const payloads = new Map<string, string>()
  const body = (p: unknown) => { const { idempotency_key: _, ...rest } = p as Record<string, unknown>; return JSON.stringify(rest) }
  const mismatch = (key: string | undefined, p: unknown) => Boolean(key && payloads.has(key) && payloads.get(key) !== body(p))
  const record = (key: string | undefined, p: unknown) => { if (key) payloads.set(key, body(p)) }
  const changed = { ok: false, error: 'invalid_input', fields: { idempotency_key: 'invalid' } }
  let lost = false
  let testFailed = false
  const loseOnce = () => {
    if (!opts.lossy || lost) return
    lost = true
    throw new Response('bad gateway', { status: 502 })
  }
  const handlers: Record<keyof RPC, Handler> = {
    'automations.connectors.list': () => {
      if (opts.scenario === 'error' || (opts.refreshFails && listed++ > 0)) throw { code: 'internal', message: 'internal error' }
      return { connectors, can_manage: opts.manage }
    },
    'automations.connections.usage': (p: { id: string }) => {
      if (!find(p.id)) notFound()
      return { usages: usages[p.id] ?? [] }
    },
    'automations.connections.create': (p: RPC['automations.connections.create']['params']) => {
      const c = connectors.find((x) => x.kind === p.kind)
      if (!c) return notFound()
      if (mismatch(p.idempotency_key, p)) return changed
      // Like the service: a retried key answers with what the first request made.
      if (p.idempotency_key && keys.has(p.idempotency_key)) return { ok: true, connection: keys.get(p.idempotency_key) }
      if (!/^[a-z][a-z0-9_]{0,39}$/.test(p.name)) return { ok: false, error: 'invalid_input', fields: { name: 'name' } }
      if (c.connections.some((x) => x.name === p.name)) return { ok: false, error: 'name_taken', fields: { name: 'taken' } }
      const secret = String(p.credentials.bot_token ?? p.credentials.password ?? p.credentials.api_key ?? '')
      if (secret === 'bad') return { ok: false, error: 'test_failed', reason: 'auth' }
      // A bot another service already uses: the person confirms before it is connected.
      // The confirmation is bound to the bot (the server's token is "<code>@<bot id>"): the code alone is not enough.
      if (secret === 'foreign' && !(p.confirm ?? []).includes('foreign_webhook@222222')) return { ok: false, error: 'confirmation_required', warnings: ['foreign_webhook@222222'] }
      const key = p.idempotency_key
      if (key && keys.has(key)) return { ok: true, connection: keys.get(key) }
      const signIn = c.kind === 'mcp' && p.credentials.auth === 'oauth2'
      const created = conn(c.kind, p.name, p.display_name || p.name, {
        id: `00000000-0000-4000-8000-${String(++seq).padStart(12, '0')}`, identity: c.kind === 'telegram_bot' ? '@new_bot' : 'account@acme.uz',
        ...(signIn ? { status: 'needs_reauth' as const, status_reason: 'reauth' as const, sign_in: true, identity: null } : {}),
      })
      c.connections.push(created)
      if (key) keys.set(key, created)
      record(key, p)
      loseOnce()
      return { ok: true, connection: created }
    },
    'automations.connections.oauth_start': (p: { kind: string; name: string; display_name?: string; connection_id?: string }) => {
      const c = connectors.find((x) => x.kind === p.kind)
      if (!c) return notFound()
      if (opts.oauthFail) {
        const back = new URLSearchParams({ oauth_error: opts.oauthFail, kind: p.kind })
        if (p.connection_id) back.set('edit', p.connection_id)
        else { if (p.name) back.set('name', p.name); if (p.display_name) back.set('display_name', p.display_name) }
        return { authorize_url: `${window.location.pathname}?${back}` }
      }
      if (p.connection_id) {
        const f = find(p.connection_id)
        if (!f) return notFound()
        Object.assign(f.connection, { status: 'connected', status_reason: null, version: f.connection.version + 1 })
        carry()
        return { authorize_url: `${window.location.pathname}?connected=${p.connection_id}&kind=${p.kind}&edit=${p.connection_id}` }
      }
      const created = conn(c.kind, p.name || 'drive_2', 'docs@acme.uz', { id: `00000000-0000-4000-8000-${String(++seq).padStart(12, '0')}`, identity: 'docs@acme.uz' })
      c.connections.push(created)
      carry()
      return { authorize_url: `${window.location.pathname}${window.location.search.replace(/[?&](connect|kind)=[^&]*/g, '').replace(/^&/, '?') || '?'}&connected=${created.id}&kind=${c.kind}` }
    },
    'automations.connections.update': (p: RPC['automations.connections.update']['params']) => {
      const f = find(p.id)
      if (!f) return notFound()
      const key = p.idempotency_key
      if (mismatch(key, p)) return changed
      if (key && keys.has(key)) return { ok: true, connection: keys.get(key) }
      if (opts.conflictOnce && !conflicted) {
        conflicted = true
        Object.assign(f.connection, { display_name: 'Renamed elsewhere', version: f.connection.version + 1, fields: { ...f.connection.fields, chat_id: '-1009999' } })
      }
      if (p.version !== f.connection.version) {
        if (opts.conflictOnce === 'error') throw { code: 'version_conflict', message: 'version conflict', data: { code: 'version_conflict' } }
        return { ok: false, error: 'version_conflict' }
      }
      if (p.credentials && Object.values(p.credentials).includes('bad')) return { ok: false, error: 'test_failed', reason: 'auth' }
      Object.assign(f.connection, { display_name: p.display_name ?? f.connection.display_name, version: f.connection.version + 1 })
      if (p.credentials && f.connection.status !== 'disabled') Object.assign(f.connection, { status: 'connected', status_reason: null })
      if (key) keys.set(key, f.connection)
      record(key, p)
      loseOnce()
      return { ok: true, connection: f.connection }
    },
    'automations.connections.test': (p: { id: string }) => {
      if (opts.testFailOnce && !testFailed) { testFailed = true; throw new Response('bad gateway', { status: 502 }) }
      const f = find(p.id)
      if (!f) return notFound()
      if (f.connection.status === 'needs_reauth') return { ok: false, error: 'needs_reauth', reason: 'reauth' }
      if (f.connection.status === 'error') return { ok: false, reason: 'auth' }
      return { ok: true, identity: f.connection.identity ?? '' }
    },
    'automations.connections.set_disabled': (p: { id: string; disabled: boolean }) => {
      const f = find(p.id)
      if (!f) return notFound()
      f.connection.status = p.disabled ? 'disabled' : 'connected'
      f.connection.version++
      return { connection: f.connection }
    },
    'automations.connections.delete': (p: { id: string }) => {
      const f = find(p.id)
      if (!f) return notFound()
      if ((usages[p.id] ?? []).length > 0) return { ok: false, error: 'CONNECTION_IN_USE', usages: usages[p.id] }
      f.connector.connections = f.connector.connections.filter((x) => x.id !== p.id)
      return { ok: true }
    },
    'automations.connections.options': () => ({ options: [] }),
    'automations.platform.set_enabled': (p: { kind: string; enabled: boolean }) => {
      if (opts.platformFails) throw { code: 'internal', message: 'internal error' }
      const c = connectors.find((x) => x.kind === p.kind)
      if (c?.platform) c.platform.enabled_for_tenant = p.enabled
      return { ok: true }
    },
  }
  const original = window.fetch.bind(window)
  window.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    if (!url.endsWith(rpcPath)) return original(input, init)
    const body = JSON.parse(String(init?.body ?? '{}')) as { id: number; method: keyof RPC; params: never }
    opts.calls.push({ method: body.method, params: body.params })
    if (opts.scenario === 'loading') await new Promise(() => {})
    const isList = body.method === 'automations.connectors.list'
    if (opts.delay > 0 && !isList) await new Promise((r) => setTimeout(r, opts.delay))
    if (opts.testDelay > 0 && body.method === 'automations.connections.test') await new Promise((r) => setTimeout(r, opts.testDelay))
    let payload: unknown
    try {
      payload = { jsonrpc: '2.0', id: body.id, result: handlers[body.method](body.params) }
    } catch (error) {
      if (error instanceof Response) return error
      payload = { jsonrpc: '2.0', id: body.id, error }
    }
    return new Response(JSON.stringify(payload), { status: 200, headers: { 'content-type': 'application/json' } })
  }
}
