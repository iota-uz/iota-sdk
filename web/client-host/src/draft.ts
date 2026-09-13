import { createSignal, onCleanup } from 'solid-js'
import { createStore, reconcile, unwrap, type SetStoreFunction, type Store } from 'solid-js/store'
import { stableSerialize } from './cache'
import { asHostError, type HostError } from './errors'
import type { NavigationService } from './navigation'

function clone<T>(value: T): T { return structuredClone(unwrap(value)) }

export interface DraftOptions {
  navigation?: NavigationService
}

export function createDraft<T extends object>(initial: T, options: DraftOptions = {}) {
  const [value, setStore] = createStore(clone(initial))
  const [baseline, setBaseline] = createSignal(clone(initial))
  const [revision, setRevision] = createSignal(0)
  const [pending, setPending] = createSignal(false)
  const [error, setError] = createSignal<HostError>()
  let saveGeneration = 0
  let saveController: AbortController | undefined
  const dirty = () => {
    revision()
    return stableSerialize(value) !== stableSerialize(baseline())
  }
  const releaseGuard = options.navigation?.guard(() => !dirty())
  onCleanup(() => {
    saveController?.abort()
    releaseGuard?.()
  })

  const set: SetStoreFunction<T> = ((...args: unknown[]) => {
    setRevision((current) => current + 1)
    setError(undefined)
    ;(setStore as (...values: unknown[]) => void)(...args)
  }) as SetStoreFunction<T>

  const accept = (snapshot: T) => {
    const next = clone(snapshot)
    setStore(reconcile(next))
    setBaseline(() => clone(next))
    setRevision((current) => current + 1)
    setError(undefined)
  }

  const reset = () => accept(baseline())
  const offerServerSnapshot = (snapshot: T): boolean => {
    if (dirty()) return false
    accept(snapshot)
    return true
  }
  const save = async (persist: (snapshot: T, signal: AbortSignal) => Promise<T>): Promise<boolean> => {
    const currentGeneration = ++saveGeneration
    const startRevision = revision()
    const snapshot = clone(value as T)
    saveController?.abort()
    saveController = new AbortController()
    setPending(true)
    setError(undefined)
    try {
      const saved = await persist(snapshot, saveController.signal)
      if (currentGeneration !== saveGeneration || saveController.signal.aborted || revision() !== startRevision) return false
      accept(saved)
      return true
    } catch (cause) {
      if (currentGeneration === saveGeneration && !saveController.signal.aborted) setError(asHostError(cause))
      throw cause
    } finally {
      if (currentGeneration === saveGeneration) setPending(false)
    }
  }

  return {
    value: value as Store<T>, set, baseline, dirty, pending, error,
    fieldErrors: () => error()?.fieldErrors ?? {}, accept, reset, offerServerSnapshot, save,
    cancelSave: () => saveController?.abort(),
  }
}
