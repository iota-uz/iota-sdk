import { createReducedMotion } from '../hooks/createReducedMotion';
import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
import { X, Plus, Archive, CaretLineLeft, CaretLineRight, Gear, Users, List, ChatCircle, MagnifyingGlass } from '../icons';
import { Menu, MenuButton, MenuItem, MenuItems } from './Menu';
import SessionSkeleton from './SessionSkeleton';
import SessionItem from './SessionItem';
import ConfirmModal from './ConfirmModal';
import SearchInput from './SearchInput';
import DateGroupHeader from './DateGroupHeader';
import { EmptyState } from './EmptyState';
import LoadingSpinner from './LoadingSpinner';
import AllChatsList from './AllChatsList';
import { useTranslation } from '../hooks/useTranslation';
import { useToast } from '../hooks/useToast';
import { groupSessionsByDate } from '../utils/sessionGrouping';
import { staggerContainerVariants, buttonVariants, } from '../animations/variants';
import type { Session, ChatDataSource } from '../types';
import { ToastContainer } from './ToastContainer';
import { toErrorDisplay, type RPCErrorDisplay } from '../utils/errorDisplay';
/** Matches sidebar width transition (duration-300) + small buffer for focus-after-expand */
const SIDEBAR_EXPAND_FOCUS_DELAY_MS = 350;
function ErrorAlert(solidProps1: {
    error: RPCErrorDisplay;
}) {
    const amber = solidProps1.error.isPermissionDenied;
    return (<div class={`mx-2 mt-4 p-3 border rounded-xl cursor-default ${amber
            ? 'bg-amber-50 dark:bg-amber-900/20 border-amber-200 dark:border-amber-800'
            : 'bg-red-50 dark:bg-red-900/20 border-red-200 dark:border-red-800'}`}>
      <p class={`text-xs font-medium ${amber
            ? 'text-amber-700 dark:text-amber-300'
            : 'text-red-600 dark:text-red-400'}`}>
        {solidProps1.error.title}
      </p>
      {solidProps1.error.description && (<p class={`mt-1 text-xs ${amber
                ? 'text-amber-600 dark:text-amber-400'
                : 'text-red-500 dark:text-red-300'}`}>
          {solidProps1.error.description}
        </p>)}
    </div>);
}
const COLLAPSE_STORAGE_KEY = 'bichat-sidebar-collapsed';
const SESSION_RECONCILE_POLL_INTERVAL_MS = 2000;
const SESSION_RECONCILE_MAX_POLLS = 30;
const ACTIVE_SESSION_MISS_MAX_RETRIES = 8;
const ACTIVE_SESSION_MISS_RETRY_DELAY_MS = 1000;
const MAX_COLLAPSED_INDICATORS = 5;
function useSidebarCollapse() {
    const [isCollapsed, setIsCollapsed] = createSignal((() => {
        try {
            return localStorage.getItem(COLLAPSE_STORAGE_KEY) === 'true';
        }
        catch {
            return false;
        }
    })());
    const isCollapsedRef = createMemo(() => ({ current: isCollapsed() }));
    createEffect(on(() => [isCollapsed()], () => {
        const cleanup = untrack(() => {
            isCollapsedRef().current = isCollapsed();
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    const toggle = () => {
        setIsCollapsed((prev) => {
            const next = !prev;
            try {
                localStorage.setItem(COLLAPSE_STORAGE_KEY, String(next));
            }
            catch { /* noop */ }
            return next;
        });
    };
    const expand = () => {
        setIsCollapsed(false);
        try {
            localStorage.setItem(COLLAPSE_STORAGE_KEY, 'false');
        }
        catch { /* noop */ }
    };
    const collapse = () => {
        setIsCollapsed(true);
        try {
            localStorage.setItem(COLLAPSE_STORAGE_KEY, 'true');
        }
        catch { /* noop */ }
    };
    return { get isCollapsed() {
            return isCollapsed();
        }, get isCollapsedRef() {
            return isCollapsedRef();
        }, toggle, expand, collapse };
}
export type ActiveTab = 'my-chats' | 'all-chats';
export interface SidebarProps {
    dataSource: ChatDataSource;
    onSessionSelect: (sessionId: string) => void;
    onNewChat: () => void;
    onArchivedView?: () => void;
    activeSessionId?: string;
    creating?: boolean;
    showAllChatsTab?: boolean;
    isOpen?: boolean;
    onClose?: () => void;
    headerSlot?: JSX.Element;
    footerSlot?: JSX.Element;
    className?: string;
    /** Controlled active tab. When provided, overrides internal state. */
    activeTab?: ActiveTab;
    /** Called when tab changes. Use with activeTab for controlled mode. */
    onTabChange?: (tab: ActiveTab) => void;
}
export default function Sidebar(solidProps2Input: SidebarProps) {
    const solidProps2 = mergeProps({ className: '' } as const, solidProps2Input);
    const solidState3 = useTranslation();
    const toast = useToast();
    const shouldReduceMotion = createReducedMotion();
    const sessionListRef = { current: null } as {
        current: HTMLElement | null;
    };
    const searchContainerRef = { current: null } as {
        current: HTMLDivElement | null;
    };
    const activeSessionMissRetriesRef = { current: {} } as {
        current: Record<string, number>;
    };
    // Collapse state — disabled when used as a mobile drawer (onClose present)
    const solidState4 = useSidebarCollapse();
    const collapsible = createMemo(() => !solidProps2.onClose); // desktop only
    // Click-on-empty-space to toggle (same pattern as SDK sidebar)
    const handleSidebarClick = (e: MouseEvent) => {
        if (!collapsible()) {
            return;
        }
        const interactive = 'a, button, input, textarea, select, summary, label, [role="button"], [role="link"], [contenteditable="true"], [data-no-sidebar-toggle]';
        if ((e.target as HTMLElement).closest(interactive)) {
            return;
        }
        solidState4.toggle();
    };
    // Keyboard shortcut: Cmd+B (toggle)
    createEffect(on(() => [collapsible(), solidState4.toggle], () => {
        const cleanup = untrack(() => {
            if (!collapsible()) {
                return;
            }
            const handleKeyDown = (e: KeyboardEvent) => {
                const isMod = e.metaKey || e.ctrlKey;
                if (isMod && e.key === 'b') {
                    e.preventDefault();
                    solidState4.toggle();
                }
            };
            document.addEventListener('keydown', handleKeyDown);
            return () => document.removeEventListener('keydown', handleKeyDown);
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    // Auto-collapse when artifacts panel expands
    createEffect(on(() => [collapsible(), solidState4.collapse], () => {
        const cleanup = untrack(() => {
            if (!collapsible()) {
                return;
            }
            const handler = (e: Event) => {
                const detail = (e as CustomEvent<{
                    expanded: boolean;
                }>).detail;
                if (detail?.expanded) {
                    solidState4.collapse();
                }
            };
            window.addEventListener('bichat:artifacts-panel-expanded', handler);
            return () => window.removeEventListener('bichat:artifacts-panel-expanded', handler);
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    const showCollapsed = createMemo(() => collapsible() && solidState4.isCollapsed);
    // Allow tooltips to escape sidebar bounds once collapse transition settles
    const [collapsedOverflowVisible, setCollapsedOverflowVisible] = createSignal(false);
    createEffect(on(() => [showCollapsed()], () => {
        const cleanup = untrack(() => {
            if (!showCollapsed()) {
                setCollapsedOverflowVisible(false);
                return;
            }
            const timer = setTimeout(() => setCollapsedOverflowVisible(true), 300);
            return () => clearTimeout(timer);
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    // View state (my chats vs all chats) — controlled or uncontrolled
    const [internalActiveTab, setInternalActiveTab] = createSignal<ActiveTab>('my-chats');
    const activeTab = createMemo(() => solidProps2.activeTab ?? internalActiveTab());
    const handleTabChange = (tab: ActiveTab) => {
        if (solidProps2.activeTab === undefined) {
            setInternalActiveTab(tab);
        }
        solidProps2.onTabChange?.(tab);
    };
    // Search state
    const [searchQuery, setSearchQuery] = createSignal('');
    // Session data
    const [sessions, setSessions] = createSignal<Session[]>([]);
    const [loading, setLoading] = createSignal(true);
    const [loadError, setLoadError] = createSignal<RPCErrorDisplay | null>(null);
    const [actionError, setActionError] = createSignal<RPCErrorDisplay | null>(null);
    const accessDenied = createMemo(() => loadError()?.isPermissionDenied === true);
    // Refresh key — bump to re-fetch sessions
    const [refreshKey, setRefreshKey] = createSignal(0);
    const [reconcilePollToken, setReconcilePollToken] = createSignal(0);
    // Confirm modal state
    const [showConfirm, setShowConfirm] = createSignal(false);
    const [sessionToArchive, setSessionToArchive] = createSignal<string | null>(null);
    // Fetch sessions
    const fetchSessions = async () => {
        try {
            setLoading(true);
            setLoadError(null);
            setActionError(null);
            const result = await solidProps2.dataSource.listSessions({ limit: 50 });
            setSessions(result.sessions);
        }
        catch (err) {
            console.error('Failed to load sessions:', err);
            setLoadError(toErrorDisplay(err, solidState3.t('BiChat.Sidebar.FailedToLoadSessions')));
        }
        finally {
            setLoading(false);
        }
    };
    createEffect(on(() => [fetchSessions, refreshKey()], () => {
        const cleanup = untrack(() => {
            fetchSessions();
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    onMount(() => {
        const cleanup = untrack(() => {
            const handleSessionsUpdated = (event: Event) => {
                setRefreshKey((k) => k + 1);
                const detail = (event as CustomEvent<{
                    reason?: string;
                }>).detail;
                const reason = detail?.reason;
                if (!reason || reason === 'session_created' || reason === 'message_sent' || reason === 'title_regenerate_requested') {
                    setReconcilePollToken((k) => k + 1);
                }
            };
            window.addEventListener('bichat:sessions-updated', handleSessionsUpdated);
            return () => {
                window.removeEventListener('bichat:sessions-updated', handleSessionsUpdated);
            };
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    });
    createEffect(on(() => [solidProps2.activeSessionId], () => {
        const cleanup = untrack(() => {
            activeSessionMissRetriesRef.current = {};
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    createEffect(on(() => [solidProps2.activeSessionId, loading(), sessions()], () => {
        const cleanup = untrack(() => {
            if (!solidProps2.activeSessionId) {
                return;
            }
            if (loading()) {
                return;
            }
            const hasActiveSession = sessions().some((session) => session.id === solidProps2.activeSessionId);
            if (hasActiveSession) {
                delete activeSessionMissRetriesRef.current[solidProps2.activeSessionId];
                return;
            }
            const attempts = activeSessionMissRetriesRef.current[solidProps2.activeSessionId] ?? 0;
            if (attempts >= ACTIVE_SESSION_MISS_MAX_RETRIES) {
                return;
            }
            activeSessionMissRetriesRef.current[solidProps2.activeSessionId] = attempts + 1;
            const timeoutId = window.setTimeout(() => {
                setRefreshKey((k) => k + 1);
                setReconcilePollToken((k) => k + 1);
            }, ACTIVE_SESSION_MISS_RETRY_DELAY_MS);
            return () => window.clearTimeout(timeoutId);
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    // Poll for title updates on sessions with placeholder titles.
    // Use a stable boolean so that updating sessions inside the poll
    // does NOT re-trigger the effect (which would create overlapping intervals).
    const hasPlaceholderTitles = createMemo(() => {
        const newChatLabel = createMemo(() => solidState3.t('BiChat.Chat.NewChat'));
        return (Array.isArray(sessions()) &&
            sessions().some((s) => s && (!s.title || s.title === newChatLabel())));
    });
    createEffect(on(() => [hasPlaceholderTitles(), solidProps2.dataSource, reconcilePollToken()], () => {
        const cleanup = untrack(() => {
            if (!hasPlaceholderTitles() && reconcilePollToken() === 0) {
                return;
            }
            let pollCount = 0;
            const intervalId = setInterval(async () => {
                pollCount++;
                try {
                    const result = await solidProps2.dataSource.listSessions({ limit: 50 });
                    setSessions(result.sessions);
                }
                catch {
                    // ignore poll errors
                }
                if (pollCount >= SESSION_RECONCILE_MAX_POLLS) {
                    clearInterval(intervalId);
                }
            }, SESSION_RECONCILE_POLL_INTERVAL_MS);
            return () => clearInterval(intervalId);
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    const handleArchiveRequest = (sessionId: string) => {
        setSessionToArchive(sessionId);
        setShowConfirm(true);
    };
    const handleUndoArchive = async (sessionId: string) => {
        try {
            await solidProps2.dataSource.unarchiveSession(sessionId);
            setRefreshKey((k) => k + 1);
            window.dispatchEvent(new CustomEvent('bichat:sessions-updated', {
                detail: { reason: 'unarchived', sessionId },
            }));
        }
        catch (undoErr) {
            console.error('Failed to restore session:', undoErr);
            toast.error(solidState3.t('BiChat.Sidebar.FailedToRestoreChat'));
        }
    };
    const confirmArchive = async () => {
        const archivedId = sessionToArchive();
        if (!archivedId) {
            return;
        }
        const wasCurrentSession = solidProps2.activeSessionId === sessionToArchive();
        try {
            await solidProps2.dataSource.archiveSession(archivedId);
            setRefreshKey((k) => k + 1);
            window.dispatchEvent(new CustomEvent('bichat:sessions-updated', {
                detail: { reason: 'archived', sessionId: archivedId },
            }));
            if (wasCurrentSession) {
                solidProps2.onSessionSelect?.('');
            }
            toast.success(solidState3.t('BiChat.Sidebar.ChatArchived'), 8000, {
                label: solidState3.t('BiChat.Common.Undo'),
                onClick: () => handleUndoArchive(archivedId),
            });
        }
        catch (err) {
            console.error('Failed to archive session:', err);
            const display = toErrorDisplay(err, solidState3.t('BiChat.Sidebar.FailedToArchiveChat'));
            setActionError(display);
            toast.error(display.title);
        }
        finally {
            setShowConfirm(false);
            setSessionToArchive(null);
        }
    };
    const handleTogglePin = async (sessionId: string, currentlyPinned: boolean) => {
        try {
            if (currentlyPinned) {
                await solidProps2.dataSource.unpinSession(sessionId);
            }
            else {
                await solidProps2.dataSource.pinSession(sessionId);
            }
            setRefreshKey((k) => k + 1);
        }
        catch (err) {
            console.error('Failed to toggle pin:', err);
            const display = toErrorDisplay(err, solidState3.t('BiChat.Sidebar.FailedToTogglePin'));
            setActionError(display);
            toast.error(display.title);
        }
    };
    const handleRenameSession = async (sessionId: string, newTitle: string) => {
        try {
            await solidProps2.dataSource.renameSession(sessionId, newTitle);
            toast.success(solidState3.t('BiChat.Sidebar.ChatRenamedSuccessfully'));
            setRefreshKey((k) => k + 1);
        }
        catch (err) {
            console.error('Failed to update session title:', err);
            const display = toErrorDisplay(err, solidState3.t('BiChat.Sidebar.FailedToRenameChat'));
            setActionError(display);
            toast.error(display.title);
        }
    };
    const handleRegenerateTitle = async (sessionId: string) => {
        try {
            await solidProps2.dataSource.regenerateSessionTitle(sessionId);
            toast.success(solidState3.t('BiChat.Sidebar.TitleRegenerated'));
            window.dispatchEvent(new CustomEvent('bichat:sessions-updated', {
                detail: { reason: 'title_regenerate_requested', sessionId },
            }));
        }
        catch (err) {
            console.error('Failed to regenerate title:', err);
            const display = toErrorDisplay(err, solidState3.t('BiChat.Sidebar.FailedToRegenerateTitle'));
            setActionError(display);
            toast.error(display.title);
        }
    };
    // Stable callbacks for SessionItem — accept session ID as parameter
    const handleSessionSelect = (sessionId: string) => solidProps2.onSessionSelect?.(sessionId);
    const handleSessionArchive = (sessionId: string) => handleArchiveRequest(sessionId);
    const handleSessionPin = (sessionId: string, pinned: boolean) => handleTogglePin(sessionId, pinned);
    const handleSessionRename = (sessionId: string, newTitle: string) => handleRenameSession(sessionId, newTitle);
    const handleSessionRegenerateTitle = (sessionId: string) => handleRegenerateTitle(sessionId);
    // Filter sessions by search
    const filteredSessions = createMemo(() => {
        if (!searchQuery().trim()) {
            return sessions();
        }
        const q = createMemo(() => searchQuery().toLowerCase());
        return sessions().filter((s) => s.title?.toLowerCase().includes(q()));
    });
    // Separate pinned and unpinned
    const pinnedSessions = createMemo(() => filteredSessions().filter((s) => s.pinned));
    const unpinnedSessions = createMemo(() => filteredSessions().filter((s) => !s.pinned));
    // Group unpinned sessions by date
    const sessionGroups = createMemo(() => {
        const groups = createMemo(() => groupSessionsByDate(unpinnedSessions(), solidState3.t));
        return Array.isArray(groups())
            ? groups().map((group) => ({
                ...group,
                sessions: Array.isArray(group.sessions) ? group.sessions : [],
            }))
            : [];
    });
    // Keep collapsed indicators in the same visual order as expanded list.
    const orderedUnpinnedSessions = createMemo(() => sessionGroups().flatMap((group) => group.sessions));
    // Collapsed sidebar indicators — pinned first, then most recent
    const collapsedIndicators = createMemo(() => {
        const seen = new Set<string>();
        const result: Session[] = [];
        for (const s of [...pinnedSessions(), ...orderedUnpinnedSessions()]) {
            if (seen.has(s.id)) {
                continue;
            }
            seen.add(s.id);
            result.push(s);
            if (result.length >= MAX_COLLAPSED_INDICATORS) {
                break;
            }
        }
        return result;
    });
    const totalSessionCount = createMemo(() => filteredSessions().length);
    const overflowCount = createMemo(() => Math.max(0, totalSessionCount() - collapsedIndicators().length));
    // Keyboard navigation for session list (WAI-ARIA listbox pattern)
    const handleSessionListKeyDown = (e: KeyboardEvent) => {
        const nav = sessionListRef.current;
        if (!nav) {
            return;
        }
        const focusableItems = Array.from(nav.querySelectorAll<HTMLElement>('button[data-session-item]'));
        if (focusableItems.length === 0) {
            return;
        }
        const currentIndex = focusableItems.indexOf(document.activeElement as HTMLElement);
        let nextIndex: number | null = null;
        switch (e.key) {
            case 'ArrowDown':
                e.preventDefault();
                nextIndex =
                    currentIndex < 0 ? 0 : Math.min(currentIndex + 1, focusableItems.length - 1);
                break;
            case 'ArrowUp':
                e.preventDefault();
                nextIndex =
                    currentIndex < 0
                        ? focusableItems.length - 1
                        : Math.max(currentIndex - 1, 0);
                break;
            case 'Home':
                e.preventDefault();
                nextIndex = 0;
                break;
            case 'End':
                e.preventDefault();
                nextIndex = focusableItems.length - 1;
                break;
        }
        if (nextIndex !== null) {
            focusableItems[nextIndex].focus();
        }
    };
    return (<>
      <aside onClick={collapsible() ? handleSidebarClick : undefined} class={`relative bg-surface-300 dark:bg-gray-900 border-r border-gray-200 dark:border-gray-700 h-full min-h-0 flex flex-col ${collapsedOverflowVisible() ? 'overflow-visible' : 'overflow-hidden'} transition-[width] duration-300 ease-in-out ${showCollapsed() ? 'w-16 cursor-e-resize'
            : collapsible() ? 'w-64 cursor-w-resize'
                : 'w-64'} ${solidProps2.className}`} style={{ "will-change": 'width' }} role="navigation" aria-label={solidState3.t('BiChat.Sidebar.ChatSessions')}>
        {/* Collapsed overlay — absolutely positioned, fades in after width shrinks */}
        {collapsible() && (<div class={`absolute inset-x-0 top-0 bottom-0 z-10 flex flex-col items-center pt-3 gap-3 transition-opacity ${showCollapsed() ? 'opacity-100 duration-150 delay-100'
                : 'opacity-0 pointer-events-none duration-100'}`}>
            <div class="group/tooltip relative">
              <button onClick={(e) => {
                e.stopPropagation();
                solidProps2.onNewChat?.();
            }} disabled={solidProps2.creating || loading() || accessDenied()} class="w-10 h-10 rounded-lg bg-primary-600 hover:bg-primary-700 active:bg-primary-800 text-white shadow-sm flex items-center justify-center disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer transition-colors focus-visible:ring-2 focus-visible:ring-primary-400/50" title={solidState3.t('BiChat.Chat.NewChat')} aria-label={solidState3.t('BiChat.Sidebar.CreateNewChat')}>
                {solidProps2.creating ? (<div class="w-4 h-4 border-2 border-white/50 border-t-transparent rounded-full animate-spin"/>) : (<Plus size={18} weight="bold"/>)}
              </button>
              <span class="pointer-events-none absolute left-full ml-2 top-1/2 -translate-y-1/2 rounded-md bg-gray-900 dark:bg-gray-100 px-2 py-1 text-xs font-medium text-white dark:text-gray-900 opacity-0 group-hover/tooltip:opacity-100 transition-opacity whitespace-nowrap shadow-lg">
                {solidState3.t('BiChat.Chat.NewChat')}
              </span>
            </div>

            {/* Search button — expands sidebar and focuses search */}
            <div class="group/search relative">
              <button onClick={(e) => {
                e.stopPropagation();
                solidState4.expand();
                setTimeout(() => {
                    const input = searchContainerRef.current?.querySelector('input');
                    input?.focus();
                }, SIDEBAR_EXPAND_FOCUS_DELAY_MS);
            }} class="w-10 h-10 rounded-lg bg-gray-100 dark:bg-gray-800 hover:bg-gray-200 dark:hover:bg-gray-700 text-gray-500 dark:text-gray-400 flex items-center justify-center cursor-pointer transition-colors focus-visible:ring-2 focus-visible:ring-primary-400/50 focus-visible:outline-none" aria-label={solidState3.t('BiChat.Sidebar.SearchChats')}>
                <MagnifyingGlass size={18}/>
              </button>
              <span class="pointer-events-none absolute left-full ml-2 top-1/2 -translate-y-1/2 rounded-md bg-gray-900 dark:bg-gray-100 px-2 py-1 text-xs font-medium text-white dark:text-gray-900 opacity-0 group-hover/search:opacity-100 group-focus-within/search:opacity-100 transition-opacity whitespace-nowrap shadow-lg">
                {solidState3.t('BiChat.Sidebar.SearchChats')}
              </span>
            </div>

            {/* Session indicators */}
            {collapsedIndicators().length > 0 && (<div class="flex flex-col items-center gap-1.5 mt-1">
                {collapsedIndicators().map((session) => {
                    const isActive = createMemo(() => session.id === solidProps2.activeSessionId);
                    const initial = session.title?.trim()?.[0]?.toUpperCase();
                    return (<div class="group/indicator relative">
                      <button onClick={(e) => {
                            e.stopPropagation();
                            solidProps2.onSessionSelect?.(session.id);
                        }} class={`w-9 h-9 rounded-full flex items-center justify-center text-xs font-medium cursor-pointer transition-colors focus-visible:ring-2 focus-visible:ring-primary-400/50 focus-visible:outline-none ${isActive() ? 'bg-primary-100 dark:bg-primary-900/40 text-primary-700 dark:text-primary-300 ring-2 ring-primary-500 dark:ring-primary-400'
                            : 'bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-700'}`} aria-label={session.title || solidState3.t('BiChat.Chat.NewChat')}>
                        {initial ? (initial) : (<ChatCircle size={16} weight="fill"/>)}
                      </button>
                      <div class="pointer-events-none absolute left-full ml-2 top-1/2 -translate-y-1/2 w-52 rounded-lg bg-gray-900 dark:bg-gray-100 px-3 py-2 text-xs font-medium text-white dark:text-gray-900 opacity-0 group-hover/indicator:opacity-100 group-focus-within/indicator:opacity-100 transition-opacity shadow-lg break-words">
                        {session.title || solidState3.t('BiChat.Chat.NewChat')}
                      </div>
                    </div>);
                })}
                {overflowCount() > 0 && (<div class="group/overflow relative">
                    <button onClick={(e) => {
                        e.stopPropagation();
                        solidState4.toggle();
                    }} class="w-9 h-9 rounded-full flex items-center justify-center text-[10px] font-semibold bg-gray-50 dark:bg-gray-800/60 text-gray-500 dark:text-gray-500 hover:bg-gray-200 dark:hover:bg-gray-700 cursor-pointer transition-colors focus-visible:ring-2 focus-visible:ring-primary-400/50 focus-visible:outline-none" aria-label={solidState3.t('BiChat.Sidebar.MoreChats', { count: overflowCount() })}>
                      +{overflowCount()}
                    </button>
                    <span class="pointer-events-none absolute left-full ml-2 top-1/2 -translate-y-1/2 rounded-md bg-gray-900 dark:bg-gray-100 px-2 py-1 text-xs font-medium text-white dark:text-gray-900 opacity-0 group-hover/overflow:opacity-100 group-focus-within/overflow:opacity-100 transition-opacity whitespace-nowrap shadow-lg">
                      {solidState3.t('BiChat.Sidebar.ChatSessions')}
                    </span>
                  </div>)}
              </div>)}
          </div>)}

        {/* Expanded content — fades out before width shrinks */}
        <div class={`flex flex-col flex-1 min-h-0 w-64 shrink-0 transition-opacity ${showCollapsed() ? 'opacity-0 pointer-events-none duration-100'
            : collapsible() ? 'opacity-100 duration-150 delay-[200ms]'
                : ''}`}>
          {/* Header — only rendered when there is content to show */}
          {(solidProps2.headerSlot || solidProps2.onClose) && (<div class="p-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
              {solidProps2.headerSlot}
              {solidProps2.onClose && (<button onClick={solidProps2.onClose} class="cursor-pointer p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 transition-smooth text-gray-600 dark:text-gray-400" title={solidState3.t('BiChat.Sidebar.CloseSidebar')} aria-label={solidState3.t('BiChat.Sidebar.CloseSidebar')}>
                  <X size={20} className="w-5 h-5"/>
                </button>)}
            </div>)}

          {/* Conditional content based on active view */}
          {activeTab() === 'all-chats' && solidProps2.showAllChatsTab ? (<AllChatsList dataSource={solidProps2.dataSource} onSessionSelect={solidProps2.onSessionSelect} activeSessionId={solidProps2.activeSessionId}/>) : (<>
              {/* Search Input */}
              <div ref={element => searchContainerRef.current = element} class="mt-3 px-4">
                <SearchInput value={searchQuery()} onChange={setSearchQuery} placeholder={solidState3.t('BiChat.Sidebar.SearchChats')}/>
              </div>

              {/* New Chat Button */}
              <div class="p-4">
                <button onClick={(e) => {
                e.stopPropagation();
                solidProps2.onNewChat?.();
            }} disabled={solidProps2.creating || loading() || accessDenied()} class="cursor-pointer w-full px-4 py-2.5 bg-primary-600 dark:bg-primary-700 text-white rounded-lg hover:bg-primary-700 hover:-translate-y-0.5 active:bg-primary-800 transition-all duration-150 font-medium shadow-sm disabled:opacity-40 disabled:cursor-not-allowed flex items-center justify-center gap-2 focus-visible:ring-2 focus-visible:ring-primary-400/50 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-gray-900" title={accessDenied() ? solidState3.t('BiChat.Sidebar.MissingPermission') : solidState3.t('BiChat.Chat.NewChat')} aria-label={solidState3.t('BiChat.Sidebar.CreateNewChat')}>
                  {solidProps2.creating ? (<>
                      <LoadingSpinner variant="spinner" size="sm"/>
                      <span>{solidState3.t('BiChat.Common.Creating')}</span>
                    </>) : (<>
                      <Plus size={16} weight="bold"/>
                      <span>{solidState3.t('BiChat.Chat.NewChat')}</span>
                    </>)}
                </button>
              </div>

              {/* Chat History */}
              <nav ref={element => sessionListRef.current = element} class="flex-1 overflow-y-auto px-2 pb-4 hide-scrollbar" aria-label={solidState3.t('BiChat.Sidebar.ChatHistory')} onKeyDown={handleSessionListKeyDown}>
                {loading() && sessions().length === 0 ? (<SessionSkeleton count={5}/>) : (<>
                    {/* Pinned Sessions */}
                    {pinnedSessions().length > 0 && (<div class="mb-4">
                        <DateGroupHeader groupName={solidState3.t('BiChat.Common.Pinned')} count={pinnedSessions().length}/>
                        <div class="space-y-1 mt-2" role="list" aria-label={solidState3.t('BiChat.Sidebar.PinnedChats')}>
                          {pinnedSessions().map((session) => {
                        const canWrite = session.access?.canWrite ?? true;
                        return (<SessionItem session={session} isActive={session.id === solidProps2.activeSessionId} onSelect={() => handleSessionSelect(session.id)} onArchive={canWrite ? () => handleSessionArchive(session.id) : undefined} onPin={canWrite ? () => handleSessionPin(session.id, session.pinned) : undefined} onRename={canWrite ? (newTitle) => handleSessionRename(session.id, newTitle) : undefined} onRegenerateTitle={canWrite ? () => handleSessionRegenerateTitle(session.id) : undefined}/>);
                    })}
                        </div>
                        <div class="border-b border-gray-200 dark:border-gray-700 my-3"/>
                      </div>)}

                    {/* Grouped Sessions by Date */}
                    {sessionGroups().map((group) => (<div class="mb-4">
                        <DateGroupHeader groupName={group.name} count={group.sessions.length}/>
                        <div class="space-y-1 mt-2" role="list" aria-label={`${group.name} chats`}>
                          {group.sessions.map((session) => {
                        const canWrite = session.access?.canWrite ?? true;
                        return (<SessionItem session={session} isActive={session.id === solidProps2.activeSessionId} onSelect={() => handleSessionSelect(session.id)} onArchive={canWrite ? () => handleSessionArchive(session.id) : undefined} onPin={canWrite ? () => handleSessionPin(session.id, session.pinned) : undefined} onRename={canWrite ? (newTitle) => handleSessionRename(session.id, newTitle) : undefined} onRegenerateTitle={canWrite ? () => handleSessionRegenerateTitle(session.id) : undefined}/>);
                    })}
                        </div>
                      </div>))}

                    {/* Empty State */}
                    {filteredSessions().length === 0 && !loading() && (<EmptyState title={searchQuery() ? solidState3.t('BiChat.Sidebar.NoChatsFound', { query: searchQuery() })
                        : solidState3.t('BiChat.Sidebar.NoChatsYet')} description={searchQuery() ? undefined
                        : solidState3.t('BiChat.Sidebar.CreateOneToGetStarted')} action={searchQuery() ? (<button onClick={() => setSearchQuery('')} class="cursor-pointer text-sm text-primary-600 dark:text-primary-400 hover:underline">
                              {solidState3.t('BiChat.Common.Clear')}
                            </button>) : undefined}/>)}
                  </>)}

                {loadError() && <ErrorAlert error={loadError()!}/>}
                {actionError() && !loadError() && <ErrorAlert error={actionError()!}/>}
              </nav>

              {/* Footer slot */}
              {solidProps2.footerSlot}
            </>)}

          {/* Footer — settings (left) + collapse toggle (right) */}
          {collapsible() && (<div class="mt-auto border-t border-gray-100 dark:border-gray-800/80 px-4 py-3 flex items-center justify-between">
              {/* Gear settings menu */}
              {(solidProps2.onArchivedView || solidProps2.showAllChatsTab) ? (<Menu>
                  <MenuButton onClick={(e: MouseEvent) => {
                    e.stopPropagation();
                }} disabled={loading() || accessDenied()} className="flex items-center justify-center rounded-lg text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 p-2" aria-label={solidState3.t('BiChat.Sidebar.Settings')} title={solidState3.t('BiChat.Sidebar.Settings')}>
                    <Gear size={20}/>
                  </MenuButton>
                  <MenuItems anchor="top start" className="w-48 bg-white dark:bg-gray-900 rounded-xl shadow-lg border border-gray-200 dark:border-gray-700 z-[var(--bichat-z-dropdown,10)] [--anchor-gap:8px] mb-1 p-1.5">
                    {solidProps2.onArchivedView && (<MenuItem>
                        {(solidProps5) => (<button onClick={(e) => {
                            e.preventDefault();
                            e.stopPropagation();
                            solidProps2.onArchivedView?.();
                            solidProps5.close();
                        }} class={`cursor-pointer flex w-full items-center gap-2.5 rounded-lg px-2.5 py-1.5 text-[13px] text-gray-600 dark:text-gray-300 transition-colors ${solidProps5.focus ? 'bg-gray-100 dark:bg-gray-800/70' : ''}`} aria-label={solidState3.t('BiChat.Sidebar.ArchivedChats')}>
                            <Archive size={16} className="text-gray-400 dark:text-gray-500"/>
                            {solidState3.t('BiChat.Sidebar.ArchivedChats')}
                          </button>)}
                      </MenuItem>)}
                    {solidProps2.showAllChatsTab && activeTab() !== 'all-chats' && (<MenuItem>
                        {(solidProps6) => (<button onClick={(e) => {
                            e.preventDefault();
                            e.stopPropagation();
                            handleTabChange('all-chats');
                            solidProps6.close();
                        }} class={`cursor-pointer flex w-full items-center gap-2.5 rounded-lg px-2.5 py-1.5 text-[13px] text-gray-600 dark:text-gray-300 transition-colors ${solidProps6.focus ? 'bg-gray-100 dark:bg-gray-800/70' : ''}`} aria-label={solidState3.t('BiChat.Sidebar.AllChats')}>
                            <Users size={16} className="text-gray-400 dark:text-gray-500"/>
                            {solidState3.t('BiChat.Sidebar.AllChats')}
                          </button>)}
                      </MenuItem>)}
                    {solidProps2.showAllChatsTab && activeTab() === 'all-chats' && (<MenuItem>
                        {(solidProps7) => (<button onClick={(e) => {
                            e.preventDefault();
                            e.stopPropagation();
                            handleTabChange('my-chats');
                            solidProps7.close();
                        }} class={`cursor-pointer flex w-full items-center gap-2.5 rounded-lg px-2.5 py-1.5 text-[13px] text-gray-600 dark:text-gray-300 transition-colors ${solidProps7.focus ? 'bg-gray-100 dark:bg-gray-800/70' : ''}`} aria-label={solidState3.t('BiChat.Sidebar.MyChats')}>
                            <List size={16} className="text-gray-400 dark:text-gray-500"/>
                            {solidState3.t('BiChat.Sidebar.MyChats')}
                          </button>)}
                      </MenuItem>)}
                  </MenuItems>
                </Menu>) : (<div />)}

              {/* Collapse toggle */}
              <button onClick={(e) => {
                e.stopPropagation();
                solidState4.toggle();
            }} class="flex items-center gap-2 rounded-lg px-3 py-2 text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50" title={solidState3.t('BiChat.Sidebar.CollapseSidebar')} aria-label={solidState3.t('BiChat.Sidebar.CollapseSidebar')}>
                <CaretLineLeft size={16}/>
                <span class="text-xs font-medium">{solidState3.t('BiChat.Sidebar.Collapse')}</span>
              </button>
            </div>)}
        </div>

        {/* Collapsed footer — expand button */}
        {collapsible() && showCollapsed() && (<div class="absolute bottom-0 inset-x-0 z-10 border-t border-gray-100 dark:border-gray-800/80 py-3 flex justify-center">
            <div class="group/tooltip relative">
              <button onClick={(e) => {
                e.stopPropagation();
                solidState4.toggle();
            }} class="w-10 h-10 flex items-center justify-center rounded-lg text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50" title={solidState3.t('BiChat.Sidebar.ExpandSidebar')} aria-label={solidState3.t('BiChat.Sidebar.ExpandSidebar')}>
                <CaretLineRight size={16}/>
              </button>
              <span class="pointer-events-none absolute left-full ml-2 top-1/2 -translate-y-1/2 rounded-md bg-gray-900 dark:bg-gray-100 px-2 py-1 text-xs font-medium text-white dark:text-gray-900 opacity-0 group-hover/tooltip:opacity-100 transition-opacity whitespace-nowrap shadow-lg">
                {solidState3.t('BiChat.Sidebar.Expand')}
              </span>
            </div>
          </div>)}
      </aside>

      {/* Confirm Archive Modal */}
      <ConfirmModal isOpen={showConfirm()} title={solidState3.t('BiChat.Sidebar.ArchiveChatSession')} message={solidState3.t('BiChat.Sidebar.ArchiveChatMessage')} confirmText={solidState3.t('BiChat.Sidebar.ArchiveButton')} cancelText={solidState3.t('BiChat.Common.Cancel')} isDanger={true} onConfirm={confirmArchive} onCancel={() => {
            setShowConfirm(false);
            setSessionToArchive(null);
        }}/>

      {/* Toast notifications */}
      <ToastContainer toasts={toast.toasts} onDismiss={toast.dismiss}/>
    </>);
}
