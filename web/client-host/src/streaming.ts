import { asHostError, dispatchHostError, HostError, type HostErrorHandlers } from './errors'
import type { QueryCache } from './cache'
import { ManagedSession, type SessionService } from './transport'

export interface StreamContract<TEvent> {
  id: string
  url: string
  method?: 'GET' | 'POST'
  body?: BodyInit | (() => BodyInit)
  headers?: Readonly<Record<string, string>>
  terminal(event: TEvent): boolean
  cursor?(event: TEvent): string | undefined
  invalidates?(event: TEvent): readonly { namespace: string; methods: readonly string[] }[]
}

export interface StreamDiagnostics {
  event?(event: { stream: string; state: 'connect' | 'event' | 'reconnect' | 'terminal' | 'error'; cursor?: string | undefined; attempt?: number; error?: HostError }): void
}

export interface ManagedStreamOptions<TEvent> {
  parse(data: string, event: string): TEvent
  session?: SessionService
  errors?: HostErrorHandlers
  diagnostics?: StreamDiagnostics
  cache?: QueryCache
  maxReconnects?: number
  staleAfterMs?: number
  baseDelayMs?: number
  fetch?: typeof globalThis.fetch
  random?: () => number
}

export interface StreamSubscription { close(): void; readonly closed: boolean }

interface SSEMessage { id?: string; event: string; data: string }

export function subscribeManagedStream<TEvent>(contract: StreamContract<TEvent>, onEvent: (event: TEvent) => void, options: ManagedStreamOptions<TEvent>): StreamSubscription {
  const controller = new AbortController()
  let cursor: string | undefined
  let terminal = false
  let attempt = 0
  let refreshed = false
  const fetcher = options.fetch ?? globalThis.fetch
  const maxReconnects = options.maxReconnects ?? 4
  const baseDelay = options.baseDelayMs ?? 250

  const run = async () => {
    while (!controller.signal.aborted && !terminal) {
      options.diagnostics?.event?.({ stream: contract.id, state: attempt === 0 ? 'connect' : 'reconnect', cursor, attempt })
      try {
        const sessionService = options.session ?? new ManagedSession({}, async () => ({}))
        const session = sessionService.snapshot()
        const body = typeof contract.body === 'function' ? contract.body() : contract.body
        const response = await fetcher(contract.url, {
          method: contract.method ?? 'GET',
          ...(body === undefined ? {} : { body }),
          credentials: 'same-origin', signal: controller.signal,
          headers: {
            accept: 'text/event-stream',
            ...(cursor ? { 'last-event-id': cursor } : {}),
            ...(session.csrf ? { 'x-csrf-token': session.csrf } : {}),
            ...session.headers,
            ...contract.headers,
          },
        })
        if (response.status === 401 && !refreshed) {
          await sessionService.refresh(controller.signal)
          refreshed = true
          continue
        }
        if (response.status === 401) throw new HostError('unauthenticated', 'Session expired')
        if (response.status === 403) throw new HostError('permission_denied', 'Permission denied')
        if (!response.ok || !response.body) throw new HostError(response.status >= 500 ? 'transient' : 'unknown', `Stream HTTP ${response.status}`)
        for await (const message of readSSE(response.body, controller.signal, options.staleAfterMs ?? 45_000)) {
          if (message.id && message.id === cursor) continue
          const event = options.parse(message.data, message.event)
          cursor = contract.cursor?.(event) ?? message.id ?? cursor
          if (contract.terminal(event)) {
            if (terminal) continue
            terminal = true
            options.diagnostics?.event?.({ stream: contract.id, state: 'terminal', cursor })
          } else {
            options.diagnostics?.event?.({ stream: contract.id, state: 'event', cursor })
          }
          onEvent(event)
          for (const invalidation of contract.invalidates?.(event) ?? []) {
            options.cache?.invalidate(invalidation.namespace, invalidation.methods)
          }
          if (terminal) break
        }
        if (!terminal) throw new HostError('transient', 'Stream ended before a terminal event')
      } catch (cause) {
        if (controller.signal.aborted) return
        const error = asHostError(cause)
        if (!error.retryable || attempt >= maxReconnects) {
          dispatchHostError(error, options.errors)
          options.diagnostics?.event?.({ stream: contract.id, state: 'error', cursor, error })
          return
        }
        attempt += 1
        const jitter = 0.75 + (options.random?.() ?? Math.random()) * 0.5
        await abortableDelay(Math.min(8_000, baseDelay * 2 ** (attempt - 1)) * jitter, controller.signal)
      }
    }
  }
  void run()
  return { close: () => controller.abort(), get closed() { return controller.signal.aborted } }
}

async function* readSSE(stream: ReadableStream<Uint8Array>, signal: AbortSignal, staleAfterMs: number): AsyncGenerator<SSEMessage> {
  const reader = stream.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  try {
    for (;;) {
      const result = await readWithTimeout(reader, staleAfterMs, signal)
      if (result.done) break
      buffer = (buffer + decoder.decode(result.value, { stream: true })).replaceAll('\r\n', '\n')
      let boundary = buffer.indexOf('\n\n')
      while (boundary >= 0) {
        const block = buffer.slice(0, boundary)
        buffer = buffer.slice(boundary + 2)
        const message = parseSSEBlock(block)
        if (message) yield message
        boundary = buffer.indexOf('\n\n')
      }
    }
  } finally {
    reader.releaseLock()
  }
}

function readWithTimeout(reader: ReadableStreamDefaultReader<Uint8Array>, staleAfterMs: number, signal: AbortSignal) {
  return new Promise<ReadableStreamReadResult<Uint8Array>>((resolve, reject) => {
    const timer = setTimeout(() => reject(new HostError('transient', 'Stream heartbeat timed out')), staleAfterMs)
    const abort = () => { clearTimeout(timer); reject(signal.reason) }
    signal.addEventListener('abort', abort, { once: true })
    void reader.read().then(
      (result) => { clearTimeout(timer); signal.removeEventListener('abort', abort); resolve(result) },
      (error) => { clearTimeout(timer); signal.removeEventListener('abort', abort); reject(error) },
    )
  })
}

function parseSSEBlock(block: string): SSEMessage | undefined {
  let id: string | undefined
  let event = 'message'
  const data: string[] = []
  for (const line of block.replaceAll('\r', '').split('\n')) {
    if (line.startsWith(':')) continue
    const split = line.indexOf(':')
    const field = split < 0 ? line : line.slice(0, split)
    const value = split < 0 ? '' : line.slice(split + 1).replace(/^ /, '')
    if (field === 'id') id = value
    if (field === 'event') event = value
    if (field === 'data') data.push(value)
  }
  return data.length > 0 ? { ...(id === undefined ? {} : { id }), event, data: data.join('\n') } : undefined
}

function abortableDelay(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(resolve, ms)
    signal.addEventListener('abort', () => { clearTimeout(timer); reject(signal.reason) }, { once: true })
  })
}
