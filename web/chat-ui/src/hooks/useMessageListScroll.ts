/** @jsxImportSource solid-js */
import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * useMessageListScroll — scroll management for the MessageList.
 *
 * Owns: auto-scroll, initial-session scroll, scroll-to-bottom button,
 * unread-count tracking, and the End-key shortcut.
 */
import { useKeyboardShortcuts, type ShortcutConfig } from './useKeyboardShortcuts';
const NEAR_BOTTOM_PX = 150;
export interface MessageListScrollOptions {
    /** Current session id — triggers initial scroll on change. */
    currentSessionId: string | undefined;
    /** True while session data is loading. */
    fetching: boolean;
    /** Number of conversation turns (triggers auto-scroll). */
    turnsLength: number;
    /** Current streaming content (controls instant vs smooth scroll). */
    streamingContent: string;
}
export interface MessageListScrollReturn {
    containerRef: {
        current: HTMLDivElement | null;
    };
    messagesEndRef: {
        current: HTMLDivElement | null;
    };
    showScrollButton: boolean;
    unreadCount: number;
    handleScrollToBottom: () => void;
}
export function useMessageListScroll(solidProps1: MessageListScrollOptions): MessageListScrollReturn {
    const containerRef = { current: null } as {
        current: HTMLDivElement | null;
    };
    const messagesEndRef = { current: null } as {
        current: HTMLDivElement | null;
    };
    const initialScrollSessionRef = { current: undefined } as {
        current: (string | undefined);
    };
    const prevTurnsLengthRef = { current: solidProps1.turnsLength };
    const isAutoScrollRef = { current: true };
    const [showScrollButton, setShowScrollButton] = createSignal(false);
    const [unreadCount, setUnreadCount] = createSignal(0);
    // -- Core scroll helper ---------------------------------------------------
    const scrollToBottom = (behavior: ScrollBehavior = 'smooth') => {
        const container = containerRef.current;
        if (container) {
            container.scrollTo({ top: container.scrollHeight, behavior });
            return;
        }
        messagesEndRef.current?.scrollIntoView({ behavior });
    };
    // -- Auto-scroll on new turns / streaming ---------------------------------
    createEffect(on(() => [solidProps1.turnsLength, solidProps1.streamingContent, scrollToBottom], () => {
        const cleanup = untrack(() => {
            if (!isAutoScrollRef.current) {
                return;
            }
            scrollToBottom(solidProps1.streamingContent ? 'auto' : 'smooth');
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    // -- Initial scroll when opening a session --------------------------------
    createEffect(on(() => [solidProps1.currentSessionId, solidProps1.fetching, solidProps1.turnsLength, scrollToBottom], () => {
        const cleanup = untrack(() => {
            if (solidProps1.fetching || !solidProps1.currentSessionId || solidProps1.currentSessionId === 'new') {
                return;
            }
            if (initialScrollSessionRef.current === solidProps1.currentSessionId) {
                return;
            }
            const runInitialScroll = () => {
                scrollToBottom('auto');
                setShowScrollButton(false);
                isAutoScrollRef.current = true;
            };
            requestAnimationFrame(() => requestAnimationFrame(runInitialScroll));
            const t1 = setTimeout(runInitialScroll, 80);
            const t2 = setTimeout(runInitialScroll, 200);
            const t3 = setTimeout(runInitialScroll, 400);
            initialScrollSessionRef.current = solidProps1.currentSessionId;
            return () => { clearTimeout(t1); clearTimeout(t2); clearTimeout(t3); };
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    // -- Scroll detection — button visibility + auto-scroll flag --------------
    onMount(() => {
        const cleanup = untrack(() => {
            const container = containerRef.current;
            if (!container) {
                return;
            }
            const onScroll = () => {
                const { scrollTop, scrollHeight, clientHeight } = container;
                const nearBottom = scrollHeight - scrollTop - clientHeight < NEAR_BOTTOM_PX;
                isAutoScrollRef.current = nearBottom;
                setShowScrollButton(!nearBottom);
                if (nearBottom) {
                    setUnreadCount(0);
                }
            };
            container.addEventListener('scroll', onScroll);
            return () => container.removeEventListener('scroll', onScroll);
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    });
    // -- Unread tracking when scrolled up -------------------------------------
    createEffect(on(() => [solidProps1.turnsLength, showScrollButton()], () => {
        const cleanup = untrack(() => {
            const prev = prevTurnsLengthRef.current;
            prevTurnsLengthRef.current = solidProps1.turnsLength;
            if (solidProps1.turnsLength > prev && showScrollButton()) {
                setUnreadCount(c => c + (solidProps1.turnsLength - prev));
            }
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    // -- End-key shortcut -----------------------------------------------------
    const shortcuts = createMemo<ShortcutConfig[]>(() => [{
            key: 'End',
            callback: () => { scrollToBottom('smooth'); setUnreadCount(0); },
            description: 'Scroll to bottom',
        }]);
    useKeyboardShortcuts(shortcuts());
    // -- Public callback for the button ---------------------------------------
    const handleScrollToBottom = () => {
        scrollToBottom('smooth');
        setUnreadCount(0);
    };
    return { containerRef, messagesEndRef, get showScrollButton() {
            return showScrollButton();
        }, get unreadCount() {
            return unreadCount();
        }, handleScrollToBottom };
}
