import { Portal as HostPortal } from '@iota-uz/sdk/solid';
import { createHorizontalSwipe, type SwipeInfo } from '../hooks/createHorizontalSwipe';
import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
import { List } from '../icons';
import SkipLink from './SkipLink';
import { useSidebarState, type UseSidebarStateReturn } from '../hooks/useSidebarState';
import { useKeyboardShortcuts, type ShortcutConfig } from '../hooks/useKeyboardShortcuts';
import { useTranslation } from '../hooks/useTranslation';
export interface SidebarDrawerProps {
    onClose?: () => void;
}
export interface BiChatLayoutProps {
    /** Render function for the sidebar. Receives `{ onClose }` when in mobile drawer mode. */
    renderSidebar: (props: SidebarDrawerProps) => JSX.Element;
    /** Main page content */
    children: JSX.Element;
    /** Callback for Cmd+N keyboard shortcut */
    onNewChat?: () => void;
    /** Key for AnimatePresence page transitions (e.g. location.pathname). Omit to disable transitions. */
    routeKey?: string;
    /** Custom class for the root container */
    className?: string;
}
export function BiChatLayout(solidProps1Input: BiChatLayoutProps) {
    const solidProps1 = mergeProps({ className: '' } as const, solidProps1Input);
    const solidState2 = useTranslation();
    const solidState3 = useSidebarState();
    const drawerRef = { current: null } as {
        current: HTMLDivElement | null;
    };
    const menuButtonRef = { current: null } as {
        current: HTMLButtonElement | null;
    };
    // Focus trap on mobile drawer
    // Cmd+N keyboard shortcut
    const shortcuts = createMemo<ShortcutConfig[]>(() => {
        if (!solidProps1.onNewChat) {
            return [];
        }
        return [{ key: 'n', ctrl: true, callback: solidProps1.onNewChat, description: 'New chat' }];
    });
    useKeyboardShortcuts(shortcuts());
    const drawerGesture = createHorizontalSwipe({ left: -120, onEnd: (event, info) => handleDrawerDragEnd(event, info) });
    // Swipe-left to close drawer
    const handleDrawerDragEnd = (_: MouseEvent | TouchEvent | PointerEvent, info: SwipeInfo) => {
        if (info.offset.x < -80) {
            solidState3.closeMobile();
        }
    };
    return (<div class={`relative flex flex-1 w-full h-full min-h-0 overflow-hidden ${solidProps1.className}`}>
      <SkipLink />

      {/* Sidebar — desktop */}
      <div class="hidden md:block">
        {solidProps1.renderSidebar({})}
      </div>

      {/* Sidebar — mobile drawer */}
      
        {solidState3.isMobile && solidState3.isMobileOpen && (<HostPortal surface="drawer" label={solidState2.t('BiChat.Sidebar.ChatSessions')} onEscape={solidState3.closeMobile}>
            {/* Backdrop */}
            <div class="fixed inset-0 z-[var(--bichat-z-overlay,30)] bg-black/40" onClick={solidState3.closeMobile} aria-hidden="true"/>
            {/* Drawer */}
            <div class="fixed inset-y-0 left-0 z-[var(--bichat-z-modal,40)] w-[18rem] max-w-[85vw] shadow-2xl" {...drawerGesture.handlers} style={{ "transform": `translateX(${drawerGesture.offset()}px)`, "touch-action": "pan-y" }} onClick={(e) => e.stopPropagation()}>
              <div ref={element => drawerRef.current = element} class="h-full bg-white dark:bg-gray-900">
                {solidProps1.renderSidebar({ onClose: solidState3.closeMobile })}
              </div>
            </div>
          </HostPortal>)}
      

      {/* Main Content */}
      <main id="main-content" class="relative flex-1 min-w-0 flex flex-col min-h-0 overflow-hidden">
        {/* Mobile menu button */}
        {solidState3.isMobile && !solidState3.isMobileOpen && (<button ref={element => menuButtonRef.current = element} onClick={solidState3.openMobile} class="md:hidden absolute top-3 left-3 z-[var(--bichat-z-sticky,20)] w-10 h-10 rounded-xl bg-white/90 dark:bg-gray-900/90 text-gray-700 dark:text-gray-200 border border-gray-200/60 dark:border-gray-800/80 shadow-sm flex items-center justify-center hover:bg-white dark:hover:bg-gray-900 transition-colors cursor-pointer focus-visible:ring-2 focus-visible:ring-primary-400/50" aria-label={solidState2.t('BiChat.Layout.OpenSidebar')} title={solidState2.t('BiChat.Layout.OpenSidebar')}>
            <List size={20} weight="bold"/>
          </button>)}
        <div class="flex flex-1 min-w-0 min-h-0">{solidProps1.children}</div>
      </main>
    </div>);
}
// Re-export useSidebarState for consumers who want custom layout control
export { useSidebarState, type UseSidebarStateReturn };
