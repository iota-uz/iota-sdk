import { MemoryQueryCache, queryKey, type QueryCache, type QueryKey } from './cache'
import { asHostError, dispatchHostError, HostError, type HostErrorHandlers } from './errors'

export interface RPCMethodContract {
  namespace: string
  method: string
  kind: 'query' | 'mutation'
  cacheable?: boolean
  maxRetries?: number
  invalidates?: readonly string[]
}

export interface RPCTransport {
  call<TRequest, TResponse>(method: string, request: TRequest, signal: AbortSignal): Promise<TResponse>
}

export interface RPCDiagnostics {
  call?(event: { method: string; key?: QueryKey; state: 'start' | 'success' | 'error' | 'cancelled'; error?: HostError }): void
}

export interface RPCClientOptions {
  cache?: QueryCache
  errors?: HostErrorHandlers
  diagnostics?: RPCDiagnostics
}

export class RPCClient {
  readonly cache: QueryCache
  private queryGeneration = 0
  private readonly latestQueryGenerations = new Map<string, number>()
  constructor(readonly transport: RPCTransport, readonly options: RPCClientOptions = {}) {
    this.cache = options.cache ?? new MemoryQueryCache()
  }

  async query<TRequest, TResponse>(contract: RPCMethodContract, request: TRequest, signal: AbortSignal): Promise<TResponse> {
    if (contract.kind !== 'query') throw new HostError('protocol', `${contract.method} is not a query`)
    const key = queryKey(contract.namespace, contract.method, request)
    if (contract.cacheable !== false) {
      const cached = this.cache.get<TResponse>(key)
      if (cached !== undefined) return cached
    }
    const encodedKey = JSON.stringify(key)
    const generation = ++this.queryGeneration
    this.latestQueryGenerations.set(encodedKey, generation)
    let attempt = 0
    const configuredRetries = contract.maxRetries ?? 2
    const maxRetries = Number.isFinite(configuredRetries)
      ? Math.max(0, Math.min(3, Math.trunc(configuredRetries)))
      : 2
    try {
      for (;;) {
        this.options.diagnostics?.call?.({ method: contract.method, key, state: 'start' })
        try {
          const response = await this.transport.call<TRequest, TResponse>(contract.method, request, signal)
          if (contract.cacheable !== false && this.latestQueryGenerations.get(encodedKey) === generation) this.cache.set(key, response)
          this.options.diagnostics?.call?.({ method: contract.method, key, state: 'success' })
          return response
        } catch (cause) {
          if (signal.aborted) {
            this.options.diagnostics?.call?.({ method: contract.method, key, state: 'cancelled' })
            throw cause
          }
          const error = asHostError(cause)
          if (!error.retryable || attempt >= maxRetries) {
            dispatchHostError(error, this.options.errors)
            this.options.diagnostics?.call?.({ method: contract.method, key, state: 'error', error })
            throw error
          }
          attempt += 1
        }
      }
    } finally {
      if (this.latestQueryGenerations.get(encodedKey) === generation) this.latestQueryGenerations.delete(encodedKey)
    }
  }

  async mutate<TRequest, TResponse>(contract: RPCMethodContract, request: TRequest, signal: AbortSignal): Promise<TResponse> {
    if (contract.kind !== 'mutation') throw new HostError('protocol', `${contract.method} is not a mutation`)
    this.options.diagnostics?.call?.({ method: contract.method, state: 'start' })
    try {
      const response = await this.transport.call<TRequest, TResponse>(contract.method, request, signal)
      this.cache.invalidate(contract.namespace, contract.invalidates ?? [])
      this.options.diagnostics?.call?.({ method: contract.method, state: 'success' })
      return response
    } catch (cause) {
      const error = asHostError(cause)
      if (!signal.aborted) dispatchHostError(error, this.options.errors)
      this.options.diagnostics?.call?.({ method: contract.method, state: signal.aborted ? 'cancelled' : 'error', error })
      throw error
    }
  }
}
