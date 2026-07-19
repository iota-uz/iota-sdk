import {
  QueryErrorResponseSchema,
  QueryRequestSchema,
  QueryResponseSchema,
  type QueryErrorCode,
  type QueryRequest,
  type QueryResponse,
} from '../contract'

export class QueryError extends Error {
  constructor(
    readonly code: QueryErrorCode,
    message: string,
    readonly status: number,
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
}

export function queryCacheKey(request: QueryRequest): string {
  return JSON.stringify([
    request.snapshotId,
    request.path,
    request.perspective ?? '',
    request.page ?? 0,
  ])
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
    const request = QueryRequestSchema.parse(input)
    if (this.snapshotId !== request.snapshotId) {
      this.snapshotId = request.snapshotId
      this.cache.clear()
    }
    const key = queryCacheKey(request)
    const cached = this.cache.get(key)
    if (cached && !options.force) return cached
    const pending = this.inFlight.get(key)
    if (pending) return pending

    const controller = new AbortController()
    this.controllers.add(controller)
    const promise = this.fetch(request, controller.signal)
      .then((response) => {
        if (this.snapshotId === request.snapshotId) this.cache.set(key, response)
        return response
      })
      .finally(() => {
        this.inFlight.delete(key)
        this.controllers.delete(controller)
      })
    this.inFlight.set(key, promise)
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
      throw new QueryError(code, message, response.status)
    }
    return QueryResponseSchema.parse(payload)
  }
}
