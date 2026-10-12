/** @jsxImportSource solid-js */
import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * useAutoScroll Hook
 * Manages auto-scroll behavior for chat containers
 */
export interface UseAutoScrollOptions {
    /** Threshold in pixels from bottom to consider "at bottom" (default: 100) */
    threshold?: number;
    /** Smooth scroll behavior (default: true) */
    smooth?: boolean;
    /** Callback when scroll position changes */
    onScroll?: (isAtBottom: boolean) => void;
}
export interface UseAutoScrollReturn {
    /** Ref to attach to the scrollable container */
    containerRef: {
        current: HTMLDivElement | null;
    };
    /** Whether the container is scrolled to the bottom */
    isAtBottom: boolean;
    /** Whether auto-scroll should be active */
    shouldAutoScroll: boolean;
    /** Manually scroll to bottom */
    scrollToBottom: (smooth?: boolean) => void;
    /** Enable/disable auto-scroll */
    setAutoScroll: (enabled: boolean) => void;
    /** Handle scroll event (attach to container if not using ref) */
    handleScroll: (e: UIEvent & {
        currentTarget: HTMLDivElement;
    }) => void;
}
/**
 * Hook for managing auto-scroll behavior in chat containers
 *
 * @example
 * ```tsx
 * const scroll = useAutoScroll({ threshold: 50 })
 *
 * // Attach to container
 * <div ref={scroll.containerRef} onScroll={scroll.handleScroll}>
 *   {messages.map(msg => <Message key={msg.id} />)}
 * </div>
 *
 * // Scroll button
 * {!scroll.isAtBottom && (
 *   <button onClick={() => scroll.scrollToBottom()}>
 *     Scroll to bottom
 *   </button>
 * )}
 * ```
 */
export function useAutoScroll(options: UseAutoScrollOptions = {}): UseAutoScrollReturn {
    const { threshold = 100, smooth = true, onScroll } = options;
    const containerRef = { current: null } as {
        current: HTMLDivElement | null;
    };
    const [isAtBottom, setIsAtBottom] = createSignal(true);
    const [shouldAutoScroll, setShouldAutoScroll] = createSignal(true);
    const checkIsAtBottom = (container: HTMLElement): boolean => {
        const { scrollTop, scrollHeight, clientHeight } = container;
        return scrollHeight - scrollTop - clientHeight <= threshold;
    };
    const handleScroll = (e: UIEvent & {
        currentTarget: HTMLDivElement;
    }) => {
        const container = e.currentTarget;
        const atBottom = checkIsAtBottom(container);
        setIsAtBottom(atBottom);
        setShouldAutoScroll(atBottom);
        onScroll?.(atBottom);
    };
    const scrollToBottom = (useSmooth?: boolean) => {
        const container = containerRef.current;
        if (!container) {
            return;
        }
        const shouldSmooth = useSmooth ?? smooth;
        container.scrollTo({
            top: container.scrollHeight,
            behavior: shouldSmooth ? 'smooth' : 'auto',
        });
        setIsAtBottom(true);
        setShouldAutoScroll(true);
    };
    const setAutoScroll = (enabled: boolean) => {
        setShouldAutoScroll(enabled);
        if (enabled) {
            // Scroll to bottom immediately when enabling
            const container = containerRef.current;
            if (container) {
                container.scrollTo({
                    top: container.scrollHeight,
                    behavior: 'auto',
                });
                setIsAtBottom(true);
            }
        }
    };
    // Auto-scroll when content changes (using MutationObserver)
    createEffect(on(() => [shouldAutoScroll()], () => {
        const cleanup = untrack(() => {
            const container = containerRef.current;
            if (!container) {
                return;
            }
            let rafId: number | null = null;
            const observer = new MutationObserver(() => {
                if (shouldAutoScroll()) {
                    if (rafId === null) {
                        rafId = requestAnimationFrame(() => {
                            container.scrollTo({
                                top: container.scrollHeight,
                                behavior: 'instant',
                            });
                            rafId = null;
                        });
                    }
                }
            });
            observer.observe(container, {
                childList: true,
                subtree: true,
                characterData: true,
            });
            return () => {
                observer.disconnect();
                if (rafId !== null) {
                    cancelAnimationFrame(rafId);
                }
            };
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    return {
        containerRef,
        get isAtBottom() {
            return isAtBottom();
        },
        get shouldAutoScroll() {
            return shouldAutoScroll();
        },
        scrollToBottom,
        setAutoScroll,
        handleScroll,
    };
}
