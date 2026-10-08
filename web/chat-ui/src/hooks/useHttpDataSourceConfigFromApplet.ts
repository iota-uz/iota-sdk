/** @jsxImportSource solid-js */
import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * Builds HttpDataSourceConfig from window.__APPLET_CONTEXT__.
 * For use with createHttpDataSource when the app is embedded via the applet framework.
 *
 * Expects the host to inject context with:
 * - config.rpcUIEndpoint, config.streamEndpoint
 * - session.csrfToken (or window.__CSRF_TOKEN__)
 */
import type { HttpDataSourceConfig } from '../data/HttpDataSource';
import type { IotaContext } from '../types/iota';
/**
 * Returns HttpDataSourceConfig derived from window.__APPLET_CONTEXT__.
 * Use with createHttpDataSource() for RPC and SSE endpoints.
 *
 * @throws Error if window.__APPLET_CONTEXT__ is not available
 */
export function useHttpDataSourceConfigFromApplet(options?: {
    rpcTimeoutMs?: number;
    streamConnectTimeoutMs?: number;
}): HttpDataSourceConfig {
    return createMemo(() => {
        const ctx = typeof window !== 'undefined' ? (window as Window & {
            __APPLET_CONTEXT__?: IotaContext;
        }).__APPLET_CONTEXT__ : undefined;
        if (!ctx) {
            throw new Error('Applet context not found. Ensure window.__APPLET_CONTEXT__ is injected by the backend.');
        }
        const rpcEndpoint = ctx.config?.rpcUIEndpoint ?? '/rpc';
        const streamEndpoint = ctx.config?.streamEndpoint ?? '/stream';
        const csrfToken = ctx.session?.csrfToken ??
            (typeof window !== 'undefined' ? (window as Window & {
                __CSRF_TOKEN__?: string;
            }).__CSRF_TOKEN__ : undefined) ??
            '';
        const isDev = typeof (import.meta as unknown as {
            env?: {
                DEV?: boolean;
            };
        }).env?.DEV === 'boolean'
            && (import.meta as unknown as {
                env?: {
                    DEV?: boolean;
                };
            }).env?.DEV;
        if (!csrfToken && isDev) {
            console.warn('[useHttpDataSourceConfigFromApplet] CSRF token is empty — requests may be rejected by the server.');
        }
        return {
            baseUrl: '',
            rpcEndpoint,
            streamEndpoint,
            csrfToken,
            rpcTimeoutMs: options?.rpcTimeoutMs ?? 120000,
            streamConnectTimeoutMs: options?.streamConnectTimeoutMs,
        };
    })();
}
