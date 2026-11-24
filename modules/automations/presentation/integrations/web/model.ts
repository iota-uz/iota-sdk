import type { RPC } from './Integrations.solid.generated'

export type ListResult = RPC['automations.connectors.list']['result']
export type ConnectorView = ListResult['connectors'][number]
export type ConnectionView = ConnectorView['connections'][number]
export type SaveResult = RPC['automations.connections.create']['result']
export type UsageView = RPC['automations.connections.usage']['result']['usages'][number]
export type Category = ConnectorView['category']
export type Status = ConnectionView['status']

export const MSG = 'Automations.Integrations.'

/** The locale key of a category (go-i18n reserves plural names such as «other» as keys). */
export const CATEGORY_KEY: Record<Category, string> = { messaging: 'Messaging', email: 'Email', ai: 'AI', documents: 'Documents', other: 'Misc' }

export const CATEGORY_ORDER: readonly Category[] = ['messaging', 'email', 'ai', 'documents', 'other']

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

/** Connectors grouped by category in the fixed order; empty groups are left out. */
export function groupByCategory(list: readonly ConnectorView[]): { category: Category; connectors: ConnectorView[] }[] {
  return CATEGORY_ORDER.map((category) => ({ category, connectors: list.filter((c) => (c.category ?? 'other') === category) }))
    .filter((g) => g.connectors.length > 0)
}

/** «Used in N automations» with the plural form of the language (Intl.PluralRules). */
export function usedByText(t: Translator, count: number, lang: string): string {
  if (count === 0) return t('UsedBy.None')
  let rule: string = 'other'
  try { rule = new Intl.PluralRules(lang).select(count) } catch { /* unknown language: other */ }
  const key = rule === 'one' ? 'UsedBy.Singular' : rule === 'few' ? 'UsedBy.Paucal' : rule === 'many' ? 'UsedBy.Multiple' : 'UsedBy.Plural'
  const s = t(key, { count })
  return s === key ? t('UsedBy.Plural', { count }) : s
}

export type FieldKind = 'text' | 'secret' | 'number' | 'boolean' | 'enum' | 'textarea'

export interface FormField {
  name: string
  title: string
  description?: string
  kind: FieldKind
  required: boolean
  options?: string[]
  placeholder?: string
}

interface SchemaProperty {
  type?: string | string[]
  title?: string
  description?: string
  enum?: unknown[]
  format?: string
  'x-secret'?: boolean
  'x-multiline'?: boolean
  examples?: unknown[]
}

/** The form fields of a credentials schema, in declaration order. */
export function schemaFields(schema: unknown): FormField[] {
  if (!schema || typeof schema !== 'object') return []
  const doc = schema as { properties?: Record<string, SchemaProperty>; required?: string[] }
  const required = new Set(doc.required ?? [])
  return Object.entries(doc.properties ?? {}).map(([name, p]) => {
    const type = Array.isArray(p.type) ? p.type.find((x) => x !== 'null') : p.type
    let kind: FieldKind = 'text'
    if (p['x-secret']) kind = 'secret'
    else if (Array.isArray(p.enum) && p.enum.length > 0) kind = 'enum'
    else if (type === 'boolean') kind = 'boolean'
    else if (type === 'integer' || type === 'number') kind = 'number'
    else if (p['x-multiline']) kind = 'textarea'
    const field: FormField = { name, title: p.title || humanize(name), kind, required: required.has(name) }
    if (p.description) field.description = p.description
    if (kind === 'enum') field.options = (p.enum ?? []).map(String)
    const example = p.examples?.[0]
    if (kind !== 'secret' && typeof example === 'string') field.placeholder = example
    return field
  })
}

function humanize(name: string): string {
  const s = name.replace(/[_-]+/g, ' ').trim()
  return s.charAt(0).toUpperCase() + s.slice(1)
}

/** A system name from a display name: Latin letters, digits and _, starting with a letter. */
export function slugify(display: string, fallback: string): string {
  const map: Record<string, string> = {
    а: 'a', б: 'b', в: 'v', г: 'g', д: 'd', е: 'e', ё: 'yo', ж: 'zh', з: 'z', и: 'i', й: 'y', к: 'k', л: 'l', м: 'm', н: 'n', о: 'o',
    п: 'p', р: 'r', с: 's', т: 't', у: 'u', ф: 'f', х: 'h', ц: 'ts', ч: 'ch', ш: 'sh', щ: 'sch', ъ: '', ы: 'y', ь: '', э: 'e', ю: 'yu',
    я: 'ya', ў: 'o', қ: 'q', ғ: 'g', ҳ: 'h', ʻ: '', ʼ: '', "'": '',
  }
  let out = ''
  for (const ch of display.toLowerCase()) {
    if (/[a-z0-9]/.test(ch)) out += ch
    else if (ch in map) out += map[ch]
    else out += '_'
  }
  out = out.replace(/_+/g, '_').replace(/^_+|_+$/g, '')
  if (!/^[a-z]/.test(out)) out = fallback.replace(/[^a-z0-9_]/g, '_') + (out ? '_' + out : '')
  return out.slice(0, 40).replace(/_+$/, '')
}

export const NAME_RE = /^[a-z][a-z0-9_]{0,39}$/

/** The localized text of a field error code from the server or the form. */
export function fieldErrorText(t: Translator, code: string): string {
  switch (code) {
    case 'required': return t('Form.Required')
    case 'name': return t('Form.NameFormat')
    case 'taken': return t('Form.NameTaken')
    case 'in_use': return t('Form.BotInUse')
    default: return t('Form.Invalid')
  }
}

/** A provider's refusal in words with the step to fix it; never the provider's own text. */
export function reasonText(t: Translator, reason: string | null | undefined): string {
  const r = reason && ['auth', 'network', 'rate_limit', 'config', 'reauth', 'unknown'].includes(reason) ? reason : 'unknown'
  return `${t(`Reasons.${r}.Text`)}. ${t(`Reasons.${r}.Fix`)}`
}

/** reasonText as a sentence of its own (the status line of a connection). */
export function reasonSentence(t: Translator, reason: string | null | undefined): string {
  const s = reasonText(t, reason)
  return s.charAt(0).toLocaleUpperCase() + s.slice(1)
}

export const OAUTH_ERRORS = ['denied', 'state', 'exchange', 'unavailable', 'refused', 'name_taken', 'version_conflict', 'not_found'] as const

/** The phrase of an OAuth2 return code. */
export function oauthErrorText(t: Translator, code: string): string {
  return t(`OAuth.${(OAUTH_ERRORS as readonly string[]).includes(code) ? code : 'exchange'}`)
}

/** The phrase of a failed save; never a raw code or the provider's text. */
export function saveErrorText(t: Translator, r: SaveResult): string {
  switch (r.error) {
    case 'test_failed': return t('Form.TestFailed', { message: reasonText(t, r.reason) })
    case 'version_conflict': return t('Form.VersionConflict')
    case 'in_progress': return t('Form.InProgress')
    case 'unavailable': return t('Form.Unavailable')
    case 'name_taken': return t('Form.NameTaken')
    case 'invalid_input': return t('Form.Invalid')
    default: return t('Form.Failed')
  }
}

/** The code an RPC error carries in its details (not_found, unavailable, forbidden). */
export function errorCode(error: unknown): string | undefined {
  const details = (error as { details?: unknown } | undefined)?.details
  if (details && typeof details === 'object' && 'code' in details) return String((details as { code: unknown }).code)
  const code = (error as { code?: unknown } | undefined)?.code
  return typeof code === 'string' ? code : undefined
}

/** The editor link of a use: the graph, focused on its first node. */
export function usageHref(base: string, u: UsageView): string {
  const node = u.node_ids[0]
  return `${base.replace(/\/$/, '')}/${encodeURIComponent(u.automation_id)}${node ? `?node=${encodeURIComponent(node)}` : ''}`
}

/** The credentials an edit form sends: undefined when nothing changed (kept on the server). */
export function changedCredentials(fields: readonly FormField[], values: Record<string, string>, initial: Record<string, string>, replaced: ReadonlySet<string>): Record<string, unknown> | undefined {
  const out: Record<string, unknown> = {}
  let changed = false
  for (const f of fields) {
    const v = values[f.name] ?? ''
    if (f.kind === 'secret') {
      if (replaced.has(f.name) && v !== '') { out[f.name] = v; changed = true }
      continue
    }
    // Numbers go as typed text: an ID beyond 2^53 (a Telegram chat) must not pass through a JS number.
    out[f.name] = f.kind === 'boolean' ? v === 'true' : v
    if (v !== (initial[f.name] ?? '')) changed = true
  }
  return changed ? out : undefined
}

/** The credentials a new connection sends. */
export function newCredentials(fields: readonly FormField[], values: Record<string, string>): Record<string, unknown> {
  const out: Record<string, unknown> = {}
  for (const f of fields) {
    const v = values[f.name] ?? ''
    if (v === '' && f.kind !== 'boolean') continue
    out[f.name] = f.kind === 'boolean' ? v === 'true' : v
  }
  return out
}

/** Required fields left empty, by name; a secret already set on the server counts as filled. */
export function missingRequired(fields: readonly FormField[], values: Record<string, string>, secretSet: (name: string) => boolean): Record<string, string> {
  const out: Record<string, string> = {}
  for (const f of fields) {
    if (!f.required || f.kind === 'boolean') continue
    if (f.kind === 'secret' && secretSet(f.name)) continue
    if ((values[f.name] ?? '').trim() === '') out[f.name] = 'required'
  }
  return out
}
