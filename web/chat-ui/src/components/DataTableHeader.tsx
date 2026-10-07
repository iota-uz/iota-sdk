import { cssLength } from "../utils/cssLength";
import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
import { CaretUp, CaretDown, DotsThreeVertical } from '../icons';
import { Menu, MenuButton, MenuItem, MenuItems } from './Menu';
import type { ColumnMeta, SortState } from '../hooks/useDataTable';
import { useTranslation } from '../hooks/useTranslation';
const MIN_COLUMN_WIDTH = 40;
interface DataTableHeaderProps {
    tableId: string;
    columns: ColumnMeta[];
    sort: SortState | null;
    onToggleSort: (columnIndex: number) => void;
    onColumnResize: (columnIndex: number, width: number) => void;
    onToggleVisibility: (columnIndex: number) => void;
    onSendMessage?: (content: string) => void;
    sendDisabled?: boolean;
    showRowNumbers?: boolean;
}
export const DataTableHeader = function DataTableHeader(solidProps1: DataTableHeaderProps) {
    return (<thead class="sticky top-0 z-10 bg-gray-100 dark:bg-gray-800">
      <tr class="border-b border-gray-200 dark:border-gray-700">
        {solidProps1.showRowNumbers && (<th scope="col" class="sticky left-0 z-20 w-10 bg-gray-100 px-2 py-2 text-right text-xs font-medium text-gray-400 dark:bg-gray-800 dark:text-gray-500 select-none">
            #
          </th>)}
        {solidProps1.columns.map((col, colIdx) => (<HeaderCell column={col} sort={solidProps1.sort} onToggleSort={solidProps1.onToggleSort} onColumnResize={solidProps1.onColumnResize} onToggleVisibility={solidProps1.onToggleVisibility} onSendMessage={solidProps1.onSendMessage} sendDisabled={solidProps1.sendDisabled} isFirstColumn={colIdx === 0 && !!solidProps1.showRowNumbers}/>))}
      </tr>
    </thead>);
};
interface HeaderCellProps {
    column: ColumnMeta;
    sort: SortState | null;
    onToggleSort: (columnIndex: number) => void;
    onColumnResize: (columnIndex: number, width: number) => void;
    onToggleVisibility: (columnIndex: number) => void;
    onSendMessage?: (content: string) => void;
    sendDisabled?: boolean;
    isFirstColumn?: boolean;
}
const HeaderCell = function HeaderCell(solidProps2: HeaderCellProps) {
    const solidState3 = useTranslation();
    const thRef = { current: null } as {
        current: HTMLTableCellElement | null;
    };
    const resizeTeardownRef = { current: null } as {
        current: ((() => void) | null) | null;
    };
    const isActive = solidProps2.sort?.columnIndex === solidProps2.column.index;
    const direction = isActive ? solidProps2.sort?.direction ?? null : null;
    onMount(() => {
        const cleanup = untrack(() => {
            return () => {
                resizeTeardownRef.current?.();
                resizeTeardownRef.current = null;
            };
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    });
    const handleResizeStart = (e: PointerEvent) => {
        e.preventDefault();
        e.stopPropagation();
        const th = thRef.current;
        if (!th) {
            return;
        }
        const startX = e.clientX;
        const startWidth = th.offsetWidth;
        const pointerId = e.pointerId;
        const target = e.currentTarget as Element;
        const onMove = (moveEvent: PointerEvent) => {
            const delta = moveEvent.clientX - startX;
            const newWidth = Math.max(MIN_COLUMN_WIDTH, startWidth + delta);
            solidProps2.onColumnResize?.(solidProps2.column.index, newWidth);
        };
        const teardown = () => {
            document.removeEventListener('pointermove', onMove);
            document.removeEventListener('pointerup', onUp);
            document.removeEventListener('pointercancel', onUp);
            resizeTeardownRef.current = null;
            try {
                target.releasePointerCapture(pointerId);
            }
            catch {
                // ignore
            }
        };
        const onUp = () => teardown();
        resizeTeardownRef.current = teardown;
        document.addEventListener('pointermove', onMove);
        document.addEventListener('pointerup', onUp);
        document.addEventListener('pointercancel', onUp);
        try {
            target.setPointerCapture(pointerId);
        }
        catch {
            // ignore
        }
    };
    return (<th ref={element => thRef.current = element} scope="col" aria-sort={direction === 'asc' ? 'ascending' : direction === 'desc' ? 'descending' : 'none'} class={`group relative select-none whitespace-nowrap border-r border-transparent px-3 py-2 text-left font-semibold text-gray-700 dark:text-gray-200 ${solidProps2.isFirstColumn ? 'sticky left-[2.5rem] z-20 bg-gray-100 dark:bg-gray-800' : ''}`} style={solidProps2.column.width ? { "width": cssLength(solidProps2.column.width), "min-width": cssLength(solidProps2.column.width) } : undefined}>
      <div class="flex items-center gap-1">
        <button type="button" class="flex cursor-pointer items-center gap-1 hover:text-gray-900 dark:hover:text-white" onClick={() => solidProps2.onToggleSort?.(solidProps2.column.index)} aria-label={solidState3.t('BiChat.DataTable.SortBy', { column: solidProps2.column.header })}>
          <span>{solidProps2.column.header}</span>
          {direction === 'asc' && <CaretUp size={12} weight="bold"/>}
          {direction === 'desc' && <CaretDown size={12} weight="bold"/>}
          {!direction && (<span class="w-3 opacity-0 transition-opacity group-hover:opacity-40">
              <CaretUp size={12}/>
            </span>)}
        </button>

        {solidProps2.onSendMessage && (<Menu>
            <MenuButton className="ml-auto rounded p-0.5 opacity-0 transition-opacity hover:bg-gray-200 group-hover:opacity-100 dark:hover:bg-gray-700" aria-label={solidState3.t('BiChat.DataTable.ColumnActions')}>
              <DotsThreeVertical size={14}/>
            </MenuButton>
            <MenuItems anchor="bottom end" className="z-50 min-w-[180px] rounded-lg border border-gray-200 bg-white py-1 text-sm shadow-lg dark:border-gray-700 dark:bg-gray-800 [--anchor-gap:4px]">
              <MenuItem>
                {(solidProps4) => (<button type="button" class={`w-full px-3 py-1.5 text-left ${solidProps4.focus ? 'bg-gray-100 dark:bg-gray-700' : ''} text-gray-700 dark:text-gray-200`} disabled={solidProps2.sendDisabled} onClick={() => solidProps2.onSendMessage?.(solidState3.t('BiChat.DataTable.Prompt.SummarizeColumn', { column: solidProps2.column.header }))}>
                    {solidState3.t('BiChat.DataTable.SummarizeColumn')}
                  </button>)}
              </MenuItem>
              <MenuItem>
                {(solidProps5) => (<button type="button" class={`w-full px-3 py-1.5 text-left ${solidProps5.focus ? 'bg-gray-100 dark:bg-gray-700' : ''} text-gray-700 dark:text-gray-200`} disabled={solidProps2.sendDisabled} onClick={() => solidProps2.onSendMessage?.(solidState3.t('BiChat.DataTable.Prompt.UniqueValues', { column: solidProps2.column.header }))}>
                    {solidState3.t('BiChat.DataTable.UniqueValues')}
                  </button>)}
              </MenuItem>
              <div class="my-1 border-t border-gray-200 dark:border-gray-700"/>
              <MenuItem>
                {(solidProps6) => (<button type="button" class={`w-full px-3 py-1.5 text-left ${solidProps6.focus ? 'bg-gray-100 dark:bg-gray-700' : ''} text-gray-700 dark:text-gray-200`} onClick={() => solidProps2.onToggleVisibility?.(solidProps2.column.index)}>
                    {solidState3.t('BiChat.DataTable.HideColumn')}
                  </button>)}
              </MenuItem>
            </MenuItems>
          </Menu>)}
      </div>

      {/* Resize handle */}
      <div role="separator" aria-orientation="vertical" aria-label={solidState3.t('BiChat.DataTable.ResizeColumn')} class="absolute right-0 top-0 h-full w-1 cursor-col-resize opacity-0 transition-opacity hover:bg-blue-400 hover:opacity-100 group-hover:opacity-40" onPointerDown={handleResizeStart}/>
    </th>);
};
