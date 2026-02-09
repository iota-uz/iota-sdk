/**
 * ToastContainer Component
 * Container for rendering toast notifications with animations
 */

import { AnimatePresence } from 'framer-motion'
import { Toast } from './Toast'
import type { ToastItem } from '../hooks/useToast'

interface ToastContainerProps {
  toasts: ToastItem[]
  onDismiss: (id: string) => void
  /** Label for dismiss buttons */
  dismissLabel?: string
}

export function ToastContainer({ toasts, onDismiss, dismissLabel }: ToastContainerProps) {
  return (
    <div className="fixed top-4 right-4 sm:top-6 sm:right-6 z-50 flex flex-col gap-2 pointer-events-none">
      <AnimatePresence>
        {toasts.map((toast) => (
          <div key={toast.id} className="pointer-events-auto">
            <Toast {...toast} onDismiss={onDismiss} dismissLabel={dismissLabel} />
          </div>
        ))}
      </AnimatePresence>
    </div>
  )
}

export default ToastContainer
