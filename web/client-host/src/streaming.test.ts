import { describe, expect, it } from 'vitest'
import { subscribeManagedStream } from './streaming'
import { ManagedSession } from './transport'

function response(blocks: string[]): Response {
  const encoder = new TextEncoder()
  return new Response(new ReadableStream({
    start(controller) {
      for (const block of blocks) controller.enqueue(encoder.encode(block))
      controller.close()
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
})
