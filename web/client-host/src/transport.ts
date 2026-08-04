import { asHostError, HostError } from './errors'
import type { RPCTransport } from './rpc'

export interface SessionState {
  csrf?: string
  headers?: Readonly<Record<string, string>>
}

export interface SessionService {
  snapshot(): SessionState
  refresh(signal: AbortSignal): Promise<SessionState>
}

export class ManagedSession implements SessionService {
  private state: SessionState
  private refreshing: Promise<SessionState> | undefined
  constructor(initial: SessionState, private readonly renew: (signal: AbortSignal) => Promise<SessionState>) { this.state = initial }
  snapshot(): SessionState { return { ...this.state, headers: { ...this.state.headers } } }
  refresh(signal: AbortSignal): Promise<SessionState> {
    this.refreshing ??= this.renew(signal).then((state) => { this.state = state; return this.snapshot() }).finally(() => { this.refreshing = undefined })
    return this.refreshing
  }
}

export class FetchRPCTransport implements RPCTransport {
  private requestID = 0
  constructor(readonly endpoint: string, readonly session: SessionService = new ManagedSession({}, async () => ({}))) {}

  async call<TRequest, TResponse>(method: string, request: TRequest, signal: AbortSignal): Promise<TResponse> {
    return this.request<TRequest, TResponse>(method, request, signal, true)
  }

  private async request<TRequest, TResponse>(method: string, request: TRequest, signal: AbortSignal, mayRefresh: boolean): Promise<TResponse> {
    const session = this.session.snapshot()
    const response = await fetch(this.endpoint, {
      method: 'POST', credentials: 'same-origin', signal,
      headers: {
        'content-type': 'application/json',
        ...(session.csrf ? { 'x-csrf-token': session.csrf } : {}),
        ...session.headers,
      },
      body: JSON.stringify({ jsonrpc: '2.0', id: ++this.requestID, method, params: request }),
    })
    if (response.status === 401 && mayRefresh) {
      await this.session.refresh(signal)
      return this.request<TRequest, TResponse>(method, request, signal, false)
    }
    if (response.status === 401) throw new HostError('unauthenticated', 'Session expired')
    if (response.status === 403) throw new HostError('permission_denied', 'Permission denied')
    if (!response.ok) throw new HostError(response.status >= 500 ? 'transient' : 'unknown', `RPC HTTP ${response.status}`)
    const payload = await response.json() as { result?: TResponse; error?: unknown }
    if (payload.error) throw asHostError(payload.error)
    return payload.result as TResponse
  }
}
