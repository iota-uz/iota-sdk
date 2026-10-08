import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
import { ArrowClockwise, Warning } from '../icons';
import { useTranslation } from '../hooks/useTranslation';
interface RetryActionAreaProps {
    /** Callback when retry button is clicked */
    onRetry: () => void;
}
export const RetryActionArea = function RetryActionArea(solidProps1: RetryActionAreaProps) {
    const solidState2 = useTranslation();
    return (<div class="flex justify-start">
      <div class="flex flex-col gap-2.5 px-4 py-3 rounded-xl border border-gray-200 dark:border-gray-700" role="status" aria-live="polite">
        <div class="flex items-center gap-2.5">
          <Warning className="w-4 h-4 text-amber-500 dark:text-amber-400 flex-shrink-0" weight="fill"/>
          <span class="text-sm text-gray-500 dark:text-gray-400">
            {solidState2.t('BiChat.Retry.Subtitle')}
          </span>
        </div>
        <button type="button" onClick={solidProps1.onRetry} class="self-start inline-flex items-center gap-1.5 px-2.5 py-1 text-sm font-medium text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50">
          <ArrowClockwise size={14}/>
          {solidState2.t('BiChat.Retry.Button')}
        </button>
      </div>
    </div>);
};
export default RetryActionArea;
