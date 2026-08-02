import {
  PanelBatchRequestSchema,
  PanelBatchStreamEventSchema,
  PanelRequestSchema,
  QueryErrorResponseSchema,
  type PanelBatchResult,
  type PanelRequest,
  type QueryErrorCode,
} from '../contract'
import { QueryError, SnapshotGoneError } from './query'

export interface PanelClientOptions {
  csrf?: string
  fetcher?: typeof fetch
}

export interface PanelLoadOptions {
  signal?: AbortSignal
  /** Called as each NDJSON batch member arrives, before slower siblings finish. */
  onResult?: (panelId: string, result: PanelBatchResult) => void
}

type SnapshotPanelRequest = PanelRequest & { snapshotId: string }
type PanelLoadResult = PanelBatchResult & { frames: NonNullable<PanelBatchResult['frames']> }

function panelKey(request: SnapshotPanelRequest): string {
  return JSON.stringify([
    request.snapshotId, request.panelId, request.recompute === true, request.search ?? '',
    request.sort?.field ?? '', request.sort?.direction ?? '', request.page ?? 1,
  ])
}

/**
 * A small per-dashboard client for independently materialised panel frames.
 * The server owns the bounded durable cache; this map only deduplicates browser
 * requests and is discarded with the runtime provider.
 */
export class PanelClient {
  private readonly inFlight = new Map<string, Promise<PanelLoadResult>>()
  private readonly controllers = new Set<AbortController>()

  constructor(
    private readonly endpoint: string,
    private readonly options: PanelClientOptions = {},
  ) {}

  /**
   * Whether a rejection is "the request was cancelled" rather than "the request
   * failed". A cancelled panel has nothing to report: its caller moved on, and
   * showing the user an error for work nobody is waiting for reads as a broken
   * panel beside working ones.
   *
   * `fetch` rejects an aborted request with a DOMException named AbortError;
   * an abort with an explicit reason rejects with that reason instead.
   */
  static isAbort(cause: unknown): boolean {
    if (cause instanceof DOMException) return cause.name === 'AbortError'
    return cause instanceof Error && cause.name === 'AbortError'
  }

  load(input: SnapshotPanelRequest, options: PanelLoadOptions = {}): Promise<PanelLoadResult> {
    const { snapshotId, ...panelInput } = input
    const request = { ...PanelRequestSchema.parse(panelInput), snapshotId }
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

  async loadBatch(inputs: SnapshotPanelRequest[], options: PanelLoadOptions = {}): Promise<Record<string, PanelBatchResult>> {
    if (inputs.length === 0) return {}
    const snapshotId = inputs[0]!.snapshotId
    if (inputs.some((input) => input.snapshotId !== snapshotId)) throw new Error('panel batch mixes snapshots')
    const panels = inputs.map((input) => PanelRequestSchema.parse({
      panelId: input.panelId, recompute: input.recompute, search: input.search, sort: input.sort, page: input.page,
    }))
    const controller = new AbortController()
    const abort = () => controller.abort(options.signal?.reason)
    options.signal?.addEventListener('abort', abort, { once: true })
    this.controllers.add(controller)
    try {
      return await this.fetchBatch(snapshotId, panels, controller.signal, options.onResult)
    } finally {
      options.signal?.removeEventListener('abort', abort)
      this.controllers.delete(controller)
    }
  }

  dispose(): void {
    for (const controller of this.controllers) controller.abort()
    this.controllers.clear()
    this.inFlight.clear()
  }

  private async fetch(request: SnapshotPanelRequest, signal: AbortSignal): Promise<PanelLoadResult> {
    const panels = await this.fetchBatch(request.snapshotId, [{
      panelId: request.panelId, recompute: request.recompute, search: request.search, sort: request.sort, page: request.page,
    }], signal)
    const result = panels[request.panelId]
    if (!result) throw new QueryError('internal', `panel ${request.panelId} missing from batch response`, 200)
    if (result.error) throw new QueryError(result.error.error, result.error.message, 200)
    if (!result.frames) throw new QueryError('internal', `panel ${request.panelId} response has no frames`, 200)
    return { ...result, frames: result.frames }
  }

  private async fetchBatch(
    snapshotId: string,
    panels: PanelRequest[],
    signal: AbortSignal,
    onResult?: (panelId: string, result: PanelBatchResult) => void,
  ): Promise<Record<string, PanelBatchResult>> {
    const response = await (this.options.fetcher ?? fetch)(this.endpoint, {
      method: 'POST',
      credentials: 'same-origin',
      headers: {
        'Content-Type': 'application/json',
        ...(this.options.csrf ? { 'X-CSRF-Token': this.options.csrf } : {}),
      },
      body: JSON.stringify(PanelBatchRequestSchema.parse({ snapshotId, panels })),
      signal,
    })
    if (!response.ok) {
      const payload: unknown = await response.json()
      const parsed = QueryErrorResponseSchema.safeParse(payload)
      const code: QueryErrorCode = parsed.success ? parsed.data.error : 'internal'
      const message = parsed.success ? parsed.data.message : `panel request failed with ${response.status}`
      if (response.status === 410 && code === 'snapshot_gone') throw new SnapshotGoneError(message)
      throw new QueryError(code, message, response.status)
    }
    if (!response.headers.get('Content-Type')?.toLowerCase().includes('ndjson')) {
      throw new QueryError('internal', 'panel response must be application/x-ndjson', response.status)
    }
    if (response.body) {
      const results: Record<string, PanelBatchResult> = {}
      const requested = new Set(panels.map(({ panelId }) => panelId))
      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffered = ''
      let complete = false
      const consume = (line: string) => {
        if (!line.trim()) return
        const item = PanelBatchStreamEventSchema.parse(JSON.parse(line))
        if (item.complete === true) {
          if (item.panelId !== undefined || item.result !== undefined) {
            throw new QueryError('internal', 'panel stream completion marker carries a result', response.status)
          }
          if (complete) throw new QueryError('internal', 'panel stream contains duplicate completion markers', response.status)
          const missing = [...requested].filter((panelId) => !results[panelId])
          if (missing.length > 0) throw new QueryError('internal', `panel stream is missing: ${missing.join(', ')}`, response.status)
          complete = true
          return
        }
        if (complete) throw new QueryError('internal', 'panel stream contains data after completion', response.status)
        if (item.complete !== undefined || typeof item.panelId !== 'string' || item.result === undefined) {
          throw new QueryError('internal', 'panel stream event is neither a result nor completion', response.status)
        }
        if (!requested.has(item.panelId)) {
          throw new QueryError('internal', 'panel stream contains an unknown panel ID', response.status)
        }
        const result = item.result
        if (results[item.panelId]) throw new QueryError('internal', `panel ${item.panelId} appears twice in stream`, response.status)
        results[item.panelId] = result
        onResult?.(item.panelId, result)
      }
      while (true) {
        const { done, value } = await reader.read()
        buffered += decoder.decode(value, { stream: !done })
        const lines = buffered.split('\n')
        buffered = lines.pop() ?? ''
        for (const line of lines) consume(line)
        if (done) break
      }
      consume(buffered)
      if (!complete) throw new QueryError('internal', 'panel stream ended before completion', response.status)
      return results
    }
    throw new QueryError('internal', 'panel stream has no body', response.status)
  }
}
