import type { RPC } from '../../Integrations.solid.generated'
import type { ConnectionView, ConnectorView, UsageView } from '../../model'

/**
 * An in-memory stand-in for the Go RPC (modules/automations/presentation/integrations): the same
 * JSON-RPC envelope, results and refusals, so the screen runs unchanged. The Go tests own the real
 * contract; this fixture drives screenshots and browser scenarios.
 */
export interface FakeOptions {
  scenario: 'default' | 'empty' | 'loading' | 'error' | 'fresh'
  delay: number
  platformFails: boolean
  /** false: the caller lacks Connections.Manage (an author). */
  manage: boolean
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

function seed(scenario: FakeOptions['scenario']): ConnectorView[] {
  if (scenario === 'empty') return []
  const fresh = scenario === 'fresh'
  return [
    {
      kind: 'telegram_bot', title: 'Telegram bot', icon: '', description: 'Send messages and receive replies through a Telegram bot.',
      category: 'messaging', auth: 'api_key', credential_schema: tokenSchema, nodes: ['Send a Telegram message', 'Ask in Telegram'], platform: null,
      connections: fresh ? [] : [
        conn('telegram_bot', 'sales_bot', 'Sales bot', { identity: '@acme_sales_bot', used_by: 2, fields: { chat_id: '-1001234567890' } }),
        conn('telegram_bot', 'support_bot', 'Support bot', { identity: '@acme_help_bot', status: 'error', status_reason: 'Unauthorized: the token was revoked. Create a new token in @BotFather and replace it.' }),
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
      connections: fresh ? [] : [conn('google_drive', 'drive', 'Company drive', { identity: 'docs@acme.uz', status: 'needs_reauth', status_reason: 'Sign in again to keep access.' })],
    },
  ]
}

const usages: Record<string, UsageView[]> = {}

type Handler = (params: never) => unknown

export function installFake(rpcPath: string, opts: FakeOptions): void {
  const connectors = seed(opts.scenario)
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
  let seq = 100
  const handlers: Record<keyof RPC, Handler> = {
    'automations.connectors.list': () => {
      if (opts.scenario === 'error') throw { code: 'internal', message: 'internal error' }
      return { connectors, can_manage: opts.manage }
    },
    'automations.connections.usage': (p: { id: string }) => {
      if (!find(p.id)) notFound()
      return { usages: usages[p.id] ?? [] }
    },
    'automations.connections.create': (p: RPC['automations.connections.create']['params']) => {
      const c = connectors.find((x) => x.kind === p.kind)
      if (!c) return notFound()
      if (!/^[a-z][a-z0-9_]{0,39}$/.test(p.name)) return { ok: false, error: 'invalid_input', fields: { name: 'name' } }
      if (c.connections.some((x) => x.name === p.name)) return { ok: false, error: 'name_taken', fields: { name: 'taken' } }
      const secret = String(p.credentials.bot_token ?? p.credentials.password ?? p.credentials.api_key ?? '')
      if (secret === 'bad') return { ok: false, error: 'test_failed', message: 'Unauthorized: the service does not accept this token.' }
      const created = conn(c.kind, p.name, p.display_name || p.name, { id: `00000000-0000-4000-8000-${String(++seq).padStart(12, '0')}`, identity: c.kind === 'telegram_bot' ? '@new_bot' : 'account@acme.uz' })
      c.connections.push(created)
      return { ok: true, connection: created }
    },
    'automations.connections.oauth_start': (p: { kind: string; name: string }) => {
      const c = connectors.find((x) => x.kind === p.kind)
      if (!c) return notFound()
      const created = conn(c.kind, p.name || 'drive_2', 'docs@acme.uz', { id: `00000000-0000-4000-8000-${String(++seq).padStart(12, '0')}`, identity: 'docs@acme.uz' })
      c.connections.push(created)
      return { authorize_url: `${window.location.pathname}${window.location.search.replace(/[?&](connect|kind)=[^&]*/g, '').replace(/^&/, '?') || '?'}&connected=${created.id}&kind=${c.kind}` }
    },
    'automations.connections.update': (p: RPC['automations.connections.update']['params']) => {
      const f = find(p.id)
      if (!f) return notFound()
      if (p.version !== f.connection.version) return { ok: false, error: 'version_conflict' }
      if (p.credentials && Object.values(p.credentials).includes('bad')) return { ok: false, error: 'test_failed', message: 'Unauthorized: the service does not accept this token.' }
      Object.assign(f.connection, { display_name: p.display_name ?? f.connection.display_name, version: f.connection.version + 1 })
      if (p.credentials && f.connection.status !== 'disabled') Object.assign(f.connection, { status: 'connected', status_reason: null })
      return { ok: true, connection: f.connection }
    },
    'automations.connections.test': (p: { id: string }) => {
      const f = find(p.id)
      if (!f) return notFound()
      if (f.connection.status === 'needs_reauth') return { ok: false, error: 'needs_reauth' }
      if (f.connection.status === 'error') return { ok: false, message: f.connection.status_reason ?? 'Unauthorized' }
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
    let payload: unknown
    try {
      payload = { jsonrpc: '2.0', id: body.id, result: handlers[body.method](body.params) }
    } catch (error) {
      payload = { jsonrpc: '2.0', id: body.id, error }
    }
    return new Response(JSON.stringify(payload), { status: 200, headers: { 'content-type': 'application/json' } })
  }
}
