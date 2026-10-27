import { createExternalSnapshot } from '../context/externalSnapshot';
import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * Toast Component
 * Individual toast notification with auto-dismiss, progress bar, and accessibility.
 * Uses native CSS transitions for enter and leave animations.
 */
import { Transition } from './Transition';
import { CheckCircle, XCircle, Info, Warning, X } from '../icons';
import type { ToastType, ToastAction } from '../hooks/useToast';
import { useTranslation } from '../hooks/useTranslation';
export interface ToastProps {
    id: string;
    type: ToastType;
    message: string;
    duration?: number;
    onDismiss: (id: string) => void;
    /** Label for dismiss button (defaults to "Dismiss") */
    dismissLabel?: string;
    /** Optional action button rendered in the toast */
    action?: ToastAction;
}
const typeConfig: Record<ToastType, {
    accent: string;
    bg: string;
    icon: string;
    progress: string;
    iconEl: typeof CheckCircle;
}> = {
    success: {
        accent: 'text-emerald-600 dark:text-emerald-400',
        bg: 'bg-emerald-50 dark:bg-emerald-950/40 border-emerald-200/80 dark:border-emerald-800/50',
        icon: 'bg-emerald-100 dark:bg-emerald-900/50',
        progress: 'bg-emerald-500 dark:bg-emerald-400',
        iconEl: CheckCircle,
    },
    error: {
        accent: 'text-red-600 dark:text-red-400',
        bg: 'bg-red-50 dark:bg-red-950/40 border-red-200/80 dark:border-red-800/50',
        icon: 'bg-red-100 dark:bg-red-900/50',
        progress: 'bg-red-500 dark:bg-red-400',
        iconEl: XCircle,
    },
    info: {
        accent: 'text-blue-600 dark:text-blue-400',
        bg: 'bg-blue-50 dark:bg-blue-950/40 border-blue-200/80 dark:border-blue-800/50',
        icon: 'bg-blue-100 dark:bg-blue-900/50',
        progress: 'bg-blue-500 dark:bg-blue-400',
        iconEl: Info,
    },
    warning: {
        accent: 'text-amber-600 dark:text-amber-400',
        bg: 'bg-amber-50 dark:bg-amber-950/40 border-amber-200/80 dark:border-amber-800/50',
        icon: 'bg-amber-100 dark:bg-amber-900/50',
        progress: 'bg-amber-500 dark:bg-amber-400',
        iconEl: Warning,
    },
};
const reducedMotionQuery = '(prefers-reduced-motion: reduce)';
let reducedMotionMql: MediaQueryList | null = null;
function getReducedMotionMql(): MediaQueryList | null {
    if (typeof window === 'undefined') {
        return null;
    }
    if (!reducedMotionMql) {
        reducedMotionMql = window.matchMedia(reducedMotionQuery);
    }
    return reducedMotionMql;
}
function subscribeReducedMotion(callback: () => void): () => void {
    const mql = getReducedMotionMql();
    if (!mql) {
        return () => { };
    }
    mql.addEventListener('change', callback);
    return () => mql.removeEventListener('change', callback);
}
function getReducedMotionSnapshot(): boolean {
    const mql = getReducedMotionMql();
    return mql ? mql.matches : false;
}
function getReducedMotionServerSnapshot(): boolean {
    return false;
}
export function Toast(solidProps1Input: ToastProps) {
    const solidProps1 = mergeProps({ duration: 5000 } as const, solidProps1Input);
    const solidState2 = useTranslation();
    const resolvedDismissLabel = createMemo(() => solidProps1.dismissLabel ?? solidState2.t('BiChat.Chat.DismissNotification'));
    const config = createMemo(() => typeConfig[solidProps1.type]);
    const Icon = config().iconEl;
    const [show, setShow] = createSignal(false);
    const [paused, setPaused] = createSignal(false);
    const remainingRef = createMemo(() => ({ current: solidProps1.duration }));
    const startRef = { current: Date.now() };
    let dismissTimer: ReturnType<typeof setTimeout> | undefined;
    onCleanup(() => clearTimeout(dismissTimer));
    const prefersReducedMotion = createExternalSnapshot(subscribeReducedMotion, getReducedMotionSnapshot);
    // Trigger enter transition on mount
    onMount(() => {
        const cleanup = untrack(() => {
            const frame = requestAnimationFrame(() => setShow(true));
            return () => cancelAnimationFrame(frame);
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    });
    // Auto-dismiss with pause/resume on hover
    createEffect(on(() => [solidProps1.id, paused(), solidProps1.onDismiss], () => {
        const cleanup = untrack(() => {
            if (paused()) {
                return;
            }
            startRef.current = Date.now();
            const timer = setTimeout(() => {
                setShow(false);
                // Wait for leave transition before removing from DOM
                clearTimeout(dismissTimer);
                dismissTimer = setTimeout(() => solidProps1.onDismiss?.(solidProps1.id), 200);
            }, remainingRef().current);
            return () => {
                // Capture how much time remains when pausing
                const elapsed = Date.now() - startRef.current;
                remainingRef().current = Math.max(0, remainingRef().current - elapsed);
                clearTimeout(timer);
            };
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    const handleDismiss = () => {
        setShow(false);
        clearTimeout(dismissTimer);
                dismissTimer = setTimeout(() => solidProps1.onDismiss?.(solidProps1.id), 200);
    };
    const ariaLive = createMemo(() => solidProps1.type === 'error' ? 'assertive' : 'polite');
    const role = createMemo(() => solidProps1.type === 'error' || solidProps1.type === 'warning' ? 'alert' : 'status');
    return (<Transition show={show()} enter="transition duration-200 ease-out" enterFrom="-translate-y-2 opacity-0 scale-95" enterTo="translate-y-0 opacity-100 scale-100" leave="transition duration-150 ease-in" leaveFrom="translate-y-0 opacity-100 scale-100" leaveTo="-translate-y-2 opacity-0 scale-95">
      <div class={`relative flex items-start gap-3 rounded-xl border px-4 py-3 shadow-lg shadow-black/5 dark:shadow-black/20 backdrop-blur-sm min-w-[320px] max-w-[420px] overflow-hidden ${config().bg}`} role={role()} aria-live={ariaLive()} aria-atomic="true" onMouseEnter={() => setPaused(true)} onMouseLeave={() => setPaused(false)}>
        {/* Icon */}
        <div class={`mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-lg ${config().icon}`}>
          <Icon size={16} className={config().accent} weight="fill"/>
        </div>

        {/* Message */}
        <p class="flex-1 pt-0.5 text-sm font-medium leading-snug text-gray-800 dark:text-gray-100">
          {solidProps1.message}
        </p>

        {/* Action button */}
        {solidProps1.action && (<button type="button" onClick={() => {
                solidProps1.action?.onClick();
                handleDismiss();
            }} class={`shrink-0 text-sm font-semibold underline-offset-2 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-primary-500/50 cursor-pointer ${config().accent}`}>
            {solidProps1.action.label}
          </button>)}

        {/* Dismiss */}
        <button onClick={handleDismiss} class="mt-0.5 -mr-1 cursor-pointer shrink-0 rounded-lg p-1 text-gray-400 hover:text-gray-600 hover:bg-gray-200/60 dark:text-gray-500 dark:hover:text-gray-300 dark:hover:bg-gray-700/60 transition-colors duration-100" aria-label={resolvedDismissLabel()}>
          <X size={14} weight="bold"/>
        </button>

        {/* Progress bar */}
        <div class="absolute inset-x-0 bottom-0 h-0.5 bg-black/5 dark:bg-white/5">
          <div class={`h-full ${config().progress} origin-left motion-reduce:animate-none`} style={prefersReducedMotion() ? {}
            : {
                "animation": `bichat-toast-progress ${solidProps1.duration}ms linear forwards`,
                "animation-play-state": paused() ? 'paused' : 'running'
            }}/>
        </div>
      </div>
    </Transition>);
}
export default Toast;
