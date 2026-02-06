/**
 * ConfirmModal Component
 * Polished confirmation dialog with contextual icon, refined typography,
 * and smooth micro-interactions.
 * Uses @headlessui/react Dialog for accessible modal behavior.
 */

import { memo } from 'react'
import { Dialog, DialogBackdrop, DialogPanel, DialogTitle, Description } from '@headlessui/react'
import { WarningCircle } from '@phosphor-icons/react'

export interface ConfirmModalProps {
  /** Whether the modal is open */
  isOpen: boolean
  /** Modal title */
  title: string
  /** Modal message/description */
  message: string
  /** Callback when user confirms */
  onConfirm: () => void
  /** Callback when user cancels */
  onCancel: () => void
  /** Confirm button text (defaults to "Confirm") */
  confirmText?: string
  /** Cancel button text (defaults to "Cancel") */
  cancelText?: string
  /** Whether this is a danger/destructive action (red confirm button) */
  isDanger?: boolean
}

function ConfirmModalBase({
  isOpen,
  title,
  message,
  onConfirm,
  onCancel,
  confirmText = 'Confirm',
  cancelText = 'Cancel',
  isDanger = false,
}: ConfirmModalProps) {
  return (
    <Dialog open={isOpen} onClose={onCancel} className="relative z-40">
      {/* Backdrop */}
      <DialogBackdrop className="fixed inset-0 bg-black/40 dark:bg-black/60 backdrop-blur-sm transition-opacity duration-200" />

      {/* Modal */}
      <div className="fixed inset-0 flex items-center justify-center z-50 p-4">
        <DialogPanel className="bg-white dark:bg-gray-800 rounded-2xl shadow-xl dark:shadow-2xl dark:shadow-black/30 max-w-sm w-full overflow-hidden">
          <div className="px-6 pt-6 pb-5">
            {/* Icon + Title */}
            <div className="flex items-start gap-3.5">
              {isDanger && (
                <div className="flex-shrink-0 flex items-center justify-center w-10 h-10 rounded-xl bg-red-50 dark:bg-red-950/40 border border-red-200/60 dark:border-red-800/40">
                  <WarningCircle size={22} weight="duotone" className="text-red-600 dark:text-red-400" />
                </div>
              )}
              <div className="flex-1 min-w-0">
                <DialogTitle className="text-base font-semibold text-gray-900 dark:text-gray-100 leading-snug">
                  {title}
                </DialogTitle>
                <Description className="mt-2 text-sm text-gray-600 dark:text-gray-400 leading-relaxed">
                  {message}
                </Description>
              </div>
            </div>
          </div>

          {/* Actions */}
          <div className="flex items-center justify-end gap-2.5 px-6 pb-5">
            <button
              onClick={onCancel}
              className="cursor-pointer px-4 py-2 text-sm font-medium rounded-xl text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700/60 hover:bg-gray-200 dark:hover:bg-gray-700 active:bg-gray-250 dark:active:bg-gray-600 transition-colors duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-gray-800"
              aria-label={`Cancel ${title.toLowerCase()}`}
              data-testid="confirm-modal-cancel"
            >
              {cancelText}
            </button>
            <button
              onClick={onConfirm}
              className={[
                'cursor-pointer px-4 py-2 text-sm font-medium rounded-xl text-white',
                'transition-all duration-150 shadow-sm hover:shadow',
                'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-gray-800',
                isDanger
                  ? 'bg-red-600 hover:bg-red-700 active:bg-red-800 focus-visible:ring-red-500/50'
                  : 'bg-primary-600 hover:bg-primary-700 active:bg-primary-800 focus-visible:ring-primary-500/50',
              ].join(' ')}
              aria-label={`Confirm ${title.toLowerCase()}`}
              data-testid="confirm-modal-confirm"
            >
              {confirmText}
            </button>
          </div>
        </DialogPanel>
      </div>
    </Dialog>
  )
}

const ConfirmModal = memo(ConfirmModalBase)
ConfirmModal.displayName = 'ConfirmModal'

export { ConfirmModal }
export default ConfirmModal
