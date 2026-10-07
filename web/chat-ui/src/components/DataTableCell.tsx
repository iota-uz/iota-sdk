import { cssLength } from "../utils/cssLength";
import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
import { Portal } from 'solid-js/web';
import { Copy, Check } from '../icons';
import type { FormattedCell } from '../utils/columnTypes';
import { useTranslation } from '../hooks/useTranslation';
interface DataTableCellProps {
    formatted: FormattedCell;
    alignment: 'left' | 'right';
    onCopy?: (value: string) => void;
    isSticky?: boolean;
    stickyClassName?: string;
}
type CellRenderer = (props: {
    formatted: FormattedCell;
    tooltipRef: {
        current: HTMLSpanElement | null;
    };
    onMouseEnter: () => void;
    onMouseLeave: () => void;
}) => JSX.Element;
const SAFE_URL_PROTOCOLS = ['http:', 'https:', 'mailto:'];
function safeHref(input: string): string {
    if (!input || typeof input !== 'string') {
        return '#';
    }
    try {
        const url = new URL(input, 'https://example.com');
        return SAFE_URL_PROTOCOLS.includes(url.protocol) ? url.href : '#';
    }
    catch {
        return '#';
    }
}
const cellRenderers: Record<string, CellRenderer> = {
    boolean: (solidProps1) => (<span class={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${solidProps1.formatted.raw === true
            ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
            : 'bg-red-100 text-red-600 dark:bg-red-900/30 dark:text-red-400'}`}>
      {solidProps1.formatted.display}
    </span>),
    url: (solidProps2) => {
        const href = safeHref(solidProps2.formatted.display);
        if (href === '#') {
            return (<span ref={element => solidProps2.tooltipRef.current = element} class="block max-w-[420px] truncate text-gray-700 dark:text-gray-300" onMouseEnter={solidProps2.onMouseEnter} onMouseLeave={solidProps2.onMouseLeave}>
          {solidProps2.formatted.display}
        </span>);
        }
        return (<a href={href} target="_blank" rel="noopener noreferrer" class="text-blue-600 underline hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300" onClick={(e) => e.stopPropagation()}>
        <span ref={element => solidProps2.tooltipRef.current = element} class="block max-w-[420px] truncate" onMouseEnter={solidProps2.onMouseEnter} onMouseLeave={solidProps2.onMouseLeave}>
          {solidProps2.formatted.display}
        </span>
      </a>);
    },
    number: (solidProps3) => (<span ref={element => solidProps3.tooltipRef.current = element} class="block font-mono text-gray-700 dark:text-gray-300 tabular-nums" onMouseEnter={solidProps3.onMouseEnter} onMouseLeave={solidProps3.onMouseLeave}>
      {solidProps3.formatted.display}
    </span>),
    date: (solidProps4) => (<span ref={element => solidProps4.tooltipRef.current = element} class="block text-gray-700 dark:text-gray-300" onMouseEnter={solidProps4.onMouseEnter} onMouseLeave={solidProps4.onMouseLeave}>
      {solidProps4.formatted.display}
    </span>),
};
const defaultRenderer: CellRenderer = (solidProps5) => (<span ref={element => solidProps5.tooltipRef.current = element} class="block max-w-[420px] truncate text-gray-700 dark:text-gray-300" onMouseEnter={solidProps5.onMouseEnter} onMouseLeave={solidProps5.onMouseLeave}>
    {solidProps5.formatted.display}
  </span>);
export const DataTableCell = function DataTableCell(solidProps6: DataTableCellProps) {
    const solidState7 = useTranslation();
    const [copied, setCopied] = createSignal(false);
    const copyTimerRef = { current: undefined } as {
        current: (ReturnType<typeof setTimeout> | undefined);
    };
    const handleCopy = (e: MouseEvent) => {
        e.stopPropagation();
        if (!solidProps6.onCopy) {
            return;
        }
        const text = solidProps6.formatted.isNull ? '' : String(solidProps6.formatted.raw ?? solidProps6.formatted.display);
        solidProps6.onCopy?.(text);
        clearTimeout(copyTimerRef.current);
        setCopied(true);
        copyTimerRef.current = setTimeout(() => setCopied(false), 2000);
    };
    const [hovered, setHovered] = createSignal(false);
    const tooltipRef = { current: null } as {
        current: HTMLSpanElement | null;
    };
    const [tooltip, setTooltip] = createSignal<{
        x: number;
        y: number;
    } | null>(null);
    const tooltipTimerRef = { current: undefined } as {
        current: (ReturnType<typeof setTimeout> | undefined);
    };
    const handleMouseEnter = () => {
        const el = tooltipRef.current;
        if (!el || el.scrollWidth <= el.clientWidth) {
            return;
        }
        tooltipTimerRef.current = setTimeout(() => {
            const rect = el.getBoundingClientRect();
            setTooltip({
                x: rect.left + rect.width / 2,
                y: rect.bottom + 4,
            });
        }, 300);
    };
    const handleMouseLeave = () => {
        clearTimeout(tooltipTimerRef.current);
        setTooltip(null);
    };
    onMount(() => {
        const cleanup = untrack(() => {
            return () => {
                clearTimeout(tooltipTimerRef.current);
                clearTimeout(copyTimerRef.current);
            };
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    });
    const hasOverflow = solidProps6.formatted.type === 'string' || solidProps6.formatted.type === 'url' || solidProps6.formatted.type === 'date' || solidProps6.formatted.type === 'number';
    const tooltipContent = solidProps6.formatted.isNull ? null : (solidProps6.formatted.type === 'number' || solidProps6.formatted.type === 'date') ? String(solidProps6.formatted.raw) : solidProps6.formatted.display;
    const rendererProps = { get formatted() {
            return solidProps6.formatted;
        }, tooltipRef, onMouseEnter: handleMouseEnter, onMouseLeave: handleMouseLeave };
    const showCopyButton = createMemo(() => solidProps6.onCopy && (hovered() || copied()) && !solidProps6.formatted.isNull);
    return (<td class={`relative px-3 py-2 align-top ${solidProps6.alignment === 'right' ? 'text-right' : 'text-left'} ${solidProps6.isSticky ? solidProps6.stickyClassName ?? '' : ''}`} onMouseEnter={() => setHovered(true)} onMouseLeave={() => setHovered(false)}>
      {solidProps6.formatted.isNull ? (<span class="text-xs text-gray-400 dark:text-gray-500">&mdash;</span>) : ((cellRenderers[solidProps6.formatted.type] ?? defaultRenderer)(rendererProps))}
      {showCopyButton() && (<button type="button" onClick={handleCopy} class={`absolute top-1 right-1 cursor-pointer rounded p-0.5 transition-colors ${copied() ? 'text-green-600 dark:text-green-400'
                : 'text-gray-400 hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300'}`} aria-label={copied() ? solidState7.t('BiChat.Message.Copied') : solidState7.t('BiChat.DataTable.Copy')}>
          {copied() ? <Check size={14} weight="bold"/> : <Copy size={14}/>}
        </button>)}
      {tooltip() && hasOverflow && tooltipContent && <Portal mount={document.body}>{<div class="pointer-events-none fixed max-w-[400px] rounded-lg bg-gray-900 px-3 py-2 text-xs text-white shadow-lg break-words dark:bg-gray-700" style={{
                    "z-index": 100001,
                    "left": cssLength(Math.min(tooltip()!.x, window.innerWidth - 420)),
                    "top": cssLength(tooltip()!.y),
                    "transform": 'translateX(-50%)'
                }}>
          {tooltipContent}
        </div>}</Portal>}
    </td>);
};
