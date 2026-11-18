import { createComponent, createRoot, createSignal } from 'solid-js'
import { describe, expect, it, vi } from 'vitest'
import { RemoteError } from '../client.ts'
import { command, query } from '../contracts.ts'
import { t } from '../schema.ts'
import { RuntimeProvider, useCommand, useQuery } from './hooks.ts'
import type { RuntimeTransport } from './transport.ts'

const Expiring = query('acme.insurance.expiring', { input: t.object({ days: t.int() }), output: t.object({ count: t.int() }) })
const Renew = command('acme.insurance.renew', { input: t.object({ id: t.string() }), output: t.object({ ok: t.bool() }) })

function deferred<T>() {
  let resolve!: (v: T) => void
  let reject!: (e: unknown) => void
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}

const tick = () => new Promise<void>((r) => setTimeout(r, 0))

/** Runs fn as the body of a page component under the runtime provider. */
function inPage<T>(transport: RuntimeTransport, fn: () => T): { value: T; dispose: () => void } {
  let value!: T
  const dispose = createRoot((dispose) => {
    createComponent(RuntimeProvider, {
      transport,
      get children() { value = fn(); return null as never },
    })
    return dispose
  })
  return { value, dispose }
}

describe('useQuery', () => {
  it('encodes the input, decodes the typed output and reports loading', async () => {
    const invoke = vi.fn(async () => ({ count: 4 }))
    const { value: q, dispose } = inPage({ invoke }, () => useQuery(Expiring, { days: 30 }))
    expect(q.loading()).toBe(true)
    await tick()
    expect(q.data()).toEqual({ count: 4 })
    expect(q.loading()).toBe(false)
    expect(invoke).toHaveBeenCalledWith('query', 'acme.insurance.expiring', { days: 30 }, expect.objectContaining({ signal: expect.any(AbortSignal) }))
    dispose()
  })

  it('ignores a late response of a superseded request', async () => {
    const first = deferred<unknown>()
    const second = deferred<unknown>()
    const calls = [first, second]
    const invoke = vi.fn(() => calls.shift()!.promise as Promise<never>)
    const [days, setDays] = createSignal(1)
    const { value: q, dispose } = inPage({ invoke }, () => useQuery(Expiring, () => ({ days: days() })))
    await tick()
    setDays(2)
    await tick()
    expect(invoke).toHaveBeenCalledTimes(2)
    second.resolve({ count: 22 })
    await tick()
    first.resolve({ count: 11 }) // arrives last, belongs to the old input
    await tick()
    expect(q.data()).toEqual({ count: 22 })
    dispose()
  })

  it('aborts the superseded request and ignores any response after disposal', async () => {
    const pending = deferred<unknown>()
    let signal: AbortSignal | undefined
    const invoke = vi.fn(async (_k, _r, _i, options?: { signal?: AbortSignal }) => { signal = options?.signal; return pending.promise as Promise<never> })
    const { value: q, dispose } = inPage({ invoke: invoke as unknown as RuntimeTransport['invoke'] }, () => useQuery(Expiring, { days: 1 }))
    await tick()
    dispose()
    expect(signal?.aborted).toBe(true)
    pending.resolve({ count: 9 })
    await tick()
    expect(q.data()).toBeUndefined()
    expect(q.loading()).toBe(true)
  })

  it('surfaces the structured error and refetches on demand', async () => {
    const body = { kind: 'domain', code: 'acme.insurance.notActive', params: { status: 'draft' } } as const
    const invoke = vi.fn()
      .mockRejectedValueOnce(new RemoteError(body))
      .mockResolvedValueOnce({ count: 1 })
    const { value: q, dispose } = inPage({ invoke }, () => useQuery(Expiring, { days: 1 }))
    await tick()
    expect(q.error()).toBeInstanceOf(RemoteError)
    expect((q.error() as RemoteError).body).toEqual(body)
    expect(q.loading()).toBe(false)
    q.refetch()
    await tick()
    expect(q.error()).toBeUndefined()
    expect(q.data()).toEqual({ count: 1 })
    dispose()
  })

  it('fails loudly outside a runtime-mounted page', () => {
    expect(() => createRoot((d) => { try { return useQuery(Expiring, { days: 1 }) } finally { d() } })).toThrow(/iota runtime host/)
  })
})

describe('useCommand', () => {
  it('runs once per call, with typed input and result, and never retries', async () => {
    const invoke = vi.fn().mockRejectedValue(new RemoteError({ kind: 'validation', code: 'validation', issues: [{ code: 'x', phase: 'input', source: 's', path: ['id'], params: null }] }))
    const { value: c, dispose } = inPage({ invoke }, () => useCommand(Renew))
    const error = await c.run({ id: 'p1' }).catch((e: unknown) => e)
    expect(error).toBeInstanceOf(RemoteError)
    expect((error as RemoteError).body.issues?.[0]?.code).toBe('x')
    expect(c.error()).toBe(error)
    expect(c.pending()).toBe(false)
    expect(invoke).toHaveBeenCalledTimes(1)
    expect(invoke.mock.calls[0]?.slice(0, 3)).toEqual(['command', 'acme.insurance.renew', { id: 'p1' }])
    invoke.mockResolvedValueOnce({ ok: true })
    await expect(c.run({ id: 'p1' })).resolves.toEqual({ ok: true })
    expect(c.error()).toBeUndefined()
    expect(invoke).toHaveBeenCalledTimes(2)
    dispose()
  })

  it('tracks pending while a run is in flight and stays quiet after disposal', async () => {
    const pending = deferred<unknown>()
    const invoke = vi.fn(() => pending.promise as Promise<never>)
    const { value: c, dispose } = inPage({ invoke }, () => useCommand(Renew))
    const run = c.run({ id: 'p1' })
    expect(c.pending()).toBe(true)
    dispose()
    pending.resolve({ ok: true })
    await expect(run).resolves.toEqual({ ok: true }) // the command ran; the caller still learns the result
    expect(c.pending()).toBe(true) // state is no longer touched after disposal
  })
})
