import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * SourcesPanel component
 * Grok-inspired collapsible citations panel.
 * Collapsed: compact pill with overlapping domain circles + count.
 * Expanded: card panel with source titles, excerpts, and domain badges.
 */
import { X } from '../icons';
import type { Citation } from '../types';
import { useTranslation } from '../hooks/useTranslation';
interface SourcesPanelProps {
    citations: Citation[];
}
/* ── Helpers ─────────────────────────────────────────────────────────────── */
function extractDomain(url: string): string {
    try {
        return new URL(url).hostname.replace(/^www\./, '');
    }
    catch {
        return '';
    }
}
const PALETTE = [
    '#c0392b', '#d35400', '#f39c12', '#27ae60',
    '#16a085', '#2980b9', '#8e44ad', '#d63384',
];
function domainColor(domain: string): string {
    let h = 0;
    for (let i = 0; i < domain.length; i++) {
        h = domain.charCodeAt(i) + ((h << 5) - h);
    }
    return PALETTE[Math.abs(h) % PALETTE.length];
}
/* ── Component ───────────────────────────────────────────────────────────── */
export function SourcesPanel(solidProps1: SourcesPanelProps) {
    const solidState2 = useTranslation();
    const [isOpen, setIsOpen] = createSignal(false);
    const open = () => setIsOpen(true);
    const close = () => setIsOpen(false);
    return <Show when={!(!solidProps1.citations?.length)}>{_visible => {
            const domains = [...new Set(solidProps1.citations.filter(c => c.url).map(c => extractDomain(c.url)).filter(Boolean))];
            const previewDomains = domains.slice(0, 5);
            /* ── Collapsed pill ─────────────────────────────────────────────────── */
            if (!isOpen()) {
                return (<div class="mt-3">
        <button type="button" onClick={open} class="cursor-pointer inline-flex items-center gap-2 rounded-full px-3 py-1.5
            bg-gray-50 hover:bg-gray-100 dark:bg-gray-700/50 dark:hover:bg-gray-600/60
            border border-gray-200/70 dark:border-gray-600/40
            transition-colors duration-150
            focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--bichat-primary,theme(colors.blue.500))]/40">
          {previewDomains.length > 0 && (<span class="flex -space-x-1.5">
              {previewDomains.map((domain, i) => (<span class="relative w-5 h-5 rounded-full flex items-center justify-center text-[8px] font-bold text-white
                    ring-2 ring-white dark:ring-gray-800 select-none" style={{ "background-color": domainColor(domain), "z-index": previewDomains.length - i }} aria-hidden="true">
                  {domain[0]?.toUpperCase()}
                </span>))}
            </span>)}
          <span class="text-xs font-medium text-gray-600 dark:text-gray-300 tabular-nums">
            {solidProps1.citations.length} {solidState2.t(solidProps1.citations.length === 1 ? 'BiChat.Sources.Source' : 'BiChat.Sources.Sources')}
          </span>
        </button>
      </div>);
            }
            /* ── Expanded panel ─────────────────────────────────────────────────── */
            return (<div class="mt-3 rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800/90 shadow-sm overflow-hidden">
      {/* Header */}
      <div class="flex items-center justify-between px-4 py-3">
        <span class="text-sm font-semibold text-gray-900 dark:text-gray-100">
          {solidState2.t('BiChat.Sources.Title')}
        </span>
        <button type="button" onClick={close} class="cursor-pointer flex items-center justify-center w-7 h-7 rounded-full
            text-gray-400 hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300
            hover:bg-gray-100 dark:hover:bg-gray-700
            transition-colors duration-150
            focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--bichat-primary)]/40" aria-label={solidState2.t('BiChat.Sources.Close')}>
          <X size={14} weight="bold"/>
        </button>
      </div>

      {/* Source list */}
      <div class="max-h-80 overflow-y-auto">
        {solidProps1.citations.map((citation, index) => {
                    const domain = citation.url ? extractDomain(citation.url) : '';
                    const cardContent = createMemo(() => (<>
              <h4 class="text-sm font-medium leading-snug text-[var(--bichat-color-accent,theme(colors.blue.600))] dark:text-blue-400">
                {citation.title || solidState2.t('BiChat.Sources.SourceN', { n: String(index + 1) })}
              </h4>
              {citation.excerpt && (<p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400 line-clamp-2 leading-relaxed">
                  {citation.excerpt}
                </p>)}
              {domain && (<div class="flex items-center gap-1.5 mt-1.5">
                  <span class="w-4 h-4 rounded-full flex items-center justify-center text-[7px] font-bold text-white flex-shrink-0 select-none" style={{ "background-color": domainColor(domain) }} aria-hidden="true">
                    {domain[0]?.toUpperCase()}
                  </span>
                  <span class="text-[11px] text-gray-400 dark:text-gray-500 truncate">
                    {domain}
                  </span>
                </div>)}
            </>));
                    const cardClass = 'block px-4 py-3 border-t border-gray-100 dark:border-gray-700/50';
                    if (citation.url) {
                        return (<a href={citation.url} target="_blank" rel="noopener noreferrer" class={`${cardClass} hover:bg-gray-50 dark:hover:bg-gray-700/40 transition-colors duration-100
                  focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-[var(--bichat-primary)]/40`}>
                {cardContent()}
              </a>);
                    }
                    return (<div class={cardClass}>
              {cardContent()}
            </div>);
                })}
      </div>
    </div>);
        }}</Show>;
}
