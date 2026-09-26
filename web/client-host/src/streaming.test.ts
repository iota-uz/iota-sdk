import { describe, expect, it } from 'vitest'
import { HostError } from './errors'
import { subscribeManagedStream } from './streaming'
import { ManagedSession } from './transport'
import { MemoryQueryCache } from './cache'

function response(blocks: string[]): Response {
  const encoder = new TextEncoder()
  return new Response(new ReadableStream({
    start(controller) {
      for (const block of blocks) controller.enqueue(encoder.encode(block))
      controller.close()
    },
  }), { status: 200, headers: { 'content-type': 'text/event-stream' } })
}

function hangingResponse(blocks: string[] = []): Response {
  const encoder = new TextEncoder()
  return new Response(new ReadableStream({
    start(controller) {
      for (const block of blocks) controller.enqueue(encoder.encode(block))
    },
  }), { status: 200, headers: { 'content-type': 'text/event-stream' } })
}

describe('managed streaming', () => {
  it('replays from the last cursor, ignores the repeated event and emits one terminal state', async () => {
    const headers: Array<string | null> = []
    const requests: Array<{ method: string | undefined; body: BodyInit | null | undefined }> = []
    let call = 0
    const events: Array<{ id: string; terminal?: boolean }> = []
    const subscription = subscribeManagedStream<{ id: string; terminal?: boolean }>(
      { id: 'bichat.run', url: '/stream', method: 'POST', body: () => JSON.stringify({ sessionId: 's1' }), cursor: (event) => event.id, terminal: (event) => Boolean(event.terminal) },
      (event) => events.push(event),
      {
        parse: (data) => JSON.parse(data) as { id: string; terminal?: boolean },
        baseDelayMs: 0,
        random: () => 0,
        fetch: async (_input, init) => {
          requests.push({ method: init?.method, body: init?.body })
          headers.push(new Headers(init?.headers).get('last-event-id'))
          call += 1
          return call === 1
            ? response(['id: 1\ndata: {"id":"1"}\n\n'])
            : response(['id: 1\ndata: {"id":"1"}\n\nid: 2\nevent: done\ndata: {"id":"2","terminal":true}\n\n'])
        },
      },
    )
    await expect.poll(() => events.length).toBe(2)
    expect(headers).toEqual([null, '1'])
    expect(requests).toEqual([{ method: 'POST', body: '{"sessionId":"s1"}' }, { method: 'POST', body: '{"sessionId":"s1"}' }])
    expect(events).toEqual([{ id: '1' }, { id: '2', terminal: true }])
    expect(subscription.closed).toBe(false)
    subscription.close()
  })

  it('refreshes session and CSRF once before surfacing an unauthenticated error', async () => {
    let refreshes = 0
    const csrf: Array<string | null> = []
    const session = new ManagedSession({ csrf: 'old' }, async () => { refreshes += 1; return { csrf: 'new' } })
    const events: string[] = []
    subscribeManagedStream<string>(
      { id: 'bichat.run', url: '/stream', terminal: () => true },
      (event) => events.push(event),
      {
        session, parse: (data) => data,
        fetch: async (_input, init) => {
          csrf.push(new Headers(init?.headers).get('x-csrf-token'))
          return csrf.length === 1 ? new Response(null, { status: 401 }) : response(['data: done\n\n'])
        },
      },
    )
    await expect.poll(() => events.length).toBe(1)
    expect(refreshes).toBe(1)
    expect(csrf).toEqual(['old', 'new'])
  })

  it('ignores BiChat heartbeat comments and named ping events without a cursor', async () => {
    const cursors: Array<string | undefined> = []
    const events: Array<{ id?: string; type: string }> = []
    subscribeManagedStream<{ id?: string; type: string }>(
      { id: 'bichat.run', url: '/stream/events', terminal: (event) => event.type === 'done', cursor: (event) => event.id },
      (event) => events.push(event),
      {
        parse: (data) => JSON.parse(data) as { id?: string; type: string },
        diagnostics: { event: (event) => cursors.push(event.cursor) },
        fetch: async () => response([
          ': stream-open\n\n',
          'id: 7\nevent: message\ndata: {"type":"content","id":"7"}\n\n',
          ': ping\n\n',
          'event: ping\ndata: {"type":"ping"}\n\n',
          'id: 8\nevent: done\ndata: {"type":"done","id":"8"}\n\n',
        ]),
      },
    )
    await expect.poll(() => events.length).toBe(3)
    expect(events).toEqual([{ type: 'content', id: '7' }, { type: 'ping' }, { type: 'done', id: '8' }])
    expect(cursors.at(-1)).toBe('8')
  })

  it('treats a stale heartbeat as transient and reconnects with the last cursor', async () => {
    let call = 0
    const ids: Array<string | null> = []
    const events: string[] = []
    subscribeManagedStream<string>(
      { id: 'bichat.run', url: '/stream/events', terminal: (event) => event === 'done' },
      (event) => events.push(event),
      {
        parse: (data) => data,
        staleAfterMs: 20,
        baseDelayMs: 0,
        random: () => 0,
        fetch: async (_input, init) => {
          ids.push(new Headers(init?.headers).get('last-event-id'))
          call += 1
          return call === 1
            ? hangingResponse(['id: 3\ndata: delta-3\n\n'])
            : response(['id: 4\ndata: delta-4\n\nid: 5\ndata: done\n\n'])
        },
      },
    )
    await expect.poll(() => events).toEqual(['delta-3', 'delta-4', 'done'])
    expect(ids).toEqual([null, '3'])
  })

  it('surfaces permission failures once without reconnecting', async () => {
    let fetches = 0
    const terminal: unknown[] = []
    subscribeManagedStream<string>(
      { id: 'bichat.run', url: '/stream', terminal: () => true },
      () => {},
      {
        parse: (data) => data,
        maxReconnects: 3,
        errors: { permissionDenied: (error) => terminal.push(error) },
        fetch: async () => {
          fetches += 1
          return new Response(null, { status: 403 })
        },
      },
    )
    await expect.poll(() => terminal.length).toBe(1)
    expect(terminal[0]).toBeInstanceOf(HostError)
    expect((terminal[0] as HostError).code).toBe('permission_denied')
    expect(fetches).toBe(1)
  })

  it('invalidates generated RPC query cache families from stream events', async () => {
    const cache = new MemoryQueryCache()
    cache.set(['reports', 'reports.load', '{"id":"1"}'], 'stale')
    const events: string[] = []
    subscribeManagedStream<string>(
      {
        id: 'report.run', url: '/stream', terminal: (event) => event === 'done',
        invalidates: (event) => (event === 'done' ? [{ namespace: 'reports', methods: ['reports.load'] }] : []),
      },
      (event) => events.push(event),
      { parse: (data) => data, cache, fetch: async () => response(['data: chunk\n\ndata: done\n\n']) },
    )
    await expect.poll(() => events).toEqual(['chunk', 'done'])
    expect(cache.get(['reports', 'reports.load', '{"id":"1"}'])).toBeUndefined()
  })
})
