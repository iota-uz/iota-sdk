/** @jsxImportSource solid-js */
import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * useToast Hook
 * Manages toast notification state
 */
export type ToastType = 'success' | 'error' | 'info' | 'warning';
export interface ToastAction {
    label: string;
    onClick: () => void;
}
export interface ToastItem {
    id: string;
    type: ToastType;
    message: string;
    duration?: number;
    action?: ToastAction;
}
export interface UseToastReturn {
    toasts: ToastItem[];
    success: (msg: string, duration?: number, action?: ToastAction) => void;
    error: (msg: string, duration?: number, action?: ToastAction) => void;
    info: (msg: string, duration?: number, action?: ToastAction) => void;
    warning: (msg: string, duration?: number, action?: ToastAction) => void;
    dismiss: (id: string) => void;
    dismissAll: () => void;
}
/**
 * Generate a unique ID for a toast
 */
function generateId(): string {
    return Math.random().toString(36).substring(7);
}
const DEDUPE_WINDOW_MS = 2500;
const MAX_ACTIVE_TOASTS = 5;
/**
 * Hook for managing toast notifications
 *
 * @example
 * ```tsx
 * const { toasts, success, error, dismiss } = useToast()
 *
 * // Show a success toast
 * success('Operation completed!')
 *
 * // Show an error toast with custom duration
 * error('Something went wrong', 10000)
 *
 * // Render toasts
 * <ToastContainer toasts={toasts} onDismiss={dismiss} />
 * ```
 */
export function useToast(): UseToastReturn {
    const [toasts, setToasts] = createSignal<ToastItem[]>([]);
    const recentToastMapRef = { current: new Map() } as {
        current: Map<string, number>;
    };
    const showToast = (type: ToastType, message: string, duration?: number, action?: ToastAction) => {
        const normalizedMessage = message.trim().toLowerCase();
        const key = `${type}:${normalizedMessage}`;
        const now = Date.now();
        const lastShownAt = recentToastMapRef.current.get(key);
        if (lastShownAt && now - lastShownAt < DEDUPE_WINDOW_MS) {
            return;
        }
        // Drop old dedupe entries to keep map small.
        for (const [mapKey, ts] of recentToastMapRef.current.entries()) {
            if (now - ts > DEDUPE_WINDOW_MS * 4) {
                recentToastMapRef.current.delete(mapKey);
            }
        }
        recentToastMapRef.current.set(key, now);
        const id = generateId();
        setToasts((prev) => {
            const next = [...prev, { id, type, message, duration, action }];
            if (next.length <= MAX_ACTIVE_TOASTS) {
                return next;
            }
            return next.slice(next.length - MAX_ACTIVE_TOASTS);
        });
    };
    const dismiss = (id: string) => {
        setToasts((prev) => prev.filter((t) => t.id !== id));
    };
    const dismissAll = () => {
        setToasts([]);
        recentToastMapRef.current.clear();
    };
    const success = (msg: string, duration?: number, action?: ToastAction) => showToast('success', msg, duration, action);
    const error = (msg: string, duration?: number, action?: ToastAction) => showToast('error', msg, duration, action);
    const info = (msg: string, duration?: number, action?: ToastAction) => showToast('info', msg, duration, action);
    const warning = (msg: string, duration?: number, action?: ToastAction) => showToast('warning', msg, duration, action);
    return {
        get toasts() {
            return toasts();
        },
        success,
        error,
        info,
        warning,
        dismiss,
        dismissAll,
    };
}
