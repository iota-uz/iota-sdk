import {
  QueryErrorResponseSchema,
  QueryRequestSchema,
  QueryResponseSchema,
  type FailureReason,
  type QueryErrorCode,
  type QueryRequest,
  type QueryResponse,
} from '../contract'

export class QueryError extends Error {
  constructor(
    readonly code: QueryErrorCode,
    message: string,
    readonly status: number,
    /**
     * What kind of failure this was, when the server classified it. `message`
     * stays a developer string that is never shown; this is the part a reader
     * can be told about. Optional: a server that does not classify, and every
     * client-side failure, leaves it unset and the generic copy applies.
     */
    readonly reason?: FailureReason,
  ) {
    super(message)
    this.name = 'QueryError'
  }
}

export class SnapshotGoneError extends QueryError {
  constructor(message = 'snapshot is unknown or expired') {
    super('snapshot_gone', message, 410)
    this.name = 'SnapshotGoneError'
  }
}

export interface QueryClientOptions {
  csrf?: string
  fetcher?: typeof fetch
}

export interface QueryOptions {
  force?: boolean
  signal?: AbortSignal
}

export function queryCacheKey(request: QueryRequest): string {
  return JSON.stringify([
    request.snapshotId,
    request.path,
    request.perspective ?? '',
    request.page ?? 0,
  ])
}

function queryFlightKey(request: QueryRequest): string {
  return JSON.stringify([queryCacheKey(request), request.prefetch === true, request.idlePrefetch === true, request.revision ?? 0])
}

export class QueryClient {
  private readonly cache = new Map<string, QueryResponse>()
  private readonly inFlight = new Map<string, Promise<QueryResponse>>()
  private readonly controllers = new Set<AbortController>()
  private snapshotId?: string

  constructor(
    private readonly endpoint: string,
    private readonly options: QueryClientOptions = {},
  ) {}

  peek(request: QueryRequest): QueryResponse | undefined {
    return this.cache.get(queryCacheKey(request))
  }

  async query(input: QueryRequest, options: QueryOptions = {}): Promise<QueryResponse> {
    if (options.signal?.aborted) {
      throw options.signal.reason instanceof Error
        ? options.signal.reason
        : new DOMException('The operation was aborted', 'AbortError')
    }
    const request = QueryRequestSchema.parse(input)
    if (this.snapshotId !== request.snapshotId) {
      this.snapshotId = request.snapshotId
      this.cache.clear()
    }
    const key = queryCacheKey(request)
    const cached = this.cache.get(key)
    if (cached && !options.force) return cached
    const flightKey = queryFlightKey(request)
    const pending = this.inFlight.get(flightKey)
    if (pending) return pending

    const controller = new AbortController()
    const abort = () => controller.abort(options.signal?.reason)
    options.signal?.addEventListener('abort', abort, { once: true })
    this.controllers.add(controller)
    const promise = this.fetch(request, controller.signal)
      .then((response) => {
        if (this.snapshotId === request.snapshotId) this.cache.set(key, response)
        return response
      })
      .finally(() => {
        options.signal?.removeEventListener('abort', abort)
        this.inFlight.delete(flightKey)
        this.controllers.delete(controller)
      })
    this.inFlight.set(flightKey, promise)
    return promise
  }

  dispose(): void {
    for (const controller of this.controllers) controller.abort()
    this.controllers.clear()
    this.inFlight.clear()
  }

  private async fetch(request: QueryRequest, signal: AbortSignal): Promise<QueryResponse> {
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
      const message = parsed.success ? parsed.data.message : `query request failed with ${response.status}`
      if (response.status === 410 && code === 'snapshot_gone') throw new SnapshotGoneError(message)
      throw new QueryError(code, message, response.status, parsed.success ? parsed.data.reason : undefined)
    }
    return QueryResponseSchema.parse(payload)
  }
}
