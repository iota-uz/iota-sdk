export interface AppletRPCError {
    code: string;
    message: string;
    details?: unknown;
}
export class AppletRPCException extends Error {
    code: string;
    details?: unknown;
    cause?: unknown;
    constructor(args: {
        code: string;
        message: string;
        details?: unknown;
        cause?: unknown;
    }) {
        super(args.message);
        this.name = 'AppletRPCException';
        this.code = args.code;
        this.details = args.details;
        this.cause = args.cause;
    }
}
export type AppletRPCSchema = Record<string, {
    params: unknown;
    result: unknown;
}>;
interface RPCRequest {
    id: string;
    method: string;
    params: unknown;
}
interface RPCResponse<TResult> {
    id: string;
    result?: TResult;
    error?: AppletRPCError;
}
export interface CreateAppletRPCClientOptions {
    endpoint: string;
    fetcher?: typeof fetch;
    timeoutMs?: number;
}
export function createAppletRPCClient(options: CreateAppletRPCClientOptions) {
    const fetcher = options.fetcher ?? fetch;
    const timeoutMs = typeof options.timeoutMs === 'number' && options.timeoutMs > 0 ? options.timeoutMs : 0;
    async function call<TParams, TResult>(method: string, params: TParams): Promise<TResult> {
        const req: RPCRequest = { id: crypto.randomUUID(), method, params };
        const startedAt = typeof performance !== 'undefined' ? performance.now() : Date.now();
        const abortController = timeoutMs > 0 ? new AbortController() : undefined;
        let timeoutHandle: ReturnType<typeof setTimeout> | undefined;
        let timedOut = false;
        maybeDispatchRPCEvent({
            id: req.id,
            method: req.method,
            status: 'start',
        });
        try {
            if (abortController) {
                timeoutHandle = setTimeout(() => {
                    timedOut = true;
                    abortController.abort();
                }, timeoutMs);
            }
            const resp = await fetcher(options.endpoint, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(req),
                signal: abortController?.signal,
            });
            if (!resp.ok) {
                throw new AppletRPCException({
                    code: 'http_error',
                    message: `HTTP ${resp.status}`,
                    details: { status: resp.status },
                });
            }
            const json = (await resp.json()) as RPCResponse<TResult>;
            if (json.error) {
                throw new AppletRPCException({
                    code: json.error.code,
                    message: json.error.message,
                    details: json.error.details,
                });
            }
            if (json.result === undefined) {
                throw new AppletRPCException({
                    code: 'invalid_response',
                    message: 'Missing result in successful response',
                });
            }
            maybeDispatchRPCEvent({
                id: req.id,
                method: req.method,
                status: 'success',
                durationMs: elapsedMs(startedAt),
            });
            return json.result as TResult;
        }
        catch (err) {
            let rpcErr: unknown = err;
            if (err instanceof Error && err.name === 'AbortError') {
                rpcErr = new AppletRPCException({
                    code: timedOut ? 'timeout' : 'aborted',
                    message: timedOut ? `RPC request timed out after ${timeoutMs}ms` : 'RPC request was aborted',
                    cause: err,
                });
            }
            maybeDispatchRPCEvent({
                id: req.id,
                method: req.method,
                status: 'error',
                durationMs: elapsedMs(startedAt),
                error: rpcErr,
            });
            throw rpcErr;
        }
        finally {
            if (timeoutHandle !== undefined) {
                clearTimeout(timeoutHandle);
            }
        }
    }
    async function callTyped<TRouter extends AppletRPCSchema, TMethod extends keyof TRouter & string>(method: TMethod, params: TRouter[TMethod]['params']): Promise<TRouter[TMethod]['result']> {
        return call(method, params) as Promise<TRouter[TMethod]['result']>;
    }
    return { call, callTyped };
}
type RPCDevEvent = {
    id: string;
    method: string;
    status: 'start' | 'success' | 'error';
    durationMs?: number;
    error?: unknown;
};
function maybeDispatchRPCEvent(detail: RPCDevEvent) {
    if (typeof window === 'undefined') {
        return;
    }
    if (!shouldEnableAppletDevtools()) {
        return;
    }
    window.dispatchEvent(new CustomEvent('iota:applet-rpc', { detail }));
}
function shouldEnableAppletDevtools(): boolean {
    if (typeof window === 'undefined') {
        return false;
    }
    const url = new URL(window.location.href);
    if (url.searchParams.get('appletDebug') === '1') {
        return true;
    }
    try {
        return window.localStorage.getItem('iotaAppletDevtools') === '1';
    }
    catch {
        return false;
    }
}
function elapsedMs(startedAt: number): number {
    const now = typeof performance !== 'undefined' ? performance.now() : Date.now();
    return Math.max(0, Math.round(now - startedAt));
}
