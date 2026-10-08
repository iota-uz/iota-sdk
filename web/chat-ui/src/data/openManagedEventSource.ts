/**
 * openManagedEventSource — shared EventSource primitive.
 *
 * Centralises the listener wiring, AbortSignal handling, and the
 * 500ms initial-connect grace used by subscribeRunEvents and
 * subscribeActiveRuns. Future fan-out helpers can adopt this instead
 * of duplicating the EventSource boilerplate.
 *
 * Behaviour:
 * - Registers an addEventListener for every name in `events`. The
 *   callback receives the event name and the JSON-parsed payload.
 * - On JSON parse failure, the callback is invoked with
 *   `{ __unparseable: true, raw: <string> }` so the caller can log or
 *   surface a synthetic error. The subscription stays open; native
 *   EventSource reconnect handles transient flaps.
 * - Initial-connect grace: when `onerror` fires before any event is
 *   received AND within `connectGraceMs` of construction, the
 *   returned promise rejects with either the caller-provided error
 *   (via `onConnectError`) or a generic Error. After the grace
 *   window expires, `onerror` is non-fatal — the browser reconnects
 *   silently.
 * - AbortSignal close: closing the source resolves the promise.
 *   Already-aborted signals short-circuit before construction.
 */
export interface UnparseableEventPayload {
    __unparseable: true;
    raw: string;
}
export interface OpenManagedEventSourceOptions {
    url: string;
    /** Every named event to subscribe to via addEventListener. */
    events: readonly string[];
    /**
     * Called for each event received. `data` is either the JSON-parsed
     * payload or a sentinel `{ __unparseable: true, raw }` when the
     * payload could not be parsed.
     */
    onMessage: (name: string, data: unknown) => void;
    /** Optional hook for raw EventSource errors (all flaps, not just initial). */
    onError?: (e: Event) => void;
    /** Close the source and resolve the promise when the signal aborts. */
    signal?: AbortSignal;
    /** Forwarded to the EventSource constructor. Defaults to true. */
    withCredentials?: boolean;
    /** Initial-connect grace window in ms. Default 500. Set 0 to disable the probe. */
    connectGraceMs?: number;
    /**
     * Maps the raw onerror Event into a rejection Error. Called only
     * when the error fires within the grace window and before any event
     * has been received. Defaults to a plain Error.
     */
    onConnectError?: (e: Event) => Error;
}
export function openManagedEventSource(opts: OpenManagedEventSourceOptions): Promise<void> {
    const graceMs = opts.connectGraceMs ?? 500;
    return new Promise<void>((resolve, reject) => {
        const startedAt = Date.now();
        const es = new EventSource(opts.url, {
            withCredentials: opts.withCredentials ?? true,
        });
        let settled = false;
        let sawEvent = false;
        const settle = (err?: Error) => {
            if (settled) {
                return;
            }
            settled = true;
            es.close();
            if (err) {
                reject(err);
            }
            else {
                resolve();
            }
        };
        if (opts.signal) {
            if (opts.signal.aborted) {
                settle();
                return;
            }
            opts.signal.addEventListener('abort', () => settle(), { once: true });
        }
        const forward = (name: string) => (evt: MessageEvent) => {
            sawEvent = true;
            let parsed: unknown;
            try {
                parsed = JSON.parse(evt.data);
            }
            catch {
                parsed = { __unparseable: true, raw: String(evt.data) } as UnparseableEventPayload;
            }
            try {
                opts.onMessage(name, parsed);
            }
            catch {
                // Caller errors are non-fatal — EventSource stays open.
            }
        };
        for (const name of opts.events) {
            es.addEventListener(name, forward(name) as EventListener);
        }
        es.onerror = (evt) => {
            opts.onError?.(evt);
            if (graceMs > 0 && !sawEvent && Date.now() - startedAt < graceMs) {
                const err = opts.onConnectError
                    ? opts.onConnectError(evt)
                    : new Error('EventSource failed to connect before first event');
                settle(err);
            }
            // Otherwise let native reconnect handle the flap.
        };
    });
}
