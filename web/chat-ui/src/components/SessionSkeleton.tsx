import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
interface SessionSkeletonProps {
    count?: number;
}
export default function SessionSkeleton(solidProps1Input: SessionSkeletonProps) {
    const solidProps1 = mergeProps({ count: 5 } as const, solidProps1Input);
    return (<div class="space-y-1 px-2">
      {Array.from({ length: solidProps1.count }).map((_, index) => (<div class="animate-pulse px-3 py-2 rounded-lg">
          <div class="flex items-center gap-2">
            {/* Icon placeholder */}
            <div class="w-5 h-5 bg-gray-300 dark:bg-gray-700 rounded"/>
            {/* Text placeholder */}
            <div class="flex-1">
              <div class="h-4 bg-gray-300 dark:bg-gray-700 rounded w-3/4"/>
            </div>
          </div>
        </div>))}
    </div>);
}
