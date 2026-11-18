import { createRoot } from 'solid-js'
import { describe, expect, it } from 'vitest'
import { createLoader } from './load'

function deferred<T>() {
  let resolve!: (v: T) => void
  let reject!: (e: unknown) => void
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}
const tick = () => new Promise((r) => setTimeout(r, 0))

describe('createLoader', () => {
  it('shows the newest response even when an older one arrives later', async () => {
    const calls = [deferred<string>(), deferred<string>()]
    let n = 0
    await createRoot(async (dispose) => {
      const l = createLoader(() => calls[n++]!.promise)
      void l.reload()
      calls[1]!.resolve('new')
      await tick()
      expect(l.data()).toBe('new')
      calls[0]!.resolve('old')
      await tick()
      expect(l.data()).toBe('new')
      expect(l.loading()).toBe(false)
      dispose()
    })
  })

  it('aborts the superseded request', async () => {
    const signals: AbortSignal[] = []
    await createRoot(async (dispose) => {
      const l = createLoader((s) => { signals.push(s); return new Promise<string>(() => {}) })
      void l.reload()
      expect(signals[0]!.aborted).toBe(true)
      expect(signals[1]!.aborted).toBe(false)
      dispose()
      expect(signals[1]!.aborted).toBe(true)
    })
  })

  it('ignores a response that arrives after the owner was disposed', async () => {
    const d = deferred<string>()
    let loader!: ReturnType<typeof createLoader<string>>
    const dispose = createRoot((dispose) => { loader = createLoader(() => d.promise); return dispose })
    dispose()
    d.resolve('late')
    await tick()
    expect(loader.data()).toBeUndefined()
  })

  it('reports an error and recovers on reload', async () => {
    let fail = true
    await createRoot(async (dispose) => {
      const l = createLoader(async () => { if (fail) throw new Error('boom'); return 'ok' })
      await tick()
      expect(l.error()?.message).toBe('boom')
      fail = false
      await l.reload()
      expect(l.data()).toBe('ok')
      expect(l.error()).toBeUndefined()
      dispose()
    })
  })
})
