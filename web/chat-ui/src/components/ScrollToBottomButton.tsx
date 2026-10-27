import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * ScrollToBottomButton Component
 * Floating button to scroll chat to bottom, shown when user scrolls up
 */
import { ArrowDown } from '../icons';
import { useTranslation } from '../hooks/useTranslation';
interface ScrollToBottomButtonProps {
    show: boolean;
    onClick: () => void;
    unreadCount?: number;
    disabled?: boolean;
    /** When set, renders a pill-style button with this label (e.g. "New messages") */
    label?: string;
}
function ScrollToBottomButton(solidProps1Input: ScrollToBottomButtonProps) {
    const solidProps1 = mergeProps({ unreadCount: 0, disabled: false } as const, solidProps1Input);
    const solidState2 = useTranslation();
    return (<>
      {solidProps1.show && (<div class="absolute bottom-8 z-10 pointer-events-none" style={{ "left": '50%', "transform": 'translateX(-50%)' }}>
          <button onClick={solidProps1.disabled ? undefined : solidProps1.onClick} disabled={solidProps1.disabled} class={`pointer-events-auto cursor-pointer shadow-lg border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700 active:bg-gray-100 dark:active:bg-gray-600 transition-colors duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-gray-900 disabled:opacity-40 disabled:cursor-not-allowed ${solidProps1.label ? 'flex items-center gap-1.5 px-4 py-2 rounded-full bg-primary-600 dark:bg-primary-500 border-primary-600 dark:border-primary-500 hover:bg-primary-700 dark:hover:bg-primary-600 active:bg-primary-800 dark:active:bg-primary-700'
                : 'p-2.5 rounded-full bg-white dark:bg-gray-800'}`} aria-label={solidProps1.label || solidState2.t('BiChat.Common.ScrollToBottom')}>
            {solidProps1.label ? (<>
                <span class="text-sm font-medium text-white">{solidProps1.label}</span>
                <ArrowDown size={16} weight="bold" className="text-white"/>
              </>) : (<div class="relative">
                <ArrowDown size={18} weight="bold" className="text-gray-700 dark:text-gray-300"/>

                {/* Unread count badge */}
                {solidProps1.unreadCount > 0 && (<span class="absolute -top-2 -right-2 min-w-[18px] h-[18px] bg-primary-600 dark:bg-primary-500 text-white text-xs font-semibold rounded-full flex items-center justify-center px-1" aria-live="polite" aria-atomic="true">
                    {solidProps1.unreadCount > 99 ? '99+' : solidProps1.unreadCount}
                  </span>)}
              </div>)}
          </button>
        </div>)}
    </>);
}
export { ScrollToBottomButton };
export default ScrollToBottomButton;
