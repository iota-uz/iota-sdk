import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
import { ArrowsIn, ArrowsOut, CaretUp, CaretDown, Columns, Copy, X, Check } from '../icons';
import { Popover, PopoverButton, PopoverPanel } from './Menu';
import type { ColumnMeta, SortState } from '../hooks/useDataTable';
import { useTranslation } from '../hooks/useTranslation';
interface DataTableToolbarProps {
    columns: ColumnMeta[];
    searchQuery: string;
    onSearchChange: (query: string) => void;
    onToggleColumnVisibility: (columnIndex: number) => void;
    onResetColumnVisibility: () => void;
    hasHiddenColumns: boolean;
    sort?: SortState | null;
    onClearSort?: () => void;
    onCopyTable?: () => void;
    isFullscreen?: boolean;
    onToggleFullscreen?: () => void;
}
export const DataTableToolbar = function DataTableToolbar(solidProps1: DataTableToolbarProps) {
    const solidState2 = useTranslation();
    const [searchFocused, setSearchFocused] = createSignal(false);
    const [copied, setCopied] = createSignal(false);
    const copyTimerRef = { current: undefined } as {
        current: (ReturnType<typeof setTimeout> | undefined);
    };
    onMount(() => {
        const cleanup = untrack(() => {
            return () => {
                if (copyTimerRef.current !== undefined) {
                    clearTimeout(copyTimerRef.current);
                    copyTimerRef.current = undefined;
                }
            };
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    });
    const handleSearchClear = () => {
        solidProps1.onSearchChange?.('');
    };
    const handleCopy = () => {
        solidProps1.onCopyTable?.();
        clearTimeout(copyTimerRef.current);
        setCopied(true);
        copyTimerRef.current = setTimeout(() => setCopied(false), 2000);
    };
    const hasSearch = solidProps1.searchQuery.length > 0;
    return (<div class="flex flex-wrap items-center gap-2 border-b border-gray-200 px-3 py-2 dark:border-gray-700">
      {/* Search */}
      <div class={`relative flex items-center transition-all ${searchFocused() || hasSearch ? 'w-48' : 'w-32'}`}>
        <input type="text" data-testid="bichat-table-search" value={solidProps1.searchQuery} onInput={(e) => solidProps1.onSearchChange?.(e.target.value)} onFocus={() => setSearchFocused(true)} onBlur={() => setSearchFocused(false)} placeholder={solidState2.t('BiChat.DataTable.Search')} class="w-full rounded-md border border-gray-300 bg-white py-1 pl-2 pr-7 text-xs text-gray-700 placeholder-gray-400 focus:border-blue-400 focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200 dark:placeholder-gray-500" aria-label={solidState2.t('BiChat.DataTable.SearchRows')}/>
        {hasSearch && (<button type="button" onClick={handleSearchClear} class="absolute right-1.5 cursor-pointer text-gray-400 hover:text-gray-600 dark:hover:text-gray-300" aria-label={solidState2.t('BiChat.DataTable.ClearSearch')}>
            <X size={12}/>
          </button>)}
      </div>

      {/* Column visibility */}
      <Popover className="relative">
        <PopoverButton className={`flex cursor-pointer items-center gap-1 rounded-md border px-2 py-1 text-xs transition-colors ${solidProps1.hasHiddenColumns ? 'border-blue-300 bg-blue-50 text-blue-700 dark:border-blue-600 dark:bg-blue-900/20 dark:text-blue-400'
            : 'border-gray-300 text-gray-600 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-400 dark:hover:bg-gray-800'}`} aria-label={solidState2.t('BiChat.DataTable.ToggleColumns')}>
          <Columns size={14}/>
          <span>{solidState2.t('BiChat.DataTable.Columns')}</span>
        </PopoverButton>
        <PopoverPanel anchor="bottom start" className="z-50 mt-1 max-h-72 min-w-[200px] overflow-auto rounded-lg border border-gray-200 bg-white py-1.5 shadow-lg dark:border-gray-700 dark:bg-gray-800">
          {solidProps1.hasHiddenColumns && (<>
              <button type="button" class="w-full px-3 py-1.5 text-left text-xs font-medium text-blue-600 hover:bg-gray-50 dark:text-blue-400 dark:hover:bg-gray-700" onClick={solidProps1.onResetColumnVisibility}>
                {solidState2.t('BiChat.DataTable.ShowAllColumns')}
              </button>
              <div class="my-1 border-t border-gray-200 dark:border-gray-700"/>
            </>)}
          {solidProps1.columns.map((col) => (<button type="button" role="checkbox" aria-checked={col.visible} class="flex w-full cursor-pointer items-center gap-2.5 px-3 py-1.5 text-left text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-700/60" onClick={() => solidProps1.onToggleColumnVisibility?.(col.index)}>
              <span aria-hidden="true" class={`flex h-4 w-4 shrink-0 items-center justify-center rounded transition-colors ${col.visible
                ? 'bg-blue-600 text-white'
                : 'border border-gray-300 bg-white dark:border-gray-500 dark:bg-gray-700'}`}>
                {col.visible && <Check size={11} weight="bold"/>}
              </span>
              <span class="truncate">{col.header}</span>
            </button>))}
        </PopoverPanel>
      </Popover>

      {/* Copy table */}
      {solidProps1.onCopyTable && (<button type="button" onClick={handleCopy} class={`flex cursor-pointer items-center gap-1 rounded-md border px-2 py-1 text-xs transition-colors ${copied() ? 'border-green-300 bg-green-50 text-green-700 dark:border-green-600 dark:bg-green-900/20 dark:text-green-400'
                : 'border-gray-300 text-gray-600 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-400 dark:hover:bg-gray-800'}`} aria-label={solidState2.t('BiChat.DataTable.CopyTable')}>
          {copied() ? <Check size={14} weight="bold"/> : <Copy size={14}/>}
          <span>{copied() ? solidState2.t('BiChat.Message.Copied') : solidState2.t('BiChat.DataTable.Copy')}</span>
        </button>)}

      {/* Expand/collapse */}
      {solidProps1.onToggleFullscreen && (<button type="button" onClick={solidProps1.onToggleFullscreen} class="flex cursor-pointer items-center gap-1 rounded-md border border-gray-300 px-2 py-1 text-xs text-gray-600 transition-colors hover:bg-gray-50 dark:border-gray-600 dark:text-gray-400 dark:hover:bg-gray-800" aria-label={solidProps1.isFullscreen ? solidState2.t('BiChat.DataTable.Collapse') : solidState2.t('BiChat.DataTable.Expand')}>
          {solidProps1.isFullscreen ? <ArrowsIn size={14}/> : <ArrowsOut size={14}/>}
          <span>{solidProps1.isFullscreen ? solidState2.t('BiChat.DataTable.Collapse') : solidState2.t('BiChat.DataTable.Expand')}</span>
        </button>)}

      {/* Sort indicator pill */}
      {solidProps1.sort && solidProps1.onClearSort && (<span class="inline-flex items-center gap-1 rounded-full border border-blue-200 bg-blue-50 px-2 py-0.5 text-xs text-blue-700 dark:border-blue-700 dark:bg-blue-900/20 dark:text-blue-400">
          <span>{solidState2.t('BiChat.DataTable.SortedBy', { column: solidProps1.columns.find((c) => c.index === solidProps1.sort!.columnIndex)?.header ?? '' })}</span>
          {solidProps1.sort.direction === 'asc' ? <CaretUp size={10} weight="bold"/> : <CaretDown size={10} weight="bold"/>}
          <button type="button" onClick={solidProps1.onClearSort} class="ml-0.5 cursor-pointer rounded-full p-0.5 hover:bg-blue-200 dark:hover:bg-blue-800" aria-label={solidState2.t('BiChat.DataTable.ClearSort')}>
            <X size={10}/>
          </button>
        </span>)}
    </div>);
};
