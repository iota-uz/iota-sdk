import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * TabbedChartGroup — wraps multiple ChartCards behind a tab bar.
 *
 * When an assistant turn produces 2+ charts, this component replaces the
 * default vertical stack with a compact tabbed card that occupies 1x space.
 *
 * Charts are rendered once and never remount — the wrapper toggles between
 * inline and fullscreen CSS so ApexCharts state is preserved across all
 * transitions.
 */
import { X } from '../icons';
import type { ChartData } from '../types';
import type { ChartCardHost } from './ChartCard';
import { useTranslation } from '../hooks/useTranslation';
import { TabBar } from './TabBar';
import { ChartCard } from './ChartCard';
export interface TabbedChartGroupProps {
    charts: ChartData[];
}
const INLINE_CLASS = 'w-full min-w-0 rounded-xl border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-900/40 overflow-hidden';
const FULLSCREEN_CLASS = 'fixed inset-4 flex flex-col overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-2xl dark:border-gray-700 dark:bg-gray-900';
export const TabbedChartGroup = function TabbedChartGroup(solidProps1: TabbedChartGroupProps) {
    const solidState2 = useTranslation();
    const [activeTabId, setActiveTabId] = createSignal('chart-0');
    const [isFullscreen, setIsFullscreen] = createSignal(false);
    const containerRef = { current: null } as {
        current: HTMLElement | null;
    };
    const fullscreenTriggerRef = { current: null } as {
        current: (HTMLElement | null) | null;
    };
    const toggleFullscreen = () => {
        setIsFullscreen((v) => {
            if (!v) {
                fullscreenTriggerRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
            }
            return !v;
        });
    };
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
    // Restore focus to trigger when exiting fullscreen
    createEffect(on(() => [isFullscreen()], () => {
        const cleanup = untrack(() => {
            if (!isFullscreen() && fullscreenTriggerRef.current) {
                const el = fullscreenTriggerRef.current;
                fullscreenTriggerRef.current = null;
                if (typeof el.focus === 'function') {
                    el.focus();
                }
            }
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    const tabs = createMemo(() => solidProps1.charts.map((chart, i) => ({
        id: `chart-${i}`,
        label: chart.title || `${solidState2.t('BiChat.Chart.Title')} ${i + 1}`,
    })));
    const host = createMemo<ChartCardHost>(() => ({ get isFullscreen() {
            return isFullscreen();
        } }));
    // Guard against stale activeTabId
    const resolvedActiveId = createMemo(() => tabs().some((tab) => tab.id === activeTabId())
        ? activeTabId() : tabs()[0]?.id ?? '');
    return <Show when={!(solidProps1.charts.length === 0)}>{_visible => {
            return (<>
      {/* Backdrop — only when fullscreen */}
      {isFullscreen() && (<div class="fixed inset-0 bg-black/60 backdrop-blur-sm" style={{ "z-index": 99998 }} onClick={toggleFullscreen} aria-hidden/>)}

      <section ref={element => containerRef.current = element} tabIndex={isFullscreen() ? -1 : undefined} class={isFullscreen() ? FULLSCREEN_CLASS : INLINE_CLASS} style={isFullscreen() ? { "z-index": 99999 } : undefined}>
        {/* Close button — fullscreen only */}
        {isFullscreen() && (<button type="button" onClick={toggleFullscreen} class="absolute right-3 top-3 z-10 cursor-pointer rounded-lg p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-gray-800 dark:hover:text-gray-300" aria-label={solidState2.t('BiChat.DataTable.Collapse')}>
            <X size={18} weight="bold"/>
          </button>)}

        <TabBar tabs={tabs()} activeTab={resolvedActiveId()} onTabChange={setActiveTabId} compact={isFullscreen()}/>

        {solidProps1.charts.map((chart, i) => {
                    const tabId = `chart-${i}`;
                    const isActive = createMemo(() => tabId === resolvedActiveId());
                    return (<div id={`${tabId}-panel`} role="tabpanel" aria-labelledby={tabId} hidden={!isActive()} class={isActive() && isFullscreen() ? 'flex-1 flex flex-col min-h-0' : isActive() ? undefined : 'hidden'}>
            <ChartCard chartData={chart} host={host()}/>
          </div>);
                })}
      </section>
    </>);
        }}</Show>;
};
