import { createContext, useContext } from 'solid-js'
import type { ClientHostServices } from '@iota-uz/sdk/solid'
import type { ManageApi } from './api'

export interface Rights {
  author: boolean
  review: boolean
  publish: boolean
  operate: boolean
}

export type NoticeKind = 'info' | 'error'

export interface Ctx {
  api: ManageApi
  host: ClientHostServices
  rights: Rights
  typesBase: string
  /** t translates a key of Extensions.Configurator.*. */
  t(key: string): string
  notify(kind: NoticeKind, text: string): void
}

export const ConfigCtx = createContext<Ctx>()

export function useConfig(): Ctx {
  const c = useContext(ConfigCtx)
  if (!c) throw new Error('configurator context is missing')
  return c
}

export const MSG_PREFIX = 'Extensions.Configurator.'

export function rightsOf(permissions: readonly string[] | undefined): Rights {
  const has = new Set(permissions ?? [])
  return {
    author: has.has('Extensions.Author'), review: has.has('Extensions.Review'),
    publish: has.has('Extensions.Publish'), operate: has.has('Extensions.Operate'),
  }
}

/** describeError turns a failure into the text shown to the user. */
export function describeError(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

/** shortHash abbreviates a content hash for display; the full value is in the title. */
export function shortHash(h: string | undefined): string {
  if (!h) return ''
  const bare = h.replace(/^src-/, '')
  return bare.length > 12 ? bare.slice(0, 12) : bare
}

export function formatTime(iso: string | undefined | null, locale?: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString(locale)
}
