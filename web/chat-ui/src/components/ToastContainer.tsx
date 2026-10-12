import { Portal as HostPortal } from '@iota-uz/sdk/solid';
import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * ToastContainer Component
 * Container for rendering toast notifications.
 * Positioned at bottom-right to stay out of the way of primary content.
 */
import { Toast } from './Toast';
import type { ToastItem } from '../hooks/useToast';
import { useTranslation } from '../hooks/useTranslation';
interface ToastContainerProps {
    toasts: ToastItem[];
    onDismiss: (id: string) => void;
    /** Label for dismiss buttons */
    dismissLabel?: string;
}
export function ToastContainer(solidProps1: ToastContainerProps) {
    const solidState2 = useTranslation();
    return <Show when={!(solidProps1.toasts.length === 0)}>{_visible => {
            return (<HostPortal surface="toast" label={solidState2.t('BiChat.Common.Notifications')}><div aria-label={solidState2.t('BiChat.Common.Notifications')} class="fixed top-6 right-6 z-[var(--bichat-z-toast,60)] flex flex-col gap-2 pointer-events-none">
      <For each={solidProps1.toasts}>{toast => (<div class="pointer-events-auto">
          <Toast {...toast} onDismiss={solidProps1.onDismiss} dismissLabel={solidProps1.dismissLabel}/>
        </div>)}</For>
    </div></HostPortal>);
        }}</Show>;
}
export default ToastContainer;
