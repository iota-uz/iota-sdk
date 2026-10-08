/** @jsxImportSource solid-js */
import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * Router adapter for BiChat sidebar and archived list.
 * Consumes a navigate function and location (pathname) and returns
 * activeSessionId plus callbacks for use with SDK Sidebar and ArchivedChatList.
 *
 * Router-agnostic: pass useNavigate()/useLocation() from react-router-dom
 * or equivalent from your router.
 */
export interface UseBichatRouterParams {
    /** Navigate to a path (e.g. from useNavigate()) */
    navigate: (path: string) => void;
    /** Current pathname (e.g. location.pathname from useLocation()) */
    pathname: string;
    /** Optional: close mobile sidebar after navigation (e.g. closeMobile from useSidebarState) */
    onNavigate?: () => void;
}
export interface UseBichatRouterReturn {
    /** Session ID extracted from pathname (e.g. /session/:id -> id) */
    activeSessionId: string | undefined;
    /** Navigate to session or home when sessionId is empty */
    onSessionSelect: (sessionId: string) => void;
    /** Navigate to new chat (home) */
    onNewChat: () => void;
    /** Navigate to archived list */
    onArchivedView: () => void;
    /** Navigate back (e.g. to home) */
    onBack: () => void;
    /** Navigate to all-chats view */
    onAllChatsView: () => void;
    /** Current sidebar tab derived from URL */
    sidebarTab: 'my-chats' | 'all-chats';
    /** Handler to change sidebar tab (navigates to appropriate URL) */
    onSidebarTabChange: (tab: 'my-chats' | 'all-chats') => void;
}
const SESSION_PATH_REGEX = /\/session\/([^/]+)/;
/**
 * Derives BiChat navigation callbacks and activeSessionId from router state.
 * Use with SDK Sidebar (onSessionSelect, onNewChat, onArchivedView, activeSessionId)
 * and ArchivedChatList (onBack, onSessionSelect).
 */
export function useBichatRouter(solidProps1: UseBichatRouterParams): UseBichatRouterReturn {
    const isAllChats = createMemo(() => solidProps1.pathname.startsWith('/all-chats'));
    const activeSessionId = createMemo(() => solidProps1.pathname.match(SESSION_PATH_REGEX)?.[1]);
    const sidebarTab = createMemo<'my-chats' | 'all-chats'>(() => (isAllChats() ? 'all-chats' : 'my-chats'));
    const maybeClose = () => {
        solidProps1.onNavigate?.();
    };
    const onSessionSelect = (sessionId: string) => {
        if (sessionId) {
            const prefix = isAllChats() ? '/all-chats' : '';
            solidProps1.navigate(`${prefix}/session/${sessionId}`);
        }
        else {
            solidProps1.navigate(isAllChats() ? '/all-chats' : '/');
        }
        maybeClose();
    };
    const onNewChat = () => {
        solidProps1.navigate(isAllChats() ? '/all-chats' : '/');
        maybeClose();
    };
    const onArchivedView = () => {
        solidProps1.navigate('/archived');
        maybeClose();
    };
    const onAllChatsView = () => {
        solidProps1.navigate('/all-chats');
        maybeClose();
    };
    const onBack = () => {
        solidProps1.navigate('/');
        maybeClose();
    };
    const onSidebarTabChange = (tab: 'my-chats' | 'all-chats') => {
        if (tab === 'all-chats') {
            solidProps1.navigate('/all-chats');
        }
        else {
            solidProps1.navigate('/');
        }
        maybeClose();
    };
    return {
        get activeSessionId() {
            return activeSessionId();
        },
        onSessionSelect,
        onNewChat,
        onArchivedView,
        onBack,
        onAllChatsView,
        get sidebarTab() {
            return sidebarTab();
        },
        onSidebarTabChange,
    };
}
