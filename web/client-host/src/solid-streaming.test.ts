// @vitest-environment jsdom
import { createRoot } from 'solid-js'
import { describe, expect, it, vi } from 'vitest'
import { CLIENT_BOOTSTRAP_VERSION } from './bootstrap'
import { HostError } from './errors'
import { SDK_IDENTITY } from './identity'
import { createManagedStream, type ManagedStreamHandle, useManagedStream } from './solid-streaming'
import { mountSolidClientRoute } from './solid'
import { ManagedSession } from './transport'

function sseResponse(blocks: string[]): Response {
  const encoder = new TextEncoder()
  return new Response(new ReadableStream({
    start(controller) {
      for (const block of blocks) controller.enqueue(encoder.encode(block))
      controller.close()
    },
  }), { status: 200, headers: { 'content-type': 'text/event-stream' } })
}

describe('createManagedStream', () => {
  it('publishes one consistent connection state and closes with the owning root', async () => {
    const events: string[] = []
    const { handle, dispose } = createRoot((dispose) => {
      const handle = createManagedStream<string>(
        { id: 'report.run', url: '/stream', terminal: (event) => event === 'done' },
        (event) => events.push(event),
        {
          parse: (data) => data,
          fetch: async () => sseResponse(['id: 1\ndata: delta\n\nid: 2\ndata: done\n\n']),
        },
      )
      return { handle, dispose }
    })
    await expect.poll(() => events).toEqual(['delta', 'done'])
    expect(handle.state()).toEqual({ status: 'done', cursor: '2' })
    dispose()
    expect(handle.closed).toBe(true)
  })

  it('moves to a typed host error state without retrying permission failures', async () => {
    const { state, dispose } = createRoot((dispose) => {
      const handle = createManagedStream<string>(
        { id: 'bichat.run', url: '/stream', terminal: () => true },
        () => {},
        { parse: (data) => data, fetch: async () => new Response(null, { status: 403 }) },
      )
      return { state: handle.state, dispose }
    })
    await expect.poll(() => state().status).toBe('error')
    const failure = state() as { status: 'error'; error: HostError }
    expect(failure.error).toBeInstanceOf(HostError)
    expect(failure.error.code).toBe('permission_denied')
    dispose()
  })

  it('suppresses diagnostics, events and state after close', async () => {
    const encoder = new TextEncoder()
    let send: ((block: string) => void) | undefined
    const seen: string[] = []
    const events: string[] = []
    const { handle, dispose } = createRoot((dispose) => {
      const handle = createManagedStream<string>(
        { id: 'report.run', url: '/stream', terminal: (event) => event === 'done' },
        (event) => events.push(event),
        {
          parse: (data) => data,
          diagnostics: { event: (event) => seen.push(event.state) },
          fetch: async () => new Response(new ReadableStream({
            start(controller) {
              send = (block) => controller.enqueue(encoder.encode(block))
            },
          }), { status: 200, headers: { 'content-type': 'text/event-stream' } }),
        },
      )
      return { handle, dispose }
    })
    await vi.waitFor(() => expect(seen).toEqual(['connect']))
    send?.('id: 1\ndata: delta\n\n')
    await expect.poll(() => seen).toEqual(['connect', 'event'])
    handle.close()
    await expect.poll(() => handle.closed).toBe(true)
    send?.('id: 2\ndata: done\n\n')
    await new Promise((resolve) => setTimeout(resolve, 20))
    expect(events).toEqual(['delta'])
    expect(seen).toEqual(['connect', 'event'])
    expect(handle.state().status).toBe('live')
    dispose()
  })
})

describe('useManagedStream inside a mounted client route', () => {
  it('consumes an authenticated stream through the host session and cancels on unmount', async () => {
    const telemetry: Array<{ name: string; data?: unknown }> = []
    const requestHeaders: Array<string | null> = []
    let controllers = 0
    let aborted = false
    const encoder = new TextEncoder()
    const root = document.createElement('div')
    const services = {
      session: new ManagedSession({ csrf: 'route-csrf' }, async () => ({ csrf: 'route-csrf' })),
      navigation: { guard: () => () => undefined, canNavigate: () => true },
      theme: { current: () => 'light' as const, set: () => undefined, subscribe: () => () => undefined },
      locale: { language: 'en', t: (key: string) => key },
      telemetry: { emit: (name: string, data?: unknown) => telemetry.push({ name, data }) },
      dispose: () => undefined,
    }
    const context = {
      bootstrapVersion: CLIENT_BOOTSTRAP_VERSION,
      protocolVersion: SDK_IDENTITY.protocolVersion,
      sdkReleaseVersion: SDK_IDENTITY.releaseVersion,
      sdkCommit: SDK_IDENTITY.sourceCommit,
      initial: {},
      theme: 'light' as const,
    }
    const captured: ManagedStreamHandle[] = []
    const received: string[] = []
    const component = () => {
      const handle = useManagedStream<string>(
        { id: 'bichat.run', url: '/stream/events', terminal: (event) => event === 'done' },
        (event) => received.push(event),
        {
          parse: (data) => data,
          fetch: async (_input, init) => {
            controllers += 1
            init?.signal?.addEventListener('abort', () => { aborted = true })
            requestHeaders.push(new Headers(init?.headers).get('x-csrf-token'))
            return new Response(new ReadableStream({
              start(streamController) {
                streamController.enqueue(encoder.encode('id: 1\ndata: delta\n\n'))
              },
            }), { status: 200, headers: { 'content-type': 'text/event-stream' } })
          },
        },
      )
      captured.push(handle)
      return document.createTextNode('route')
    }
    const dispose = mountSolidClientRoute({ root, component, props: {}, context, services })
    await expect.poll(() => received).toEqual(['delta'])
    expect(requestHeaders).toEqual(['route-csrf'])
    expect(telemetry.map((entry) => entry.name)).toEqual(['iota.stream.connect'])
    expect(telemetry[0]?.data).toMatchObject({ stream: 'bichat.run' })
    dispose()
    expect(controllers).toBe(1)
    expect(aborted).toBe(true)
    expect(captured[0]?.closed).toBe(true)
  })
})
