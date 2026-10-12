import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * ChartCard Component
 * Renders chart visualizations using ApexCharts
 *
 * Supports multiple chart types: line, bar, pie, area, donut
 * Includes PNG export functionality and responsive styling
 */
import ApexCharts, { ApexOptions } from 'apexcharts';
import { DownloadSimple } from '../icons';
import type { ChartData } from '../types';
import { useTranslation } from '../hooks/useTranslation';
/** External container control. When provided, the card runs in embedded mode. */
export interface ChartCardHost {
    isFullscreen: boolean;
}
interface ChartCardProps {
    chartData: ChartData;
    onExportError?: (error: string) => void;
    /** When provided, the card runs in embedded mode — strips outer chrome, fills container height in fullscreen. */
    host?: ChartCardHost;
}
interface InlineTooltipState {
    left: number;
    top: number;
    label: string;
    seriesName: string;
    value: string;
}
interface HoverEventLike {
    target?: EventTarget | null;
    clientX?: number;
    clientY?: number;
}
// Default color palette if none provided
const DEFAULT_COLORS = ['#6366f1', '#06b6d4', '#f59e0b', '#ef4444', '#8b5cf6'];
const TOOLTIP_X_PADDING = 12;
const TOOLTIP_Y_PADDING = 8;
const TOOLTIP_CURSOR_Y_OFFSET = 12;
function clamp(value: number, min: number, max: number): number {
    return Math.min(Math.max(value, min), max);
}
function getEventClientValue(event: unknown, key: 'clientX' | 'clientY'): number | null {
    if (!event || typeof event !== 'object') {
        return null;
    }
    const value = (event as HoverEventLike)[key];
    return typeof value === 'number' ? value : null;
}
function isRecord(value: unknown): value is Record<string, unknown> {
    return typeof value === 'object' && value !== null && !Array.isArray(value);
}
function cloneDeep<T>(value: T): T {
    if (Array.isArray(value)) {
        return value.map((item) => cloneDeep(item)) as T;
    }
    if (isRecord(value)) {
        const out: Record<string, unknown> = {};
        Object.entries(value).forEach(([k, v]) => {
            out[k] = cloneDeep(v);
        });
        return out as T;
    }
    return value;
}
function detectCurrencyHint(text: string): string | null {
    const codeMatch = text.match(/\b(USD|EUR|GBP|JPY|CHF|AUD|CAD|NZD|CNY|INR|RUB|UZS)\b/i);
    if (codeMatch) {
        return codeMatch[1].toUpperCase();
    }
    if (text.includes('$')) {
        return 'USD';
    }
    if (text.includes('EUR') || text.includes('€')) {
        return 'EUR';
    }
    if (text.includes('GBP') || text.includes('£')) {
        return 'GBP';
    }
    if (text.includes('JPY') || text.includes('¥')) {
        return 'JPY';
    }
    return null;
}
function extractYValues(chartType: ChartData['chartType'], series: ChartData['series'], richSeries?: unknown): number[] {
    if (chartType === 'pie' || chartType === 'donut') {
        if (Array.isArray(richSeries)) {
            return richSeries.filter((v): v is number => typeof v === 'number' && Number.isFinite(v));
        }
        return series[0]?.data || [];
    }
    return series.flatMap((s) => s.data).filter((v): v is number => Number.isFinite(v));
}
function createMoneyFormatter(title: string, seriesNames: string[], yValues: number[]): ((value: number) => string) | null {
    if (yValues.length === 0) {
        return null;
    }
    const textSignals = [title, ...seriesNames].join(' ');
    const currencyHint = detectCurrencyHint(textSignals);
    const monetaryKeywords = /\b(revenue|sales|amount|price|cost|profit|income|expense|balance|payment|salary|budget|currency|premium)\b/i.test(textSignals) || /[$€£¥]/.test(textSignals);
    if (!monetaryKeywords && !currencyHint) {
        return null;
    }
    const currency = currencyHint || 'USD';
    const maxFractionDigits = yValues.some((value) => Math.abs(value) < 1) ? 4 : 2;
    try {
        const formatter = new Intl.NumberFormat(undefined, {
            style: 'currency',
            currency,
            maximumFractionDigits: maxFractionDigits,
        });
        return (value: number): string => formatter.format(value);
    }
    catch {
        const fallback = new Intl.NumberFormat(undefined, { maximumFractionDigits: maxFractionDigits });
        return (value: number): string => fallback.format(value);
    }
}
function applyMoneyFormatting(options: ApexOptions, formatter: ((value: number) => string) | null): ApexOptions {
    if (!formatter) {
        return options;
    }
    const next: ApexOptions = { ...options };
    const yaxis = next.yaxis;
    if (Array.isArray(yaxis)) {
        next.yaxis = yaxis.map((axis) => ({
            ...axis,
            labels: {
                ...(axis?.labels || {}),
                formatter: axis?.labels?.formatter || ((value: number) => formatter(value)),
            },
        }));
    }
    else {
        next.yaxis = {
            ...(yaxis || {}),
            labels: {
                ...(yaxis?.labels || {}),
                formatter: yaxis?.labels?.formatter || ((value: number) => formatter(value)),
            },
        };
    }
    const tooltipY = next.tooltip?.y;
    const tooltipYObject = !Array.isArray(tooltipY) && isRecord(tooltipY) ? tooltipY : {};
    const existingFormatter = !Array.isArray(tooltipY) && isRecord(tooltipY) && typeof tooltipY.formatter === 'function'
        ? tooltipY.formatter
        : undefined;
    next.tooltip = {
        ...(next.tooltip || {}),
        y: {
            ...tooltipYObject,
            formatter: existingFormatter || ((value: number) => formatter(value)),
        },
    };
    return next;
}
/**
 * ChartCard renders a single chart visualization with optional PNG export.
 */
export function ChartCard(solidProps1: ChartCardProps) {
    const solidState2 = useTranslation();
    const chartId = createUniqueId().replace(/:/g, '_');
    const [isExporting, setIsExporting] = createSignal(false);
    const [useInlineTooltip, setUseInlineTooltip] = createSignal(false);
    const [inlineTooltip, setInlineTooltip] = createSignal<InlineTooltipState | null>(null);
    const cardRef = { current: null } as {
        current: HTMLDivElement | null;
    };
    const data = createMemo(() => solidProps1.chartData);
    const chartType = createMemo(() => data().chartType);
    const title = createMemo(() => data().title);
    const series = createMemo(() => data().series);
    const labels = createMemo(() => data().labels);
    const colors = createMemo(() => data().colors);
    const height = createMemo(() => data().height ?? 350);
    const richOptions = createMemo(() => (isRecord(solidProps1.chartData.options) ? cloneDeep(solidProps1.chartData.options) : null));
    const chartLabels = createMemo(() => (labels() ?? []).filter((label): label is string => label !== null));
    const hasValidData = createMemo(() => series().some(s => s.data?.length > 0));
    createEffect(on(() => [chartType()], () => {
        const cleanup = untrack(() => {
            const rootNode = cardRef.current?.getRootNode();
            const inShadowRoot = typeof ShadowRoot !== 'undefined' && rootNode instanceof ShadowRoot;
            setUseInlineTooltip(chartType() === 'bar' && inShadowRoot);
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    createEffect(on(() => [useInlineTooltip()], () => {
        const cleanup = untrack(() => {
            if (!useInlineTooltip()) {
                setInlineTooltip(null);
            }
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    const apexSeries = createMemo(() => {
        const configured = richOptions()?.series;
        if (Array.isArray(configured)) {
            if (chartType() === 'pie' || chartType() === 'donut') {
                const numeric = configured.filter((v): v is number => typeof v === 'number' && Number.isFinite(v));
                if (numeric.length)
                    return numeric;
            }
            else {
                const mapped = configured.filter((item): item is {
                    name?: unknown;
                    data: unknown[];
                } => isRecord(item) && Array.isArray(item.data))
                    .map((item, index) => ({ name: typeof item.name === 'string' && item.name.trim() ? item.name : `Series ${index + 1}`, data: item.data.map(v => typeof v === 'number' && Number.isFinite(v) ? v : null) }))
                    .filter(item => item.data.some(v => v !== null));
                if (mapped.length)
                    return mapped;
            }
        }
        return chartType() === 'pie' || chartType() === 'donut' ? series()[0]?.data ?? [] : series().map(s => ({ name: s.name, data: s.data }));
    });
    const xaxisConfig = createMemo<ApexOptions['xaxis']>(() => {
        if (chartType() === 'pie' || chartType() === 'donut')
            return {};
        const configured = richOptions()?.xaxis;
        return isRecord(configured) ? configured as ApexOptions['xaxis'] : { categories: chartLabels() };
    });
    const labelsConfig = createMemo(() => {
        if (chartType() !== 'pie' && chartType() !== 'donut')
            return [];
        const configured = richOptions()?.labels;
        if (Array.isArray(configured)) {
            const values = configured.filter((v): v is string => typeof v === 'string');
            if (values.length)
                return values;
        }
        return chartLabels();
    });
    const handleDataPointMouseEnter = (event: unknown, _chartContext: unknown, config: {
        seriesIndex?: number;
        dataPointIndex?: number;
    }) => {
        if (!useInlineTooltip() || chartType() !== 'bar' || !cardRef.current) {
            return;
        }
        const target = (event &&
            typeof event === 'object' &&
            'target' in event &&
            (event as {
                target?: Element | null;
            }).target?.closest('.apexcharts-bar-area')) ||
            null;
        if (!(target instanceof SVGGraphicsElement)) {
            return;
        }
        const seriesIndex = typeof config?.seriesIndex === 'number' ? config.seriesIndex : 0;
        const dataPointIndex = typeof config?.dataPointIndex === 'number' ? config.dataPointIndex : -1;
        if (dataPointIndex < 0) {
            return;
        }
        const cardRect = cardRef.current.getBoundingClientRect();
        const barRect = target.getBoundingClientRect();
        const rawValue = series()?.[seriesIndex]?.data?.[dataPointIndex] ?? target.getAttribute('val') ?? '';
        const value = typeof rawValue === 'number' && Number.isFinite(rawValue)
            ? rawValue.toLocaleString()
            : String(rawValue);
        const label = chartLabels()[dataPointIndex] || '';
        const seriesName = series()?.[seriesIndex]?.name || 'Value';
        const cursorX = getEventClientValue(event, 'clientX');
        const cursorY = getEventClientValue(event, 'clientY');
        const fallbackLeft = barRect.left - cardRect.left + barRect.width / 2;
        const fallbackTop = barRect.top - cardRect.top - TOOLTIP_Y_PADDING;
        const maxLeft = Math.max(cardRect.width - TOOLTIP_X_PADDING, TOOLTIP_X_PADDING);
        const maxTop = Math.max(cardRect.height - TOOLTIP_Y_PADDING, TOOLTIP_Y_PADDING);
        const left = clamp(cursorX !== null ? cursorX - cardRect.left : fallbackLeft, TOOLTIP_X_PADDING, maxLeft);
        const top = clamp(cursorY !== null ? cursorY - cardRect.top - TOOLTIP_CURSOR_Y_OFFSET : fallbackTop, TOOLTIP_Y_PADDING, maxTop);
        setInlineTooltip({ left, top, get label() {
                return label;
            }, seriesName, value });
    };
    const handleMouseLeave = () => {
        setInlineTooltip(null);
    };
    const options = createMemo<ApexOptions>(() => {
        const base = richOptions() ? (richOptions() as ApexOptions)
            : ({
                plotOptions: {
                    bar: { columnWidth: '60%' },
                },
                legend: { position: 'bottom', horizontalAlign: 'center' },
                dataLabels: { enabled: chartType() === 'pie' || chartType() === 'donut' },
                stroke: {
                    curve: 'smooth',
                    width: chartType() === 'line' || chartType() === 'area' ? 2 : 0,
                },
                fill: { opacity: chartType() === 'area' ? 0.4 : 1 },
                grid: {
                    borderColor: 'var(--bichat-color-chart-grid, rgba(148, 163, 184, 0.15))',
                    strokeDashArray: 3,
                },
            } as ApexOptions);
        const yValues = extractYValues(chartType(), series(), richOptions()?.series);
        const moneyFormatter = createMoneyFormatter(title(), series().map((s) => s.name), yValues);
        const next: ApexOptions = ({
            ...base,
            chart: {
                ...(base.chart || {}),
                id: chartId,
                type: chartType() as 'line' | 'bar' | 'area' | 'pie' | 'donut',
                toolbar: {
                    ...(base.chart?.toolbar || {}),
                    show: false,
                },
                animations: {
                    ...(base.chart?.animations || {}),
                    enabled: false,
                },
                fontFamily: 'inherit',
                ...(useInlineTooltip() ? {
                    events: {
                        dataPointMouseEnter: handleDataPointMouseEnter,
                        mouseLeave: handleMouseLeave,
                    },
                }
                    : {}),
            },
            tooltip: {
                ...(base.tooltip || {}),
                enabled: !useInlineTooltip(),
                followCursor: true,
            },
            title: {
                ...(base.title || {}),
                text: title(),
                align: base.title?.align || 'left',
                style: { fontSize: '14px', fontWeight: 600, ...(base.title?.style || {}) },
            },
            colors: colors()?.length ? colors() : (base.colors && base.colors.length ? base.colors : DEFAULT_COLORS),
            xaxis: xaxisConfig(),
            labels: labelsConfig(),
        });
        if (solidProps1.chartData.logarithmic && chartType() !== 'pie' && chartType() !== 'donut') {
            if (Array.isArray(next.yaxis)) {
                next.yaxis = next.yaxis.map((axis) => ({ ...axis, logarithmic: true }));
            }
            else {
                next.yaxis = { ...(next.yaxis || {}), logarithmic: true };
            }
        }
        return applyMoneyFormatting(next, moneyFormatter);
    });
    return <>{createMemo(() => {
            if (!hasValidData()) {
                return (<div class="rounded-xl border border-gray-200/80 bg-white p-4 shadow-sm dark:border-gray-700/60 dark:bg-gray-800">
        <p class="text-sm text-gray-500 dark:text-gray-400">
          {title() && <span class="font-medium">{title()}: </span>}
          {solidState2.t('BiChat.Chart.NoData')}
        </p>
      </div>);
            }
            const handleExportPNG = async () => {
                setIsExporting(true);
                try {
                    const chart = ApexCharts.getChartByID(chartId);
                    if (!chart) {
                        const msg = 'Chart instance not available';
                        console.error(msg);
                        solidProps1.onExportError?.(msg);
                        return;
                    }
                    const result = await chart.dataURI({ scale: 2 });
                    if (!('imgURI' in result)) {
                        const msg = 'Unexpected dataURI result format';
                        console.error(msg);
                        solidProps1.onExportError?.(msg);
                        return;
                    }
                    const link = document.createElement('a');
                    link.href = result.imgURI;
                    link.download = `${title().replace(/[^a-z0-9]/gi, '_').toLowerCase()}_chart.png`;
                    link.click();
                }
                catch (error) {
                    const msg = error instanceof Error ? error.message : 'Failed to export chart';
                    console.error('Failed to export chart:', error);
                    solidProps1.onExportError?.(msg);
                }
                finally {
                    setIsExporting(false);
                }
            };
            const fillHeight = solidProps1.host?.isFullscreen ?? false;
            const cardClassName = solidProps1.host ? `group/chart relative w-full min-w-0 overflow-hidden${fillHeight ? ' flex flex-col flex-1' : ''}`
                : 'group/chart relative rounded-xl border border-gray-200/80 bg-white p-4 shadow-sm transition-shadow duration-200 hover:shadow dark:border-gray-700/60 dark:bg-gray-800';
            const chartHeight = createMemo(() => fillHeight
                ? '100%'
                : options().chart?.height ?? height());
            return (<div ref={element => cardRef.current = element} class={cardClassName} onMouseLeave={handleMouseLeave} data-testid="bichat-chart-card">
      <div class={fillHeight ? 'flex-1 min-h-0 p-4' : undefined}>
        <OwnedApexChart options={options()} series={apexSeries()} type={chartType()} width="100%" height={chartHeight()}/>
      </div>
      {useInlineTooltip() && inlineTooltip() && (<div class="pointer-events-none absolute z-20" style={{
                        "left": `${inlineTooltip()!.left}px`,
                        "top": `${inlineTooltip()!.top}px`,
                        "transform": 'translate(-50%, -100%)'
                    }}>
          <div class="min-w-[140px] overflow-hidden rounded-md border border-gray-200 bg-white text-xs text-gray-700 shadow-md dark:border-gray-600 dark:bg-gray-900 dark:text-gray-200">
            {inlineTooltip()!.label && (<div class="border-b border-gray-200 px-2.5 py-1.5 font-medium dark:border-gray-600">
                {inlineTooltip()!.label}
              </div>)}
            <div class="flex items-center gap-2 px-2.5 py-1.5">
              <span class="h-2 w-2 rounded-full bg-primary-600"/>
              <span>
                {inlineTooltip()!.seriesName}: <strong>{inlineTooltip()!.value}</strong>
              </span>
            </div>
          </div>
        </div>)}
      <div class="flex justify-end pt-2">
        <button type="button" onClick={handleExportPNG} disabled={isExporting()} class="inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-xs font-medium text-gray-400 opacity-0 transition-all duration-150 hover:bg-gray-100 hover:text-gray-600 focus-visible:opacity-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 group-hover/chart:opacity-100 disabled:opacity-50 dark:text-gray-500 dark:hover:bg-gray-700 dark:hover:text-gray-300" title={solidState2.t('BiChat.Chart.Download')}>
          {isExporting() ? (<span class="text-gray-500 dark:text-gray-400">{solidState2.t('BiChat.Chart.Exporting')}</span>) : (<>
              <DownloadSimple className="h-3.5 w-3.5" weight="bold"/>
              <span>{solidState2.t('BiChat.Chart.DownloadPNG')}</span>
            </>)}
        </button>
      </div>
    </div>);
        })}</>;
}
function OwnedApexChart(props: {
    options: ApexOptions;
    series: ApexOptions['series'];
    type: ChartData['chartType'];
    width: string | number;
    height: string | number;
}) {
    let element!: HTMLDivElement;
    let chart: ApexCharts | undefined;
    let disposed = false;
    let pending = Promise.resolve();
    const options = (): ApexOptions => ({ ...props.options, series: props.series, chart: { ...props.options.chart, type: props.type, width: props.width, height: props.height } });
    onMount(() => {
        chart = new ApexCharts(element, options());
        pending = chart.render();
        void pending.catch(error => {
            if (!disposed)
                console.error('Ali chart render failed', error);
        });
    });
    createEffect(on(() => [props.options, props.series, props.type, props.width, props.height], () => {
        const next = options();
        pending = pending.catch(() => { }).then(async () => {
            if (!disposed && chart)
                await chart.updateOptions(next, false, true);
        });
        void pending.catch(error => {
            if (!disposed)
                console.error('Ali chart update failed', error);
        });
    }));
    onCleanup(() => { disposed = true; const owned = chart; chart = undefined; void pending.catch(() => { }).then(() => owned?.destroy()); });
    return <div ref={element} style={{ "width": String(props.width), "height": typeof props.height === 'number' ? props.height + 'px' : props.height }}/>;
}
