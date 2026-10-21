import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
interface TabBarProps {
    tabs: Array<{
        id: string;
        label: string;
    }>;
    activeTab: string;
    onTabChange: (tabId: string) => void;
    /** Tighter padding for space-constrained contexts like fullscreen overlays. */
    compact?: boolean;
}
function TabBar(solidProps1Input: TabBarProps) {
    const solidProps1 = mergeProps({ compact: false } as const, solidProps1Input);
    const instanceId = createUniqueId();
    const tablistRef = { current: null } as {
        current: HTMLDivElement | null;
    };
    const handleKeyDown = (e: KeyboardEvent) => {
        const currentIndex = solidProps1.tabs.findIndex((tab) => tab.id === solidProps1.activeTab);
        if (currentIndex < 0) {
            return;
        }
        let nextIndex: number | null = null;
        switch (e.key) {
            case 'ArrowRight':
                e.preventDefault();
                nextIndex = (currentIndex + 1) % solidProps1.tabs.length;
                break;
            case 'ArrowLeft':
                e.preventDefault();
                nextIndex = (currentIndex - 1 + solidProps1.tabs.length) % solidProps1.tabs.length;
                break;
            case 'Home':
                e.preventDefault();
                nextIndex = 0;
                break;
            case 'End':
                e.preventDefault();
                nextIndex = solidProps1.tabs.length - 1;
                break;
        }
        if (nextIndex !== null) {
            solidProps1.onTabChange?.(solidProps1.tabs[nextIndex].id);
            // Focus the newly activated tab button
            const tablist = tablistRef.current;
            if (tablist) {
                const buttons = tablist.querySelectorAll<HTMLElement>('[role="tab"]');
                buttons[nextIndex]?.focus();
            }
        }
    };
    return <Show when={!(solidProps1.tabs.length === 0)}>{_visible => {
            return (<div ref={element => tablistRef.current = element} class={`flex justify-center gap-1 border-b border-gray-200 dark:border-gray-700 ${solidProps1.compact ? 'px-3 pt-2 pb-1' : 'px-4 pt-4 pb-2'}`} role="tablist" onKeyDown={handleKeyDown}>
      {solidProps1.tabs.map((tab) => (<TabButton id={tab.id} label={tab.label} isActive={solidProps1.activeTab === tab.id} onClick={() => solidProps1.onTabChange?.(tab.id)} layoutId={instanceId + '-tab'} compact={solidProps1.compact}/>))}
    </div>);
        }}</Show>;
}
interface TabButtonProps {
    id: string;
    label: string;
    isActive: boolean;
    onClick: () => void;
    layoutId: string;
    compact?: boolean;
}
function TabButton(solidProps2: TabButtonProps) {
    return (<button id={solidProps2.id} role="tab" aria-selected={solidProps2.isActive} aria-controls={`${solidProps2.id}-panel`} tabIndex={solidProps2.isActive ? 0 : -1} onClick={solidProps2.onClick} class={`
        cursor-pointer relative rounded-t-lg font-medium transition-smooth focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50
        ${solidProps2.compact ? 'px-3 py-1.5 text-xs' : 'px-4 py-2 text-sm'}
        ${solidProps2.isActive ? 'text-primary-700 dark:text-primary-400'
            : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200'}
      `}>
      {solidProps2.label}

      {/* Active indicator */}
      {solidProps2.isActive && (<div class="absolute bottom-0 left-0 right-0 h-0.5 bg-primary-600 dark:bg-primary-500"/>)}
    </button>);
}
const MemoizedTabBar = TabBar;
MemoizedTabBar; /* Solid components are named by their declarations. */
export { MemoizedTabBar as TabBar };
export default MemoizedTabBar;
