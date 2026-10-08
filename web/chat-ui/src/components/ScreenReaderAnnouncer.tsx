import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
interface ScreenReaderAnnouncerProps {
    message: string;
    politeness?: 'polite' | 'assertive';
    clearAfter?: number;
}
/**
 * Screen reader announcer component for live region updates
 * Uses ARIA live regions to announce dynamic content changes
 *
 * @param message - The message to announce
 * @param politeness - 'polite' (wait for pause) or 'assertive' (immediate)
 * @param clearAfter - Optional milliseconds to clear message after announcement
 *
 * @example
 * <ScreenReaderAnnouncer
 *   message="New message received"
 *   politeness="polite"
 * />
 */
export default function ScreenReaderAnnouncer(solidProps1Input: ScreenReaderAnnouncerProps) {
    const solidProps1 = mergeProps({ politeness: 'polite' } as const, solidProps1Input);
    const [announcement, setAnnouncement] = createSignal(solidProps1.message);
    createEffect(on(() => [solidProps1.message, solidProps1.clearAfter], () => {
        const cleanup = untrack(() => {
            setAnnouncement(solidProps1.message);
            if (solidProps1.clearAfter && solidProps1.message) {
                const timer = setTimeout(() => {
                    setAnnouncement('');
                }, solidProps1.clearAfter);
                return () => clearTimeout(timer);
            }
            return undefined;
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    return (<div role="status" aria-live={solidProps1.politeness} aria-atomic="true" class="sr-only">
      {announcement()}
    </div>);
}
