/**
 * ConfirmModal Component
 * Generic confirmation dialog with customizable title, message, and actions
 */

import { useEffect, useRef, memo } from 'react'
import { useFocusTrap } from '../hooks/useFocusTrap'

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
  const modalRef = useRef<HTMLDivElement>(null)

  // Trap focus within modal when open
  useFocusTrap(modalRef, isOpen)

  useEffect(() => {
    if (!isOpen) return

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onCancel()
      } else if (e.key === 'Enter') {
        onConfirm()
      }
    }

    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [isOpen, onConfirm, onCancel])

  if (!isOpen) return null

  return (
    <>
      {/* Backdrop */}
      <div
        className="fixed inset-0 bg-black/40 dark:bg-black/60 backdrop-blur-sm transition-opacity z-40"
        onClick={onCancel}
        aria-hidden="true"
      />

      {/* Modal */}
      <div
        className="fixed inset-0 flex items-center justify-center z-50"
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="confirm-modal-title"
        aria-describedby="confirm-modal-description"
      >
        <div
          ref={modalRef}
          className="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-sm w-full mx-4"
        >
          {/* Header */}
          <div className="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
            <h2
              id="confirm-modal-title"
              className="text-lg font-semibold text-gray-900 dark:text-white"
            >
              {title}
            </h2>
          </div>

          {/* Body */}
          <div className="px-6 py-4">
            <p id="confirm-modal-description" className="text-gray-600 dark:text-gray-300">
              {message}
            </p>
          </div>

          {/* Footer - Actions */}
          <div className="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex justify-end gap-3">
            <button
              onClick={onCancel}
              className="px-4 py-2 rounded-lg bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-gray-100 hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors font-medium"
              aria-label={`Cancel ${title.toLowerCase()}`}
              data-testid="confirm-modal-cancel"
            >
              {cancelText}
            </button>
            <button
              onClick={onConfirm}
              className={`px-4 py-2 rounded-lg text-white font-medium transition-colors ${
                isDanger
                  ? 'bg-red-600 dark:bg-red-700 hover:bg-red-700 dark:hover:bg-red-800'
                  : 'bg-primary-600 dark:bg-primary-700 hover:bg-primary-700 dark:hover:bg-primary-800'
              }`}
              aria-label={`Confirm ${title.toLowerCase()}`}
              data-testid="confirm-modal-confirm"
            >
              {confirmText}
            </button>
          </div>
        </div>
      </div>
    </>
  )
}

const ConfirmModal = memo(ConfirmModalBase)
ConfirmModal.displayName = 'ConfirmModal'

export { ConfirmModal }
export default ConfirmModal
