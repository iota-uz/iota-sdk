export interface AppletRPCError {
  code: string
  message: string
  details?: unknown
}

export type AppletRPCSchema = Record<string, { params: unknown; result: unknown }>

interface RPCRequest {
  id: string
  method: string
  params: unknown
}

interface RPCResponse<TResult> {
  id: string
  result?: TResult
  error?: AppletRPCError
}

export interface CreateAppletRPCClientOptions {
  endpoint: string
  fetcher?: typeof fetch
}

export function createAppletRPCClient(options: CreateAppletRPCClientOptions) {
  const fetcher = options.fetcher ?? fetch

  async function call<TParams, TResult>(method: string, params: TParams): Promise<TResult> {
    const req: RPCRequest = { id: crypto.randomUUID(), method, params }
    const startedAt = typeof performance !== 'undefined' ? performance.now() : Date.now()
    maybeDispatchRPCEvent({
      id: req.id,
      method: req.method,
      status: 'start',
    })

    try {
      const resp = await fetcher(options.endpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(req),
      })

      if (!resp.ok) {
        throw { code: 'http_error', message: `HTTP ${resp.status}` } satisfies AppletRPCError
      }

      const json = (await resp.json()) as RPCResponse<TResult>
      if (json.error) {
        throw json.error
      }

      maybeDispatchRPCEvent({
        id: req.id,
        method: req.method,
        status: 'success',
        durationMs: elapsedMs(startedAt),
      })

      return json.result as TResult
    } catch (err) {
      maybeDispatchRPCEvent({
        id: req.id,
        method: req.method,
        status: 'error',
        durationMs: elapsedMs(startedAt),
        error: err,
      })
      throw err
    }
  }

  async function callTyped<
    TRouter extends AppletRPCSchema,
    TMethod extends keyof TRouter & string,
  >(method: TMethod, params: TRouter[TMethod]['params']): Promise<TRouter[TMethod]['result']> {
    return call(method, params) as Promise<TRouter[TMethod]['result']>
  }

  return { call, callTyped }
}

type RPCDevEvent = {
  id: string
  method: string
  status: 'start' | 'success' | 'error'
  durationMs?: number
  error?: unknown
}

function maybeDispatchRPCEvent(detail: RPCDevEvent) {
  if (typeof window === 'undefined') return

  const url = new URL(window.location.href)
  const enabledByQuery = url.searchParams.get('appletDebug') === '1'
  let enabledByStorage = false
  try {
    enabledByStorage = window.localStorage.getItem('iotaAppletDevtools') === '1'
  } catch {
    enabledByStorage = false
  }

  if (!enabledByQuery && !enabledByStorage) return

  window.dispatchEvent(new CustomEvent('iota:applet-rpc', { detail }))
}

function elapsedMs(startedAt: number): number {
  const now = typeof performance !== 'undefined' ? performance.now() : Date.now()
  return Math.max(0, Math.round(now - startedAt))
}
