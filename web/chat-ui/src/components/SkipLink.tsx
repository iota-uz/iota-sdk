import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * Skip to main content link for keyboard navigation
 * Hidden by default, visible on keyboard focus
 * Allows users to skip navigation and go directly to main content
 */
import { useTranslation } from '../hooks/useTranslation';
export default function SkipLink() {
    const solidState1 = useTranslation();
    return (<a href="#main-content" class="sr-only focus-visible:not-sr-only focus-visible:absolute focus-visible:top-4 focus-visible:left-4 focus-visible:z-50 focus-visible:bg-primary-600 focus-visible:text-white focus-visible:px-4 focus-visible:py-2 focus-visible:rounded-lg focus-visible:shadow-lg">
      {solidState1.t('BiChat.SkipLink.Label')}
    </a>);
}
