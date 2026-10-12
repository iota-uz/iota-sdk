/**
 * Message sending, SSE streaming, and HITL question handling.
 *
 * @internal — Not part of the public API. Consumed by HttpDataSource.
 */
import type { BichatRPC } from './rpc.generated';
import type { ActiveRunDelivery, Attachment, StreamChunk, StreamStatus, QuestionAnswers, SendMessageOptions, AsyncRunAccepted, } from '../types';
import { parseBichatStream } from '../utils/sseParser';
import { STREAM_EVENT_TYPES, isTerminalEvent, } from '../utils/eventNames';
import { openManagedEventSource } from './openManagedEventSource';
import { ensureAttachmentUpload, assertUploadReferences, type CoreUploadResponse, } from './AttachmentUploader';
// ---------------------------------------------------------------------------
// Errors
// ---------------------------------------------------------------------------
/**
 * Thrown by {@link subscribeRunEvents} when the underlying EventSource
 * emits `onerror` before the first event arrives within the
 * initial-connect grace window. Distinguishes boundary failures
 * (401 / 404 / 503) from transient mid-run flaps, which the browser's
 * native auto-reconnect continues to handle silently.
 */
export class RunEventsConnectError extends Error {
    readonly cause?: Event;
    constructor(message: string, cause?: Event) {
        super(message);
        this.name = 'RunEventsConnectError';
        this.cause = cause;
    }
}
// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------
interface Result<T> {
    success: boolean;
    data?: T;
    error?: string;
}
type RPCCaller = <TMethod extends keyof BichatRPC & string>(method: TMethod, params: BichatRPC[TMethod]['params']) => Promise<BichatRPC[TMethod]['result']>;
type AttachmentLifecycleLogger = (event: 'stream_send_with_upload_ids', details: Record<string, unknown>) => void;
export interface MessageTransportDeps {
    callRPC: RPCCaller;
    baseUrl: string;
    streamEndpoint: string;
    rpcTimeoutMs: number;
    streamConnectTimeoutMs?: number;
    createHeaders: (additionalHeaders?: Record<string, string>) => Headers;
    uploadFileFn: (file: File) => Promise<CoreUploadResponse>;
    logAttachmentLifecycle: AttachmentLifecycleLogger;
}
// ---------------------------------------------------------------------------
// Request-id generation
// ---------------------------------------------------------------------------
/**
 * Generate a UUID-ish idempotency key for a single send. Prefers
 * crypto.randomUUID when available (every evergreen browser + Node
 * 16.7+); falls back to a Math.random-based v4 string so the feature
 * still works in constrained WebViews.
 *
 * The resulting id is returned inline in the POST /stream body as
 * `requestId` so the backend's SetNX dedupe can collapse duplicates.
 *
 * Backend contract:
 * - Dedupe window is 30 minutes on SetNX (see SDK stream handler).
 * - UUID v4 is expected — the backend parses the value as
 *   `uuid.UUID` via the Go stdlib, so malformed ids are rejected
 *   at the boundary.
 * - A double-click or cross-tab retry that sends the same
 *   `requestId` converges on the same run; the second call returns
 *   the existing run's `runId` instead of spawning a duplicate.
 */
function generateRequestId(): string {
    if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
        return crypto.randomUUID();
    }
    // RFC 4122 v4 fallback. Not cryptographically strong, but sufficient
    // for idempotency keys that live < 30 min.
    const bytes = new Uint8Array(16);
    for (let i = 0; i < bytes.length; i++) {
        bytes[i] = Math.floor(Math.random() * 256);
    }
    bytes[6] = (bytes[6] & 0x0f) | 0x40;
    bytes[8] = (bytes[8] & 0x3f) | 0x80;
    const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
    return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}
// ---------------------------------------------------------------------------
// Stream sending
// ---------------------------------------------------------------------------
export async function* sendMessage(deps: MessageTransportDeps, sessionId: string, content: string, attachments: Attachment[] = [], signal?: AbortSignal, options?: SendMessageOptions): AsyncGenerator<StreamChunk> {
    // Create new abort controller for this stream
    const abortController = new AbortController();
    // Link external signal if provided, with cleanup
    let onExternalAbort: (() => void) | undefined;
    if (signal) {
        onExternalAbort = () => { abortController.abort(); };
        signal.addEventListener('abort', onExternalAbort);
    }
    const url = `${deps.baseUrl}${deps.streamEndpoint}`;
    let connectionTimeoutID: ReturnType<typeof setTimeout> | undefined;
    let connectionTimedOut = false;
    try {
        const uploads = await Promise.all(attachments.map((attachment, attachmentIndex) => ensureAttachmentUpload(attachment, { sessionId, attachmentIndex }, deps.uploadFileFn)));
        const streamAttachments = assertUploadReferences(uploads);
        deps.logAttachmentLifecycle('stream_send_with_upload_ids', {
            sessionId,
            attachmentCount: streamAttachments.length,
        });
        // Idempotency: one request_id per send, client-generated unless
        // the caller provided a deterministic one (e.g. retry flow). The
        // backend dedupes duplicates within a ~30 min window so a double
        // click / tab-level retry converges on the same run. Falls back to
        // a stable pseudo-id on environments that lack crypto.randomUUID.
        const requestId = options?.requestId ?? generateRequestId();
        const payload: Record<string, unknown> = {
            sessionId,
            content,
            debugMode: options?.debugMode ?? false,
            replaceFromMessageId: options?.replaceFromMessageID,
            attachments: streamAttachments,
            requestId,
        };
        if (options?.reasoningEffort) {
            payload.reasoningEffort = options.reasoningEffort;
        }
        if (options?.model) {
            payload.model = options.model;
        }
        const timeoutMs = deps.streamConnectTimeoutMs ?? 0;
        if (timeoutMs > 0) {
            connectionTimeoutID = setTimeout(() => {
                connectionTimedOut = true;
                abortController.abort();
            }, timeoutMs);
        }
        const response = await fetch(url, {
            method: 'POST',
            headers: deps.createHeaders(),
            body: JSON.stringify(payload),
            signal: abortController.signal,
        });
        if (connectionTimeoutID !== undefined) {
            clearTimeout(connectionTimeoutID);
            connectionTimeoutID = undefined;
        }
        if (!response.ok) {
            throw new Error(`Stream request failed: HTTP ${response.status}`);
        }
        if (!response.body) {
            throw new Error('Response body is null');
        }
        const reader = response.body.getReader();
        for await (const chunk of parseBichatStream(reader)) {
            yield chunk;
            if (chunk.type === 'done' || chunk.type === 'error') {
                return;
            }
        }
    }
    catch (err) {
        if (err instanceof Error) {
            if (err.name === 'AbortError') {
                // A connection timeout is a genuine failure — surface it as an error
                // chunk. A user-initiated stop or a provider unmount, however, must
                // stay an AbortError so the machine takes its soft-cancel path (restore
                // input, no error banner) instead of treating the send as failed and
                // discarding the turn. Re-throw to preserve the error's identity;
                // yielding an error chunk here would flatten it into a generic Error.
                if (connectionTimedOut) {
                    yield {
                        type: 'error',
                        error: `Stream request timed out after ${deps.streamConnectTimeoutMs}ms`,
                    };
                }
                else {
                    throw err;
                }
            }
            else {
                yield {
                    type: 'error',
                    error: err.message,
                };
            }
        }
        else {
            yield {
                type: 'error',
                error: 'Unknown error',
            };
        }
    }
    finally {
        if (connectionTimeoutID !== undefined) {
            clearTimeout(connectionTimeoutID);
        }
        if (signal && onExternalAbort) {
            signal.removeEventListener('abort', onExternalAbort);
        }
    }
}
// ---------------------------------------------------------------------------
// Stop stream (explicit stop — backend discards partial assistant message)
// ---------------------------------------------------------------------------
function buildStreamUrl(deps: Pick<MessageTransportDeps, 'baseUrl' | 'streamEndpoint'>, path: string): string {
    const base = deps.baseUrl.replace(/\/+$/, '');
    const streamPath = deps.streamEndpoint.replace(/\/$/, '');
    return `${base}${streamPath}${path}`;
}
const DEFAULT_STOP_STREAM_TIMEOUT_MS = 5000;
export async function stopStream(deps: Pick<MessageTransportDeps, 'baseUrl' | 'streamEndpoint' | 'createHeaders'> & {
    timeoutMs?: number;
}, sessionId: string): Promise<void> {
    const timeoutMs = deps.timeoutMs ?? DEFAULT_STOP_STREAM_TIMEOUT_MS;
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), timeoutMs);
    try {
        const stopUrl = buildStreamUrl(deps, '/stop');
        const response = await fetch(stopUrl, {
            method: 'POST',
            headers: deps.createHeaders(),
            body: JSON.stringify({ sessionId }),
            signal: controller.signal,
        });
        if (!response.ok) {
            console.warn('Stop stream request failed:', response.status);
        }
    }
    catch (err) {
        if (err instanceof Error && err.name === 'AbortError') {
            console.warn('Stop stream request timed out');
        }
        else {
            throw err;
        }
    }
    finally {
        clearTimeout(timeoutId);
    }
}
// ---------------------------------------------------------------------------
// Stream status and resume (refresh-safe)
// ---------------------------------------------------------------------------
const DEFAULT_STREAM_STATUS_TIMEOUT_MS = 5000;
type StreamStatusResumeDeps = Pick<MessageTransportDeps, 'baseUrl' | 'streamEndpoint' | 'createHeaders'> & {
    timeoutMs?: number;
    connectTimeoutMs?: number;
};
export async function getStreamStatus(deps: StreamStatusResumeDeps, sessionId: string): Promise<StreamStatus | null> {
    const timeoutMs = deps.timeoutMs ?? DEFAULT_STREAM_STATUS_TIMEOUT_MS;
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), timeoutMs);
    try {
        const url = buildStreamUrl(deps, `/status?sessionId=${encodeURIComponent(sessionId)}`);
        const response = await fetch(url, {
            method: 'GET',
            headers: deps.createHeaders(),
            signal: controller.signal,
        });
        if (!response.ok) {
            if (response.status === 404) {
                return null;
            }
            console.warn('Stream status request failed:', response.status);
            return null;
        }
        const data = (await response.json()) as StreamStatus;
        return data;
    }
    catch (err) {
        if (err instanceof Error && err.name === 'AbortError') {
            return null;
        }
        throw err;
    }
    finally {
        clearTimeout(timeoutId);
    }
}
export async function resumeStream(deps: StreamStatusResumeDeps, sessionId: string, runId: string, onChunk: (chunk: StreamChunk) => void, signal?: AbortSignal): Promise<void> {
    const url = buildStreamUrl(deps, '/resume');
    const controller = new AbortController();
    let timeoutId: ReturnType<typeof setTimeout> | undefined;
    const timeoutMs = deps.connectTimeoutMs;
    if (timeoutMs != null && timeoutMs > 0) {
        timeoutId = setTimeout(() => controller.abort(), timeoutMs);
    }
    if (signal) {
        signal.addEventListener('abort', () => controller.abort());
    }
    try {
        const response = await fetch(url, {
            method: 'POST',
            headers: deps.createHeaders(),
            body: JSON.stringify({ sessionId, runId }),
            signal: controller.signal,
        });
        if (!response.ok) {
            throw new Error(`Resume stream failed: HTTP ${response.status}`);
        }
        if (timeoutId !== undefined) {
            clearTimeout(timeoutId);
            timeoutId = undefined;
        }
        if (!response.body) {
            throw new Error('Resume response body is null');
        }
        const reader = response.body.getReader();
        for await (const chunk of parseBichatStream(reader)) {
            onChunk(chunk);
            if (chunk.type === 'done' || chunk.type === 'error') {
                return;
            }
        }
    }
    finally {
        if (timeoutId !== undefined) {
            clearTimeout(timeoutId);
        }
    }
}
// ---------------------------------------------------------------------------
// Cursor-based event tail (Last-Event-ID reconnect)
// ---------------------------------------------------------------------------
type StreamEventsDeps = Pick<MessageTransportDeps, 'baseUrl' | 'streamEndpoint'>;
export interface SubscribeRunEventsOptions {
    /** When set, start from the last seen event id instead of a full replay. */
    lastEventId?: string;
    /** Fires for every chunk (content / tool / snapshot / done / error / …). */
    onChunk: (chunk: StreamChunk) => void;
    /** Optional hook for raw SSE errors (connection blips, parse failures). */
    onError?: (event: Event) => void;
    /** AbortSignal closes the underlying EventSource. */
    signal?: AbortSignal;
}
/**
 * Open a native EventSource against GET /stream/events for a run. The
 * browser handles reconnect + Last-Event-ID automatically on transient
 * network drops; we forward each SSE event onto onChunk and resolve the
 * returned promise when a terminal event (done / error) arrives or the
 * caller aborts.
 *
 * Kept separate from sendMessage so the applet can connect to a run
 * that was started by another tab (shared request_id) or that the
 * current session already had in flight (tab reopen, device switch).
 */
export function subscribeRunEvents(deps: StreamEventsDeps, sessionId: string, runId: string, options: SubscribeRunEventsOptions): Promise<void> {
    const base = buildStreamUrl(deps, '/events');
    const qs = new URLSearchParams({ sessionId, runId });
    const url = `${base}?${qs.toString()}`;
    // EventSource ignores custom headers in most browsers, so we leave
    // auth to the cookie that backs the session. Last-Event-ID is only
    // honoured by native reconnects — when the caller explicitly
    // supplies one (first connect after a known cursor) we append it as
    // a query parameter the server reads as a fallback. Native
    // reconnect adds the real `Last-Event-ID` header for subsequent
    // drops.
    const withCursor = options.lastEventId
        ? `${url}&${new URLSearchParams({ lastEventId: options.lastEventId }).toString()}`
        : url;
    // Track whether the caller has already received a terminal chunk —
    // openManagedEventSource only closes on abort or initial-connect
    // error, so we drive the settle handshake through AbortController.
    const settleController = new AbortController();
    if (options.signal) {
        if (options.signal.aborted) {
            settleController.abort();
        }
        else {
            options.signal.addEventListener('abort', () => settleController.abort(), {
                once: true,
            });
        }
    }
    return openManagedEventSource({
        url: withCursor,
        events: STREAM_EVENT_TYPES,
        withCredentials: true,
        signal: settleController.signal,
        onError: options.onError,
        onConnectError: (evt) => new RunEventsConnectError('EventSource failed to connect before first event', evt),
        onMessage: (name, data) => {
            if (typeof data === 'object' &&
                data !== null &&
                (data as {
                    __unparseable?: boolean;
                }).__unparseable) {
                // Surface parse failures as synthetic error chunks but keep
                // the stream open — native EventSource reconnects on transient
                // flaps.
                options.onChunk({
                    type: 'error',
                    error: `Failed to parse event: ${(data as {
                        raw: string;
                    }).raw}`,
                });
                return;
            }
            // Some server payloads omit `type` because it duplicates the SSE
            // event name; back-fill it so downstream consumers can rely on
            // StreamChunk.type being populated.
            const parsed = data as Partial<StreamChunk> & Record<string, unknown>;
            if (!parsed.type) {
                parsed.type = name as StreamChunk['type'];
            }
            options.onChunk(parsed as StreamChunk);
            if (isTerminalEvent(name) || isTerminalEvent(String(parsed.type))) {
                settleController.abort();
            }
        },
    });
}
// ---------------------------------------------------------------------------
// Active-run sidebar fan-out (per-tenant status SSE)
// ---------------------------------------------------------------------------
export interface SubscribeActiveRunsOptions {
    onEvent: (event: ActiveRunDelivery) => void;
    onError?: (event: Event) => void;
    signal?: AbortSignal;
}
/**
 * Open an EventSource to the per-tenant active-run feed. Emits one
 * "snapshot" row per currently-running session on connect then live
 * "update" deltas as runs transition. Used by the chat list to render
 * a status dot without polling each session.
 *
 * Never resolves on its own — the caller should abort via the signal
 * when the component unmounts; the returned promise only settles on
 * signal abort or initial-connect failure.
 */
export function subscribeActiveRuns(deps: StreamEventsDeps, options: SubscribeActiveRunsOptions): Promise<void> {
    const url = buildStreamUrl(deps, '/active-runs');
    return openManagedEventSource({
        url,
        events: ['snapshot', 'update'],
        withCredentials: true,
        signal: options.signal,
        onError: options.onError,
        onMessage: (name, data) => {
            if (typeof data !== 'object' ||
                data === null ||
                (data as {
                    __unparseable?: boolean;
                }).__unparseable) {
                // Server contract guarantees JSON; silently skip malformed
                // frames rather than surfacing them as fake entries.
                return;
            }
            const body = data as Omit<ActiveRunDelivery, 'event'>;
            options.onEvent({ event: name as ActiveRunDelivery['event'], ...body });
        },
    });
}
// ---------------------------------------------------------------------------
// Question submission / rejection
// ---------------------------------------------------------------------------
export async function submitQuestionAnswers(callRPC: RPCCaller, sessionId: string, questionId: string, answers: QuestionAnswers): Promise<Result<AsyncRunAccepted>> {
    try {
        // Convert QuestionAnswers to flat map[string]string for RPC
        const flatAnswers: Record<string, string> = {};
        for (const [qId, answerData] of Object.entries(answers)) {
            if (answerData.customText) {
                flatAnswers[qId] = answerData.customText;
            }
            else if (answerData.options.length > 0) {
                flatAnswers[qId] = answerData.options.join(', ');
            }
        }
        const result = await callRPC('bichat.question.submit', {
            sessionId,
            checkpointId: questionId,
            answers: flatAnswers,
        });
        return {
            success: true,
            data: normalizeAsyncRunAccepted(result),
        };
    }
    catch (err) {
        return { success: false, error: err instanceof Error ? err.message : 'Unknown error' };
    }
}
export async function rejectPendingQuestion(callRPC: RPCCaller, sessionId: string): Promise<Result<AsyncRunAccepted>> {
    try {
        const result = await callRPC('bichat.question.reject', { sessionId });
        return { success: true, data: normalizeAsyncRunAccepted(result) };
    }
    catch (err) {
        return { success: false, error: err instanceof Error ? err.message : 'Unknown error' };
    }
}
function isAsyncRunOperation(value: string): value is AsyncRunAccepted['operation'] {
    return value === 'question_submit' || value === 'question_reject' || value === 'session_compact';
}
function normalizeAsyncRunAccepted(input: {
    accepted: boolean;
    operation: string;
    sessionId: string;
    runId: string;
    startedAt: number;
}): AsyncRunAccepted {
    if (!input.accepted) {
        throw new Error('Async run request was not accepted');
    }
    if (!isAsyncRunOperation(input.operation)) {
        throw new Error(`Unexpected async operation: ${input.operation}`);
    }
    if (!input.sessionId || !input.runId) {
        throw new Error('Missing async run metadata');
    }
    return {
        accepted: true,
        operation: input.operation,
        sessionId: input.sessionId,
        runId: input.runId,
        startedAt: input.startedAt,
    };
}
