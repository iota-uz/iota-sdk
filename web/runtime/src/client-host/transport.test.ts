import { describe, expect, it, vi } from 'vitest'
import { RemoteError } from '../client.ts'
import { createRpcTransport, TransportError, UnauthenticatedError } from './transport.ts'

function reply(status: number, body: unknown): Response {
  return new Response(typeof body === 'string' ? body : JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } })
}

const noSleep = () => Promise.resolve()

describe('rpc transport', () => {
  it('posts the call as JSON with credentials and the session CSRF token', async () => {
    const fetch = vi.fn(async () => reply(200, { ok: true, value: { n: 1 } }))
    const t = createRpcTransport({ fetch, session: { snapshot: () => ({ csrf: 'tok', headers: { 'x-extra': '1' } }), refresh: async () => ({}) } })
    await expect(t.invoke('query', 'acme.insurance.expiring', { days: 3 })).resolves.toEqual({ n: 1 })
    const [url, init] = fetch.mock.calls[0] as unknown as [string, RequestInit]
    expect(url).toBe('/_ext/rpc')
    expect(init.method).toBe('POST')
    expect(init.credentials).toBe('same-origin')
    expect((init.headers as Record<string, string>)['x-csrf-token']).toBe('tok')
    expect((init.headers as Record<string, string>)['x-extra']).toBe('1')
    expect(JSON.parse(init.body as string)).toEqual({ kind: 'query', resource: 'acme.insurance.expiring', input: { days: 3 } })
  })

  it('keeps {kind, code, params, issues} of a structured error', async () => {
    const error = {
      kind: 'validation', code: 'validation', params: { a: 1 },
      issues: [{ code: 'acme.insurance.tooLong', phase: 'input', source: 'acme.insurance.check', path: ['notes'], params: { max: 10 } }],
    }
    const t = createRpcTransport({ fetch: vi.fn(async () => reply(422, { ok: false, error })) })
    const caught = await t.invoke('command', 'acme.insurance.renew', null).catch((e: unknown) => e)
    expect(caught).toBeInstanceOf(RemoteError)
    expect((caught as RemoteError).body).toEqual(error)
  })

  it('never repeats a mutation, whatever happens', async () => {
    for (const failure of [
      () => Promise.reject(new TypeError('network down')),
      async () => reply(503, 'bad gateway'),
      async () => reply(401, { ok: false }),
    ]) {
      const fetch = vi.fn(failure)
      const refresh = vi.fn(async () => ({}))
      const t = createRpcTransport({ fetch, queryRetries: 5, sleep: noSleep, session: { snapshot: () => ({}), refresh } })
      await expect(t.invoke('command', 'acme.insurance.renew', null)).rejects.toBeTruthy()
      await expect(t.invoke('atomic', 'acme.insurance.activate', null)).rejects.toBeTruthy()
      expect(fetch).toHaveBeenCalledTimes(2) // once per call
      expect(refresh).not.toHaveBeenCalled()
    }
  })

  it('retries a query once after a network failure or a transient status', async () => {
    const fetch = vi.fn()
      .mockRejectedValueOnce(new TypeError('network down'))
      .mockResolvedValueOnce(reply(503, 'upstream'))
      .mockResolvedValueOnce(reply(200, { ok: true, value: 7 }))
    const t = createRpcTransport({ fetch, queryRetries: 2, sleep: noSleep })
    await expect(t.invoke('query', 'q', null)).resolves.toBe(7)
    expect(fetch).toHaveBeenCalledTimes(3)

    const failing = vi.fn(async () => reply(503, 'upstream'))
    const t2 = createRpcTransport({ fetch: failing, sleep: noSleep })
    const error = await t2.invoke('query', 'q', null).catch((e: unknown) => e)
    expect(error).toBeInstanceOf(TransportError)
    expect((error as TransportError).status).toBe(503)
    expect(failing).toHaveBeenCalledTimes(2) // the default is one retry
  })

  it('does not retry a structured error', async () => {
    const fetch = vi.fn(async () => reply(500, { ok: false, error: { kind: 'internal', code: 'runtime.internal' } }))
    const t = createRpcTransport({ fetch, sleep: noSleep })
    await expect(t.invoke('query', 'q', null)).rejects.toBeInstanceOf(RemoteError)
    expect(fetch).toHaveBeenCalledTimes(1)
  })

  it('refreshes the session once for a query, and reports an expired session', async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(reply(401, { ok: false, error: { kind: 'denied', code: 'ui.unauthenticated' } }))
      .mockResolvedValueOnce(reply(200, { ok: true, value: 'fresh' }))
    const refresh = vi.fn(async () => ({}))
    const t = createRpcTransport({ fetch, session: { snapshot: () => ({}), refresh } })
    await expect(t.invoke('query', 'q', null)).resolves.toBe('fresh')
    expect(refresh).toHaveBeenCalledTimes(1)

    const gone = vi.fn(async () => reply(401, 'unauthorized'))
    const t2 = createRpcTransport({ fetch: gone, reauthUrl: '/login', session: { snapshot: () => ({}), refresh: async () => { throw new Error('no') } } })
    const error = await t2.invoke('query', 'q', null).catch((e: unknown) => e)
    expect(error).toBeInstanceOf(UnauthenticatedError)
    expect((error as UnauthenticatedError).reauthUrl).toBe('/login')
  })

  it('stops on abort without retrying', async () => {
    const controller = new AbortController()
    const fetch = vi.fn((_url: unknown, init?: RequestInit) => new Promise<Response>((_resolve, reject) => {
      init?.signal?.addEventListener('abort', () => reject(new DOMException('Aborted', 'AbortError')))
    }))
    const t = createRpcTransport({ fetch, sleep: noSleep })
    const pending = t.invoke('query', 'q', null, { signal: controller.signal })
    controller.abort()
    await expect(pending).rejects.toMatchObject({ name: 'AbortError' })
    expect(fetch).toHaveBeenCalledTimes(1)
  })
})
