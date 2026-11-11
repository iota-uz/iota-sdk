import type { ShellUser } from './shell-host'
import { ShellRegistry, type ShellNavigationItem } from './shell-registry'

export const SHELL_CONTEXT_VERSION = '1.0.0'
export const SHELL_CONTEXT_ELEMENT_ID = 'iota-shell-context'

interface WireNavigationItem {
  key: string
  title: string
  href?: string
  parent?: string
  workspace?: string
  pinned?: boolean
  beta?: boolean
  order?: number
}

export interface ShellContext {
  version: string
  navigation: ShellNavigationItem[]
  user: ShellUser | null
}

function major(version: string): string { return version.split('.', 1)[0] ?? '' }

function parseItem(raw: unknown, index: number): ShellNavigationItem {
  const item = raw as Partial<WireNavigationItem> | null
  if (!item || typeof item.key !== 'string' || typeof item.title !== 'string') {
    throw new Error(`shell context navigation[${index}] requires string key and title`)
  }
  return {
    key: item.key,
    title: item.title,
    ...(item.href ? { href: item.href } : {}),
    ...(item.parent ? { parent: item.parent } : {}),
    ...(item.workspace ? { workspace: item.workspace } : {}),
    ...(typeof item.order === 'number' ? { order: item.order } : {}),
  }
}

/** Returns null when the page carries no shell context (for example a login page). */
export function readShellContext(document: Document, elementID = SHELL_CONTEXT_ELEMENT_ID): ShellContext | null {
  const element = document.getElementById(elementID)
  if (!element?.textContent) return null
  const payload = JSON.parse(element.textContent) as { version?: unknown; navigation?: unknown; user?: { name?: unknown; email?: unknown } }
  if (typeof payload.version !== 'string' || major(payload.version) !== major(SHELL_CONTEXT_VERSION)) {
    throw new Error(`shell context ${String(payload.version)} is incompatible with ${SHELL_CONTEXT_VERSION}`)
  }
  if (!Array.isArray(payload.navigation)) throw new Error('shell context navigation must be an array')
  const user = payload.user && typeof payload.user.name === 'string' && typeof payload.user.email === 'string'
    ? { name: payload.user.name, email: payload.user.email }
    : null
  return { version: payload.version, navigation: payload.navigation.map(parseItem), user }
}

const registries = new WeakMap<Document, ShellRegistry>()

/**
 * One registry per document, seeded from the server-rendered context. Templ
 * pages and Solid routes on the same document share it, which is the common
 * boundary for navigation and region contributions.
 */
export function documentShellRegistry(document: Document): ShellRegistry {
  let registry = registries.get(document)
  if (!registry) {
    registry = new ShellRegistry(readShellContext(document)?.navigation ?? [])
    registries.set(document, registry)
  }
  return registry
}
