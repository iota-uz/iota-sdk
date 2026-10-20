import { createEffect, createSignal, onCleanup, type Accessor } from 'solid-js'
import type { SaveIntent } from './draft'

export type ReportLeaveChoice = 'save' | 'discard' | 'stay'

/** The part of a report draft a leave guard needs. */
export interface GuardedDraft {
  dirty: Accessor<boolean>
  cancel(): void
  save(intent?: SaveIntent): Promise<unknown>
}

export interface ReportLeaveGuard {
  /** A leave request is waiting for the reader's choice. */
  pending: Accessor<boolean>
  /** The save chosen in the dialog is in flight. */
  saving: Accessor<boolean>
  /** The save chosen in the dialog failed; the request stays open. */
  failed: Accessor<boolean>
  /**
   * Asks before the host navigates away. Resolves true when leaving may
   * proceed: nothing was unsaved, the reader discarded, or the save succeeded.
   * A failed save never resolves true.
   */
  request(): Promise<boolean>
  choose(choice: ReportLeaveChoice): Promise<void>
}

/**
 * Guards in-app navigation and browser unload while a draft has unsaved
 * changes. Browser unload uses the browser's own confirmation; in-app
 * navigation awaits request() and the editor renders the choice.
 */
export function createReportLeaveGuard(draft: GuardedDraft, options: { intent?: SaveIntent; beforeUnload?: boolean } = {}): ReportLeaveGuard {
  const [pending, setPending] = createSignal(false)
  const [saving, setSaving] = createSignal(false)
  const [failed, setFailed] = createSignal(false)
  let settle: ((leave: boolean) => void) | undefined

  if (options.beforeUnload !== false && typeof window !== 'undefined') {
    createEffect(() => {
      if (!draft.dirty()) return
      const warn = (event: BeforeUnloadEvent) => {
        event.preventDefault()
        event.returnValue = ''
      }
      window.addEventListener('beforeunload', warn)
      onCleanup(() => window.removeEventListener('beforeunload', warn))
    })
  }

  const finish = (leave: boolean) => {
    const resolve = settle
    settle = undefined
    setPending(false)
    setFailed(false)
    resolve?.(leave)
  }

  return {
    pending,
    saving,
    failed,
    request() {
      if (!draft.dirty()) return Promise.resolve(true)
      settle?.(false)
      setPending(true)
      setFailed(false)
      return new Promise<boolean>((resolve) => { settle = resolve })
    },
    async choose(choice) {
      if (!pending() || saving()) return
      if (choice === 'stay') return finish(false)
      if (choice === 'discard') {
        draft.cancel()
        return finish(true)
      }
      setSaving(true)
      setFailed(false)
      try {
        await draft.save(options.intent ?? 'draft')
        finish(true)
      } catch {
        setFailed(true)
      } finally {
        setSaving(false)
      }
    },
  }
}
