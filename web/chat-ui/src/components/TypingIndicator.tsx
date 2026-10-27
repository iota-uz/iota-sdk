import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
import { verbTransitionVariants } from '../animations/variants';
import { useTranslation } from '../hooks/useTranslation';
export interface TypingIndicatorProps {
    /** Custom thinking verbs to rotate through */
    verbs?: string[];
    /** Verb rotation interval in ms (defaults to 3000) */
    rotationInterval?: number;
    /** Additional CSS classes */
    className?: string;
}
// Translation keys for default thinking verbs
const THINKING_KEYS = [
    'BiChat.Thinking.Thinking',
    'BiChat.Thinking.Processing',
    'BiChat.Thinking.Analyzing',
    'BiChat.Thinking.Synthesizing',
    'BiChat.Thinking.Computing',
    'BiChat.Thinking.WorkingOnIt',
];
// Check if user prefers reduced motion
const prefersReducedMotion = () => {
    if (typeof window === 'undefined') {
        return false;
    }
    return window.matchMedia('(prefers-reduced-motion: reduce)').matches;
};
// Random selector without immediate repeat
const getRandomVerb = (verbs: string[], current: string): string => {
    const available = verbs.filter((v) => v !== current);
    if (available.length === 0) {
        return current || verbs[0] || '';
    }
    return available[Math.floor(Math.random() * available.length)];
};
function TypingIndicator(solidProps1Input: TypingIndicatorProps) {
    const solidProps1 = mergeProps({ rotationInterval: 3000, className: '' } as const, solidProps1Input);
    const solidState2 = useTranslation();
    const verbs = createMemo(() => {
        if (solidProps1.verbs) {
            return solidProps1.verbs;
        }
        return THINKING_KEYS.map((key) => solidState2.t(key));
    });
    const [verb, setVerb] = createSignal((() => verbs()[Math.floor(Math.random() * verbs().length)])());
    createEffect(on(() => [verbs(), solidProps1.rotationInterval], () => {
        const cleanup = untrack(() => {
            if (prefersReducedMotion()) {
                return;
            }
            const interval = setInterval(() => {
                setVerb((prev) => getRandomVerb(verbs(), prev));
            }, solidProps1.rotationInterval);
            return () => clearInterval(interval);
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    return (<div role="status" aria-live="polite" class={`flex items-center gap-2.5 text-gray-500 dark:text-gray-400 ${solidProps1.className}`}>
      <div class="flex items-center gap-1" aria-hidden="true">
        <span class="w-1.5 h-1.5 rounded-full bg-gray-400 dark:bg-gray-500 animate-bounce motion-reduce:animate-none [animation-delay:0ms]"/>
        <span class="w-1.5 h-1.5 rounded-full bg-gray-400 dark:bg-gray-500 animate-bounce motion-reduce:animate-none [animation-delay:150ms]"/>
        <span class="w-1.5 h-1.5 rounded-full bg-gray-400 dark:bg-gray-500 animate-bounce motion-reduce:animate-none [animation-delay:300ms]"/>
      </div>
      <div class="overflow-hidden h-6 relative">
        
          <span class="text-sm bichat-thinking-shimmer block" aria-label={solidState2.t('BiChat.Thinking.AriaLabel', { get verb() {
                return verb();
            } })}>
            {verb()}...
          </span>
        
      </div>
    </div>);
}
const MemoizedTypingIndicator = TypingIndicator;
MemoizedTypingIndicator; /* Solid components are named by their declarations. */
export { MemoizedTypingIndicator as TypingIndicator };
export default MemoizedTypingIndicator;
