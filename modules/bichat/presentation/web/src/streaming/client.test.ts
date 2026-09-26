import { afterEach, describe, expect, it, vi } from 'vitest'
import { BichatStreamClient, StreamCancelledError } from './client'
import type { StreamChunk } from './protocol'

function sseResponse(body: string, status = 200): Response {
  const encoder = new TextEncoder()
  const stream = new ReadableStream<Uint8Array>({
    start(controller) {
      controller.enqueue(encoder.encode(body))
      controller.close()
    },
  })
  return new Response(stream, { status, headers: { 'Content-Type': 'text/event-stream' } })
}

describe('BichatStreamClient.sendMessage', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('posts the send payload with CSRF header and streams chunks', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      sseResponse('event: stream_started\ndata: {"runId":"run-1"}\n\nevent: chunk\ndata: {"content":"Hi"}\n\nevent: done\ndata: {}\n\n'),
    )
    vi.stubGlobal('fetch', fetchMock)

    const chunks: StreamChunk[] = []
    const client = new BichatStreamClient({ streamEndpoint: '/bi-chat/stream', csrfToken: 'token-1' })
    await client.sendMessage({ sessionId: 's-1', content: 'Hello' }, { onEvent: (c) => chunks.push(c) })

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/bi-chat/stream')
    expect((init.headers as Record<string, string>)['X-CSRF-Token']).toBe('token-1')
    expect(JSON.parse(String(init.body)).sessionId).toBe('s-1')
    expect(chunks.map((c) => c.type)).toEqual(['stream_started', 'chunk', 'done'])
  })

  it('throws a typed connect error with status on HTTP failure', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('Access denied', { status: 403 })))
    const client = new BichatStreamClient({ streamEndpoint: '/bi-chat/stream' })
    await expect(
      client.sendMessage({ sessionId: 's-1', content: 'x' }, { onEvent: () => {} }),
    ).rejects.toMatchObject({ name: 'StreamConnectError', status: 403 })
  })

  it('maps an aborted send to StreamCancelledError', async () => {
    const controller = new AbortController()
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation((_url: string, init: RequestInit) => new Promise((_resolve, reject) => {
        init.signal?.addEventListener('abort', () => reject(new DOMException('aborted', 'AbortError')))
      })),
    )
    const client = new BichatStreamClient({ streamEndpoint: '/bi-chat/stream' })
    const pending = client.sendMessage({ sessionId: 's-1', content: 'x' }, { onEvent: () => {} }, controller.signal)
    controller.abort()
    await expect(pending).rejects.toBeInstanceOf(StreamCancelledError)
  })

  it('synthesizes an error chunk when the connection closes without a terminal event', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(sseResponse('event: chunk\ndata: {"content":"partial"}\n\n')))
    const chunks: StreamChunk[] = []
    const client = new BichatStreamClient({ streamEndpoint: '/bi-chat/stream' })
    await client.sendMessage({ sessionId: 's-1', content: 'x' }, { onEvent: (c) => chunks.push(c) })
    expect(chunks.at(-1)?.type).toBe('error')
  })

  it('does not synthesize an error after a terminal event', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(sseResponse('event: cancelled\ndata: {}\n\n')))
    const chunks: StreamChunk[] = []
    const client = new BichatStreamClient({ streamEndpoint: '/bi-chat/stream' })
    await client.sendMessage({ sessionId: 's-1', content: 'x' }, { onEvent: (c) => chunks.push(c) })
    expect(chunks.map((c) => c.type)).toEqual(['cancelled'])
  })

  it('posts run stop requests', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response('', { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)
    const client = new BichatStreamClient({ streamEndpoint: '/bi-chat/stream', csrfToken: 't' })
    await client.stop('s-1', 'run-9')
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/bi-chat/stream/stop')
    expect(JSON.parse(String(init.body))).toEqual({ sessionId: 's-1', runId: 'run-9' })
  })
})
