import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * StreamingCursor Component
 * Animated cursor shown during AI response streaming
 */
import { useTranslation } from '../hooks/useTranslation';
function StreamingCursor() {
    const solidState1 = useTranslation();
    return (<span class="inline-block w-1.5 h-4 ml-0.5 bg-primary-600 dark:bg-primary-500 animate-pulse" aria-label={solidState1.t('BiChat.Common.AITyping')}/>);
}
export { StreamingCursor };
export default StreamingCursor;
