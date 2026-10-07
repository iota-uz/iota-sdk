import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * TabbedTableGroup — wraps multiple InteractiveTableCards behind a tab bar.
 *
 * When an assistant turn produces 2+ tables, this component replaces the
 * default vertical stack with a compact tabbed card that occupies 1x space.
 *
 * Tables are rendered once and never remount — the wrapper toggles between
 * inline and fullscreen CSS so useDataTable state (search, sort, page) is
 * preserved across all transitions.
 */
import { X } from '../icons';
import type { RenderTableData } from '../types';
import type { TableCardHost } from './InteractiveTableCard';
import { useTranslation } from '../hooks/useTranslation';
import { TabBar } from './TabBar';
import { InteractiveTableCard } from './InteractiveTableCard';
export interface TabbedTableGroupProps {
    tables: RenderTableData[];
    onSendMessage?: (content: string) => void;
    sendDisabled?: boolean;
}
const INLINE_CLASS = 'w-full min-w-0 rounded-xl border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-900/40 overflow-hidden';
const FULLSCREEN_CLASS = 'fixed inset-4 flex flex-col overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-2xl dark:border-gray-700 dark:bg-gray-900';
export const TabbedTableGroup = function TabbedTableGroup(solidProps1Input: TabbedTableGroupProps) {
    const solidProps1 = mergeProps({ sendDisabled: false } as const, solidProps1Input);
    const solidState2 = useTranslation();
    const [activeTabId, setActiveTabId] = createSignal(solidProps1.tables[0]?.id ?? '');
    const [isFullscreen, setIsFullscreen] = createSignal(false);
    const containerRef = { current: null } as {
        current: HTMLElement | null;
    };
    const toggleFullscreen = () => setIsFullscreen((v) => !v);
    // Escape key + focus management for fullscreen
    createEffect(on(() => [isFullscreen()], () => {
        const cleanup = untrack(() => {
            if (!isFullscreen()) {
                return;
            }
            containerRef.current?.focus();
            const onKeyDown = (e: KeyboardEvent) => {
                if (e.key === 'Escape') {
                    e.stopPropagation();
                    setIsFullscreen(false);
                }
            };
            document.addEventListener('keydown', onKeyDown);
            return () => document.removeEventListener('keydown', onKeyDown);
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    const tabs = createMemo(() => solidProps1.tables.map((table, i) => ({
        id: table.id,
        label: `${table.title || `${solidState2.t('BiChat.Table.QueryResults')} ${i + 1}`} (${Math.max(table.totalRows || 0, table.rows.length)})`,
    })));
    const host = createMemo<TableCardHost>(() => ({ onToggleFullscreen: toggleFullscreen, get isFullscreen() {
            return isFullscreen();
        } }));
    // Guard against empty or stale activeTabId
    const resolvedActiveId = createMemo(() => tabs().some((tab) => tab.id === activeTabId())
        ? activeTabId() : tabs()[0]?.id ?? '');
    return <Show when={!(solidProps1.tables.length === 0)}>{_visible => {
            return (<>
      {/* Backdrop — only when fullscreen */}
      {isFullscreen() && (<div class="fixed inset-0 bg-black/60 backdrop-blur-sm" style={{ "z-index": 99998 }} onClick={toggleFullscreen} aria-hidden/>)}

      <section ref={element => containerRef.current = element} tabIndex={isFullscreen() ? -1 : undefined} class={isFullscreen() ? FULLSCREEN_CLASS : INLINE_CLASS} style={isFullscreen() ? { "z-index": 99999 } : undefined}>
        {/* Close button — fullscreen only */}
        {isFullscreen() && (<button type="button" onClick={toggleFullscreen} class="absolute right-3 top-3 z-10 cursor-pointer rounded-lg p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-gray-800 dark:hover:text-gray-300" aria-label={solidState2.t('BiChat.DataTable.Collapse')}>
            <X size={18} weight="bold"/>
          </button>)}

        <TabBar tabs={tabs()} activeTab={resolvedActiveId()} onTabChange={setActiveTabId} compact={isFullscreen()}/>

        {solidProps1.tables.map((table) => {
                    const isActive = createMemo(() => table.id === resolvedActiveId());
                    return (<div id={`${table.id}-panel`} role="tabpanel" aria-labelledby={table.id} hidden={!isActive()} class={isActive() && isFullscreen() ? 'flex-1 flex flex-col min-h-0' : isActive() ? undefined : 'hidden'}>
            <InteractiveTableCard table={table} onSendMessage={solidProps1.onSendMessage} sendDisabled={solidProps1.sendDisabled} host={host()}/>
          </div>);
                })}
      </section>
    </>);
        }}</Show>;
};
