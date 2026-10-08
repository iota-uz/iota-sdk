/**
 * Shared chart spec parsing for draw_chart tool output and artifact metadata.
 * Used by SessionArtifactPreview, HttpDataSource (turn.charts), and MarkdownRenderer (code blocks).
 */
import type { ChartData, ChartSeries } from '../types';
export const SUPPORTED_CHART_TYPES = new Set<ChartData['chartType']>([
    'line',
    'bar',
    'area',
    'pie',
    'donut',
]);
export function isRecord(value: unknown): value is Record<string, unknown> {
    return typeof value === 'object' && value !== null && !Array.isArray(value);
}
function cloneValue<T>(value: T): T {
    if (Array.isArray(value)) {
        return value.map((item) => cloneValue(item)) as T;
    }
    if (isRecord(value)) {
        const out: Record<string, unknown> = {};
        Object.entries(value).forEach(([key, nested]) => {
            out[key] = cloneValue(nested);
        });
        return out as T;
    }
    return value;
}
function toStringArray(value: unknown): string[] | undefined {
    if (!Array.isArray(value)) {
        return undefined;
    }
    const result = value
        .map((item) => {
        if (typeof item === 'string') {
            return item;
        }
        if (typeof item === 'number' && Number.isFinite(item)) {
            return String(item);
        }
        return null;
    })
        .filter((item): item is string => typeof item === 'string');
    return result.length > 0 ? result : undefined;
}
function toChartSeriesFromApex(value: unknown): ChartSeries[] | null {
    if (!Array.isArray(value) || value.length === 0) {
        return null;
    }
    const result: ChartSeries[] = [];
    for (let i = 0; i < value.length; i++) {
        const item = value[i];
        if (!isRecord(item) || !Array.isArray(item.data)) {
            return null;
        }
        const data: number[] = [];
        for (const point of item.data) {
            if (typeof point === 'number' && Number.isFinite(point)) {
                data.push(point);
                continue;
            }
            if (isRecord(point) && typeof point.y === 'number' && Number.isFinite(point.y)) {
                data.push(point.y);
                continue;
            }
            return null;
        }
        if (data.length === 0) {
            return null;
        }
        const fallbackName = `Series ${i + 1}`;
        const name = typeof item.name === 'string' && item.name.trim() ? item.name : fallbackName;
        result.push({ name, data });
    }
    return result.length > 0 ? result : null;
}
function normalizeChartType(value: unknown): ChartData['chartType'] | null {
    if (typeof value !== 'string') {
        return null;
    }
    const normalized = value.trim().toLowerCase();
    if (SUPPORTED_CHART_TYPES.has(normalized as ChartData['chartType'])) {
        return normalized as ChartData['chartType'];
    }
    return null;
}
function readLogarithmicFromOptions(options: Record<string, unknown>): boolean | undefined {
    const yaxis = options.yaxis;
    if (isRecord(yaxis) && typeof yaxis.logarithmic === 'boolean') {
        return yaxis.logarithmic;
    }
    if (Array.isArray(yaxis)) {
        for (const axis of yaxis) {
            if (isRecord(axis) && typeof axis.logarithmic === 'boolean') {
                if (axis.logarithmic) {
                    return true;
                }
            }
        }
    }
    return undefined;
}
function parseApexChartDataFromSpec(spec: Record<string, unknown>, fallbackTitle: string): ChartData | null {
    const options = isRecord(spec.options) ? spec.options : spec;
    const chart = isRecord(options.chart) ? options.chart : undefined;
    const chartType = normalizeChartType(chart?.type);
    if (!chartType) {
        return null;
    }
    const titleRaw = isRecord(options.title) ? options.title.text : options.title;
    const title = typeof titleRaw === 'string' && titleRaw.trim() ? titleRaw : fallbackTitle;
    const seriesRaw = options.series;
    let series: ChartSeries[] | null = null;
    if (chartType === 'pie' || chartType === 'donut') {
        if (Array.isArray(seriesRaw)) {
            const numeric = seriesRaw.filter((point): point is number => typeof point === 'number' && Number.isFinite(point));
            if (numeric.length > 0) {
                series = [{ name: title, data: numeric }];
            }
            else {
                series = toChartSeriesFromApex(seriesRaw);
            }
        }
    }
    else {
        series = toChartSeriesFromApex(seriesRaw);
    }
    if (!series) {
        return null;
    }
    const labels = toStringArray(options.labels) ||
        (isRecord(options.xaxis) ? toStringArray(options.xaxis.categories) : undefined);
    const colors = toStringArray(options.colors);
    const height = chart && typeof chart.height === 'number' && Number.isFinite(chart.height)
        ? chart.height
        : undefined;
    const logarithmic = readLogarithmicFromOptions(options);
    return {
        chartType,
        title,
        series,
        labels,
        colors,
        height,
        options: cloneValue(options),
        logarithmic,
    };
}
/**
 * Parses a chart spec object (Apex options at top-level or under `options`) into ChartData.
 */
export function parseChartDataFromSpec(spec: Record<string, unknown>, fallbackTitle = 'Chart'): ChartData | null {
    return parseApexChartDataFromSpec(spec, fallbackTitle);
}
/**
 * Parses a JSON string as a chart spec (e.g. from a ```chart or ```json code block).
 */
export function parseChartDataFromJsonString(json: string, fallbackTitle = 'Chart'): ChartData | null {
    const trimmed = json.trim();
    if (!trimmed) {
        return null;
    }
    let spec: unknown;
    try {
        spec = JSON.parse(trimmed);
    }
    catch {
        return null;
    }
    if (!isRecord(spec)) {
        return null;
    }
    return parseChartDataFromSpec(spec, fallbackTitle);
}
