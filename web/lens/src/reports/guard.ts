import { createEffect, createRoot, createSignal, onCleanup, type Accessor } from 'solid-js'
import type { SaveIntent } from './draft'

export type ReportLeaveChoice = 'save' | 'discard' | 'stay'

/** The part of a report draft a leave guard needs. */
export interface GuardedDraft {
  dirty: Accessor<boolean>
  /** A save is in flight; the guard waits for it instead of competing with it. */
  saving: Accessor<boolean>
  cancel(): void
  save(intent?: SaveIntent): Promise<unknown>
}

export interface ReportLeaveGuard {
  /** A leave request is waiting for the reader's choice. */
  pending: Accessor<boolean>
  /** The choice is being carried out: an earlier save is settling or the chosen save is in flight. */
  saving: Accessor<boolean>
  /** The chosen action failed; the request stays open. */
  failed: Accessor<boolean>
  /** Which choice failed, for the message shown with it. */
  failure: Accessor<'save' | 'discard' | undefined>
  /**
   * Asks before the host navigates away. Resolves true when leaving may
   * proceed: nothing was unsaved, an in-flight save completed with nothing
   * left unsaved, the reader discarded, or the chosen save succeeded. A
   * failed save never resolves true.
   */
  request(): Promise<boolean>
  /** Carries out the reader's choice. It never rejects; a failure is reported through failed. */
  choose(choice: ReportLeaveChoice): Promise<void>
}

/** Resolves once the draft has no save in flight. */
function settled(draft: GuardedDraft): Promise<void> {
  if (!draft.saving()) return Promise.resolve()
  return new Promise((resolve) => {
    createRoot((dispose) => {
      createEffect(() => {
        if (draft.saving()) return
        dispose()
        resolve()
      })
    })
  })
}

/**
 * Guards in-app navigation and browser unload while a draft has unsaved
 * changes. Browser unload uses the browser's own confirmation; in-app
 * navigation awaits request() and the editor renders the choice.
 */
export function createReportLeaveGuard(draft: GuardedDraft, options: { intent?: SaveIntent; beforeUnload?: boolean } = {}): ReportLeaveGuard {
  const [pending, setPending] = createSignal(false)
  const [saving, setSaving] = createSignal(false)
  const [failure, setFailure] = createSignal<'save' | 'discard'>()
  let settle: ((leave: boolean) => void) | undefined
  let request = 0

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
    setFailure(undefined)
    resolve?.(leave)
  }

  return {
    pending,
    saving,
    failed: () => failure() !== undefined,
    failure,
    async request() {
      const current = ++request
      if (draft.saving()) await settled(draft)
      if (current !== request) return false
      if (!draft.dirty()) return true
      settle?.(false)
      setPending(true)
      setFailure(undefined)
      return new Promise<boolean>((resolve) => { settle = resolve })
    },
    async choose(choice) {
      if (!pending() || saving()) return
      if (choice === 'stay') return finish(false)
      setSaving(true)
      setFailure(undefined)
      try {
        await settled(draft)
        if (!draft.dirty()) return finish(true)
        if (choice === 'discard') draft.cancel()
        else await draft.save(options.intent ?? 'draft')
        finish(true)
      } catch (cause: unknown) {
        console.error(`[lens] leave guard: ${choice} failed`, cause)
        setFailure(choice)
      } finally {
        setSaving(false)
      }
    },
  }
}
