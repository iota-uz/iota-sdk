import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * DateSeparator Component
 * Renders a centered date label with horizontal divider lines.
 * Shows "Today", "Yesterday", or a formatted date for older messages.
 */
import { isToday, isYesterday, format } from 'date-fns';
import { useTranslation } from '../hooks/useTranslation';
interface DateSeparatorProps {
    date: Date;
}
export function DateSeparator(solidProps1: DateSeparatorProps) {
    const solidState2 = useTranslation();
    const label = createMemo(() => {
        if (isToday(solidProps1.date)) {
            return solidState2.t('BiChat.DateGroup.Today');
        }
        if (isYesterday(solidProps1.date)) {
            return solidState2.t('BiChat.DateGroup.Yesterday');
        }
        return format(solidProps1.date, 'MMM d');
    });
    return (<div class="flex items-center gap-3 py-2 select-none" aria-label={label()}>
      <div class="h-px flex-1 bg-gray-200 dark:bg-gray-700"/>
      <span class="text-[11px] font-medium text-gray-400 dark:text-gray-500 uppercase tracking-wide">
        {label()}
      </span>
      <div class="h-px flex-1 bg-gray-200 dark:bg-gray-700"/>
    </div>);
}
export default DateSeparator;
