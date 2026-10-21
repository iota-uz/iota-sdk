import { createExternalSnapshot, createReactiveSnapshot } from './externalSnapshot';
import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * Chat session context provider and hooks.
 *
 * Thin React wrapper around ChatMachine. All async logic (session fetch,
 * streaming, HITL, slash commands, queue, rate limiting) lives in the
 * framework-agnostic ChatMachine class.
 *
 * Split into 3 focused contexts to minimize re-renders:
 * - ChatSessionContext: session lifecycle (session, fetching, error, debug)
 * - ChatMessagingContext: turns + streaming + tool interactions
 * - ChatInputContext: input form state (message, inputError, queue)
 */
import type { ChatDataSource, ChatSessionStateValue, ChatMessagingStateValue, ChatInputStateValue, } from '../types';
import { RateLimiter, type RateLimiterConfig } from '../utils/RateLimiter';
import { ChatMachine } from '../machine/ChatMachine';
// ---------------------------------------------------------------------------
// Internal context — holds the machine instance
// ---------------------------------------------------------------------------
const MachineCtx = createContext<ChatMachine | null>(null);
// ---------------------------------------------------------------------------
// Provider props
// ---------------------------------------------------------------------------
export interface ChatSessionProviderProps {
    dataSource: ChatDataSource;
    sessionId?: string;
    /**
     * External rate limiter instance. Captured once at mount — changing this prop
     * after initial render has no effect. For most cases, use `rateLimitConfig`
     * instead and let the provider create the limiter internally.
     */
    rateLimiter?: RateLimiter;
    /**
     * Configuration for the built-in rate limiter (ignored when `rateLimiter` is
     * provided). Captured once at mount — changing after initial render has no effect.
     */
    rateLimitConfig?: RateLimiterConfig;
    /**
     * Called when the machine creates a new session (e.g. on first message in a
     * "new chat"). Use this to navigate your SPA router to the new session URL.
     */
    onSessionCreated?: (sessionId: string) => void;
    children: JSX.Element;
}
const DEFAULT_RATE_LIMIT_CONFIG: RateLimiterConfig = {
    maxRequests: 20,
    windowMs: 60000,
};
// ---------------------------------------------------------------------------
// Provider
// ---------------------------------------------------------------------------
export function ChatSessionProvider(solidProps1: ChatSessionProviderProps) {
    // Create machine once (stable across re-renders)
    const machineRef = { current: null } as {
        current: (ChatMachine | null) | null;
    };
    if (!machineRef.current) {
        machineRef.current = new ChatMachine({
            get dataSource() {
                return solidProps1.dataSource;
            },
            rateLimiter: solidProps1.rateLimiter ||
                new RateLimiter(solidProps1.rateLimitConfig || DEFAULT_RATE_LIMIT_CONFIG),
            get onSessionCreated() {
                return solidProps1.onSessionCreated;
            },
        });
    }
    const machine = machineRef.current;
    // Sync mutable config (dataSource, onSessionCreated) on every render
    createEffect(on(() => [machine, solidProps1.dataSource, solidProps1.onSessionCreated], () => {
        const cleanup = untrack(() => {
            machine.updateConfig({ get dataSource() {
                    return solidProps1.dataSource;
                }, get onSessionCreated() {
                    return solidProps1.onSessionCreated;
                } });
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    // Sync sessionId prop → machine
    createEffect(on(() => [machine, solidProps1.sessionId], () => {
        const cleanup = untrack(() => {
            machine.setSessionId(solidProps1.sessionId);
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    // Cleanup on unmount
    createEffect(on(() => [machine], () => {
        const cleanup = untrack(() => {
            return () => {
                machine.dispose();
            };
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    return (<MachineCtx.Provider value={machine}>
      {solidProps1.children}
    </MachineCtx.Provider>);
}
// ---------------------------------------------------------------------------
// Private helper
// ---------------------------------------------------------------------------
function useMachine(): ChatMachine {
    const machine = useContext(MachineCtx);
    if (!machine) {
        throw new Error('Chat hooks must be used within ChatSessionProvider');
    }
    return machine;
}
// ---------------------------------------------------------------------------
// Public hooks (same signatures as before)
// ---------------------------------------------------------------------------
export function useChatSession(): ChatSessionStateValue {
    const machine = useMachine();
    return createReactiveSnapshot(machine.subscribeSession, machine.getSessionSnapshot);
}
export function useChatMessaging(): ChatMessagingStateValue {
    const machine = useMachine();
    return createReactiveSnapshot(machine.subscribeMessaging, machine.getMessagingSnapshot);
}
/** Returns messaging context or null when outside ChatSessionProvider. */
export function useOptionalChatMessaging(): ChatMessagingStateValue | null {
    const machine = useContext(MachineCtx);
    if (!machine)
        return null;
    return createReactiveSnapshot(machine.subscribeMessaging, machine.getMessagingSnapshot);
}
export function useChatInput(): ChatInputStateValue {
    const machine = useMachine();
    return createReactiveSnapshot(machine.subscribeInput, machine.getInputSnapshot);
}
// Helpers for useOptionalChatMessaging (must call hooks unconditionally)
function noopSubscribe(): () => void { return () => { }; }
function nullSnapshot(): null { return null; }
