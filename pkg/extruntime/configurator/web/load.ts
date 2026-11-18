import { createSignal, onCleanup, type Accessor } from 'solid-js'

export interface Loader<T> {
  data: Accessor<T | undefined>
  error: Accessor<Error | undefined>
  loading: Accessor<boolean>
  /** reload fetches again; the response of any earlier call is ignored. */
  reload(): Promise<void>
}

/**
 * createLoader runs fn now and on every reload. Only the newest call may set
 * the result: a slow earlier response, or one that returns after the owner was
 * disposed, is dropped, and an in-flight request is aborted when superseded.
 */
export function createLoader<T>(fn: (signal: AbortSignal) => Promise<T>): Loader<T> {
  const [data, setData] = createSignal<T>()
  const [error, setError] = createSignal<Error>()
  const [loading, setLoading] = createSignal(false)
  let generation = 0
  let controller: AbortController | undefined
  let disposed = false

  const reload = async (): Promise<void> => {
    const current = ++generation
    controller?.abort()
    controller = new AbortController()
    const signal = controller.signal
    setLoading(true)
    try {
      const value = await fn(signal)
      if (disposed || current !== generation) return
      setData(() => value)
      setError(undefined)
    } catch (err) {
      if (disposed || current !== generation || signal.aborted) return
      setError(err instanceof Error ? err : new Error(String(err)))
    } finally {
      if (!disposed && current === generation) setLoading(false)
    }
  }
  onCleanup(() => {
    disposed = true
    controller?.abort()
  })
  void reload()
  return { data, error, loading, reload }
}
