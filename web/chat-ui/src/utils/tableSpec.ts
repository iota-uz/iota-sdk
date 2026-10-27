import type { RenderTableData, RenderTableExport } from '../types';
function isRecord(value: unknown): value is Record<string, unknown> {
    return typeof value === 'object' && value !== null && !Array.isArray(value);
}
function readString(value: unknown): string | null {
    if (typeof value !== 'string') {
        return null;
    }
    const trimmed = value.trim();
    return trimmed.length > 0 ? trimmed : null;
}
function readPositiveInteger(value: unknown): number | null {
    if (typeof value === 'string') {
        const trimmed = value.trim();
        if (!/^\d+$/.test(trimmed)) {
            return null;
        }
        const parsed = Number(trimmed);
        return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : null;
    }
    if (typeof value !== 'number' || !Number.isFinite(value)) {
        return null;
    }
    const n = Math.floor(value);
    return n > 0 ? n : null;
}
function normalizeRows(value: unknown): unknown[][] {
    if (!Array.isArray(value)) {
        return [];
    }
    const rows: unknown[][] = [];
    for (const row of value) {
        if (!Array.isArray(row)) {
            continue;
        }
        rows.push(row);
    }
    return rows;
}
function parseExport(value: unknown): RenderTableExport | undefined {
    if (!isRecord(value)) {
        return undefined;
    }
    const url = readString(value.url);
    if (!url) {
        return undefined;
    }
    return {
        url,
        filename: readString(value.filename) || 'table_export.xlsx',
        rowCount: readPositiveInteger(value.row_count) || readPositiveInteger(value.rowCount) || undefined,
        fileSizeKB: readPositiveInteger(value.file_size_kb) || readPositiveInteger(value.fileSizeKB) || undefined,
    };
}
/**
 * Parses RenderTableData from a record (e.g. artifact metadata from render_table tool).
 */
export function parseRenderTableDataFromMetadata(metadata: Record<string, unknown>, fallbackId: string): RenderTableData | null {
    if (!isRecord(metadata)) {
        return null;
    }
    return parseRenderTableDataFromObject(metadata, fallbackId);
}
function parseRenderTableDataFromObject(parsed: Record<string, unknown>, fallbackId: string): RenderTableData | null {
    const columns = Array.isArray(parsed.columns)
        ? parsed.columns
            .map((column) => readString(column))
            .filter((column): column is string => column !== null)
        : [];
    if (columns.length === 0) {
        return null;
    }
    const rows = normalizeRows(parsed.rows);
    const headersRaw = Array.isArray(parsed.headers)
        ? parsed.headers
            .map((header) => readString(header))
            .filter((header): header is string => header !== null)
        : [];
    const headers = headersRaw.length === columns.length ? headersRaw : columns;
    const columnTypesRaw = Array.isArray(parsed.column_types)
        ? parsed.column_types
        : Array.isArray(parsed.columnTypes)
            ? parsed.columnTypes
            : [];
    const columnTypes = columnTypesRaw.length === columns.length
        ? columnTypesRaw.map((t) => readString(t) || 'string')
        : undefined;
    const totalRows = readPositiveInteger(parsed.total_rows) ||
        readPositiveInteger(parsed.totalRows) ||
        readPositiveInteger(parsed.row_count) ||
        readPositiveInteger(parsed.rowCount) ||
        rows.length;
    const pageSize = readPositiveInteger(parsed.page_size) || readPositiveInteger(parsed.pageSize) || 25;
    const query = readString(parsed.query) || readString(parsed.sql);
    if (!query) {
        return null;
    }
    return {
        id: readString(parsed.id) || fallbackId,
        title: readString(parsed.title) || undefined,
        query,
        columns,
        columnTypes,
        headers,
        rows,
        totalRows,
        pageSize,
        truncated: parsed.truncated === true,
        truncatedReason: readString(parsed.truncated_reason) || readString(parsed.truncatedReason) || undefined,
        export: parseExport(parsed.export),
        exportPrompt: readString(parsed.export_prompt) || readString(parsed.exportPrompt) || undefined,
    };
}
export function parseRenderTableDataFromJsonString(json: string, fallbackId: string): RenderTableData | null {
    const trimmed = json.trim();
    if (!trimmed) {
        return null;
    }
    let parsed: unknown;
    try {
        parsed = JSON.parse(trimmed);
    }
    catch {
        return null;
    }
    if (!isRecord(parsed)) {
        return null;
    }
    return parseRenderTableDataFromObject(parsed, fallbackId);
}
