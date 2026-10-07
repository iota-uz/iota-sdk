import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
import type { ColumnMeta, ColumnStats } from '../hooks/useDataTable';
import { useTranslation } from '../hooks/useTranslation';
interface DataTableFooterProps {
    visibleColumns: ColumnMeta[];
    stats: Map<number, ColumnStats>;
    showRowNumbers?: boolean;
}
function formatStat(value: number): string {
    return new Intl.NumberFormat(undefined, { maximumFractionDigits: 2 }).format(value);
}
interface StatRowProps {
    label: string;
    visibleColumns: ColumnMeta[];
    stats: Map<number, ColumnStats>;
    getValue: (s: ColumnStats) => number;
    showRowNumbers?: boolean;
    odd?: boolean;
}
const StatRow = function StatRow(solidProps1: StatRowProps) {
    const zebra = solidProps1.odd ? 'bg-gray-100 dark:bg-gray-800' : 'bg-gray-50 dark:bg-gray-900';
    return (<tr class={zebra}>
      <td colSpan={solidProps1.showRowNumbers ? 2 : 1} class={`sticky left-0 z-20 px-3 py-1.5 text-left text-xs font-medium text-gray-500 dark:text-gray-400 select-none ${zebra}`}>
        {solidProps1.label}
      </td>
      {solidProps1.visibleColumns.map((col, colIdx) => {
            if (colIdx === 0 && solidProps1.showRowNumbers) {
                return null;
            }
            const s = solidProps1.stats.get(col.index);
            return (<td class={`px-3 py-1.5 text-xs ${col.type === 'number' ? 'text-right' : 'text-left'} text-gray-600 dark:text-gray-300`}>
            {s ? (<span class="font-mono tabular-nums font-medium">
                {formatStat(solidProps1.getValue(s))}
              </span>) : null}
          </td>);
        })}
    </tr>);
};
export const DataTableFooter = function DataTableFooter(solidProps2: DataTableFooterProps) {
    const solidState3 = useTranslation();
    return <Show when={!(solidProps2.stats.size === 0)}>{_visible => {
            return (<tfoot class="sticky bottom-0 z-10 border-t-2 border-gray-300 dark:border-gray-600">
      <StatRow label={solidState3.t('BiChat.DataTable.StatsSum')} visibleColumns={solidProps2.visibleColumns} stats={solidProps2.stats} getValue={(s) => s.sum} showRowNumbers={solidProps2.showRowNumbers}/>
      <StatRow label={solidState3.t('BiChat.DataTable.StatsAvg')} visibleColumns={solidProps2.visibleColumns} stats={solidProps2.stats} getValue={(s) => s.avg} showRowNumbers={solidProps2.showRowNumbers} odd/>
      <StatRow label={solidState3.t('BiChat.DataTable.StatsMin')} visibleColumns={solidProps2.visibleColumns} stats={solidProps2.stats} getValue={(s) => s.min} showRowNumbers={solidProps2.showRowNumbers}/>
      <StatRow label={solidState3.t('BiChat.DataTable.StatsMax')} visibleColumns={solidProps2.visibleColumns} stats={solidProps2.stats} getValue={(s) => s.max} showRowNumbers={solidProps2.showRowNumbers} odd/>
    </tfoot>);
        }}</Show>;
};
