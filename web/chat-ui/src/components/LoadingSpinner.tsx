import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * LoadingSpinner Component
 * Displays animated loading indicators
 */
import { CircleNotch } from '../icons';
type SpinnerVariant = 'spinner' | 'dots' | 'pulse';
interface LoadingSpinnerProps {
    variant?: SpinnerVariant;
    size?: 'sm' | 'md' | 'lg';
    message?: string;
}
function SpinnerLoader(solidProps1Input: {
    size: 'sm' | 'md' | 'lg';
    message?: string;
}) {
    const solidProps1 = mergeProps({ size: 'md' } as const, solidProps1Input);
    const sizeMap = {
        sm: 16,
        md: 32,
        lg: 48,
    };
    const sizeClasses = {
        sm: 'h-4 w-4',
        md: 'h-8 w-8',
        lg: 'h-12 w-12',
    };
    return (<div class="flex flex-col items-center justify-center" role="status" aria-live="polite">
      <CircleNotch size={sizeMap[solidProps1.size]} className={`${sizeClasses[solidProps1.size]} animate-spin motion-reduce:animate-none text-[var(--bichat-primary)]`}/>
      {solidProps1.message && <p class="mt-2 text-sm text-gray-600 dark:text-gray-400">{solidProps1.message}</p>}
    </div>);
}
function DotsLoader(solidProps2Input: {
    size: 'sm' | 'md' | 'lg';
    message?: string;
}) {
    const solidProps2 = mergeProps({ size: 'md' } as const, solidProps2Input);
    const dotSizeClasses = {
        sm: 'w-1.5 h-1.5',
        md: 'w-2 h-2',
        lg: 'w-3 h-3',
    };
    const gapClasses = {
        sm: 'gap-0.5',
        md: 'gap-1',
        lg: 'gap-1.5',
    };
    return (<div class="flex flex-col items-center justify-center" role="status" aria-live="polite">
      <div class={`flex ${gapClasses[solidProps2.size]}`}>
        {[0, 1, 2].map((index) => (<div class={`${dotSizeClasses[solidProps2.size]} bg-[var(--bichat-primary)] rounded-full animate-bounce motion-reduce:animate-none`} style={{ "animation-delay": `${index * 0.15}s` }}/>))}
      </div>
      {solidProps2.message && <p class="mt-3 text-sm text-gray-600 dark:text-gray-400">{solidProps2.message}</p>}
    </div>);
}
function PulseLoader(solidProps3Input: {
    size: 'sm' | 'md' | 'lg';
    message?: string;
}) {
    const solidProps3 = mergeProps({ size: 'md' } as const, solidProps3Input);
    const sizeClasses = {
        sm: 'h-4 w-4',
        md: 'h-8 w-8',
        lg: 'h-12 w-12',
    };
    return (<div class="flex flex-col items-center justify-center" role="status" aria-live="polite">
      <div class={`${sizeClasses[solidProps3.size]} bg-[var(--bichat-primary)] rounded-full animate-pulse motion-reduce:animate-none`}/>
      {solidProps3.message && <p class="mt-2 text-sm text-gray-600 dark:text-gray-400">{solidProps3.message}</p>}
    </div>);
}
function LoadingSpinner(solidProps4Input: LoadingSpinnerProps) {
    const solidProps4 = mergeProps({ variant: 'spinner', size: 'md' } as const, solidProps4Input);
    return <>{createMemo(() => {
            switch (solidProps4.variant) {
                case 'dots':
                    return <DotsLoader size={solidProps4.size} message={solidProps4.message}/>;
                case 'pulse':
                    return <PulseLoader size={solidProps4.size} message={solidProps4.message}/>;
                case 'spinner':
                default:
                    return <SpinnerLoader size={solidProps4.size} message={solidProps4.message}/>;
            }
        })}</>;
}
const MemoizedLoadingSpinner = LoadingSpinner;
MemoizedLoadingSpinner; /* Solid components are named by their declarations. */
export { MemoizedLoadingSpinner as LoadingSpinner };
export default MemoizedLoadingSpinner;
