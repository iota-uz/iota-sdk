import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * DebugPanel Component
 * Debug trace viewer with metric chips, expandable tool calls,
 * and terminal-inspired code blocks.
 *
 * Debug UI is English-only (developer-facing) — no i18n.
 */
import { Bug, Timer, Lightning, Wrench, CaretDown, CheckCircle, XCircle, Copy, Check, CircleNotch, ArrowUp, ArrowDown, Stack, Database, ArrowSquareOut, } from '../icons';
import type { DebugTrace, StreamToolPayload } from '../types';
import { hasMeaningfulUsage, hasDebugTrace } from '../utils/debugTrace';
import { calculateCompletionTokensPerSecond, formatDuration, formatGenerationDuration, } from '../utils/debugMetrics';
export interface DebugPanelProps {
    trace?: DebugTrace;
}
// ─── CopyPill ───────────────────────────────────────────────
interface UseCopyFeedbackResult {
    copied: boolean;
    copy: (text: string) => Promise<void>;
}
function useCopyFeedback(): UseCopyFeedbackResult {
    const [copied, setCopied] = createSignal(false);
    const timerRef = { current: null } as {
        current: (ReturnType<typeof setTimeout> | number | null) | null;
    };
    const copy = async (text: string) => {
        try {
            await navigator.clipboard.writeText(text);
            setCopied(true);
            if (timerRef.current !== null) {
                window.clearTimeout(timerRef.current);
            }
            timerRef.current = window.setTimeout(() => {
                setCopied(false);
                timerRef.current = null;
            }, 2000);
        }
        catch (err) {
            console.error('Copy failed:', err);
        }
    };
    onMount(() => {
        const cleanup = untrack(() => () => {
            if (timerRef.current !== null) {
                window.clearTimeout(timerRef.current);
            }
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    });
    return { get copied() {
            return copied();
        }, copy };
}
function CopyPill(solidProps1: {
    text: string;
}) {
    const solidState2 = useCopyFeedback();
    const handleCopy = async (e: MouseEvent) => {
        e.stopPropagation();
        await solidState2.copy(solidProps1.text);
    };
    return (<button onClick={handleCopy} class={[
            'flex items-center gap-1 px-2 py-0.5 rounded-md text-[10px] font-medium',
            'transition-all duration-200',
            solidState2.copied ? 'bg-emerald-100 dark:bg-emerald-900/30 text-emerald-600 dark:text-emerald-400'
                : 'text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700/50',
        ].join(' ')}>
      {solidState2.copied ? <Check size={10} weight="bold"/> : <Copy size={10}/>}
      <span>{solidState2.copied ? 'Copied!' : 'Copy'}</span>
    </button>);
}
// ─── InlineCopyButton ────────────────────────────────────────
function InlineCopyButton(solidProps3: {
    text: string;
}) {
    const solidState4 = useCopyFeedback();
    const handleCopy = async (e: MouseEvent) => {
        e.stopPropagation();
        await solidState4.copy(solidProps3.text);
    };
    return (<button onClick={handleCopy} aria-label={solidState4.copied ? 'Copied' : 'Copy'} class={[
            'flex-shrink-0 p-1 rounded transition-colors duration-150',
            solidState4.copied ? 'text-emerald-500 dark:text-emerald-400'
                : 'text-gray-300 dark:text-gray-600 hover:text-gray-500 dark:hover:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700/40',
        ].join(' ')}>
      {solidState4.copied ? <Check size={11} weight="bold"/> : <Copy size={11}/>}
    </button>);
}
// ─── MetricChip ─────────────────────────────────────────────
interface MetricChipProps {
    icon: JSX.Element;
    value: string;
    label: string;
}
function MetricChip(solidProps5: MetricChipProps) {
    return (<span class="inline-flex items-center gap-1.5 px-2 py-1 rounded-md bg-gray-50 dark:bg-gray-800/40 text-[11px] tabular-nums">
      {solidProps5.icon}
      <span class="font-mono font-medium text-gray-700 dark:text-gray-300">{solidProps5.value}</span>
      <span class="text-gray-400 dark:text-gray-500">{solidProps5.label}</span>
    </span>);
}
// ─── ToolCard ───────────────────────────────────────────────
function ToolCard(solidProps6: {
    tool: StreamToolPayload;
}) {
    const [expanded, setExpanded] = createSignal(false);
    const hasResult = !!solidProps6.tool.result && !solidProps6.tool.error;
    const hasError = !!solidProps6.tool.error;
    const status = hasError
        ? {
            icon: <XCircle size={12} weight="fill"/>,
            pillBg: 'bg-red-50 dark:bg-red-950/30',
            pillText: 'text-red-500 dark:text-red-400',
            borderColor: 'border-l-red-400 dark:border-l-red-500',
        }
        : hasResult
            ? {
                icon: <CheckCircle size={12} weight="fill"/>,
                pillBg: 'bg-emerald-50 dark:bg-emerald-950/30',
                pillText: 'text-emerald-500 dark:text-emerald-400',
                borderColor: 'border-l-emerald-400 dark:border-l-emerald-500',
            }
            : {
                icon: <CircleNotch size={12} weight="bold" className="animate-spin"/>,
                pillBg: 'bg-gray-100 dark:bg-gray-800',
                pillText: 'text-gray-400 dark:text-gray-500',
                borderColor: 'border-l-gray-300 dark:border-l-gray-600',
            };
    return (<div class={[
            'rounded-lg overflow-hidden',
            'border border-gray-200/60 dark:border-gray-700/40',
            'border-l-2', status.borderColor,
            'bg-white dark:bg-gray-800/50',
            'transition-all duration-150',
        ].join(' ')}>
      {/* Header */}
      <button onClick={() => setExpanded(!expanded())} aria-expanded={expanded()} aria-label={`${solidProps6.tool.name} — ${hasError ? 'error' : hasResult ? 'success' : 'pending'}`} class="w-full flex items-center gap-2.5 px-3 py-2.5 hover:bg-gray-50/60 dark:hover:bg-gray-700/20 transition-colors cursor-pointer">
        {/* Status pill */}
        <span class={`flex items-center justify-center w-5 h-5 rounded-full ${status.pillBg} ${status.pillText}`}>
          {status.icon}
        </span>

        {/* Tool name */}
        <span class="flex-1 min-w-0 text-left font-mono text-xs font-medium text-gray-800 dark:text-gray-200 truncate">
          {solidProps6.tool.name}
        </span>

        {/* Duration chip */}
        {solidProps6.tool.durationMs !== undefined && (<span class="flex-shrink-0 px-2 py-0.5 rounded-full bg-gray-100 dark:bg-gray-700/60 text-[10px] font-mono font-medium text-gray-500 dark:text-gray-400 tabular-nums">
            {formatDuration(solidProps6.tool.durationMs)}
          </span>)}

        {/* Chevron */}
        <CaretDown size={12} weight="bold" className={[
            'flex-shrink-0 text-gray-300 dark:text-gray-600',
            'transition-transform duration-200',
            expanded() ? 'rotate-180' : '',
        ].join(' ')}/>
      </button>

      {/* Expandable content — CSS grid animation for smooth height */}
      <div class={[
            'grid transition-[grid-template-rows] duration-200 ease-out',
            expanded() ? 'grid-rows-[1fr]' : 'grid-rows-[0fr]',
        ].join(' ')}>
        <div class="overflow-hidden min-h-0">
          <div class="px-3 pb-3 pt-1 space-y-2">
            {/* Arguments — terminal-style dark code block */}
            {solidProps6.tool.arguments && (<div class="rounded-lg bg-[#1a1b26] dark:bg-gray-950 overflow-hidden ring-1 ring-gray-800/10 dark:ring-white/5">
                <div class="flex items-center justify-between px-3 py-1.5 bg-[#1e1f2e] dark:bg-gray-900/80 border-b border-white/5">
                  <span class="text-[10px] uppercase tracking-wider font-medium text-gray-500">
                    Arguments
                  </span>
                  <CopyPill text={solidProps6.tool.arguments}/>
                </div>
                <pre class="p-3 text-[11px] font-mono text-gray-300 overflow-x-auto max-h-60 overflow-y-auto whitespace-pre-wrap break-all leading-relaxed">
                  {solidProps6.tool.arguments}
                </pre>
              </div>)}

            {/* Result — terminal-style dark code block */}
            {solidProps6.tool.result && (<div class="rounded-lg bg-[#1a1b26] dark:bg-gray-950 overflow-hidden ring-1 ring-gray-800/10 dark:ring-white/5">
                <div class="flex items-center justify-between px-3 py-1.5 bg-[#1e1f2e] dark:bg-gray-900/80 border-b border-white/5">
                  <span class="text-[10px] uppercase tracking-wider font-medium text-gray-500">
                    Result
                  </span>
                  <CopyPill text={solidProps6.tool.result}/>
                </div>
                <pre class="p-3 text-[11px] font-mono text-gray-300 overflow-x-auto max-h-60 overflow-y-auto whitespace-pre-wrap break-all leading-relaxed">
                  {solidProps6.tool.result}
                </pre>
              </div>)}

            {/* Error — red-tinted dark block */}
            {solidProps6.tool.error && (<div class="rounded-lg bg-red-950/80 dark:bg-red-950/40 overflow-hidden ring-1 ring-red-800/20">
                <div class="px-3 py-1.5 border-b border-red-800/20">
                  <span class="text-[10px] uppercase tracking-wider font-medium text-red-400">
                    Error
                  </span>
                </div>
                <pre class="p-3 text-[11px] font-mono text-red-300 overflow-x-auto whitespace-pre-wrap break-all leading-relaxed">
                  {solidProps6.tool.error}
                </pre>
              </div>)}
          </div>
        </div>
      </div>
    </div>);
}
// ─── DebugPanel ─────────────────────────────────────────────
export function DebugPanel(solidProps7: DebugPanelProps) {
    const hasData = !!solidProps7.trace && hasDebugTrace(solidProps7.trace);
    const traceID = solidProps7.trace?.traceId?.trim() || '';
    const traceURL = solidProps7.trace?.traceUrl?.trim() || '';
    const safeTraceURL = (() => {
        if (!traceURL) {
            return '';
        }
        try {
            const parsed = new URL(traceURL);
            if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
                return '';
            }
            return parsed.toString();
        }
        catch {
            return '';
        }
    })();
    const tokensPerSecond = calculateCompletionTokensPerSecond(solidProps7.trace?.usage, solidProps7.trace?.generationMs);
    // Build metric list from available data
    const metrics: MetricChipProps[] = [];
    if (hasData && solidProps7.trace) {
        if (solidProps7.trace.generationMs !== undefined) {
            metrics.push({
                icon: <Timer size={12} weight="duotone" className="text-amber-500 dark:text-amber-400"/>,
                value: formatGenerationDuration(solidProps7.trace.generationMs),
                label: 'generation',
            });
        }
        if (tokensPerSecond !== null) {
            metrics.push({
                icon: <Lightning size={12} weight="fill" className="text-orange-500 dark:text-orange-400"/>,
                value: `${tokensPerSecond.toFixed(1)}/s`,
                label: 'tok/s',
            });
        }
        if (hasMeaningfulUsage(solidProps7.trace.usage) && solidProps7.trace.usage) {
            metrics.push({
                icon: <Stack size={12} weight="duotone" className="text-violet-500 dark:text-violet-400"/>,
                value: solidProps7.trace.usage.totalTokens.toLocaleString(),
                label: 'total',
            }, {
                icon: <ArrowUp size={12} weight="bold" className="text-blue-500 dark:text-blue-400"/>,
                value: solidProps7.trace.usage.promptTokens.toLocaleString(),
                label: 'prompt',
            }, {
                icon: <ArrowDown size={12} weight="bold" className="text-indigo-500 dark:text-indigo-400"/>,
                value: solidProps7.trace.usage.completionTokens.toLocaleString(),
                label: 'completion',
            });
            if (solidProps7.trace.usage.cachedTokens !== undefined && solidProps7.trace.usage.cachedTokens > 0) {
                metrics.push({
                    icon: <Database size={12} weight="duotone" className="text-pink-500 dark:text-pink-400"/>,
                    value: solidProps7.trace.usage.cachedTokens.toLocaleString(),
                    label: 'cached',
                });
            }
        }
    }
    return (<div class="mt-4 pt-4 border-t border-gray-100 dark:border-gray-700/50">
      {/* Header */}
      <div class="flex items-center justify-between mb-4">
        <div class="flex items-center gap-2.5">
          <div class="flex items-center justify-center w-6 h-6 rounded-lg bg-gray-100 dark:bg-gray-800">
            <Bug size={14} weight="duotone" className="text-gray-500 dark:text-gray-400"/>
          </div>
          <h3 class="text-[11px] uppercase tracking-widest font-semibold text-gray-400 dark:text-gray-500">
            Debug
          </h3>
        </div>
        {hasData && solidProps7.trace && (<CopyPill text={JSON.stringify(solidProps7.trace, null, 2)}/>)}
      </div>

      {hasData && solidProps7.trace ? (<div class="space-y-4">
          {(traceID || solidProps7.trace.sessionId) && (<div class="rounded-lg border border-gray-200/60 dark:border-gray-700/40 bg-gray-50/50 dark:bg-gray-800/40 px-3 py-2 space-y-1.5">
              {traceID && (<div class="flex items-center gap-2 min-w-0">
                  <span class="flex-shrink-0 text-[10px] uppercase tracking-wider text-gray-400 dark:text-gray-500 w-14">Trace</span>
                  <span class="flex-1 min-w-0 font-mono text-[11px] text-gray-700 dark:text-gray-300 truncate" title={traceID}>{traceID}</span>
                  <InlineCopyButton text={traceID}/>
                </div>)}
              {solidProps7.trace.sessionId && (<div class="flex items-center gap-2 min-w-0">
                  <span class="flex-shrink-0 text-[10px] uppercase tracking-wider text-gray-400 dark:text-gray-500 w-14">Session</span>
                  <span class="flex-1 min-w-0 font-mono text-[11px] text-gray-700 dark:text-gray-300 truncate" title={solidProps7.trace.sessionId}>{solidProps7.trace.sessionId}</span>
                  <InlineCopyButton text={solidProps7.trace.sessionId}/>
                </div>)}
              {safeTraceURL && (<div class="flex items-center gap-2 min-w-0 pt-0.5">
                  <span class="flex-shrink-0 w-14"/>
                  <a href={safeTraceURL} target="_blank" rel="noopener noreferrer" aria-label="Open in Langfuse" class="inline-flex items-center gap-1.5 text-[11px] font-medium text-blue-500 hover:text-blue-600 dark:text-blue-400 dark:hover:text-blue-300 transition-colors duration-150">
                    <ArrowSquareOut size={11} weight="bold"/>
                    <span>Open in Langfuse</span>
                  </a>
                </div>)}
            </div>)}

          {(solidProps7.trace.thinking || solidProps7.trace.observationReason) && (<div class="rounded-lg border border-gray-200/60 dark:border-gray-700/40 bg-gray-50/50 dark:bg-gray-800/40 p-3 space-y-2">
              {solidProps7.trace.observationReason && (<div class="text-[11px] text-amber-700 dark:text-amber-300">
                  Observation: <span class="font-mono">{solidProps7.trace.observationReason}</span>
                </div>)}
              {solidProps7.trace.thinking && (<div>
                  <div class="text-[10px] uppercase tracking-wider text-gray-500 dark:text-gray-400 mb-1">Reasoning</div>
                  <pre class="p-2 rounded bg-gray-100 dark:bg-gray-900 text-[11px] whitespace-pre-wrap break-words text-gray-700 dark:text-gray-200">
                    {solidProps7.trace.thinking}
                  </pre>
                </div>)}
            </div>)}

          {((solidProps7.trace.attempts && solidProps7.trace.attempts.length > 0) ||
                (solidProps7.trace.spans && solidProps7.trace.spans.length > 0) ||
                (solidProps7.trace.events && solidProps7.trace.events.length > 0)) && (<div class="rounded-lg border border-gray-200/60 dark:border-gray-700/40 bg-gray-50/50 dark:bg-gray-800/40 p-3 space-y-2">
              <div class="text-[10px] uppercase tracking-wider text-gray-500 dark:text-gray-400">
                Trace Graph
              </div>
              <div class="flex flex-wrap gap-1.5">
                {!!solidProps7.trace.attempts?.length && (<span class="px-1.5 py-0.5 rounded bg-gray-100 dark:bg-gray-900 text-[10px] text-gray-700 dark:text-gray-300">
                    attempts: {solidProps7.trace.attempts.length}
                  </span>)}
                {!!solidProps7.trace.spans?.length && (<span class="px-1.5 py-0.5 rounded bg-gray-100 dark:bg-gray-900 text-[10px] text-gray-700 dark:text-gray-300">
                    spans: {solidProps7.trace.spans.length}
                  </span>)}
                {!!solidProps7.trace.events?.length && (<span class="px-1.5 py-0.5 rounded bg-gray-100 dark:bg-gray-900 text-[10px] text-gray-700 dark:text-gray-300">
                    events: {solidProps7.trace.events.length}
                  </span>)}
              </div>
              {solidProps7.trace.attempts?.[solidProps7.trace.attempts.length - 1] && (<div class="text-[11px] text-gray-700 dark:text-gray-300">
                  {[
                        solidProps7.trace.attempts[solidProps7.trace.attempts.length - 1].model,
                        solidProps7.trace.attempts[solidProps7.trace.attempts.length - 1].provider,
                        solidProps7.trace.attempts[solidProps7.trace.attempts.length - 1].finishReason,
                    ]
                        .filter(Boolean)
                        .join(' • ')}
                </div>)}
            </div>)}

          {/* Metric chips */}
          {metrics.length > 0 && (<div class="flex flex-wrap gap-1.5">
              {metrics.map((m, i) => (<MetricChip {...m}/>))}
            </div>)}

          {/* Tool calls */}
          {solidProps7.trace.tools.length > 0 && (<div>
              <div class="flex items-center gap-2 mb-2.5">
                <Wrench size={13} weight="duotone" className="text-gray-400 dark:text-gray-500"/>
                <span class="text-[11px] font-medium text-gray-500 dark:text-gray-400">
                  Tool Calls
                </span>
                <span class="px-1.5 py-0.5 rounded-full bg-gray-100 dark:bg-gray-800 text-[10px] font-mono font-medium text-gray-500 dark:text-gray-400 tabular-nums">
                  {solidProps7.trace.tools.length}
                </span>
              </div>
              <div class="space-y-1.5">
                {solidProps7.trace.tools.map((tool, idx) => (<ToolCard tool={tool}/>))}
              </div>
            </div>)}

        </div>) : (<p class="text-xs text-gray-400 dark:text-gray-500 italic">
          Debug info unavailable
        </p>)}
    </div>);
}
export default DebugPanel;
