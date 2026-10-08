import { splitProps } from 'solid-js';
import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
import { MagnifyingGlass } from '../icons';
import type { RenderTableData } from '../types';
import { useTranslation } from '../hooks/useTranslation';
import { useDataTable, type DataTableOptions } from '../hooks/useDataTable';
import { TableExportButton } from './TableExportButton';
import { DataTableHeader } from './DataTableHeader';
import { DataTableCell } from './DataTableCell';
import { DataTableToolbar } from './DataTableToolbar';
import { DataTableFooter } from './DataTableFooter';
import { FullscreenOverlay } from './FullscreenOverlay';
const FULL_WIDTH_CLASS = 'w-full min-w-0 max-w-full';
function getPageNumbers(current: number, total: number): (number | 'ellipsis')[] {
    if (total <= 7) {
        return Array.from({ length: total }, (_, i) => i + 1);
    }
    const pages: (number | 'ellipsis')[] = [1];
    const left = Math.max(2, current - 1);
    const right = Math.min(total - 1, current + 1);
    if (left > 2) {
        pages.push('ellipsis');
    }
    for (let i = left; i <= right; i++) {
        pages.push(i);
    }
    if (right < total - 1) {
        pages.push('ellipsis');
    }
    pages.push(total);
    return pages;
}
function PaginationButton(rawProps: {
    onClick: () => void;
    disabled?: boolean;
    active?: boolean;
    children: JSX.Element;
    'aria-label'?: string;
    'aria-current'?: 'page';
}) {
    const [nativeProps, rest] = splitProps(rawProps, ["onClick", "disabled", "active", "children"]);
    return (<button type="button" onClick={nativeProps.onClick} disabled={nativeProps.disabled} class={`min-w-[1.5rem] cursor-pointer rounded px-1.5 py-0.5 text-xs font-medium transition-colors ${nativeProps.active ? 'bg-blue-600 text-white'
            : 'text-gray-600 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-700'} disabled:cursor-not-allowed disabled:opacity-40`} {...rest}>
      {nativeProps.children}
    </button>);
}
/** External container control. When provided, the card runs in embedded mode. */
export interface TableCardHost {
    onToggleFullscreen: () => void;
    isFullscreen: boolean;
}
interface InteractiveTableCardProps {
    table: RenderTableData;
    onSendMessage?: (content: string) => void;
    sendDisabled?: boolean;
    options?: DataTableOptions;
    /** When provided, the card runs in embedded mode — strips outer chrome, hides header, delegates fullscreen to the host. */
    host?: TableCardHost;
}
export const InteractiveTableCard = function InteractiveTableCard(solidProps1Input: InteractiveTableCardProps) {
    const solidProps1 = mergeProps({ sendDisabled: false } as const, solidProps1Input);
    const solidState2 = useTranslation();
    const dt = useDataTable(() => solidProps1.table, solidProps1.options);
    const canExportViaPrompt = createMemo(() => !!solidProps1.onSendMessage && !!solidProps1.table.exportPrompt);
    const exportDisabled = createMemo(() => solidProps1.sendDisabled || (!solidProps1.table.export?.url && !canExportViaPrompt()));
    const handleExport = () => {
        if (solidProps1.table.export?.url) {
            try {
                const parsed = new URL(solidProps1.table.export.url, window.location.origin);
                if (!['http:', 'https:', 'blob:'].includes(parsed.protocol)) {
                    console.warn('[InteractiveTableCard] Blocked export URL with unsafe protocol:', parsed.protocol);
                    return;
                }
            }
            catch {
                console.warn('[InteractiveTableCard] Blocked malformed export URL');
                return;
            }
            const link = document.createElement('a');
            link.href = solidProps1.table.export.url;
            link.download = solidProps1.table.export.filename || 'table_export.xlsx';
            link.rel = 'noopener noreferrer';
            document.body.appendChild(link);
            link.click();
            document.body.removeChild(link);
            return;
        }
        if (canExportViaPrompt() && solidProps1.table.exportPrompt) {
            solidProps1.onSendMessage?.(solidProps1.table.exportPrompt);
        }
    };
    const handleCellCopy = (text: string) => {
        navigator.clipboard.writeText(text).catch(() => {
            /* clipboard API unavailable */
        });
    };
    const handleCopyTable = () => {
        const tsv = dt.getTableAsTSV();
        navigator.clipboard.writeText(tsv).catch(() => {
            /* clipboard API unavailable */
        });
    };
    const [internalFullscreen, setInternalFullscreen] = createSignal(false);
    const isFullscreen = createMemo(() => solidProps1.host ? solidProps1.host.isFullscreen : internalFullscreen());
    const toggleFullscreen = createMemo(() => solidProps1.host ? solidProps1.host.onToggleFullscreen : () => setInternalFullscreen((v) => !v));
    const [hoveredRow, setHoveredRow] = createSignal<number | null>(null);
    const rowBg = (rowIndex: number): string => hoveredRow() === rowIndex
        ? 'bg-gray-100 dark:bg-gray-800/40'
        : rowIndex % 2 === 1 ? 'bg-gray-50 dark:bg-gray-900' : 'bg-white dark:bg-gray-900';
    const hasHiddenColumns = dt.columns.some((c) => !c.visible);
    const from = dt.totalFilteredRows === 0 ? 0 : (dt.page - 1) * dt.pageSize + 1;
    const to = Math.min(dt.page * dt.pageSize, dt.totalFilteredRows);
    const loadedRowsCount = createMemo(() => solidProps1.table.rows.length);
    const reportedRowsCount = createMemo(() => Math.max(solidProps1.table.totalRows || 0, loadedRowsCount()));
    const renderToolbar = () => (<DataTableToolbar columns={dt.columns} searchQuery={dt.searchQuery} onSearchChange={dt.setSearchQuery} onToggleColumnVisibility={dt.toggleColumnVisibility} onResetColumnVisibility={dt.resetColumnVisibility} hasHiddenColumns={hasHiddenColumns} sort={dt.sort} onClearSort={dt.clearSort} onCopyTable={handleCopyTable} isFullscreen={isFullscreen()} onToggleFullscreen={toggleFullscreen()}/>);
    const renderHeader = () => (<header class="flex flex-wrap items-center justify-between gap-2 border-b border-gray-200 px-3 py-2 dark:border-gray-700">
      <div class="min-w-0">
        <h4 class="truncate text-sm font-semibold text-gray-900 dark:text-gray-100">
          {solidProps1.table.title || solidState2.t('BiChat.Table.QueryResults')}
        </h4>
        <p class="text-xs text-gray-500 dark:text-gray-400">
          {dt.totalFilteredRows === loadedRowsCount()
            ? loadedRowsCount() === reportedRowsCount()
                ? loadedRowsCount() === 1
                    ? solidState2.t('BiChat.Table.OneRowLoaded')
                    : solidState2.t('BiChat.Table.RowsLoaded', { count: String(loadedRowsCount()) })
                : solidState2.t('BiChat.DataTable.FilteredRows', {
                    filtered: String(loadedRowsCount()),
                    total: String(reportedRowsCount()),
                })
            : solidState2.t('BiChat.DataTable.FilteredRows', {
                filtered: String(dt.totalFilteredRows),
                total: String(loadedRowsCount()),
            })}
          {solidProps1.table.truncated ? ` ${solidState2.t('BiChat.Table.TruncatedSuffix')}` : ''}
        </p>
      </div>

      <TableExportButton onClick={handleExport} disabled={exportDisabled()} label={solidState2.t('BiChat.Table.ExportToExcel')} disabledTooltip={solidProps1.sendDisabled ? solidState2.t('BiChat.Table.PleaseWait') : solidState2.t('BiChat.Table.ExportUnavailable')}/>
    </header>);
    const renderTable = (scrollClass: string) => (<div class={`${FULL_WIDTH_CLASS} ${scrollClass}`}>
      <table class="min-w-full border-collapse text-sm">
        <DataTableHeader tableId={solidProps1.table.id} columns={dt.visibleColumns} sort={dt.sort} onToggleSort={dt.toggleSort} onColumnResize={dt.setColumnWidth} onToggleVisibility={dt.toggleColumnVisibility} onSendMessage={solidProps1.onSendMessage} sendDisabled={solidProps1.sendDisabled} showRowNumbers/>
        <tbody>
          {dt.pageRows.map((row, rowIndex) => (<tr class={`border-b border-gray-100 dark:border-gray-800 ${rowBg(rowIndex)}`} onMouseEnter={() => setHoveredRow(rowIndex)} onMouseLeave={() => setHoveredRow(null)}>
              <td class={`sticky left-0 z-[2] w-10 px-2 py-2 text-right text-xs tabular-nums text-gray-400 dark:text-gray-500 select-none ${rowBg(rowIndex)}`}>
                {(dt.page - 1) * dt.pageSize + rowIndex + 1}
              </td>
              {dt.visibleColumns.map((col, colIdx) => (<DataTableCell formatted={dt.formatCell(row[col.index], col.index)} alignment={dt.getCellAlignment(col.index)} onCopy={handleCellCopy} isSticky={colIdx === 0} stickyClassName={`sticky left-[2.5rem] z-[1] ${rowBg(rowIndex)}`}/>))}
            </tr>))}
          {dt.pageRows.length === 0 && (<tr>
              <td colSpan={dt.visibleColumns.length + 1} class="px-3 py-10 text-center">
                {dt.searchQuery ? (<div class="flex flex-col items-center gap-2">
                    <MagnifyingGlass size={32} className="text-gray-300 dark:text-gray-600" weight="duotone"/>
                    <p class="text-sm font-medium text-gray-500 dark:text-gray-400">
                      {solidState2.t('BiChat.DataTable.NoMatchingRows')}
                    </p>
                    <button type="button" onClick={() => dt.setSearchQuery('')} class="cursor-pointer text-xs font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300">
                      {solidState2.t('BiChat.DataTable.ClearSearchAction')}
                    </button>
                  </div>) : (<span class="text-sm text-gray-500 dark:text-gray-400">
                    {solidState2.t('BiChat.Table.NoRows')}
                  </span>)}
              </td>
            </tr>)}
        </tbody>
        <DataTableFooter visibleColumns={dt.visibleColumns} stats={dt.columnStats} showRowNumbers/>
      </table>
    </div>);
    const renderPagination = (idPrefix: string) => (<footer class="flex flex-wrap items-center justify-between gap-2 border-t border-gray-200 px-3 py-2 dark:border-gray-700">
      <div class="flex items-center gap-2">
        <div class="text-xs text-gray-500 dark:text-gray-400">
          {solidState2.t('BiChat.Table.Showing', {
            from: String(from),
            to: String(to),
            total: String(dt.totalFilteredRows),
        })}
        </div>
        <label class="text-xs text-gray-500 dark:text-gray-400" for={`${idPrefix}-page-size`}>
          {solidState2.t('BiChat.Table.RowsLabel')}
        </label>
        <select id={`${idPrefix}-page-size`} value={dt.pageSize} onChange={(event) => dt.setPageSize(Number(event.target.value))} class="rounded border border-gray-300 bg-white px-2 py-1 text-xs text-gray-700 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200">
          {dt.pageSizeOptions.map((option) => (<option value={option}>
              {option}
            </option>))}
        </select>
      </div>

      <nav class="flex items-center gap-1" aria-label="Pagination">
        <PaginationButton onClick={() => dt.setPage(1)} disabled={dt.page <= 1} aria-label={solidState2.t('BiChat.DataTable.FirstPage')}>
          &laquo;
        </PaginationButton>
        <PaginationButton onClick={() => dt.setPage(Math.max(1, dt.page - 1))} disabled={dt.page <= 1} aria-label={solidState2.t('BiChat.Table.Prev')}>
          &lsaquo;
        </PaginationButton>
        {getPageNumbers(dt.page, dt.totalPages).map((item, i) => item === 'ellipsis' ? (<span class="px-1 text-xs text-gray-400" aria-hidden>
              &hellip;
            </span>) : (<PaginationButton onClick={() => dt.setPage(item)} active={item === dt.page} aria-current={item === dt.page ? 'page' : undefined}>
              {item}
            </PaginationButton>))}
        <PaginationButton onClick={() => dt.setPage(Math.min(dt.totalPages, dt.page + 1))} disabled={dt.page >= dt.totalPages} aria-label={solidState2.t('BiChat.Table.Next')}>
          &rsaquo;
        </PaginationButton>
        <PaginationButton onClick={() => dt.setPage(dt.totalPages)} disabled={dt.page >= dt.totalPages} aria-label={solidState2.t('BiChat.DataTable.LastPage')}>
          &raquo;
        </PaginationButton>
      </nav>
    </footer>);
    const renderTruncationNotice = () => solidProps1.table.truncated ? (<p class="border-t border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:border-amber-700/60 dark:bg-amber-900/20 dark:text-amber-300">
        {solidState2.t('BiChat.Table.TruncatedNotice')}
      </p>) : null;
    const fillHeight = createMemo(() => solidProps1.host?.isFullscreen ?? false);
    const sectionClassName = createMemo(() => solidProps1.host ? `${FULL_WIDTH_CLASS} overflow-hidden${fillHeight() ? ' flex flex-col flex-1' : ''}`
        : `${FULL_WIDTH_CLASS} rounded-xl border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-900/40 overflow-hidden`);
    return (<>
      <section class={sectionClassName()} data-testid="bichat-table-card">
        {renderToolbar()}
        {!solidProps1.host && renderHeader()}
        {renderTable(fillHeight() ? 'flex-1 overflow-auto' : 'max-h-[420px] overflow-auto')}
        {renderPagination(solidProps1.table.id)}
        {renderTruncationNotice()}
      </section>

      {/* Standalone fullscreen — only when not hosted */}
      {!solidProps1.host && isFullscreen() && (<FullscreenOverlay title={solidProps1.table.title || solidState2.t('BiChat.Table.QueryResults')} onClose={() => setInternalFullscreen(false)} closeLabel={solidState2.t('BiChat.DataTable.Collapse')}>
          {renderToolbar()}
          {renderHeader()}
          {renderTable('flex-1 overflow-auto')}
          {renderPagination(`${solidProps1.table.id}-fs`)}
          {renderTruncationNotice()}
        </FullscreenOverlay>)}
    </>);
};
