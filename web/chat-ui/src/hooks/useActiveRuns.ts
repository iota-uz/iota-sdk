/** @jsxImportSource solid-js */
import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * useActiveRuns — live map of per-session generation status.
 *
 * Subscribes to the data source's `subscribeActiveRuns` channel
 * (`GET /bi-chat/stream/active-runs`) and maintains a
 * sessionId → status dictionary so the sidebar can render a status
 * dot next to each session card without polling /stream/status per
 * session.
 *
 * The hook is a no-op when:
 * - the data source does not implement subscribeActiveRuns (older
 *   backends without the active-run index);
 * - `enabled` is false (e.g. the user is offline).
 */
import type { ActiveRunDelivery, ChatDataSource } from '../types';
export interface ActiveRunSnapshot {
    runId: string;
    status: ActiveRunDelivery['status'];
    updatedAt: number;
}
export interface UseActiveRunsOptions {
    enabled?: boolean;
    /** Optional hook for raw SSE errors. */
    onError?: (event: Event) => void;
    /**
     * Keep terminal-status entries in the map for this many milliseconds
     * before pruning them. Default 0 preserves the previous behaviour
     * (synchronous delete). Useful when consumers want to render a
     * "completed" pulse animation before the dot disappears.
     */
    retainTerminalMs?: number;
    /**
     * How long to wait for an initial snapshot batch before declaring
     * the hook `ready` when the server has zero active runs.
     * Default 250ms.
     */
    emptyStateTimeoutMs?: number;
}
export interface UseActiveRunsResult {
    /** sessionId → current live status. Terminal statuses are emitted then the entry is removed. */
    runs: Record<string, ActiveRunSnapshot>;
    /** True once the initial HGETALL snapshot is delivered. */
    ready: boolean;
    /** Convenience: undefined when not active. */
    status: (sessionId: string) => ActiveRunDelivery['status'] | undefined;
}
const TERMINAL = new Set<ActiveRunDelivery['status']>([
    'completed',
    'cancelled',
    'failed',
]);
export function useActiveRuns(source: Pick<ChatDataSource, 'subscribeActiveRuns'> | (() => Pick<ChatDataSource, 'subscribeActiveRuns'>), options: UseActiveRunsOptions = {}): UseActiveRunsResult {
    const [runs, setRuns] = createSignal<Record<string, ActiveRunSnapshot>>({});
    const [ready, setReady] = createSignal(false);
    const onErrorRef = { current: options.onError };
    onErrorRef.current = options.onError;
    const dataSource = () => typeof source === 'function' ? source() : source;
    const enabled = () => options.enabled ?? true;
    const retainTerminalMs = () => options.retainTerminalMs ?? 0;
    const emptyStateTimeoutMs = () => options.emptyStateTimeoutMs ?? 250;
    createEffect(on(() => [dataSource(), enabled(), retainTerminalMs(), emptyStateTimeoutMs()], () => {
        const cleanup = untrack(() => {
            if (!enabled()) {
                return;
            }
            const subscribe = dataSource().subscribeActiveRuns;
            if (!subscribe) {
                return;
            }
            const controller = new AbortController();
            // Collect snapshot rows into a staging object so we only flip
            // `ready` once at the end of the initial batch. Each SSE
            // "snapshot" event is a single row; the server never emits a
            // delimiter, so we use a microtask coalescer instead.
            let stagingTimer: ReturnType<typeof setTimeout> | undefined;
            const staging: Record<string, ActiveRunSnapshot> = {};
            let sawSnapshotRow = false;
            // Pending terminal-removal timers keyed by sessionId so unmount
            // can clear them and repeat terminals for the same session don't
            // leak timers.
            const pendingTerminalTimers = new Map<string, ReturnType<typeof setTimeout>>();
            const flushSnapshot = () => {
                setRuns((prev) => ({ ...prev, ...staging }));
                setReady(true);
                stagingTimer = undefined;
            };
            setRuns({}); setReady(false);
            const subscription = subscribe.call(dataSource(), {
                signal: controller.signal,
                onError: (evt) => options.onError?.(evt),
                onEvent: (evt) => {
                    if (controller.signal.aborted) return;
                    if (evt.event === 'snapshot') {
                        sawSnapshotRow = true;
                        // If a terminal-removal timer is pending for this sessionId
                        // (e.g. the previous run completed and we're inside the
                        // retention window), cancel it. A fresh snapshot — typically
                        // delivered after native EventSource reconnect or when the
                        // user restarts a run — must NOT be wiped by a stale prune.
                        const pending = pendingTerminalTimers.get(evt.sessionId);
                        if (pending !== undefined) {
                            clearTimeout(pending);
                            pendingTerminalTimers.delete(evt.sessionId);
                        }
                        staging[evt.sessionId] = {
                            runId: evt.runId,
                            status: evt.status,
                            updatedAt: evt.updatedAt,
                        };
                        if (stagingTimer === undefined) {
                            stagingTimer = setTimeout(flushSnapshot, 16);
                        }
                        return;
                    }
                    // update: apply immediately.
                    if (TERMINAL.has(evt.status)) {
                        if (retainTerminalMs() <= 0) {
                            // Legacy synchronous delete path.
                            setRuns((prev) => {
                                if (!(evt.sessionId in prev)) {
                                    return prev;
                                }
                                const next = { ...prev };
                                delete next[evt.sessionId];
                                return next;
                            });
                            return;
                        }
                        // Retain the entry with its terminal status so the consumer
                        // can render a completion pulse, then prune after the
                        // retention window.
                        setRuns((prev) => ({
                            ...prev,
                            [evt.sessionId]: {
                                runId: evt.runId,
                                status: evt.status,
                                updatedAt: evt.updatedAt,
                            },
                        }));
                        const existing = pendingTerminalTimers.get(evt.sessionId);
                        if (existing !== undefined) {
                            clearTimeout(existing);
                        }
                        const handle = setTimeout(() => {
                            pendingTerminalTimers.delete(evt.sessionId);
                            setRuns((prev) => {
                                if (!(evt.sessionId in prev)) {
                                    return prev;
                                }
                                const next = { ...prev };
                                delete next[evt.sessionId];
                                return next;
                            });
                        }, retainTerminalMs());
                        pendingTerminalTimers.set(evt.sessionId, handle);
                        return;
                    }
                    // Non-terminal update: write through and cancel any pending
                    // prune for this session (run restarted before the retention
                    // timer fired).
                    const pending = pendingTerminalTimers.get(evt.sessionId);
                    if (pending !== undefined) {
                        clearTimeout(pending);
                        pendingTerminalTimers.delete(evt.sessionId);
                    }
                    setRuns((prev) => ({
                        ...prev,
                        [evt.sessionId]: {
                            runId: evt.runId,
                            status: evt.status,
                            updatedAt: evt.updatedAt,
                        },
                    }));
                },
            });
            // The subscription promise rejects on initial-connect failures
            // (401/503 before the grace window elapses). Route that through
            // onError instead of leaking an unhandled rejection to the global
            // scope — callers expect a single failure surface.
            subscription?.catch((err: unknown) => {
                if (controller.signal.aborted) {
                    return;
                }
                const surface = err instanceof Event ? err : new Event('error');
                options.onError?.(surface);
            });
            // If the server has zero active runs on connect it never emits a
            // snapshot row; mark ready after a brief idle so consumers don't
            // spin forever.
            const readyTimeout = setTimeout(() => {
                if (!sawSnapshotRow) {
                    setReady(true);
                }
            }, emptyStateTimeoutMs());
            return () => {
                controller.abort();
                if (stagingTimer !== undefined) {
                    clearTimeout(stagingTimer);
                }
                clearTimeout(readyTimeout);
                for (const handle of pendingTerminalTimers.values()) {
                    clearTimeout(handle);
                }
                pendingTerminalTimers.clear();
            };
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    const status = (sessionId: string) => runs()[sessionId]?.status;
    return { get runs() {
            return runs();
        }, get ready() {
            return ready();
        }, status };
}
