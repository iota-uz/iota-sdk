import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * IOTA SDK integration context provider
 * Consumes server-side context from window.__APPLET_CONTEXT__ or accepts it as a prop.
 */
import type { IotaContext as IotaContextType } from '../types/iota';
const IotaContext = createContext<IotaContextType | null>(null);
interface IotaContextProviderProps {
    /**
     * Explicit context object. When provided, the window global is not read.
     * Useful for tests, Storybook, or apps that manage their own context.
     */
    context?: IotaContextType;
    children: JSX.Element;
}
export function IotaContextProvider(solidProps1: IotaContextProviderProps) {
    // Prefer explicit prop; fall back to window global
    const resolved = solidProps1.context ?? (typeof window !== 'undefined' ? window.__APPLET_CONTEXT__ : undefined);
    if (!resolved) {
        throw new Error('APPLET_CONTEXT not found. Pass a `context` prop or ensure the server injected context into window.__APPLET_CONTEXT__.');
    }
    return (<IotaContext.Provider value={resolved}>
      {solidProps1.children}
    </IotaContext.Provider>);
}
export function useIotaContext(): IotaContextType {
    const context = useContext(IotaContext);
    if (!context) {
        throw new Error('useIotaContext must be used within IotaContextProvider');
    }
    return context;
}
/**
 * Check if user has a specific permission
 */
export function hasPermission(permission: string): boolean {
    const context = typeof window !== 'undefined' ? window.__APPLET_CONTEXT__ : undefined;
    if (!context) {
        return false;
    }
    return context.user.permissions.includes(permission);
}
