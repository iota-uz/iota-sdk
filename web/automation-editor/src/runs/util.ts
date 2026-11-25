// Helpers of the runs view: one request state for every action (F1–F3), times by locale, sizes,
// subject names and the not-found answer.
import { createSignal, onCleanup, type Accessor } from 'solid-js'
import { intlLocale } from '../locale'
import type { EditorLocale, Subject } from '../types'

export const SLOW_MS = 4000

export type ActionState = { status: 'idle' } | { status: 'busy'; slow: boolean } | { status: 'failed' }

/**
 * One state for an action (F1–F3): idle → busy → slow (after 4 s) → failed | idle. A second start while
 * busy is ignored: exactly one request is in flight. A failure keeps the state until the next start.
 */
export function createAction(): { state: Accessor<ActionState>; busy: () => boolean; run<T>(fn: () => Promise<T>): Promise<T | undefined>; reset(): void } {
  const [state, setState] = createSignal<ActionState>({ status: 'idle' })
  let timer: ReturnType<typeof setTimeout> | undefined
  let alive = true
  onCleanup(() => {
    alive = false
    clearTimeout(timer)
  })
  const busy = () => state().status === 'busy'
  return {
    state,
    busy,
    reset: () => setState({ status: 'idle' }),
    async run<T>(fn: () => Promise<T>): Promise<T | undefined> {
      if (busy()) return undefined
      setState({ status: 'busy', slow: false })
      timer = setTimeout(() => {
        if (alive) setState((s) => (s.status === 'busy' ? { status: 'busy', slow: true } : s))
      }, SLOW_MS)
      try {
        const result = await fn()
        clearTimeout(timer)
        if (alive) setState({ status: 'idle' })
        return result
      } catch (error) {
        clearTimeout(timer)
        console.warn('automation-editor: action failed', error)
        if (alive) setState({ status: 'failed' })
        throw error
      }
    },
  }
}

/** A clock that ticks once a minute: relative times share it (S6). */
export function createMinuteClock(): Accessor<number> {
  const [now, setNow] = createSignal(Date.now())
  const timer = setInterval(() => setNow(Date.now()), 60_000)
  onCleanup(() => clearInterval(timer))
  return now
}

export function relativeTime(iso: string, now: number, locale: EditorLocale): string {
  const at = Date.parse(iso)
  if (Number.isNaN(at)) return ''
  const seconds = Math.round((at - now) / 1000)
  const fmt = new Intl.RelativeTimeFormat(intlLocale(locale), { numeric: 'auto' })
  const abs = Math.abs(seconds)
  if (abs < 60) return fmt.format(seconds, 'second')
  if (abs < 3600) return fmt.format(Math.round(seconds / 60), 'minute')
  if (abs < 86400) return fmt.format(Math.round(seconds / 3600), 'hour')
  return fmt.format(Math.round(seconds / 86400), 'day')
}

export function absoluteTime(iso: string, locale: EditorLocale): string {
  const at = Date.parse(iso)
  if (Number.isNaN(at)) return ''
  return new Intl.DateTimeFormat(intlLocale(locale), { dateStyle: 'medium', timeStyle: 'medium' }).format(at)
}

export function shortTime(iso: string, locale: EditorLocale): string {
  const at = Date.parse(iso)
  if (Number.isNaN(at)) return ''
  return new Intl.DateTimeFormat(intlLocale(locale), { hour: '2-digit', minute: '2-digit', second: '2-digit' }).format(at)
}

export function duration(fromIso: string, toIso: string | null, locale: EditorLocale): string | null {
  if (!toIso) return null
  const ms = Date.parse(toIso) - Date.parse(fromIso)
  if (!Number.isFinite(ms) || ms < 0) return null
  const unit = ms < 1000 ? { value: ms, unit: 'millisecond' } : ms < 60_000 ? { value: Math.round(ms / 100) / 10, unit: 'second' } : { value: Math.round(ms / 6000) / 10, unit: 'minute' }
  return new Intl.NumberFormat(intlLocale(locale), { style: 'unit', unit: unit.unit, unitDisplay: 'short' }).format(unit.value)
}

export function formatBytes(bytes: number, locale: EditorLocale): string {
  const kb = bytes / 1024
  const fmt = (value: number, unit: string) => new Intl.NumberFormat(intlLocale(locale), { style: 'unit', unit, unitDisplay: 'short', maximumFractionDigits: 1 }).format(value)
  return kb < 1024 ? fmt(kb, 'kilobyte') : fmt(kb / 1024, 'megabyte')
}

/** The name of a run's subject: its label, else its key; null for a run without one. */
export function subjectName(subject: Subject | null): string | null {
  if (!subject || !subject.entity || subject.entity === 'none') return null
  return subject.label || subject.key || null
}

/** A rejection that says the object is not there (deleted, or another tenant's). */
export function isNotFound(error: unknown): boolean {
  if (!error || typeof error !== 'object') return false
  const e = error as { code?: unknown; data?: { code?: unknown }; name?: unknown; status?: unknown }
  return e.code === 'not_found' || e.data?.code === 'not_found' || e.name === 'RunNotFound' || e.status === 404
}

/** JSON text of a value for display and copy (never interpreted as HTML). */
export function jsonText(value: unknown): string {
  if (typeof value === 'string') return value
  try {
    return JSON.stringify(value, null, 2) ?? ''
  } catch {
    return String(value)
  }
}
