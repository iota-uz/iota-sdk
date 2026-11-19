/**
 * The drill return protocol, mirrored from Go `pkg/lens/drillreturn`.
 *
 * When a reader leaves a drill level for a host page (operational records),
 * the runtime adds `lens-return` to the destination: a versioned token with
 * the site-relative URL that restores the exploration (levels, perspective
 * and filters live in that URL, so it works in a new tab), the dashboard
 * title and the level labels. The host renders a Back link to `backURL`.
 * The host's own state (sort, page, scroll) comes back as an opaque
 * `lens-host` token keyed by its leaf path; the runtime keeps it in its URL
 * and hands the same state back the next time that leaf is opened.
 * fixtures/return-context.json is the shared acceptance fixture.
 */
export const returnParam = 'lens-return'
export const hostStateParam = 'lens-host'
export const returnProtocolVersion = 1
const maxEncodedBytes = 6 << 10

export interface LensReturnContext {
  v: 1
  /** Site-relative Lens URL that restores the exploration. */
  href: string
  /** The dashboard or report title the reader came from. */
  title: string
  /** Labels of the drill levels, outermost first. */
  trail?: Array<string>
  /** The opaque state this leaf's host handed back earlier. */
  host?: string
}

export interface LensHostState {
  v: 1
  /** The host page path the state belongs to. */
  leaf: string
  state: string
}

function toBase64URL(text: string): string {
  const bytes = new TextEncoder().encode(text)
  let binary = ''
  for (const byte of bytes) binary += String.fromCharCode(byte)
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

function fromBase64URL(token: string): string | undefined {
  if (!/^[A-Za-z0-9_-]+$/.test(token)) return undefined
  try {
    const padded = token.replace(/-/g, '+').replace(/_/g, '/') + '='.repeat((4 - (token.length % 4)) % 4)
    const binary = atob(padded)
    const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0))
    return new TextDecoder('utf-8', { fatal: true }).decode(bytes)
  } catch { return undefined }
}

/** A path on this site: one leading slash, no scheme or host. */
export function isSiteRelative(href: string): boolean {
  if (!href.startsWith('/') || href.startsWith('//') || /[\\\r\n\t]/.test(href)) return false
  try {
    const base = 'https://lens.invalid'
    return new URL(href, base).origin === base
  } catch { return false }
}

function validContext(value: unknown): value is LensReturnContext {
  if (!value || typeof value !== 'object') return false
  const candidate = value as Record<string, unknown>
  const allowed = new Set(['v', 'href', 'title', 'trail', 'host'])
  if (Object.keys(candidate).some((key) => !allowed.has(key))) return false
  if (candidate.v !== returnProtocolVersion) return false
  if (typeof candidate.href !== 'string' || !isSiteRelative(candidate.href)) return false
  if (typeof candidate.title !== 'string' || candidate.title.trim() === '') return false
  if (candidate.trail !== undefined && (!Array.isArray(candidate.trail) || candidate.trail.some((label) => typeof label !== 'string'))) return false
  if (candidate.host !== undefined && typeof candidate.host !== 'string') return false
  return true
}

function validHostState(value: unknown): value is LensHostState {
  if (!value || typeof value !== 'object') return false
  const candidate = value as Record<string, unknown>
  if (Object.keys(candidate).some((key) => key !== 'v' && key !== 'leaf' && key !== 'state')) return false
  return candidate.v === returnProtocolVersion && typeof candidate.leaf === 'string' && isSiteRelative(candidate.leaf)
    && typeof candidate.state === 'string' && candidate.state !== ''
}

function encode(value: object): string | undefined {
  const token = toBase64URL(JSON.stringify(value))
  return token.length > maxEncodedBytes ? undefined : token
}

function decode(token: string | null | undefined): unknown {
  if (!token || token.length > maxEncodedBytes) return undefined
  const text = fromBase64URL(token)
  if (text === undefined) return undefined
  try { return JSON.parse(text) as unknown } catch { return undefined }
}

/** The token for a context, or undefined when it is invalid or too large. Field order matches Go. */
export function encodeReturnContext(context: Omit<LensReturnContext, 'v'> & { v?: 1 }): string | undefined {
  const value: LensReturnContext = {
    v: 1,
    href: context.href,
    title: context.title,
    ...(context.trail && context.trail.length > 0 ? { trail: [...context.trail] } : {}),
    ...(context.host ? { host: context.host } : {}),
  }
  return validContext(value) ? encode(value) : undefined
}

export function decodeReturnContext(token: string | null | undefined): LensReturnContext | undefined {
  const value = decode(token)
  return validContext(value) ? value : undefined
}

export function encodeHostState(state: Omit<LensHostState, 'v'> & { v?: 1 }): string | undefined {
  const value: LensHostState = { v: 1, leaf: state.leaf, state: state.state }
  return validHostState(value) ? encode(value) : undefined
}

export function decodeHostState(token: string | null | undefined): LensHostState | undefined {
  const value = decode(token)
  return validHostState(value) ? value : undefined
}

/** The return context of a host page opened from Lens, or undefined. */
export function readReturnContext(location: URL | string): LensReturnContext | undefined {
  const url = typeof location === 'string' ? new URL(location, 'https://lens.invalid') : location
  return decodeReturnContext(url.searchParams.get(returnParam))
}

/**
 * The link back to the exploration. A non-empty `state` is attached for
 * `leaf` (the host page's own path) and returned on its next visit.
 */
export function backURL(context: LensReturnContext, leaf: string, state?: string): string | undefined {
  if (!validContext(context)) return undefined
  const base = 'https://lens.invalid'
  const target = new URL(context.href, base)
  target.searchParams.delete(hostStateParam)
  if (state) {
    const token = encodeHostState({ leaf, state })
    if (!token) return undefined
    target.searchParams.set(hostStateParam, token)
  }
  return `${target.pathname}${target.search}${target.hash}`
}

/**
 * Adds the return context to a host destination. Only same-origin page
 * links are decorated; a drawer hash link stays inside Lens and keeps its
 * own navigation. The host state kept in the Lens URL is returned only to
 * the leaf it belongs to.
 */
export function withReturnContext(
  href: string,
  current: URL,
  context: { title: string; trail: Array<string> } | undefined,
): string {
  if (!context) return href
  let target: URL
  try { target = new URL(href, current) } catch { return href }
  if (target.origin !== current.origin) return href
  if (target.pathname === current.pathname && target.hash.startsWith('#lens-drawer=')) return href
  const lens = new URL(current)
  lens.searchParams.delete(returnParam)
  const kept = decodeHostState(lens.searchParams.get(hostStateParam))
  lens.searchParams.delete(hostStateParam)
  const token = encodeReturnContext({
    href: `${lens.pathname}${lens.search}`,
    title: context.title,
    trail: context.trail,
    ...(kept && kept.leaf === target.pathname ? { host: kept.state } : {}),
  })
  if (!token) return href
  target.searchParams.set(returnParam, token)
  return target.href
}
