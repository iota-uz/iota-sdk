import { afterEach, describe, expect, it, vi } from 'vitest'
import { FetchRPCTransport, ManagedSession } from './transport'

afterEach(() => vi.unstubAllGlobals())

describe('ManagedSession', () => {
  it('deduplicates concurrent refresh and swaps the snapshot atomically', async () => {
    let calls = 0
    let release!: () => void
    const gate = new Promise<void>((resolve) => { release = resolve })
    const session = new ManagedSession({ csrf: 'old' }, async () => { calls += 1; await gate; return { csrf: 'new' } })
    const signal = new AbortController().signal
    const first = session.refresh(signal)
    const second = session.refresh(signal)
    expect(session.snapshot().csrf).toBe('old')
    release()
    await Promise.all([first, second])
    expect(calls).toBe(1)
    expect(session.snapshot().csrf).toBe('new')
  })

  it('refreshes once on 401 and every subsequent call reads the current CSRF token', async () => {
    const tokens: string[] = []
    let responses = 0
    vi.stubGlobal('fetch', vi.fn(async (_url: string, init: RequestInit) => {
      tokens.push(new Headers(init.headers).get('x-csrf-token') ?? '')
      responses += 1
      if (responses === 1) return new Response('', { status: 401 })
      return Response.json({ result: { ok: true } })
    }))
    const session = new ManagedSession({ csrf: 'old' }, async () => ({ csrf: 'new' }))
    const transport = new FetchRPCTransport('/rpc', session)
    await transport.call('product.get', {}, new AbortController().signal)
    await transport.call('product.get', {}, new AbortController().signal)
    expect(tokens).toEqual(['old', 'new', 'new'])
  })
})
