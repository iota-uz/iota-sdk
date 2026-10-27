import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * ConfirmModal Component
 * Polished confirmation dialog with contextual icon, refined typography,
 * and smooth micro-interactions.
 * Uses @headlessui/react Dialog for accessible modal behavior.
 */
import { InlineDialog, InlineDialogBackdrop, InlineDialogPanel, InlineDialogTitle, InlineDialogDescription, } from './InlineDialog';
import { WarningCircle } from '../icons';
import { useTranslation } from '../hooks/useTranslation';
export interface ConfirmModalProps {
    /** Whether the modal is open */
    isOpen: boolean;
    /** Modal title */
    title: string;
    /** Modal message/description */
    message: string;
    /** Callback when user confirms */
    onConfirm: () => void;
    /** Callback when user cancels */
    onCancel: () => void;
    /** Confirm button text (defaults to "Confirm") */
    confirmText?: string;
    /** Cancel button text (defaults to "Cancel") */
    cancelText?: string;
    /** Whether this is a danger/destructive action (red confirm button) */
    isDanger?: boolean;
}
function ConfirmModalBase(solidProps1Input: ConfirmModalProps) {
    const solidProps1 = mergeProps({ isDanger: false } as const, solidProps1Input);
    const solidState2 = useTranslation();
    const resolvedConfirmText = createMemo(() => solidProps1.confirmText?.trim() ? solidProps1.confirmText : solidState2.t('BiChat.Common.Confirm'));
    const resolvedCancelText = createMemo(() => solidProps1.cancelText?.trim() ? solidProps1.cancelText : solidState2.t('BiChat.Common.Cancel'));
    return (<InlineDialog open={solidProps1.isOpen} onClose={solidProps1.onCancel} className="relative z-40">
      {/* Backdrop */}
      <InlineDialogBackdrop className="fixed inset-0 bg-black/40 dark:bg-black/60 backdrop-blur-sm transition-opacity duration-200"/>

      {/* Modal */}
      <div class="fixed inset-0 flex items-center justify-center z-50 p-4">
        <InlineDialogPanel className="bg-white dark:bg-gray-800 rounded-2xl shadow-xl dark:shadow-2xl dark:shadow-black/30 max-w-sm w-full overflow-hidden">
          <div class="px-6 pt-6 pb-5">
            {/* Icon + Title */}
            <div class="flex items-start gap-4">
              {solidProps1.isDanger && (<div class="flex-shrink-0 flex items-center justify-center w-10 h-10 rounded-xl bg-red-50 dark:bg-red-950/40 border border-red-200/60 dark:border-red-800/40">
                  <WarningCircle size={22} weight="duotone" className="text-red-600 dark:text-red-400"/>
                </div>)}
              <div class="flex-1 min-w-0">
                <InlineDialogTitle className="text-base font-semibold text-gray-900 dark:text-gray-100 leading-snug">
                  {solidProps1.title}
                </InlineDialogTitle>
                <InlineDialogDescription className="mt-2 text-sm text-gray-600 dark:text-gray-400 leading-relaxed">
                  {solidProps1.message}
                </InlineDialogDescription>
              </div>
            </div>
          </div>

          {/* Actions */}
          <div class="flex items-center justify-end gap-2.5 px-6 pb-5">
            <button type="button" onClick={solidProps1.onCancel} {...(solidProps1.isDanger ? { 'data-autofocus': true } : {})} class="cursor-pointer px-4 py-2 text-sm font-medium rounded-xl text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700/60 hover:bg-gray-200 dark:hover:bg-gray-700 active:bg-gray-250 dark:active:bg-gray-600 transition-colors duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-gray-800" data-testid="confirm-modal-cancel">
              {resolvedCancelText()}
            </button>
            <button type="button" {...(!solidProps1.isDanger ? { 'data-autofocus': true } : {})} onClick={solidProps1.onConfirm} class={[
            'cursor-pointer px-4 py-2 text-sm font-medium rounded-xl text-white',
            'transition-all duration-150 shadow-sm hover:shadow',
            'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-gray-800',
            solidProps1.isDanger ? 'bg-red-600 hover:bg-red-700 active:bg-red-800 focus-visible:ring-red-500/50'
                : 'bg-primary-600 hover:bg-primary-700 active:bg-primary-800 focus-visible:ring-primary-500/50',
        ].join(' ')} data-testid="confirm-modal-confirm">
              {resolvedConfirmText()}
            </button>
          </div>
        </InlineDialogPanel>
      </div>
    </InlineDialog>);
}
const ConfirmModal = ConfirmModalBase;
ConfirmModal; /* Solid components are named by their declarations. */
export { ConfirmModal };
export default ConfirmModal;
