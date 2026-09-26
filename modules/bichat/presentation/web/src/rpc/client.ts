import type { BichatRPC } from '../rpc.generated'
import type { SolidAppletContext } from '../applet/solidAppletElement'

export type BichatMethod = keyof BichatRPC & string

export interface BichatRPCError {
  code: string | number
  message: string
  details?: unknown
}

export class RPCError extends Error {
  readonly code: string | number
  readonly details?: unknown
  readonly status?: number

  constructor(init: { code: string | number; message: string; details?: unknown; status?: number }) {
    super(init.message)
    this.name = 'RPCError'
    this.code = init.code
    this.details = init.details
    this.status = init.status
  }
}

export interface BichatRPCClientOptions {
  endpoint: string
  csrfToken?: string | (() => string)
  timeoutMs?: number
}

function currentCsrf(token: BichatRPCClientOptions['csrfToken']): string {
  if (typeof token === 'function') return token()
  return token ?? ''
}

/**
 * Typed JSON-RPC 2.0 client for the applet RPC dispatcher. One request per
 * call (no batching), CSRF header on every mutation, timeout guard.
 */
export class BichatRPCClient {
  private nextID = 1

  constructor(private readonly options: BichatRPCClientOptions) {}

  async call<M extends BichatMethod>(
    method: M,
    params: BichatRPC[M]['params'],
  ): Promise<BichatRPC[M]['result']> {
    const id = String(this.nextID++)
    const controller = new AbortController()
    const timer = this.options.timeoutMs
      ? setTimeout(() => controller.abort(), this.options.timeoutMs)
      : undefined

    let response: Response
    try {
      response = await fetch(this.options.endpoint, {
        method: 'POST',
        credentials: 'same-origin',
        signal: controller.signal,
        headers: {
          'Content-Type': 'application/json',
          ...(currentCsrf(this.options.csrfToken)
            ? { 'X-CSRF-Token': currentCsrf(this.options.csrfToken) }
            : {}),
        },
        body: JSON.stringify({ jsonrpc: '2.0', id, method, params }),
      })
    } catch (error) {
      if (timer) clearTimeout(timer)
      if (controller.signal.aborted) throw new RPCError({ code: 'timeout', message: `RPC ${method} timed out` })
      throw new RPCError({ code: 'network', message: `RPC ${method} failed: ${String(error)}` })
    }
    if (timer) clearTimeout(timer)

    if (response.status === 401) {
      throw new RPCError({ code: 'unauthenticated', message: 'Authentication required', status: 401 })
    }
    if (response.status === 403) {
      throw new RPCError({ code: 'permission_denied', message: 'Permission denied', status: 403 })
    }
    if (!response.ok) {
      throw new RPCError({ code: 'transient', message: `RPC ${method} failed with status ${response.status}`, status: response.status })
    }

    const payload = (await response.json()) as {
      id?: unknown
      result?: unknown
      error?: { code?: unknown; message?: string; details?: unknown }
    }
    if (payload.error) {
      const code = payload.error.code
      throw new RPCError({
        code: typeof code === 'string' || typeof code === 'number' ? code : 'error',
        message: payload.error.message ?? 'Request failed',
        details: payload.error.details,
      })
    }
    return payload.result as BichatRPC[M]['result']
  }
}

export function createBichatRPCClient(ctx: SolidAppletContext): BichatRPCClient {
  return new BichatRPCClient({
    endpoint: ctx.config.rpcUIEndpoint,
    csrfToken: () => ctx.session?.csrfToken ?? '',
  })
}
