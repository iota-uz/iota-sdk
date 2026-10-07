import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
interface DateGroupHeaderProps {
    groupName: string;
    count: number;
}
/**
 * Sticky header for date-based session groups
 * Displays group name and session count
 */
export default function DateGroupHeader(solidProps1: DateGroupHeaderProps) {
    return (<div class="sticky top-0 bg-surface-300 dark:bg-gray-900 px-4 py-2 text-sm font-medium z-10 border-b border-gray-100 dark:border-gray-800">
      <div class="flex items-center justify-between">
        <span class="text-gray-700 dark:text-gray-300 font-semibold">{solidProps1.groupName}</span>
        <span class="text-xs text-gray-500 dark:text-gray-400 bg-gray-100 dark:bg-gray-800 px-2 py-0.5 rounded-full">
          {solidProps1.count}
        </span>
      </div>
    </div>);
}
