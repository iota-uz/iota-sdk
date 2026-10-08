export type ColumnType = 'string' | 'number' | 'boolean' | 'date' | 'url' | 'null';
export interface FormattedCell {
    display: string;
    raw: unknown;
    type: ColumnType;
    isNull: boolean;
}
const URL_PATTERN = /^https?:\/\//i;
const ISO_DATE_PATTERN = /^\d{4}-\d{2}-\d{2}([T ]\d{2}:\d{2}(:\d{2}(\.\d+)?(Z|[+-]\d{2}:?\d{2})?)?)?$/;
const MAX_SAMPLE = 20;
// Long integers (phone numbers, IDs) lose meaning when formatted with thousand
// separators and may exceed the safe integer range.
function isLongInteger(v: unknown): boolean {
    if (typeof v === 'number') {
        return Number.isInteger(v) && Math.abs(v) >= 1e9;
    }
    if (typeof v === 'string') {
        return /^\d{10,}$/.test(v.trim());
    }
    return false;
}
export function inferColumnType(values: unknown[], backendHint?: string): ColumnType {
    const nonNull = values.filter((v) => v !== null && v !== undefined);
    const sample = nonNull.slice(0, MAX_SAMPLE);
    // Override backend 'number' hint when all values look like identifiers (phone numbers, IDs).
    if (backendHint === 'number' && sample.length > 0 && sample.every(isLongInteger)) {
        return 'string';
    }
    if (backendHint) {
        switch (backendHint) {
            case 'number':
                return 'number';
            case 'boolean':
                return 'boolean';
            case 'date':
                return 'date';
            default:
                return 'string';
        }
    }
    if (nonNull.length === 0) {
        return 'null';
    }
    if (sample.every((v) => typeof v === 'number' || (typeof v === 'string' && v.trim() !== '' && !isNaN(Number(v))))) {
        if (sample.every(isLongInteger)) {
            return 'string';
        }
        return 'number';
    }
    if (sample.every((v) => typeof v === 'boolean' || v === 'true' || v === 'false')) {
        return 'boolean';
    }
    if (sample.every((v) => typeof v === 'string' && ISO_DATE_PATTERN.test(v))) {
        return 'date';
    }
    if (sample.length > 0 && sample.every((v) => typeof v === 'string' && URL_PATTERN.test(v))) {
        return 'url';
    }
    return 'string';
}
export function formatCellValue(value: unknown, type: ColumnType): FormattedCell {
    if (value === null || value === undefined) {
        return { display: 'NULL', raw: value, type, isNull: true };
    }
    switch (type) {
        case 'number': {
            const num = typeof value === 'number' ? value : Number(value);
            if (!isFinite(num)) {
                return { display: String(value), raw: value, type, isNull: false };
            }
            // Large integers (≥10 digits) are typically identifiers (phone numbers, IDs)
            // — display without thousand separators to preserve readability.
            if (Number.isInteger(num) && Math.abs(num) >= 1e9) {
                return { display: String(num), raw: value, type, isNull: false };
            }
            const display = new Intl.NumberFormat(undefined, {
                maximumFractionDigits: 6,
            }).format(num);
            return { display, raw: value, type, isNull: false };
        }
        case 'boolean': {
            const bool = typeof value === 'boolean' ? value : String(value).toLowerCase() === 'true';
            return { display: bool ? 'true' : 'false', raw: value, type, isNull: false };
        }
        case 'date': {
            const str = String(value);
            const date = new Date(str);
            if (isNaN(date.getTime())) {
                return { display: str, raw: value, type, isNull: false };
            }
            const now = new Date();
            const sameCalendarDay = date.toDateString() === now.toDateString();
            let display: string;
            if (sameCalendarDay) {
                display = date.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
            }
            else {
                const diffDays = Math.abs(now.getTime() - date.getTime()) / (1000 * 60 * 60 * 24);
                if (diffDays < 365) {
                    display = date.toLocaleDateString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
                }
                else {
                    display = date.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
                }
            }
            return { display, raw: value, type, isNull: false };
        }
        case 'url': {
            return { display: String(value), raw: value, type, isNull: false };
        }
        default: {
            const str = typeof value === 'object'
                ? JSON.stringify(value)
                : String(value);
            return { display: str, raw: value, type, isNull: false };
        }
    }
}
