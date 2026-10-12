/** @jsxImportSource solid-js */
import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * useSidebarState — mobile breakpoint detection + drawer open/close state.
 * SSR-safe. Auto-closes drawer when resizing to desktop.
 */
const MOBILE_QUERY = '(max-width: 767px)';
function getIsMobile(): boolean {
    if (typeof window === 'undefined') {
        return false;
    }
    return window.matchMedia(MOBILE_QUERY).matches;
}
export interface UseSidebarStateReturn {
    isMobile: boolean;
    isMobileOpen: boolean;
    openMobile: () => void;
    closeMobile: () => void;
    toggleMobile: () => void;
}
export function useSidebarState(): UseSidebarStateReturn {
    const [isMobile, setIsMobile] = createSignal(getIsMobile());
    const [isMobileOpen, setIsMobileOpen] = createSignal(false);
    onMount(() => {
        const cleanup = untrack(() => {
            if (typeof window === 'undefined') {
                return;
            }
            const mql = window.matchMedia(MOBILE_QUERY);
            const handler = (e: MediaQueryListEvent) => {
                setIsMobile(e.matches);
                if (!e.matches) {
                    setIsMobileOpen(false);
                }
            };
            // Safari < 14 uses addListener; modern browsers use addEventListener
            if (mql.addEventListener) {
                mql.addEventListener('change', handler);
            }
            else if (mql.addListener) {
                mql.addListener(handler);
            }
            return () => {
                if (mql.removeEventListener) {
                    mql.removeEventListener('change', handler);
                }
                else if (mql.removeListener) {
                    mql.removeListener(handler);
                }
            };
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    });
    const openMobile = () => setIsMobileOpen(true);
    const closeMobile = () => setIsMobileOpen(false);
    const toggleMobile = () => setIsMobileOpen((v) => !v);
    return { get isMobile() {
            return isMobile();
        }, get isMobileOpen() {
            return isMobileOpen();
        }, openMobile, closeMobile, toggleMobile };
}
