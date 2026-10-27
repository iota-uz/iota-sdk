/** @jsxImportSource solid-js */
import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
import type { RenderTableData } from '../types';
import { type ColumnType, type FormattedCell, inferColumnType, formatCellValue } from '../utils/columnTypes';
export interface ColumnMeta {
    index: number;
    name: string;
    header: string;
    type: ColumnType;
    width: number | null;
    visible: boolean;
}
export interface SortState {
    columnIndex: number;
    direction: 'asc' | 'desc';
}
export interface ColumnStats {
    sum: number;
    avg: number;
    min: number;
    max: number;
    count: number;
    nullCount: number;
}
export interface DataTableOptions {
    defaultPageSize?: number;
    enableSearch?: boolean;
    enableSort?: boolean;
    enableResize?: boolean;
    enableColumnVisibility?: boolean;
}
const PAGE_SIZE_OPTIONS = [10, 25, 50, 100, 200];
export interface UseDataTableReturn {
    columns: ColumnMeta[];
    visibleColumns: ColumnMeta[];
    page: number;
    pageSize: number;
    totalPages: number;
    totalFilteredRows: number;
    pageSizeOptions: number[];
    pageRows: unknown[][];
    setPage: (page: number) => void;
    setPageSize: (size: number) => void;
    sort: SortState | null;
    toggleSort: (columnIndex: number) => void;
    clearSort: () => void;
    searchQuery: string;
    setSearchQuery: (query: string) => void;
    columnStats: Map<number, ColumnStats>;
    toggleColumnVisibility: (columnIndex: number) => void;
    resetColumnVisibility: () => void;
    setColumnWidth: (columnIndex: number, width: number) => void;
    formatCell: (value: unknown, columnIndex: number) => FormattedCell;
    getCellAlignment: (columnIndex: number) => 'left' | 'right';
    getTableAsTSV: () => string;
}
export function useDataTable(source: RenderTableData | (() => RenderTableData), options?: DataTableOptions): UseDataTableReturn {
    const table = new Proxy({} as RenderTableData, { get: (_target, key) => Reflect.get(typeof source === 'function' ? source() : source, key) });
    const defaultPageSize = Math.min(Math.max(table.pageSize || options?.defaultPageSize || 25, 1), 200);
    const [page, setPage] = createSignal(1);
    const [pageSize, setPageSize] = createSignal(defaultPageSize);
    const [sort, setSort] = createSignal<SortState | null>(null);
    const [searchQuery, setSearchQuery] = createSignal('');
    const [columnVisibility, setColumnVisibility] = createSignal<Map<number, boolean>>(new Map());
    const [columnWidths, setColumnWidths] = createSignal<Map<number, number>>(new Map());
    // Reset state only when the identity changes, preserving sort on streamed updates.
    const identity = createMemo(() => table.id);
    createEffect(on(identity, () => {
        const cleanup = untrack(() => {
            setPage(1);
            setPageSize(Math.min(Math.max(table.pageSize || options?.defaultPageSize || 25, 1), 200));
            setSort(null);
            setSearchQuery('');
            setColumnVisibility(new Map());
            setColumnWidths(new Map());
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    })); // eslint-disable-line react-hooks/exhaustive-deps
    // Type inference depends only on table data
    const columnTypes = createMemo(() => {
        return table.columns.map((_, index) => {
            const backendHint = table.columnTypes?.[index];
            const columnValues = table.rows.map((row) => row[index]);
            return inferColumnType(columnValues, backendHint);
        });
    });
    // Column metadata: types + width/visibility
    const columns = createMemo<ColumnMeta[]>(() => {
        return table.columns.map((name, index) => ({
            index,
            name,
            header: table.headers[index] || name,
            type: columnTypes()[index],
            width: columnWidths().get(index) ?? null,
            visible: columnVisibility().get(index) ?? true,
        }));
    });
    const visibleColumns = createMemo(() => columns().filter((c) => c.visible));
    // Apply search filter
    const searchFilteredRows = createMemo(() => {
        if (!searchQuery().trim()) {
            return table.rows;
        }
        const query = createMemo(() => searchQuery().toLowerCase());
        const visibleIndices = new Set(visibleColumns().map((c) => c.index));
        return table.rows.filter((row) => row.some((cell, i) => {
            if (!visibleIndices.has(i)) {
                return false;
            }
            if (cell === null || cell === undefined) {
                return false;
            }
            return String(cell).toLowerCase().includes(query());
        }));
    });
    // Apply sort
    const sortedRows = createMemo(() => {
        const currentSort = sort();
        if (!currentSort) {
            return searchFilteredRows();
        }
        const col = createMemo(() => columns()[currentSort.columnIndex]);
        if (!col()) {
            return searchFilteredRows();
        }
        const sorted = createMemo(() => [...searchFilteredRows()]);
        const colIdx = createMemo(() => currentSort.columnIndex);
        const dir = createMemo(() => currentSort.direction === 'asc' ? 1 : -1);
        sorted().sort((a, b) => {
            const aVal = a[colIdx()];
            const bVal = b[colIdx()];
            // Nulls always last
            const aNull = aVal === null || aVal === undefined;
            const bNull = bVal === null || bVal === undefined;
            if (aNull && bNull) {
                return 0;
            }
            if (aNull) {
                return 1;
            }
            if (bNull) {
                return -1;
            }
            switch (col().type) {
                case 'number': {
                    const aNum = typeof aVal === 'number' ? aVal : Number(aVal);
                    const bNum = typeof bVal === 'number' ? bVal : Number(bVal);
                    if (isNaN(aNum) && isNaN(bNum)) {
                        return 0;
                    }
                    if (isNaN(aNum)) {
                        return 1;
                    }
                    if (isNaN(bNum)) {
                        return -1;
                    }
                    return (aNum - bNum) * dir();
                }
                case 'date': {
                    const aTime = new Date(String(aVal)).getTime();
                    const bTime = new Date(String(bVal)).getTime();
                    if (isNaN(aTime) && isNaN(bTime)) {
                        return 0;
                    }
                    if (isNaN(aTime)) {
                        return 1;
                    }
                    if (isNaN(bTime)) {
                        return -1;
                    }
                    return (aTime - bTime) * dir();
                }
                case 'boolean': {
                    const aBool = aVal === true || aVal === 'true' ? 1 : 0;
                    const bBool = bVal === true || bVal === 'true' ? 1 : 0;
                    return (aBool - bBool) * dir();
                }
                default: {
                    return String(aVal).localeCompare(String(bVal)) * dir();
                }
            }
        });
        return sorted();
    });
    const totalFilteredRows = createMemo(() => sortedRows().length);
    // Compute stats on filtered numeric columns (visible columns only)
    const columnStats = createMemo<Map<number, ColumnStats>>(() => {
        const stats = new Map<number, ColumnStats>();
        for (const col of visibleColumns()) {
            if (col.type !== 'number') {
                continue;
            }
            let sum = 0;
            let min = Infinity;
            let max = -Infinity;
            let count = 0;
            let nullCount = 0;
            for (const row of sortedRows()) {
                const val = row[col.index];
                if (val === null || val === undefined) {
                    nullCount++;
                    continue;
                }
                const num = typeof val === 'number' ? val : Number(val);
                if (isNaN(num)) {
                    continue;
                }
                sum += num;
                min = Math.min(min, num);
                max = Math.max(max, num);
                count++;
            }
            if (count > 0) {
                stats.set(col.index, {
                    sum,
                    avg: sum / count,
                    min,
                    max,
                    count,
                    nullCount,
                });
            }
        }
        return stats;
    });
    // Pagination
    const totalPages = createMemo(() => Math.max(1, Math.ceil(totalFilteredRows() / pageSize())));
    createEffect(on(() => [totalPages()], () => {
        const cleanup = untrack(() => {
            setPage((prev) => Math.min(prev, totalPages()));
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    const pageRows = createMemo(() => {
        const start = createMemo(() => (page() - 1) * pageSize());
        return sortedRows().slice(start(), start() + pageSize());
    });
    const pageSizeOptions = createMemo(() => {
        const set = new Set([...PAGE_SIZE_OPTIONS, defaultPageSize]);
        return [...set].sort((a, b) => a - b);
    });
    // Reset page on search/sort change
    createEffect(on(() => [searchQuery(), sort()], () => {
        const cleanup = untrack(() => {
            setPage(1);
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    // Actions
    const toggleSort = (columnIndex: number) => {
        if (options?.enableSort === false) {
            return;
        }
        setSort((prev) => {
            if (!prev || prev.columnIndex !== columnIndex) {
                return { columnIndex, direction: 'asc' };
            }
            if (prev.direction === 'asc') {
                return { columnIndex, direction: 'desc' };
            }
            return null;
        });
    };
    const handleSetSearchQuery = (query: string) => {
        if (options?.enableSearch === false) {
            return;
        }
        setSearchQuery(query);
    };
    const toggleColumnVisibility = (columnIndex: number) => {
        if (options?.enableColumnVisibility === false) {
            return;
        }
        const numColumns = table.columns.length;
        setColumnVisibility((prev) => {
            const next = new Map(prev);
            const currentlyVisible = prev.get(columnIndex) ?? true;
            if (currentlyVisible) {
                let visibleCount = 0;
                for (let i = 0; i < numColumns; i++) {
                    if (prev.get(i) ?? true) {
                        visibleCount++;
                    }
                }
                if (visibleCount <= 1) {
                    return prev;
                }
            }
            next.set(columnIndex, !currentlyVisible);
            return next;
        });
    };
    const resetColumnVisibility = () => {
        setColumnVisibility(new Map());
    };
    const setColumnWidthCb = (columnIndex: number, width: number) => {
        if (options?.enableResize === false) {
            return;
        }
        setColumnWidths((prev) => {
            const next = new Map(prev);
            next.set(columnIndex, Math.max(60, width));
            return next;
        });
    };
    const formatCell = (value: unknown, columnIndex: number): FormattedCell => {
        const type = columnTypes()[columnIndex] ?? 'string';
        return formatCellValue(value, type);
    };
    const getCellAlignment = (columnIndex: number): 'left' | 'right' => {
        return columnTypes()[columnIndex] === 'number' ? 'right' : 'left';
    };
    const clearSort = () => setSort(null);
    const handleSetPageSize = (size: number) => {
        setPageSize(size);
        setPage(1);
    };
    const getTableAsTSV = (): string => {
        const escape = (v: string) => v.replace(/[\t\r\n]/g, ' ');
        const visCols = columns().filter((c) => c.visible);
        const headerRow = visCols.map((c) => escape(c.header)).join('\t');
        const dataRows = sortedRows().map((row) => visCols.map((c) => {
            const val = row[c.index];
            if (val === null || val === undefined) {
                return '';
            }
            return escape(String(val));
        }).join('\t'));
        return [headerRow, ...dataRows].join('\n');
    };
    return {
        get columns() {
            return columns();
        },
        get visibleColumns() {
            return visibleColumns();
        },
        get page() {
            return page();
        },
        get pageSize() {
            return pageSize();
        },
        get totalPages() {
            return totalPages();
        },
        get totalFilteredRows() {
            return totalFilteredRows();
        },
        get pageSizeOptions() {
            return pageSizeOptions();
        },
        get pageRows() {
            return pageRows();
        },
        setPage,
        setPageSize: handleSetPageSize,
        get sort() {
            return sort();
        },
        toggleSort,
        clearSort,
        get searchQuery() {
            return searchQuery();
        },
        setSearchQuery: handleSetSearchQuery,
        get columnStats() {
            return columnStats();
        },
        toggleColumnVisibility,
        resetColumnVisibility,
        setColumnWidth: setColumnWidthCb,
        formatCell,
        getCellAlignment,
        getTableAsTSV,
    };
}
