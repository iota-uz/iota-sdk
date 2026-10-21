import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
interface CompactionDoodleProps {
    title: string;
    subtitle: string;
}
export function CompactionDoodle(solidProps1: CompactionDoodleProps) {
    return (<div class="flex items-center gap-2.5 rounded-xl border border-gray-200/70 bg-white/95 px-3.5 py-2 shadow-sm backdrop-blur-sm dark:border-gray-700/50 dark:bg-gray-800/95">
      <div class="relative flex h-5 w-5 items-center justify-center">
        <span class="absolute inline-flex h-full w-full animate-ping rounded-full bg-primary-400/30"/>
        <span class="relative inline-flex h-2 w-2 rounded-full bg-primary-500"/>
      </div>
      <div class="flex items-baseline gap-1.5">
        <span class="text-xs font-medium text-gray-700 dark:text-gray-200">{solidProps1.title}</span>
        <span class="text-[11px] text-gray-400 dark:text-gray-500">{solidProps1.subtitle}</span>
      </div>
    </div>);
}
export default CompactionDoodle;
