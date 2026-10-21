import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
import type { ColumnMeta, ColumnStats } from '../hooks/useDataTable';
import { useTranslation } from '../hooks/useTranslation';
interface DataTableStatsBarProps {
    columns: ColumnMeta[];
    stats: Map<number, ColumnStats>;
}
function formatStat(value: number): string {
    return new Intl.NumberFormat(undefined, { maximumFractionDigits: 2 }).format(value);
}
export const DataTableStatsBar = function DataTableStatsBar(solidProps1: DataTableStatsBarProps) {
    const solidState2 = useTranslation();
    return <Show when={!(solidProps1.stats.size === 0)}>{_visible => {
            return (<div class="border-t border-gray-200 bg-gray-50 px-3 py-2 dark:border-gray-700 dark:bg-gray-900/60">
      <div class="flex flex-wrap gap-4">
        {solidProps1.columns.map((col) => {
                    const s = solidProps1.stats.get(col.index);
                    if (!s) {
                        return null;
                    }
                    return (<div class="min-w-0">
              <span class="text-xs font-medium text-gray-700 dark:text-gray-200">{col.header}</span>
              <div class="mt-0.5 flex flex-wrap gap-x-3 gap-y-0.5 text-[10px] text-gray-500 dark:text-gray-400">
                <span>{solidState2.t('BiChat.DataTable.StatsSum')}: <b class="font-mono tabular-nums">{formatStat(s.sum)}</b></span>
                <span>{solidState2.t('BiChat.DataTable.StatsAvg')}: <b class="font-mono tabular-nums">{formatStat(s.avg)}</b></span>
                <span>{solidState2.t('BiChat.DataTable.StatsMin')}: <b class="font-mono tabular-nums">{formatStat(s.min)}</b></span>
                <span>{solidState2.t('BiChat.DataTable.StatsMax')}: <b class="font-mono tabular-nums">{formatStat(s.max)}</b></span>
              </div>
            </div>);
                })}
      </div>
    </div>);
        }}</Show>;
};
