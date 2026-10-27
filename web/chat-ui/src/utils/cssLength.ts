export function cssLength(value: string | number | null | undefined): string | undefined { return value == null ? undefined : typeof value === 'number' ? value + 'px' : value }
