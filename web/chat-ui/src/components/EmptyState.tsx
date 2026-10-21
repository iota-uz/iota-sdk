import { createReducedMotion } from '../hooks/createReducedMotion';
import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
import { fadeInVariants } from '../animations/variants';
export interface EmptyStateProps {
    /** Optional icon to display */
    icon?: JSX.Element;
    /** Main title text */
    title: string;
    /** Optional description text */
    description?: string;
    /** Optional action element (button, link, etc.) */
    action?: JSX.Element;
    /** Additional CSS classes */
    className?: string;
    /** Size variant */
    size?: 'sm' | 'md' | 'lg';
}
const sizeClasses = {
    sm: {
        container: 'py-6 px-3',
        title: 'text-sm',
        description: 'text-xs',
    },
    md: {
        container: 'py-8 px-4',
        title: 'text-base',
        description: 'text-sm',
    },
    lg: {
        container: 'py-12 px-6',
        title: 'text-lg',
        description: 'text-base',
    },
};
function EmptyState(solidProps1Input: EmptyStateProps) {
    const solidProps1 = mergeProps({ className: '', size: 'md' } as const, solidProps1Input);
    const sizes = createMemo(() => sizeClasses[solidProps1.size]);
    const prefersReducedMotion = createReducedMotion();
    const duration = prefersReducedMotion() ? 0 : 0.4;
    return (<div class={`flex items-center justify-center ${sizes().container} ${solidProps1.className}`}>
      <div class="text-center max-w-md">
        {/* Icon */}
        {solidProps1.icon && (<div class="mb-4 flex justify-center">
            {solidProps1.icon}
          </div>)}

        {/* Title */}
        <h3 class={`${sizes().title} font-medium text-gray-900 dark:text-white mb-2`}>
          {solidProps1.title}
        </h3>

        {/* Description */}
        {solidProps1.description && (<p class={`${sizes().description} text-gray-500 dark:text-gray-400 mb-4`}>
            {solidProps1.description}
          </p>)}

        {/* Action */}
        {solidProps1.action && (<div>
            {solidProps1.action}
          </div>)}
      </div>
    </div>);
}
const MemoizedEmptyState = EmptyState;
MemoizedEmptyState; /* Solid components are named by their declarations. */
export { MemoizedEmptyState as EmptyState };
export default MemoizedEmptyState;
