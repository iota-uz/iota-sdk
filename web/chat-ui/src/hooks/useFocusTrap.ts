type RefObject<T> = {
    current: T | null;
};
/** @jsxImportSource solid-js */
import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * Resolve the element that actually holds focus, piercing shadow boundaries.
 *
 * Inside a shadow tree `document.activeElement` collapses to the shadow *host*
 * (the custom element), so it never equals an inner control. Reading
 * `activeElement` from the container's own root — and then descending through
 * any nested shadow roots — yields the real focused element. Without this the
 * Tab wrap-around comparisons below never match when BiChat is embedded in its
 * shadow-DOM host, silently disabling the trap.
 */
function deepActiveElement(root: DocumentOrShadowRoot): HTMLElement | null {
    let active = root.activeElement;
    while (active?.shadowRoot?.activeElement) {
        active = active.shadowRoot.activeElement;
    }
    return active instanceof HTMLElement ? active : null;
}
/**
 * Hook to trap focus within a container (for modals, sidebars)
 * Ensures Tab and Shift+Tab cycle through focusable elements only
 *
 * @param containerRef - React ref to the container element
 * @param isActive - Whether the focus trap is currently active
 * @param restoreFocusOnDeactivate - Element to restore focus to when deactivated
 *
 * @example
 * const modalRef = useRef<HTMLDivElement>(null)
 * useFocusTrap(modalRef, isOpen)
 */
export function useFocusTrap(containerRef: RefObject<HTMLElement | null>, isActive: () => boolean, restoreFocusOnDeactivate?: HTMLElement | null) {
    createEffect(on(() => [containerRef, isActive(), restoreFocusOnDeactivate], () => {
        const cleanup = untrack(() => {
            if (!isActive() || !containerRef.current) {
                return;
            }
            const container = containerRef.current;
            const root = container.getRootNode() as unknown as DocumentOrShadowRoot;
            const previouslyFocused = deepActiveElement(root);
            // Get all focusable elements
            const getFocusableElements = (): HTMLElement[] => {
                const selector = [
                    'button:not([disabled])',
                    '[href]',
                    'input:not([disabled])',
                    'select:not([disabled])',
                    'textarea:not([disabled])',
                    '[tabindex]:not([tabindex="-1"])',
                ].join(', ');
                return Array.from(container.querySelectorAll(selector)) as HTMLElement[];
            };
            // Move focus inside on activation. Honour an explicit [data-autofocus]
            // target (e.g. a confirm/cancel button) and otherwise fall back to the
            // first focusable. Captured previouslyFocused above first, so focus can be
            // restored to the trigger on deactivate.
            const focusableElements = getFocusableElements();
            const initialTarget = container.querySelector<HTMLElement>('[data-autofocus]') ??
                focusableElements[0];
            initialTarget?.focus();
            // Handle Tab key to cycle focus
            const handleTabKey = (e: KeyboardEvent) => {
                if (e.key !== 'Tab') {
                    return;
                }
                const focusableElements = getFocusableElements();
                if (focusableElements.length === 0) {
                    return;
                }
                const firstElement = focusableElements[0];
                const lastElement = focusableElements[focusableElements.length - 1];
                const active = deepActiveElement(root);
                if (e.shiftKey) {
                    // Shift+Tab: cycle backwards
                    if (active === firstElement) {
                        e.preventDefault();
                        lastElement.focus();
                    }
                }
                else {
                    // Tab: cycle forwards
                    if (active === lastElement) {
                        e.preventDefault();
                        firstElement.focus();
                    }
                }
            };
            container.addEventListener('keydown', handleTabKey);
            // Cleanup and restore focus
            return () => {
                container.removeEventListener('keydown', handleTabKey);
                // Restore focus to previously focused element or custom element
                if (restoreFocusOnDeactivate) {
                    restoreFocusOnDeactivate.focus();
                }
                else if (previouslyFocused) {
                    previouslyFocused.focus();
                }
            };
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
}
