import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
import { CaretRight, XCircle } from '../icons';
import type { ActivityStep } from '../types';
import { useTranslation } from '../hooks/useTranslation';
import { getToolLabel } from '../utils/toolLabels';
import { groupSteps } from '../utils/activitySteps';
// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
function formatDuration(ms: number): string {
    return ms < 1000 ? `${ms}ms` : `${(ms / 1000).toFixed(1)}s`;
}
// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------
export interface ActivityTraceProps {
    thinkingContent: string;
    activeSteps: ActivityStep[];
    /** Consumer tool label prefix (e.g. 'Ali.Tools') for custom tool translations. */
    toolLabelPrefix?: string;
    className?: string;
}
// ---------------------------------------------------------------------------
// Main Component
// ---------------------------------------------------------------------------
function ActivityTraceInner(solidProps1Input: ActivityTraceProps) {
    const solidProps1 = mergeProps({ className: '' } as const, solidProps1Input);
    const solidState2 = useTranslation();
    // Hooks must be called unconditionally (React rules of hooks)
    const solidState3 = createMemo(() => groupSteps(solidProps1.activeSteps));
    const hasContent = createMemo(() => solidProps1.thinkingContent || solidProps1.activeSteps.length > 0);
    return <Show when={!(!hasContent())}>{_visible => {
            const hasActiveSteps = createMemo(() => solidProps1.activeSteps.some((s) => s.status === 'active'));
            return (<>
      <div role="status" aria-live="polite" class={`flex gap-3 ${solidProps1.className}`}>
        {/* AI avatar */}
        <div class="flex-shrink-0 w-8 h-8 rounded-full bg-primary-600 flex items-center justify-center text-white font-medium text-xs">
          AI
        </div>

        {/* Trace content */}
        <div class="flex-1 max-w-[85%] space-y-2">
          {/* Thinking bubble */}
          
            {solidProps1.thinkingContent && (<div class="overflow-hidden">
                <ThinkingBubble content={solidProps1.thinkingContent}/>
              </div>)}
          

          {/* Activity steps */}
          <div class="space-y-1">
            
              {solidState3().topLevel.map((step) => (<StepItem step={step} t={solidState2.t} toolLabelPrefix={solidProps1.toolLabelPrefix}/>))}
            

            {/* Agent groups (sub-agent tool calls) */}
            
              {solidState3().agentGroups.map(([agentName, steps]) => (<AgentGroup agentName={agentName} steps={steps} t={solidState2.t} toolLabelPrefix={solidProps1.toolLabelPrefix}/>))}
            

            {/* Typing shimmer while tools are running */}
            {hasActiveSteps() && !solidProps1.thinkingContent && (<div class="flex items-center gap-1.5 pt-0.5" aria-hidden="true">
                <span class="w-1.5 h-1.5 rounded-full bg-gray-400 dark:bg-gray-500 animate-bounce motion-reduce:animate-none [animation-delay:0ms]"/>
                <span class="w-1.5 h-1.5 rounded-full bg-gray-400 dark:bg-gray-500 animate-bounce motion-reduce:animate-none [animation-delay:150ms]"/>
                <span class="w-1.5 h-1.5 rounded-full bg-gray-400 dark:bg-gray-500 animate-bounce motion-reduce:animate-none [animation-delay:300ms]"/>
              </div>)}
          </div>
        </div>
      </div>
    </>);
        }}</Show>;
}
// ---------------------------------------------------------------------------
// ThinkingBubble
// ---------------------------------------------------------------------------
function ThinkingBubble(solidProps4: {
    content: string;
}) {
    // Show last 2 lines of thinking, truncated
    const truncated = createMemo(() => {
        const lines = solidProps4.content.trim().split('\n');
        const last = lines.slice(-2).join('\n');
        return last.length > 200 ? '…' + last.slice(-200) : last;
    });
    return (<div class="flex items-start gap-2 text-sm text-gray-500 dark:text-gray-400">
      <span class="inline-block w-2 h-2 mt-1.5 rounded-full bg-primary-400 animate-pulse shrink-0"/>
      <div class="min-w-0 overflow-hidden">
        <span class="font-medium bichat-thinking-shimmer">
          {truncated()}
        </span>
      </div>
    </div>);
}
// ---------------------------------------------------------------------------
// StepItem
// ---------------------------------------------------------------------------
interface StepItemProps {
    step: ActivityStep;
    t: (key: string, params?: Record<string, string | number | boolean>) => string;
    toolLabelPrefix?: string;
}
function StepItem(solidProps5: StepItemProps) {
    const label = createMemo(() => solidProps5.step.type === 'thinking'
        ? solidProps5.t('BiChat.Thinking.Thinking')
        : getToolLabel(solidProps5.t, solidProps5.step.toolName, solidProps5.step.arguments, solidProps5.toolLabelPrefix));
    const isCompleted = solidProps5.step.status === 'completed';
    const isFailed = solidProps5.step.status === 'failed';
    const duration = solidProps5.step.durationMs != null ? formatDuration(solidProps5.step.durationMs) : null;
    return (<div class={`flex items-center gap-2 text-sm transition-colors duration-200 ${isFailed
            ? 'text-red-500 dark:text-red-400'
            : isCompleted
                ? 'text-gray-400 dark:text-gray-500'
                : 'text-gray-600 dark:text-gray-300'}`} aria-label={`${label()}, ${isFailed ? 'failed' : isCompleted ? 'completed' : 'in progress'}`}>
      <StatusIndicator status={solidProps5.step.status}/>
      <span class={isCompleted || isFailed ? '' : 'font-medium'}>{label()}</span>
      {solidProps5.step.error && (<span class="text-xs text-red-400 truncate max-w-[200px]" title={solidProps5.step.error}>
          {solidProps5.step.error}
        </span>)}
      {duration && (<span class="text-xs text-gray-400 dark:text-gray-500 tabular-nums">
          {duration}
        </span>)}
    </div>);
}
// ---------------------------------------------------------------------------
// AgentGroup
// ---------------------------------------------------------------------------
interface AgentGroupProps {
    agentName: string;
    steps: ActivityStep[];
    t: (key: string, params?: Record<string, string | number | boolean>) => string;
    toolLabelPrefix?: string;
}
function AgentGroup(solidProps6: AgentGroupProps) {
    const [collapsed, setCollapsed] = createSignal(false);
    const completedCount = solidProps6.steps.filter((s) => s.status === 'completed').length;
    return (<div class="ml-4 pl-3 border-l-2 border-primary-200 dark:border-primary-800 space-y-1">
      <button type="button" onClick={() => setCollapsed((c) => !c)} class="flex items-center gap-1 text-xs font-medium text-primary-600 dark:text-primary-400 cursor-pointer hover:text-primary-700 dark:hover:text-primary-300 transition-colors" aria-expanded={!collapsed()}>
        <CaretRight size={12} weight="bold" className={`shrink-0 transition-transform duration-150 ${collapsed() ? '' : 'rotate-90'}`}/>
        <span>
          {solidProps6.agentName}
          {collapsed() && (<span class="ml-1 text-gray-400 dark:text-gray-500">
              ({completedCount}/{solidProps6.steps.length})
            </span>)}
        </span>
      </button>
      {!collapsed() && (<>
          {solidProps6.steps.map((step) => (<StepItem step={step} t={solidProps6.t} toolLabelPrefix={solidProps6.toolLabelPrefix}/>))}
        </>)}
    </div>);
}
// ---------------------------------------------------------------------------
// StatusIndicator
// ---------------------------------------------------------------------------
function StatusIndicator(solidProps7: {
    status: ActivityStep['status'];
}) {
    return <>{createMemo(() => {
            if (solidProps7.status === 'failed') {
                return (<div class="shrink-0">
        <XCircle size={14} weight="fill" className="text-red-500 dark:text-red-400"/>
      </div>);
            }
            if (solidProps7.status === 'completed') {
                return (<svg class="w-3.5 h-3.5 text-green-500 dark:text-green-400 shrink-0" viewBox="0 0 16 16" fill="none">
        <path d="M3 8.5L6.5 12L13 4" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
      </svg>);
            }
            return (<span class="relative flex h-3.5 w-3.5 shrink-0 items-center justify-center">
      <span class="absolute inline-flex h-full w-full animate-ping rounded-full bg-primary-400 opacity-50 motion-reduce:animate-none"/>
      <span class="relative inline-flex h-2 w-2 rounded-full bg-primary-500"/>
    </span>);
        })}</>;
}
// ---------------------------------------------------------------------------
// Export
// ---------------------------------------------------------------------------
export const ActivityTrace = ActivityTraceInner;
ActivityTrace; /* Solid components are named by their declarations. */
