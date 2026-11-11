// Browser transport for useCommand / useQuery: POST /_ext/rpc, executed on the
// server as the session user with the permissions they hold now. Business
// errors keep {kind, code, params, issues}; mutations are never retried.
import { RemoteError, type ClientTransport, type RemoteErrorBody } from '../client.ts'
import type { ImplKind, Json } from '../model.ts'
import { RPC_PATH } from './protocol.ts'

export interface CallOptions {
  signal?: AbortSignal
}

/** The transport the hooks use: ClientTransport plus cancellation. */
export interface RuntimeTransport {
  invoke(kind: Extract<ImplKind, 'query' | 'command' | 'atomic'>, resource: string, input: Json, options?: CallOptions): Promise<Json>
}

/** The request never produced a structured answer (network, proxy, malformed body). */
export class TransportError extends Error {
  readonly kind = 'transport'
  readonly status: number | undefined
  readonly retriable: boolean
  // No parameter properties: the host build strips types only (scripts/build-host.mjs).
  constructor(message: string, status: number | undefined, retriable: boolean, options?: { cause?: unknown }) {
    super(message, options)
    this.name = 'TransportError'
    this.status = status
    this.retriable = retriable
  }
}

/** The session is gone and could not be refreshed. */
export class UnauthenticatedError extends Error {
  readonly kind = 'unauthenticated'
  readonly reauthUrl: string | undefined
  constructor(reauthUrl?: string) {
    super('Session expired')
    this.name = 'UnauthenticatedError'
    this.reauthUrl = reauthUrl
  }
}

/** Session access reused from the client host (CSRF token, refresh). */
export interface SessionLike {
  snapshot(): { csrf?: string; headers?: Readonly<Record<string, string>> }
  refresh(signal: AbortSignal): Promise<unknown>
}

export interface RpcTransportOptions {
  endpoint?: string
  session?: SessionLike
  fetch?: typeof fetch
  /** Extra attempts for a query after a network failure or 502/503/504. Default 1. Commands never retry. */
  queryRetries?: number
  retryDelayMs?: number
  sleep?: (ms: number) => Promise<void>
  reauthUrl?: string
}

interface Envelope {
  ok?: boolean
  value?: Json
  error?: RemoteErrorBody
  invocation?: string
}

const TRANSIENT = new Set([502, 503, 504])

function abortError(signal: AbortSignal): unknown {
  return signal.reason ?? new DOMException('Aborted', 'AbortError')
}

export function createRpcTransport(options: RpcTransportOptions = {}): RuntimeTransport {
  const endpoint = options.endpoint ?? RPC_PATH
  const doFetch = options.fetch ?? ((...args: Parameters<typeof fetch>) => fetch(...args))
  const retries = Math.max(0, options.queryRetries ?? 1)
  const delay = options.retryDelayMs ?? 200
  const sleep = options.sleep ?? ((ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms)))

  async function once(kind: string, resource: string, input: Json, signal: AbortSignal | undefined): Promise<Json> {
    const session = options.session?.snapshot()
    let response: Response
    try {
      response = await doFetch(endpoint, {
        method: 'POST',
        credentials: 'same-origin',
        ...(signal ? { signal } : {}),
        headers: {
          'content-type': 'application/json',
          accept: 'application/json',
          ...(session?.csrf ? { 'x-csrf-token': session.csrf } : {}),
          ...session?.headers,
        },
        body: JSON.stringify({ kind, resource, input }),
      })
    } catch (cause) {
      if (signal?.aborted) throw abortError(signal)
      throw new TransportError(`Request to ${resource} failed`, undefined, true, { cause })
    }
    let body: Envelope | undefined
    try {
      body = await response.json() as Envelope
    } catch {
      body = undefined
    }
    if (body && body.ok === true) return (body.value ?? null) as Json
    // The session is gone, whatever the body says.
    if (response.status === 401) throw new UnauthenticatedError(options.reauthUrl)
    if (body?.error && typeof body.error.kind === 'string') throw new RemoteError(body.error)
    throw new TransportError(`Request to ${resource} failed with HTTP ${response.status}`, response.status, TRANSIENT.has(response.status))
  }

  async function invoke(kind: Extract<ImplKind, 'query' | 'command' | 'atomic'>, resource: string, input: Json, call: CallOptions = {}): Promise<Json> {
    const query = kind === 'query'
    let refreshed = false
    for (let attempt = 0; ; attempt++) {
      if (call.signal?.aborted) throw abortError(call.signal)
      try {
        return await once(kind, resource, input, call.signal)
      } catch (error) {
        if (!query) throw error // a mutation may have run; never repeat it
        if (call.signal?.aborted) throw error
        if (error instanceof UnauthenticatedError && !refreshed && options.session) {
          // Rejected before execution, so a read can safely follow a refresh.
          refreshed = true
          try {
            await options.session.refresh(call.signal ?? new AbortController().signal)
          } catch {
            throw error
          }
          continue
        }
        if (error instanceof TransportError && error.retriable && attempt < retries) {
          await sleep(delay)
          continue
        }
        throw error
      }
    }
  }

  return { invoke }
}

/** Adapts a RuntimeTransport (with a cancellation signal) to the author-facing ClientTransport. */
export function bindTransport(transport: RuntimeTransport, signal?: AbortSignal): ClientTransport {
  return { invoke: (kind, resource, input) => transport.invoke(kind, resource, input, signal ? { signal } : {}) }
}
