import { createMemo, createSignal } from 'solid-js'
import { unwrap } from 'solid-js/store'
import { ContextBoardSchema } from '../contract/schemas'
import type { ContextBoard, ContextFilters } from '../contract/types'

export const filterContextParam = 'lens-filters'
const copy = <T,>(value: T): T => structuredClone(unwrap(value))

export function encodeFilterBoard(board: ContextBoard): string {
  const bytes = new TextEncoder().encode(JSON.stringify(ContextBoardSchema.parse(board)))
  if (bytes.length > 16384) throw new Error('Filter context exceeds its limit')
  return btoa(String.fromCharCode(...bytes)).replaceAll('+', '-').replaceAll('/', '_').replace(/=+$/, '')
}

export function decodeFilterBoard(value: string): ContextBoard {
  if (value.length > 21846 || !/^[A-Za-z0-9_-]+$/.test(value)) throw new Error('Invalid filter context')
  const raw = atob(value.replaceAll('-', '+').replaceAll('_', '/'))
  const bytes = Uint8Array.from(raw, (character) => character.charCodeAt(0))
  if (bytes.length > 16384) throw new Error('Filter context exceeds its limit')
  return ContextBoardSchema.parse(JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(bytes)))
}

export function filterBoardURL(base: string, board: ContextBoard): string {
  const target = new URL(base, window.location.href)
  target.searchParams.set(filterContextParam, encodeFilterBoard(board))
  for (const key of ['period', 'range-start', 'range-end', 'RangeStart', 'RangeEnd', 'issue_at_from', 'issue_at_to', '_f', 'lens-state']) target.searchParams.delete(key)
  return target.pathname + target.search + target.hash
}

/** Apply is temporary; only an explicit host acknowledgement advances saved defaults. */
export function createFilterContextDraft(initial: ContextFilters, saved: ContextFilters, host: {
  apply: (filters: ContextFilters) => void
  saveDefaults?: (filters: ContextFilters) => Promise<void>
}) {
  const [applied, setApplied] = createSignal(copy(initial))
  const [defaults, setDefaults] = createSignal(copy(saved))
  const [draft, setDraft] = createSignal(copy(initial))
  const [saving, setSaving] = createSignal(false)
  const [error, setError] = createSignal<unknown>()
  return {
    value: () => copy(draft()), applied: () => copy(applied()), saving, error,
    dirty: createMemo(() => JSON.stringify(draft()) !== JSON.stringify(applied())),
    update(change: (value: ContextFilters) => ContextFilters) { setDraft(() => copy(change(copy(draft())))); setError(undefined) },
    reset() { setDraft(() => copy(defaults())); setError(undefined) },
    cancel() { setDraft(() => copy(applied())); setError(undefined) },
    apply() { const submitted = copy(draft()); host.apply(copy(submitted)); setApplied(() => submitted) },
    async saveDefaults() {
      if (!host.saveDefaults || saving()) throw new Error('Saving filter defaults is unavailable')
      const submitted = copy(draft()); setSaving(true); setError(undefined)
      try { await host.saveDefaults(copy(submitted)); setDefaults(() => submitted) }
      catch (cause) { setError(cause); throw cause }
      finally { setSaving(false) }
    },
  }
}
