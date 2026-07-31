import {
  PanelRequestSchema,
  PanelResponseSchema,
  QueryErrorResponseSchema,
  type PanelRequest,
  type PanelResponse,
  type QueryErrorCode,
} from '../contract'
import { QueryError, SnapshotGoneError } from './query'

export interface PanelClientOptions {
  csrf?: string
  fetcher?: typeof fetch
}

export interface PanelLoadOptions {
  signal?: AbortSignal
}

function panelKey(request: PanelRequest): string {
  return `${request.snapshotId}:${request.panelId}:${request.recompute ? 'recompute' : 'load'}:${request.search ?? ''}`
}

/**
 * A small per-dashboard client for independently materialised panel frames.
 * The server owns the bounded durable cache; this map only deduplicates browser
 * requests and is discarded with the runtime provider.
 */
export class PanelClient {
  private readonly inFlight = new Map<string, Promise<PanelResponse>>()
  private readonly controllers = new Set<AbortController>()

  constructor(
    private readonly endpoint: string,
    private readonly options: PanelClientOptions = {},
  ) {}

  load(input: PanelRequest, options: PanelLoadOptions = {}): Promise<PanelResponse> {
    const request = PanelRequestSchema.parse(input)
    const key = panelKey(request)
    const existing = this.inFlight.get(key)
    if (existing) return existing

    const controller = new AbortController()
    const abort = () => controller.abort(options.signal?.reason)
    options.signal?.addEventListener('abort', abort, { once: true })
    this.controllers.add(controller)
    const pending = this.fetch(request, controller.signal).finally(() => {
      options.signal?.removeEventListener('abort', abort)
      this.controllers.delete(controller)
      this.inFlight.delete(key)
    })
    this.inFlight.set(key, pending)
    return pending
  }

  dispose(): void {
    for (const controller of this.controllers) controller.abort()
    this.controllers.clear()
    this.inFlight.clear()
  }

  private async fetch(request: PanelRequest, signal: AbortSignal): Promise<PanelResponse> {
    const response = await (this.options.fetcher ?? fetch)(this.endpoint, {
      method: 'POST',
      credentials: 'same-origin',
      headers: {
        'Content-Type': 'application/json',
        ...(this.options.csrf ? { 'X-CSRF-Token': this.options.csrf } : {}),
      },
      body: JSON.stringify(request),
      signal,
    })
    const payload: unknown = await response.json()
    if (!response.ok) {
      const parsed = QueryErrorResponseSchema.safeParse(payload)
      const code: QueryErrorCode = parsed.success ? parsed.data.error : 'internal'
      const message = parsed.success ? parsed.data.message : `panel request failed with ${response.status}`
      if (response.status === 410 && code === 'snapshot_gone') throw new SnapshotGoneError(message)
      throw new QueryError(code, message, response.status)
    }
    return PanelResponseSchema.parse(payload)
  }
}
