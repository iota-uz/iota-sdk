/** @jsxImportSource solid-js */
import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * Hook to prevent background page scroll while a modal/overlay is open.
 *
 * Locks `document.body` overflow and compensates for the removed scrollbar so
 * the page behind the modal doesn't shift sideways. A module-level reference
 * count keeps the lock stable when several modals overlap (only the first lock
 * stores/applies styles; only the last release restores them) — closing one
 * dialog while another is still open never unlocks prematurely.
 *
 * @param isOpen - Whether the modal is currently open
 *
 * @example
 * useModalLock(isOpen)
 */
// Shared across all hook instances so overlapping locks ref-count correctly.
let lockCount = 0;
let restorePreviousStyles: (() => void) | null = null;
function applyBodyLock(): void {
    const body = document.body;
    const previousOverflow = body.style.overflow;
    const previousPaddingRight = body.style.paddingRight;
    // The scrollbar disappears once overflow is hidden; pad by its width so the
    // layout stays put. Zero when the page has no vertical scrollbar.
    const scrollbarWidth = window.innerWidth - document.documentElement.clientWidth;
    body.style.overflow = 'hidden';
    if (scrollbarWidth > 0) {
        const currentPadding = parseFloat(window.getComputedStyle(body).paddingRight) || 0;
        body.style.paddingRight = `${currentPadding + scrollbarWidth}px`;
    }
    restorePreviousStyles = () => {
        body.style.overflow = previousOverflow;
        body.style.paddingRight = previousPaddingRight;
    };
}
export function useModalLock(isOpen: () => boolean) {
    createEffect(on(() => [isOpen()], () => {
        const cleanup = untrack(() => {
            if (!isOpen()) {
                return;
            }
            if (lockCount === 0) {
                applyBodyLock();
            }
            lockCount += 1;
            return () => {
                lockCount -= 1;
                if (lockCount === 0 && restorePreviousStyles) {
                    restorePreviousStyles();
                    restorePreviousStyles = null;
                }
            };
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
}
