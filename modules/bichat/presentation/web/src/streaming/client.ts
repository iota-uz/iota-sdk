import {
  isTerminalStreamEvent,
  parseStreamChunk,
  readSSEResponse,
  type StreamChunk,
} from './protocol'

export interface SendMessageBody {
  sessionId: string
  content: string
  attachments?: Array<Record<string, unknown>>
  debugMode?: boolean
  replaceFromMessageId?: string
  reasoningEffort?: string
  model?: string
  requestId?: string
}

export interface StreamHandlers {
  onEvent: (chunk: StreamChunk) => void
}

export class StreamConnectError extends Error {
  readonly status: number
  constructor(message: string, status: number) {
    super(message)
    this.name = 'StreamConnectError'
    this.status = status
  }
}

export class StreamCancelledError extends Error {
  constructor() {
    super('stream cancelled')
    this.name = 'StreamCancelledError'
  }
}

export interface BichatStreamClientOptions {
  streamEndpoint: string
  csrfToken?: string | (() => string)
  connectTimeoutMs?: number
}

function csrfHeader(options: BichatStreamClientOptions): Record<string, string> {
  const token = typeof options.csrfToken === 'function' ? options.csrfToken() : options.csrfToken
  return token ? { 'X-CSRF-Token': token } : {}
}

/**
 * Client for the BiChat streaming endpoints:
 * - POST {streamEndpoint}        — send a message, SSE response
 * - POST {streamEndpoint}/stop   — cancel the active run
 * - POST {streamEndpoint}/resume — resume a run after refresh, SSE response
 * - GET  {streamEndpoint}/events — per-run tail with Last-Event-ID replay
 * - GET  {streamEndpoint}/active-runs — sidebar fan-out snapshot
 */
export class BichatStreamClient {
  constructor(private readonly options: BichatStreamClientOptions) {}

  async sendMessage(
    body: SendMessageBody,
    handlers: StreamHandlers,
    signal?: AbortSignal,
  ): Promise<void> {
    await this.stream(this.options.streamEndpoint, body, handlers, signal)
  }

  async resume(
    body: { sessionId: string; runId: string },
    handlers: StreamHandlers,
    signal?: AbortSignal,
  ): Promise<void> {
    await this.stream(`${this.options.streamEndpoint}/resume`, body, handlers, signal)
  }

  async stop(sessionId: string, runId?: string): Promise<void> {
    await fetch(`${this.options.streamEndpoint}/stop`, {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json', ...csrfHeader(this.options) },
      body: JSON.stringify({ sessionId, ...(runId ? { runId } : {}) }),
    })
  }

  /**
   * Tails one run's event log over a native EventSource. The browser
   * reconnects automatically and replays via the Last-Event-ID header; the
   * promise settles on the first terminal event or when the signal aborts.
   */
  subscribeRunEvents(
    params: { sessionId: string; runId?: string },
    handlers: StreamHandlers,
    signal?: AbortSignal,
  ): { settled: Promise<void>; close: () => void } {
    const url = new URL(`${this.options.streamEndpoint}/events`, window.location.origin)
    url.searchParams.set('sessionId', params.sessionId)
    if (params.runId) url.searchParams.set('runId', params.runId)

    const source = new EventSource(url.toString(), { withCredentials: true })
    let finish = () => source.close()
    const settled = new Promise<void>((resolve) => {
      const complete = () => {
        source.close()
        signal?.removeEventListener('abort', onAbort)
        resolve()
      }
      const onAbort = () => complete()
      signal?.addEventListener('abort', onAbort)
      finish = complete

      for (const type of ['chunk', 'content', 'thinking', 'tool_start', 'tool_end', 'text_block_end', 'snapshot', 'interrupt', 'citation', 'usage', 'ping', 'stream_started']) {
        source.addEventListener(type, (event) => {
          const chunk = parseStreamChunk(type, (event as MessageEvent<string>).data)
          if (chunk) handlers.onEvent(chunk)
        })
      }
      for (const type of ['done', 'cancelled', 'error', 'failed']) {
        source.addEventListener(type, (event) => {
          const chunk = parseStreamChunk(type, (event as MessageEvent<string>).data)
          if (chunk) handlers.onEvent(chunk)
          complete()
        })
      }
      source.onerror = () => {
        // Native reconnection handles transient drops; the promise settles
        // only via terminal event, abort or explicit close.
      }
    })
    return { settled, close: () => finish() }
  }

  subscribeActiveRuns(
    handlers: { onSnapshot?: (payload: unknown) => void; onUpdate?: (payload: unknown) => void },
    signal?: AbortSignal,
  ): { close: () => void } {
    const url = new URL(`${this.options.streamEndpoint}/active-runs`, window.location.origin)
    const source = new EventSource(url.toString(), { withCredentials: true })
    const onAbort = () => source.close()
    signal?.addEventListener('abort', onAbort)
    if (handlers.onSnapshot) {
      source.addEventListener('snapshot', (event) => handlers.onSnapshot?.((event as MessageEvent).data))
    }
    if (handlers.onUpdate) {
      source.addEventListener('update', (event) => handlers.onUpdate?.((event as MessageEvent).data))
    }
    return { close: () => source.close() }
  }

  private async stream(
    url: string,
    body: unknown,
    handlers: StreamHandlers,
    signal?: AbortSignal,
  ): Promise<void> {
    const controller = new AbortController()
    const onAbort = () => controller.abort()
    signal?.addEventListener('abort', onAbort)
    const timer = this.options.connectTimeoutMs
      ? setTimeout(() => controller.abort(), this.options.connectTimeoutMs)
      : undefined

    let response: Response
    try {
      response = await fetch(url, {
        method: 'POST',
        credentials: 'same-origin',
        signal: controller.signal,
        headers: { 'Content-Type': 'application/json', ...csrfHeader(this.options) },
        body: JSON.stringify(body),
      })
    } catch (error) {
      if (timer) clearTimeout(timer)
      signal?.removeEventListener('abort', onAbort)
      if (signal?.aborted) throw new StreamCancelledError()
      if (controller.signal.aborted) {
        throw new StreamConnectError('stream connect timed out', 0)
      }
      throw new StreamConnectError(`stream request failed: ${String(error)}`, 0)
    }
    if (timer) clearTimeout(timer)
    signal?.removeEventListener('abort', onAbort)

    if (!response.ok) {
      let message = `stream request failed with status ${response.status}`
      try {
        const text = await response.text()
        if (text) message = text
      } catch {
        // keep status-based message
      }
      throw new StreamConnectError(message, response.status)
    }

    try {
      let sawTerminal = false
      await readSSEResponse(response, (eventName, rawData) => {
        const chunk = parseStreamChunk(eventName, rawData)
        if (!chunk) return
        handlers.onEvent(chunk)
        if (isTerminalStreamEvent(chunk.type)) sawTerminal = true
      })
      if (!sawTerminal && !signal?.aborted) {
        handlers.onEvent({ type: 'error', error: 'connection closed before the run finished' })
      }
    } catch (error) {
      if (signal?.aborted) throw new StreamCancelledError()
      if (controller.signal.aborted) throw new StreamCancelledError()
      handlers.onEvent({ type: 'error', error: String(error) })
    }
  }
}

export function createBichatStreamClient(options: BichatStreamClientOptions): BichatStreamClient {
  return new BichatStreamClient({
    ...options,
    connectTimeoutMs: options.connectTimeoutMs ?? 30_000,
  })
}
