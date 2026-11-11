// Wire contract between pkg/extruntime/ui (Go) and the browser runtime. The Go
// types are PageData and RuntimeContext; keep them in step.
import type { TypeJSON } from '../model.ts'
import type { SlotContribution } from './slots.ts'

/** `initial.kind` of the client route context of a runtime page. */
export const PAGE_DATA_KIND = 'iota.runtime.page'
/** Id of the JSON element published on authenticated pages with a shell binding. */
export const RUNTIME_CONTEXT_ELEMENT_ID = 'iota-runtime-context'
export const RUNTIME_CONTEXT_VERSION = '1.0.0'
export const RPC_PATH = '/_ext/rpc'

export interface PageRef {
  id: string
  package: string
  version: string
  /** Immutable asset URL of the page's client chunk. */
  module: string
  /** Export of the module that holds the page; default export when absent. */
  export?: string
}

export interface PageData {
  kind: typeof PAGE_DATA_KIND
  page: PageRef
  /** Path parameters as strings; decode them with `paramTypes`. */
  params: Record<string, string>
  /** Types of the route's parameters when the page implements a typed route. */
  paramTypes?: TypeJSON
  /** Route id whose native screen this page replaces. */
  replacedRoute?: string
  /** Slot contributions by slot ID, resolved for this viewer (activation, order, permissions). */
  slots?: Record<string, SlotContribution[]>
}

export interface ShellRef {
  id: string
  package: string
  version: string
  module: string
  export?: string
  binding: string
}

export interface WorkspaceRef { id: string; label: string; icon?: string; order: number }
export interface PaletteRef { id: string; label: string; href: string; keywords: string[] }

export interface RuntimeUserRef { id: string; name: string; email?: string }

export interface RuntimeContext {
  version: string
  protocolVersion: string
  sdkReleaseVersion: string
  sdkCommit: string
  rpc: string
  locale: string
  permissions: string[]
  user?: RuntimeUserRef
  shell?: ShellRef
  navigation: { workspaces: WorkspaceRef[]; palette: PaletteRef[] }
  /** Slot contributions by slot ID, resolved for this viewer; the shell renderer's `<Slot>` reads them. */
  slots?: Record<string, SlotContribution[]>
}

export function isPageData(value: unknown): value is PageData {
  if (!value || typeof value !== 'object') return false
  const v = value as Partial<PageData>
  return v.kind === PAGE_DATA_KIND && !!v.page && typeof v.page.module === 'string' && typeof v.page.id === 'string'
}

function major(version: string): string { return version.split('.', 1)[0] ?? '' }

/** Returns null when the page carries no runtime context. */
export function readRuntimeContext(document: Document, elementID = RUNTIME_CONTEXT_ELEMENT_ID): RuntimeContext | null {
  const element = document.getElementById(elementID)
  if (!element?.textContent) return null
  const value = JSON.parse(element.textContent) as Partial<RuntimeContext>
  if (typeof value.version !== 'string' || major(value.version) !== major(RUNTIME_CONTEXT_VERSION)) {
    throw new Error(`runtime context ${String(value.version)} is incompatible with ${RUNTIME_CONTEXT_VERSION}`)
  }
  if (typeof value.rpc !== 'string' || !value.navigation) throw new Error('runtime context is incomplete')
  return value as RuntimeContext
}
