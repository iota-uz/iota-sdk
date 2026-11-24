import type { AutomationGraph, EditorCatalog, EditorLocale, RunData, RunState, RunSummary, RunsView, RunToken, RunWait } from '@iota-uz/sdk/automation-editor'
import type { RPC } from './Automations.solid.generated'

export type ListParams = RPC['automations.list']['params']
export type ListResult = RPC['automations.list']['result']
export type ListItem = ListResult['items'][number]
export type StatusFilter = NonNullable<ListParams['status']>
export type RunStatus = NonNullable<ListItem['last_run']>['status']
export type TriggerKind = ListItem['triggers'][number]['kind']
export type GetResult = RPC['automations.get']['result']
export type ReviewResult = RPC['automations.enable_review']['result']
export type EnableResult = RPC['automations.enable']['result']
export type Can = ListResult['can']
export type ConnectionOptions = GetResult['connections']

export const MSG = 'Automations.Graphs.'

export interface Translator {
  (key: string, vars?: Record<string, string | number>): string
}

/** A translator over the host's messages: keys are below MSG; {name} placeholders are replaced. */
export function translator(t: (key: string) => string): Translator {
  return (key, vars) => {
    let s = t(MSG + key)
    if (s === MSG + key) s = key
    for (const [k, v] of Object.entries(vars ?? {})) s = s.split(`{${k}}`).join(String(v))
    return s
  }
}

/** The editor's locale of the user's language. */
export function editorLocale(lang: string): EditorLocale {
  const l = lang.toLowerCase()
  if (l === 'uz-cyrl' || l.startsWith('uz-cyrl')) return 'uz-Cyrl'
  if (l.startsWith('uz')) return 'uz'
  if (l.startsWith('ru')) return 'ru'
  return 'en'
}

/** BCP 47 tag of the user's language for Intl. */
export function intlTag(lang: string): string {
  switch (editorLocale(lang)) {
    case 'ru': return 'ru-RU'
    case 'uz': return 'uz-Latn-UZ'
    case 'uz-Cyrl': return 'uz-Cyrl-UZ'
    default: return 'en-US'
  }
}

/**
 * A counted phrase with the plural form of the language (Intl.PluralRules): keys <base>.Singular,
 * .Paucal, .Multiple, .Plural (go-i18n reserves the CLDR names as keys).
 */
export function counted(t: Translator, base: string, count: number, lang: string): string {
  let rule = 'other'
  try { rule = new Intl.PluralRules(intlTag(lang)).select(count) } catch { /* unknown language: other */ }
  const key = `${base}.${rule === 'one' ? 'Singular' : rule === 'few' ? 'Paucal' : rule === 'many' ? 'Multiple' : 'Plural'}`
  const s = t(key, { count })
  return s === key ? t(`${base}.Plural`, { count }) : s
}

/** The status of a graph as the list and the editor name it. */
export type GraphStatus = { kind: 'enabled'; version: number } | { kind: 'pending'; version: number; enabled: number | null } | { kind: 'disabled' }

export function graphStatus(item: Pick<ListItem, 'enabled' | 'enabled_version' | 'latest_version' | 'pending_version'>): GraphStatus {
  if (item.pending_version) return { kind: 'pending', version: item.latest_version, enabled: item.enabled_version }
  if (item.enabled && item.enabled_version !== null) return { kind: 'enabled', version: item.enabled_version }
  return { kind: 'disabled' }
}

export function graphStatusText(t: Translator, s: GraphStatus): string {
  switch (s.kind) {
    case 'enabled': return t('Status.Enabled', { version: s.version })
    case 'pending': return s.enabled !== null ? t('Status.PendingEnabled', { version: s.version, enabled: s.enabled }) : t('Status.Pending', { version: s.version })
    default: return t('Status.Disabled')
  }
}

/** The icon of a run status and whether it needs attention. */
export type IconName =
  | 'check' | 'x' | 'clock' | 'play' | 'pause' | 'alert' | 'dot' | 'bolt' | 'calendar' | 'hand' | 'plug' | 'filter' | 'plus' | 'search'
  | 'robot' | 'code' | 'key' | 'globe' | 'lock' | 'copy' | 'trash' | 'refresh' | 'arrow'

export function runIcon(s: RunStatus): IconName {
  switch (s) {
    case 'done': return 'check'
    case 'failed': case 'filter_failed': return 'x'
    case 'blocked': return 'alert'
    case 'waiting': case 'queued': return 'clock'
    case 'cancelled': return 'pause'
    default: return 'play'
  }
}

export function runTone(s: RunStatus): 'ok' | 'bad' | 'warn' | 'muted' {
  switch (s) {
    case 'done': return 'ok'
    case 'failed': case 'filter_failed': return 'bad'
    case 'blocked': return 'warn'
    default: return 'muted'
  }
}

/** The locale key of a run status (RunStatus of #1335). */
export function runStatusKey(s: RunStatus): string {
  const keys: Record<RunStatus, string> = {
    queued: 'Queued', running: 'Running', waiting: 'Waiting', blocked: 'Blocked', done: 'Done', failed: 'Failed', cancelled: 'Cancelled', filter_failed: 'FilterFailed',
  }
  return `Run.${keys[s]}`
}

export function triggerIcon(k: TriggerKind): IconName {
  switch (k) {
    case 'schedule': return 'calendar'
    case 'manual': return 'hand'
    case 'inbound': return 'plug'
    default: return 'bolt'
  }
}

/** «5 minutes ago» in the user's language; absolute date after a week. */
export function relativeTime(iso: string, now: Date, lang: string): string {
  const at = new Date(iso)
  if (Number.isNaN(at.getTime())) return ''
  const diff = Math.round((at.getTime() - now.getTime()) / 1000)
  const abs = Math.abs(diff)
  const tag = intlTag(lang)
  let rtf: Intl.RelativeTimeFormat
  try { rtf = new Intl.RelativeTimeFormat(tag, { numeric: 'auto' }) } catch { rtf = new Intl.RelativeTimeFormat('en', { numeric: 'auto' }) }
  if (abs < 45) return rtf.format(0, 'second')
  if (abs < 45 * 60) return rtf.format(Math.round(diff / 60), 'minute')
  if (abs < 22 * 3600) return rtf.format(Math.round(diff / 3600), 'hour')
  if (abs < 7 * 86400) return rtf.format(Math.round(diff / 86400), 'day')
  return absoluteTime(iso, lang)
}

export function absoluteTime(iso: string, lang: string): string {
  const at = new Date(iso)
  if (Number.isNaN(at.getTime())) return ''
  try {
    return new Intl.DateTimeFormat(intlTag(lang), { dateStyle: 'medium', timeStyle: 'short' }).format(at)
  } catch {
    return at.toISOString()
  }
}

/** The list's filters as the URL keeps them (D1): ?q=…&status=…. */
export interface Filters { query: string; status: StatusFilter | '' }

const STATUSES: readonly StatusFilter[] = ['enabled', 'disabled', 'attention']

export function readFilters(search: string): Filters {
  const q = new URLSearchParams(search)
  const status = q.get('status') ?? ''
  return { query: (q.get('q') ?? '').trim(), status: (STATUSES as readonly string[]).includes(status) ? status as StatusFilter : '' }
}

/** The URL of the list with the filters, keeping any other parameter. */
export function writeFilters(href: string, f: Filters): string {
  const url = new URL(href)
  if (f.query) url.searchParams.set('q', f.query)
  else url.searchParams.delete('q')
  if (f.status) url.searchParams.set('status', f.status)
  else url.searchParams.delete('status')
  return url.pathname + url.search + url.hash
}

/** The code of an RPC refusal: the error's details.code, else its code. */
export function errorCode(error: unknown): string | undefined {
  const details = (error as { details?: unknown; data?: unknown } | undefined)
  const d = details?.details ?? details?.data
  if (d && typeof d === 'object' && 'code' in d) return String((d as { code: unknown }).code)
  const code = (error as { code?: unknown } | undefined)?.code
  return typeof code === 'string' ? code : undefined
}

/** The phrase of a refusal (C2): what happened and what to do; an unknown code names the action. */
export function errorText(t: Translator, error: unknown, action: string): string {
  const code = errorCode(error)
  if (typeof navigator !== 'undefined' && navigator.onLine === false) return t('Errors.Offline')
  switch (code) {
    case 'forbidden': return t('Errors.Forbidden')
    case 'not_found': return t('Errors.NotFound')
    case 'unauthenticated': case 'unauthorized': return t('Errors.SignIn')
    case 'invalid_input': return t('Errors.InvalidInput')
    case 'connection_unavailable': return t('Errors.ConnectionUnavailable')
    case 'limit_exceeded': return t('Errors.LimitExceeded')
    case 'conflict': return t('Errors.Conflict')
    case 'HUMAN_REQUIRED': return t('Errors.HumanRequired')
    case 'not_enabled': return t('Errors.NotEnabled')
    case 'run_finished': return t('Errors.RunFinished')
  }
  if (code) console.warn('automations: refusal', code, error)
  return t('Errors.Generic', { action })
}

/** A token of highlighted source: its text and its kind. */
export interface Token { text: string; kind: 'plain' | 'keyword' | 'string' | 'comment' | 'number' }

const KEYWORDS = new Set([
  'async', 'await', 'break', 'case', 'catch', 'class', 'const', 'continue', 'default', 'delete', 'do', 'else', 'export', 'extends',
  'false', 'finally', 'for', 'from', 'function', 'if', 'import', 'in', 'instanceof', 'interface', 'let', 'new', 'null', 'of', 'return',
  'static', 'switch', 'this', 'throw', 'true', 'try', 'type', 'typeof', 'undefined', 'var', 'void', 'while', 'yield', 'as',
])

/** Splits TypeScript or JavaScript into tokens for highlighting (no evaluation; any input is safe). */
export function highlight(source: string): Token[] {
  const out: Token[] = []
  const push = (text: string, kind: Token['kind']) => {
    if (!text) return
    const last = out[out.length - 1]
    if (last && last.kind === kind) last.text += text
    else out.push({ text, kind })
  }
  let i = 0
  const n = source.length
  while (i < n) {
    const c = source[i]!
    const next = source[i + 1]
    if (c === '/' && next === '/') {
      const end = source.indexOf('\n', i)
      const stop = end < 0 ? n : end
      push(source.slice(i, stop), 'comment')
      i = stop
    } else if (c === '/' && next === '*') {
      const end = source.indexOf('*/', i + 2)
      const stop = end < 0 ? n : end + 2
      push(source.slice(i, stop), 'comment')
      i = stop
    } else if (c === '"' || c === "'" || c === '`') {
      let j = i + 1
      while (j < n && source[j] !== c) j += source[j] === '\\' ? 2 : 1
      const stop = Math.min(n, j + 1)
      push(source.slice(i, stop), 'string')
      i = stop
    } else if (/[0-9]/.test(c) && !/[A-Za-z_$0-9]/.test(source[i - 1] ?? '')) {
      let j = i
      while (j < n && /[0-9._xXa-fA-FnoObB]/.test(source[j]!)) j++
      push(source.slice(i, j), 'number')
      i = j
    } else if (/[A-Za-z_$]/.test(c)) {
      let j = i
      while (j < n && /[A-Za-z_$0-9]/.test(source[j]!)) j++
      const word = source.slice(i, j)
      push(word, KEYWORDS.has(word) ? 'keyword' : 'plain')
      i = j
    } else {
      push(c, 'plain')
      i++
    }
  }
  return out
}

/** A field of a manual start's input form, from the trigger's input_schema. */
export interface InputField {
  name: string
  title: string
  description?: string
  kind: 'text' | 'textarea' | 'number' | 'integer' | 'boolean' | 'enum' | 'json'
  required: boolean
  /** Labels of an enum's values; the form keeps the index, inputValue sends the typed value. */
  options?: string[]
  enumValues?: unknown[]
}

interface SchemaProperty {
  type?: string | string[]
  title?: string
  description?: string
  enum?: unknown[]
  maxLength?: number
  format?: string
}

export function inputFields(schema: unknown): InputField[] {
  if (!schema || typeof schema !== 'object') return []
  const doc = schema as { properties?: Record<string, SchemaProperty>; required?: string[] }
  const required = new Set(doc.required ?? [])
  return Object.entries(doc.properties ?? {}).map(([name, p]) => {
    const type = Array.isArray(p.type) ? p.type.find((x) => x !== 'null') : p.type
    let kind: InputField['kind'] = 'text'
    if (Array.isArray(p.enum) && p.enum.length > 0) kind = 'enum'
    else if (type === 'boolean') kind = 'boolean'
    else if (type === 'integer') kind = 'integer'
    else if (type === 'number') kind = 'number'
    else if (type === 'object' || type === 'array') kind = 'json'
    else if ((p.maxLength ?? 0) > 200) kind = 'textarea'
    const field: InputField = { name, title: p.title ?? name, kind, required: required.has(name) }
    if (p.description) field.description = p.description
    if (kind === 'enum') {
      field.enumValues = [...(p.enum ?? [])]
      field.options = field.enumValues.map((v) => (typeof v === 'string' ? v : JSON.stringify(v)))
    }
    return field
  })
}

/** The input of a manual start from the form's values; a field that cannot be read is an error by name. */
export function inputValue(fields: InputField[], values: Record<string, string | boolean>): { input: Record<string, unknown>; errors: Record<string, 'required' | 'number' | 'json'> } {
  const input: Record<string, unknown> = {}
  const errors: Record<string, 'required' | 'number' | 'json'> = {}
  for (const f of fields) {
    const v = values[f.name]
    if (f.kind === 'boolean') {
      input[f.name] = Boolean(v)
      continue
    }
    const s = typeof v === 'string' ? v.trim() : ''
    if (s === '') {
      if (f.required) errors[f.name] = 'required'
      continue
    }
    if (f.kind === 'enum') {
      // The form holds the index of the value: 1 stays a number, "1" a string.
      const i = Number(s)
      if (Number.isInteger(i) && f.enumValues && i >= 0 && i < f.enumValues.length) input[f.name] = f.enumValues[i]
      else errors[f.name] = 'required'
    } else if (f.kind === 'number' || f.kind === 'integer') {
      const num = Number(s)
      if (!Number.isFinite(num) || (f.kind === 'integer' && !Number.isInteger(num))) errors[f.name] = 'number'
      else input[f.name] = num
    } else if (f.kind === 'json') {
      try { input[f.name] = JSON.parse(s) } catch { errors[f.name] = 'json' }
    } else {
      input[f.name] = s
    }
  }
  return { input, errors }
}

/** A manual trigger of a graph with its input schema and title. */
export interface ManualTrigger { id: string; title: string; schema: unknown }

export function manualTriggers(graph: AutomationGraph, catalog: EditorCatalog): ManualTrigger[] {
  const manual = new Set(catalog.events.filter((e) => e.manual).map((e) => e.key))
  return graph.triggers.filter((tr) => manual.has(tr.event)).map((tr) => ({
    id: tr.id,
    title: catalog.events.find((e) => e.key === tr.event)?.title ?? tr.id,
    schema: (tr.params as { input_schema?: unknown } | undefined)?.input_schema ?? null,
  }))
}

/** An empty graph of a new automation. */
export function emptyGraph(): AutomationGraph {
  return { name: '', run_as: 'owner', triggers: [], nodes: [], edges: [] }
}

/** The path of a graph (and of its runs tab or a run). */
export function graphPath(base: string, id: string, opts?: { tab?: 'runs'; run?: string; node?: string }): string {
  const root = `${base.replace(/\/$/, '')}/${encodeURIComponent(id)}`
  const path = opts?.run ? `${root}/runs/${encodeURIComponent(opts.run)}` : root
  const q = new URLSearchParams()
  if (opts?.tab && !opts.run) q.set('tab', opts.tab)
  if (opts?.node) q.set('node', opts.node)
  const s = q.toString()
  return s ? `${path}?${s}` : path
}

/** Why a version does not enable, in words (AL7): an unavailable connection or a missing right is named. */
export function problemText(t: Translator, p: { code?: string; message: string }): string {
  switch (p.code) {
    case 'connection_unavailable': return t('Review.Problem.Connection')
    case 'triggers_unreadable': return t('Review.Problem.Triggers')
    case 'unknown_node_type': return t('Review.Problem.NodeType')
  }
  return t('Review.Problem.Invalid')
}

// ——— runs (#1347): the wire of automations.runs / automations.run → the editor's RunSummary / RunData ———
type WireRun = RPC['automations.runs']['result']['items'][number]
type WireRunData = RPC['automations.run']['result']
const RUN_STATES: readonly RunState[] = ['running', 'waiting', 'blocked', 'failed', 'done', 'cancelled']

/** A run in the editor's shape: no subject for "none", the error as its phrase, the trigger by id. */
export function runSummaryOf(r: WireRun & { subject_url?: string }): RunSummary {
  const subject = r.subject.entity && r.subject.entity !== 'none'
    ? { entity: r.subject.entity, key: r.subject.key, ...(r.subject.label ? { label: r.subject.label } : {}), ...(r.subject_url ? { url: r.subject_url } : {}) }
    : null
  return {
    run_id: r.run_id, version: r.version, state: r.state, subject, trigger: r.trigger_id,
    error: r.error?.message ?? null, created_at: r.created_at, updated_at: r.updated_at, current_nodes: r.current_nodes,
  }
}

const tokenState = (s: string): RunToken['state'] => (s === 'running' ? 'active' : s === 'waiting' ? 'waiting' : s === 'cancelled' ? 'cancelled' : 'done')
const waitKind = (k: string): RunWait['kind'] | null => {
  switch (k) {
    case 'event': case 'timer': case 'call': return k
    case 'task': return 'human'
  }
  return null // a join is the join's own state, not a wait of the run
}

/** A run with its tokens, steps and open waits in the editor's shape. */
export function runDataOf(d: WireRunData): RunData {
  const nodeOf = new Map(d.tokens.map((t) => [t.id, t.node_id]))
  const waits: RunWait[] = []
  for (const w of d.waits) {
    const kind = waitKind(w.kind)
    const node = w.node_id || nodeOf.get(w.token_id)
    if (!kind || w.status !== 'open' || !node) continue
    waits.push({ node, token_id: w.token_id, kind, since: d.run.updated_at, timeout_at: w.timeout_at, wait_id: w.wait_id })
  }
  return {
    ...runSummaryOf(d.run),
    tokens: d.tokens.map((t) => ({ token_id: t.id, node: t.node_id || null, state: tokenState(t.status), parent: t.parent, branch: t.branch ?? null, foreach_index: t.foreach_index })),
    steps: d.steps.map((s) => ({
      node: s.node_id, token_id: s.token_id, attempt: s.attempt, input: s.input ?? null, outcome: s.outcome || null, output: s.output ?? null,
      logs: s.logs ?? [], error: s.error ? { code: s.error.code, message: s.error.message } : null,
      started_at: s.started_at, ended_at: s.ended_at, ...(s.hidden ? { hidden: true } : {}),
    })),
    waits,
  }
}

/** The Runs tab's state from a URL: /…/runs/{run}?node=&state=&subject=, or ?tab=runs&state=&subject=. */
export function readRunsView(pathname: string, search: string): RunsView {
  const q = new URLSearchParams(search)
  const m = /\/runs\/([0-9a-fA-F-]{36})\/?$/.exec(pathname)
  const state = (q.get('state') ?? '').split(',').filter((s): s is RunState => (RUN_STATES as readonly string[]).includes(s))
  return { run: m ? m[1]!.toLowerCase() : null, node: m ? q.get('node') : null, state, subject: q.get('subject') ?? '' }
}

/** The URL of the Runs tab's state; the open version (?version=) is kept. */
export function writeRunsView(base: string, id: string, view: RunsView, current: string): string {
  const cur = new URL(current)
  const url = new URL(view.run ? graphPath(base, id, { run: view.run }) : graphPath(base, id, { tab: 'runs' }), cur.origin)
  const version = cur.searchParams.get('version')
  if (version) url.searchParams.set('version', version)
  if (view.run && view.node) url.searchParams.set('node', view.node)
  if (view.state && view.state.length > 0) url.searchParams.set('state', view.state.join(','))
  if (view.subject) url.searchParams.set('subject', view.subject)
  return url.pathname + url.search
}
